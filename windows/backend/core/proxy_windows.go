//go:build windows

package core

import (
	"fmt"
	"sync"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxyEnableValue    = `ProxyEnable`
	proxyServerValue    = `ProxyServer`
	autoConfigURLValue  = `AutoConfigURL`
)

// ProxySnapshot — предыдущее состояние системного прокси для восстановления.
type ProxySnapshot struct {
	ProxyEnable   uint32
	ProxyServer   string
	AutoConfigURL string
}

// ProxyManager — жизненный цикл системного прокси Windows: snapshot → set →
// restore. Restore обязателен на любом пути выхода из Start (fail-closed
// распространяется и на системные настройки).
type ProxyManager struct {
	mu          sync.Mutex
	snapshot    ProxySnapshot
	originalSet bool
}

// Snapshot фиксирует текущее состояние. Обязателен перед SetProxy.
func (pm *ProxyManager) Snapshot() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.READ)
	if err != nil {
		return fmt.Errorf("open internet settings: %w", err)
	}
	defer key.Close()

	pm.snapshot = ProxySnapshot{}
	if enable, _, err := key.GetIntegerValue(proxyEnableValue); err == nil {
		pm.snapshot.ProxyEnable = uint32(enable)
	}
	if server, _, err := key.GetStringValue(proxyServerValue); err == nil {
		pm.snapshot.ProxyServer = server
	}
	if acURL, _, err := key.GetStringValue(autoConfigURLValue); err == nil {
		pm.snapshot.AutoConfigURL = acURL
	}
	pm.originalSet = true
	return nil
}

// SetProxy направляет системный SOCKS-прокси на туннель.
func (pm *ProxyManager) SetProxy(socksAddr string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if !pm.originalSet {
		return fmt.Errorf("cannot set proxy without prior snapshot")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings for write: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(proxyServerValue, fmt.Sprintf("socks=%s", socksAddr)); err != nil {
		return fmt.Errorf("set proxy server: %w", err)
	}
	if err := key.SetDWordValue(proxyEnableValue, 1); err != nil {
		return fmt.Errorf("set proxy enable: %w", err)
	}
	_ = key.DeleteValue(autoConfigURLValue) // PAC не должен перебивать SOCKS

	notifySystemProxyChange()
	return nil
}

// Restore возвращает исходное состояние прокси.
func (pm *ProxyManager) Restore() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if !pm.originalSet {
		return nil
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings for restore: %w", err)
	}
	defer key.Close()

	if err := key.SetDWordValue(proxyEnableValue, pm.snapshot.ProxyEnable); err != nil {
		return fmt.Errorf("restore proxy enable: %w", err)
	}
	if pm.snapshot.ProxyServer != "" {
		if err := key.SetStringValue(proxyServerValue, pm.snapshot.ProxyServer); err != nil {
			return fmt.Errorf("restore proxy server: %w", err)
		}
	} else {
		_ = key.DeleteValue(proxyServerValue)
	}
	if pm.snapshot.AutoConfigURL != "" {
		if err := key.SetStringValue(autoConfigURLValue, pm.snapshot.AutoConfigURL); err != nil {
			return fmt.Errorf("restore autoconfig url: %w", err)
		}
	} else {
		_ = key.DeleteValue(autoConfigURLValue)
	}

	notifySystemProxyChange()
	pm.originalSet = false
	return nil
}

// notifySystemProxyChange — InternetSetOptionW(SETTINGS_CHANGED + REFRESH).
func notifySystemProxyChange() {
	dll := syscall.NewLazyDLL("wininet.dll")
	proc := dll.NewProc("InternetSetOptionW")
	if proc.Find() != nil {
		return
	}
	const (
		internetOptionRefresh         = 37
		internetOptionSettingsChanged = 39
	)
	proc.Call(0, internetOptionSettingsChanged, 0, 0)
	proc.Call(0, internetOptionRefresh, 0, 0)
}
