// Package childbootstrap defines the fixed application handoff wire and child
// sequence. It acquires no native resources and grants no image, installation,
// elevation, network or process-tree authority. Callers retain original owners.
package childbootstrap

import "errors"

const FrameSize = 36
const SeedSize = FrameSize + 16
const AncestorSeedSize = FrameSize + 24

var ErrProtocol = errors.New("child bootstrap protocol unavailable")

type Purpose uint8

const (
	PurposeUpdate Purpose = iota + 1
	PurposeRestart
	PurposeElevation
	PurposeUpdateAncestor
	PurposeRestartAncestor
)

// Protocol binds one fixed purpose to exact bytes. A zero Protocol is invalid.
// The binding is correlation only; callers independently authenticate its source.
type Protocol struct {
	purpose Purpose
	binding [32]byte
}

type Stages struct {
	Seed, Armed, Go, Owned, Use, Ack string
}

func New(purpose Purpose, binding [32]byte) (Protocol, error) {
	if !validPurpose(purpose) {
		return Protocol{}, ErrProtocol
	}
	return Protocol{purpose: purpose, binding: binding}, nil
}

func validPurpose(purpose Purpose) bool {
	return purpose == PurposeUpdate || purpose == PurposeRestart || purpose == PurposeElevation ||
		purpose == PurposeUpdateAncestor || purpose == PurposeRestartAncestor
}

// Stages returns wire data only. These names never authorize a native operation.
func (protocol Protocol) Stages() Stages {
	switch protocol.purpose {
	case PurposeUpdate:
		return Stages{Seed: "SE06", Armed: "ARM6", Go: "GO06", Owned: "OWN6", Use: "USE6", Ack: "ACK6"}
	case PurposeRestart:
		return Stages{Seed: "SR01", Armed: "AR01", Go: "GR01", Owned: "OR01", Use: "UR01", Ack: "KR01"}
	case PurposeElevation:
		return Stages{Seed: "SEU1", Armed: "ARU1", Go: "GOU1", Owned: "OWU1", Use: "USU1", Ack: "AKU1"}
	case PurposeUpdateAncestor:
		return Stages{Seed: "SE07", Armed: "ARM7", Go: "GO07", Owned: "OWN7", Use: "USE7", Ack: "ACK7"}
	case PurposeRestartAncestor:
		return Stages{Seed: "SR02", Armed: "AR02", Go: "GR02", Owned: "OR02", Use: "UR02", Ack: "KR02"}
	default:
		return Stages{}
	}
}

// Frame formats bytes only, preserving the existing four-byte stage encoding.
// Authenticating the binding and accepting a stage remain caller responsibilities.
func Frame(stage string, binding [32]byte) [FrameSize]byte {
	var frame [FrameSize]byte
	copy(frame[:4], stage)
	copy(frame[4:], binding[:])
	return frame
}

func ValidExchange(send, receive int) bool {
	return validExchangeSize(send, receive, SeedSize)
}

func validExchangeSize(send, receive, seedSize int) bool {
	return seedSize > 0 && (send == seedSize && receive == FrameSize || // parent seed/ARM
		send == FrameSize && receive == FrameSize || // ARM/GO, GO/OWN, OWN/USE, USE/ACK
		send == 0 && receive == seedSize || // child seed
		send == FrameSize && receive == 0) // child ACK
}
