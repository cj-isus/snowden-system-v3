package childbootstrap

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
)

const (
	// Fixed sibling image, shared with the installer without importing update
	// or the GUI/core dependency graph into the minimal helper.
	ApplicationFile = "snowden-system.exe"

	BrokerArgument       = "--internal-bootstrap-broker-v1"
	BrokerPinnedArgument = "--internal-bootstrap-broker-v2"
	BrokerGUIArgument    = "--internal-broker-gui-v1"

	BrokerControlSeedSize   = FrameSize + 3*8
	BrokerSuccessorSeedSize = FrameSize + 4*8
)

type BrokerPurpose uint8

const (
	BrokerControl   BrokerPurpose = iota + 1 // original App -> elevated helper
	BrokerSuccessor                          // helper -> elevated GUI
)

// BrokerProtocol is a distinct wire from GUI6/R1/U1. Binding and raw target
// handles are correlation data, never image, SID, Job or elevation authority.
// Only elevation is defined here; there is no argv-selected executable path.
type BrokerProtocol struct {
	purpose   BrokerPurpose
	binding   [32]byte
	helperPin string // control-v2 data; authority comes from the authenticated original App
}

// NewPinnedBrokerProtocol selects a distinct control wire. A helper must still
// authenticate the held App/peer before trusting this independently supplied pin.
func NewPinnedBrokerProtocol(binding [32]byte, helperPin string) (BrokerProtocol, error) {
	if len(helperPin) != 64 {
		return BrokerProtocol{}, ErrProtocol
	}
	raw, err := hex.DecodeString(helperPin)
	if err != nil || hex.EncodeToString(raw) != helperPin {
		return BrokerProtocol{}, ErrProtocol
	}
	return BrokerProtocol{purpose: BrokerControl, binding: binding, helperPin: helperPin}, nil
}

func (protocol BrokerProtocol) HelperPin() string { return protocol.helperPin }
func (protocol BrokerProtocol) PinnedControl() bool {
	return protocol.purpose == BrokerControl && protocol.helperPin != ""
}

func NewBrokerProtocol(purpose BrokerPurpose, binding [32]byte) (BrokerProtocol, error) {
	if purpose != BrokerControl && purpose != BrokerSuccessor {
		return BrokerProtocol{}, ErrProtocol
	}
	return BrokerProtocol{purpose: purpose, binding: binding}, nil
}

func (protocol BrokerProtocol) Purpose() BrokerPurpose { return protocol.purpose }

// Binding returns correlation bytes only, for a purpose-specific transport name.
func (protocol BrokerProtocol) Binding() [32]byte { return protocol.binding }

func (protocol BrokerProtocol) Stages() Stages {
	switch protocol.purpose {
	case BrokerControl:
		if protocol.PinnedControl() {
			return Stages{Seed: "SB02", Armed: "AB02", Go: "GB02", Owned: "OB02", Use: "UB02", Ack: "KB02"}
		}
		return Stages{Seed: "SB01", Armed: "AB01", Go: "GB01", Owned: "OB01", Use: "UB01", Ack: "KB01"}
	case BrokerSuccessor:
		return Stages{Seed: "SG01", Armed: "AG01", Go: "GG01", Owned: "OG01", Use: "UG01", Ack: "KG01"}
	default:
		return Stages{}
	}
}

func (protocol BrokerProtocol) seedSize() int {
	switch protocol.purpose {
	case BrokerControl:
		return BrokerControlSeedSize
	case BrokerSuccessor:
		return BrokerSuccessorSeedSize
	default:
		return 0
	}
}

// ValidHandles checks only this wire's numeric shape, never native authority.
func (protocol BrokerProtocol) ValidHandles(handles [4]uint64) bool {
	size := protocol.seedSize()
	return size != 0 && validTargetHandles(handles, (size-FrameSize)/8)
}

func (protocol BrokerProtocol) Arguments(resume bool) ([]string, error) {
	var argument string
	switch protocol.purpose {
	case BrokerControl:
		argument = BrokerArgument
		if protocol.PinnedControl() {
			argument = BrokerPinnedArgument
		}
	case BrokerSuccessor:
		argument = BrokerGUIArgument
	default:
		return nil, ErrProtocol
	}
	args := []string{argument, hex.EncodeToString(protocol.binding[:]), "tun", "stopped"}
	if resume {
		args[3] = "resume"
	}
	if protocol.PinnedControl() {
		args = append(args, protocol.helperPin)
	}
	return args, nil
}

