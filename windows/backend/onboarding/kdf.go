package onboarding

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/crypto/scrypt"
)

const (
	maxKDFMemory       = 96 << 20
	maxKDFWork         = 1 << 21 // N*r*p units, not milliseconds
	maxActiveKDFMemory = 128 << 20
	maxActiveKDFWork   = 1 << 22
	maxActiveKDFs      = 2
	// MaxPassphraseSize bounds password encoding and PBKDF2 input processing.
	MaxPassphraseSize = 4096
)

var (
	ErrKDFLimit   = errors.New("onboarding: параметры scrypt превышают допустимую стоимость")
	ErrKDFBusy    = errors.New("onboarding: дождитесь завершения текущих криптографических операций")
	ErrKDFCompute = errors.New("onboarding: ошибка выработки ключа")
)

type kdfCost struct{ memory, work uint64 }
type keyFunction func(password, salt []byte, n, r, p, keyLen int) ([]byte, error)

// The pinned x/crypto v0.51.0 allocates v=128*N*r, xy=256*r and
// b=128*r*p bytes. This bounds estimated active workspace, not total Go RSS.
func estimateKDF(n, r, p int) (kdfCost, error) {
	if n < 1<<14 || n&(n-1) != 0 || r < 8 || p < 1 {
		return kdfCost{}, ErrBadFormat
	}
	rp, ok := checkedMul(uint64(r), uint64(p))
	if !ok || rp >= 1<<30 {
		return kdfCost{}, kdfLimitError()
	}
	sum, ok := checkedAdd(uint64(n), uint64(p))
	if !ok {
		return kdfCost{}, kdfLimitError()
	}
	sum, ok = checkedAdd(sum, 2)
	if !ok {
		return kdfCost{}, kdfLimitError()
	}
	block, ok := checkedMul(128, uint64(r))
	if !ok {
		return kdfCost{}, kdfLimitError()
	}
	memory, ok := checkedMul(block, sum)
	if !ok {
		return kdfCost{}, kdfLimitError()
	}
	work, ok := checkedMul(uint64(n), rp)
	if !ok {
		return kdfCost{}, kdfLimitError()
	}
	if memory > maxKDFMemory || work > maxKDFWork {
		return kdfCost{}, kdfLimitError()
	}
	return kdfCost{memory: memory, work: work}, nil
}

func kdfLimitError() error { return fmt.Errorf("%w: %w", ErrBadFormat, ErrKDFLimit) }
func checkedMul(a, b uint64) (uint64, bool) {
	if a != 0 && b > ^uint64(0)/a {
		return 0, false
	}
	return a * b, true
}
func checkedAdd(a, b uint64) (uint64, bool) {
	if a > ^uint64(0)-b {
		return 0, false
	}
	return a + b, true
}

type kdfGate struct {
	mu      sync.Mutex
	active  int
	used    kdfCost
	max     kdfCost
	workers int
}

func newKDFGate() *kdfGate {
	return &kdfGate{max: kdfCost{maxActiveKDFMemory, maxActiveKDFWork}, workers: maxActiveKDFs}
}

// Fail fast: no wait queue, goroutine or untrusted allocation on rejection.
func (g *kdfGate) acquire(cost kdfCost) (func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active >= g.workers || cost.memory > g.max.memory || cost.work > g.max.work || g.used.memory > g.max.memory-cost.memory || g.used.work > g.max.work-cost.work {
		return nil, ErrKDFBusy
	}
	g.active++
	g.used.memory += cost.memory
	g.used.work += cost.work
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.active--
			g.used.memory -= cost.memory
			g.used.work -= cost.work
		})
	}, nil
}

type kdfExecutor struct {
	gate *kdfGate
	key  keyFunction
}

func (e *kdfExecutor) derive(password string, salt []byte, n, r, p int) (key []byte, err error) {
	if len(password) > MaxPassphraseSize {
		return nil, ErrTooLarge
	}
	cost, err := estimateKDF(n, r, p)
	if err != nil {
		return nil, err
	}
	if e == nil || e.gate == nil || e.key == nil {
		return nil, ErrKDFCompute
	}
	release, err := e.gate.acquire(cost)
	if err != nil {
		return nil, err
	}
	defer release()
	defer func() {
		if recover() != nil {
			clear(key)
			key = nil
			err = ErrKDFCompute
		}
	}()
	key, err = e.key([]byte(password), salt, n, r, p, keyLen)
	if err != nil || len(key) != keyLen {
		clear(key)
		return nil, ErrKDFCompute
	}
	return key, nil
}

// Seal and Open share this immutable executor and its process-local gate.
var defaultKDF = &kdfExecutor{gate: newKDFGate(), key: scrypt.Key}
