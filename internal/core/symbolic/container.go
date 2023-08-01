package symbolic

import (
	"bufio"
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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
	Contains(value SymbolicValue) (yes bool, possible bool)
}

// An AnyContainer represents a symbolic Iterable we do not know the concrete type.
type AnyContainer struct {
	_ int
	SerializableMixin
}

func (*AnyContainer) Test(v SymbolicValue) bool {
	_, ok := v.(Container)

	return ok
}

func (*AnyContainer) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%container")))
}

func (*AnyContainer) Contains(value SymbolicValue) (yes bool, possible bool) {
	return false, true
}

func (*AnyContainer) WidestOfType() SymbolicValue {
	return ANY_CONTAINER
}

func (*AnyContainer) IteratorElementKey() SymbolicValue {
	return ANY
}

func (*AnyContainer) IteratorElementValue() SymbolicValue {
	return ANY
}
