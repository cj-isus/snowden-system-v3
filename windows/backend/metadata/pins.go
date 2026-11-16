package metadata

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
)

const MaxPinCertificateSize = 16 << 10
const maxPinSetJSONSize = 96 << 10

var ErrNoActivePin = errors.New("metadata: no active HY2 certificate pin")

// CertificatePin authenticates a single leaf certificate, not a delegated CA.
// SHA256 is lowercase hex of DER bytes, independent of PEM line endings.
type CertificatePin struct {
	SHA256      string `json:"sha256"`
	Certificate string `json:"certificate"`
}

// HY2PinSet is bound to Channel.ID and Hostname by the envelope signature.
// Next overlaps Current. Revoked fingerprints are excluded before rendering.
// A full replacement travels in a newer signed metadata version.
type HY2PinSet struct {
	Current CertificatePin  `json:"current"`
	Next    *CertificatePin `json:"next,omitempty"`
	Revoked []string        `json:"revoked,omitempty"`
}

// LeafPinSet names the reusable certificate-value checks. It does not transfer
// HY2 channel authority: each consumer must bind its own purpose/ID/hostname.
// The alias preserves the existing HY2 Go API, wire shape and error identities.
type LeafPinSet = HY2PinSet

func (p *HY2PinSet) Clone() *HY2PinSet {
	if p == nil {
		return nil
	}
	out := *p
	out.Revoked = slices.Clone(p.Revoked)
	if p.Next != nil {
		next := *p.Next
		out.Next = &next
	}
	return &out
}

// NewCertificatePin is a publisher helper; it reads no files or keys.
// Channel hostname, EKU and rotation checks belong to the complete policy.
func NewCertificatePin(data []byte) (CertificatePin, error) {
	cert, err := parsePinCertificate(string(data))
	if err != nil {
		return CertificatePin{}, err
	}
	sum := sha256.Sum256(cert.Raw)
	return CertificatePin{SHA256: hex.EncodeToString(sum[:]), Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))}, nil
}

func parsePinCertificate(data string) (*x509.Certificate, error) {
	if len(data) == 0 || len(data) > MaxPinCertificateSize {
		return nil, errors.New("metadata: invalid pin certificate size")
	}
	trimmed := bytes.TrimSpace([]byte(data))
	// pem.Decode skips malformed leading blocks. Check the whole input before
	// decoding so a damaged block cannot hide material before a valid leaf.
	if !bytes.HasPrefix(trimmed, []byte("-----BEGIN CERTIFICATE-----")) || bytes.Count(trimmed, []byte("-----BEGIN ")) != 1 {
		return nil, errors.New("metadata: expected exactly one certificate PEM")
	}
	block, rest := pem.Decode(trimmed)
	if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("metadata: pin must contain exactly one certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("metadata: malformed pin certificate")
	}
	if cert.IsCA || !cert.NotAfter.After(cert.NotBefore) {
		return nil, errors.New("metadata: pin must be a leaf with a valid interval")
	}
	return cert, nil
}

func (p *HY2PinSet) validate(host string) ([]*x509.Certificate, error) {
	if p == nil {
		return nil, errors.New("metadata: missing HY2 pin policy")
	}
	entries := []CertificatePin{p.Current}
	if p.Next != nil {
		entries = append(entries, *p.Next)
	}
	certs := make([]*x509.Certificate, 0, len(entries))
	ids := make(map[string]bool, len(entries))
	for _, entry := range entries {
		cert, err := parsePinCertificate(entry.Certificate)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(cert.Raw)
		if entry.SHA256 != hex.EncodeToString(sum[:]) || ids[entry.SHA256] {
			return nil, errors.New("metadata: mismatched or duplicate pin fingerprint")
		}
		// Verify at issuance for static hostname/EKU/critical-extension checks.
		// Runtime uses the real clock and standard x509 verification again.
		roots := x509.NewCertPool()
		roots.AddCert(cert)
		if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: host, CurrentTime: cert.NotBefore, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
			return nil, errors.New("metadata: pin certificate does not authorize channel hostname/server-auth")
		}
		ids[entry.SHA256] = true
		certs = append(certs, cert)
	}
	if len(certs) == 2 && (certs[1].NotBefore.Before(certs[0].NotBefore) || !certs[1].NotAfter.After(certs[0].NotAfter) || !certs[0].NotAfter.After(certs[1].NotBefore)) {
		return nil, errors.New("metadata: next pin must extend current with overlap")
	}
	if len(p.Revoked) > len(entries) {
		return nil, errors.New("metadata: too many pin revocations")
	}
	seen := make(map[string]bool, len(p.Revoked))
	for _, id := range p.Revoked {
		if !ids[id] || seen[id] {
			return nil, errors.New("metadata: unknown or duplicate pin revocation")
		}
		seen[id] = true
	}
	return certs, nil
}

