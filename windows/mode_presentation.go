package main

import (
	"fmt"

	"github.com/snowden-system/windows/backend/core"
)

// Facts about this process's immutable mode flag and compiled negative gates.
// None of these fields proves an adapter, proxy write, child or protected path.
type ProcessModeFactsView struct {
	Schema             int    `json:"schema"`
	PID                int    `json:"pid"`
	Mode               string `json:"mode"`
	TUNAdmission       bool   `json:"tunAdmission"`
	ElevationAdmission bool   `json:"elevationAdmission"`
}

func processModeFacts(pid int) *ProcessModeFactsView {
	mode := "socks"
	if tunRequested() {
		mode = "tun"
	}
	return &ProcessModeFactsView{Schema: 1, PID: pid, Mode: mode,
		TUNAdmission: core.NativeTUNForwardingAvailable(), ElevationAdmission: core.ElevatedGUIAvailable()}
}

// Same early compiled rejection for legacy and ordered requests. No Stop,
// UAC, helper preparation or native process launch occurs in this predicate.
func modeRequestRejection(enable bool) error {
	if enable && !core.NativeTUNForwardingAvailable() {
		return fmt.Errorf("TUN пока недоступен: системный допуск трафика не принят")
	}
	if enable && !core.ElevatedGUIAvailable() {
		return fmt.Errorf("перезапуск с правами администратора пока недоступен: системная передача ещё не принята")
	}
	return nil
}
