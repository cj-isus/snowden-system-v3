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

// compile-time guard: файл только для Windows.
var _ = os.Getpid
