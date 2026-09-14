package main

// netguard.go — фактическое состояние NetGuard (tools/netguard) для UI-страницы
// и ручной запуск stale-proxy heal из UI. Только чтение источников и честные
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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows/registry"

	"github.com/snowden-system/windows/backend/core"
)

// NetGuardTask — состояние одной плановой задачи auto-heal.
type NetGuardTask struct {
	Name     string `json:"name"`
	Status   string `json:"status"`   // машинный: ready/running/disabled/queued/hidden/no-access/not-found/unknown:<n>
	LastRun  string `json:"lastRun"`  // RFC3339, пусто = не запускалась
	NextRun  string `json:"nextRun"`  // RFC3339, пусто = расписания нет
	LastCode string `json:"lastCode"` // код возврата последнего запуска (десятичный)
}

// NetGuardProxy — системный прокси HKCU и живость его loopback-листенера.
type NetGuardProxy struct {
	Enabled       bool   `json:"enabled"`
	Server        string `json:"server"`
	Port          int    `json:"port"`          // 0 = не loopback-формат
	ListenerAlive bool   `json:"listenerAlive"` // актуально только при Port>0
	Stale         bool   `json:"stale"`         // включён + наш формат + листенер мёртв
}

// NetGuardEvent — событие из лога auto-heal.
type NetGuardEvent struct {
	Time string `json:"time"`
	Kind string `json:"kind"` // HEALED | HEALING | PROBLEM | WEEKLY
	Text string `json:"text"`
}

// NetGuardStatus — всё, что показывает страница NetGuard.
type NetGuardStatus struct {
	Tasks       []NetGuardTask  `json:"tasks"`
	Proxy       NetGuardProxy   `json:"proxy"`
	SocksAlive  bool            `json:"socksAlive"` // листенер VPN на 1080
	Events      []NetGuardEvent `json:"events"`
	LastRun     string          `json:"lastRun"`
	DoHDisabled bool            `json:"dohDisabled"` // true = лога нет (netguard не установлен)
}

// netguardTaskNames — плановые задачи auto-heal в порядке отображения.
var netguardTaskNames = []string{"NetGuard-AutoHeal", "NetGuard-AutoHeal-User"}

// NetGuardStatus — биндинг для UI (Wails).
func (a *App) NetGuardStatus() (*NetGuardStatus, error) {
	st := &NetGuardStatus{Tasks: []NetGuardTask{}, Events: []NetGuardEvent{}}

	// 1. Плановые задачи (COM API; каждая ошибка — отдельный честный факт).
	st.Tasks = netguardAllTasks(netguardTaskNames)

	// 2. Системный прокси (HKCU) + живость SOCKS:1080.
	st.Proxy = netguardProxyState()
	st.SocksAlive = LoopbackAlive(1080)

	// 3. Лог и события.
	logFile := filepath.Join(os.Getenv("ProgramData"), "netguard", "netguard.log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		st.DoHDisabled = true
		return st, nil // лога нет — это факт, не ошибка
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	st.Events = netguardEvents(lines, 12)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "RUN COMPLETE") || strings.Contains(lines[i], "skipping all heals") {
			st.LastRun = lines[i]
			break
		}
	}
	return st, nil
}

// RunProxyHeal — ручной heal зависшего системного прокси из UI (кнопка
// «Починить сейчас»). Тот же код, что startup self-heal
// (core.HealStaleSystemProxy), поэтому правила консервативны: только наш
// loopback-формат при мёртвом листенере; PAC/внешние/живые прокси не трогаются.
func (a *App) RunProxyHeal() (bool, error) {
	healed, err := core.HealStaleSystemProxy()
	if err != nil {
		a.appendLog("warn", "proxy heal (ручной): "+err.Error())
		return false, err
	}
	if healed {
		a.appendLog("info", "proxy heal (ручной): зависший системный прокси отключён")
	} else {
		a.appendLog("info", "proxy heal (ручной): чинить нечего (прокси валиден или не наш формат)")
	}
	return healed, nil
}

// ---------- задачи планировщика (COM, locale-independent) --------------------

