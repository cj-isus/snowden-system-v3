package main

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/getlantern/systray"
)

var errTrayRetirement = errors.New("исходный трей не подтвердил завершение обработчиков и native-цикла")

// systray callbacks/Quit are process-global. Never replace this original owner.
var processTray struct {
	sync.Mutex
	owner *traySession
}

type trayWorker struct {
	done     chan struct{}
	returned bool // immutable after original done, not merely WaitGroup.Done
}

type traySession struct {
	app              *App
	ctx              context.Context
	stop             chan struct{}
	readyDone        chan struct{}
	runDone          chan struct{}
	done             chan struct{}
	stopOnce         sync.Once
	workers          sync.WaitGroup
	mu               sync.Mutex
	workerTasks      []*trayWorker // sealed by readyDone, same original workers only
	workersSealed    bool          // mu; Add and original ready completion share this gate
	failed           atomic.Bool
	runEntered       atomic.Bool
	runSkipped       atomic.Bool
	runReturned      atomic.Bool
	runFrameReturned atomic.Bool
	readyEntered     atomic.Bool
	readyReturned    atomic.Bool
	exitObserved     atomic.Bool
	exitReturned     atomic.Bool
	quitAttempted    atomic.Bool
	quitReturned     atomic.Bool
	retireReturned   atomic.Bool
}

func (session *traySession) cancel() { session.stopOnce.Do(func() { close(session.stop) }) }

func (session *traySession) fail() {
	session.failed.Store(true)
	session.cancel() // value-only negative signal; no native/engine work in a fault guard
}

func (session *traySession) stopping() bool {
	if session.ctx.Err() != nil || session.app.closing.Load() || session.failed.Load() {
		return true
	}
	select {
	case <-session.stop:
		return true
	default:
		return false
	}
}

func (session *traySession) run() {
	defer close(session.runDone)
	returned := false
	defer func() {
		payload := recover()
		session.runFrameReturned.Store(returned && payload == nil)
		if !returned || payload != nil {
			session.fail()
		}
	}()
	session.runBody()
	returned = true
}

func (session *traySession) runBody() {
	defer session.cancel()
	if session.stopping() {
		session.runSkipped.Store(true)
		return // no Register/queued ready callback
	}
	runtime.LockOSThread() // keep native registration/pump on their original thread
	session.runEntered.Store(true)
	systray.Run(session.ready, session.exit)
	session.runReturned.Store(true) // actual library Run return only
}

func (session *traySession) ready() {
	if !session.readyEntered.CompareAndSwap(false, true) {
		return
	}
	defer close(session.readyDone)
	returned := false
	defer func() {
		payload := recover()
		session.mu.Lock()
		session.workersSealed = true
		session.mu.Unlock()
		session.readyReturned.Store(returned && payload == nil)
		if !returned || payload != nil {
			session.fail()
		}
	}()
	session.app.onTrayReady(session)
	returned = true // includes original ticker/menu-body defers
}

func (session *traySession) exit() {
	session.exitObserved.Store(true)
	returned := false
	defer func() {
		payload := recover()
		session.exitReturned.Store(returned && payload == nil)
		if !returned || payload != nil {
			session.fail()
		}
	}()
	session.cancel()
	returned = true
}

func (session *traySession) startWorker(call func()) {
	if session.stopping() {
		return
	}
	select {
	case <-session.readyDone:
		session.fail()
		return
	default:
	}
	worker := &trayWorker{done: make(chan struct{})}
	session.mu.Lock()
	if session.workersSealed || !session.readyEntered.Load() {
		session.mu.Unlock()
		session.fail()
		return
	}
	session.workerTasks = append(session.workerTasks, worker)
	session.workers.Add(1) // only original ready body can add before readyDone
	session.mu.Unlock()
	go func() {
		defer session.workers.Done()
		defer close(worker.done)
		defer func() {
			if payload := recover(); payload != nil || !worker.returned {
				session.fail()
			}
		}()
		call()
		worker.returned = true
	}()
}

func (session *traySession) retire() {
	returned, observed := false, false
	defer func() {
		payload := recover()
		session.retireReturned.Store(returned && payload == nil)
		if !returned || payload != nil {
			session.fail()
		}
		if observed {
			close(session.done)
		} // observed App frames only, result still required
	}()
	observed = session.retireBody()
	returned = true
}

func (session *traySession) retireBody() bool {
	select {
	case <-session.ctx.Done():
		session.cancel()
	case <-session.stop:
	}
	select {
	case <-session.readyDone:
	case <-session.runDone:
		if session.runSkipped.Load() {
			return true
		}
		<-session.readyDone // queued/unobserved ready callback remains pending
	}
	session.workers.Wait() // readyDone seals Add; each original receipt checked too
	session.mu.Lock()
	workers := append([]*trayWorker(nil), session.workerTasks...)
	session.mu.Unlock()
	for _, worker := range workers {
		<-worker.done
		if !worker.returned {
			session.failed.Store(true)
		}
	}
	if !session.exitObserved.Load() {
		session.quitAttempted.Store(true)
		systray.Quit() // no PostMessage/DestroyWindow result exposed by this API
		session.quitReturned.Store(true)
	}
	<-session.runDone
	if !session.runReturned.Load() {
		return false
	}
	if !session.exitObserved.Load() || !session.exitReturned.Load() {
		session.failed.Store(true)
	}
	return true
}

func (a *App) startTray() {
	ctx := func() context.Context {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() || a.quitRequested {
			return nil
		}
		return a.ctx
	}()
	if ctx == nil {
		return
	}
	processTray.Lock()
	if processTray.owner != nil || a.closing.Load() {
		processTray.Unlock()
		return
	}
	session := &traySession{app: a, ctx: ctx, stop: make(chan struct{}), readyDone: make(chan struct{}), runDone: make(chan struct{}), done: make(chan struct{})}
	processTray.owner = session
	processTray.Unlock()
	go session.retire()
	go session.run()
}

func (a *App) cancelTray() (<-chan struct{}, error) {
	processTray.Lock()
	session := processTray.owner
	processTray.Unlock()
	if session == nil {
		return nil, nil
	}
	if session.app != a {
		return nil, errTrayRetirement
	}
	session.cancel()
	return session.done, nil
}

func (a *App) joinTray(ctx context.Context, done <-chan struct{}) error {
	processTray.Lock()
	session := processTray.owner
	processTray.Unlock()
	if session == nil {
		if done == nil {
			return nil
		}
		return errTrayRetirement
	}
	if session.app != a || done != session.done {
		return errTrayRetirement
	}
	if err := joinAppProducerContext(ctx, done, errTrayRetirement); err != nil {
		return err
	}
	if session.failed.Load() || !session.runFrameReturned.Load() || !session.retireReturned.Load() ||
		!session.runSkipped.Load() && (!session.runReturned.Load() || !session.readyReturned.Load() || !session.exitReturned.Load()) ||
		session.quitAttempted.Load() && !session.quitReturned.Load() {
		return errTrayRetirement
	}
	return nil // observed normal App/library API frames, not native window/thread destruction
}

func (a *App) stopTray() error {
	done, err := a.cancelTray()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
	defer cancel()
	return a.joinTray(ctx, done)
}
