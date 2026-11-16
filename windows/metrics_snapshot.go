package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	metricsSnapshotBytes       = 2 << 20
	metricsSnapshotConnections = 2048
	metricsSnapshotStringBytes = 4096
	metricsSnapshotMembers     = 64
	metricsSnapshotDepth       = 8
	metricsSnapshotTokens      = 262144
)

var errMetricsSnapshot = errors.New("metrics snapshot unavailable or outside supported bounds")

// Bound the complete wire body before the typed decoder clones connections,
// metadata strings or chains. This is an intake bound, not a transport/FD/RSS
// retirement receipt. Adapt the existing strict JSON token-walk pattern without
// importing a trust verifier into the presentation package.
func readMetricsSnapshot(ctx context.Context, input io.Reader) (clashSnapshot, error) {
	if ctx == nil || ctx.Err() != nil || input == nil {
		return clashSnapshot{}, errMetricsSnapshot
	}
	raw, err := io.ReadAll(io.LimitReader(input, metricsSnapshotBytes+1))
	if err != nil || len(raw) > metricsSnapshotBytes || !utf8.Valid(raw) || ctx.Err() != nil {
		return clashSnapshot{}, errMetricsSnapshot
	}
	if err := checkMetricsJSON(ctx, raw); err != nil {
		return clashSnapshot{}, errMetricsSnapshot
	}
	var snapshot clashSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil || !validMetricsSnapshot(snapshot) || ctx.Err() != nil {
		return clashSnapshot{}, errMetricsSnapshot
	}
	return snapshot, nil
}

const (
	metricsJSONValue = iota
	metricsJSONRoot
	metricsJSONConnections
	metricsJSONConnection
	metricsJSONCounter
)

// Unknown SDK fields are skipped structurally within the same finite budget.
// Required counters must actually be present; omitted/null is not known zero.
func checkMetricsJSON(ctx context.Context, raw []byte) error {
	if ctx == nil || ctx.Err() != nil || len(raw) > metricsSnapshotBytes {
		return errMetricsSnapshot
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	remaining := metricsSnapshotTokens
	next := func() (json.Token, error) {
		if remaining == 0 || ctx.Err() != nil {
			return nil, errMetricsSnapshot
		}
		remaining--
		token, err := decoder.Token()
		if text, ok := token.(string); ok && len(text) > metricsSnapshotStringBytes {
			return nil, errMetricsSnapshot
		}
		if number, ok := token.(json.Number); ok && len(number) > 32 {
			return nil, errMetricsSnapshot
		}
		return token, err
	}
	var walk func(int, int) error
	walk = func(depth, role int) error {
		if depth > metricsSnapshotDepth {
			return errMetricsSnapshot
		}
		token, err := next()
		if err != nil {
			return errMetricsSnapshot
		}
		if role == metricsJSONCounter {
			number, ok := token.(json.Number)
			if !ok {
				return errMetricsSnapshot
			}
			value, err := number.Int64()
			if err != nil || value < 0 {
				return errMetricsSnapshot
			}
			return nil
		}
		delim, composite := token.(json.Delim)
		if role == metricsJSONRoot || role == metricsJSONConnection {
			if !composite || delim != '{' {
				return errMetricsSnapshot
			}
		} else if role == metricsJSONConnections {
			if token == nil {
				return nil // the SDK may encode an empty connection slice as null
			}
			if !composite || delim != '[' {
				return errMetricsSnapshot
			}
		}
		if !composite {
			return nil
		}
		if delim != '{' && delim != '[' {
			return errMetricsSnapshot
		}
		seen := make(map[string]bool)
		count, limit := 0, metricsSnapshotMembers
		if role == metricsJSONConnections {
			limit = metricsSnapshotConnections
		}
		for decoder.More() {
			if count == limit {
				return errMetricsSnapshot // before typed array growth or next value
			}
			count++
			childRole := metricsJSONValue
			if role == metricsJSONConnections {
				childRole = metricsJSONConnection
			}
			if delim == '{' {
				key, err := next()
				name, ok := key.(string)
				if err != nil || !ok || name == "" || len(name) > metricsSnapshotMembers {
					return errMetricsSnapshot
				}
				for index := 0; index < len(name); index++ {
					value := name[index]
					if !(value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_' || value == '-') {
						return errMetricsSnapshot
					}
				}
				folded := strings.ToLower(name)
				if seen[folded] {
					return errMetricsSnapshot // includes typed decoder's case aliases
				}
				seen[folded] = true
				if role == metricsJSONRoot {
					switch folded {
					case "uploadtotal", "downloadtotal":
						if name != "uploadTotal" && name != "downloadTotal" {
							return errMetricsSnapshot
						}
						childRole = metricsJSONCounter
					case "connections":
						if name != "connections" {
							return errMetricsSnapshot
						}
						childRole = metricsJSONConnections
					}
				} else if role == metricsJSONConnection && (folded == "upload" || folded == "download") {
					if name != folded {
						return errMetricsSnapshot
					}
					childRole = metricsJSONCounter
				}
			}
			if err := walk(depth+1, childRole); err != nil {
				return err
			}
		}
		end, err := next()
		if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
			return errMetricsSnapshot
		}
		if role == metricsJSONRoot && (!seen["uploadtotal"] || !seen["downloadtotal"] || !seen["connections"]) ||
			role == metricsJSONConnection && (!seen["upload"] || !seen["download"]) {
			return errMetricsSnapshot
		}
		return nil
	}
	if err := walk(0, metricsJSONRoot); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errMetricsSnapshot
	}
	return nil
}

func validMetricsSnapshot(snapshot clashSnapshot) bool {
	if snapshot.UploadTotal < 0 || snapshot.DownloadTotal < 0 || len(snapshot.Connections) > metricsSnapshotConnections {
		return false
	}
	for _, conn := range snapshot.Connections {
		if conn.Upload < 0 || conn.Download < 0 || len(conn.Chains) > metricsSnapshotMembers {
			return false
		}
		for _, value := range []string{conn.ID, conn.Metadata.Network, conn.Metadata.Host, conn.Metadata.DestinationIP, conn.Metadata.DestinationPort, conn.Metadata.ProcessPath} {
			if len(value) > metricsSnapshotStringBytes {
				return false
			}
		}
		for _, chain := range conn.Chains {
			if len(chain) > metricsSnapshotStringBytes {
				return false
			}
		}
	}
	return true
}
