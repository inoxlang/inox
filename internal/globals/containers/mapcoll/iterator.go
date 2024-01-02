package mapcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (s *Map) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1
	var ids []core.FastId
	for k := range s.values {
		ids = append(ids, k)
	}

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			return i < len(ids)-1
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return s.keys[ids[i]]
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return s.values[ids[i]]
		},
	})
}
