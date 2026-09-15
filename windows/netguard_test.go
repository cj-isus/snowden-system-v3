package main

// Тесты чистых функций netguard.go. COM-путь (GetTask, scode) проверяется
// live-пробой (.tmp/probe-com, факты в journal V2-023); здесь — детерминизм
// маппингов и парсера, чтобы локаль не могла сломать статусы.

import (
	"bytes"
	"errors"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/go-ole/go-ole"
	"golang.org/x/text/encoding/charmap"
)

func TestNetguardTaskState(t *testing.T) {
	cases := []struct {
		in   int32
		want string
	}{
		{0, "unknown"},
		{1, "disabled"},
		{2, "queued"},
		{3, "ready"},
		{4, "running"},
		{42, "unknown:42"},
		{-1, "unknown:-1"},
	}
	for _, tc := range cases {
		if got := netguardTaskState(tc.in); got != tc.want {
			t.Errorf("netguardTaskState(%d) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestNetguardScode(t *testing.T) {
	// DISP_E_EXCEPTION-обёртка с EXCEPINFO — как отдаёт GetTask (probe 2026-09-10).
	excep := ole.NewErrorWithSubError(uintptr(0x80020009), "Ошибка.", ole.EXCEPINFO{})
	// go-ole отдаёт scode в String() EXCEPINFO; подделываем через Format:
	// структура не даёт записать scode напрямую, поэтому проверяем два пути:
	// 1) прямой HRESULT (не-exception) — читается из Code();
	// 2) отсутствие распознаваемого scode — ok=false.
	direct := ole.NewError(uintptr(0x80070005))

	sc, ok := netguardScode(direct)
	if !ok || sc != 0x80070005 {
		t.Fatalf("netguardScode(direct) = 0x%08x, %v; want 0x80070005, true", sc, ok)
	}
	// EXCEPINFO-обёртка без scode в строковом представлении — не распознаётся.
	if sc, ok := netguardScode(excep); ok {
		t.Fatalf("netguardScode(excep без scode в строке) = 0x%08x; want ok=false", sc)
	}
	if _, ok := netguardScode(errors.New("не OleError")); ok {
		t.Fatalf("netguardScode(обычная ошибка) должен вернуть ok=false")
	}
}

func TestAtoi32(t *testing.T) {
	cases := []struct {
		in string
		n  int32
		ok bool
	}{
		{"3", 3, true},
		{"0", 0, true},
		{" 4 ", 4, true},
		{"", 0, false},
		{"x", 0, false},
		{"2147483648", 0, false}, // переполнение int32
	}
	for _, tc := range cases {
		n, ok := atoi32(tc.in)
		if ok != tc.ok || (ok && n != tc.n) {
			t.Errorf("atoi32(%q) = %d, %v; want %d, %v", tc.in, n, ok, tc.n, tc.ok)
		}
	}
}

func TestNetguardLogLines(t *testing.T) {
	// cp1251-байты слова «адаптер» — то, что Windows PowerShell 5.1 кладёт в
	// лог по умолчанию на ru-системе (V2-051: мojibake на странице NetGuard).
	cp1251, err := charmap.Windows1251.NewEncoder().Bytes([]byte("адаптер"))
	if err != nil {
		t.Fatalf("cp1251 encoder: %v", err)
	}
	if utf8.Valid(cp1251) {
		t.Fatalf("cp1251-байты не должны быть валидным UTF-8")
	}
	utf16LE := func(s string) []byte {
		b := []byte{0xFF, 0xFE} // BOM
		for _, r := range utf16.Encode([]rune(s)) {
			b = append(b, byte(r), byte(r>>8))
		}
		return b
	}

	cases := []struct {
		name string
		in   []byte
		want []string
	}{
		{"utf8 as-is",
			[]byte("2026-09-10 17:54:37 HEALING: адаптер — switching"),
			[]string{"2026-09-10 17:54:37 HEALING: адаптер — switching"}},
		{"cp1251 decoded",
			append(append([]byte("2026-09-10 17:54:37 HEALING: adapter '"), cp1251...), []byte("' - switching")...),
			[]string{"2026-09-10 17:54:37 HEALING: adapter 'адаптер' - switching"}},
		{"mixed cp1251 + utf8 (перекатка писца)",
			bytes.Join([][]byte{
				append([]byte("2026-09-10 17:54:37 HEALING: '"), cp1251...),
				[]byte("2026-09-11 09:00:00 PROBLEM: адаптер"),
			}, []byte("\n")),
			[]string{"2026-09-10 17:54:37 HEALING: 'адаптер", "2026-09-11 09:00:00 PROBLEM: адаптер"}},
		{"utf8 bom stripped",
			append([]byte{0xEF, 0xBB, 0xBF}, []byte("2026-09-10 17:54:37 PROBLEM: адаптер")...),
			[]string{"2026-09-10 17:54:37 PROBLEM: адаптер"}},
		{"utf16le bom",
			utf16LE("2026-09-10 17:54:37 PROBLEM: адаптер"),
			[]string{"2026-09-10 17:54:37 PROBLEM: адаптер"}},
		{"crlf split",
			[]byte("a\r\nb"),
			[]string{"a", "b"}},
		{"ascii untouched",
			[]byte("2026-09-10 17:54:37 HEALED: stale proxy disabled"),
			[]string{"2026-09-10 17:54:37 HEALED: stale proxy disabled"}},
	}
	for _, tc := range cases {
		got := netguardLogLines(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: netguardLogLines = %q; want %q", tc.name, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: line %d = %q; want %q", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}
