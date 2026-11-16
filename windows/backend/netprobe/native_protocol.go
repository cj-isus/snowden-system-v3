package netprobe

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"
)

const (
	NativeProtocol            = 1
	NativeHelperName          = "snowden-native-probe.exe"
	NativeArgument            = "--snowden-native-dns-v1"
	MaxNativeRequest          = 2048
	MaxNativeReply            = 56 << 10
	MaxNativeAttempts         = 8
	MaxNativeImageBytes int64 = 32 << 20
)

var ErrNativeProtocol = errors.New("invalid bounded native observation protocol")

// Public measurement inputs only. Endpoints/name/port are fixed in this helper
// version; neither profiles nor command arguments select arbitrary raw packets.
type NativeRequest struct {
	Schema        int      `json:"schema"`
	Nonce         string   `json:"nonce"`
	Networks      []string `json:"networks"`
	QueryIDs      []uint16 `json:"query_ids"`
	TimeoutMillis int      `json:"timeout_millis"`
}

type NativeResult struct {
	Network       string `json:"network"`
	ServerIP      string `json:"server_ip"`
	QueryID       uint16 `json:"query_id"`
	Stage         string `json:"stage"`
	ErrorKind     string `json:"error_kind"`
	Packet        []byte `json:"packet"`
	ElapsedMicros int64  `json:"elapsed_micros"`
}

// Complete means the requested bounded batch finished, not all transports work.
// The helper never asserts a protected route, epoch, egress or application health.
type NativeReply struct {
	Schema   int            `json:"schema"`
	Nonce    string         `json:"nonce"`
	Complete bool           `json:"complete"`
	Results  []NativeResult `json:"results"`
}

func (reply NativeReply) Clone() NativeReply {
	reply.Results = slices.Clone(reply.Results)
	for index := range reply.Results {
		reply.Results[index].Packet = slices.Clone(reply.Results[index].Packet)
	}
	return reply
}

func NativeEndpoints(network string) []string {
	switch network {
	case "udp4", "tcp4":
		return []string{"1.1.1.1", "8.8.8.8"}
	case "udp6", "tcp6":
		return []string{"2606:4700:4700::1111", "2001:4860:4860::8888"}
	}
	return nil
}

func (r NativeRequest) Clone() NativeRequest {
	r.Networks, r.QueryIDs = slices.Clone(r.Networks), slices.Clone(r.QueryIDs)
	return r
}

func (r NativeRequest) Validate() error {
	if r.Schema != NativeProtocol || len(r.Nonce) != 64 || len(r.Networks) < 1 || len(r.Networks) > 4 || len(r.QueryIDs) != 2*len(r.Networks) || r.TimeoutMillis < 500 || r.TimeoutMillis > 12000 {
		return ErrNativeProtocol
	}
	nonce, err := hex.DecodeString(r.Nonce)
	if err != nil || len(nonce) != 32 || hex.EncodeToString(nonce) != r.Nonce {
		return ErrNativeProtocol
	}
	networks, ids := map[string]bool{}, map[uint16]bool{}
	for _, network := range r.Networks {
		if len(NativeEndpoints(network)) != 2 || networks[network] {
			return ErrNativeProtocol
		}
		networks[network] = true
	}
	for _, id := range r.QueryIDs {
		if ids[id] {
			return ErrNativeProtocol
		}
		ids[id] = true
	}
	return nil
}

func NewNativeRequest(networks []string, timeoutMillis int) (NativeRequest, error) {
	if len(networks) < 1 || len(networks) > 4 || timeoutMillis < 500 || timeoutMillis > 12000 {
		return NativeRequest{}, ErrNativeProtocol
	}
	seen := map[string]bool{}
	for _, network := range networks {
		if len(NativeEndpoints(network)) != 2 || seen[network] {
			return NativeRequest{}, ErrNativeProtocol
		}
		seen[network] = true
	}
	var entropy [34]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return NativeRequest{}, err
	}
	request := NativeRequest{Schema: NativeProtocol, Nonce: hex.EncodeToString(entropy[:32]), Networks: slices.Clone(networks), TimeoutMillis: timeoutMillis}
	seed := binary.BigEndian.Uint16(entropy[32:])
	for i := 0; i < 2*len(networks); i++ {
		request.QueryIDs = append(request.QueryIDs, seed+uint16(i))
	}
	return request, request.Validate()
}

