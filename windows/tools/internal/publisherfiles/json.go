package publisherfiles

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Object rejects duplicate/null top-level fields. Call again for nested maps;
// signed metadata itself uses delivery's recursive JSON and SDK validators.
func Object(data []byte, limit int) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if token, err := dec.Token(); err != nil || token != json.Delim('{') {
		return nil, errors.New("publisher: object required")
	}
	out := map[string]json.RawMessage{}
	for dec.More() {
		token, err := dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || name == "" || len(out) >= limit || out[name] != nil {
			return nil, errors.New("publisher: duplicate property or object bound")
		}
		var raw json.RawMessage
		if dec.Decode(&raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, errors.New("publisher: invalid/null property")
		}
		out[name] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("publisher: trailing object data")
	}
	return out, nil
}

func RelativeJSON(rel string) bool {
	if len(rel) == 0 || len(rel) > 240 || !strings.HasSuffix(rel, ".json") || path.Clean(rel) != rel || strings.HasPrefix(rel, "/") {
		return false
	}
	parts := strings.Split(rel, "/")
	if len(parts) > 12 {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	for _, ch := range rel {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("/._-", ch)) {
			return false
		}
	}
	return true
}

func ReadRelativeJSON(root, rel string, limit int64) ([]byte, error) {
	if !RelativeJSON(rel) {
		return nil, errors.New("publisher: invalid relative input")
	}
	current := root
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || i < len(parts)-1 && !info.IsDir() {
			return nil, errors.New("publisher: missing/nonregular/symlink input component")
		}
	}
	return Read(current, limit)
}
