package transientcontainers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
)

var (
	QUEUE_PROPNAMES = []string{"enqueue", "dequeue", "peek"}
	_               = []symbolic.Iterable{(*TransientQueue)(nil)}
	_               = []symbolic.PotentiallySharable{(*TransientQueue)(nil)}
)

type TransientQueue struct {
	symbolic.UnassignablePropsMixin
	shared bool
}

func (*TransientQueue) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*TransientQueue)
	return ok
}

func (*TransientQueue) Enqueue(ctx *symbolic.Context, elem symbolic.Value) {
	if ok, reason := symbolic.IsSharableOrClonable(elem); !ok {
		if reason != "" {
			reason = ": " + reason
		}
		ctx.AddSymbolicGoFunctionError("passed value is not sharable nor clonable" + reason)
	}
}

func (*TransientQueue) Dequeue(ctx *symbolic.Context) (symbolic.Value, *symbolic.Bool) {
	return symbolic.ANY, nil
}

func (*TransientQueue) Peek(ctx *symbolic.Context) (symbolic.Value, *symbolic.Bool) {
	return symbolic.ANY, nil
}

func (*TransientQueue) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (*TransientQueue) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (*TransientQueue) WidestOfType() symbolic.Value {
	return &TransientQueue{}
}
