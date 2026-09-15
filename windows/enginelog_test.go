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

// ---------- V2-054b: tailer без повторов ----------

// rewriteFile — перезаписать файл целиком (эмуляция записи ядра/ротации).
func rewriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func texts(msgs []engineLogMsg) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.text)
	}
	return out
}

func wantLines(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("delivered %d lines %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line[%d] = %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
}

// Инкрементальность — центральная гарантия V2-054b: повторное чтение файла
// доставляет только НОВЫЕ строки (регресс «240 дублей одной ошибки»).
func TestReadNewEngineLinesIncremental(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.log")
	st := &tailerState{}

	// Файла нет — честное «новых нет», без ошибки.
	if got := readNewEngineLines(path, st); len(got) != 0 {
		t.Fatalf("absent file: got %q", texts(got))
	}

	rewriteFile(t, path, "INFO a\nERROR b\nDEBUG quiet\n")
	wantLines(t, texts(readNewEngineLines(path, st)),
		[]string{"движок INFO a", "движок ERROR b"}) // debug не доставляется

	// Дозапись: только новая строка, старые НЕ переигрываются.
	rewriteFile(t, path, "INFO a\nERROR b\nDEBUG quiet\nINFO c\n")
	wantLines(t, texts(readNewEngineLines(path, st)), []string{"движок INFO c"})

	// Ротация/пересоздание (усечение — файл стал КОРОЧЕ): новый файл —
	// читаем с начала и доставляем ЦЕЛИКОМ (last-гвард сброшен): строки
	// новой сессии движка — новые факты, даже если текст совпадает.
	rewriteFile(t, path, "INFO a\nINFO x\n")
	wantLines(t, texts(readNewEngineLines(path, st)), []string{"движок INFO a", "движок INFO x"})

	// Дозапись без усечения: только новые строки; штатный повтор строки
	// самим ядром (деградация) гасится подряд-гвардом — одна строка.
	rewriteFile(t, path, "INFO a\nINFO x\nINFO x\nINFO x\nINFO y\n")
	wantLines(t, texts(readNewEngineLines(path, st)), []string{"движок INFO y"})

	// Снова усечение: новый файл доставляется целиком (включая
	// не-подрядный повтор x — гвард действует только на ПОДРЯД дубликаты).
	rewriteFile(t, path, "INFO a\nINFO x\nINFO y\nINFO x\n")
	wantLines(t, texts(readNewEngineLines(path, st)), []string{"движок INFO a", "движок INFO x", "движок INFO y", "движок INFO x"})
}

// Частичная строка (ядро пишет без \n, дозапись следующим Systemd-броском):
// недописанный хвост не доставляется и не ломает оффсет; после дописки
// строка доставляется ровно один раз.
func TestReadNewEngineLinesPartialLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.log")
	st := &tailerState{}

	rewriteFile(t, path, "INFO a\nERROR bom") // без \n
	wantLines(t, texts(readNewEngineLines(path, st)), []string{"движок INFO a"})

	rewriteFile(t, path, "INFO a\nERROR bombed out\n")
	wantLines(t, texts(readNewEngineLines(path, st)), []string{"движок ERROR bombed out"})
}

// Живой цикл tailer'а: доставляет новые строки в журнал и останавливается
// по engineLogDone. Путь подменён на временную директорию (герметичность),
// logFileCh подменён (без записи в AppData).
func TestStartEngineLogTailerDeliversAndStops(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.log")
	old := engineLogPathAt
	engineLogPathAt = func() string { return path }
	t.Cleanup(func() { engineLogPathAt = old })

	a := &App{}
	a.engineLogDone = make(chan struct{})
	fileCh := make(chan LogLine, 256)
	a.logFileCh = fileCh
	a.startEngineLogTailer()
	t.Cleanup(func() { a.stopEngineLogTailer() })

	rewriteFile(t, path, "ERROR force closed via ClientConn.Close\nINFO started\n")

	deadline := time.Now().Add(3 * time.Second)
	var got []string
	for time.Now().Before(deadline) {
		select {
		case ln := <-fileCh:
			got = append(got, ln.Text)
			if len(got) == 2 {
				goto delivered
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Fatalf("tailer did not deliver 2 lines in time: %q", got)
delivered:
	wantLines(t, got, []string{
		"движок ERROR force closed via ClientConn.Close",
		"движок INFO started",
	})
	// Второй проход по тому же файлу — ноль доставок (нет повторов).
	time.Sleep(700 * time.Millisecond)
	select {
	case ln := <-fileCh:
		t.Fatalf("unexpected replay: %q", ln.Text)
	case <-time.After(300 * time.Millisecond):
	}
}
