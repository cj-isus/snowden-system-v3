package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

const (
	WebSocketOptionsVersion     = 1
	WebSocketTLSOptionsVersion  = 2
	MaxWebSocketPathBytes       = 2048
	maxWebSocketPathOptionsJSON = 16 << 10
	maxWebSocketOptionsJSON     = 112 << 10 // bounded path + exact TLS wrapper/pin object; parent envelope caps unchanged
	legacyWebSocketPath         = "/ws"
)

var ErrWebSocketOptions = errors.New("metadata: invalid or unsupported WebSocket options")

// These owned values are authenticated by the complete Channel and
// envelope, not by a runtime override. Zero is the legacy, omitted representation.
// Version 1 carries only a path; version 2 requires purpose-bound endpoint TLS.
type WebSocketOptions struct {
	Version int                   `json:"version"`
	Path    string                `json:"path"`
	TLS     *WebSocketTLSIdentity `json:"tls,omitempty"`
}

func (options WebSocketOptions) IsZero() bool {
	return options.Version == 0 && options.Path == "" && options.TLS == nil
}

func (options WebSocketOptions) Clone() WebSocketOptions {
	options.TLS = options.TLS.Clone()
	return options
}

func (options WebSocketOptions) Validate() error {
	if options.IsZero() {
		return nil
	}
	switch options.Version {
	case WebSocketOptionsVersion:
		if options.TLS != nil {
			return ErrWebSocketOptions
		}
	case WebSocketTLSOptionsVersion:
		if err := options.TLS.validate(); err != nil {
			return err
		}
	default:
		return ErrWebSocketOptions
	}
	path := options.Path
	if len(path) == 0 || len(path) > MaxWebSocketPathBytes || path[0] != '/' || strings.HasPrefix(path, "//") {
		return ErrWebSocketOptions
	}
	for i := 0; i < len(path); i++ {
		if path[i] <= 0x20 || path[i] >= 0x7f || path[i] == '?' || path[i] == '#' || path[i] == '\\' {
			return ErrWebSocketOptions
		}
	}
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return ErrWebSocketOptions // never include the possibly private path in an error
	}
	for i := 0; i < len(decoded); i++ {
		if decoded[i] < 0x20 || decoded[i] == 0x7f || decoded[i] == '\\' {
			return ErrWebSocketOptions
		}
	}
	// The pinned SDK passes its path to net/url.setPath, not URL.Parse. A raw
	// query would become path text. Only accept a representation EscapedPath
	// preserves byte-for-byte; never clean, normalize or re-encode signed bytes.
	wire := url.URL{Path: decoded, RawPath: path}
	if wire.EscapedPath() != path {
		return ErrWebSocketOptions
	}
	return nil
}

func (options WebSocketOptions) EffectivePath() (string, error) {
	if err := options.Validate(); err != nil {
		return "", err
	}
	if options.IsZero() {
		return legacyWebSocketPath, nil
	}
	return options.Path, nil
}

func (options *WebSocketOptions) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || len(data) > maxWebSocketOptionsJSON {
		return ErrWebSocketOptions
	}
	fields, err := decodeMetadataObject(data, maxWebSocketOptionsJSON, "version", "path", "tls")
	if err != nil || len(fields) < 2 {
		return ErrWebSocketOptions
	}
	for _, key := range []string{"version", "path"} {
		if bytes.Equal(bytes.TrimSpace(fields[key]), []byte("null")) {
			return ErrWebSocketOptions
		}
	}
	var out WebSocketOptions
	if json.Unmarshal(fields["version"], &out.Version) != nil || json.Unmarshal(fields["path"], &out.Path) != nil {
		return ErrWebSocketOptions
	}
	switch out.Version {
	case WebSocketOptionsVersion:
		if len(fields) != 2 || len(data) > maxWebSocketPathOptionsJSON {
			return ErrWebSocketOptions
		}
	case WebSocketTLSOptionsVersion:
		if len(fields) != 3 || bytes.Equal(bytes.TrimSpace(fields["tls"]), []byte("null")) || json.Unmarshal(fields["tls"], &out.TLS) != nil {
			return ErrWebSocketOptions
		}
	default:
		return ErrWebSocketOptions
	}
	if out.Validate() != nil {
		return ErrWebSocketOptions
	}
	*options = out // no partial publication; a present null/empty object never means legacy absence
	return nil
}
