package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"time"
)

const MaxWorkerRequestBytes = 20 << 20
const MaxWorkerReplyBytes = 6 << 20

const workerRequestMagic = "SNTUFQ01"
const workerReplyMagic = "SNTUFR01"

const (
	workerErrTrust uint8 = 1 << iota
	workerErrBudget
	workerErrCanceled
	workerErrDeadline
)

type workerArtifact struct {
	Kind  Kind   `json:"kind"`
	Path  string `json:"path"`
	Bytes []byte `json:"bytes"`
}

type workerInput struct {
	Schema   int              `json:"schema"`
	Nonce    string           `json:"nonce"`
	State    *State           `json:"state"`
	Package  *Package         `json:"package"`
	Requests []workerArtifact `json:"requests"`
	Now      time.Time        `json:"now"`
}

type workerProof struct {
	Kind              Kind      `json:"kind"`
	Path              string    `json:"path"`
	SHA256            string    `json:"sha256"`
	RootVersion       int64     `json:"root_version"`
	Expires           time.Time `json:"expires"`
	PinPolicyRequired bool      `json:"pin_policy_required"`
}

type workerResult struct {
	Schema int           `json:"schema"`
	Source string        `json:"source"`
	Status uint8         `json:"status"`
	Next   *State        `json:"next"`
	Proofs []workerProof `json:"proofs"`
}

func decodeWorkerJSON(raw []byte, target any) error {
	if err := strictJSON(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return ErrTrust
	}
	// Both endpoints use the same compiled encoder. Exact round-trip also
	// rejects case aliases, omitted fields and alternate wire encodings.
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(canonical, raw) {
		return ErrTrust
	}
	return nil
}

func workerWrite(ctx context.Context, writer io.Writer, data []byte) error {
	for len(data) != 0 {
		if err := contextError(ctx); err != nil {
			return err
		}
		chunk := data[:min(len(data), 32<<10)]
		n, err := writer.Write(chunk)
		if n < 0 || n > len(chunk) || err != nil || n == 0 {
			return ErrTrust
		}
		data = data[n:]
	}
	return nil
}

func workerReadFrame(reader io.Reader, magic string, limit int) (payload []byte, tag [32]byte, err error) {
	var header [44]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil || string(header[:8]) != magic {
		return nil, tag, ErrTrust
	}
	size := binary.BigEndian.Uint32(header[8:12])
	if size == 0 || uint64(size) > uint64(limit) {
		return nil, tag, ErrBudget
	}
	copy(tag[:], header[12:])
	payload = make([]byte, int(size)) // bound established before allocation
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, tag, ErrTrust
	}
	var tail [1]byte
	if n, err := reader.Read(tail[:]); n != 0 || err != io.EOF {
		return nil, tag, ErrTrust // one frame and exact EOF, never a trailing command
	}
	return payload, tag, nil
}

func workerWriteFrame(ctx context.Context, writer io.Writer, magic string, payload []byte, tag [32]byte, limit int) error {
	if len(payload) == 0 || len(payload) > limit {
		return ErrBudget
	}
	var header [44]byte
	copy(header[:8], magic)
	binary.BigEndian.PutUint32(header[8:12], uint32(len(payload)))
	copy(header[12:], tag[:])
	if err := workerWrite(ctx, writer, header[:]); err != nil {
		return err
	}
	return workerWrite(ctx, writer, payload)
}

func workerResultMAC(key [32]byte, request [32]byte, raw []byte) [32]byte {
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte("snowden-sdk-worker-result-v1\x00"))
	_, _ = mac.Write(request[:])
	_, _ = mac.Write(raw)
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}
