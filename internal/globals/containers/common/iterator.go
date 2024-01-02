package common

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

type CollectionIterator struct {
	HasNext_ func(*CollectionIterator, *core.Context) bool
	Next_    func(*CollectionIterator, *core.Context) bool
	Key_     func(*CollectionIterator, *core.Context) core.Value
	Value_   func(*CollectionIterator, *core.Context) core.Value

	_key   core.Value
	_value core.Value
}

func (it *CollectionIterator) HasNext(ctx *core.Context) bool {
	return it.HasNext_(it, ctx)
}

func (it *CollectionIterator) Next(ctx *core.Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	return it.Next_(it, ctx)
}

func (it *CollectionIterator) Key(ctx *core.Context) core.Value {
	return it.Key_(it, ctx)
}

func (it *CollectionIterator) Value(ctx *core.Context) core.Value {
	return it.Value_(it, ctx)
}

func (it *CollectionIterator) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return it
}

func (it *CollectionIterator) IsMutable() bool {
	return true
}

func (it *CollectionIterator) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*CollectionIterator)
	return ok && it == otherIt
}

func (it *CollectionIterator) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *CollectionIterator) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", it))
}