// netguardAllTasks — факты по списку задач: одна строка-факт на задачу.
func netguardAllTasks(names []string) []NetGuardTask {
	out := make([]NetGuardTask, 0, len(names))
	for _, name := range names {
		out = append(out, netguardTaskInfo(name))
	}
	return out
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
	notFound := NetGuardTask{Name: name, Status: "not-found"}
	t, err := comGetTask(name)
	if err != nil {
		if sc, ok := netguardScode(err); ok {
			switch sc {
			case hrTaskNotFound:
				return notFound
			case hrTaskAccessDeny:
				return NetGuardTask{Name: name, Status: "no-access"}
			}
		}
		return NetGuardTask{Name: name, Status: fmt.Sprintf("unknown:0x%08x", errCodeOf(err))}
	}
	return *t
}

func errCodeOf(err error) uint32 {
	if oe, ok := err.(*ole.OleError); ok {
		return uint32(oe.Code())
	}
	return 0
}

// comGetTask — GetTask + чтение фактических полей задачи.
func comGetTask(name string) (*NetGuardTask, error) {
	_ = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()

	svc, err := oleutil.CreateObject("Schedule.Service")
	if err != nil {
		return nil, fmt.Errorf("schedule.service: %w", err)
	}
	defer svc.Release()
	disp, err := svc.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("schedule.service QI: %w", err)
	}
	defer disp.Release()
	if _, err := oleutil.CallMethod(disp, "Connect"); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	folderV, err := oleutil.CallMethod(disp, "GetFolder", `\`)
	if err != nil {
		return nil, fmt.Errorf("root folder: %w", err)
	}
	folder := folderV.ToIDispatch()
	defer folder.Release()

	tv, err := oleutil.CallMethod(folder, "GetTask", name)
	if err != nil {
		return nil, err // scode-факт обрабатывает вызывающий
	}
	// ToIDispatch отдаёт тот же указатель без AddRef; освобождаем РОВНО один
	// раз через t.Release(). tv.Clear() здесь нельзя: для VT_DISPATCH он тоже
	// вызовет Release → двойное освобождение → crash (поймано live-тестом).
	t := tv.ToIDispatch()
	defer t.Release()

	var lastErr error
	get := func(prop string) (string, bool) {
		v, err := oleutil.GetProperty(t, prop)
		if err != nil {
			lastErr = err
			return "", false
		}
		val := v.Value()
		v.Clear() // Value() уже скопировал данные; Clear освобождает BSTR/дату
		switch x := val.(type) {
		case string:
			return x, true
		case time.Time:
			return x.Format(time.RFC3339), true
		case float64:
			return fmt.Sprintf("%d", int64(x)), true
		case int64:
			return fmt.Sprintf("%d", x), true
		case int32:
			return fmt.Sprintf("%d", x), true
		case bool:
			return fmt.Sprintf("%v", x), true
		default:
			return fmt.Sprintf("%v", val), true
		}
	}

	stateRaw, okS := get("State")
	resRaw, okR := get("LastTaskResult")
	lastRun, okL := get("LastRunTime")
	nextRun, okN := get("NextRunTime")
	if lastErr != nil || !okS || !okR || !okL || !okN {
		if lastErr == nil {
			lastErr = fmt.Errorf("поля задачи недоступны")
		}
		return nil, lastErr
	}

	st, _ := atoi32(stateRaw) // State: маленькое неотрицательное число
	out := &NetGuardTask{
		Name:     name,
		Status:   netguardTaskState(st),
		LastRun:  lastRun,
		NextRun:  nextRun,
		LastCode: resRaw,
	}
	return out, nil
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

// netguardProxyState — ProxyEnable/ProxyServer из HKCU + живость листенера.
// Парсер формата и живость порта — общие с ядром (proxyheal_windows.go):
// core.LoopbackProxyPort / core.LoopbackAlive.
func netguardProxyState() NetGuardProxy {
	p := NetGuardProxy{}
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return p
	}
	defer k.Close()
	if enable, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		p.Enabled = enable == 1
	}
	if server, _, err := k.GetStringValue("ProxyServer"); err == nil {
		p.Server = server
		if port, ok := core.LoopbackProxyPort(server); ok {
			p.Port = port
		}
	}
	if p.Port > 0 {
		p.ListenerAlive = core.LoopbackAlive(p.Port)
		p.Stale = p.Enabled && !p.ListenerAlive
	}
	return p
}

// LoopbackAlive — есть ли TCP-листенер на 127.0.0.1:<port>. Тонкая обёртка
// над ядром (та же проверка, что в HealStaleSystemProxy — единый факт).
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
