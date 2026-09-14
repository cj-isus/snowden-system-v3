package main

// Тесты чистых функций netguard.go. COM-путь (GetTask, scode) проверяется
// live-пробой (.tmp/probe-com, факты в journal V2-023); здесь — детерминизм
// маппингов и парсера, чтобы локаль не могла сломать статусы.

import (
	"errors"
	"testing"

	"github.com/go-ole/go-ole"
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
