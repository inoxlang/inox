package transientqueue

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
)

// PotentiallySharable impl for TransientQueue

func (*TransientQueue) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (q *TransientQueue) Share(originState *core.GlobalState) {
	threadSafeQueue := memds.NewTSArrayQueue[core.Value]()

	q.threadUnsafe.ForEachElem(func(i int, e core.Value) error {
		shared, err := core.ShareOrClone(e, originState)
		if err != nil {
			panic(err)
		}
		threadSafeQueue.Enqueue(shared)
		return nil
	})

	q.threadUnsafe = nil
	q.threadSafe = threadSafeQueue
}

func (q *TransientQueue) IsShared() bool {
	return q.threadSafe != nil
}

func (*TransientQueue) SmartLock(state *core.GlobalState) {
}

func (*TransientQueue) SmartUnlock(state *core.GlobalState) {
}
