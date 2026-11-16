package repository

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

func digest(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func approved(raw []byte, expected string) bool { return digest(raw) == expected }

// Keygen is an explicit publisher command only. A custody workstation generates
// one key; no threshold private-key list exists in any public plan or command.
func keygen(publicPath, privatePath string) ([]byte, error) {
	a, err := filepath.Abs(publicPath)
	if err != nil {
		return nil, err
	}
	b, err := filepath.Abs(privatePath)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(a, b) {
		return nil, errors.New("repository: public/private output paths must differ")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	defer clear(priv)
	key, err := tuf.KeyFromPublicKey(pub)
	if err != nil {
		return nil, err
	}
	id, err := key.ID()
	if err != nil {
		return nil, err
	}
	public, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	private := make([]byte, hex.EncodedLen(len(priv)))
	defer clear(private)
	hex.Encode(private, priv)
	if err = publisherfiles.PublishNew(privatePath, private); err != nil {
		return nil, err
	}
	if err = publisherfiles.PublishNew(publicPath, public); err != nil {
		return nil, fmt.Errorf("repository: private output retained; public publication failed: %w", err)
	}
	return json.Marshal(struct {
		KeyID        string `json:"keyid"`
		PublicSHA256 string `json:"public_file_sha256"`
	}{id, digest(public)})
}

func readPrivate(arg string) (ed25519.PrivateKey, error) {
	if !strings.HasPrefix(arg, "@") || len(arg) == 1 {
		return nil, errors.New("repository: private input requires explicit @file")
	}
	raw, err := publisherfiles.Read(arg[1:], 1024)
	if err != nil {
		return nil, errors.New("repository: private input unavailable")
	}
	defer clear(raw)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) != ed25519.PrivateKeySize*2 {
		return nil, errors.New("repository: invalid private encoding")
	}
	priv := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	if _, err = hex.Decode(priv, trimmed); err != nil {
		clear(priv)
		return nil, errors.New("repository: invalid private encoding")
	}
	canonical := ed25519.NewKeyFromSeed(priv[:ed25519.SeedSize])
	defer clear(canonical)
	if !bytes.Equal(priv, canonical) {
		clear(priv)
		return nil, errors.New("repository: inconsistent private/public material")
	}
	return priv, nil
}

func signDraft(role, input, approval, privateArg, output string, now time.Time) ([]byte, error) {
	raw, err := publisherfiles.Read(input, delivery.MaxMetadataBytes)
	if err != nil {
		return nil, err
	}
	if !approved(raw, approval) {
		return nil, errors.New("repository: independently approved draft SHA256 required")
	}
	d, err := decodeDocument(role, raw)
	if err != nil {
		return nil, err
	}
	if len(d.signatures()) != 0 {
		return nil, errors.New("repository: each custodian signs the same unsigned draft")
	}
	if err = d.fresh(now); err != nil {
		return nil, err
	}
	priv, err := readPrivate(privateArg)
	if err != nil {
		return nil, err
	}
	defer clear(priv)
	signer, err := signature.LoadSigner(priv, crypto.Hash(0))
	if err != nil {
		return nil, errors.New("repository: signer unavailable")
	}
	if err = d.sign(signer); err != nil {
		return nil, errors.New("repository: SDK signing failed")
	}
	out, err := d.encode()
	if err != nil {
		return nil, err
	}
	return publishDocument(d, nil, out, false, output, now)
}

type AssemblyPlan struct {
	Schema     int      `json:"schema"`
	Role       string   `json:"role"`
	Draft      string   `json:"draft"`
	Parent     string   `json:"parent"`
	Signatures []string `json:"signatures"`
}

func assemble(filename, draftApproval, parentApproval, output string, now time.Time) ([]byte, error) {
	raw, err := publisherfiles.Read(filename, 32<<10)
	if err != nil {
		return nil, err
	}
	fields, err := publisherfiles.Object(raw, 5)
	if err != nil || len(fields) != 5 {
		return nil, errors.New("repository: exact assembly plan required")
	}
	for _, name := range []string{"schema", "role", "draft", "parent", "signatures"} {
		if fields[name] == nil {
			return nil, errors.New("repository: assembly field missing/aliased")
		}
	}
	var plan AssemblyPlan
	if json.Unmarshal(raw, &plan) != nil || plan.Schema != 1 || len(plan.Signatures) < 1 || len(plan.Signatures) > 64 {
		return nil, errors.New("repository: invalid assembly plan")
	}
	base, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return nil, err
	}
	draft, err := publisherfiles.ReadRelativeJSON(base, plan.Draft, delivery.MaxMetadataBytes)
	if err != nil {
		return nil, err
	}
	if !approved(draft, draftApproval) {
		return nil, errors.New("repository: approved draft changed")
	}
	d, err := decodeDocument(plan.Role, draft)
	if err != nil {
		return nil, err
	}
	if len(d.signatures()) != 0 {
		return nil, errors.New("repository: unsigned draft required")
	}
	if err = d.fresh(now); err != nil {
		return nil, err
	}
	parent, err := loadParent(base, plan.Parent, parentApproval, d, now)
	if err != nil {
		return nil, err
	}
	keys, err := authorizedKeys(parent, d)
	if err != nil {
		return nil, err
	}
	payload, err := cjson.EncodeCanonical(d.signed())
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	sigs := []tuf.Signature{}
	remaining := delivery.MaxPackageBytes
	for _, rel := range plan.Signatures {
		partial, err := publisherfiles.ReadRelativeJSON(base, rel, int64(min(remaining, delivery.MaxMetadataBytes)))
		if err != nil {
			return nil, err
		}
		remaining -= len(partial)
		part, err := decodeDocument(plan.Role, partial)
		if err != nil {
			return nil, err
		}
		other, err := cjson.EncodeCanonical(part.signed())
		if err != nil || !bytes.Equal(payload, other) || len(part.signatures()) != 1 {
			return nil, errors.New("repository: partial signature is for a different draft")
		}
		sig := part.signatures()[0]
		if seen[sig.KeyID] || len(sig.Signature) != ed25519.SignatureSize || keys[sig.KeyID] == nil || len(sig.UnrecognizedFields) != 0 {
			return nil, errors.New("repository: duplicate/unknown signature")
		}
		pub, err := keys[sig.KeyID].ToPublicKey()
		if err != nil {
			return nil, err
		}
		verifier, err := signature.LoadVerifier(pub, crypto.Hash(0))
		if err != nil {
			return nil, err
		}
		if err = verifier.VerifySignature(bytes.NewReader(sig.Signature), bytes.NewReader(payload)); err != nil {
			return nil, errors.New("repository: invalid partial signature")
		}
		seen[sig.KeyID] = true
		sigs = append(sigs, sig)
	}
	sort.Slice(sigs, func(i, j int) bool { return sigs[i].KeyID < sigs[j].KeyID })
	d.setSignatures(sigs)
	if parent != nil {
		if err = parent.verify(d); err != nil {
			return nil, err
		}
	}
	if d.root != nil {
		if err = d.verify(d); err != nil {
			return nil, err
		}
	}
	out, err := d.encode()
	if err != nil {
		return nil, err
	}
	return publishDocument(d, parent, out, true, output, now)
}

