package core

import (
	"errors"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

const (
	SMART_LOCK_HOLD_TIMEOUT = 100 * time.Millisecond
)

var (
	ErrValueNotShared        = errors.New("value is not shared")
	ErrLockReEntry           = errors.New("lock re-entry")
	ErrHeldLockWithoutHolder = errors.New("held lock without holder")
)

// A SmartLock is a lock that ignores locking operations until the value it protects is shared.
// It is not intended to be held for long durations and it should not be used to isolate transactions.
// The context of the current holder state (module) may be cancelled by other modules calling the Lock method.
type SmartLock struct {
	lockLock          sync.Mutex //protects the SmartLock's fields
	holderState       *GlobalState
	holdStart         RelativeTimeInstant64
	firstEntry        string
	totalWaitPressure ModulePriority //TODO: use max(new value, math.MaxInt32) to update this field.
	takeover          bool
	isValueShared     bool
}

func (lock *SmartLock) IsValueShared() bool {
	return lock.isValueShared
}

func (lock *SmartLock) AssertValueShared() {
	if !lock.isValueShared {
		panic(ErrValueNotShared)
	}
}

func (lock *SmartLock) Share(originState *GlobalState, fn func()) {
	if lock.isValueShared {
		return
	}
	lock.isValueShared = true
	fn()
}

// IsHeld tells whether the lock is held, regardless of the state of the holder (cancelled or not).
func (lock *SmartLock) IsHeld() bool {
	lock.lockLock.Lock()
	defer lock.lockLock.Unlock()

	return lock.holderState != nil
}

func (lock *SmartLock) Lock(state *GlobalState, embedder PotentiallySharable, ignoreLockedValues ...bool) {
	if state == nil {
		panic(errors.New("cannot lock smart lock: nil state"))
	}

	//IMPORTANT:
	//Locking/unlocking of SmartLock should be cheap because there are potentially thousands of operations per second.
	//No channel or goroutine should be created by default.

	if !lock.isValueShared {
		return
	}
	//TODO: extract logic for reuse ?

	if len(ignoreLockedValues) == 0 || !ignoreLockedValues[0] {
		for _, e := range state.lockedValues {
			if e == embedder {
				return //already locked
			}
		}
	}

	if lock.tryLockIfNoPressure(state) {
		return
	}

	state.Ctx.PauseCPUTimeDepletion()
	needResumingDepletion := true
	defer func() {
		if needResumingDepletion {
			state.Ctx.ResumeCPUTimeDepletion()
		}
	}()

	//priority := state.ComputePriority()
	//waitPressure := priority

	//TODO: give priority to modules with a high wait pressure (priority * time spent waiting).

	for {
		select {
		case <-state.Ctx.Done():
			panic(state.Ctx.Err())
		default:
			lock.lockLock.Lock()

			if lock.holderState == state {
				lock.lockLock.Unlock()
				panic(ErrLockReEntry)
			}

			// Acquire the lock if there is no holder and the wait pressure is zero.
			if lock.holderState == nil && lock.totalWaitPressure == 0 {
				func() {
					defer lock.lockLock.Unlock()
					lock.holderState = state
					lock.takeover = false
					lock.holdStart = GetRelativeTimeInstant64()
					lock.firstEntry = string(debug.Stack())
				}()
				return
			}

			//Acquire the lock if the holder is not running.
			if lock.holderState.Ctx.IsDone() {
				func() {
					defer lock.lockLock.Unlock()
					lock.holderState = state
					lock.takeover = false
					lock.holdStart = GetRelativeTimeInstant64()
					lock.firstEntry = string(debug.Stack())
				}()
				return
			}

			//Cancel the execution of the holder if it has held the lock for too long, and acquire the lock.
			if time.Since(lock.holdStart.Time()) >= SMART_LOCK_HOLD_TIMEOUT {
				func() {
					needUnlock := true
					defer func() {
						if needUnlock {
							lock.lockLock.Unlock()
						}
					}()

					prevHolder := lock.holderState

					lock.takeover = true
					lock.holderState = state
					lock.holdStart = GetRelativeTimeInstant64()
					lock.firstEntry = string(debug.Stack())

					//Release the internal lock.
					needUnlock = false
					lock.lockLock.Unlock()

					//Resume CPU time depletion early because CancelGracefully() may perform some work.
					needResumingDepletion = false
					state.Ctx.ResumeCPUTimeDepletion()

					prevHolder.Ctx.CancelGracefully()
				}()
				return
			}

			lock.lockLock.Unlock()
			runtime.Gosched()
		}
	}
}

// Acquire the lock if there is no holder and the wait pressure is zero.
func (lock *SmartLock) tryLockIfNoPressure(state *GlobalState) bool {
	lock.lockLock.Lock()
	defer lock.lockLock.Unlock()

	if lock.holderState == state {
		panic(ErrLockReEntry)
	}

	if lock.holderState == nil && lock.totalWaitPressure == 0 {
		lock.holderState = state
		lock.takeover = false
		lock.holdStart = GetRelativeTimeInstant64()
		lock.firstEntry = string(debug.Stack())
		return true
	}
	return false
}

func (lock *SmartLock) Unlock(state *GlobalState, embedder PotentiallySharable, ignoreLockedValues ...bool) {
	if state == nil {
		panic(errors.New("cannot unlock smart lock: nil state"))
	}

	//IMPORTANT:
	//Locking/unlocking of SmartLock should be cheap because there are potentially thousands of operations per second.
	//No channel or goroutine should be created by default.

	if !lock.isValueShared {
		return
	}

	if len(ignoreLockedValues) == 0 || !ignoreLockedValues[0] {
		for _, e := range state.lockedValues {
			if e == embedder {
				return //no need to unlock now.
			}
		}
	}

	lock.lockLock.Lock()

	if lock.takeover { //lock.holderState is the state taking over the lock.
		lock.takeover = false
		lock.lockLock.Unlock()
		return
	}

	if lock.holderState != state {
		lock.lockLock.Unlock()
		state.Ctx.WarnLogEvent().Msg("holder state is not the state provided for unlocking a smart lock")
		return
	}

	lock.holderState = nil
	lock.lockLock.Unlock()
}
