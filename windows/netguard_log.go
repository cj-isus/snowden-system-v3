package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Resource policy for this reader only; not measured file size or OS I/O limits.
const (
	netguardLogTailBytes     = 64 * 1024
	netguardLogEncodingBytes = 256
	netguardLogReadBytes     = netguardLogTailBytes + netguardLogEncodingBytes
	netguardLogRows          = 256
	netguardLogLineBytes     = 4096
)

// Counts describe an observed fragment only. Other states have unknown content,
// not an empty history. No field is recovery, DNS or protection authority.
type NetGuardLogObservation struct {
	Schema     int    `json:"schema"`
	State      string `json:"state"` // observed | missing | busy | unavailable
	BytesRead  int    `json:"bytesRead"`
	ByteLimit  int    `json:"byteLimit"`
	Rows       int    `json:"rows"`
	Truncated  bool   `json:"truncated"`
	Incomplete bool   `json:"incomplete"`
}

type netguardLogRead struct{ originalLogFile }

// One process-local original logging owner; an unreturned Open/I/O/Close cannot
// be replaced by another polling call. This is not an interprocess file lock.
var netguardLogLane struct {
	sync.Mutex
	owner *netguardLogRead
}

func netguardLogObservation(state string) NetGuardLogObservation {
	return NetGuardLogObservation{Schema: 1, State: state, ByteLimit: netguardLogReadBytes}
}

func readNetguardLog() (lines []string, observation NetGuardLogObservation) {
	observation = netguardLogObservation("unavailable")
	base := os.Getenv("ProgramData")
	if len(base) == 0 || len(base) > 32700 || strings.IndexByte(base, 0) >= 0 || !filepath.IsAbs(base) {
		return nil, observation // never silently read a relative working-directory log
	}
	if !netguardLogLane.TryLock() {
		return nil, netguardLogObservation("busy")
	}
	defer netguardLogLane.Unlock()
	if netguardLogLane.owner != nil {
		return nil, observation // original unknown retirement remains retained
	}
	op := &netguardLogRead{}
	netguardLogLane.owner = op // publish custody before Open can acquire anything
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			lines, observation = nil, netguardLogObservation("unavailable")
		}
		if err := op.retire(); err != nil {
			lines, observation = nil, netguardLogObservation("unavailable")
			return // do not clear or retry an uncertain original owner
		}
		netguardLogLane.owner = nil
	}()
	if err := op.openFile(filepath.Join(base, "netguard", "netguard.log"), os.O_RDONLY, 0); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			observation = netguardLogObservation("missing")
		}
		returned = true
		return nil, observation
	}
	op.ioPending = true
	window, err := op.window()
	op.ioPending = false // only the actual synchronous I/O return retires this phase
	if err != nil {
		returned = true
		return nil, observation
	}
	lines, window.truncated, window.incomplete = netguardLogRecords(window)
	observation = netguardLogObservation("observed")
	observation.BytesRead, observation.Rows = window.bytesRead, len(lines)
	observation.Truncated, observation.Incomplete = window.truncated, window.incomplete
	returned = true
	return lines, observation // defer still requires original Close before publication
}

type netguardLogWindow struct {
	data                  []byte
	utf16LE               bool
	truncated, incomplete bool
	bytesRead             int
}

func (op *netguardLogRead) window() (window netguardLogWindow, err error) {
	info, err := op.file.Stat()
	if err != nil {
		return window, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return window, errLogFileRetirement
	}
	size := info.Size()
	prefixSize := min(size, int64(netguardLogEncodingBytes))
	prefix := make([]byte, int(prefixSize))
	if err = netguardReadLogAt(op.file, prefix, 0); err != nil {
		return window, err
	}
	window.bytesRead = len(prefix)
	window.utf16LE = netguardUTF16LE(prefix[:len(prefix)&^1])
	start := max(int64(0), size-netguardLogTailBytes)
	if window.utf16LE && start%2 != 0 {
		start++
	}
	window.truncated = start > 0
	window.data = make([]byte, int(size-start))
	if err = netguardReadLogAt(op.file, window.data, start); err != nil {
		return window, err
	}
	window.bytesRead += len(window.data)
	// Appends are allowed. A short read or a smaller final size cannot become
	// an empty successful history. This is not a coherent whole-file snapshot.
	after, err := op.file.Stat()
	if err != nil {
		return window, err
	}
	if !after.Mode().IsRegular() || after.Size() < size {
		return window, io.ErrUnexpectedEOF
	}
	if window.utf16LE && len(window.data)%2 != 0 {
		window.data = window.data[:len(window.data)-1]
		window.incomplete = true
	}
	return window, nil
}

func netguardReadLogAt(file *os.File, data []byte, offset int64) error {
	if len(data) == 0 {
		return nil
	}
	n, err := file.ReadAt(data, offset)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// Trim partial records and cap rows before the legacy multi-encoding decoder
// allocates its string/split output. Oversized decoded records are omitted whole.
func netguardLogRecords(window netguardLogWindow) ([]string, bool, bool) {
	data, step := window.data, 1
	if window.utf16LE {
		step = 2
	}
	newline := func(i int) bool { return data[i] == '\n' && (step == 1 || data[i+1] == 0) }
	if window.truncated {
		first := -1
		for i := 0; i+step <= len(data); i += step {
			if newline(i) {
				first = i + step
				break
			}
		}
		window.incomplete = true
		if first < 0 {
			return nil, true, true
		}
		data = data[first:]
	}
	last := -1
	for i := len(data) - step; i >= 0; i -= step {
		if newline(i) {
			last = i + step
			break
		}
	}
	if last < 0 {
		return nil, window.truncated, window.incomplete || len(data) > 0
	}
	if last != len(data) {
		window.incomplete = true
		data = data[:last]
	}
	rows := 0
	for i := len(data) - step; i >= 0; i -= step {
		if !newline(i) {
			continue
		}
		rows++
		if rows > netguardLogRows {
			data, window.truncated = data[i+step:], true
			break
		}
	}
	if window.utf16LE && !(len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE) {
		marked := make([]byte, len(data)+2)
		marked[0], marked[1] = 0xFF, 0xFE
		copy(marked[2:], data)
		data = marked
	}
	decoded := netguardLogLines(data)
	lines := make([]string, 0, min(len(decoded), netguardLogRows))
	for _, line := range decoded {
		if line == "" {
			continue
		}
		if len(line) > netguardLogLineBytes {
			window.incomplete = true
			continue
		}
		lines = append(lines, line)
	}
	return lines, window.truncated, window.incomplete
}
