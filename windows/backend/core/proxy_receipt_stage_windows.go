//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"

	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows"
)

// One original sidecar domain; caller serializes with the proxy locks/lease.
// Keep the slots after any uncertain acquisition/publication/Close or DPAPI
// result. The sole file contains a purpose-separated encrypted intent.
type proxyReceiptStage struct {
	path                     string
	reader                   *filetxn.BoundedReadSlot
	writer                   *filetxn.BoundedWriteSlot
	crypto                   proxyReceiptCrypto
	intent                   proxyStageIntent
	writeIntent              *proxyStageIntent
	writeCipher              []byte // exact borrowed writer input, retained after unknown publication
	info                     os.FileInfo
	digest                   [sha256.Size]byte
	loaded, present, pending bool
	failure                  error
}

func newProxyReceiptStage(path string) *proxyReceiptStage {
	return &proxyReceiptStage{path: filepath.Join(filepath.Dir(path), "receipt.v1.stage.bin"), reader: filetxn.NewBoundedReadSlot(maxProxyStageCipher),
		writer: filetxn.NewBoundedWriteSlot(maxProxyStageCipher), crypto: proxyReceiptCrypto{domain: proxyCryptoStage}}
}

func (stage *proxyReceiptStage) resourcesReturned() bool {
	return stage != nil && stage.reader.Retired() && stage.crypto.retired() && stage.writer.Check() == nil
}

func (stage *proxyReceiptStage) idle() bool {
	return stage != nil && !stage.pending && stage.resourcesReturned()
}

func (stage *proxyReceiptStage) load() (result error) {
	if stage == nil || !stage.idle() || stage.failure != nil {
		return ErrProxyStageUnavailable
	}
	if stage.loaded {
		return nil
	}
	stage.pending = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errors.Join(ErrProxyStageUnavailable, ErrProxyStateUncertain)
		}
		if returned && stage.resourcesReturned() {
			stage.pending = false
		}
		if stage.pending {
			result = errors.Join(result, ErrProxyStageUnavailable, ErrProxyStateUncertain)
		}
		if result != nil {
			stage.failure = result
		}
	}()
	raw, info, err := stage.reader.ReadSnapshot(stage.path, maxProxyStageCipher)
	if errors.Is(err, os.ErrNotExist) && stage.reader.Retired() {
		stage.loaded, returned = true, true
		return nil
	}
	if err != nil {
		returned = true
		return ErrProxyStageUnavailable
	}
	plain, err := stage.crypto.transform(raw, false)
	if err != nil {
		returned = true
		return errors.Join(ErrProxyStageUnavailable, err)
	}
	intent, err := decodeProxyStage(plain)
	clear(plain)
	if err != nil {
		returned = true
		return err
	}
	stage.intent, stage.info, stage.digest = intent, info, sha256.Sum256(raw)
	stage.loaded, stage.present, returned = true, true, true
	return nil
}

func (stage *proxyReceiptStage) recheck() error {
	if stage == nil || !stage.loaded || stage.failure != nil || !stage.resourcesReturned() {
		return ErrProxyStageUnavailable
	}
	raw, info, err := stage.reader.ReadSnapshot(stage.path, maxProxyStageCipher)
	if !stage.present && errors.Is(err, os.ErrNotExist) && stage.reader.Retired() {
		return nil
	}
	if err != nil || !stage.present || !sameProxyReceiptFile(stage.info, info) || sha256.Sum256(raw) != stage.digest {
		return ErrProxyStageConflict
	}
	return nil
}

func (stage *proxyReceiptStage) put(intent proxyStageIntent, parent proxyAnchorState) (result error) {
	if err := stage.load(); err != nil {
		return err
	}
	stage.pending = true
	stage.writeIntent = &intent
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errors.Join(ErrProxyStageUnavailable, ErrProxyStateUncertain)
		}
		if returned && stage.resourcesReturned() {
			stage.pending = false
		}
		if stage.pending {
			result = errors.Join(result, ErrProxyStageUnavailable, ErrProxyStateUncertain)
		}
		if result != nil {
			stage.failure = result
		}
	}()
	if stage.present && !(parent.sequence >= 2 && !parent.pending && stage.intent.parentSequence == parent.sequence-2 &&
		stage.intent.target == parent.committed && stage.intent.path == intent.path && bytes.Equal(stage.intent.sid, intent.sid)) {
		returned = true
		return ErrProxyStageConflict // do not erase an orphan or another unfinished intent
	}
	if err := stage.recheck(); err != nil {
		returned = true
		return err
	}
	plain, err := encodeProxyStage(intent)
	if err != nil {
		returned = true
		return err
	}
	stage.writeCipher, err = stage.crypto.transform(plain, true)
	if stage.crypto.retired() {
		clear(plain)
	}
	if err != nil {
		returned = true
		return errors.Join(ErrProxyStageUnavailable, err)
	}
	if err := stage.recheck(); err != nil {
		returned = true
		return err
	}
	if stage.present {
		err = stage.writer.Write(stage.path, stage.writeCipher)
	} else {
		err = stage.writer.WriteNew(stage.path, stage.writeCipher)
	}
	if err != nil {
		returned = true
		return errors.Join(ErrProxyStageUnavailable, err)
	}
	observed, info, err := stage.reader.ReadSnapshot(stage.path, maxProxyStageCipher)
	if err != nil || !bytes.Equal(observed, stage.writeCipher) {
		returned = true
		return ErrProxyStageConflict
	}
	stage.intent, stage.info, stage.digest, stage.present = intent, info, sha256.Sum256(stage.writeCipher), true
	stage.writeIntent, stage.writeCipher = nil, nil // only after known publication and original writer/reader retirement
	returned = true
	return nil
}

func proxyStageFileFor(info os.FileInfo) (proxyStageFile, error) {
	if info == nil {
		return proxyStageFile{}, nil
	}
	if !filetxn.SameWindowsFile(info, info) {
		return proxyStageFile{}, ErrProxyStageConflict
	}
	native, ok := info.Sys().(windows.ByHandleFileInformation)
	if !ok {
		return proxyStageFile{}, ErrProxyStageConflict
	}
	file := proxyStageFile{present: true, volume: native.VolumeSerialNumber, high: native.FileIndexHigh, low: native.FileIndexLow,
		size: uint64(info.Size()), written: uint64(native.LastWriteTime.HighDateTime)<<32 | uint64(native.LastWriteTime.LowDateTime)}
	if !file.valid() {
		return proxyStageFile{}, ErrProxyStageConflict
	}
	return file, nil
}

func (file proxyStageFile) matches(info os.FileInfo) bool {
	if !file.present {
		return info == nil
	}
	if !filetxn.MatchesWindowsFileID(info, file.volume, file.high, file.low) {
		return false
	}
	observed, err := proxyStageFileFor(info)
	return err == nil && observed == file
}

func (store *proxyReceiptStore) stageIntent(record proxyDurableReceipt, raw []byte) error {
	parent, err := proxyStageFileFor(store.fileInfo)
	if err != nil {
		return err
	}
	intent := proxyStageIntent{parentSequence: store.anchor.state.sequence, parent: store.anchor.state.committed, file: parent,
		path: store.anchorPath(), sid: bytes.Clone(store.record.sid), target: proxyAnchorFor(record, raw), raw: bytes.Clone(raw)}
	return store.stage.put(intent, store.anchor.state)
}
