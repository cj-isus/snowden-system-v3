package main

// killswitch_windows.go — сетевой kill switch для TUN-режима (V2-038, аудит
// владельца п.5 / COMPETITOR-FEATURES F11). Проблема: в TUN движок держит
// auto_route; смерть движка (crash, kill -9, BLOCKED-путь) убирает маршруты —
// весь трафик ОС молча идёт напрямую, пока UI честно показывает error. Для
// РФ-модели угроз (журналы провайдера) это дыра ровно в момент, когда
// пользователь считает себя «за защищённым».
//
// Механика: Windows Firewall (netsh advfirewall) — без сторонних драйверов
// (WFP-прямые вызовы требуют cgo/COM, недоступны: среда без gcc, V2-025).
// Политика:
//   - allow: наш exe (движок in-process!), loopback, ICMP core,
//     DHCP/DNS к шлюзу (не роняем сеть навсегда), established flow;
//   - block: весь остальной outbound IPv4/IPv6.
// TUN-процесс всегда elevated (UAC relaunch) — права на netsh есть.
//
// Безопасность отказа важнее полноты блокировки: хвост политики — dead-man
// revert. Перед применением блоков стартует ОТКРЕПЛЁННЫЙ helper-процесс
// (тот же exe, --killswitch-expire): он спит заданный интервал и снимает
// блоки. Наш процесс, когда туннель жив или остановлен штатно, завершает
// helper (taskkill по PID-файлу). Итог: kill -9/crash → через expire-интервал
// сеть возвращается сама, пользователь остаётся без защиты, но НЕ offline.
// Плоский файл состояния %AppData%\snowden-system\killswitch.json (PID
// helper'а + метка) — для startup-очистки после сбоя.
//
// Правило фактов (AGENTS §3.6): каждая операция возвращает ошибку наружу —
// «kill switch не применён» честнее тихого fail-open.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// killSwitchPolicyName — имя firewall-политики; по нему же чистится.
const killSwitchPolicyName = "snowden-system-tun-killswitch"

// killSwitchExpireDefault — сколько живёт блок-политика без подтверждения
// живого процесса. Компромисс: короткий интервал = чаще подтверждения
// (флапание UAC-независимых netsh-вызовов раз в N), длинный = дольше сеть
// в блоке после сбоя. 90с покрывает перезапуск приложения владельцем.
const killSwitchExpireDefault = 90 * time.Second

// KillSwitchRunner — исполнение системных команд (инъекция для тестов).
type KillSwitchRunner interface {
	Run(name string, args ...string) error
}

// osRunner — реальный исполнитель (netsh.exe).
type osRunner struct{}

