package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
)

const WebSocketServerAuthPurpose = "websocket_server_auth"

var ErrWebSocketTLSIdentity = errors.New("metadata: invalid WebSocket endpoint TLS identity")

// Only public leaf certificates are accepted. The signed wrapper binds this
// use of the common pin-value checks to one channel, purpose and service name.
type WebSocketTLSIdentity struct {
	Purpose   string      `json:"purpose"`
	ChannelID string      `json:"channel_id"`
	Hostname  string      `json:"hostname"`
	Pins      *LeafPinSet `json:"pins"`
}

func (identity *WebSocketTLSIdentity) Clone() *WebSocketTLSIdentity {
	if identity == nil {
		return nil
	}
	out := *identity
	out.Pins = identity.Pins.Clone()
	return &out
}

func (identity *WebSocketTLSIdentity) validate() error {
	if identity == nil || identity.Purpose != WebSocketServerAuthPurpose || !ValidChannelID(identity.ChannelID) || identity.Hostname == "" || identity.Pins == nil {
		return ErrWebSocketTLSIdentity
	}
	if _, err := identity.Pins.validate(identity.Hostname); err != nil {
		return err
	}
	return nil
}

func (identity *WebSocketTLSIdentity) UnmarshalJSON(data []byte) error {
	fields, err := decodeMetadataObject(data, maxWebSocketOptionsJSON, "purpose", "channel_id", "hostname", "pins")
	if err != nil || len(fields) != 4 {
		return ErrWebSocketTLSIdentity
	}
	for _, key := range []string{"purpose", "channel_id", "hostname", "pins"} {
		if bytes.Equal(bytes.TrimSpace(fields[key]), []byte("null")) {
			return ErrWebSocketTLSIdentity
		}
	}
	var out WebSocketTLSIdentity
	if json.Unmarshal(fields["purpose"], &out.Purpose) != nil || json.Unmarshal(fields["channel_id"], &out.ChannelID) != nil || json.Unmarshal(fields["hostname"], &out.Hostname) != nil || json.Unmarshal(fields["pins"], &out.Pins) != nil || out.validate() != nil {
		return ErrWebSocketTLSIdentity
	}
	*identity = out
	return nil
}

// SignedLeafPins returns already shape-checked values; it is not a proof or an
// alternate authority loader. An empty purpose is the legacy HY2 namespace.
func (channel Channel) SignedLeafPins() (*LeafPinSet, string) {
	if channel.Protocol == "hysteria2" && channel.Transport == "quic" {
		return channel.HY2Pins, ""
	}
	identity := channel.WS.TLS
	if channel.Protocol == "vless" && channel.Transport == "ws" && channel.WS.Version == WebSocketTLSOptionsVersion && identity != nil && identity.Purpose == WebSocketServerAuthPurpose && identity.ChannelID == channel.ID && identity.Hostname == channel.Hostname {
		return identity.Pins, identity.Purpose
	}
	return nil, ""
}
