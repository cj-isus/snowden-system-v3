package wire

import (
	"errors"
	"io"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/quicvarint"
)

// A NewTokenFrame is a NEW_TOKEN frame
type NewTokenFrame struct {
	Token []byte
}

func parseNewTokenFrame(b []byte, _ protocol.Version) (*NewTokenFrame, int, error) {
	frame, n, err := parseNewTokenFrameView(b)
	if err != nil {
		return nil, n, err
	}
	token := make([]byte, len(frame.Token))
	copy(token, frame.Token)
	frame.Token = token
	return frame, n, nil
}

// The native caller retains the packet until optional cache admission/copy.
func parseNewTokenFrameView(b []byte) (*NewTokenFrame, int, error) {
	tokenLen, l, err := quicvarint.Parse(b)
	if err != nil {
		return nil, 0, replaceUnexpectedEOF(err)
	}
	b = b[l:]
	if tokenLen == 0 {
		return nil, 0, errors.New("token must not be empty")
	}
	if uint64(len(b)) < tokenLen {
		return nil, 0, io.EOF
	}
	n := int(tokenLen) // tokenLen <= len(b), so the conversion/addition are bounded
	return &NewTokenFrame{Token: b[:n:n]}, l + n, nil
}

func (f *NewTokenFrame) Append(b []byte, _ protocol.Version) ([]byte, error) {
	b = append(b, byte(FrameTypeNewToken))
	b = quicvarint.Append(b, uint64(len(f.Token)))
	b = append(b, f.Token...)
	return b, nil
}

// Length of a written frame
func (f *NewTokenFrame) Length(protocol.Version) protocol.ByteCount {
	return 1 + protocol.ByteCount(quicvarint.Len(uint64(len(f.Token)))+len(f.Token))
}
