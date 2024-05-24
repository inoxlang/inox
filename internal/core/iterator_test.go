package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/stretchr/testify/assert"
)

func TestIntegerQuantityRangeIteration(t *testing.T) {

	t.Run("known start", func(t *testing.T) {
		t.Run("end == start", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			byteCountRange := QuantityRange{
				unknownStart: false,
				inclusiveEnd: true,
				start:        ByteCount(0),
				end:          ByteCount(0),
			}
			it := byteCountRange.Iterator(ctx, IteratorConfiguration{})

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(0), it.Key(ctx))
			assert.Equal(t, ByteCount(0), it.Value(ctx))

			assert.False(t, it.HasNext(ctx))
			assert.False(t, it.Next(ctx))
		})

		t.Run("end == start + 1", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			byteCountRange := QuantityRange{
				unknownStart: false,
				inclusiveEnd: true,
				start:        ByteCount(0),
				end:          ByteCount(1),
			}
			it := byteCountRange.Iterator(ctx, IteratorConfiguration{})

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(0), it.Key(ctx))
			assert.Equal(t, ByteCount(0), it.Value(ctx))

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(1), it.Key(ctx))
			assert.Equal(t, ByteCount(1), it.Value(ctx))

			assert.False(t, it.HasNext(ctx))
			assert.False(t, it.Next(ctx))
		})

		t.Run("end == start + 2", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			byteCountRange := QuantityRange{
				unknownStart: false,
				inclusiveEnd: true,
				start:        ByteCount(0),
				end:          ByteCount(2),
			}
			it := byteCountRange.Iterator(ctx, IteratorConfiguration{})

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(0), it.Key(ctx))
			assert.Equal(t, ByteCount(0), it.Value(ctx))

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(1), it.Key(ctx))
			assert.Equal(t, ByteCount(1), it.Value(ctx))

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(2), it.Key(ctx))
			assert.Equal(t, ByteCount(2), it.Value(ctx))

			assert.False(t, it.HasNext(ctx))
			assert.False(t, it.Next(ctx))
		})
	})

	t.Run("unknown start", func(t *testing.T) {
		t.Run("end == general start", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			byteCountRange := QuantityRange{
				unknownStart: true,
				inclusiveEnd: true,
				end:          ByteCount(0),
			}
			it := byteCountRange.Iterator(ctx, IteratorConfiguration{})

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(0), it.Key(ctx))
			assert.Equal(t, ByteCount(0), it.Value(ctx))

			assert.False(t, it.HasNext(ctx))
			assert.False(t, it.Next(ctx))
		})

		t.Run("end == general start + 1", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			byteCountRange := QuantityRange{
				unknownStart: true,
				inclusiveEnd: true,
				end:          ByteCount(1),
			}
			it := byteCountRange.Iterator(ctx, IteratorConfiguration{})

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(0), it.Key(ctx))
			assert.Equal(t, ByteCount(0), it.Value(ctx))

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(1), it.Key(ctx))
			assert.Equal(t, ByteCount(1), it.Value(ctx))

			assert.False(t, it.HasNext(ctx))
			assert.False(t, it.Next(ctx))
		})
	})
}

func TestByteSliceIteration(t *testing.T) {

	t.Run("single byte", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		slice := NewByteSlice([]byte{'a'}, true, "")
		it := slice.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Byte('a'), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two bytes", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		slice := NewByteSlice([]byte{'a', 'b'}, true, "")
		it := slice.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Byte('a'), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Byte('b'), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("three elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		slice := NewByteSlice([]byte{'a', 'b', 'c'}, true, "")
		it := slice.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Byte('a'), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Byte('b'), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, Byte('c'), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

}

func TestObjectIteration(t *testing.T) {

	t.Run("no properties", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewObjectFromMap(nil, ctx).
			Iterator(ctx, IteratorConfiguration{})

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single entry", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewObjectFromMap(ValMap{
			"a": Int(2),
		}, ctx).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, String("a"), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two entries", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewObjectFromMap(ValMap{
			"a": Int(2),
			"b": Int(3),
		}, ctx).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, String("a"), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, String("b"), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	//TODO: add tests
}

