package transientcontainers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
)

func (q *TransientQueue) IsMutable() bool {
	return true
}

func (q *TransientQueue) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "enqueue":
		return symbolic.WrapGoMethod(q.Enqueue), true
	case "dequeue":
		return symbolic.WrapGoMethod(q.Dequeue), true
	case "peek":
		return symbolic.WrapGoMethod(q.Peek), true
	}
	return nil, false
}

func (q *TransientQueue) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, q)
}

func (*TransientQueue) PropertyNames() []string {
	return QUEUE_PROPNAMES
}

func (*TransientQueue) PrettyPrint(w prettyprint.PrettyPrintWriter, config *prettyprint.PrettyPrintConfig) {
	w.WriteName("transient-queue")
}

func (q *TransientQueue) IsSharable() (bool, string) {
	return true, ""
}

func (q *TransientQueue) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	copy := *q
	copy.shared = true
	return &copy
}

func (q *TransientQueue) IsShared() bool {
	return q.shared
}
