package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiValue(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		intOrNil := NewMultivalue(&Int{}, Nil)

		assert.True(t, intOrNil.Test(intOrNil))
		assert.True(t, intOrNil.Test(&Int{}))
		assert.True(t, intOrNil.Test(Nil))
		assert.False(t, intOrNil.Test(&String{}))

		intOrStringOrNil := NewMultivalue(&Int{}, &String{}, Nil)

		assert.True(t, intOrStringOrNil.Test(intOrNil))
		assert.False(t, intOrNil.Test(intOrStringOrNil))

		intListOrStringList := NewMultivalue(NewListOf(&Int{}), NewListOf(&String{}))
		anyList := NewListOf(ANY_SERIALIZABLE)

		assert.True(t, intListOrStringList.Test(intListOrStringList))
		assert.False(t, intListOrStringList.Test(anyList))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		intOrNil := NewMultivalue(&Int{}, Nil)
		assert.False(t, intOrNil.IsWidenable())

		widened, ok := intOrNil.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)

		identAOrIdentB := NewMultivalue(&Identifier{name: "a"}, &Identifier{name: "b"})
		assert.True(t, identAOrIdentB.IsWidenable())

		widened, ok = identAOrIdentB.Widen()
		assert.True(t, ok)
		assert.Equal(t, &Identifier{}, widened)
	})

	t.Run("as Indexable", func(t *testing.T) {
		assert.Implements(t, (*Indexable)(nil), NewMultivalue(
			Indexable(NewListOf(ANY_SERIALIZABLE)),
			Indexable(NewListOf(&Int{})),
		).as(INDEXABLE_INTERFACE_TYPE))

		_, ok := NewMultivalue(
			Indexable(NewListOf(ANY_SERIALIZABLE)),
			&Int{},
		).as(INDEXABLE_INTERFACE_TYPE).(Indexable)
		assert.False(t, ok)

		_, ok = NewMultivalue(
			&Int{},
			Indexable(NewListOf(ANY_SERIALIZABLE)),
		).as(INDEXABLE_INTERFACE_TYPE).(Indexable)
		assert.False(t, ok)
	})

	t.Run("Iterable", func(t *testing.T) {
		//TODO: test with iterable but not indexable values

		assert.Implements(t, (*Iterable)(nil), NewMultivalue(
			Iterable(NewListOf(ANY_SERIALIZABLE)),
			Iterable(NewListOf(&Int{})),
		).as(ITERABLE_INTERFACE_TYPE))

		_, ok := NewMultivalue(
			Iterable(NewListOf(ANY_SERIALIZABLE)),
			&Int{},
		).as(ITERABLE_INTERFACE_TYPE).(Iterable)
		assert.False(t, ok)

		_, ok = NewMultivalue(
			&Int{},
			Iterable(NewListOf(ANY_SERIALIZABLE)),
		).as(ITERABLE_INTERFACE_TYPE).(Iterable)
		assert.False(t, ok)
	})

	t.Run("Iprops", func(t *testing.T) {
		assert.Implements(t, (*IProps)(nil), NewMultivalue(
			NewObject(map[string]Serializable{"a": &Int{}}, nil, nil),
			NewObject(map[string]Serializable{"b": &Int{}}, nil, nil),
		).as(IPROPS_INTERFACE_TYPE))

		_, ok := NewMultivalue(
			NewObject(map[string]Serializable{"a": &Int{}}, nil, nil),
			ANY_BOOL,
		).as(IPROPS_INTERFACE_TYPE).(IProps)
		assert.False(t, ok)

		_, ok = NewMultivalue(
			ANY_BOOL,
			NewObject(map[string]Serializable{"a": &Int{}}, nil, nil),
		).as(IPROPS_INTERFACE_TYPE).(IProps)
		assert.False(t, ok)
	})
}
