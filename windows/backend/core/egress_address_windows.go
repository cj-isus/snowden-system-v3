//go:build windows

package core

import (
	"context"
	"net"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	maxEgressAddressRecords   = 32
	ownedEgressWinsockVersion = 0x202
)

var egressWinsockAPI = windows.NewLazySystemDLL("ws2_32.dll")
var egressAddressProcedures = [...]*windows.LazyProc{
	egressWinsockAPI.NewProc("WSAStartup"), egressWinsockAPI.NewProc("WSACleanup"),
	egressWinsockAPI.NewProc("GetAddrInfoW"), egressWinsockAPI.NewProc("FreeAddrInfoW"),
}

// The original egress task calls this API synchronously. In particular there is
// no net.Resolver goroutine to abandon after its caller's context expires. The
// OS call may exceed that requested deadline: retain this task until it returns.
// WSA ownership is exactly one successful reference, never a global reset.
type egressAddressQuery struct {
	called, returned                         bool
	startupEntered, startupReturned, started bool
	lookupEntered, lookupReturned            bool
	freeAttempted, freeReturned              bool
	cleanupAttempted, cleanupReturned        bool
	data                                     windows.WSAData
	records                                  *windows.AddrinfoW
}

func (query *egressAddressQuery) lookup(ctx context.Context, host string) (addresses []string, err error) {
	if query.called {
		return nil, ErrEgressObservation
	}
	query.called = true
	addresses, err = query.lookupBody(ctx, host)
	query.returned = true
	return addresses, err
}

func (query *egressAddressQuery) lookupBody(ctx context.Context, host string) ([]string, error) {
	if host != "ipapi.co" || contextError(ctx) != nil {
		return nil, ErrEgressObservation
	}
	for _, procedure := range egressAddressProcedures {
		if err := procedure.Find(); err != nil || contextError(ctx) != nil {
			return nil, ErrEgressObservation
		}
	}
	query.startupEntered = true
	err := windows.WSAStartup(ownedEgressWinsockVersion, &query.data)
	query.startupReturned, query.started = true, err == nil
	if err != nil || query.data.Version != ownedEgressWinsockVersion || contextError(ctx) != nil {
		return nil, ErrEgressObservation
	}
	name, err := windows.UTF16PtrFromString(host)
	if err != nil {
		return nil, ErrEgressObservation
	}
	service, err := windows.UTF16PtrFromString("443")
	if err != nil {
		return nil, ErrEgressObservation
	}
	hints := windows.AddrinfoW{Family: windows.AF_INET, Socktype: windows.SOCK_STREAM, Protocol: windows.IPPROTO_TCP}
	query.lookupEntered = true
	err = windows.GetAddrInfoW(name, service, &hints, &query.records)
	query.lookupReturned = true // includes the original native call, not cancellation of another frame
	runtime.KeepAlive(name)
	runtime.KeepAlive(service)
	runtime.KeepAlive(&hints)
	runtime.KeepAlive(query)
	if err != nil || query.records == nil || contextError(ctx) != nil {
		return nil, ErrEgressObservation
	}
	var seen [maxEgressAddressRecords]*windows.AddrinfoW
	addresses := make([]string, 0, maxEgressAddresses)
	count := 0
	for record := query.records; record != nil; record = record.Next {
		if count == len(seen) || contextError(ctx) != nil {
			return nil, ErrEgressObservation
		}
		for _, previous := range seen[:count] {
			if previous == record {
				return nil, ErrEgressObservation
			}
		}
		seen[count], count = record, count+1
		if record.Family != windows.AF_INET || record.Socktype != windows.SOCK_STREAM || record.Protocol != windows.IPPROTO_TCP || record.Addr == 0 || record.Addrlen != unsafe.Sizeof(windows.RawSockaddrInet4{}) {
			return nil, ErrEgressObservation
		}
		address := (*windows.RawSockaddrInet4)(unsafe.Pointer(record.Addr))
		port := *(*[2]byte)(unsafe.Pointer(&address.Port))
		ip := net.IPv4(address.Addr[0], address.Addr[1], address.Addr[2], address.Addr[3])
		if address.Family != windows.AF_INET || port != [2]byte{1, 187} || !publicProbeIPv4(ip) {
			return nil, ErrEgressObservation
		}
		value, duplicate := ip.String(), false
		for _, previous := range addresses {
			duplicate = duplicate || previous == value
		}
		if !duplicate {
			if len(addresses) == maxEgressAddresses {
				return nil, ErrEgressObservation
			}
			addresses = append(addresses, value)
		}
	}
	if len(addresses) == 0 || contextError(ctx) != nil {
		return nil, ErrEgressObservation
	}
	return addresses, nil // owned copies; native result list remains with this query
}

func (query *egressAddressQuery) retire() (result error) {
	if !query.called {
		return nil
	}
	result = ErrEgressObservation
	receipt := &lifecycleCleanup{}
	defer func() {
		if receipt.result == nil && query.returned && (!query.lookupEntered || query.lookupReturned) && query.freeReturned && (!query.started || query.cleanupReturned) {
			result = nil
		}
	}()
	defer lifecycleCleanupStep("egress original Winsock reference", receipt, func() error {
		if query.startupEntered && !query.startupReturned || query.lookupEntered && !query.lookupReturned || !query.freeReturned {
			return ErrEgressObservation
		}
		if !query.started {
			return nil
		}
		if query.cleanupAttempted {
			return ErrEgressObservation
		}
		query.cleanupAttempted = true
		if err := windows.WSACleanup(); err != nil {
			return ErrEgressObservation
		}
		query.cleanupReturned = true
		return nil
	})
	defer lifecycleCleanupStep("egress original address result list", receipt, func() error {
		if query.lookupEntered && !query.lookupReturned {
			return ErrEgressObservation
		}
		if query.records == nil {
			query.freeReturned = true
			return nil
		}
		if query.freeAttempted {
			return ErrEgressObservation
		}
		query.freeAttempted = true
		windows.FreeAddrInfoW(query.records)
		runtime.KeepAlive(query)
		query.freeReturned = true
		query.records = nil // only after the original void native Free returned normally
		return nil
	})
	return result
}
