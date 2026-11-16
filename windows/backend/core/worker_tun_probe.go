package core

// Created only by the original worker under Engine.mu -> admission.mu after
// inspecting its actual phase. This value is not serialized or supplied by a
// request/report. A periodic observation does not reuse/refresh a native receipt.
type workerTUNProbeBinding struct {
	admission         *forwardingAdmission
	subject, selector string
	epoch             uint64
	native            *workerNativeTUNReceipt
	existing          bool
}

func (binding *workerTUNProbeBinding) result() ProbeResult {
	unavailable := nativeTUNReceiptResult(nil)
	if binding != nil && binding.existing {
		unavailable = ProbeResult{Name: "TUN admission", Status: "unavailable", Detail: "Действующий допуск исходного процесса и выбранного выхода не подтверждён. Прежнее native-наблюдение не обновлено."}
	}
	if binding == nil || binding.admission == nil || binding.epoch == 0 {
		return unavailable
	}
	gate := binding.admission
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.closed || gate.epoch != binding.epoch || gate.tunScope != nil {
		return unavailable
	}
	if binding.existing {
		if !gate.open.Load() || gate.subject != binding.subject || gate.selector != binding.selector || binding.native != nil {
			return unavailable
		}
		return ProbeResult{Name: "TUN admission", Passed: true, Status: "pass", Detail: "Допуск исходного процесса и выбранного выхода остаётся действующим. Native-путь повторно не измерялся; прежнее наблюдение не обновлено."}
	}
	if gate.open.Load() || binding.native == nil || !binding.native.current(binding.subject, binding.selector, binding.epoch, binding.native.nonce) {
		return unavailable
	}
	return nativeTUNReceiptResult(binding.native)
}
