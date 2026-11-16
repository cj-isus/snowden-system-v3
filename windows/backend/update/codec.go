package update

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const MaxManifestSize = 64 << 10

// DecodeStrict is shared by App and publisher tools. Only native_helper nests;
// duplicate/aliased/null/unknown/trailing fields must not be folded silently.
func DecodeStrict(raw []byte) (*Manifest, error) {
	if len(raw) == 0 || len(raw) > MaxManifestSize {
		return nil, errors.New("update: manifest outside size bound")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	first, err := dec.Token()
	if err != nil || first != json.Delim('{') {
		return nil, errors.New("update: manifest object required")
	}
	seen := map[string]bool{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, errors.New("update: malformed manifest field")
		}
		name, ok := token.(string)
		if !ok || name != strings.ToLower(name) || seen[name] {
			return nil, errors.New("update: duplicate/aliased manifest field")
		}
		seen[name] = true
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errors.New("update: invalid/null manifest field")
		}
		if name == "native_helper" {
			if err := strictNativeHelper(value); err != nil {
				return nil, err
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return nil, errors.New("update: malformed manifest object")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("update: trailing manifest data")
	}
	dec = json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var manifest Manifest
	if err := dec.Decode(&manifest); err != nil {
		return nil, errors.New("update: malformed/unknown manifest field")
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func strictNativeHelper(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	first, err := dec.Token()
	if err != nil || first != json.Delim('{') {
		return errors.New("update: native_helper object required")
	}
	allowed := map[string]bool{"file": true, "sha256": true, "size": true, "protocol": true}
	for dec.More() {
		token, err := dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] {
			return errors.New("update: native_helper duplicate/aliased/unknown field")
		}
		delete(allowed, name)
		var value json.RawMessage
		if dec.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("update: native_helper invalid/null field")
		}
	}
	last, err := dec.Token()
	if err != nil || last != json.Delim('}') || len(allowed) != 0 || dec.Decode(new(any)) != io.EOF {
		return errors.New("update: incomplete native_helper object")
	}
	return nil
}
