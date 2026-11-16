//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"sync"
	"unsafe"

	"github.com/snowden-system/windows/backend/core"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

// Read only this provider: at most 32 rules, 8 conditions per rule, one page of
// 16 pointers. Native metadata/RPC time and OS allocation are not hard bounded.
// In particular, do not route FWPM_FILTER0 through pinned wf's sequential
// RawContext+ProviderContextKey representation of the SDK's 16-byte C union.
type diagnosticWFPEnumTemplate struct {
	provider        *windows.GUID
	layer           windows.GUID
	kind, flags     uint32
	providerContext unsafe.Pointer
	count           uint32
	conditions      *diagnosticWFPCondition
	actionMask      uint32
	callout         *windows.GUID
}
type diagnosticWFPSublayer struct {
	key      windows.GUID
	display  diagnosticWFPDisplay
	flags    uint32
	provider *windows.GUID
	data     diagnosticWFPBlob
	weight   uint16
}

var (
	diagnosticWFPEnumCreate  = diagnosticWFPDLL.NewProc("FwpmFilterCreateEnumHandle0")
	diagnosticWFPEnumRead    = diagnosticWFPDLL.NewProc("FwpmFilterEnum0")
	diagnosticWFPEnumDestroy = diagnosticWFPDLL.NewProc("FwpmFilterDestroyEnumHandle0")
	diagnosticWFPFree        = diagnosticWFPDLL.NewProc("FwpmFreeMemory0")
	diagnosticWFPGetSublayer = diagnosticWFPDLL.NewProc("FwpmSubLayerGetByKey0")
)

type diagnosticWFPReader struct {
	enum            windows.Handle
	memory          unsafe.Pointer // API output stored here BEFORE each call
	count           uint32
	freeStarted     bool
	freeReturned    bool
	destroyStarted  bool
	destroyReturned bool
	queryPending    bool
	queryKey        windows.GUID // retained input for the original fixed-object query
	template        diagnosticWFPEnumTemplate
}

func (reader *diagnosticWFPReader) callsReturned() bool {
	return reader != nil && !reader.queryPending && (!reader.freeStarted || reader.freeReturned) &&
		(!reader.destroyStarted || reader.destroyReturned)
}

func (reader *diagnosticWFPReader) startQuery() error {
	if !reader.callsReturned() || reader.freeStarted || reader.destroyStarted {
		return core.ErrHelperCleanup
	}
	reader.queryPending = true
	return nil
}

// Original synchronous cleanup step. Register all steps before invoking them;
// Goexit still runs later deferred steps, whose graph checks may refuse native I/O.
func wfpCleanupStep(result *error, cleanup func() error) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			*result = errors.Join(*result, core.ErrHelperCleanup, fmt.Errorf("WFP cleanup did not return normally (%T)", payload))
		}
	}()
	err := cleanup()
	if *result == nil {
		*result = err
	} else {
		*result = errors.Join(*result, err)
	}
	returned = true
}

func (reader *diagnosticWFPReader) free() (finalErr error) {
	wfpCleanupStep(&finalErr, func() error {
		if !reader.callsReturned() || reader.freeStarted {
			return core.ErrHelperCleanup
		}
		if reader.memory == nil {
			return nil
		}
		reader.freeStarted, reader.freeReturned = true, false
		diagnosticWFPFree.Call(uintptr(unsafe.Pointer(&reader.memory))) // documented void; only original allocation
		reader.freeReturned = true
		runtime.KeepAlive(reader)
		reader.memory, reader.freeStarted = nil, false
		return nil
	})
	return finalErr
}

func (reader *diagnosticWFPReader) destroy(handle windows.Handle) (finalErr error) {
	wfpCleanupStep(&finalErr, func() error {
		if !reader.callsReturned() || reader.destroyStarted {
			return core.ErrHelperCleanup
		}
		if reader.enum == 0 {
			return nil
		}
		reader.destroyStarted, reader.destroyReturned = true, false
		code, _, _ := diagnosticWFPEnumDestroy.Call(uintptr(handle), uintptr(reader.enum))
		reader.destroyReturned = true
		if code != 0 {
			return core.ErrHelperCleanup
		}
		reader.enum, reader.destroyStarted = 0, false
		return nil
	})
	return finalErr
}

