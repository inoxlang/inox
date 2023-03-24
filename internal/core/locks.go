package internal

import (
	"sync"
	"sync/atomic"
)

type SmartLock struct {
	valueShared atomic.Bool
	lock        sync.Mutex
}

func (lock *SmartLock) IsValueShared() bool {
	return lock.valueShared.Load()
}

func (lock *SmartLock) Share(originState *GlobalState, fn func()) {
	if lock.valueShared.CompareAndSwap(false, true) {
		fn()
	}
}

func (lock *SmartLock) Lock(state *GlobalState, embedder PotentiallySharable) {
	if !lock.valueShared.Load() {
		return
	}
	//TODO: extract logic for reuse ?

	if state != nil {
		for _, e := range state.LockedValues {
			if e == embedder {
				return //already locked
			}
		}
	}
	lock.lock.Lock()
}

func (lock *SmartLock) Unlock(state *GlobalState, embedder PotentiallySharable) {
	if !lock.valueShared.Load() {
		return
	}

	if state != nil {
		for _, e := range state.LockedValues {
			if e == embedder {
				return //already locked
			}
		}
	}
	lock.lock.Unlock()
}

func (lock *SmartLock) ForceLock() {
	if !lock.valueShared.Load() {
		return
	}

	lock.lock.Lock()
}

func (lock *SmartLock) ForceUnlock() {
	if !lock.valueShared.Load() {
		return
	}

	lock.lock.Unlock()
}