// BrokerInvocationRequested reserves both complete and malformed internal flags
// anywhere in argv. Until a platform dispatcher owns them, they must be terminal
// before any normal GUI/helper service. This does not parse or authorize a role.
func BrokerInvocationRequested(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-bootstrap-broker") || strings.HasPrefix(arg, "--internal-broker-gui") {
			return true
		}
	}
	return false
}

func ParseBrokerArguments(args []string) (protocol BrokerProtocol, resume bool, err error) {
	if (len(args) != 4 && len(args) != 5) || len(args[1]) != 64 || args[2] != "tun" || args[3] != "resume" && args[3] != "stopped" {
		return BrokerProtocol{}, false, ErrProtocol
	}
	var purpose BrokerPurpose
	switch args[0] {
	case BrokerArgument, BrokerPinnedArgument:
		purpose = BrokerControl
	case BrokerGUIArgument:
		purpose = BrokerSuccessor
	default:
		return BrokerProtocol{}, false, ErrProtocol
	}
	raw, decodeErr := hex.DecodeString(args[1])
	if decodeErr != nil || hex.EncodeToString(raw) != args[1] {
		return BrokerProtocol{}, false, ErrProtocol
	}
	var binding [32]byte
	copy(binding[:], raw)
	if args[0] == BrokerPinnedArgument {
		if len(args) != 5 {
			return BrokerProtocol{}, false, ErrProtocol
		}
		protocol, err = NewPinnedBrokerProtocol(binding, args[4])
		return protocol, args[3] == "resume", err
	}
	if len(args) != 4 {
		return BrokerProtocol{}, false, ErrProtocol
	}
	protocol, err = NewBrokerProtocol(purpose, binding)
	return protocol, args[3] == "resume", err
}

// Seed order is role-specific and cannot be interchanged:
//
//   - control: ancestor Job query, helper Job query, App query/sync, zero;
//   - successor: GUI Job query, helper query/sync, ancestor Job query, App query/sync.
//
// Nonzero entries must be distinct target-process values. The platform must
// still validate pointer width, object types, access, noninheritance, membership,
// fixed images, primary-user SID and pipe peer against held original handles.
func (protocol BrokerProtocol) EncodeSeed(handles [4]uint64) ([]byte, error) {
	size := protocol.seedSize()
	if size == 0 || !validTargetHandles(handles, (size-FrameSize)/8) {
		return nil, ErrProtocol
	}
	raw := make([]byte, size)
	frame := Frame(protocol.Stages().Seed, protocol.binding)
	copy(raw, frame[:])
	for index := 0; index < (size-FrameSize)/8; index++ {
		binary.LittleEndian.PutUint64(raw[FrameSize+index*8:], handles[index])
	}
	return raw, nil
}

func (protocol BrokerProtocol) DecodeSeed(raw []byte) ([4]uint64, error) {
	var handles [4]uint64
	size := protocol.seedSize()
	if size == 0 || len(raw) != size {
		return handles, ErrProtocol
	}
	want := Frame(protocol.Stages().Seed, protocol.binding)
	if !bytes.Equal(raw[:FrameSize], want[:]) {
		return handles, ErrProtocol
	}
	count := (size - FrameSize) / 8
	for index := 0; index < count; index++ {
		handles[index] = binary.LittleEndian.Uint64(raw[FrameSize+index*8:])
	}
	if !validTargetHandles(handles, count) {
		return [4]uint64{}, ErrProtocol // malformed input never authorizes Close
	}
	return handles, nil
}

func validTargetHandles(handles [4]uint64, count int) bool {
	if count < 2 || count > 4 {
		return false
	}
	for index, handle := range handles {
		if index >= count {
			if handle != 0 {
				return false
			}
			continue
		}
		if handle == 0 || handle >= ^uint64(15) {
			return false
		}
		for prior := 0; prior < index; prior++ {
			if handle == handles[prior] {
				return false
			}
		}
	}
	return true
}

// Exact lengths for this role only; legacy ValidExchange remains unchanged.
func (protocol BrokerProtocol) ValidExchange(send, receive int) bool {
	return validExchangeSize(send, receive, protocol.seedSize())
}
