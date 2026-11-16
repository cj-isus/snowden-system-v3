//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/snowden-system/windows/backend/core"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

type nativeWFPProvider struct {
	key     windows.GUID
	display diagnosticWFPDisplay
	flags   uint32
	data    diagnosticWFPBlob
	service *uint16
}

type nativeWFPObjectState struct {
	providerPresent bool
	providerValid   bool
	sublayerPresent bool
	sublayerOwned   bool
	sublayerValid   bool
}

var (
	wfpGetProvider    = diagnosticWFPDLL.NewProc("FwpmProviderGetByKey0")
	wfpAddProvider    = diagnosticWFPDLL.NewProc("FwpmProviderAdd0")
	wfpAddSublayer    = diagnosticWFPDLL.NewProc("FwpmSubLayerAdd0")
	wfpDeleteProvider = diagnosticWFPDLL.NewProc("FwpmProviderDeleteByKey0")
	wfpDeleteSublayer = diagnosticWFPDLL.NewProc("FwpmSubLayerDeleteByKey0")
	wfpDeleteFilter   = diagnosticWFPDLL.NewProc("FwpmFilterDeleteByKey0")
	wfpImageAppID     = diagnosticWFPDLL.NewProc("FwpmGetAppIdFromFileName0")
)

// Only the original active session transaction may combine these object facts.
// Names are annotations; fixed GUID/provider/flags/service/weight are checked.
func (owner *nativeWFPSessionOwner) inspectObjects(ctx context.Context) (state nativeWFPObjectState, finalErr error) {
	complete := false
	defer func() {
		if payload := recover(); payload != nil || !complete && finalErr == nil {
			finalErr = core.ErrNativeScopeUnavailable
		}
		finalErr = errors.Join(finalErr, owner.reader.free())
	}()
	if ctx == nil || ctx.Err() != nil || !owner.callsReturned() || owner.handle == 0 || owner.closed || owner.closeStarted || owner.transaction == nil || owner.transaction.phase != wfpTransactionActive ||
		owner.reader.memory != nil || owner.reader.freeStarted || unsafe.Sizeof(nativeWFPProvider{}) != 64 || unsafe.Sizeof(diagnosticWFPSublayer{}) != 72 {
		return state, core.ErrNativeScopeUnavailable
	}
	if err := owner.queryObject(wfpGetProvider, owner.provider, uint32(windows.FWP_E_PROVIDER_NOT_FOUND)); err != nil {
		return state, err
	}
	if owner.reader.memory != nil {
		provider := (*nativeWFPProvider)(owner.reader.memory)
		if provider.key != owner.provider {
			return state, core.ErrNativeScopeUnavailable
		}
		state.providerPresent = true
		state.providerValid = provider.flags == 1 && provider.data.size == 0 && (provider.service == nil || *provider.service == 0)
	}
	if err := owner.reader.free(); err != nil {
		return state, err
	}
	if err := ctx.Err(); err != nil {
		return state, err
	}
	if err := owner.queryObject(diagnosticWFPGetSublayer, windows.GUID(wfpSublayerID), uint32(windows.FWP_E_SUBLAYER_NOT_FOUND)); err != nil {
		return state, err
	}
	if owner.reader.memory != nil {
		sublayer := (*diagnosticWFPSublayer)(owner.reader.memory)
		if sublayer.key != windows.GUID(wfpSublayerID) {
			return state, core.ErrNativeScopeUnavailable
		}
		state.sublayerPresent = true
		state.sublayerOwned = sublayer.provider != nil && *sublayer.provider == owner.provider
		state.sublayerValid = state.sublayerOwned && sublayer.flags == 1 && sublayer.weight == ^uint16(0) && sublayer.data.size == 0
	}
	resultErr := ctx.Err()
	complete = true
	return state, resultErr
}

