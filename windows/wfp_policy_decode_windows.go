//go:build windows

package main

import (
	"errors"
	"net/netip"
	"slices"
	"unsafe"

	"github.com/snowden-system/windows/backend/core"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

const (
	maxWFPAppIDUnits     = 32761 // includes one NUL; preserves the existing 65522-byte blob cap
	maxWFPAppIDBlobBytes = 2 * maxWFPAppIDUnits
	maxWFPAppIDTextBytes = 3 * (maxWFPAppIDUnits - 1) // UTF-8/WTF-8 maximum per non-NUL UTF-16 unit
)

// Pure decoding of the fixed returned WFP ABI. The caller owns the native
// allocation throughout; these bounds do not validate arbitrary memory pointers.
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
			return decodeWFPAppIDBlob((*diagnosticWFPBlob)(unsafe.Pointer(v.value)))
		}
	}
	return nil, core.ErrNativeScopeUnavailable
}

// Windows strings can contain WTF-8 representations of unpaired surrogates.
// Reuse the platform codec and require an exact round trip instead of applying
// UTF-8 validation or silently normalizing an opaque native application identity.
func encodeWFPAppIDText(value string) ([]uint16, error) {
	if len(value) == 0 || len(value) > maxWFPAppIDTextBytes {
		return nil, core.ErrNativeScopeUnavailable
	}
	units, err := windows.UTF16FromString(value)
	if err != nil {
		return nil, errors.Join(core.ErrNativeScopeUnavailable, err)
	}
	if len(units) < 2 || len(units) > maxWFPAppIDUnits || windows.UTF16ToString(units) != value {
		return nil, core.ErrNativeScopeUnavailable
	}
	return units, nil // retained by the original producer before native use
}

func decodeWFPAppIDBlob(blob *diagnosticWFPBlob) (string, error) {
	if blob == nil || blob.data == nil || blob.size < 4 || blob.size > maxWFPAppIDBlobBytes || blob.size%2 != 0 {
		return "", core.ErrNativeScopeUnavailable
	}
	units := unsafe.Slice(blob.data, int(blob.size/2))
	last := len(units) - 1
	if units[last] != 0 {
		return "", core.ErrNativeScopeUnavailable
	}
	for _, unit := range units[:last] {
		if unit == 0 {
			return "", core.ErrNativeScopeUnavailable
		}
	}
	value := windows.UTF16ToString(units[:last])
	encoded, err := windows.UTF16FromString(value)
	if err != nil {
		return "", errors.Join(core.ErrNativeScopeUnavailable, err)
	}
	if !slices.Equal(units, encoded) {
		return "", core.ErrNativeScopeUnavailable // no lossy/colliding native identity
	}
	return value, nil // independent Go bytes; original native allocation remains caller-owned
}
