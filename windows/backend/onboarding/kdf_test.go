package onboarding

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fakeExecutor(fn keyFunction) *kdfExecutor {
	if fn == nil {
		fn = func([]byte, []byte, int, int, int, int) ([]byte, error) { return make([]byte, keyLen), nil }
	}
	return &kdfExecutor{gate: newKDFGate(), key: fn}
}

func assertGateEmpty(t *testing.T, g *kdfGate) {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active != 0 || g.used != (kdfCost{}) {
		t.Fatal("KDF reservation leaked")
	}
}

func TestKDFCostRejectsBeforeCompute(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name    string
		n, r, p int
	}{
		{"two-gib-workspace", 1 << 21, 8, 1}, {"large-block-workspace", 1 << 14, 4096, 1}, {"excessive-work", 1 << 15, 8, 1024},
		{"zero-n", 0, 8, 1}, {"negative-n", -1, 8, 1}, {"weak-n", 1 << 13, 8, 1}, {"non-power-of-two", 20000, 8, 1},
		{"weak-r", 1 << 15, 7, 1}, {"negative-r", 1 << 15, -8, 1}, {"zero-p", 1 << 15, 8, 0}, {"negative-p", 1 << 15, 8, -1},
		{"overflow-rp", 1 << 15, maxInt, maxInt}, {"huge-n", maxInt, 8, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			e := fakeExecutor(func([]byte, []byte, int, int, int, int) ([]byte, error) { calls++; return make([]byte, keyLen), nil })
			if key, err := e.derive("synthetic-passphrase", make([]byte, 32), tc.n, tc.r, tc.p); err == nil || key != nil {
				t.Fatal("unsafe cost accepted")
			}
			if calls != 0 {
				t.Fatal("unsafe cost reached KDF")
			}
			assertGateEmpty(t, e.gate)
		})
	}
}

func TestKDFCostAcceptedAndAccounted(t *testing.T) {
	for _, tc := range []struct {
		name    string
		n, r, p int
	}{
		{"legacy-min", 1 << 14, 8, 1}, {"export-default", 1 << 15, 8, 1}, {"larger-memory", 1 << 16, 8, 1}, {"work-boundary", 1 << 15, 8, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cost, err := estimateKDF(tc.n, tc.r, tc.p)
			if err != nil {
				t.Fatal(err)
			}
			wantMemory := uint64(128) * uint64(tc.r) * (uint64(tc.n) + uint64(tc.p) + 2)
			if cost.memory != wantMemory || cost.work != uint64(tc.n)*uint64(tc.r)*uint64(tc.p) {
				t.Fatal("allocation estimate drifted")
			}
			e := fakeExecutor(nil)
			e.key = func([]byte, []byte, int, int, int, int) ([]byte, error) {
				e.gate.mu.Lock()
				defer e.gate.mu.Unlock()
				if e.gate.active != 1 || e.gate.used != cost {
					t.Fatal("KDF executed without reservation")
				}
				return make([]byte, keyLen), nil
			}
			key, err := e.derive("synthetic-passphrase", make([]byte, 32), tc.n, tc.r, tc.p)
			if err != nil || len(key) != keyLen {
				t.Fatal(err)
			}
			assertGateEmpty(t, e.gate)
		})
	}
}

func TestKDFCheckedArithmetic(t *testing.T) {
	if _, ok := checkedMul(^uint64(0), 2); ok {
		t.Fatal("multiply overflow accepted")
	}
	if _, ok := checkedAdd(^uint64(0), 1); ok {
		t.Fatal("addition overflow accepted")
	}
	if v, ok := checkedMul(0, ^uint64(0)); !ok || v != 0 {
		t.Fatal("zero product")
	}
	if v, ok := checkedAdd(^uint64(0)-1, 1); !ok || v != ^uint64(0) {
		t.Fatal("exact max sum")
	}
}

func TestKDFReservationFinalizer(t *testing.T) {
	for _, name := range []string{"error", "panic", "nil-key", "short-key", "long-key"} {
		t.Run(name, func(t *testing.T) {
			synthetic := []byte("synthetic-private-key-material!!!")
			e := fakeExecutor(func([]byte, []byte, int, int, int, int) ([]byte, error) {
				switch name {
				case "error":
					return synthetic, errors.New("synthetic-private-error")
				case "panic":
					panic("synthetic-private-panic")
				case "nil-key":
					return nil, nil
				case "short-key":
					return make([]byte, 16), nil
				default:
					return make([]byte, 33), nil
				}
			})
			key, err := e.derive("synthetic-passphrase", make([]byte, 32), scryptN, scryptR, scryptP)
			if !errors.Is(err, ErrKDFCompute) || key != nil || strings.Contains(err.Error(), "private") {
				t.Fatalf("fault result: %v", err)
			}
			assertGateEmpty(t, e.gate)
			if name == "error" {
				for _, b := range synthetic {
					if b != 0 {
						t.Fatal("failed key not cleared")
					}
				}
			}
			e.key = func([]byte, []byte, int, int, int, int) ([]byte, error) { return make([]byte, keyLen), nil }
			if _, err := e.derive("synthetic-passphrase", make([]byte, 32), scryptN, scryptR, scryptP); err != nil {
				t.Fatal("slot not reusable:", err)
			}
		})
	}
}

func TestKDFGateLimits(t *testing.T) {
	for _, name := range []string{"workers", "memory", "work"} {
		t.Run(name, func(t *testing.T) {
			g := newKDFGate()
			cost, _ := estimateKDF(scryptN, scryptR, scryptP)
			switch name {
			case "workers":
				g.workers = 1
			case "memory":
				g.max.memory = cost.memory*2 - 1
			case "work":
				g.max.work = cost.work*2 - 1
			}
			release, err := g.acquire(cost)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := g.acquire(cost); !errors.Is(err, ErrKDFBusy) {
				t.Fatal("aggregate limit ignored")
			}
			release()
			release()
			assertGateEmpty(t, g)
		})
	}
}

func TestKDFSealAndOpenShareAdmission(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var closed atomic.Bool
	finish := func() {
		if closed.CompareAndSwap(false, true) {
			close(release)
		}
	}
	t.Cleanup(finish)
	var calls atomic.Int32
	e := fakeExecutor(func([]byte, []byte, int, int, int, int) ([]byte, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		return make([]byte, keyLen), nil
	})
	c := bundleCodec{kdf: e}
	header := testHeaderTransport(t, nil)
	results := make(chan error, 2)
	go func() { _, err := c.seal(&BundlePayload{Schema: 1}, "synthetic-passphrase"); results <- err }()
	go func() { _, err := c.open(header, "synthetic-passphrase"); results <- err }()
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("worker not admitted")
		}
	}
	third := make(chan error, 1)
	go func() { _, err := c.open(header, "synthetic-passphrase"); third <- err }()
	select {
	case err := <-third:
		if !errors.Is(err, ErrKDFBusy) {
			t.Fatalf("busy result: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("unbounded KDF wait queue")
	}
	if calls.Load() != 2 {
		t.Fatal("busy operation invoked KDF")
	}
	finish()
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err != nil && !errors.Is(err, ErrWrongPassphrase) {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("worker did not finish")
		}
	}
	assertGateEmpty(t, e.gate)
}

func TestKDFConcurrentMemoryCapacity(t *testing.T) {
	g := newKDFGate()
	cost, err := estimateKDF(1<<16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	release, err := g.acquire(cost)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := g.acquire(cost); !errors.Is(err, ErrKDFBusy) {
		t.Fatal(fmt.Sprint("two larger workspaces exceed aggregate memory: ", err))
	}
}
