// Package repository authors and signs bounded public TUF metadata for the
// existing publisher CLIs. It never collects a threshold's private keys.
package repository

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/snowden-system/windows/backend/delivery"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

type document struct {
	role         string
	sourceSHA256 string // exact input bytes, not a signing authority
	root         *tuf.Metadata[tuf.RootType]
	targets      *tuf.Metadata[tuf.TargetsType]
	snapshot     *tuf.Metadata[tuf.SnapshotType]
	timestamp    *tuf.Metadata[tuf.TimestampType]
}

func decodeDocument(role string, raw []byte) (*document, error) {
	if err := delivery.CheckPublicationMetadata(role, raw); err != nil {
		return nil, err
	}
	d := &document{role: role, sourceSHA256: digest(raw)}
	var err error
	switch role {
	case tuf.ROOT:
		d.root, err = tuf.Root().FromBytes(raw)
	case tuf.TARGETS, "profiles", "updates":
		d.targets, err = tuf.Targets().FromBytes(raw)
	case tuf.SNAPSHOT:
		d.snapshot, err = tuf.Snapshot().FromBytes(raw)
	case tuf.TIMESTAMP:
		d.timestamp, err = tuf.Timestamp().FromBytes(raw)
	default:
		return nil, delivery.ErrTrust
	}
	if err != nil || d.version() < 1 || d.expires().IsZero() {
		return nil, errors.New("repository: invalid role document")
	}
	return d, nil
}

func (d *document) value() any {
	switch {
	case d.root != nil:
		return d.root
	case d.targets != nil:
		return d.targets
	case d.snapshot != nil:
		return d.snapshot
	default:
		return d.timestamp
	}
}

func (d *document) signed() any {
	switch {
	case d.root != nil:
		return d.root.Signed
	case d.targets != nil:
		return d.targets.Signed
	case d.snapshot != nil:
		return d.snapshot.Signed
	default:
		return d.timestamp.Signed
	}
}

func (d *document) signatures() []tuf.Signature {
	switch {
	case d.root != nil:
		return d.root.Signatures
	case d.targets != nil:
		return d.targets.Signatures
	case d.snapshot != nil:
		return d.snapshot.Signatures
	default:
		return d.timestamp.Signatures
	}
}

func (d *document) setSignatures(sigs []tuf.Signature) {
	switch {
	case d.root != nil:
		d.root.Signatures = sigs
	case d.targets != nil:
		d.targets.Signatures = sigs
	case d.snapshot != nil:
		d.snapshot.Signatures = sigs
	default:
		d.timestamp.Signatures = sigs
	}
}

func (d *document) sign(signer signature.Signer) error {
	var err error
	switch {
	case d.root != nil:
		_, err = d.root.Sign(signer)
	case d.targets != nil:
		_, err = d.targets.Sign(signer)
	case d.snapshot != nil:
		_, err = d.snapshot.Sign(signer)
	default:
		_, err = d.timestamp.Sign(signer)
	}
	return err
}

func (d *document) version() int64 {
	switch {
	case d.root != nil:
		return d.root.Signed.Version
	case d.targets != nil:
		return d.targets.Signed.Version
	case d.snapshot != nil:
		return d.snapshot.Signed.Version
	default:
		return d.timestamp.Signed.Version
	}
}

func (d *document) expires() time.Time {
	switch {
	case d.root != nil:
		return d.root.Signed.Expires
	case d.targets != nil:
		return d.targets.Signed.Expires
	case d.snapshot != nil:
		return d.snapshot.Signed.Expires
	default:
		return d.timestamp.Signed.Expires
	}
}

func (d *document) verify(child *document) error {
	if d.root != nil {
		return d.root.VerifyDelegate(child.role, child.value())
	}
	if d.role == tuf.TARGETS && d.targets != nil {
		return d.targets.VerifyDelegate(child.role, child.value())
	}
	return errors.New("repository: parent cannot delegate this role")
}

func (d *document) encode() ([]byte, error) {
	raw, err := json.Marshal(d.value())
	if err != nil {
		return nil, err
	}
	if err = delivery.CheckPublicationMetadata(d.role, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (d *document) fresh(now time.Time) error {
	if now.IsZero() || !d.expires().After(now) {
		return errors.New("repository: expired metadata")
	}
	return nil
}
