package core

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

// These are resource ceilings, not measured latency or throughput SLOs.
const (
	// V9 adds service-DNS pause/resume retirement, never forwarding authority.
	// Exact hello/argument matching prevents an old worker ACKing this boundary.
	EngineWorkerArgument  = "--internal-engine-worker-v9"
	workerProtocolVersion = 9
	maxWorkerConfigBytes  = 2 << 20
	maxWorkerRequestBytes = maxWorkerConfigBytes + 4096
	maxWorkerReplyBytes   = 64 << 10
	workerHelloTimeout    = 5 * time.Second
	workerStartTimeout    = 20 * time.Second
	workerControlTimeout  = 5 * time.Second
	workerCloseTimeout    = 2 * time.Second
	workerReapTimeout     = 4 * time.Second
)

var (
	ErrEngineWorkerUnavailable = errors.New("isolated engine unavailable")
	errWorkerProtocol          = errors.New("invalid engine worker protocol")
)

type workerRequest struct {
	Version              int                    `json:"version"`
	ID                   uint64                 `json:"id"`
	Action               string                 `json:"action"`
	Raw                  json.RawMessage        `json:"raw,omitempty"`
	Selector             string                 `json:"selector,omitempty"`
	Tag                  string                 `json:"tag,omitempty"`
	Subject              string                 `json:"subject,omitempty"`
	Probe                *workerProbeRequest    `json:"probe,omitempty"`
	AdmissionEpoch       uint64                 `json:"admission_epoch,omitempty"`
	AdmissionToken       string                 `json:"admission_token,omitempty"`
	TUNScope             *workerTUNScopeRequest `json:"tun_scope,omitempty"`
	ScopeNonce           string                 `json:"scope_nonce,omitempty"`
	NativeEgress         string                 `json:"native_egress,omitempty"`
	NativeAdmissionToken string                 `json:"native_admission_token,omitempty"`
	replyState           *workerStateReplyScope // parent-only original request expectation; never JSON
}

type workerReply struct {
	Version              int          `json:"version"`
	ID                   uint64       `json:"id"`
	Code                 string       `json:"code"`
	Previous             string       `json:"previous,omitempty"`
	Subject              string       `json:"subject,omitempty"`
	AdmissionEpoch       uint64       `json:"admission_epoch,omitempty"`
	Admitted             bool         `json:"admitted"`
	AdmissionToken       string       `json:"admission_token,omitempty"`
	NativeAdmissionToken string       `json:"native_admission_token,omitempty"`
	Report               *ProbeReport `json:"report,omitempty"`
	ScopeNonce           string       `json:"scope_nonce,omitempty"`
}

func writeWorkerFrame(w io.Writer, value any, limit int) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false) // avoid up to 6x expansion of a bounded raw config
	if err := encoder.Encode(value); err != nil || buffer.Len() == 0 || buffer.Len() > limit {
		return errWorkerProtocol
	}
	raw := buffer.Bytes()
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(raw)))
	if _, err := io.Copy(w, bytes.NewReader(header[:])); err != nil {
		return err
	}
	_, err := io.Copy(w, bytes.NewReader(raw))
	return err
}

func readWorkerFrame(r io.Reader, value any, limit int) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > uint32(limit) {
		return errWorkerProtocol
	}
	raw := make([]byte, int(size))
	if _, err := io.ReadFull(r, raw); err != nil {
		return err
	}
	if !validWorkerObject(raw, value) {
		return errWorkerProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errWorkerProtocol // never return untrusted values or config text
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errWorkerProtocol
	}
	return nil
}

// DisallowUnknownFields alone accepts case aliases and duplicate properties.
// Require one exact top-level command/reply spelling before typed decoding.
func validWorkerObject(raw []byte, target any) bool {
	var allowed string
	switch target.(type) {
	case *workerRequest:
		allowed = "|version|id|action|raw|selector|tag|subject|probe|admission_epoch|admission_token|tun_scope|scope_nonce|native_egress|native_admission_token|"
	case *workerReply:
		allowed = "|version|id|code|previous|subject|admission_epoch|admitted|admission_token|native_admission_token|report|scope_nonce|"
	case *workerProbeRequest:
		allowed = "|targets|dns_probe_targets|expected_egress|timeout_millis|tun|native_nonce|"
	case *workerTUNScopeRequest:
		allowed = "|actor_handle|actor_pid|actor_created|nonce|source_port|timeout_millis|"
	default:
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] || strings.Contains(name, "|") || !strings.Contains(allowed, "|"+name+"|") {
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
		if _, request := target.(*workerRequest); request && name == "probe" && !validWorkerObject(value, new(workerProbeRequest)) {
			return false
		}
		if _, request := target.(*workerRequest); request && name == "tun_scope" && !validWorkerObject(value, new(workerTUNScopeRequest)) {
			return false
		}
	}
	if _, scope := target.(*workerTUNScopeRequest); scope && len(seen) != 6 {
		return false
	}
	end, err := decoder.Token()
	return err == nil && end == json.Delim('}') && decoder.Decode(new(any)) == io.EOF
}

