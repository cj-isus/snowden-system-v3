//go:build !windows

package main

func startNativePowerNotifications(*networkPollerOwner) (powerSubscription, error) {
	return nil, errPowerNotificationUnavailable // no synthetic native continuity
}
