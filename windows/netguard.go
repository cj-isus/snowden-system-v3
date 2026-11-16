package main

// netguard.go — фактическое состояние NetGuard (tools/netguard) для UI-страницы
// и legacy recovery API (без original receipt недоступен). Только честные
// факты: «нет доступа», «не найдена», «нет данных» — никогда не выдумка.
//
// Задачи планировщика читаются через Task Scheduler COM API (Schedule.Service):
//   - поля типизированы (State int, VT_DATE → time.Time) — не зависят от
//     локали, в отличие от schtasks CSV (где /nh даёт 3 колонки у user-задач:
//     Name, NextRun, Status — и «Готова» уезжает в NextRun, Status пустой);
//   - ошибка GetTask различает «задача скрыта правами» (0x80070005) и «задачи
//     нет» (0x80070002) — schtasks в обоих случаях даёт exit 1 с
//     локализованным текстом, а COM даёт точный scode (probe 2026-09-10:
//     non-elevated, оба случая воспроизведены, см. .tmp/probe-com).

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/go-ole/go-ole"
	"golang.org/x/text/encoding/charmap"

	"github.com/snowden-system/windows/backend/core"
)

// NetGuardTask — состояние одной плановой задачи auto-heal.
type NetGuardTask struct {
	Name     string `json:"name"`
	Status   string `json:"status"`   // машинный: ready/running/disabled/queued/hidden/no-access/not-found/unknown:<n>
	LastRun  string `json:"lastRun"`  // RFC3339 conversion; empty = no reported date, not history proof
	NextRun  string `json:"nextRun"`  // RFC3339 conversion; empty does not prove no schedule
	LastCode string `json:"lastCode"` // код возврата последнего запуска (десятичный)
}

// NetGuardProxy — системный прокси HKCU и живость его loopback-листенера.
type NetGuardProxy struct {
	Enabled       bool   `json:"enabled"`
	Server        string `json:"server"`
	Port          int    `json:"port"`          // 0 = не loopback-формат
	ListenerAlive bool   `json:"listenerAlive"` // актуально только при Port>0
	Stale         bool   `json:"stale"`         // legacy field; no ownership assertion from port reachability
	readErr       error  // internal only: an unknown registry read is not disabled
}

// NetGuardEvent — событие из лога auto-heal.
type NetGuardEvent struct {
	Time string `json:"time"`
	Kind string `json:"kind"` // HEALED | HEALING | PROBLEM | WEEKLY
	Text string `json:"text"`
}

// NetGuardStatus — всё, что показывает страница NetGuard.
type NetGuardStatus struct {
	Tasks       []NetGuardTask             `json:"tasks"`
	Proxy       NetGuardProxy              `json:"proxy"`
	SocksAlive  bool                       `json:"socksAlive"` // legacy local TCP fact, never VPN identity
	Events      []NetGuardEvent            `json:"events"`
	LastRun     string                     `json:"lastRun"`
	DoHDisabled bool                       `json:"dohDisabled"` // legacy field; log absence cannot prove DNS policy
	Log         NetGuardLogObservation     `json:"log"`
	Loopbacks   []core.LoopbackObservation `json:"loopbacks"`
}

// netguardTaskNames — плановые задачи auto-heal в порядке отображения.
var netguardTaskNames = []string{"NetGuard-AutoHeal", "NetGuard-AutoHeal-User", "NetGuard-AutoHeal-NetEvent"}

// NetGuardStatus — биндинг для UI (Wails).
func (a *App) NetGuardStatus() (*NetGuardStatus, error) {
	parent := a.observationParent() // before COM/registry/log waits, never recapture
	st := &NetGuardStatus{Tasks: []NetGuardTask{}, Events: []NetGuardEvent{}}

	// 1. Плановые задачи (COM API; каждая ошибка — отдельный честный факт).
	st.Tasks = netguardAllTasks(netguardTaskNames)

	// 2. Системный прокси (HKCU) + живость SOCKS:1080.
	st.Proxy = netguardProxyState()
	if st.Proxy.readErr != nil {
		return nil, fmt.Errorf("system proxy observation unavailable: %w", st.Proxy.readErr)
	}
	ports := [2]int{1080, st.Proxy.Port}
	count := 1
	if st.Proxy.Port > 0 && st.Proxy.Port != 1080 {
		count = 2
	}
	probeParent := parent
	if !a.observationParentCurrent(parent) {
		probeParent = nil
	}
	st.Loopbacks = core.ObserveLoopback(probeParent, ports[:count]...)

	// 3. Only a bounded, read-only log fragment. Its failure does not discard
	// independent scheduler/proxy observations or authorize any repair.
	lines, observation := readNetguardLog()
	st.Log = observation
	st.Events = netguardEvents(lines, 12)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "RUN COMPLETE") || strings.Contains(lines[i], "skipping all heals") {
			st.LastRun = lines[i]
			break
		}
	}
	if parent != nil && !a.observationParentCurrent(parent) {
		for index := range st.Loopbacks {
			switch st.Loopbacks[index].Outcome {
			case core.LoopbackReachable, core.LoopbackRefused, core.LoopbackTimeout:
				st.Loopbacks[index].Outcome = core.LoopbackCanceled
			}
		}
	}
	for _, fact := range st.Loopbacks {
		if fact.Outcome != core.LoopbackReachable {
			continue
		}
		if fact.Port == 1080 {
			st.SocksAlive = true
		}
		if fact.Port == st.Proxy.Port {
			st.Proxy.ListenerAlive = true
		}
	}
	return st, nil
}

