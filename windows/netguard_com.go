package main

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var errNetguardCOMUnavailable = errors.New("NetGuard COM observation unavailable")

const (
	netguardCOMTaskLimit = 3
	netguardCOMNameBytes = 128
)

// The VARIANT owns its dispatch reference. ToIDispatch only borrows it;
// never pair Clear with a second Release of that same borrowed reference.
type netguardCOMVariant struct {
	value          *ole.VARIANT
	clearAttempted bool
}

type netguardCOMOwner struct {
	threadLockAttempted, threadLocked        bool
	initAttempted, initReturned, initialized bool
	callPending, failed                      bool
	service                                  *ole.IUnknown
	dispatch                                 *ole.IDispatch
	reply, folder, task, property            netguardCOMVariant
}

// One synchronous observer/apartment in this process. An unknown native phase
// retains the original graph/thread custody; no timeout spawns its replacement.
var netguardCOMLane struct {
	sync.Mutex
	owner *netguardCOMOwner
}

func validNetguardTaskNames(names []string) bool {
	if len(names) == 0 || len(names) > netguardCOMTaskLimit {
		return false
	}
	for _, name := range names {
		if len(name) == 0 || len(name) > netguardCOMNameBytes || strings.IndexByte(name, 0) >= 0 {
			return false
		}
	}
	return true
}

func collectNetguardTasks(names []string) []NetGuardTask {
	if !validNetguardTaskNames(names) {
		return []NetGuardTask{}
	}
	// Bounded names/rows before copying; no caller mutates this original list.
	names = append([]string(nil), names...)
	out := make([]NetGuardTask, 0, len(names))
	err := withNetguardCOM(func(owner *netguardCOMOwner) error {
		for _, name := range names {
			task, err := owner.readTask(name)
			out = append(out, netguardTaskOutcome(name, task, err))
		}
		return nil
	})
	if err != nil {
		// No partial batch escapes failed original cleanup or apartment setup.
		out = out[:0]
		for _, name := range names {
			out = append(out, NetGuardTask{Name: name, Status: "unknown"})
		}
	}
	return out
}

// Preserve the existing one-task source seam, using the same original owner.
func comGetTask(name string) (task *NetGuardTask, err error) {
	if !validNetguardTaskNames([]string{name}) {
		return nil, errNetguardCOMUnavailable
	}
	err = withNetguardCOM(func(owner *netguardCOMOwner) error {
		var readErr error
		task, readErr = owner.readTask(name)
		return readErr // preserve GetTask HRESULT only after successful retirement
	})
	if err != nil {
		task = nil
	}
	return task, err
}

func withNetguardCOM(read func(*netguardCOMOwner) error) (err error) {
	if read == nil || !netguardCOMLane.TryLock() {
		return errNetguardCOMUnavailable
	}
	defer netguardCOMLane.Unlock()
	if netguardCOMLane.owner != nil {
		return errNetguardCOMUnavailable
	}
	owner := &netguardCOMOwner{}
	netguardCOMLane.owner = owner
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			err = errNetguardCOMUnavailable
		}
		if retirement := owner.retire(); retirement != nil {
			err = errors.Join(err, retirement)
			return // original owner, references and thread custody remain retained
		}
		netguardCOMLane.owner = nil
	}()
	owner.threadLockAttempted = true
	runtime.LockOSThread()
	owner.threadLocked = true
	owner.initAttempted = true
	initErr := owner.call(func() error { return ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED) })
	owner.initReturned = true
	// go-ole1.3.0 returns OleError(1) for S_FALSE. Both S_OK and S_FALSE
	// acquire one balanced initialization; every other error denies all COM work.
	if initErr != nil {
		if code, ok := initErr.(*ole.OleError); !ok || code.Code() != 1 {
			returned = true
			return fmt.Errorf("NetGuard apartment initialization: %w", initErr)
		}
	}
	owner.initialized = true
	if err = owner.prepare(); err == nil {
		err = read(owner)
	}
	returned = true
	return err
}

func (owner *netguardCOMOwner) call(fn func() error) error {
	if owner.callPending || owner.failed {
		return errNetguardCOMUnavailable
	}
	owner.callPending = true
	err := fn()
	owner.callPending = false // only an actual normal callback return clears this
	return err
}

