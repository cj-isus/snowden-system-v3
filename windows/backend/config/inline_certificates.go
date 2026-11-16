package config

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"maps"
	"slices"
)

// Remove only bounded, fully parsed public certificate fields from the
// generic private-material scan. The runtime config itself stays unchanged.
// x509 hostname/time verification is retained by the native TLS client.
func withoutInlineCertificates(c *Config) (*Config, error) {
	out := *c
	out.Outbounds = slices.Clone(c.Outbounds)
	for i, ob := range out.Outbounds {
		value, present := ob.TLS["certificate"]
		if !present {
			continue
		}
		if _, mixed := ob.TLS["certificate_path"]; mixed {
			return nil, errors.New("inline certificate and certificate_path cannot be combined")
		}
		if err := validateInlineCertificates(value); err != nil {
			return nil, err
		}
		out.Outbounds[i].TLS = maps.Clone(ob.TLS)
		delete(out.Outbounds[i].TLS, "certificate")
	}
	return &out, nil
}

func validateInlineCertificates(value any) error {
	var certificates []string
	switch v := value.(type) {
	case string:
		certificates = []string{v}
	case []string:
		certificates = v
	case []any:
		if len(v) > 64 {
			return errors.New("too many inline certificates")
		}
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return errors.New("inline certificates must be strings")
			}
			certificates = append(certificates, s)
		}
	default:
		return errors.New("invalid inline certificate type")
	}
	// Signed HY2 Current/Next remains limited to two in metadata. Runtime also
	// accepts bounded owned snapshots of legacy public CA/certificate bundles.
	if len(certificates) == 0 || len(certificates) > 64 {
		return errors.New("one to 64 inline certificates required")
	}
	total := 0
	for _, certificate := range certificates {
		total += len(certificate)
		if total > 1<<20 {
			return errors.New("inline certificate bundle exceeds bound")
		}
		if len(certificate) == 0 || len(certificate) > 16<<10 {
			return errors.New("invalid inline certificate size")
		}
		data := bytes.TrimSpace([]byte(certificate))
		// pem.Decode may skip a damaged first block. The scan exemption must
		// cover exactly one block across the complete input, not just its tail.
		if !bytes.HasPrefix(data, []byte("-----BEGIN CERTIFICATE-----")) || bytes.Count(data, []byte("-----BEGIN ")) != 1 {
			return errors.New("inline field requires exactly one certificate PEM")
		}
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 {
			return errors.New("inline field requires exactly one public certificate")
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return errors.New("invalid inline certificate DER")
		}
	}
	return nil
}
