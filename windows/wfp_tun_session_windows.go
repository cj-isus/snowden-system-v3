//go:build windows

package main

import (
	"context"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

// Fixed Windows/amd64 FWPM_SESSION0/FWPM_FILTER0 ABI only. The signed desktop
// target is amd64; other architectures fail before any native acquisition.
// https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_filter0
// https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_session0
// This is a fixed-policy session/rule adapter, not a general WFP SDK. The pinned
// wf package still supplies AppID and public value types.
type diagnosticWFPDisplay struct{ name, description *uint16 }
type diagnosticWFPBlob struct {
	size uint32
	data *uint16
}
type diagnosticWFPValue struct {
	typeID uint32
	value  uintptr
}
type diagnosticWFPCondition struct {
	field windows.GUID
	match uint32
	value diagnosticWFPValue
}
type diagnosticWFPSession struct {
	key       windows.GUID
	display   diagnosticWFPDisplay
	flags     uint32
	timeout   uint32
	processID uint32
	sid       *windows.SID
	username  *uint16
	kernel    uint32
}
type diagnosticWFPAction struct {
	typeID uint32
	key    windows.GUID
}
type diagnosticWFPFilter struct {
	key        windows.GUID
	display    diagnosticWFPDisplay
	flags      uint32
	provider   *windows.GUID
	data       diagnosticWFPBlob
	layer      windows.GUID
	sublayer   windows.GUID
	weight     diagnosticWFPValue
	count      uint32
	conditions *diagnosticWFPCondition
	action     diagnosticWFPAction
	context    [2]uint64 // C union UINT64/GUID occupies 16 bytes, aligned to 8
	reserved   *windows.GUID
	id         uint64
	effective  diagnosticWFPValue
}

var (
	diagnosticWFPDLL   = windows.NewLazySystemDLL("fwpuclnt.dll")
	diagnosticWFPOpen  = diagnosticWFPDLL.NewProc("FwpmEngineOpen0")
	diagnosticWFPAdd   = diagnosticWFPDLL.NewProc("FwpmFilterAdd0")
	diagnosticWFPClose = diagnosticWFPDLL.NewProc("FwpmEngineClose0")
)

// All API outputs and pointer backing stores belong to this object BEFORE the
// first acquisition. No wf.New partial-constructor/Close handle loss, unsafe
// access to wf.Session internals, second mutable session or key-based adoption.
type nativeWFPSessionOwner struct {
	handle        windows.Handle
	openAttempted bool
	openReturned  bool
	openCode      uint32
	closeStarted  bool
	closeReturned bool
	closeErr      error
	closed        bool
	writePending  bool // original Add/Delete has not returned; no transaction/Close may bypass it
	dynamic       bool
	provider      windows.GUID
	session       diagnosticWFPSession
	name          []uint16
	queryPath     []uint16
	reader        diagnosticWFPReader
	transaction   *nativeWFPTransaction
}

type nativeDiagnosticWFP struct {
	nativeWFPSessionOwner
	addAttempted bool
	addReturned  bool
	filterID     uint64
	luid, weight uint64
	appID        []uint16
	blob         diagnosticWFPBlob
	conditions   [6]diagnosticWFPCondition
	filter       diagnosticWFPFilter
}

func (owner *nativeWFPSessionOwner) openSession() error {
	return owner.openSessionContext(context.Background())
}

func (owner *nativeWFPSessionOwner) openSessionContext(ctx context.Context) error {
	return owner.openSessionModeContext(ctx, true, "Snowden owned policy observation")
}

func (owner *nativeWFPSessionOwner) openSessionMode(dynamic bool, name string) error {
	return owner.openSessionModeContext(context.Background(), dynamic, name)
}

func (owner *nativeWFPSessionOwner) openSessionModeContext(ctx context.Context, dynamic bool, name string) error {
	if err := wfpOperationScope(ctx); err != nil {
		return err
	}
	if !owner.callsReturned() || owner.openAttempted || owner.closeStarted || owner.closed || len(name) == 0 || len(name) > 128 || runtime.GOARCH != "amd64" ||
		unsafe.Sizeof(diagnosticWFPSession{}) != 72 || unsafe.Sizeof(diagnosticWFPCondition{}) != 40 ||
		unsafe.Sizeof(diagnosticWFPFilter{}) != 200 || unsafe.Offsetof(diagnosticWFPFilter{}.reserved) != 168 {
		return fmt.Errorf("WFP diagnostic native scope/ABI unavailable")
	}
	for _, proc := range []*windows.LazyProc{diagnosticWFPOpen, diagnosticWFPAdd, diagnosticWFPClose} {
		if err := proc.Find(); err != nil {
			return err
		}
	}
	var err error
	owner.name, err = windows.UTF16FromString(name)
	if err != nil {
		return err
	}
	owner.dynamic = dynamic
	owner.session = diagnosticWFPSession{timeout: 1000, display: diagnosticWFPDisplay{name: &owner.name[0]}}
	if dynamic {
		owner.session.flags = 1
	}
	if err := wfpOperationScope(ctx); err != nil {
		return err // preparation/Find may have outlived the original operation
	}
	owner.openAttempted = true
	code, _, _ := diagnosticWFPOpen.Call(0, 10, 0, uintptr(unsafe.Pointer(&owner.session)), uintptr(unsafe.Pointer(&owner.handle)))
	runtime.KeepAlive(owner)
	owner.openCode, owner.openReturned = uint32(code), true
	if code != 0 || owner.handle == 0 {
		return fmt.Errorf("WFP diagnostic session unavailable (%d)", uint32(code))
	}
	owner.provider = windows.GUID(wfpProviderID)
	return wfpOperationScope(ctx) // late success retains this original handle for negative cleanup
}

func (owner *nativeDiagnosticWFP) open(ctx context.Context, appID string, port uint16, luid uint64) error {
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err
	}
	if !owner.callsReturned() || appID == "" || len(appID) > 32760 || port == 0 || luid == 0 || !owner.dynamic || owner.addAttempted || owner.handle == 0 || !owner.openReturned || owner.openCode != 0 || owner.closeStarted || owner.closed || owner.transaction != nil && owner.transaction.phase != wfpTransactionFinished {
		return fmt.Errorf("WFP diagnostic native scope unavailable")
	}
	var err error
	owner.appID, err = windows.UTF16FromString(appID)
	if err != nil {
		return err
	}
	owner.provider, owner.luid, owner.weight = windows.GUID(wfpProviderID), luid, 14
	owner.blob = diagnosticWFPBlob{size: uint32(2 * len(owner.appID)), data: &owner.appID[0]}
	value := func(kind uint32, value uintptr) diagnosticWFPValue {
		return diagnosticWFPValue{typeID: kind, value: value}
	}
	owner.conditions = [6]diagnosticWFPCondition{
		{field: windows.GUID(wf.FieldALEAppID), value: value(12, uintptr(unsafe.Pointer(&owner.blob)))},
		{field: windows.GUID(wf.FieldIPLocalInterface), value: value(4, uintptr(unsafe.Pointer(&owner.luid)))},
		{field: windows.GUID(wf.FieldIPProtocol), value: value(1, 6)},
		{field: windows.GUID(wf.FieldIPLocalPort), value: value(2, uintptr(port))},
		{field: windows.GUID(wf.FieldIPRemoteAddress), value: value(3, 0x01010101)},
		{field: windows.GUID(wf.FieldIPRemotePort), value: value(2, 443)},
	}
	owner.filter = diagnosticWFPFilter{
		key: wfpStableGUID(30), display: diagnosticWFPDisplay{name: &owner.name[0]}, provider: &owner.provider,
		layer: windows.GUID(wf.LayerALEAuthConnectV4), sublayer: windows.GUID(wfpSublayerID),
		weight: value(4, uintptr(unsafe.Pointer(&owner.weight))), count: uint32(len(owner.conditions)), conditions: &owner.conditions[0],
		action: diagnosticWFPAction{typeID: uint32(wf.ActionPermit)},
	}
	// No persistent/boot/hard-action flag. BFE ties this one filter to the
	// original dynamic session, including client-process death/RPC rundown.
	if err := wfpDiagnosticOperationScope(ctx); err != nil {
		return err // no original Add was issued; pointer backing stores remain owned
	}
	owner.addAttempted = true
	owner.writePending = true
	code, _, _ := diagnosticWFPAdd.Call(uintptr(owner.handle), uintptr(unsafe.Pointer(&owner.filter)), 0, uintptr(unsafe.Pointer(&owner.filterID)))
	owner.addReturned = true
	owner.writePending = false
	runtime.KeepAlive(owner)
	if code != 0 || owner.filterID == 0 {
		return fmt.Errorf("WFP diagnostic filter unavailable (%d)", uint32(code))
	}
	return wfpDiagnosticOperationScope(ctx) // recorded Add success is not current-operation authority
}