// RunProxyHeal retains the old wire API. Without a persistent verified original
// receipt it returns unavailable and cannot infer ownership from a local port.
func (a *App) RunProxyHeal() (bool, error) {
	healed, err := core.HealStaleSystemProxy()
	if err != nil {
		a.appendLog("warn", "proxy heal (ручной): "+err.Error())
		return false, err
	}
	if healed {
		a.appendLog("info", "proxy heal (ручной): зависший системный прокси отключён")
	} else {
		a.appendLog("info", "proxy heal (ручной): восстановление не выполнялось")
	}
	return healed, nil
}

// netguardLogLines — декодирование байтов лога в строки. Писец — PowerShell
// Add-Content без -Encoding: Windows PowerShell 5.1 пишет ANSI-кодировкой
// системы (cp1251 в ru-системах), а не UTF-8 → русская кириллица в сообщениях
// HEALING/PROBLEM приходила «бесовщиной» (������������ �⨯). Формат честно
// определяется по факту: чистый UTF-8 — читаем как есть; иначе валидная
// UTF-16LE (какой был бы лог, созданный на UTF-16-системе) — как UTF-16LE;
// иначе cp1251. Старые и новые строки одного файла могут различаться — решает
// каждая строка. ASCII (даты, маркеры HEALED/PROBLEM) не затрагивается.
func netguardLogLines(data []byte) []string {
	// UTF-16LE BOM или характерный чередующийся NUL — вся строка в UTF-16LE.
	hasBOM16 := len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE
	if netguardUTF16LE(data) {
		if hasBOM16 {
			data = data[2:]
		}
		u16 := make([]uint16, 0, len(data)/2)
		for i := 0; i+1 < len(data); i += 2 {
			u16 = append(u16, uint16(data[i])|uint16(data[i+1])<<8)
		}
		data = []byte(string(utf16.Decode(u16)))
	}
	// UTF-8 BOM (PowerShell -Encoding UTF8 пишет с BOM) — снять.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	raw := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if !utf8.ValidString(line) {
			if dec, err := charmap.Windows1251.NewDecoder().String(line); err == nil {
				line = dec
			}
		}
		out = append(out, line)
	}
	return out
}

// Shared legacy encoding heuristic; a bounded prefix can select code-unit
// alignment before the recent record window is decoded. Not a format proof.
func netguardUTF16LE(data []byte) bool {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return true
	}
	if len(data) == 0 || len(data)%2 != 0 {
		return false
	}
	for i := 1; i < len(data) && i < 256; i += 2 {
		if data[i] != 0 {
			return false
		}
	}
	return true
}

// ---------- задачи планировщика (COM, locale-independent) --------------------

// netguardAllTasks — факты по списку задач: одна строка-факт на задачу.
func netguardAllTasks(names []string) []NetGuardTask {
	return collectNetguardTasks(names)
}

// netguardTaskState — машинный статус из COM-значения State.
// TASK_STATE: 0 Unknown, 1 Disabled, 2 Queued, 3 Ready, 4 Running.
func netguardTaskState(state int32) string {
	switch state {
	case 0:
		return "unknown"
	case 1:
		return "disabled"
	case 2:
		return "queued"
	case 3:
		return "ready"
	case 4:
		return "running"
	default:
		return fmt.Sprintf("unknown:%d", state)
	}
}

