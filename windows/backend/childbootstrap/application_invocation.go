package childbootstrap

import "encoding/hex"

const (
	ApplicationUpdateArgument          = "--internal-update-gui-v6"
	ApplicationRestartArgument         = "--internal-restart-gui-v1"
	ApplicationElevationArgument       = "--internal-elevated-gui-v1"
	ApplicationUpdateAncestorArgument  = "--internal-update-gui-v7"
	ApplicationRestartAncestorArgument = "--internal-restart-gui-v2"
)

// NewApplicationProtocol binds only a fixed versioned application role. Argument
// and binding values are correlation data, never image/elevation/SDK authority.
func NewApplicationProtocol(argument string, binding [32]byte) (Protocol, error) {
	var purpose Purpose
	switch argument {
	case ApplicationUpdateArgument:
		purpose = PurposeUpdate
	case ApplicationRestartArgument:
		purpose = PurposeRestart
	case ApplicationElevationArgument:
		purpose = PurposeElevation
	case ApplicationUpdateAncestorArgument:
		purpose = PurposeUpdateAncestor
	case ApplicationRestartAncestorArgument:
		purpose = PurposeRestartAncestor
	default:
		return Protocol{}, ErrProtocol
	}
	return New(purpose, binding)
}

func (protocol Protocol) ApplicationArguments(tun, resume bool) ([]string, error) {
	var argument string
	switch protocol.purpose {
	case PurposeUpdate:
		argument = ApplicationUpdateArgument
	case PurposeRestart:
		argument = ApplicationRestartArgument
	case PurposeUpdateAncestor:
		argument = ApplicationUpdateAncestorArgument
	case PurposeRestartAncestor:
		argument = ApplicationRestartAncestorArgument
	case PurposeElevation:
		if !tun {
			return nil, ErrProtocol
		}
		argument = ApplicationElevationArgument
	default:
		return nil, ErrProtocol
	}
	args := []string{argument, hex.EncodeToString(protocol.binding[:]), "proxy", "stopped"}
	if tun {
		args[2] = "tun"
	}
	if resume {
		args[3] = "resume"
	}
	return args, nil // caller-owned four words, no generic executable/extra-argv mode
}

func ParseApplicationArguments(args []string) (protocol Protocol, tun, resume bool, err error) {
	if len(args) != 4 || len(args[1]) != 64 || args[2] != "tun" && args[2] != "proxy" || args[3] != "resume" && args[3] != "stopped" {
		return Protocol{}, false, false, ErrProtocol
	}
	raw, err := hex.DecodeString(args[1])
	if err != nil || hex.EncodeToString(raw) != args[1] {
		return Protocol{}, false, false, ErrProtocol
	}
	var binding [32]byte
	copy(binding[:], raw)
	protocol, err = NewApplicationProtocol(args[0], binding)
	if err != nil {
		return Protocol{}, false, false, err
	}
	tun, resume = args[2] == "tun", args[3] == "resume"
	if protocol.purpose == PurposeElevation && !tun {
		return Protocol{}, false, false, ErrProtocol
	}
	return protocol, tun, resume, nil
}