func (owner *nativeWFPSessionOwner) queryObject(proc *windows.LazyProc, key windows.GUID, notFound uint32) error {
	if !owner.callsReturned() || owner.reader.memory != nil || owner.reader.freeStarted {
		return core.ErrHelperCleanup
	}
	if err := proc.Find(); err != nil {
		return err
	}
	if err := owner.reader.startQuery(); err != nil {
		return err
	}
	owner.reader.queryKey = key
	code, _, _ := proc.Call(uintptr(owner.handle), uintptr(unsafe.Pointer(&owner.reader.queryKey)), uintptr(unsafe.Pointer(&owner.reader.memory)))
	owner.reader.queryPending = false
	runtime.KeepAlive(owner)
	if uint32(code) == notFound && owner.reader.memory == nil {
		return nil
	}
	if code != 0 || owner.reader.memory == nil {
		return fmt.Errorf("WFP fixed object query unavailable (%d)", uint32(code))
	}
	return nil
}

type nativeWFPWrite struct {
	batch      *nativeWFPBatch // original slot owner, never a later transaction/context
	key        windows.GUID
	filterID   uint64
	attempted  bool
	returned   bool
	code       uint32
	text       [2][]uint16
	provider   nativeWFPProvider
	sublayer   diagnosticWFPSublayer
	filter     diagnosticWFPFilter
	conditions [8]diagnosticWFPCondition
	values     [8]uint64
	weight     uint64
	blob       diagnosticWFPBlob
}

func (owner *nativeWFPPolicyOwner) nextWrite() (*nativeWFPWrite, error) {
	if owner == nil || owner.batch == nil {
		return nil, core.ErrNativeScopeUnavailable
	}
	if err := owner.batch.operationScope(); err != nil {
		return nil, err
	}
	if !owner.session.callsReturned() || owner.session.dynamic || owner.session.handle == 0 || owner.session.closed || owner.session.closeStarted || owner.batch.count == len(owner.batch.writes) ||
		owner.session.transaction == nil || owner.session.transaction.phase != wfpTransactionActive || owner.session.transaction.readOnly ||
		owner.batch.transaction != owner.session.transaction || owner.session.transaction.batch != owner.batch {
		return nil, core.ErrNativeScopeUnavailable
	}
	write := &nativeWFPWrite{batch: owner.batch}
	owner.batch.writes[owner.batch.count] = write // custody BEFORE a native mutation
	owner.batch.count++
	return write, nil
}

func (owner *nativeWFPPolicyOwner) callWrite(write *nativeWFPWrite, proc *windows.LazyProc, args ...uintptr) error {
	if owner == nil || write == nil || write.attempted || !owner.session.callsReturned() {
		return core.ErrHelperCleanup
	}
	batch := write.batch
	if batch == nil || owner.batch != batch || batch.transaction == nil || owner.session.transaction != batch.transaction ||
		batch.transaction.batch != batch || batch.transaction.phase != wfpTransactionActive || batch.transaction.readOnly {
		return core.ErrNativeScopeUnavailable
	}
	if err := batch.operationScope(); err != nil {
		return err
	}
	if err := proc.Find(); err != nil {
		return err
	}
	if err := batch.operationScope(); err != nil {
		return err // Find/input preparation cannot extend the original write's authority
	}
	write.attempted = true
	owner.session.writePending = true
	code, _, _ := proc.Call(args...)
	write.code, write.returned = uint32(code), true
	owner.session.writePending = false
	runtime.KeepAlive(owner)
	if code != 0 {
		return fmt.Errorf("WFP policy mutation unavailable (%d)", uint32(code))
	}
	return batch.operationScope() // returned outputs precede late source-error projection
}

func (owner *nativeWFPPolicyOwner) addProvider() error {
	write, err := owner.nextWrite()
	if err != nil {
		return err
	}
	write.text[0], err = windows.UTF16FromString(wfpProviderName)
	if err != nil {
		return err
	}
	write.provider = nativeWFPProvider{key: owner.session.provider, flags: 1, display: diagnosticWFPDisplay{name: &write.text[0][0]}}
	return owner.callWrite(write, wfpAddProvider, uintptr(owner.session.handle), uintptr(unsafe.Pointer(&write.provider)), 0)
}