// HRESULT-коды задач планировщика, важные для честного статуса.
const (
	hrTaskNotFound   = uint32(0x80070002) // ERROR_FILE_NOT_FOUND — задачи нет
	hrTaskAccessDeny = uint32(0x80070005) // E_ACCESSDENIED — есть, но скрыта правами (SYSTEM)
	hrDispException  = uint32(0x80020009) // DISP_E_EXCEPTION — обёртка EXCEPINFO
)

// netguardScode — точный HRESULT из ошибки Invoke. GetTask падает через
// DISP_E_EXCEPTION с EXCEPINFO; у go-ole scode живёт в строковом виде
// (EXCEPINFO.String() → "..., scode: 0x80070005").
func netguardScode(err error) (uint32, bool) {
	oe, ok := err.(*ole.OleError)
	if !ok {
		return 0, false
	}
	if code := oe.Code(); code != uintptr(hrDispException) {
		return uint32(code), true // не-exception HRESULT — сам по себе ответ
	}
	se, ok := oe.SubError().(ole.EXCEPINFO)
	if !ok {
		return 0, false
	}
	s := se.String()
	const marker = "scode: 0x"
	i := strings.Index(s, marker)
	if i < 0 {
		return 0, false
	}
	hex := s[i+len(marker):]
	if j := strings.IndexAny(hex, ",) "); j >= 0 {
		hex = hex[:j]
	}
	var v uint64
	if _, err := fmt.Sscanf(hex, "%x", &v); err != nil || v == 0 {
		// scode 0x0 = поле не заполнено (не ошибка) — не считаем фактом.
		return 0, false
	}
	return uint32(v), true
}

// netguardTaskInfo — одна задача через COM; каждая ошибка — честный факт.
func netguardTaskInfo(name string) NetGuardTask {
	t, err := comGetTask(name)
	return netguardTaskOutcome(name, t, err)
}

func netguardTaskOutcome(name string, t *NetGuardTask, err error) NetGuardTask {
	if err != nil {
		if sc, ok := netguardScode(err); ok {
			switch sc {
			case hrTaskNotFound:
				return NetGuardTask{Name: name, Status: "not-found"}
			case hrTaskAccessDeny:
				return NetGuardTask{Name: name, Status: "no-access"}
			}
		}
		if code := errCodeOf(err); code != 0 {
			return NetGuardTask{Name: name, Status: fmt.Sprintf("unknown:0x%08x", code)}
		}
		return NetGuardTask{Name: name, Status: "unknown"}
	}
	if t == nil {
		return NetGuardTask{Name: name, Status: "unknown"}
	}
	return *t
}

func errCodeOf(err error) uint32 {
	if oe, ok := err.(*ole.OleError); ok {
		return uint32(oe.Code())
	}
	return 0
}

func atoi32(s string) (int32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	var n int64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int64(s[i]-'0')
		if n > 1<<31-1 {
			return 0, false
		}
	}
	return int32(n), true
}

// ---------- прокси и лог ------------------------------------------------------

// Port reachability is separate from ownership/protection. Stale cannot be
// asserted without a persistent original receipt; registry failures propagate.
func netguardProxyState() NetGuardProxy {
	p := NetGuardProxy{}
	snapshot, err := core.ReadSystemProxySnapshot()
	if err != nil {
		p.readErr = err
		return p
	}
	p.Enabled, p.Server = snapshot.ProxyEnable != 0, snapshot.ProxyServer
	if port, ok := core.LoopbackProxyPort(snapshot.ProxyServer); ok {
		p.Port = port
	}
	// TCP observations are collected once in NetGuardStatus with its original
	// App lifetime. Registry decoding alone must not launch a second dial.
	return p
}

// LoopbackAlive — есть ли TCP-листенер на 127.0.0.1:<port>. Тонкая обёртка
// над ядром: только наблюдение TCP, без права reset или вывода VPN healthy.
func LoopbackAlive(port int) bool {
	return core.LoopbackAlive(port)
}

// netguardEvents — последние <limit> существенных событий из лога.
func netguardEvents(lines []string, limit int) []NetGuardEvent {
	kinds := []string{"HEALED", "HEALING", "PROBLEM", "WEEKLY"}
	out := []NetGuardEvent{}
	for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		kind := ""
		for _, k := range kinds {
			if strings.Contains(line, k+":") {
				kind = k
				break
			}
		}
		if kind == "" {
			continue
		}
		ev := NetGuardEvent{Kind: kind, Text: line}
		if len(line) >= 19 {
			ev.Time = line[:19]
			ev.Text = strings.TrimSpace(line[19:])
		}
		out = append(out, ev)
	}
	// разворот в хронологический порядок
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
