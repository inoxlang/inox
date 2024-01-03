package transientqueue

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/memds"
)

func (s *TransientQueue) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	var it *memds.ArrayQueueIterator[core.Value]
	if s.threadUnsafe != nil {
		it = s.threadUnsafe.Iterator()
	} else {
		it = s.threadSafe.Iterator()
	}
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
			return it.Value()
		},
	})
}
