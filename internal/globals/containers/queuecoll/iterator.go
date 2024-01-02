package queuecoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (s *Queue) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	it := s.elements.Iterator()
	var next core.Value

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			if next == nil {
				if !it.Next() {
					return false
				}
				next = it.Value()
			}
			return true
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			next = nil
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(it.Index())
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return it.Value().(core.Value)
		},
	})
}