func (owner *nativeWFPPolicyOwner) addSublayer() error {
	write, err := owner.nextWrite()
	if err != nil {
		return err
	}
	write.text[0], err = windows.UTF16FromString(wfpSublayerName)
	if err != nil {
		return err
	}
	write.sublayer = diagnosticWFPSublayer{key: windows.GUID(wfpSublayerID), flags: 1, provider: &owner.session.provider, weight: ^uint16(0), display: diagnosticWFPDisplay{name: &write.text[0][0]}}
	return owner.callWrite(write, wfpAddSublayer, uintptr(owner.session.handle), uintptr(unsafe.Pointer(&write.sublayer)), 0)
}

func (owner *nativeWFPPolicyOwner) addRule(spec wfpRuleSpec) error {
	write, err := owner.nextWrite()
	if err != nil {
		return err
	}
	if len(spec.name) == 0 || len(spec.name) > 128 || len(spec.conditions) > len(write.conditions) || spec.weight > 15 || spec.action != wf.ActionPermit && spec.action != wf.ActionBlock ||
		!(spec.id >= 10 && spec.id <= 15 || spec.id >= 18 && spec.id <= 21) || spec.layer != wf.LayerALEAuthConnectV4 && spec.layer != wf.LayerALEAuthConnectV6 {
		return core.ErrNativeScopeUnavailable
	}
	write.text[0], err = windows.UTF16FromString(spec.name)
	if err != nil {
		return err
	}
	write.key, write.weight = wfpStableGUID(spec.id), spec.weight
	for index, condition := range spec.conditions {
		if condition == nil {
			return core.ErrNativeScopeUnavailable
		}
		encoded := &write.conditions[index]
		encoded.field, encoded.match = windows.GUID(condition.Field), uint32(condition.Op)
		switch condition.Field {
		case wf.FieldALEAppID:
			value, ok := condition.Value.(string)
			if !ok || len(value) == 0 || len(value) > 32760 || condition.Op != wf.MatchTypeEqual || write.text[1] != nil {
				return core.ErrNativeScopeUnavailable
			}
			write.text[1], err = windows.UTF16FromString(value)
			if err != nil {
				return err
			}
			write.blob = diagnosticWFPBlob{size: uint32(2 * len(write.text[1])), data: &write.text[1][0]}
			encoded.value = diagnosticWFPValue{typeID: 12, value: uintptr(unsafe.Pointer(&write.blob))}
		case wf.FieldIPLocalInterface:
			value, ok := condition.Value.(uint64)
			if !ok || value == 0 || condition.Op != wf.MatchTypeEqual {
				return core.ErrNativeScopeUnavailable
			}
			write.values[index] = value
			encoded.value = diagnosticWFPValue{typeID: 4, value: uintptr(unsafe.Pointer(&write.values[index]))}
		case wf.FieldIPProtocol:
			value, ok := condition.Value.(wf.IPProto)
			if !ok || condition.Op != wf.MatchTypeEqual {
				return core.ErrNativeScopeUnavailable
			}
			encoded.value = diagnosticWFPValue{typeID: 1, value: uintptr(value)}
		case wf.FieldIPLocalPort, wf.FieldIPRemotePort:
			value, ok := condition.Value.(uint16)
			if !ok || condition.Op != wf.MatchTypeEqual {
				return core.ErrNativeScopeUnavailable
			}
			encoded.value = diagnosticWFPValue{typeID: 2, value: uintptr(value)}
		case wf.FieldFlags:
			value, ok := condition.Value.(wf.ConditionFlag)
			if !ok || condition.Op != wf.MatchTypeFlagsAllSet {
				return core.ErrNativeScopeUnavailable
			}
			encoded.value = diagnosticWFPValue{typeID: 3, value: uintptr(value)}
		default:
			return core.ErrNativeScopeUnavailable
		}
	}
	write.filter = diagnosticWFPFilter{key: write.key, flags: 1, provider: &owner.session.provider, layer: windows.GUID(spec.layer), sublayer: windows.GUID(wfpSublayerID),
		display: diagnosticWFPDisplay{name: &write.text[0][0]}, weight: diagnosticWFPValue{typeID: 4, value: uintptr(unsafe.Pointer(&write.weight))},
		count: uint32(len(spec.conditions)), action: diagnosticWFPAction{typeID: uint32(spec.action)}}
	if len(spec.conditions) != 0 {
		write.filter.conditions = &write.conditions[0]
	}
	if err := owner.callWrite(write, diagnosticWFPAdd, uintptr(owner.session.handle), uintptr(unsafe.Pointer(&write.filter)), 0, uintptr(unsafe.Pointer(&write.filterID))); err != nil {
		return err
	}
	if write.filterID == 0 {
		return core.ErrNativeScopeUnavailable
	}
	return nil
}

