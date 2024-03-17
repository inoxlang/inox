package setcoll

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func BenchmarkAddElemToUnsharedSet(b *testing.B) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	b.Run("repr", func(b *testing.B) {
		b.Run("small", func(b *testing.B) {
			set := NewSetWithConfig(ctx, nil, SetConfig{
				Uniqueness: common.UniquenessConstraint{
					Type: common.UniqueRepr,
				},
			})

			records := make([]*core.Record, b.N)
			for i := range records {
				records[i] = core.NewRecordFromMap(core.ValMap{
					"a": core.String(strings.Repeat("x", 10)),
					"i": core.Int(i),
				})
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				set.Add(ctx, records[i])
			}
		})

		b.Run("med size", func(b *testing.B) {
			set := NewSetWithConfig(ctx, nil, SetConfig{
				Uniqueness: common.UniquenessConstraint{
					Type: common.UniqueRepr,
				},
			})

			records := make([]*core.Record, b.N)
			for i := range records {
				records[i] = core.NewRecordFromMap(core.ValMap{
					"a": core.String(strings.Repeat("x", 1000)),
					"i": core.Int(i),
				})
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				set.Add(ctx, records[i])
			}
		})

		b.Run("large", func(b *testing.B) {
			set := NewSetWithConfig(ctx, nil, SetConfig{
				Uniqueness: common.UniquenessConstraint{
					Type: common.UniqueRepr,
				},
			})

			records := make([]*core.Record, b.N)
			for i := range records {
				records[i] = core.NewRecordFromMap(core.ValMap{
					"a": core.String(strings.Repeat("x", 10_000)),
					"i": core.Int(i),
				})
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				set.Add(ctx, records[i])
			}
		})
	})
}

func BenchmarkAddElemToSharedSet(b *testing.B) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	set := NewSetWithConfig(ctx, nil, SetConfig{
		Uniqueness: common.UniquenessConstraint{
			Type: common.UniqueRepr,
		},
	})

	set.Share(ctx.MustGetClosestState())

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		set.Add(ctx, core.Int(i))
	}
}