func (reader *diagnosticWFPReader) cleanup(handle windows.Handle) (result error) {
	defer wfpCleanupStep(&result, func() error { return reader.destroy(handle) })
	defer wfpCleanupStep(&result, reader.free)
	return nil
}

func (owner *nativeWFPSessionOwner) readPolicy(ctx context.Context, sublayer bool) ([]*wf.Rule, bool, error) {
	return owner.readPolicyMode(ctx, sublayer, false)
}

// Identity-only rows exist solely for explicit removal of a damaged owned
// policy. They carry no action/weight/conditions and cannot pass verification.
func (owner *nativeWFPSessionOwner) readRuleIdentities(ctx context.Context) ([]*wf.Rule, error) {
	rules, _, err := owner.readPolicyMode(ctx, false, true)
	return rules, err
}

func (owner *nativeWFPSessionOwner) readPolicyMode(ctx context.Context, sublayer, identitiesOnly bool) (rules []*wf.Rule, validSublayer bool, finalErr error) {
	reader := &owner.reader
	complete, ownsTransaction := false, false
	defer func() {
		if recovered := recover(); recovered != nil || !complete && finalErr == nil {
			finalErr = core.ErrNativeScopeUnavailable
		}
		if ctx != nil {
			finalErr = errors.Join(finalErr, ctx.Err())
		}
		if finalErr != nil {
			rules, validSublayer = nil, false
		}
	}()
	defer func() {
		if ownsTransaction {
			wfpCleanupStep(&finalErr, owner.abortTransaction)
		}
	}()
	defer wfpCleanupStep(&finalErr, func() error { return reader.cleanup(owner.handle) })
	if ctx == nil || ctx.Err() != nil || !owner.callsReturned() || owner.handle == 0 || owner.closeStarted || owner.closed || reader.memory != nil || reader.enum != 0 || reader.freeStarted || reader.destroyStarted ||
		unsafe.Sizeof(diagnosticWFPEnumTemplate{}) != 72 || unsafe.Sizeof(diagnosticWFPSublayer{}) != 72 {
		return nil, false, core.ErrNativeScopeUnavailable
	}
	if owner.transaction == nil || owner.transaction.phase == wfpTransactionFinished {
		ownsTransaction = true // retain the original pending transaction even on Begin error
		if err := owner.beginTransaction(true); err != nil {
			return nil, false, err
		}
	} else if owner.transaction.phase != wfpTransactionActive {
		return nil, false, core.ErrNativeScopeUnavailable
	}
	for _, proc := range []*windows.LazyProc{diagnosticWFPEnumCreate, diagnosticWFPEnumRead, diagnosticWFPEnumDestroy, diagnosticWFPFree, diagnosticWFPGetSublayer} {
		if err := proc.Find(); err != nil {
			return nil, false, err
		}
	}
	if sublayer {
		state, err := owner.inspectObjects(ctx)
		if err != nil {
			return nil, false, err
		}
		validSublayer = state.providerValid && state.sublayerValid
		if !validSublayer {
			return nil, false, core.ErrNativeScopeUnavailable
		}
	}
	reader.template = diagnosticWFPEnumTemplate{provider: &owner.provider, flags: 8 | 16, actionMask: ^uint32(0)}
	if err := reader.startQuery(); err != nil {
		return nil, false, err
	}
	code, _, _ := diagnosticWFPEnumCreate.Call(uintptr(owner.handle), uintptr(unsafe.Pointer(&reader.template)), uintptr(unsafe.Pointer(&reader.enum)))
	reader.queryPending = false
	runtime.KeepAlive(owner)
	if uint32(code) == uint32(windows.FWP_E_PROVIDER_NOT_FOUND) && reader.enum == 0 {
		state, err := owner.inspectObjects(ctx)
		if err != nil || state.providerPresent {
			return nil, false, core.ErrNativeScopeUnavailable
		}
		resultErr := ctx.Err()
		complete = true
		return nil, validSublayer, resultErr
	}
	if code != 0 || reader.enum == 0 {
		return nil, false, core.ErrNativeScopeUnavailable
	}
	seen := map[wf.RuleID]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		reader.count = 0
		if err := reader.startQuery(); err != nil {
			return nil, false, err
		}
		code, _, _ = diagnosticWFPEnumRead.Call(uintptr(owner.handle), uintptr(reader.enum), 16, uintptr(unsafe.Pointer(&reader.memory)), uintptr(unsafe.Pointer(&reader.count)))
		reader.queryPending = false
		runtime.KeepAlive(owner)
		if code != 0 || reader.count > 16 || reader.count > 0 && reader.memory == nil || len(rules)+int(reader.count) > 32 {
			return nil, false, core.ErrNativeScopeUnavailable
		}
		if reader.count == 0 {
			break
		}
		for _, raw := range unsafe.Slice((**diagnosticWFPFilter)(reader.memory), int(reader.count)) {
			var rule *wf.Rule
			var err error
			if identitiesOnly {
				if raw == nil || raw.provider == nil || *raw.provider != owner.provider || raw.id == 0 {
					return nil, false, core.ErrNativeScopeUnavailable
				}
				rule = &wf.Rule{ID: wf.RuleID(raw.key), Provider: wfpProviderID, KernelID: raw.id, Persistent: raw.flags&1 != 0, Sublayer: wf.SublayerID(raw.sublayer)}
			} else {
				rule, err = decodeOwnedWFPRule(raw)
			}
			if err != nil || seen[rule.ID] {
				return nil, false, core.ErrNativeScopeUnavailable
			}
			seen[rule.ID] = true
			rules = append(rules, rule)
		}
		if err := reader.free(); err != nil {
			return nil, false, err
		}
	}
	resultErr := ctx.Err()
	complete = true
	return rules, validSublayer, resultErr
}