func (owner *nativeWFPSessionOwner) close() error {
	if !owner.callsReturned() {
		return fmt.Errorf("WFP original native operation did not return")
	}
	if owner.openAttempted && !owner.openReturned {
		return fmt.Errorf("WFP original session open did not return") // retain outputs; no unknown-handle Close
	}
	if err := owner.reader.cleanup(owner.handle); err != nil {
		return err
	}
	if owner.closed {
		return nil
	}
	if owner.closeStarted {
		return owner.closeErr // never retry an uncertain native Close as proof
	}
	if owner.handle == 0 {
		if !owner.openAttempted || owner.openReturned && owner.openCode != 0 {
			owner.closed = true
			return nil // no handle returned and no filter operation was attempted
		}
		return fmt.Errorf("WFP diagnostic partial session unavailable")
	}
	owner.closeStarted, owner.closeErr = true, fmt.Errorf("WFP diagnostic session closure unconfirmed")
	code, _, _ := diagnosticWFPClose.Call(uintptr(owner.handle))
	owner.closeReturned = true
	runtime.KeepAlive(owner)
	if code != 0 {
		return owner.closeErr // retain the original numeric handle + sticky uncertainty
	}
	owner.handle, owner.closed, owner.closeErr = 0, true, nil
	if owner.transaction != nil {
		owner.transaction.phase = wfpTransactionFinished
		owner.transaction.sessionRetired = true // cleanup only; never commit authority
	}
	return nil
}