func (owner *netguardCOMOwner) prepare() error {
	err := owner.call(func() error {
		var err error
		owner.service, err = oleutil.CreateObject("Schedule.Service")
		return err
	})
	if err != nil {
		return fmt.Errorf("schedule.service: %w", err)
	}
	if owner.service == nil {
		return errNetguardCOMUnavailable
	}
	err = owner.call(func() error {
		var err error
		owner.dispatch, err = owner.service.QueryInterface(ole.IID_IDispatch)
		return err
	})
	if err != nil {
		return fmt.Errorf("schedule.service QI: %w", err)
	}
	if owner.dispatch == nil {
		return errNetguardCOMUnavailable
	}
	err = owner.call(func() error {
		var err error
		owner.reply.value, err = oleutil.CallMethod(owner.dispatch, "Connect")
		return err
	})
	if err != nil {
		return fmt.Errorf("schedule.service Connect: %w", err)
	}
	if owner.reply.value == nil {
		return errNetguardCOMUnavailable
	}
	if err = owner.clear(&owner.reply); err != nil {
		return err
	}
	err = owner.call(func() error {
		var err error
		owner.folder.value, err = oleutil.CallMethod(owner.dispatch, "GetFolder", `\`)
		return err
	})
	if err != nil {
		return fmt.Errorf("schedule.service root folder: %w", err)
	}
	if netguardBorrowDispatch(owner.folder.value) == nil {
		return errNetguardCOMUnavailable
	}
	return nil
}

func netguardBorrowDispatch(value *ole.VARIANT) *ole.IDispatch {
	if value == nil || value.VT != ole.VT_DISPATCH || value.Val == 0 {
		return nil
	}
	return value.ToIDispatch()
}

func (owner *netguardCOMOwner) readTask(name string) (task *NetGuardTask, err error) {
	defer func() {
		if clearErr := owner.clear(&owner.property); clearErr != nil {
			task, err = nil, errors.Join(err, clearErr)
			return
		}
		if clearErr := owner.clear(&owner.task); clearErr != nil {
			task, err = nil, errors.Join(err, clearErr)
		}
	}()
	if owner.task.value != nil || owner.property.value != nil {
		return nil, errNetguardCOMUnavailable
	}
	folder := netguardBorrowDispatch(owner.folder.value)
	if folder == nil {
		return nil, errNetguardCOMUnavailable
	}
	err = owner.call(func() error {
		var err error
		owner.task.value, err = oleutil.CallMethod(folder, "GetTask", name)
		return err
	})
	if err != nil {
		return nil, err
	} // exact GetTask failure, not a setup heuristic
	dispatch := netguardBorrowDispatch(owner.task.value)
	if dispatch == nil {
		return nil, errNetguardCOMUnavailable
	}
	_, state, err := owner.readProperty(dispatch, "State")
	if err != nil {
		return nil, err
	}
	lastCode, _, err := owner.readProperty(dispatch, "LastTaskResult")
	if err != nil {
		return nil, err
	}
	lastRun, _, err := owner.readProperty(dispatch, "LastRunTime")
	if err != nil {
		return nil, err
	}
	nextRun, _, err := owner.readProperty(dispatch, "NextRunTime")
	if err != nil {
		return nil, err
	}
	return &NetGuardTask{Name: name, Status: netguardTaskState(int32(state)), LastCode: lastCode, LastRun: lastRun, NextRun: nextRun}, nil
}

func (owner *netguardCOMOwner) readProperty(dispatch *ole.IDispatch, name string) (text string, number int64, err error) {
	if owner.property.value != nil {
		return "", 0, errNetguardCOMUnavailable
	}
	defer func() {
		if clearErr := owner.clear(&owner.property); clearErr != nil {
			text, number, err = "", 0, errors.Join(err, clearErr)
		}
	}()
	err = owner.call(func() error {
		var err error
		owner.property.value, err = oleutil.GetProperty(dispatch, name)
		return err
	})
	if err != nil {
		return "", 0, fmt.Errorf("NetGuard task property: %w", err)
	}
	value := owner.property.value
	if value == nil {
		return "", 0, errNetguardCOMUnavailable
	}
	switch name {
	case "State", "LastTaskResult":
		switch value.VT {
		case ole.VT_I4:
			number = int64(int32(value.Val))
		case ole.VT_UI4:
			number = int64(uint32(value.Val))
		default:
			return "", 0, errNetguardCOMUnavailable
		}
		if name == "State" && (number < 0 || number > math.MaxInt32) {
			return "", 0, errNetguardCOMUnavailable
		}
		return strconv.FormatInt(number, 10), number, nil
	case "LastRunTime", "NextRunTime":
		if value.VT != ole.VT_DATE {
			return "", 0, errNetguardCOMUnavailable
		}
		date := math.Float64frombits(uint64(value.Val))
		if math.IsNaN(date) || math.IsInf(date, 0) {
			return "", 0, errNetguardCOMUnavailable
		}
		if date == 0 {
			return "", 0, nil
		} // explicit OLE DATE zero, not missing data
		var observed time.Time
		err = owner.call(func() error {
			var err error
			observed, err = ole.GetVariantDate(uint64(value.Val))
			return err
		})
		if err != nil || observed.Year() < 1 || observed.Year() > 9999 {
			return "", 0, errNetguardCOMUnavailable
		}
		return observed.Format(time.RFC3339), 0, nil
	default:
		return "", 0, errNetguardCOMUnavailable
	}
}

func (owner *netguardCOMOwner) clear(slot *netguardCOMVariant) error {
	if owner.callPending || owner.failed || slot.clearAttempted {
		return errNetguardCOMUnavailable
	}
	if slot.value == nil {
		return nil
	}
	slot.clearAttempted = true
	err := owner.call(func() error { return slot.value.Clear() })
	if err != nil {
		owner.failed = true // retain original VARIANT and all dependent parents
		return errNetguardCOMUnavailable
	}
	*slot = netguardCOMVariant{}
	return nil
}

func (owner *netguardCOMOwner) retire() (err error) {
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			owner.failed, err = true, errNetguardCOMUnavailable
		}
	}()
	if owner.callPending || owner.failed || owner.threadLockAttempted && !owner.threadLocked || owner.initAttempted && !owner.initReturned {
		returned = true
		return errNetguardCOMUnavailable
	}
	for _, slot := range []*netguardCOMVariant{&owner.property, &owner.task, &owner.folder, &owner.reply} {
		if err = owner.clear(slot); err != nil {
			returned = true
			return err
		}
	}
	if owner.dispatch != nil {
		if err = owner.call(func() error { owner.dispatch.Release(); owner.dispatch = nil; return nil }); err != nil {
			returned = true
			return err
		}
	}
	if owner.service != nil {
		if err = owner.call(func() error { owner.service.Release(); owner.service = nil; return nil }); err != nil {
			returned = true
			return err
		}
	}
	if owner.initialized {
		if err = owner.call(func() error { ole.CoUninitialize(); owner.initialized = false; return nil }); err != nil {
			returned = true
			return err
		}
	}
	if owner.threadLocked {
		if err = owner.call(func() error { runtime.UnlockOSThread(); owner.threadLocked = false; return nil }); err != nil {
			returned = true
			return err
		}
	}
	returned = true
	return nil
}
