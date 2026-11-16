package deliverypack

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

// Publication owns the existing issuer verifier result. It is not installation
// acceptance. Getters return copies; no caller can manufacture a private proof.
type Publication struct {
	value  *assembled
	kind   delivery.Kind
	target string
}

func PreparePublication(ctx context.Context, kind delivery.Kind, filename, approvedRoot string) (*Publication, error) {
	if ctx == nil || kind != delivery.Profile && kind != delivery.UpdateManifest && kind != delivery.PinPolicy {
		return nil, errors.New("delivery-pack: context and explicit supported purpose required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hash, err := hex.DecodeString(approvedRoot)
	if err != nil || len(hash) != 32 || approvedRoot != strings.ToLower(approvedRoot) {
		return nil, errors.New("delivery-pack: exact lowercase out-of-band root SHA256 required")
	}
	plan, base, err := loadPlan(filename, kind)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	value, err := assemble(ctx, plan, base, approvedRoot, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return &Publication{value: value, kind: kind, target: plan.Target}, nil
}

func (p *Publication) Check(now time.Time) error {
	if p == nil || p.value == nil {
		return delivery.ErrTrust
	}
	return p.value.check(p.kind, p.target, now)
}

func (p *Publication) Metadata() map[string][]byte {
	out := map[string][]byte{}
	for name, raw := range p.value.records {
		out[name] = bytes.Clone(raw)
	}
	return out
}

func (p *Publication) Artifact() []byte      { return bytes.Clone(p.value.artifact) }
func (p *Publication) ClientPackage() []byte { return bytes.Clone(p.value.data) }
func (p *Publication) EpochGrant() []byte    { return bytes.Clone(p.value.grantRaw) }
func (p *Publication) PinPolicy() []byte     { return bytes.Clone(p.value.pinRaw) }
func (p *Publication) Summary() Report {
	out := p.value.report
	out.Metadata = slices.Clone(out.Metadata)
	if out.ProfileVersion != nil {
		v := *out.ProfileVersion
		out.ProfileVersion = &v
	}
	return out
}
