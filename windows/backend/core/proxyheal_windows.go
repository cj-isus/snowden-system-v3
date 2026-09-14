//go:build windows

package core

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/sys/windows/registry"
)

// HealStaleSystemProxy — catch-all очистка после аварийного завершения
// процесса (kill/crash/сон): Manager не успел вызвать Restore, и системный
// proxy остался направлен на мёртвый локальный SOCKS → все proxy-aware
// приложения висят (инцидент 2026-09-09: OutlineService teardown-ошибки,
// ProxyEnable=1 на 127.0.0.1:1080 без листенера).
//
// Правила (fail-safe, консервативно):
//   - чиним ТОЛЬКО loopback-прокси вида "socks=127.0.0.1:<port>" или
//     "127.0.0.1:<port>" (формат, который пишет ProxyManager.SetProxy);
//   - перед отключением проверяем TCP-соединением, что на порту НЕТ
//     листенера: живой туннель не трогаем никогда;
//   - PAC (AutoConfigURL) и внешние (не-loopback) прокси не трогаем.
//
// Вызывается на старте приложения (startup self-heal); возвращает true,
// если было исправление.
//
// Те же предикаты (LoopbackProxyPort, LoopbackAlive) используются UI-страницей
// NetGuard (windows/netguard.go) — единая точка истины про «наш» формат прокси
// и живость листенера.
func HealStaleSystemProxy() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("heal: open internet settings: %w", err)
	}
	enable, _, err := key.GetIntegerValue(proxyEnableValue)
	key.Close()
	if err != nil || enable != 1 {
		return false, nil // прокси не включён — чисто
	}

	key, err = registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("heal: open internet settings: %w", err)
	}
	server, _, err := key.GetStringValue(proxyServerValue)
	key.Close()
	if err != nil {
		return false, nil // включён, но адреса нет — не наш формат, не трогаем
	}

	port, ok := LoopbackProxyPort(server)
	if !ok {
		return false, nil // PAC/внешний/комбинированный — вне зоны ответственности
	}
	if LoopbackAlive(port) {
		return false, nil // листенер жив (VPN работает) — прокси валиден
	}

	if err := clearSystemProxy(); err != nil {
		return false, err
	}
	return true, nil
}

// LoopbackProxyPort разбирает "socks=127.0.0.1:<port>" / "127.0.0.1:<port>".
// Прочие форматы ("http=...;https=...", хосты не-loopback) — не наша запись.
func LoopbackProxyPort(server string) (int, bool) {
	re := regexp.MustCompile(`^(?:socks=)?127\.0\.0\.1:(\d+)$`)
	m := re.FindStringSubmatch(server)
	if m == nil {
		return 0, false
	}
	port, err := strconv.Atoi(m[1])
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

// LoopbackAlive — есть ли TCP-листенер на 127.0.0.1:<port>.
func LoopbackAlive(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// clearSystemProxy — ProxyEnable=0 + уведомление WinINET (тот же путь,
// что и Restore, но без снапшота: снапшот к мёртвому состоянию не нужен).
// clearSystemProxy — ProxyEnable=0 + уведомление WinINET (тот же путь,
// что и Restore, но без снапшота: снапшот к мёртвому состоянию не нужен).
func clearSystemProxy() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("heal: open internet settings for write: %w", err)
	}
	defer key.Close()
	if err := key.SetDWordValue(proxyEnableValue, 0); err != nil {
		return fmt.Errorf("heal: disable proxy: %w", err)
	}
	notifySystemProxyChange()
	return nil
}
