package rankingcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (r *Ranking) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	rank := -1

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			return rank < len(r.rankItems)-1
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			rank++
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(rank)
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return &Rank{
				ranking: r,
				rank:    rank,
			}
		},
	})
}
