package main

// infobindings_windows.go — сборщик сетевых фактов Windows: адаптер маршрута
// по умолчанию (PowerShell Get-NetRoute/Get-NetIPAddress/Get-NetAdapterStatistics),
// SSID через netsh. Всё — факты ОС; вне Windows файл не собирается.

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// hiddenExec — запуск консольной утилиты без мигания окна (GUI-процесс).
func hiddenExec(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Output()
}

// defaultAdapterFacts — адаптер маршрута по умолчанию: IPv4, имя, физический
// тип носителя, скорость и счётчики трафика. Все значения — факты ОС.
func defaultAdapterFacts() (ip, alias, media string, speedBps, inOctets, outOctets uint64, err error) {
	const script = `
$r = Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction Stop |
  Sort-Object RouteMetric, InterfaceMetric | Select-Object -First 1
if (-not $r) { Write-Output 'NO_DEFAULT_ROUTE'; exit 0 }
$ipi = Get-NetIPAddress -InterfaceIndex $r.InterfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue |
  Select-Object -First 1
$a = Get-NetAdapter -InterfaceIndex $r.InterfaceIndex -ErrorAction SilentlyContinue
$s = Get-NetAdapterStatistics -Name $a.Name -ErrorAction SilentlyContinue
$media = ''
if ($a.MediaType -eq 'Native802.11') { $media = 'Native802.11' }
elseif ($a.MediaType -eq '802.3') { $media = '802.3' }
Write-Output ($ipi.IPAddress)
Write-Output ($a.Name)
Write-Output ($media)
Write-Output ([string]([uint64]$a.Speed))
Write-Output ([string]([uint64]$s.ReceivedBytes))
Write-Output ([string]([uint64]$s.SentBytes))
`
	out, err := hiddenExec("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		return "", "", "", 0, 0, 0, fmt.Errorf("powershell: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	// Парсим построчно; недостающие значения остаются нулями (честное «нет данных»).
	get := func(i int) string {
		if i < len(lines) {
			return strings.TrimSpace(lines[i])
		}
		return ""
	}
	if get(0) == "NO_DEFAULT_ROUTE" {
		return "", "", "", 0, 0, 0, fmt.Errorf("маршрут по умолчанию не найден")
	}
	parseU64 := func(s string) uint64 {
		v, _ := strconv.ParseUint(s, 10, 64)
		return v
	}
	return get(0), get(1), get(2), parseU64(get(3)), parseU64(get(4)), parseU64(get(5)), nil
}

// netshShowInterfaces — SSID подключённого Wi-Fi (факт ОС, никакого угадывания).
func netshShowInterfaces() (string, error) {
	out, err := hiddenExec("netsh", "wlan", "show", "interfaces")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "SSID") && !strings.HasPrefix(t, "BSSID") {
			if idx := strings.Index(t, ":"); idx >= 0 {
				if v := strings.TrimSpace(t[idx+1:]); v != "" {
					return v, nil
				}
			}
		}
	}
	return "", fmt.Errorf("SSID не найден в выводе netsh")
}

// adapterNetClassByIndex — класс сети адаптера по индексу интерфейса:
// мгновенный системный вызов GetIfEntry2Ex (MIB_IF_ROW2: Type + MediaType),
// без PowerShell. Живёт на цикле опроса netwatch (каждый переход) и на пути
// startCore (transportpref) — процесс-спавн здесь = секунды на поллинг.
// Классификация — общая точка netwatch.classifyNetClass.
func adapterNetClassByIndex(idx int) string {
	if idx <= 0 {
		return ""
	}
	row := &windows.MibIfRow2{}
	row.InterfaceIndex = uint32(idx)
	if err := windows.GetIfEntry2Ex(windows.MibIfTableNormal, row); err != nil {
		return "" // факт недоступен — класс неизвестен, не «wifi по умолчанию»
	}
	return classifyNetClass(row.Type, row.MediaType)
}

// adapterNetClass — класс сети по имени адаптера (общая точка с Windows-
// сборщиком фактов GetInfo; оставлена для имени — классификация та же).
func adapterNetClass(alias string) string {
	if alias == "" {
		return ""
	}
	const script = `
$a = Get-NetAdapter | Where-Object { $_.Name -eq $args[0] } | Select-Object -First 1
if (-not $a) { Write-Output 'NO_ADAPTER'; exit 0 }
Write-Output ($a.MediaType)
Write-Output ($a.InterfaceType)
`
	out, err := hiddenExec("powershell", "-NoProfile", "-NonInteractive", "-Command", script, alias)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		switch t {
		case "", "NO_ADAPTER":
			continue
		case "Native802.11":
			return "wifi"
		case "802.3":
			return "ethernet"
		case "Wireless WAN":
			return "mobile"
		}
	}
	return ""
}

// compile-time guard: файл только для Windows.
var _ = os.Getpid
