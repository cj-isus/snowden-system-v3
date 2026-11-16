package delivery

import (
	"bytes"
	"encoding/json"
	"time"
)

// PublicationHead is public routing information, never an enrollment approval
// or application/TUF proof. Only a private accepted publisher checkpoint can
// authenticate a stored head; a downloader treats every field as untrusted.
type PublicationHead struct {
	Schema          int       `json:"schema"`
	ReleaseID       string    `json:"release_id"`
	CandidateSHA256 string    `json:"candidate_sha256"`
	Directory       string    `json:"directory"`
	RootSHA256      string    `json:"root_sha256"`
	TimestampSHA256 string    `json:"timestamp_sha256"`
	ValidUntil      time.Time `json:"valid_until"`
	Purpose         Kind      `json:"purpose,omitempty"` // absent is an ordinary application release
}

func DecodePublicationHead(raw []byte) (*PublicationHead, error) {
	if len(raw) == 0 || len(raw) > 16<<10 {
		return nil, ErrBudget
	}
	if err := strictJSON(raw); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) < 7 || len(fields) > 8 {
		return nil, ErrTrust
	}
	if value, exists := fields["purpose"]; exists {
		var purpose Kind
		if len(fields) != 8 || json.Unmarshal(value, &purpose) != nil || purpose != PinPolicy {
			return nil, ErrTrust
		}
	} else if len(fields) != 7 {
		return nil, ErrTrust
	}
	for _, name := range []string{"schema", "release_id", "candidate_sha256", "directory", "root_sha256", "timestamp_sha256", "valid_until"} {
		value, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrTrust
		}
	}
	var head PublicationHead
	if json.Unmarshal(raw, &head) != nil || head.Schema != 1 || !validHash(head.ReleaseID) || !validHash(head.CandidateSHA256) || !validHash(head.RootSHA256) || !validHash(head.TimestampSHA256) || head.Directory != "releases/"+head.ReleaseID || head.ValidUntil.IsZero() {
		return nil, ErrTrust
	}
	return &head, nil
}
