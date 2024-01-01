package core

import (
	"errors"
	"sync"
)

var (
	ErrValueNotShared = errors.New("value is not shared")
)

// A SmartLock is a lock that ignores locking operations until the value it protects is shared.
type SmartLock struct {
	valueShared bool
	lock        sync.Mutex
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
