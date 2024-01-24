package threadcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (s *MessageThread) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			return i < len(s.elements)-1
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(i)
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return s.elements[i].actualElement
		},
	})
}