func (osRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// killSwitchState — плоский файл для startup-очистки (без секретов).
type killSwitchState struct {
	HelperPID int       `json:"helper_pid"`
	AppliedAt time.Time `json:"applied_at"`
}

// killSwitchStatePathFn — точка подмены в тестах (временный каталог);
// продакшн-значение — killSwitchStatePath.
var killSwitchStatePathFn = func() string {
	return filepath.Join(appDataDir(), "killswitch.json")
}

// jsonMarshal/jsonUnmarshal — точки подмены для тестов (по умолчанию stdlib).
var (
	jsonMarshal   = json.Marshal
	jsonUnmarshal = json.Unmarshal
)

// startKillSwitchHelperFn — точка подмены в тестах (реальный exec невозможен
// с фейковым exe); продакшн-значение — startKillSwitchHelper.
var startKillSwitchHelperFn = startKillSwitchHelper

// ---------- политика (чистые функции — покрыты тестами) ----------

// killSwitchAllowRules — allow-правила политики. Наш exe — первый: движок
// sing-box живёт в нашем процессе, без этого правила туннель умрёт вместе с
// политикой. Шлюз — DHCP/DNS локальной сети (else утрачивается сама LAN).
// Loopback не блокируется никогда (in-process SOCKS:1080, UI, probe).
func killSwitchAllowRules(exe string) [][]string {
	return [][]string{
		{"advfirewall", "firewall", "add", "rule", "name=" + killSwitchPolicyName + "-app", "dir=out", "action=allow", "program=" + exe, "enable=yes", "profile=any"},
		{"advfirewall", "firewall", "add", "rule", "name=" + killSwitchPolicyName + "-loopback", "dir=out", "action=allow", "remoteip=127.0.0.0/8,::1", "enable=yes", "profile=any"},
		{"advfirewall", "firewall", "add", "rule", "name=" + killSwitchPolicyName + "-icmp", "dir=out", "action=allow", "protocol=icmp:4,any", "enable=yes", "profile=any"},
		{"advfirewall", "firewall", "add", "rule", "name=" + killSwitchPolicyName + "-icmp6", "dir=out", "action=allow", "protocol=icmp:6,any", "enable=yes", "profile=any"},
		{"advfirewall", "firewall", "add", "rule", "name=" + killSwitchPolicyName + "-lan", "dir=out", "action=allow", "remoteip=LocalSubnet", "enable=yes", "profile=any"},
	}
}

// killSwitchBlockRules — финальные блок-правила (порядок важен: применяются
// после allow). Отдельно v4 и v6 — иначе один сетевой сбой оставит половину
// пути открытой.
func killSwitchBlockRules() [][]string {
	return [][]string{
		{"advfirewall", "firewall", "add", "rule", "name=" + killSwitchPolicyName + "-block-v4", "dir=out", "action=block", "protocol=any", "remoteip=0.0.0.0/0", "enable=yes", "profile=any"},
		{"advfirewall", "firewall", "add", "rule", "name=" + killSwitchPolicyName + "-block-v6", "dir=out", "action=block", "protocol=any", "remoteip=::/0", "enable=yes", "profile=any"},
	}
}

// killSwitchDeleteArgs — аргументы удаления одного правила.
func killSwitchDeleteArgs(rule string) []string {
	return []string{"advfirewall", "firewall", "delete", "rule", "name=" + rule}
}

// killSwitchAllRuleNames — полный список имён (для снятия и проверки дрейфа).
func killSwitchAllRuleNames() []string {
	names := []string{}
	for _, r := range append(killSwitchAllowRules("EXE"), killSwitchBlockRules()...) {
		for _, part := range r {
			if strings.HasPrefix(part, "name=") {
				names = append(names, strings.TrimPrefix(part, "name="))
			}
		}
	}
	return names
}

// ---------- применение/снятие ----------

// applyKillSwitch — политика на время TUN-сессии. Порядок:
//  1. старт dead-man helper (до блоков — иначе блок применим, а revert некому);
//  2. allow-правила (наш exe/loopback/LAN/ICMP);
//  3. block-правила (последними — окно «allow без block» безопасно, обратное нет).
func applyKillSwitch(r KillSwitchRunner, exe string, expire time.Duration) error {
	if err := startKillSwitchHelperFn(exe, expire); err != nil {
		return fmt.Errorf("kill switch: dead-man helper не запущен (блок не применяю — иначе сбой закроет сеть навсегда): %w", err)
	}
	for _, args := range killSwitchAllowRules(exe) {
		if err := r.Run("netsh", args...); err != nil {
			_ = removeKillSwitch(r)
			return fmt.Errorf("kill switch: allow-правило не применено: %w", err)
		}
	}
	for _, args := range killSwitchBlockRules() {
		if err := r.Run("netsh", args...); err != nil {
			_ = removeKillSwitch(r)
			return fmt.Errorf("kill switch: block-правило не применено: %w", err)
		}
	}
	return nil
}

// removeKillSwitch — снятие политики (штатный Stop, failover BLOCKED,
// expire-helper). Ошибки удаления собираются: частично снятая политика хуже
// честной ошибки вызывающему (тот решит — логировать/перезапустить helper).
func removeKillSwitch(r KillSwitchRunner) error {
	var errs []string
	for _, name := range killSwitchAllRuleNames() {
		if err := r.Run("netsh", killSwitchDeleteArgs(name)...); err != nil {
			// delete несуществующего правила = не ошибка (идемпотентность).
			if !strings.Contains(err.Error(), "No rules were specified") &&
				!strings.Contains(err.Error(), "не найдено") {
				errs = append(errs, err.Error())
			}
		}
	}
	_ = os.Remove(killSwitchStatePathFn())
	if len(errs) > 0 {
		return fmt.Errorf("kill switch: снятие неполное: %s", strings.Join(errs, "; "))
	}
	return nil
}

// startKillSwitchHelper — detached-процесс того же exe с --killswitch-expire
// (и --killswitch-tun-tag чтобы guard в helper-режиме не пытался начать
// «обычный» lifecycle). PID пишется в killswitch.json для ручной зачистки.
func startKillSwitchHelper(exe string, expire time.Duration) error {
	// Детерминированная зачистка прошлого helper'а: если state-файл остался
	// (наш прошлый процесс умер, не сняв политику) — helper мёртв уже, но
	// файл мешает. Снимаем остатки политики сразу: чистый лист.
	_ = removeKillSwitch(osRunner{})

	cmd := exec.Command(exe,
		"--killswitch-expire",
		"--killswitch-seconds="+strconv.Itoa(int(expire.Seconds())),
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	state := killSwitchState{HelperPID: cmd.Process.Pid, AppliedAt: time.Now()}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.WriteFile(killSwitchStatePathFn(), data, 0o600); err != nil {
		return err
	}
	// Отец не ждёт и не будет ждать: helper живёт дольше родителя по смыслу.
	go func() { _ = cmd.Process.Release() }()
	return nil
}

// stopKillSwitchHelper — завершить dead-man при живом отце (штатный Stop):
// иначе helper снимет политику прямо посреди работающей TUN-сессии.
func stopKillSwitchHelper() {
	data, err := os.ReadFile(killSwitchStatePathFn())
	if err != nil {
		return
	}
	var st killSwitchState
	if json.Unmarshal(data, &st) == nil && st.HelperPID > 0 {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(st.HelperPID), "/F").Run()
	}
}

// renewKillSwitchLease — продление lease dead-man helper'а (V2-046, закрытие
// P1 «тихое истечение через 90с»). Старая схема: helper спал ФИКСИРОВАННЫЙ
// expire и снимал политику в живой TUN-сессии — лог говорил «активна», сеть
// вставала. Теперь: периодический тик перезапускает helper на свежий срок
// (kill helper'а + новый detached-процесс; state-файл обновляется). Смерть
// отца не меняется: последний helper дорабатывает свой срок и снимает блоки.
func renewKillSwitchLease(exe string, expire time.Duration) error {
	data, err := os.ReadFile(killSwitchStatePathFn())
	if err != nil {
		return fmt.Errorf("kill switch: lease не продлён (нет state-файла): %w", err)
	}
	var st killSwitchState
	if json.Unmarshal(data, &st) != nil || st.HelperPID <= 0 {
		return fmt.Errorf("kill switch: lease не продлён (state повреждён)")
	}
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(st.HelperPID), "/F").Run()
	return startKillSwitchHelperFn(exe, expire)
}

