package main

// Native local route/adapter observation; SSID report from bounded system netsh.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"golang.org/x/sys/windows"
)

// Reuse the validated native route row; counters start no process or socket.
func collectAdapterFacts(ctx context.Context) (adapterSnapshot, error) {
	for _, procedure := range observerRouteProcedures {
		if !lifecyclePublicationCurrent(ctx) {
			return adapterSnapshot{}, errAdapterObservation
		}
		if err := procedure.Find(); err != nil {
			return adapterSnapshot{}, errAdapterObservation
		}
	}
	for _, family := range [...]uint16{windows.AF_INET, windows.AF_INET6} {
		var snapshot adapterSnapshot
		_, err := localRouteSnapshotWithRow(ctx, family, true, func(fp netFingerprint, row windows.MibIfRow2) {
			target := "9.9.9.9"
			if family == windows.AF_INET6 {
				target = "2620:fe::fe"
			}
			snapshot.address, snapshot.netClass = fp.ip, fp.netClass
			snapshot.stats = NetStats{InOctets: row.InOctets, OutOctets: row.OutOctets,
				SpeedBps: min(row.ReceiveLinkSpeed, row.TransmitLinkSpeed), Alias: fp.alias,
				SourceSchema: 1, Source: "native_route_adapter", AdapterID: fmt.Sprintf("%x:%d", fp.luid, fp.ifaceIndex), RouteTarget: target}
		})
		if err == nil && lifecyclePublicationCurrent(ctx) && snapshot.stats.AdapterID != "" {
			return snapshot, nil
		}
		if !errors.Is(err, errRouteFamilyLookup) || !lifecyclePublicationCurrent(ctx) {
			return adapterSnapshot{}, errAdapterObservation
		}
	}
	return adapterSnapshot{}, errAdapterObservation
}

// netshShowInterfaces — SSID подключённого Wi-Fi (факт ОС, никакого угадывания).
func netshShowInterfaces(parent context.Context, alias string) (string, error) {
	if !lifecyclePublicationCurrent(parent) {
		return "", errAdapterObservation
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	directory, err := windows.GetSystemDirectory()
	if err != nil || directory == "" {
		return "", errAdapterObservation
	}
	out, err := core.CaptureOptionalHelper(ctx, filepath.Join(directory, "netsh.exe"), []string{"wlan", "show", "interfaces"}, 16<<10)
	if err != nil || !lifecyclePublicationCurrent(ctx) {
		return "", errAdapterObservation
	}
	matched := false
	for _, line := range netguardLogLines(out) {
		t := strings.TrimSpace(line)
		if colon := strings.IndexByte(t, ':'); colon >= 0 {
			key, value := strings.TrimSpace(t[:colon]), strings.TrimSpace(t[colon+1:])
			if key == "Name" || key == "Имя" {
				matched = strings.EqualFold(value, alias)
				continue
			}
		}
		if matched && strings.HasPrefix(t, "SSID") && !strings.HasPrefix(t, "BSSID") {
			if idx := strings.Index(t, ":"); idx >= 0 {
				if v := strings.TrimSpace(t[idx+1:]); v != "" && len(v) <= 128 && !strings.ContainsAny(v, "\x00\r\n") {
					return v, nil
				}
			}
		}
	}
	return "", errAdapterObservation
}

// adapterNetClassByIndex — класс сети адаптера по индексу интерфейса:
// локальный системный вызов GetIfEntry2Ex (MIB_IF_ROW2: Type + MediaType),
// без PowerShell. Legacy index-only caller не получает route/source/LUID scope.
// Netwatch/transport preference используют класс из общего native route snapshot,
// без второго чтения. Классификация — общая точка netwatch.classifyNetClass.
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
	iface, err := net.InterfaceByName(alias)
	if err != nil {
		return ""
	}
	return adapterNetClassByIndex(iface.Index)
}
