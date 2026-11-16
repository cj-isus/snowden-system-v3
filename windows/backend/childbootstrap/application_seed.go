package childbootstrap

import "encoding/binary"

func (protocol Protocol) NeedsAncestor() bool {
	return protocol.purpose == PurposeUpdateAncestor || protocol.purpose == PurposeRestartAncestor
}

func (protocol Protocol) SeedSize() int {
	if !validPurpose(protocol.purpose) {
		return 0
	}
	if protocol.NeedsAncestor() {
		return AncestorSeedSize
	}
	return SeedSize
}

func (protocol Protocol) ValidExchange(send, receive int) bool {
	return validExchangeSize(send, receive, protocol.SeedSize())
}

// These are target-process values only: own GUI Job, original parent process,
// optional retained ancestor Job. Legacy callers must supply zero in slot3.
// Shape/binding do not authenticate an image, principal, Job or original intent.
func (protocol Protocol) EncodeSeed(handles [3]uint64) ([]byte, error) {
	size := protocol.SeedSize()
	if size == 0 || !validTargetHandles([4]uint64{handles[0], handles[1], handles[2], 0}, (size-FrameSize)/8) {
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

func ValidAncestorHandles(handles [3]uint64) bool {
	return validTargetHandles([4]uint64{handles[0], handles[1], handles[2], 0}, 3)
}
