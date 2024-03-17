package setcoll

import (
	"maps"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (s *Set) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1

	closestState := ctx.MustGetClosestState()
	s._lock(closestState)
	defer s._unlock(closestState)

	elements := maps.Clone(s.elementByKey)

	for _, removedKey := range s.pendingRemovals {
		delete(elements, removedKey)
	}

	for _, inclusion := range s.pendingInclusions {
		elements[inclusion.key] = inclusion.value
	}

	var keys []string
	for k := range elements {
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
			return core.String(keys[i])
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return elements[keys[i]]
		},
	})
}
