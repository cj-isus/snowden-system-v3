//go:build windows

package core

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

var proxyWinINETOptions = windows.NewLazySystemDLL("wininet.dll").NewProc("InternetSetOptionW")

// One original lookup and two fixed calls. Normal API failure and unknown
// callback completion differ; success is not consumer refresh/traffic proof.
type proxyNotificationAttempt struct {
	state                     proxyRegistryState
	findStarted, findReturned bool
	callStarted, callReturned [2]bool
	callErr                   [2]error
	callOK                    [2]bool
	returned                  bool
	result                    error
}

func (attempt *proxyNotificationAttempt) run() (result error) {
	if attempt == nil || attempt.findStarted {
		return ErrProxyStateUncertain
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain // never expose callback panic payload
		}
		attempt.returned, attempt.result = returned, result
	}()
	attempt.findStarted = true
	err := proxyWinINETOptions.Find()
	attempt.findReturned = true
	if err != nil {
		returned = true
		return err // lookup returned, no InternetSetOption call was attempted
	}
	for index, option := range [2]uintptr{39, 37} { // SETTINGS_CHANGED, REFRESH
		attempt.callStarted[index] = true
		ok, _, callErr := proxyWinINETOptions.Call(0, option, 0, 0)
		attempt.callReturned[index] = true
		attempt.callOK[index] = ok != 0
		if ok == 0 {
			if callErr == nil || callErr == windows.ERROR_SUCCESS {
				callErr = errors.New("InternetSetOptionW returned FALSE")
			}
			attempt.callErr[index] = callErr
			result = errors.Join(result, fmt.Errorf("WinINET refresh option %d: %w", option, callErr))
		}
	}
	returned = true
	return result
}

// Existing writer and locks only. Retain before Find/Call; a completed normal
// error permits the existing restore sequence, an unknown attempt never does.
func (pm *ProxyManager) notifyLocked() error {
	if err := pm.readyLocked(); err != nil {
		return err
	}
	attempt := &proxyNotificationAttempt{state: pm.current}
	pm.notification = attempt
	return attempt.run()
}
