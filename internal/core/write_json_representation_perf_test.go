package core

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/jsoniter"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

func BenchmarkWriteIntJSONRepresentation(b *testing.B) {

	b.Run("small", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
		val := Int(12)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})

	b.Run("medium", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

		b.ResetTimer()

		val := Int(1_234_456)

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})

	b.Run("large", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

		b.ResetTimer()

		val := Int(1_234_456_789)

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})
}

func BenchmarkWriteFloatJSONRepresentation(b *testing.B) {

	b.Run("small", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

		b.ResetTimer()

		val := Float(1.2)

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})

	b.Run("medium repr length", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

		b.ResetTimer()

		val := Float(1.234_456)

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})

	b.Run("long repr length", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

		b.ResetTimer()

		val := Float(1.234_456_789)

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})
}

func BenchmarkWriteObjectJSONRepresentation(b *testing.B) {

	//In the following benchmarks we don't marshal the same value several times
	//in order to ignore any future optimization.

	b.Run("small", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

		values := utils.Repeat(b.N, func(index int) *Object {
			return NewObjectFromMapNoInit(ValMap{
				"a": String(strings.Repeat("x", 10)),
			})
		})

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})

	b.Run("medium size", func(b *testing.B) {
		b.Run("single property", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Object {
				return NewObjectFromMapNoInit(ValMap{
					"a": String(strings.Repeat("x", 100)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
		b.Run("two properties", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Object {
				return NewObjectFromMapNoInit(ValMap{
					"a": String(strings.Repeat("x", 50)),
					"b": String(strings.Repeat("x", 50)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
	})

	b.Run("long repr length", func(b *testing.B) {
		b.Run("single property", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Object {
				return NewObjectFromMapNoInit(ValMap{
					"a": String(strings.Repeat("x", 1000)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
		b.Run("two properties", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Object {
				return NewObjectFromMapNoInit(ValMap{
					"a": String(strings.Repeat("x", 500)),
					"b": String(strings.Repeat("x", 500)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
		b.Run("many properties", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Object {
				return NewObjectFromMapNoInit(ValMap{
					"a": String(strings.Repeat("x", 100)),
					"b": String(strings.Repeat("x", 100)),
					"c": String(strings.Repeat("x", 100)),
					"d": String(strings.Repeat("x", 100)),
					"e": String(strings.Repeat("x", 100)),
					"f": String(strings.Repeat("x", 100)),
					"g": String(strings.Repeat("x", 100)),
					"h": String(strings.Repeat("x", 100)),
					"i": String(strings.Repeat("x", 100)),
					"j": String(strings.Repeat("x", 100)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
	})
}

func BenchmarkWriteRecordJSONRepresentation(b *testing.B) {

	//In the following benchmarks we don't marshal the same value several times
	//in order to ignore any future optimization.

	b.Run("small", func(b *testing.B) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

		values := utils.Repeat(b.N, func(index int) *Record {
			return NewRecordFromMap(ValMap{
				"a": String(strings.Repeat("x", 10)),
			})
		})

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
		}
	})

	b.Run("medium size", func(b *testing.B) {
		b.Run("single property", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Record {
				return NewRecordFromMap(ValMap{
					"a": String(strings.Repeat("x", 100)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
		b.Run("two properties", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Record {
				return NewRecordFromMap(ValMap{
					"a": String(strings.Repeat("x", 50)),
					"b": String(strings.Repeat("x", 50)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
	})

	b.Run("long repr length", func(b *testing.B) {
		b.Run("single property", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Record {
				return NewRecordFromMap(ValMap{
					"a": String(strings.Repeat("x", 1000)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
		b.Run("two properties", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Record {
				return NewRecordFromMap(ValMap{
					"a": String(strings.Repeat("x", 500)),
					"b": String(strings.Repeat("x", 500)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
		b.Run("many properties", func(b *testing.B) {
			ctx := NewContextWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

			values := utils.Repeat(b.N, func(index int) *Record {
				return NewRecordFromMap(ValMap{
					"a": String(strings.Repeat("x", 100)),
					"b": String(strings.Repeat("x", 100)),
					"c": String(strings.Repeat("x", 100)),
					"d": String(strings.Repeat("x", 100)),
					"e": String(strings.Repeat("x", 100)),
					"f": String(strings.Repeat("x", 100)),
					"g": String(strings.Repeat("x", 100)),
					"h": String(strings.Repeat("x", 100)),
					"i": String(strings.Repeat("x", 100)),
					"j": String(strings.Repeat("x", 100)),
				})
			})

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				utils.PanicIfErr(values[i].WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{}, 0))
			}
		})
	})
}
