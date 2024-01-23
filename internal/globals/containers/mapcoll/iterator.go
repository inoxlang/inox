package mapcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (s *Map) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1
	var entries []entry

add_entries:
	for serializedKey, entry := range s.entryByKey {
		for _, removedKey := range s.pendingRemovals {
			if serializedKey == removedKey {
				continue add_entries
			}
		}
		entries = append(entries, entry)
	}

	for _, inclusion := range s.pendingInclusions {
		entries = append(entries, inclusion.entry)
	}

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			return i < len(entries)-1
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return entries[i].key
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return entries[i].value
		},
	})
}