// Certificates includes an authenticated future Next so an existing TLS
// config accepts it when its own NotBefore arrives. Expired/revoked entries
// are excluded. At least one non-revoked certificate must work now.
func (p *HY2PinSet) Certificates(host string, now time.Time) ([]string, error) {
	if p == nil {
		return nil, ErrNoActivePin
	}
	certs, err := p.validate(host)
	if err != nil {
		return nil, err
	}
	entries := []CertificatePin{p.Current}
	if p.Next != nil {
		entries = append(entries, *p.Next)
	}
	var out []string
	active := false
	for i, cert := range certs {
		if slices.Contains(p.Revoked, entries[i].SHA256) || !cert.NotAfter.After(now) {
			continue
		}
		out = append(out, entries[i].Certificate)
		active = active || !now.Before(cert.NotBefore)
	}
	if !active {
		return nil, ErrNoActivePin
	}
	return out, nil
}

// SessionDeadline cannot identify which allowed leaf an existing connection
// authenticated. Recycle by the earliest active allowed leaf's NotAfter rather
// than borrowing a future Next's lifetime for a connection using Current.
func (p *HY2PinSet) SessionDeadline(host string, now time.Time) (time.Time, error) {
	certs, err := p.validate(host)
	if err != nil {
		return time.Time{}, err
	}
	entries := []CertificatePin{p.Current}
	if p.Next != nil {
		entries = append(entries, *p.Next)
	}
	var deadline time.Time
	for i, cert := range certs {
		if slices.Contains(p.Revoked, entries[i].SHA256) || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			continue
		}
		if deadline.IsZero() || cert.NotAfter.Before(deadline) {
			deadline = cert.NotAfter
		}
	}
	if deadline.IsZero() {
		return time.Time{}, ErrNoActivePin
	}
	return deadline, nil
}

// NextActivation is only a wakeup hint for an already authenticated pin set.
// It never makes a certificate usable early or extends a live session.
func (p *HY2PinSet) NextActivation(host string, now time.Time) (time.Time, error) {
	certs, err := p.validate(host)
	if err != nil {
		return time.Time{}, err
	}
	if p.Next == nil || slices.Contains(p.Revoked, p.Next.SHA256) || !now.Before(certs[1].NotBefore) {
		return time.Time{}, nil
	}
	return certs[1].NotBefore.UTC(), nil
}

// New trust objects require exact property names and reject duplicate keys,
// unknown fields, null objects and trailing values before typed decoding.
func decodePinObject(data []byte, allowed ...string) (map[string]json.RawMessage, error) {
	return decodeMetadataObject(data, maxPinSetJSONSize, allowed...)
}

func decodeMetadataObject(data []byte, limit int, allowed ...string) (map[string]json.RawMessage, error) {
	if limit <= 0 || len(data) > limit {
		return nil, errors.New("metadata: pin object too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, errors.New("metadata: expected pin object")
	}
	out := map[string]json.RawMessage{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok || !slices.Contains(allowed, key) || out[key] != nil {
			return nil, errors.New("metadata: unknown, aliased or duplicate pin property")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("metadata: trailing pin data")
	}
	return out, nil
}

func (p *CertificatePin) UnmarshalJSON(data []byte) error {
	obj, err := decodePinObject(data, "sha256", "certificate")
	if err != nil {
		return err
	}
	var out CertificatePin
	if len(obj) != 2 || bytes.Equal(bytes.TrimSpace(obj["sha256"]), []byte("null")) || bytes.Equal(bytes.TrimSpace(obj["certificate"]), []byte("null")) || json.Unmarshal(obj["sha256"], &out.SHA256) != nil || json.Unmarshal(obj["certificate"], &out.Certificate) != nil {
		return errors.New("metadata: incomplete certificate pin")
	}
	*p = out
	return nil
}

func (p *HY2PinSet) UnmarshalJSON(data []byte) error {
	obj, err := decodePinObject(data, "current", "next", "revoked")
	if err != nil {
		return err
	}
	var out HY2PinSet
	if err := json.Unmarshal(obj["current"], &out.Current); err != nil {
		return fmt.Errorf("metadata: current pin: %w", err)
	}
	if next, ok := obj["next"]; ok {
		out.Next = new(CertificatePin)
		if err := json.Unmarshal(next, out.Next); err != nil {
			return err
		}
	}
	if revoked, ok := obj["revoked"]; ok {
		if err := json.Unmarshal(revoked, &out.Revoked); err != nil {
			return errors.New("metadata: invalid pin revocations")
		}
	}
	*p = out
	return nil
}