func EncodeNativeRequest(request NativeRequest) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	raw, err := json.Marshal(request)
	if err != nil || len(raw) > MaxNativeRequest {
		return "", ErrNativeProtocol
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeNativeRequest(token string) (NativeRequest, error) {
	if len(token) < 1 || len(token) > base64.RawURLEncoding.EncodedLen(MaxNativeRequest) {
		return NativeRequest{}, ErrNativeProtocol
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) > MaxNativeRequest || base64.RawURLEncoding.EncodeToString(raw) != token {
		return NativeRequest{}, ErrNativeProtocol
	}
	fields, err := nativeObject(raw, "schema", "nonce", "networks", "query_ids", "timeout_millis")
	if err != nil {
		return NativeRequest{}, err
	}
	if _, err = nativeArray(fields["networks"], 4); err != nil {
		return NativeRequest{}, err
	}
	if _, err = nativeArray(fields["query_ids"], MaxNativeAttempts); err != nil {
		return NativeRequest{}, err
	}
	var request NativeRequest
	if json.Unmarshal(raw, &request) != nil {
		return NativeRequest{}, ErrNativeProtocol
	}
	return request, request.Validate()
}

func EncodeNativeReply(reply NativeReply, request NativeRequest) ([]byte, error) {
	if err := reply.Validate(request); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(reply)
	if err != nil || len(raw) > MaxNativeReply {
		return nil, ErrNativeProtocol
	}
	return raw, nil
}

func DecodeNativeReply(raw []byte, request NativeRequest) (NativeReply, error) {
	if len(raw) < 1 || len(raw) > MaxNativeReply {
		return NativeReply{}, ErrNativeProtocol
	}
	fields, err := nativeObject(raw, "schema", "nonce", "complete", "results")
	if err != nil {
		return NativeReply{}, err
	}
	records, err := nativeArray(fields["results"], MaxNativeAttempts)
	if err != nil {
		return NativeReply{}, err
	}
	for _, record := range records {
		fields, err := nativeObject(record, "network", "server_ip", "query_id", "stage", "error_kind", "packet", "elapsed_micros")
		if err != nil {
			return NativeReply{}, err
		}
		var encoded string
		if json.Unmarshal(fields["packet"], &encoded) != nil || len(encoded) > base64.StdEncoding.EncodedLen(MaxDNSMessage) {
			return NativeReply{}, ErrNativeProtocol
		}
		packet, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(packet) > MaxDNSMessage || base64.StdEncoding.EncodeToString(packet) != encoded {
			return NativeReply{}, ErrNativeProtocol
		}
	}
	var reply NativeReply
	if json.Unmarshal(raw, &reply) != nil {
		return NativeReply{}, ErrNativeProtocol
	}
	return reply, reply.Validate(request)
}

func (reply NativeReply) Validate(request NativeRequest) error {
	if request.Validate() != nil || reply.Schema != NativeProtocol || reply.Nonce != request.Nonce || reply.Results == nil || len(reply.Results) > 2*len(request.Networks) {
		return ErrNativeProtocol
	}
	position := 0
	for index, network := range request.Networks {
		attempts, succeeded := 0, false
		for position < len(reply.Results) && reply.Results[position].Network == network && attempts < 2 && !succeeded {
			result := reply.Results[position]
			if result.ServerIP != NativeEndpoints(network)[attempts] || result.QueryID != request.QueryIDs[2*index+attempts] || result.Packet == nil || len(result.Packet) > MaxDNSMessage || result.ElapsedMicros < 0 || result.ElapsedMicros > int64(request.TimeoutMillis)*1000 {
				return ErrNativeProtocol
			}
			switch result.ErrorKind {
			case "":
				if result.Stage != "response" || NativeDNSResponseKind(result.Packet, result.QueryID) != "" {
					return ErrNativeProtocol
				}
				succeeded = true
			case "invalid_dns_response", "dns_answer_missing", "dns_truncated":
				if result.Stage != "response" || NativeDNSResponseKind(result.Packet, result.QueryID) != result.ErrorKind {
					return ErrNativeProtocol
				}
			case "canceled", "timeout":
				if len(result.Packet) != 0 || result.Stage != "dial" && result.Stage != "write" && result.Stage != "read" {
					return ErrNativeProtocol
				}
			case "dial_unavailable", "write_incomplete", "read_unavailable", "message_limit":
				stages := map[string]string{"dial_unavailable": "dial", "write_incomplete": "write", "read_unavailable": "read", "message_limit": "read"}
				if len(result.Packet) != 0 || result.Stage != stages[result.ErrorKind] {
					return ErrNativeProtocol
				}
			default:
				return ErrNativeProtocol
			}
			position++
			attempts++
		}
		if !succeeded && attempts < 2 {
			if reply.Complete || position != len(reply.Results) {
				return ErrNativeProtocol
			}
			return nil // only an incomplete prefix is allowed after cancellation
		}
	}
	if position != len(reply.Results) {
		return ErrNativeProtocol
	}
	return nil
}

// NativeDNSResponseKind is shared by the helper and the receiver. The receiver
// rechecks the bounded raw packet instead of trusting the helper's DNS verdict.
func NativeDNSResponseKind(packet []byte, id uint16) string {
	_, err := DecodeDNSResponse(packet, id)
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrDNSAnswerMissing):
		return "dns_answer_missing"
	case errors.Is(err, ErrDNSTruncated):
		return "dns_truncated"
	default:
		return "invalid_dns_response"
	}
}

func nativeObject(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrNativeProtocol
	}
	allowed, fields := map[string]bool{}, map[string]json.RawMessage{}
	for _, name := range names {
		allowed[name] = true
	}
	for decoder.More() {
		token, err = decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] || fields[name] != nil {
			return nil, ErrNativeProtocol
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrNativeProtocol
		}
		fields[name] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || len(fields) != len(allowed) {
		return nil, ErrNativeProtocol
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, ErrNativeProtocol
	}
	return fields, nil
}

func nativeArray(raw []byte, limit int) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, ErrNativeProtocol
	}
	values := []json.RawMessage{}
	for decoder.More() {
		if len(values) >= limit {
			return nil, ErrNativeProtocol
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrNativeProtocol
		}
		values = append(values, value)
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim(']') {
		return nil, ErrNativeProtocol
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, ErrNativeProtocol
	}
	return values, nil
}
