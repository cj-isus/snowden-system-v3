package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/netprobe"
	"github.com/snowden-system/windows/backend/update"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

func cmdSignPair(copyPayloads bool) {
	if len(os.Args) < 7 || len(os.Args) > 8 {
		fatal("usage: updatetool sign-pair|sign-pair-copy <app.exe> <helper.exe> <version> <private.key> <new-outdir> [notes]")
	}
	appPath, helperPath, version, keyPath, outDir := os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]
	if _, err := update.ParseVersion(version); err != nil {
		fatal("version: %v", err)
	}
	if len(os.Args) == 8 && len(os.Args[7]) > 16<<10 {
		fatal("pair notes exceed bound")
	}
	app, err := update.DescribePayload(appPath, update.MaxPayloadSize)
	if err != nil {
		fatal("app: %v", err)
	}
	helper, err := update.DescribePayload(helperPath, netprobe.MaxNativeImageBytes)
	if err != nil {
		fatal("helper: %v", err)
	}
	// Source basenames cannot select installation destinations or collide.
	app.File, helper.File = update.ApplicationFile, netprobe.NativeHelperName
	m := &update.Manifest{Schema: update.PairSchema, Target: update.PairTarget, ActivationProtocol: update.PairActivationProtocol, Version: version, ReleasedAt: time.Now().UTC(),
		File: app.File, SHA256: app.SHA256, Size: app.Size, NativeHelper: &update.NativeHelper{Payload: helper, Protocol: netprobe.NativeProtocol}}
	if len(os.Args) == 8 {
		m.Notes = os.Args[7]
	}
	if err := m.Validate(); err != nil {
		fatal("pair: %v", err)
	}
	if err := m.Sign(loadPriv(keyPath)); err != nil {
		fatal("sign: %v", err)
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil || len(raw) > update.MaxManifestSize {
		fatal("pair manifest exceeds bound")
	}
	// A fresh output prevents overwriting an accepted release or either input.
	// On failure keep this incomplete directory for the operator; no recursive
	// cleanup, public-file adoption, force overwrite or implicit reuse.
	if err := os.Mkdir(outDir, 0o700); err != nil {
		fatal("new output directory required: %v", err)
	}
	if copyPayloads {
		for _, item := range []struct {
			source string
			file   update.Payload
		}{{appPath, app}, {helperPath, helper}} {
			if err := publisherfiles.PublishNewStreamChecked(filepath.Join(outDir, item.file.File), func(dst io.Writer) error {
				return item.file.CopyVerified(item.source, dst)
			}, nil); err != nil {
				fatal("pair artifact: %v", err)
			}
		}
	}
	check := func() error {
		if copyPayloads {
			return m.VerifyPayloadSet(outDir)
		}
		if err := app.Verify(appPath); err != nil {
			return err
		}
		return helper.Verify(helperPath)
	}
	if err := publisherfiles.PublishNewChecked(filepath.Join(outDir, "update.json"), raw, check); err != nil {
		fatal("pair manifest: %v", err)
	}
	fmt.Printf("ok: schema2 v%s target=%s; signed app/helper hashes; installer/readiness not checked\n", m.Version, m.Target)
}

func cmdVerifyPair() {
	if len(os.Args) != 5 {
		fatal("usage: updatetool verify-pair <update.json> <payload-dir> <trusted_keys.json>")
	}
	raw, err := publisherfiles.Read(os.Args[2], update.MaxManifestSize)
	if err != nil {
		fatal("manifest: %v", err)
	}
	m, err := update.DecodeStrict(raw)
	if err != nil || m.Schema != update.PairSchema {
		fatal("schema2 pair manifest required: %v", err)
	}
	policy, present, err := metadata.ReadTrustPolicyFile(os.Args[4])
	if err != nil || !present {
		fatal("trusted keys unavailable: %v", err)
	}
	if err := m.Verify(policy.VerificationKeys(metadata.RoleUpdate)); err != nil {
		fatal("signature: %v", err)
	}
	if err := m.VerifyPayloadSet(os.Args[3]); err != nil {
		fatal("pair payloads: %v", err)
	}
	fmt.Printf("ok: schema2 signature/app/helper hashes v%s; role assignment/enrollment/version/installer/native gates remain caller-owned\n", m.Version)
}
