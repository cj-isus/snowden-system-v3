//go:build !with_clash_api

package main

// metrics_noclash.go — dev-сборка без with_clash_api: блок experimental.clash_api
// не рендерится, GetMetrics честно сообщает о недоступности.

// clashAPIBuilt — ядро собрано без clash-api.
const clashAPIBuilt = false
