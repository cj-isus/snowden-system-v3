package main

// Local route observation only. The structures and wrappers below are from
// the pinned x/sys; no connection, socket, helper process or OS setting change.

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procObserverGetBestRoute2 = modIPHlpAPI.NewProc("GetBestRoute2")
	observerRouteProcedures   = [...]*windows.LazyProc{
		modIPHlpAPI.NewProc("GetBestInterfaceEx"), procObserverGetBestRoute2,
		modIPHlpAPI.NewProc("GetIfEntry2Ex"), modIPHlpAPI.NewProc("GetUnicastIpAddressEntry"),
	}
	errRouteFamilyLookup = errors.New("local route family lookup unavailable")
	errRouteSnapshot     = errors.New("local route snapshot unavailable")
)

func netFingerprintOS(parent context.Context) netFingerprint {
	if !lifecyclePublicationCurrent(parent) {
		return netFingerprint{}
	}
	ctx, cancel := context.WithTimeout(parent, 700*time.Millisecond)
	defer cancel()
	ifaces, err := net.Interfaces() // AF_UNSPEC on Windows; no sockets
	if err != nil || !lifecyclePublicationCurrent(ctx) || len(ifaces) == 0 || len(ifaces) > 256 {
		return netFingerprint{}
	}
	links, up := 0, false
	for _, iface := range ifaces {
		if !lifecyclePublicationCurrent(ctx) || iface.Index <= 0 {
			return netFingerprint{}
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Include virtual, IPv6-only and our TUN for negative evidence.
		links++
		up = up || iface.Flags&net.FlagUp != 0
	}
	if !lifecyclePublicationCurrent(ctx) {
		return netFingerprint{}
	}
	if links > 0 && !up {
		return netFingerprint{allLinksDown: true}
	}
	if !up {
		return netFingerprint{}
	}
	for _, procedure := range observerRouteProcedures {
		if !lifecyclePublicationCurrent(ctx) {
			return netFingerprint{}
		}
		if err := procedure.Find(); err != nil || !lifecyclePublicationCurrent(ctx) {
			return netFingerprint{} // missing API is a sample failure, not an SDK Addr panic
		}
	}
	for _, family := range [...]uint16{windows.AF_INET, windows.AF_INET6} {
		fp, err := localRouteSnapshot(ctx, family)
		if err == nil && lifecyclePublicationCurrent(ctx) {
			return fp
		}
		if !errors.Is(err, errRouteFamilyLookup) || !lifecyclePublicationCurrent(ctx) {
			return netFingerprint{} // malformed/forbidden/stale is not another lane
		}
	}
	return netFingerprint{}
}

// GetBestInterfaceEx first provides a nonzero interface, as required by
// GetBestRoute2. Re-reading it at the end detects a changed best-interface
// selection; these local calls still do not form an atomic OS snapshot.
func localRouteSnapshot(ctx context.Context, family uint16) (netFingerprint, error) {
	return localRouteSnapshotWithRow(ctx, family, false, nil)
}

// Read-only adapter telemetry may include a virtual route. The existing
// physical-path observer keeps its original exclusions through the wrapper.
// capture receives the same validated row, never a second unrelated MIB read.
func localRouteSnapshotWithRow(ctx context.Context, family uint16, includeVirtual bool, capture func(netFingerprint, windows.MibIfRow2)) (netFingerprint, error) {
	var destination windows.RawSockaddrInet
	var lookup windows.Sockaddr
	switch family {
	case windows.AF_INET:
		sa := &windows.SockaddrInet4{Addr: [4]byte{9, 9, 9, 9}}
		lookup = sa
		raw := (*windows.RawSockaddrInet4)(unsafe.Pointer(&destination))
		raw.Family, raw.Addr = family, sa.Addr
	case windows.AF_INET6:
		sa := &windows.SockaddrInet6{Addr: [16]byte{0x26, 0x20, 0, 0xfe, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xfe}}
		lookup = sa
		raw := (*windows.RawSockaddrInet6)(unsafe.Pointer(&destination))
		raw.Family, raw.Addr = family, sa.Addr
	default:
		return netFingerprint{}, errRouteSnapshot
	}
	if !lifecyclePublicationCurrent(ctx) {
		return netFingerprint{}, errRouteSnapshot
	}
	var index uint32
	lookupErr := windows.GetBestInterfaceEx(lookup, &index)
	if !lifecyclePublicationCurrent(ctx) {
		return netFingerprint{}, errRouteSnapshot
	}
	if lookupErr != nil {
		return netFingerprint{}, errRouteFamilyLookup
	}
	if index == 0 {
		return netFingerprint{}, errRouteSnapshot
	}
	if err := procObserverGetBestRoute2.Find(); err != nil || !lifecyclePublicationCurrent(ctx) {
		return netFingerprint{}, errRouteSnapshot
	}
	var route windows.MibIpForwardRow2
	var source windows.RawSockaddrInet
	status, _, _ := procObserverGetBestRoute2.Call(
		0, uintptr(index), 0, uintptr(unsafe.Pointer(&destination)), 0,
		uintptr(unsafe.Pointer(&route)), uintptr(unsafe.Pointer(&source)))
	runtime.KeepAlive(&destination)
	runtime.KeepAlive(&route)
	runtime.KeepAlive(&source)
	if !lifecyclePublicationCurrent(ctx) {
		return netFingerprint{}, errRouteSnapshot
	}
	if uint32(status) != 0 { // NETIOAPI return status, not stale thread LastError
		return netFingerprint{}, errRouteFamilyLookup
	}
	if route.InterfaceIndex != index || route.InterfaceLuid == 0 || route.Loopback != 0 || source.Family != family {
		return netFingerprint{}, errRouteSnapshot
	}
	address := localRouteAddress(&source)
	if !address.IsGlobalUnicast() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.Is4In6() {
		return netFingerprint{}, errRouteSnapshot
	}
	adapter := windows.MibIfRow2{InterfaceLuid: route.InterfaceLuid}
	adapterErr := windows.GetIfEntry2Ex(windows.MibIfTableNormal, &adapter)
	if adapterErr != nil || !lifecyclePublicationCurrent(ctx) || adapter.InterfaceLuid != route.InterfaceLuid ||
		adapter.InterfaceIndex != index || adapter.OperStatus != windows.IfOperStatusUp {
		return netFingerprint{}, errRouteSnapshot
	}
	switch adapter.Type {
	case windows.IF_TYPE_SOFTWARE_LOOPBACK:
		return netFingerprint{}, errRouteSnapshot
	case windows.IF_TYPE_PPP, windows.IF_TYPE_TUNNEL:
		if !includeVirtual {
			return netFingerprint{}, errRouteSnapshot
		}
	}
	alias := windows.UTF16ToString(adapter.Alias[:])
	if alias == "" || !includeVirtual && strings.EqualFold(alias, tunAdapterName) {
		return netFingerprint{}, errRouteSnapshot
	}
	// Confirm this exact source is still a usable unicast of the same LUID.
	unicast := windows.MibUnicastIpAddressRow{
		Address:       *(*windows.RawSockaddrInet6)(unsafe.Pointer(&source)),
		InterfaceLuid: route.InterfaceLuid, InterfaceIndex: index,
	}
	unicastErr := windows.GetUnicastIpAddressEntry(&unicast)
	if unicastErr != nil || !lifecyclePublicationCurrent(ctx) || unicast.InterfaceLuid != route.InterfaceLuid ||
		unicast.InterfaceIndex != index || unicast.DadState != windows.IpDadStatePreferred || unicast.SkipAsSource != 0 || unicast.ValidLifetime == 0 ||
		unicast.Address.Family != family || localRouteAddress((*windows.RawSockaddrInet)(unsafe.Pointer(&unicast.Address))) != address {
		return netFingerprint{}, errRouteSnapshot
	}
	var finalIndex uint32
	finalErr := windows.GetBestInterfaceEx(lookup, &finalIndex)
	if finalErr != nil || !lifecyclePublicationCurrent(ctx) || finalIndex != index || int(index) <= 0 {
		return netFingerprint{}, errRouteSnapshot
	}
	fingerprint := netFingerprint{ifaceIndex: int(index), ip: address.String(), alias: alias,
		luid: route.InterfaceLuid, netClass: classifyNetClass(adapter.Type, adapter.MediaType)}
	if capture != nil {
		capture(fingerprint, adapter)
	}
	return fingerprint, nil
}

func localRouteAddress(source *windows.RawSockaddrInet) netip.Addr {
	switch source.Family {
	case windows.AF_INET:
		return netip.AddrFrom4((*windows.RawSockaddrInet4)(unsafe.Pointer(source)).Addr)
	case windows.AF_INET6:
		return netip.AddrFrom16((*windows.RawSockaddrInet6)(unsafe.Pointer(source)).Addr)
	default:
		return netip.Addr{}
	}
}
