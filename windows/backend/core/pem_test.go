package core

import (
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func writeTempPEMFromDER(t *testing.T, der []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "cert.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
