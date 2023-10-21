package symbolic

import (
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ErrCannotAddNonSharableToSharedContainer = errors.New("cannot add a non sharable element to a shared container")

	ANY_CONTAINER = &AnyContainer{}

	_ = []Container{
		(*List)(nil), (*Tuple)(nil), (*Object)(nil), (*Record)(nil), (*IntRange)(nil), (*RuneRange)(nil), (*QuantityRange)(nil),

		(*AnyContainer)(nil),
	}
)

type Container interface {
	Serializable
	Iterable
	Contains(value Value) (yes bool, possible bool)
}

// An AnyContainer represents a symbolic Iterable we do not know the concrete type.
type AnyContainer struct {
	_ int
	SerializableMixin
}

func (*AnyContainer) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Container)

	return ok
}

func (*AnyContainer) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("container")
}

func (*AnyContainer) Contains(value Value) (yes bool, possible bool) {
	return false, true
}

func (*AnyContainer) WidestOfType() Value {
	return ANY_CONTAINER
}

func (*AnyContainer) IteratorElementKey() Value {
	return ANY
}

func (*AnyContainer) IteratorElementValue() Value {
	return ANY
}
