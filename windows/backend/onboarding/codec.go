package onboarding

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type bundleCodec struct{ kdf *kdfExecutor }

func (c bundleCodec) seal(payload *BundlePayload, passphrase string) (string, error) {
	if payload == nil {
		return "", errors.New("onboarding: payload nil")
	}
	if len(passphrase) > MaxPassphraseSize {
		return "", ErrTooLarge
	}
	cp := *payload
	if cp.Schema == 0 {
		cp.Schema = 1
	}
	if cp.Schema != 1 {
		return "", ErrBadFormat
	}
	if !payloadSizeAllowed(&cp) {
		return "", ErrTooLarge
	}
	plain, err := json.Marshal(&cp)
	if err != nil {
		return "", ErrBadFormat
	}
	defer clear(plain)
	if len(plain) > MaxBundleSize {
		return "", ErrTooLarge
	}
	var gz bytes.Buffer
	zw, err := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	if err != nil {
		return "", ErrBadFormat
	}
	if _, err := zw.Write(plain); err != nil {
		return "", ErrBadFormat
	}
	if err := zw.Close(); err != nil {
		return "", ErrBadFormat
	}
	// Predict encoded output before randomness/KDF, so Seal cannot emit a
	// transport which Open would reject for size.
	if encodedSize(gz.Len()+16) > MaxBundleSize {
		return "", ErrTooLarge
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", ErrBadFormat
	}
	key, err := c.kdf.derive(passphrase, salt, scryptN, scryptR, scryptP)
	if err != nil {
		return "", err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrBadFormat
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrBadFormat
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", ErrBadFormat
	}
	ct := gcm.Seal(nil, nonce, gz.Bytes(), aadFor(1, scryptN, scryptR, scryptP, salt))
	sealed := sealedFormat{Magic: magic, Version: 1, N: scryptN, R: scryptR, P: scryptP, SaltB64: base64.StdEncoding.EncodeToString(salt), NonceB64: base64.StdEncoding.EncodeToString(nonce), DataB64: base64.StdEncoding.EncodeToString(ct)}
	raw, err := json.Marshal(sealed)
	if err != nil {
		return "", ErrBadFormat
	}
	return magic + "1." + base64.RawURLEncoding.EncodeToString(raw), nil
}

// Cheap lower bounds avoid marshaling arbitrarily large caller-owned inputs.
// The encoded JSON length is checked again for escaping/structural overhead.
func payloadSizeAllowed(p *BundlePayload) bool {
	if len(p.TrustedKeys) > MaxBundleSize/16 || len(p.Secrets) > MaxBundleSize/16 {
		return false
	}
	remaining := MaxBundleSize
	consume := func(n int) bool {
		if n > remaining {
			return false
		}
		remaining -= n
		return true
	}
	if !consume(len(p.DeviceName)) || !consume(len(p.Envelope)) {
		return false
	}
	for _, k := range p.TrustedKeys {
		if !consume(len(k.KeyID)) || !consume(len(k.Public)) || !consume(len(k.Comment)) {
			return false
		}
	}
	for _, s := range p.Secrets {
		if !consume(len(s.Kind)) || !consume(len(s.ID)) || !consume(len(s.Value)) {
			return false
		}
	}
	return true
}

func encodedSize(ciphertextSize int) int {
	header := sealedFormat{Magic: magic, Version: 1, N: scryptN, R: scryptR, P: scryptP, SaltB64: strings.Repeat("A", base64.StdEncoding.EncodedLen(saltLen)), NonceB64: strings.Repeat("A", base64.StdEncoding.EncodedLen(nonceLen))}
	raw, _ := json.Marshal(header)
	return len(magic+"1.") + base64.RawURLEncoding.EncodedLen(len(raw)+base64.StdEncoding.EncodedLen(ciphertextSize))
}

func decodeStrict(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrBadFormat
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return ErrBadFormat
	}
	return nil
}

func (c bundleCodec) open(transport, passphrase string) (*BundlePayload, error) {
	if len(transport) > MaxBundleSize || len(passphrase) > MaxPassphraseSize {
		return nil, ErrTooLarge
	}
	transport = strings.TrimSpace(transport)
	prefix := magic + "1."
	if !strings.HasPrefix(transport, prefix) {
		return nil, ErrBadFormat
	}
	raw, err := base64.RawURLEncoding.DecodeString(transport[len(prefix):])
	if err != nil {
		return nil, ErrBadFormat
	}
	var sealed sealedFormat
	if err := decodeStrict(raw, &sealed); err != nil {
		return nil, err
	}
	if sealed.Magic != magic || sealed.Version != 1 {
		return nil, ErrBadFormat
	}
	if _, err := estimateKDF(sealed.N, sealed.R, sealed.P); err != nil {
		return nil, err
	}
	salt, err := base64.StdEncoding.DecodeString(sealed.SaltB64)
	if err != nil || len(salt) < 16 || len(salt) > 64 {
		return nil, ErrBadFormat
	}
	nonce, err := base64.StdEncoding.DecodeString(sealed.NonceB64)
	if err != nil || len(nonce) != nonceLen {
		return nil, ErrBadFormat
	}
	ct, err := base64.StdEncoding.DecodeString(sealed.DataB64)
	if err != nil || len(ct) < 16 {
		return nil, ErrBadFormat
	}
	key, err := c.kdf.derive(passphrase, salt, sealed.N, sealed.R, sealed.P)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrBadFormat
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrBadFormat
	}
	plain, err := gcm.Open(nil, nonce, ct, aadFor(sealed.Version, sealed.N, sealed.R, sealed.P, salt))
	if err != nil {
		return nil, ErrWrongPassphrase
	}
	defer clear(plain)
	zr, err := gzip.NewReader(bytes.NewReader(plain))
	if err != nil {
		return nil, ErrBadFormat
	}
	defer zr.Close()
	payloadJSON, err := io.ReadAll(io.LimitReader(zr, MaxBundleSize+1))
	if err != nil {
		return nil, ErrBadFormat
	}
	defer clear(payloadJSON)
	if len(payloadJSON) > MaxBundleSize {
		return nil, ErrTooLarge
	}
	var payload BundlePayload
	if err := decodeStrict(payloadJSON, &payload); err != nil {
		return nil, err
	}
	if payload.Schema != 1 {
		return nil, ErrBadFormat
	}
	return &payload, nil
}
