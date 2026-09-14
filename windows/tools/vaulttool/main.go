// vaulttool — одноразовый CLI для наполнения DPAPI-хранилища проверенными
// секретами (без UI). Значения читаются из файлов/ENV и никогда не печатаются:
// наружу — только метаданные (fingerprint, статус проверки).
//
// Использование:
//
//	vaulttool fill <vault.v1.json> <spec.json>
//	vaulttool list <vault.v1.json>
//	vaulttool verify-all <vault.v1.json>
//
// spec.json: [{"kind":"vless-uuid","path":"secrets/channels/vless_uuid"}, ...]
// (или {"env":"NAME"} вместо path). kind custom — доп. поля title/hint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/snowden-system/windows/backend/secretvault"
)

type specItem struct {
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
	Env   string `json:"env,omitempty"`
	Title string `json:"title,omitempty"`
	Hint  string `json:"hint,omitempty"`
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vaulttool: "+format+"\n", args...)
	os.Exit(1)
}

func valueOf(it specItem) string {
	if it.Path != "" {
		b, err := os.ReadFile(it.Path)
		if err != nil {
			fatal("read %s: %v", it.Path, err)
		}
		return strings.TrimSpace(string(b))
	}
	if it.Env != "" {
		v := os.Getenv(it.Env)
		if v == "" {
			fatal("env %s пуст", it.Env)
		}
		return strings.TrimSpace(v)
	}
	fatal("spec item без path/env")
	return ""
}

func main() {
	if len(os.Args) < 3 {
		fatal("usage: vaulttool fill|list|verify-all <vault.json> [spec.json]")
	}
	cmd, vaultPath := os.Args[1], os.Args[2]
	m := secretvault.NewManager(vaultPath)

	switch cmd {
	case "fill":
		raw, err := os.ReadFile(os.Args[3])
		if err != nil {
			fatal("read spec: %v", err)
		}
		var spec []specItem
		if err := json.Unmarshal(raw, &spec); err != nil {
			fatal("parse spec: %v", err)
		}
		for _, it := range spec {
			v := valueOf(it)
			kind := secretvault.Kind(it.Kind)
			title, hint := it.Title, it.Hint
			if title == "" {
				title = "Секрет " + string(kind)
			}
			meta, err := m.AddSave(string(kind), title, v, hint)
			if err != nil {
				fatal("save %s: %v", it.Kind, err)
			}
			fmt.Printf("saved %-18s fingerprint=%s status=%s\n", it.Kind, meta.Fingerprint, meta.VerifyStatus)
		}
	case "verify-all":
		metas, err := m.List()
		if err != nil {
			fatal("list: %v", err)
		}
		bad := 0
		for _, mt := range metas {
			if mt.StoredValue == "" {
				fmt.Printf("skip  %-18s (значение не задано)\n", mt.Kind)
				continue
			}
			meta, err := m.Verify(mt.ID)
			status, fp := meta.VerifyStatus, meta.Fingerprint
			if err != nil && meta.VerifyStatus == "" {
				status = "failed"
			}
			if status != "ok" {
				bad++
			}
			fmt.Printf("verify %-18s fingerprint=%s status=%s %s\n", mt.Kind, fp, status, meta.VerifyError)
		}
		if bad > 0 {
			os.Exit(2)
		}
	case "list":
		metas, err := m.List()
		if err != nil {
			fatal("list: %v", err)
		}
		for _, mt := range metas {
			set := "empty"
			if mt.StoredValue != "" {
				set = "dpapi"
			}
			fmt.Printf("%-18s value=%-5s fingerprint=%s verify=%s reveals=%d\n",
				mt.Kind, set, mt.Fingerprint, mt.VerifyStatus, mt.Reveals)
		}
	default:
		fatal("неизвестная команда: %s", cmd)
	}
}
