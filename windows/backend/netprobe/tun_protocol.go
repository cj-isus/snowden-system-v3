package netprobe

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"strings"
)

const (
	TUNArgument     = "--snowden-native-tun-v1"
	TUNProtocol     = 1
	TUNProbeURL     = "https://1.1.1.1/cdn-cgi/trace"
	TUNProbeAddress = "1.1.1.1:443"
	MaxTUNRequest   = 1024
	MaxTUNMessage   = 8 << 10
	MaxTUNBody      = 4096
)

// These are correlated diagnostic values, never forwarding authority. The
// parent must establish the original image/process/Job, socket custody and
// actual engine/WFP scope independently of any decoded message.
type TUNRequest struct {
	Schema        int    `json:"schema"`
	Nonce         string `json:"nonce"`
	Subject       string `json:"subject"`
	Selector      string `json:"selector"`
	Epoch         uint64 `json:"epoch"`
	TimeoutMillis int    `json:"timeout_millis"`
}

type TUNMessage struct {
	Schema     int    `json:"schema"`
	Nonce      string `json:"nonce"`
	Subject    string `json:"subject"`
	Selector   string `json:"selector"`
	Epoch      uint64 `json:"epoch"`
	Stage      string `json:"stage"`
	SourcePort uint16 `json:"source_port"`
	HTTPStatus int    `json:"http_status"`
	Body       []byte `json:"body"`
	ErrorKind  string `json:"error_kind"`
}

func tunDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value
}

func (request TUNRequest) Validate() error {
	if request.Schema != TUNProtocol || !tunDigest(request.Nonce) || !tunDigest(request.Subject) || request.Epoch == 0 || request.TimeoutMillis < 500 || request.TimeoutMillis > 8000 || len(request.Selector) < 1 || len(request.Selector) > 128 {
		return ErrNativeProtocol
	}
	for _, c := range []byte(request.Selector) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", rune(c))) {
			return ErrNativeProtocol
		}
	}
	return nil
}

func NewTUNRequest(subject, selector string, epoch uint64, timeoutMillis int) (TUNRequest, error) {
	request := TUNRequest{Schema: TUNProtocol, Nonce: strings.Repeat("0", 64), Subject: subject, Selector: selector, Epoch: epoch, TimeoutMillis: timeoutMillis}
	if err := request.Validate(); err != nil {
		return TUNRequest{}, err
	}
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return TUNRequest{}, err
	}
	request.Nonce = hex.EncodeToString(entropy[:])
	return request, nil
}

func WriteTUNRequest(writer io.Writer, request TUNRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(request)
	if err != nil || len(raw) > MaxTUNRequest {
		return ErrNativeProtocol
	}
	return writeTUNFrame(writer, raw, MaxTUNRequest)
}

func ReadTUNRequest(reader io.Reader) (TUNRequest, error) {
	raw, err := readTUNFrame(reader, MaxTUNRequest)
	if err != nil {
		return TUNRequest{}, err
	}
	if _, err := nativeObject(raw, "schema", "nonce", "subject", "selector", "epoch", "timeout_millis"); err != nil {
		return TUNRequest{}, err
	}
	var request TUNRequest
	if json.Unmarshal(raw, &request) != nil {
		return TUNRequest{}, ErrNativeProtocol
	}
	return request, request.Validate()
}

func NewTUNMessage(request TUNRequest, stage string, port uint16) TUNMessage {
	return TUNMessage{Schema: TUNProtocol, Nonce: request.Nonce, Subject: request.Subject, Selector: request.Selector, Epoch: request.Epoch, Stage: stage, SourcePort: port, Body: []byte{}}
}

func (message TUNMessage) Validate(request TUNRequest) error {
	if request.Validate() != nil || message.Schema != TUNProtocol || message.Nonce != request.Nonce || message.Subject != request.Subject || message.Selector != request.Selector || message.Epoch != request.Epoch || message.SourcePort == 0 || message.Body == nil || len(message.Body) > MaxTUNBody {
		return ErrNativeProtocol
	}
	switch message.Stage {
	case "ready", "go", "release", "released":
		if message.HTTPStatus != 0 || len(message.Body) != 0 || message.ErrorKind != "" {
			return ErrNativeProtocol
		}
	case "result":
		if message.ErrorKind == "" {
			if message.HTTPStatus < 100 || message.HTTPStatus > 599 {
				return ErrNativeProtocol
			}
		} else {
			if message.HTTPStatus != 0 || len(message.Body) != 0 {
				return ErrNativeProtocol
			}
			switch message.ErrorKind {
			case "connect_unavailable", "tls_unavailable", "http_unavailable", "response_limit", "timeout", "canceled":
			default:
				return ErrNativeProtocol
			}
		}
	default:
		return ErrNativeProtocol
	}
	return nil
}

func WriteTUNMessage(writer io.Writer, message TUNMessage, request TUNRequest) error {
	if message.Validate(request) != nil {
		return ErrNativeProtocol
	}
	raw, err := json.Marshal(message)
	if err != nil || len(raw) == 0 || len(raw) > MaxTUNMessage {
		return ErrNativeProtocol
	}
	return writeTUNFrame(writer, raw, MaxTUNMessage)
}

func ReadTUNMessage(reader io.Reader, request TUNRequest) (TUNMessage, error) {
	if request.Validate() != nil {
		return TUNMessage{}, ErrNativeProtocol
	}
	raw, err := readTUNFrame(reader, MaxTUNMessage)
	if err != nil {
		return TUNMessage{}, err
	}
	fields, err := nativeObject(raw, "schema", "nonce", "subject", "selector", "epoch", "stage", "source_port", "http_status", "body", "error_kind")
	if err != nil {
		return TUNMessage{}, err
	}
	var body string
	if json.Unmarshal(fields["body"], &body) != nil || len(body) > base64.StdEncoding.EncodedLen(MaxTUNBody) {
		return TUNMessage{}, ErrNativeProtocol
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(body)
	if err != nil || len(decoded) > MaxTUNBody || base64.StdEncoding.EncodeToString(decoded) != body {
		return TUNMessage{}, ErrNativeProtocol
	}
	var message TUNMessage
	if json.Unmarshal(raw, &message) != nil {
		return TUNMessage{}, ErrNativeProtocol
	}
	return message, message.Validate(request)
}

func writeTUNFrame(writer io.Writer, raw []byte, limit int) error {
	if len(raw) == 0 || len(raw) > limit {
		return ErrNativeProtocol
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(raw)))
	if _, err := io.Copy(writer, bytes.NewReader(header[:])); err != nil {
		return err
	}
	_, err := io.Copy(writer, bytes.NewReader(raw))
	return err
}

func readTUNFrame(reader io.Reader, limit int) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > uint32(limit) {
		return nil, ErrNativeProtocol
	}
	raw := make([]byte, int(size))
	if _, err := io.ReadFull(reader, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// Only an observed HTTP fact, not a route verdict or a data-to-permit API.
// The parent must rederive this from the captured complete bounded response.
func (message TUNMessage) ObservedEgress(request TUNRequest) (net.IP, error) {
	if message.Validate(request) != nil || message.Stage != "result" || message.ErrorKind != "" || message.HTTPStatus != 200 {
		return nil, ErrNativeProtocol
	}
	var address net.IP
	count := 0
	for _, line := range strings.Split(string(message.Body), "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "ip="); found {
			count++
			address = net.ParseIP(strings.TrimSpace(value))
		}
	}
	if count != 1 || address == nil || address.To4() == nil {
		return nil, ErrNativeProtocol
	}
	return address.To4(), nil
}
