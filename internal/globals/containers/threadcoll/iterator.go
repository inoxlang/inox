package threadcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

const (
	MAX_ITERATOR_THREAD_SEGMENT_SIZE = 10
)

// Iterator returns a thread-unsafe iterator that starts at the current most recently added (last) element, the type of keys is core.ULID.
// The iterators iterates over the MessageThread, not over a snapshot
func (t *MessageThread) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	closestState := ctx.GetClosestState()
	t._lock(closestState)
	defer t._unlock(closestState)

	core.NewEmptyPatternIterator()
	cursor := core.MAX_ULID

	const MAX_SEGMENT_SIZE = MAX_ITERATOR_THREAD_SEGMENT_SIZE
	segment := make([]internalElement, 0, MAX_SEGMENT_SIZE)
	first := true

	var currentElement internalElement

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			if !first && len(segment) > 0 {
				return true
			}
			first = false
			segment = segment[:0]
			t.getElementsBefore(ctx, cursor, MAX_SEGMENT_SIZE, &segment)
			//Only the elements visible by $ctx's tx should be present so no need to postprocess $segment.

			if len(segment) == 0 {
				return false
			}

			//Set the cursor to the creation time of the oldest's element in the segment.
			cursor = segment[len(segment)-1].ulid
			return true
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			//We assume that HasNext_ is called before Next(), therefore $segment should have been updated.
			currentElement = segment[0]
			copy(segment[0:len(segment)-1], segment[1:])
			segment = segment[:len(segment)-1]
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return currentElement.ulid
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return currentElement.actualElement
		},
	})
}
