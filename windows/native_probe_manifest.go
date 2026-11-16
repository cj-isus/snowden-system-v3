package main

import "github.com/snowden-system/windows/backend/netprobe"

// Set only by the packaging build from the separately built helper bytes.
// A default/dev Wails build has no execution pin; it must remain unavailable.
var nativeProbeHelperSHA256 string

type NativeProbeCapabilities struct {
	Schema         int    `json:"schema"`
	Protocol       int    `json:"protocol"`
	HelperName     string `json:"helper_name"`
	PinConfigured  bool   `json:"pin_configured"`
	ImageState     string `json:"image_state"`
	ExecutionState string `json:"execution_state"`
}

// Compiled capability only. It neither opens an owner file nor performs native
// I/O, and does not imply the pinned helper is present, validated or working.
func (a *App) GetNativeProbeCapabilities() NativeProbeCapabilities {
	return NativeProbeCapabilities{Schema: 1, Protocol: netprobe.NativeProtocol, HelperName: netprobe.NativeHelperName,
		PinConfigured: canonicalApprovalSHA256(nativeProbeHelperSHA256), ImageState: "not_checked", ExecutionState: "manual_action"}
}
