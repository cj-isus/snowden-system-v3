package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// TestEngineLogWrittenAndClassified — live-гейт V2-046: ядро пишет engine-лог
// в файл (log.output), классификатор уровней разбирает каждую строку.
// Дешёвый (без сети, порт 19877 локальный), но настоящий: реальный box.Start.
func TestEngineLogWrittenAndClassified(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "engine.log")
	// WithEngineLogPath принимается рендером (проверка опции; пустой набор
	// каналов честно отказывает раньше записи лог-блока).
	if _, err := render.RenderFrom([]render.ChannelDescriptor{}, nil, render.WithEngineLogPath(logPath)); err == nil {
		t.Fatal("expected render error for empty channels")
	}
	raw, merr := json.Marshal(map[string]any{
		"log":       map[string]any{"level": "info", "output": logPath, "timestamp": false},
		"inbounds":  []map[string]any{{"type": "mixed", "tag": "in", "listen": "127.0.0.1", "listen_port": 19877}},
		"outbounds": []map[string]any{{"type": "direct", "tag": "direct"}},
	})
	if merr != nil {
		t.Fatalf("marshal: %v", merr)
	}
	e := core.NewEngine(raw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	_ = e.Stop()
	data, rerr := os.ReadFile(logPath)
	if rerr != nil {
		t.Fatalf("engine log not written: %v", rerr)
	}
	if len(data) == 0 {
		t.Fatal("engine.log is empty")
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, ln := range lines {
		if classifyEngineLine(ln) == "" {
			t.Fatalf("unclassified line %q", ln)
		}
	}
}

// TestClassifyEngineLine — таблица уровней форматтера sing-box.
func TestClassifyEngineLine(t *testing.T) {
	cases := map[string]string{
		"INFO inbound/mixed[in]: tcp server started": "info",
		"WARN outbound/vless[x]: handshake timeout":  "warn",
		"ERROR inbound/tun-in: process packet":       "error",
		"FATAL unexpected":                           "error",
		"DEBUG close connection":                     "",
		"TRACE something":                            "",
	}
	for line, want := range cases {
		if got := classifyEngineLine(line); got != want {
			t.Errorf("classifyEngineLine(%q) = %q, want %q", line, got, want)
		}
	}
}
