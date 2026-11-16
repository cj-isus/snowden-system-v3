package protocol

import (
	"errors"
	"io"
)

var errTCPResponse = errors.New("invalid HY2 TCP response framing")

// TCPResponseReader retains partial framing across a read deadline. One owner
// serializes Read; it never consumes payload bytes past the bounded response.
// Message storage is allocated only after its wire length has been checked.
type TCPResponseReader struct {
	phase       uint8
	status      [1]byte
	used        int
	integer     [8]byte
	integerUsed int
	integerWant int
	message     []byte
	messageText string
	padding     [256]byte
	remaining   uint64
	err         error
}

func fillTCPResponse(r io.Reader, target []byte, used *int) error {
	for *used < len(target) {
		n, err := r.Read(target[*used:])
		if n < 0 || n > len(target)-*used {
			return errTCPResponse
		}
		*used += n
		if *used == len(target) {
			return nil // same full-field completion rule as io.ReadFull
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}

func (r *TCPResponseReader) readInteger(input io.Reader) (uint64, error) {
	if r.integerWant == 0 {
		if err := fillTCPResponse(input, r.integer[:1], &r.integerUsed); err != nil {
			return 0, err
		}
		r.integerWant = 1 << (r.integer[0] >> 6)
	}
	if err := fillTCPResponse(input, r.integer[:r.integerWant], &r.integerUsed); err != nil {
		return 0, err
	}
	value := uint64(r.integer[0] & 0x3f)
	for _, b := range r.integer[1:r.integerWant] {
		value = value<<8 | uint64(b)
	}
	r.integerUsed, r.integerWant = 0, 0
	return value, nil
}

func (r *TCPResponseReader) Read(input io.Reader) (bool, string, error) {
	if r.err != nil {
		return false, "", r.err
	}
	for {
		switch r.phase {
		case 0:
			if err := fillTCPResponse(input, r.status[:], &r.used); err != nil {
				return false, "", err
			}
			if r.status[0] > 1 {
				r.err = errTCPResponse
				return false, "", r.err
			}
			r.phase, r.used = 1, 0
		case 1:
			length, err := r.readInteger(input)
			if err != nil {
				return false, "", err
			}
			if length > MaxMessageLength {
				r.err = errTCPResponse
				return false, "", r.err
			}
			r.message = make([]byte, int(length))
			r.phase = 2
		case 2:
			if err := fillTCPResponse(input, r.message, &r.used); err != nil {
				return false, "", err
			}
			r.phase, r.used = 3, 0
		case 3:
			length, err := r.readInteger(input)
			if err != nil {
				return false, "", err
			}
			if length > MaxPaddingLength {
				r.err = errTCPResponse
				return false, "", r.err
			}
			r.remaining, r.phase = length, 4
		case 4:
			for r.remaining != 0 {
				want := min(int(r.remaining), len(r.padding))
				n, err := input.Read(r.padding[:want])
				if n < 0 || n > want {
					r.err = errTCPResponse
					return false, "", r.err
				}
				r.remaining -= uint64(n)
				if r.remaining == 0 {
					break
				}
				if err != nil {
					return false, "", err
				}
				if n == 0 {
					return false, "", io.ErrNoProgress
				}
			}
			r.messageText = string(r.message)
			r.message, r.phase = nil, 5
		case 5:
			return r.status[0] == 0, r.messageText, nil
		}
	}
}