func decodeOwnedWFPRule(raw *diagnosticWFPFilter) (*wf.Rule, error) {
	if raw == nil || raw.provider == nil || *raw.provider != windows.GUID(wfpProviderID) || raw.id == 0 || raw.count > 8 || raw.count > 0 && raw.conditions == nil || raw.weight.typeID != 4 || raw.weight.value == 0 || raw.effective.typeID != 4 || raw.effective.value == 0 || raw.flags & ^uint32(1|2|8|16|32|64) != 0 || raw.data.size != 0 || raw.action.key != (windows.GUID{}) || raw.context[0] != 0 || raw.reserved != nil {
		return nil, core.ErrNativeScopeUnavailable
	}
	weight := *(*uint64)(unsafe.Pointer(raw.weight.value))
	if weight != *(*uint64)(unsafe.Pointer(raw.effective.value)) {
		return nil, core.ErrNativeScopeUnavailable
	}
	rule := &wf.Rule{ID: wf.RuleID(raw.key), KernelID: raw.id, Provider: wfpProviderID, Layer: wf.LayerID(raw.layer), Sublayer: wf.SublayerID(raw.sublayer), Weight: weight,
		Action: wf.Action(raw.action.typeID), Persistent: raw.flags&1 != 0, BootTime: raw.flags&2 != 0, HardAction: raw.flags&8 != 0, PermitIfMissing: raw.flags&16 != 0, Disabled: raw.flags&32 != 0}
	if rule.Action != wf.ActionPermit && rule.Action != wf.ActionBlock {
		return nil, core.ErrNativeScopeUnavailable
	}
	for _, condition := range unsafe.Slice(raw.conditions, int(raw.count)) {
		value, err := decodeWFPCondition(condition)
		if err != nil {
			return nil, err
		}
		rule.Conditions = append(rule.Conditions, &wf.Match{Field: wf.FieldID(condition.field), Op: wf.MatchType(condition.match), Value: value})
	}
	return rule, nil
}

