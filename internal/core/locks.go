package core

import (
	"context"
	"errors"

	"golang.org/x/sync/semaphore"

	"sync"
)

var (
	ErrValueNotShared = errors.New("value is not shared")
)

// A SmartLock is a lock that ignores locking operations until the value it protects is shared.
type SmartLock struct {
	lock        sync.Mutex
	valueShared bool
}

func (lock *SmartLock) IsValueShared() bool {
	return lock.valueShared
}

func (lock *SmartLock) AssertValueShared() {
	if !lock.valueShared {
		panic(ErrValueNotShared)
	}
}

func (lock *SmartLock) Share(originState *GlobalState, fn func()) {
	if lock.valueShared {
		return
	}
	lock.valueShared = true
	fn()
}

func (lock *SmartLock) Lock(state *GlobalState, embedder PotentiallySharable) {
	//IMPORTANT:
	//Locking/unlocking of SmartLock should be cheap because there are potentially thousands of operations per second.
	//No channel or goroutine should be created.

	if !lock.valueShared {
		return
	}
	//TODO: extract logic for reuse ?

	if state != nil {
		for _, e := range state.lockedValues {
			if e == embedder {
				return //already locked
			}
		}
	}

	if lock.lock.TryLock() {
		return
	}
	if state != nil {
		state.Ctx.PauseCPUTimeDepletion()
		defer state.Ctx.ResumeCPUTimeDepletion()
	}
	lock.lock.Lock()
}

func (lock *SmartLock) Unlock(state *GlobalState, embedder PotentiallySharable) {
	//IMPORTANT:
	//Locking/unlocking of SmartLock should be cheap because there are potentially thousands of operations per second.
	//No channel or goroutine should be created.

	if !lock.valueShared {
		return
	}

	if state != nil {
		for _, e := range state.lockedValues {
			if e == embedder {
				return //already locked
			}
		}
	}
	//there is no .TryLock method so for performance reasons we avoid pausing the CPU time depletion
	lock.lock.Unlock()
}

func (lock *SmartLock) ForceLock() {
	if !lock.valueShared {
		return
	}

	lock.lock.Lock()
}

func (lock *SmartLock) ForceUnlock() {
	if !lock.valueShared {
		return
	}

	lock.lock.Unlock()
}

// CMutex implements a cancelable mutex  (in fact also a try-able mutex)

type CMutex struct {
	sema *semaphore.Weighted
}

// NewCMutex is ctor for CMutex

func NewCMutex() *CMutex {

	return &CMutex{sema: semaphore.NewWeighted(1)}

}

// Lock with context

func (m *CMutex) Lock(ctx context.Context) (err error) {

	err = m.sema.Acquire(ctx, 1)

	return

}

// Unlock should only be called after a successful Lock

func (m *CMutex) Unlock() {

	m.sema.Release(1)

}

// TryLock returns true if lock acquired

func (m *CMutex) TryLock() bool {

	return m.sema.TryAcquire(1)

}
