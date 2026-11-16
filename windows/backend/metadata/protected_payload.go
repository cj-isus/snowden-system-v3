package metadata

import "errors"

const RestrictionPayloadConfidentiality = "unsupported_payload_confidentiality"

var ErrPayloadConfidentialityUnsupported = errors.New("VLESS over ShadowTLS is not supported for protected payload: no approved inner encryption")

// CheckProtectedPayload is a local execution capability gate, not signed schema
// validation or proof of a healthy/encrypted path. Keep unsupported descriptors
// and their signed history intact; do not read their runtime credentials.
// No current schema field declares an independently validated inner cipher for
// VLESS-over-ShadowTLS. A donor TLS handshake is not that capability.
func (channel Channel) CheckProtectedPayload() error {
	if channel.Protocol == "vless" && channel.Transport == "shadowtls" {
		return ErrPayloadConfidentialityUnsupported
	}
	return nil
}