func TestRecordIteration(t *testing.T) {

	t.Run("no properties", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewRecordFromMap(nil).
			Iterator(ctx, IteratorConfiguration{})

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single entry", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewRecordFromMap(ValMap{
			"a": Int(2),
		}).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, String("a"), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two entries", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewRecordFromMap(ValMap{
			"a": Int(2),
			"b": Int(3),
		}).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, String("a"), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, String("b"), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	//TODO: add tests
}

func TestExactValuePatternIteration(t *testing.T) {

	t.Run("iterator", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)
		it := ExactValuePattern{value: Int(2)}.Iterator(ctx, IteratorConfiguration{})
		assert.True(t, it.HasNext(ctx))

		//next
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestUnionPatternIteration(t *testing.T) {

	t.Run("single element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := UnionPattern{
			cases: []Pattern{
				&ExactValuePattern{value: Int(2)},
			},
		}.Iterator(ctx, IteratorConfiguration{})
		assert.True(t, it.HasNext(ctx))

		//next
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := UnionPattern{
			cases: []Pattern{
				&ExactValuePattern{value: Int(2)},
				&ExactValuePattern{value: Int(3)},
			},
		}.Iterator(ctx, IteratorConfiguration{})
		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestIntersectionPatternIteration(t *testing.T) {

	t.Run("single element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (&IntersectionPattern{
			cases: []Pattern{
				&ExactValuePattern{value: Int(2)},
			},
		}).Iterator(ctx, IteratorConfiguration{})
		assert.True(t, it.HasNext(ctx))

		//next
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (&IntersectionPattern{
			cases: []Pattern{
				NewIncludedEndIntRangePattern(1, 3, -1),
				NewIncludedEndIntRangePattern(2, 4, -1),
			},
		}).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestKeyFilteredIterator(t *testing.T) {

	t.Run("single element base iterator: key does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter: NewSingleElementIntRangePattern(1),
		}
		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single element base iterator, key ok", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter: NewSingleElementIntRangePattern(0),
		}
		assert.True(t, it.HasNext(ctx))

		//next
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, both keys match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter: NewIncludedEndIntRangePattern(0, 1, -1),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, first key does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter: NewSingleElementIntRangePattern(1),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, second key does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter: NewSingleElementIntRangePattern(0),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestValueFilteredIterator(t *testing.T) {

	t.Run("single element base iterator: value does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &ValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			valueFilter: NewSingleElementIntRangePattern(1),
		}
		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single element base iterator, value ok", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &ValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			valueFilter: NewSingleElementIntRangePattern(2),
		}
		assert.True(t, it.HasNext(ctx))

		//next
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, both values match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &ValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			valueFilter: NewIncludedEndIntRangePattern(2, 3, -1),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, first value does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &ValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			valueFilter: NewSingleElementIntRangePattern(3),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, second value does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &ValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			valueFilter: NewSingleElementIntRangePattern(2),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestKeyValueFilteredIterator(t *testing.T) {

	t.Run("single element base iterator: value does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewSingleElementIntRangePattern(0),
			valueFilter: NewSingleElementIntRangePattern(1),
		}
		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single element base iterator: key does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewSingleElementIntRangePattern(1),
			valueFilter: NewSingleElementIntRangePattern(2),
		}
		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single element base iterator: both key & value do not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewSingleElementIntRangePattern(1),
			valueFilter: NewSingleElementIntRangePattern(1),
		}
		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single element base iterator, key & value ok", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewSingleElementIntRangePattern(0),
			valueFilter: NewSingleElementIntRangePattern(2),
		}
		assert.True(t, it.HasNext(ctx))

		//next
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, both keys & values match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewIncludedEndIntRangePattern(0, 1, -1),
			valueFilter: NewIncludedEndIntRangePattern(2, 3, -1),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, first value does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewIncludedEndIntRangePattern(0, 1, -1),
			valueFilter: NewSingleElementIntRangePattern(3),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, first key does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewSingleElementIntRangePattern(1),
			valueFilter: NewIncludedEndIntRangePattern(2, 3, -1),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, Int(3), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, second value does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewIncludedEndIntRangePattern(0, 1, -1),
			valueFilter: NewSingleElementIntRangePattern(2),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, second key does not match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewSingleElementIntRangePattern(0),
			valueFilter: NewIncludedEndIntRangePattern(2, 3, -1),
		}

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, Int(2), it.Value(ctx))

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two-element base iterator, no match", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := &KeyValueFilteredIterator{
			it: UnionPattern{
				cases: []Pattern{
					&ExactValuePattern{value: Int(2)},
					&ExactValuePattern{value: Int(3)},
				},
			}.Iterator(ctx, IteratorConfiguration{}),
			keyFilter:   NewSingleElementIntRangePattern(10),
			valueFilter: NewSingleElementIntRangePattern(10),
		}

		//
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestObjectPatternIteration(t *testing.T) {

	t.Run("no properties", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewExactObjectPattern(nil).Iterator(ctx, IteratorConfiguration{})

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single entry", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewExactObjectPattern([]ObjectPatternEntry{{Name: "a", Pattern: &ExactValuePattern{value: Int(2)}}}).
			Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, objFrom(ValMap{"a": Int(2)}), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two entries", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewExactObjectPattern([]ObjectPatternEntry{
			{
				Name: "a",
				Pattern: &UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
					},
				},
			},
			{
				Name: "b",
				Pattern: &UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
					},
				},
			},
		}).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, objFrom(ValMap{"a": Int(2), "b": Int(2)}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, objFrom(ValMap{"a": Int(2), "b": Int(3)}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, objFrom(ValMap{"a": Int(3), "b": Int(2)}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, objFrom(ValMap{"a": Int(3), "b": Int(3)}), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

//

func TestRecordPatternIteration(t *testing.T) {

	t.Run("no properties", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewExactRecordPattern(nil).Iterator(ctx, IteratorConfiguration{})

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single entry", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewExactRecordPattern([]RecordPatternEntry{{Name: "a", Pattern: &ExactValuePattern{value: Int(2)}}}).
			Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, NewRecordFromMap(ValMap{"a": Int(2)}), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two entries", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := NewExactRecordPattern([]RecordPatternEntry{
			{
				Name: "a",
				Pattern: &UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
					},
				},
			},
			{
				Name: "b",
				Pattern: &UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
					},
				},
			},
		}).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, NewRecordFromMap(ValMap{"a": Int(2), "b": Int(2)}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, NewRecordFromMap(ValMap{"a": Int(2), "b": Int(3)}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, NewRecordFromMap(ValMap{"a": Int(3), "b": Int(2)}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, NewRecordFromMap(ValMap{"a": Int(3), "b": Int(3)}), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestBoolListIteration(t *testing.T) {

	t.Run("no elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (newBoolList()).Iterator(ctx, IteratorConfiguration{})

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (newBoolList(True)).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, True, it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (newBoolList(True, False)).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, True, it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, False, it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("three elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (newBoolList(True, False, True)).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, True, it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, False, it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, True, it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

}

func TestListPatternIteration(t *testing.T) {

	t.Run("no elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := ListPattern{
			elementPatterns: []Pattern{},
		}.Iterator(ctx, IteratorConfiguration{})

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (&ListPattern{
			elementPatterns: []Pattern{
				&UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
					},
				},
			},
		}).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, newList(&ValueList{elements: []Serializable{Int(2)}}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, newList(&ValueList{elements: []Serializable{Int(3)}}), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		it := (&ListPattern{
			elementPatterns: []Pattern{
				&UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
					},
				},
				&UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
					},
				},
			},
		}).Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, newList(&ValueList{elements: []Serializable{Int(2), Int(2)}}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, newList(&ValueList{elements: []Serializable{Int(2), Int(3)}}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, newList(&ValueList{elements: []Serializable{Int(3), Int(2)}}), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, newList(&ValueList{elements: []Serializable{Int(3), Int(3)}}), it.Value(ctx))

		//next
		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

}

func TestSequenceStringPatternIteration(t *testing.T) {

	t.Run("single element", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		seqPattern := &SequenceStringPattern{
			elements: []StringPattern{
				&RuneRangeStringPattern{runes: RuneRange{'a', 'b'}},
			},
		}

		it := seqPattern.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, String("a"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, String("b"), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		seqPattern := &SequenceStringPattern{
			elements: []StringPattern{
				&RuneRangeStringPattern{runes: RuneRange{'a', 'b'}},
				&RuneRangeStringPattern{runes: RuneRange{'0', '1'}},
			},
		}

		it := seqPattern.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, String("a0"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, String("a1"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, String("b0"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, String("b1"), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("three elements", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		seqPattern := &SequenceStringPattern{
			elements: []StringPattern{
				&RuneRangeStringPattern{runes: RuneRange{'a', 'b'}},
				&RuneRangeStringPattern{runes: RuneRange{'0', '1'}},
				&RuneRangeStringPattern{runes: RuneRange{'0', '1'}},
			},
		}

		it := seqPattern.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, String("a00"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, String("a01"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, String("a10"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, String("a11"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(4), it.Key(ctx))
		assert.Equal(t, String("b00"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(5), it.Key(ctx))
		assert.Equal(t, String("b01"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(6), it.Key(ctx))
		assert.Equal(t, String("b10"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(7), it.Key(ctx))
		assert.Equal(t, String("b11"), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

}

func TestRepeatedStringPatternIteration(t *testing.T) {

	t.Run("zero or more times", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		patt := &RepeatedPatternElement{
			element:    &RuneRangeStringPattern{runes: RuneRange{'a', 'b'}},
			quantifier: ast.ZeroOrMoreOccurrences,
		}

		it := patt.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, String(""), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, String("a"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, String("b"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, String("aa"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(4), it.Key(ctx))
		assert.Equal(t, String("ab"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(5), it.Key(ctx))
		assert.Equal(t, String("ba"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(6), it.Key(ctx))
		assert.Equal(t, String("bb"), it.Value(ctx))

		//...
	})

	t.Run("one or more times", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		patt := &RepeatedPatternElement{
			element:    &RuneRangeStringPattern{runes: RuneRange{'a', 'b'}},
			quantifier: ast.AtLeastOneOccurrence,
		}

		it := patt.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, String("a"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, String("b"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, String("aa"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, String("ab"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(4), it.Key(ctx))
		assert.Equal(t, String("ba"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(5), it.Key(ctx))
		assert.Equal(t, String("bb"), it.Value(ctx))

		//...
	})

	t.Run("exactly 1 times", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		patt := &RepeatedPatternElement{
			element:    &RuneRangeStringPattern{runes: RuneRange{'a', 'b'}},
			quantifier: ast.ExactlyOneOccurrence,
		}

		it := patt.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, String("a"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, String("b"), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("exactly 2 times", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		patt := &RepeatedPatternElement{
			element:    &RuneRangeStringPattern{runes: RuneRange{'a', 'b'}},
			quantifier: ast.ExactOccurrenceCount,
			exactCount: 2,
		}

		it := patt.Iterator(ctx, IteratorConfiguration{})

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(0), it.Key(ctx))
		assert.Equal(t, String("aa"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(1), it.Key(ctx))
		assert.Equal(t, String("ab"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(2), it.Key(ctx))
		assert.Equal(t, String("ba"), it.Value(ctx))

		//next
		assert.True(t, it.HasNext(ctx))
		assert.True(t, it.Next(ctx))
		assert.Equal(t, Int(3), it.Key(ctx))
		assert.Equal(t, String("bb"), it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})
}

func TestDifferencePatternIteration(t *testing.T) {

	t.Run("iterator", func(t *testing.T) {
		t.Run("no elements", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			it := (&DifferencePattern{
				base:    &ExactValuePattern{value: Int(1)},
				removed: &ExactValuePattern{value: Int(1)},
			}).Iterator(ctx, IteratorConfiguration{})

			assert.False(t, it.HasNext(ctx))
			assert.False(t, it.Next(ctx))
		})

		t.Run("elements", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			it := (&DifferencePattern{
				base: &UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(1)},
						&ExactValuePattern{value: Int(2)},
						&ExactValuePattern{value: Int(3)},
						&ExactValuePattern{value: Int(4)},
					},
				},
				removed: &UnionPattern{
					cases: []Pattern{
						&ExactValuePattern{value: Int(1)},
						&ExactValuePattern{value: Int(3)},
					},
				},
			}).Iterator(ctx, IteratorConfiguration{})

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(0), it.Key(ctx))
			assert.Equal(t, Int(2), it.Value(ctx))

			//next
			assert.True(t, it.HasNext(ctx))
			assert.True(t, it.Next(ctx))
			assert.Equal(t, Int(1), it.Key(ctx))
			assert.Equal(t, Int(4), it.Value(ctx))

			//next
			assert.False(t, it.HasNext(ctx))
			assert.False(t, it.Next(ctx))
		})
	})
}