func validWorkerTag(tag string) bool {
	if tag == "" || len(tag) > 128 {
		return false
	}
	for _, c := range []byte(tag) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", rune(c))) {
			return false
		}
	}
	return true
}

func validWorkerRequest(r workerRequest, previous uint64) bool {
	if r.Version != workerProtocolVersion || r.ID == 0 || previous == ^uint64(0) || r.ID != previous+1 {
		return false
	}
	noScope := r.TUNScope == nil && r.ScopeNonce == "" && r.NativeEgress == "" && r.NativeAdmissionToken == ""
	ordinary := noScope && r.Subject == "" && r.Probe == nil && r.AdmissionEpoch == 0 && r.AdmissionToken == ""
	switch r.Action {
	case "start":
		return ordinary && len(r.Raw) > 0 && len(r.Raw) <= maxWorkerConfigBytes && r.Selector == "" && r.Tag == ""
	case "switch":
		return ordinary && len(r.Raw) == 0 && r.Selector == "proxy" && validWorkerTag(r.Tag)
	case "current":
		return ordinary && len(r.Raw) == 0 && r.Selector == "proxy" && r.Tag == ""
	case "probe":
		if !noScope || len(r.Raw) != 0 || r.Selector != "proxy" || !validWorkerTag(r.Tag) || !workerSubject(r.Subject) || r.Probe == nil || r.AdmissionEpoch != 0 || r.AdmissionToken != "" {
			return false
		}
		_, err := r.Probe.settings()
		return err == nil
	case "admit":
		return noScope && len(r.Raw) == 0 && r.Selector == "proxy" && validWorkerTag(r.Tag) && workerSubject(r.Subject) && r.Probe == nil && r.AdmissionEpoch != 0 && (r.AdmissionToken == "" || workerSubject(r.AdmissionToken))
	case "admit_native":
		return len(r.Raw) == 0 && r.Selector == "proxy" && validWorkerTag(r.Tag) && workerSubject(r.Subject) && r.Probe == nil && r.AdmissionEpoch != 0 &&
			r.AdmissionToken == "" && r.TUNScope == nil && r.NativeEgress == "" && workerSubject(r.ScopeNonce) && workerSubject(r.NativeAdmissionToken)
	case "dns_pause", "dns_resume":
		return noScope && len(r.Raw) == 0 && r.Selector == "proxy" && validWorkerTag(r.Tag) && workerSubject(r.Subject) && r.Probe == nil && r.AdmissionEpoch != 0 && r.AdmissionToken == ""
	case "tun_scope_open", "tun_scope_close":
		if len(r.Raw) != 0 || r.Selector != "proxy" || !validWorkerTag(r.Tag) || !workerSubject(r.Subject) || r.Probe != nil || r.AdmissionEpoch == 0 || r.AdmissionToken != "" || r.NativeEgress != "" || r.NativeAdmissionToken != "" {
			return false
		}
		if r.Action == "tun_scope_open" {
			return r.ScopeNonce == "" && r.TUNScope.valid()
		}
		return r.TUNScope == nil && workerSubject(r.ScopeNonce)
	case "tun_native_confirm":
		address, ok := nativeReportedAddress(r.NativeEgress)
		return ok && address.String() == r.NativeEgress && len(r.Raw) == 0 && r.Selector == "proxy" && validWorkerTag(r.Tag) && workerSubject(r.Subject) && r.Probe == nil && r.TUNScope == nil && r.AdmissionEpoch != 0 && r.AdmissionToken == "" && r.NativeAdmissionToken == "" && workerSubject(r.ScopeNonce)
	case "stop":
		return ordinary && len(r.Raw) == 0 && r.Selector == "" && r.Tag == ""
	default:
		return false
	}
}