// Callers derive keys ONLY from the same original transaction's owned snapshot
// or its fixed provider/sublayer lookup. No outside lookup/key authorizes delete.
func (owner *nativeWFPPolicyOwner) deleteObject(proc *windows.LazyProc, key windows.GUID) error {
	write, err := owner.nextWrite()
	if err != nil {
		return err
	}
	write.key = key
	return owner.callWrite(write, proc, uintptr(owner.session.handle), uintptr(unsafe.Pointer(&write.key)))
}

// FwpmGetAppId has no session argument but still returns an owned allocation,
// including partial-error outputs. Reuse the same original reader resource
// record; a failed free cannot be hidden by another SDK call or new session.
func (owner *nativeWFPSessionOwner) imageAppID(ctx context.Context, path string) (appID string, finalErr error) {
	complete := false
	defer func() {
		if payload := recover(); payload != nil || !complete && finalErr == nil {
			finalErr = core.ErrNativeImageUnavailable
		}
		finalErr = errors.Join(finalErr, owner.reader.free())
		if ctx != nil {
			finalErr = errors.Join(finalErr, ctx.Err())
		}
		if finalErr != nil {
			appID = ""
		}
	}()
	if ctx == nil || ctx.Err() != nil || !owner.callsReturned() || !filepath.IsAbs(path) || len(path) > 32760 || owner.closed || owner.closeStarted || owner.reader.memory != nil || owner.reader.freeStarted || owner.reader.enum != 0 || owner.reader.destroyStarted {
		return "", core.ErrNativeImageUnavailable
	}
	for _, proc := range []*windows.LazyProc{wfpImageAppID, diagnosticWFPFree} {
		if err := proc.Find(); err != nil {
			return "", err
		}
	}
	var err error
	owner.queryPath, err = windows.UTF16FromString(path)
	if err != nil {
		return "", err
	}
	if err := owner.reader.startQuery(); err != nil {
		return "", err
	}
	code, _, _ := wfpImageAppID.Call(uintptr(unsafe.Pointer(&owner.queryPath[0])), uintptr(unsafe.Pointer(&owner.reader.memory)))
	owner.reader.queryPending = false
	runtime.KeepAlive(owner)
	if code != 0 || owner.reader.memory == nil {
		return "", core.ErrNativeImageUnavailable
	}
	value, err := decodeWFPCondition(diagnosticWFPCondition{field: windows.GUID(wf.FieldALEAppID), value: diagnosticWFPValue{typeID: 12, value: uintptr(owner.reader.memory)}})
	if err != nil {
		return "", err
	}
	appID, ok := value.(string)
	if !ok || appID == "" {
		return "", core.ErrNativeImageUnavailable
	}
	complete = true
	return appID, nil
}

func readOwnedWFPAppID(ctx context.Context, path string) (string, error) {
	return withOwnedWFPReader(ctx, func(owner *nativeWFPSessionOwner) (string, error) { return owner.imageAppID(ctx, path) })
}