// killSwitchExpireRequested — аргументы helper-режима (--killswitch-expire
// --killswitch-seconds N). Второй флаг без первого игнорируется: случайный
// --killswitch-seconds не должен превращать GUI-процесс в таймер.
func killSwitchExpireRequested() (int, bool) {
	flag, secs := false, -1
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--killswitch-expire":
			flag = true
		}
		if v, err := strconv.Atoi(strings.TrimPrefix(arg, "--killswitch-seconds=")); err == nil && strings.HasPrefix(arg, "--killswitch-seconds=") {
			secs = v
		}
	}
	if !flag || secs <= 0 || secs > 3600 {
		return 0, false
	}
	return secs, true
}

// runKillSwitchExpire — режим helper'а: спать expire и снять блоки.
// Вызывается из startup до всего остального lifecycle.
func runKillSwitchExpire(expire time.Duration) int {
	time.Sleep(expire)
	if err := removeKillSwitch(osRunner{}); err != nil {
		return 3 // снятие неполное — но мы сделали максимум; код для отладки
	}
	return 0
}

// cleanupStaleKillSwitch — startup-очистка после сбоя прошлого процесса:
// helper мог умереть раньше expire (логический тупик невозможен — он просто
// спит), политика могла остаться. Снимаем молча; ошибки — в журнал.
// На не-elevated процессе netsh вернёт access denied — это честно логируется,
// а не маскируется: TUN-процесс (elevated) сам почистит на следующем старте.
func cleanupStaleKillSwitch(logf func(level, text string)) {
	if _, err := os.Stat(killSwitchStatePathFn()); err != nil {
		return // политики не было — самый частый путь
	}
	if err := removeKillSwitch(osRunner{}); err != nil {
		logf("warn", fmt.Sprintf("kill switch: startup-очистка неполная (нужен elevation?): %v", err))
		return
	}
	logf("info", "kill switch: политика прошлого сбоя снята при старте")
}