func decodeWFPCondition(condition diagnosticWFPCondition) (any, error) {
	v := condition.value
	// Read only the active C union member's width. Padding/unused upper bytes
	// are not required to be zero and must not change an otherwise exact match.
	switch wf.FieldID(condition.field) {
	case wf.FieldIPProtocol:
		if v.typeID == 1 {
			return wf.IPProto(*(*uint8)(unsafe.Pointer(&v.value))), nil
		}
	case wf.FieldFlags:
		if v.typeID == 3 {
			return wf.ConditionFlag(*(*uint32)(unsafe.Pointer(&v.value))), nil
		}
	case wf.FieldIPLocalPort, wf.FieldIPRemotePort:
		if v.typeID == 2 {
			return *(*uint16)(unsafe.Pointer(&v.value)), nil
		}
	case wf.FieldIPLocalInterface:
		if v.typeID == 4 && v.value != 0 {
			return *(*uint64)(unsafe.Pointer(v.value)), nil
		}
	case wf.FieldIPRemoteAddress:
		if v.typeID == 3 {
			x := *(*uint32)(unsafe.Pointer(&v.value))
			return netip.AddrFrom4([4]byte{byte(x >> 24), byte(x >> 16), byte(x >> 8), byte(x)}), nil
		}
	case wf.FieldALEAppID:
		if v.typeID == 12 && v.value != 0 {
			blob := (*diagnosticWFPBlob)(unsafe.Pointer(v.value))
			if blob.size >= 2 && blob.size <= 65522 && blob.size%2 == 0 && blob.data != nil {
				units := unsafe.Slice(blob.data, int(blob.size/2))
				if units[len(units)-1] != 0 {
					break
				}
				for _, unit := range units[:len(units)-1] {
					if unit == 0 {
						return nil, core.ErrNativeScopeUnavailable
					}
				}
				return windows.UTF16ToString(units), nil
			}
		}
	}
	return nil, core.ErrNativeScopeUnavailable
}

// Existing root readers also use this retained native lane. An uncertain page,
// enum or session prevents another read session and any successful policy gate.
var wfpReadLane struct {
	sync.Mutex
	owner *nativeWFPSessionOwner
}

type nativeWFPReadSnapshot struct {
	rules []*wf.Rule
	valid bool
}

func readOwnedWFPPolicy(ctx context.Context, sublayer bool) ([]*wf.Rule, bool, error) {
	value, err := withOwnedWFPReader(ctx, func(owner *nativeWFPSessionOwner) (nativeWFPReadSnapshot, error) {
		if err := owner.openSession(); err != nil {
			return nativeWFPReadSnapshot{}, err
		}
		rules, valid, err := owner.readPolicy(ctx, sublayer)
		return nativeWFPReadSnapshot{rules: rules, valid: valid}, err
	})
	return value.rules, value.valid, err
}

func withOwnedWFPReader[T any](ctx context.Context, read func(*nativeWFPSessionOwner) (T, error)) (value T, finalErr error) {
	var zero T
	if !wfpReadLane.TryLock() {
		return zero, core.ErrHelperBusy
	}
	defer wfpReadLane.Unlock()
	complete := false
	defer func() {
		if recovered := recover(); recovered != nil || !complete && finalErr == nil {
			finalErr = core.ErrNativeScopeUnavailable
		}
		if wfpReadLane.owner != nil {
			err := guardedWFPReadClose(wfpReadLane.owner)
			if err == nil {
				wfpReadLane.owner = nil
			} else {
				finalErr = errors.Join(finalErr, core.ErrHelperCleanup, err)
			}
		}
		if ctx != nil && ctx.Err() != nil {
			finalErr = errors.Join(finalErr, ctx.Err())
		}
		if finalErr != nil {
			value = zero
		}
	}()
	if wfpReadLane.owner != nil {
		if err := guardedWFPReadClose(wfpReadLane.owner); err != nil {
			return zero, err
		}
		wfpReadLane.owner = nil
	}
	if ctx == nil || ctx.Err() != nil {
		return zero, core.ErrNativeScopeUnavailable
	}
	owner := &nativeWFPSessionOwner{}
	wfpReadLane.owner = owner
	value, finalErr = read(owner)
	complete = true
	return value, finalErr
}

func guardedWFPReadClose(owner *nativeWFPSessionOwner) (err error) {
	wfpCleanupStep(&err, owner.close)
	return err
}

func readOwnedWFPRules(ctx context.Context) ([]*wf.Rule, error) {
	rules, _, err := readOwnedWFPPolicy(ctx, false)
	return rules, err
}
