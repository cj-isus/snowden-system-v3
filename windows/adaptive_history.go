package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/netprobe"
)

const maxAdaptiveHistoryBytes = 32 << 10

var adaptiveMu sync.Mutex
var adaptiveLast []SiteStatus

// One retained I/O domain, reusing original filetxn acquisition/Close receipts.
// History is never a trust source, proof of command completion or current scope.
var adaptiveHistoryIO = struct {
	mu     sync.Mutex
	reader *filetxn.BoundedReadSlot
	writer *filetxn.BoundedWriteSlot
}{reader: filetxn.NewBoundedReadSlot(maxAdaptiveHistoryBytes), writer: filetxn.NewBoundedWriteSlot(maxAdaptiveHistoryBytes)}

func adaptivePath() string {
	return filepath.Join(appDataDir(), "adaptive-sites.json")
}

func rememberAdaptive(sites []SiteStatus) {
	adaptiveMu.Lock()
	defer adaptiveMu.Unlock()
	adaptiveLast = append([]SiteStatus(nil), sites...)
}

func adaptiveMemory() []SiteStatus {
	adaptiveMu.Lock()
	defer adaptiveMu.Unlock()
	if adaptiveLast == nil {
		return nil
	}
	return append([]SiteStatus{}, adaptiveLast...)
}

func readAdaptiveHistory() AdaptiveStatus {
	out := adaptiveStatusBase()
	out.Note += " Сохранённые записи — история HTTP-наблюдений. Они не подтверждают завершение последней команды, текущую сессию или доступность сайтов сейчас."
	if cached := adaptiveMemory(); cached != nil {
		out.Sites = cached
		return out
	}
	cached, err := loadAdaptive()
	if err != nil {
		out.Note += " История на диске недоступна."
		return out
	}
	if cached != nil {
		out.Sites = cached
	}
	return out
}

func validAdaptiveSite(site SiteStatus) bool {
	host, err := netprobe.CanonicalDNSHost(site.Domain)
	if err != nil || host != site.Domain || len(site.Detail) > 1024 || len(site.CheckedAt) > 64 {
		return false
	}
	if _, err := time.Parse(time.RFC3339, site.CheckedAt); err != nil {
		return false
	}
	validCode := site.Code == 0 || site.Code >= 100 && site.Code <= 599
	return validCode && (!site.OK || site.Code >= 200 && site.Code < 400) &&
		(!site.Challenge || !site.OK && (site.Code == 403 || site.Code == 429))
}

func validAdaptiveSites(sites []SiteStatus) bool {
	if len(sites) > maxAdaptiveTargets {
		return false
	}
	seen := make(map[string]bool, len(sites))
	for _, site := range sites {
		if !validAdaptiveSite(site) || seen[site.Domain] {
			return false
		}
		seen[site.Domain] = true
	}
	return true
}

func persistAdaptive(sites []SiteStatus) error {
	if !validAdaptiveSites(sites) {
		return filetxn.ErrBoundedWrite
	}
	raw, err := json.MarshalIndent(sites, "", "  ")
	if err != nil || len(raw) > maxAdaptiveHistoryBytes {
		return filetxn.ErrBoundedWrite
	}
	adaptiveHistoryIO.mu.Lock()
	defer adaptiveHistoryIO.mu.Unlock()
	if !adaptiveHistoryIO.reader.Retired() {
		return filetxn.ErrReadClose // never overwrite around an uncertain original reader
	}
	return adaptiveHistoryIO.writer.Write(adaptivePath(), raw)
}

func loadAdaptive() ([]SiteStatus, error) {
	adaptiveHistoryIO.mu.Lock()
	defer adaptiveHistoryIO.mu.Unlock()
	if err := adaptiveHistoryIO.writer.Check(); err != nil {
		return nil, err
	}
	raw, err := adaptiveHistoryIO.reader.Read(adaptivePath(), maxAdaptiveHistoryBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if opening, err := decoder.Token(); err != nil || opening != json.Delim('[') {
		return nil, filetxn.ErrBoundedRead
	}
	sites := make([]SiteStatus, 0, maxAdaptiveTargets)
	for decoder.More() {
		if len(sites) >= maxAdaptiveTargets {
			return nil, filetxn.ErrBoundedRead // before decoding another record
		}
		var site SiteStatus
		if err := decoder.Decode(&site); err != nil {
			return nil, filetxn.ErrBoundedRead
		}
		sites = append(sites, site)
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim(']') || !validAdaptiveSites(sites) {
		return nil, filetxn.ErrBoundedRead
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, filetxn.ErrBoundedRead
	}
	return sites, nil
}
