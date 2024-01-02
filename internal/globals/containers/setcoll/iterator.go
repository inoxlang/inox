package setcoll

import (
	"maps"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (s *Set) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1

	closestState := ctx.GetClosestState()
	s.lock.Lock(closestState, s)
	defer s.lock.Unlock(closestState, s)

	elements := maps.Clone(s.elementByKey)

	var keys []string
	for k := range s.elementByKey {
		keys = append(keys, k)
	}

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			return i < len(keys)-1
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return core.Str(keys[i])
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return elements[keys[i]]
		},
	})
}