func loadParent(base, rel, approval string, child *document, now time.Time) (*document, error) {
	if rel == "" {
		if child.role != tuf.ROOT || child.version() != 1 || approval != "bootstrap" {
			return nil, errors.New("repository: only version1 root has explicit bootstrap")
		}
		return nil, nil
	}
	role := tuf.ROOT
	if child.role == "profiles" || child.role == "updates" {
		role = tuf.TARGETS
	}
	raw, err := publisherfiles.ReadRelativeJSON(base, rel, delivery.MaxMetadataBytes)
	if err != nil {
		return nil, err
	}
	if !approved(raw, approval) {
		return nil, errors.New("repository: independently approved parent SHA256 required")
	}
	parent, err := decodeDocument(role, raw)
	if err != nil {
		return nil, err
	}
	if len(parent.signatures()) == 0 {
		return nil, errors.New("repository: finalized parent required")
	}
	if parent.root != nil {
		if err = parent.verify(parent); err != nil {
			return nil, err
		}
	}
	if child.root != nil {
		if parent.version() == int64(^uint64(0)>>1) || child.version() != parent.version()+1 {
			return nil, errors.New("repository: root rotation must increment exactly one version")
		}
	} else if err = parent.fresh(now); err != nil {
		return nil, err
	}
	return parent, nil
}

func authorizedKeys(parent, child *document) (map[string]*tuf.Key, error) {
	out := map[string]*tuf.Key{}
	add := func(authority *document) error {
		var keys map[string]*tuf.Key
		var ids []string
		if authority.root != nil {
			role := authority.root.Signed.Roles[child.role]
			if role == nil {
				return delivery.ErrTrust
			}
			keys, ids = authority.root.Signed.Keys, role.KeyIDs
		} else if authority.targets != nil && authority.targets.Signed.Delegations != nil {
			keys = authority.targets.Signed.Delegations.Keys
			for _, role := range authority.targets.Signed.Delegations.Roles {
				if role.Name == child.role {
					ids = role.KeyIDs
				}
			}
		}
		if len(ids) == 0 {
			return delivery.ErrTrust
		}
		for _, id := range ids {
			key := keys[id]
			if key == nil {
				return delivery.ErrTrust
			}
			actual, err := key.ID()
			if err != nil || actual != id {
				return delivery.ErrTrust
			}
			if prior := out[id]; prior != nil && prior.Value.PublicKey != key.Value.PublicKey {
				return delivery.ErrTrust
			}
			out[id] = key
		}
		return nil
	}
	if parent != nil {
		if err := add(parent); err != nil {
			return nil, err
		}
	}
	if child.root != nil {
		if err := add(child); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func report(d *document, raw []byte, threshold bool) ([]byte, error) {
	ids := []string{}
	for _, sig := range d.signatures() {
		ids = append(ids, sig.KeyID)
	}
	return json.Marshal(struct {
		Role                         string    `json:"role"`
		Version                      int64     `json:"version"`
		SHA256                       string    `json:"sha256"`
		Bytes                        int       `json:"bytes"`
		Expires                      time.Time `json:"expires"`
		Signers                      []string  `json:"signers"`
		ThresholdChecked             bool      `json:"role_threshold_checked"`
		RequiresRepositoryAcceptance bool      `json:"requires_repository_acceptance"`
	}{d.role, d.version(), digest(raw), len(raw), d.expires(), ids, threshold, true})
}
