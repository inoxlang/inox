package rankingcoll

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
)

var (
	ErrRankingEntryListShouldHaveEvenLength              = errors.New(`flat rank entry list should have an even length: [<value>, <float>,  <value>, <float>]`)
	ErrRankingEntryListShouldHaveFloatScoresAtOddIndexes = errors.New(`flat rank entry list should have scores at odd indexes : [<value>, <float>,  <value>, <float>]`)
	ErrRankingCanOnlyContainValuesWithFastId             = errors.New("a Ranking can only contain values having a fast id")
	ErrRankingCanOnlyRankValuesWithAPositiveScore        = errors.New("a Ranking can only rank values with a positive score")
	ErrRankingCannotContainDuplicates                    = errors.New("a Ranking cannot contain duplicates")
)

func NewRanking(ctx *core.Context, flatEntries *core.List) *Ranking {

	ranking := &Ranking{
		map_: map[core.TransientID]core.Serializable{},
	}

	if flatEntries.Len()%2 != 0 {
		panic(ErrRankingEntryListShouldHaveEvenLength)
	}

	halfEntryCount := flatEntries.Len()
	for i := 0; i < halfEntryCount; i += 2 {
		value := flatEntries.At(ctx, i)
		valueScore, ok := flatEntries.At(ctx, i+1).(core.Float)
		if !ok {
			panic(ErrRankingEntryListShouldHaveFloatScoresAtOddIndexes)
		}

		ranking.Add(ctx, value.(core.Serializable), valueScore)
	}

	return ranking
}

type Ranking struct {
	map_      map[core.TransientID]core.Serializable
	rankItems []RankItem
}

type RankItem struct {
	valueIds []core.TransientID
	score    float64
}

func (r *Ranking) Add(ctx *core.Context, value core.Serializable, score core.Float) {
	id, ok := core.TransientIdOf(value)
	if !ok {
		panic(ErrRankingCanOnlyContainValuesWithFastId)
	}

	if _, ok := r.map_[id]; ok {
		panic(ErrRankingCannotContainDuplicates)
	}

	r.map_[id] = value

	if score < 0 {
		panic(ErrRankingCanOnlyRankValuesWithAPositiveScore)
	}

	var prevRank int = -1

	for rank, item := range r.rankItems {
		if float64(score) == item.score {
			item.valueIds = append(item.valueIds, id)
			r.rankItems[rank] = item
			return
		} else if float64(score) < item.score {
			prevRank = rank
		} else {
			r.rankItems = append([]RankItem{{
				score:    float64(score),
				valueIds: []core.TransientID{id},
			}}, r.rankItems...)
			return
		}
	}

	if prevRank < 0 {
		r.rankItems = append(r.rankItems, RankItem{
			score:    float64(score),
			valueIds: []core.TransientID{id},
		})
	} else {
		r.rankItems = append(r.rankItems, RankItem{})

		copy(r.rankItems[prevRank+2:], r.rankItems[prevRank+1:])

		r.rankItems[prevRank+1] = RankItem{
			score:    float64(score),
			valueIds: []core.TransientID{id},
		}
	}
}

func (r *Ranking) Remove(ctx *core.Context, removedVal core.Serializable) {
	panic(errors.New("removal not implemented yet"))
}
