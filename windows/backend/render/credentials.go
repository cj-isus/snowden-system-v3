package render

import "fmt"

// ValidateCredentials checks staged secrets without needing a pin, engine,
// network access or runtime config side effects. Planned/disabled/revoked
// channels need not have provisioned credentials yet.
func ValidateCredentials(channels []ChannelDescriptor, src SecretsSource) error {
	for _, ch := range Selectable(channels) {
		var refs []string
		switch {
		case ch.Protocol == "hysteria2":
			refs = []string{"hy2-password", "hy2-obfs-password"}
		case ch.Transport == "tcp-reality":
			refs = []string{ch.UUIDRef, ch.RealityPublicKeyRef, ch.RealityShortIDRef}
		case ch.Transport == "shadowtls":
			refs = []string{ch.UUIDRef, ch.ShadowTLSPasswordRef}
		default:
			refs = []string{"vless-uuid"}
		}
		if len(refs) != len(ch.CredentialRefs) {
			return fmt.Errorf("channel %s: unexpected credential roles", ch.ID)
		}
		for _, ref := range refs {
			if _, err := getOrErr(src, ch, ref); err != nil {
				return fmt.Errorf("channel %s: %w", ch.ID, err)
			}
		}
	}
	return nil
}
