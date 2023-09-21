package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiValue(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		intOrNil := NewMultivalue(ANY_INT, Nil)

		assert.True(t, intOrNil.Test(intOrNil))
		assert.True(t, intOrNil.Test(ANY_INT))
		assert.True(t, intOrNil.Test(Nil))
		assert.False(t, intOrNil.Test(&String{}))

		intOrStringOrNil := NewMultivalue(ANY_INT, &String{}, Nil)

		assert.True(t, intOrStringOrNil.Test(intOrNil))
		assert.False(t, intOrNil.Test(intOrStringOrNil))

		intListOrStringList := NewMultivalue(NewListOf(ANY_INT), NewListOf(&String{}))
		anyList := NewListOf(ANY_SERIALIZABLE)

		assert.True(t, intListOrStringList.Test(intListOrStringList))
		assert.False(t, intListOrStringList.Test(anyList))
	})

	t.Run("as Indexable", func(t *testing.T) {

		t.Run("if all values are indexable the multivalue is indexable", func(t *testing.T) {
			as := NewMultivalue(
				Indexable(NewListOf(ANY_SERIALIZABLE)),
				Indexable(NewListOf(ANY_INT)),
			).as(INDEXABLE_INTERFACE_TYPE)

			assert.Implements(t, (*Indexable)(nil), as)
			assert.Same(t, as, as.(asInterface).as(INDEXABLE_INTERFACE_TYPE))
		})

		t.Run("if the first value is not indexable the multivalue is not indexable", func(t *testing.T) {
			mv := NewMultivalue(
				Indexable(NewListOf(ANY_SERIALIZABLE)),
				ANY_INT,
			)
			_, ok := mv.as(INDEXABLE_INTERFACE_TYPE).(Indexable)
			assert.False(t, ok)
		})

		t.Run("if the second value is not indexable the multivalue is not indexable", func(t *testing.T) {
			mv := NewMultivalue(
				ANY_INT,
				Indexable(NewListOf(ANY_SERIALIZABLE)),
			)
			_, ok := mv.as(INDEXABLE_INTERFACE_TYPE).(Indexable)
			assert.False(t, ok)
		})
	})

	t.Run("as Iterable", func(t *testing.T) {
		//TODO: test with iterable but not indexable values

		t.Run("if all values are iterable the multivalue is iterable", func(t *testing.T) {
			as := NewMultivalue(
				Iterable(NewListOf(ANY_SERIALIZABLE)),
				Iterable(NewListOf(ANY_INT)),
			).as(ITERABLE_INTERFACE_TYPE)

			assert.Implements(t, (*Iterable)(nil), as)
			assert.Same(t, as, as.(asInterface).as(ITERABLE_INTERFACE_TYPE))
		})

		t.Run("if the first value is not iterable the multivalue is not iterable", func(t *testing.T) {
			mv := NewMultivalue(
				Iterable(NewListOf(ANY_SERIALIZABLE)),
				ANY_INT,
			)
			_, ok := mv.as(ITERABLE_INTERFACE_TYPE).(Iterable)
			assert.False(t, ok)
		})

		t.Run("if the second value is not iterable the multivalue is not iterable", func(t *testing.T) {
			mv := NewMultivalue(
				ANY_INT,
				Iterable(NewListOf(ANY_SERIALIZABLE)),
			)
			_, ok := mv.as(ITERABLE_INTERFACE_TYPE).(Iterable)
			assert.False(t, ok)
		})
	})

	t.Run("as Serializable", func(t *testing.T) {

		t.Run("if all values are serializable the multivalue is serializable", func(t *testing.T) {
			as := NewMultivalue(ANY_INT, ANY_BOOL).as(SERIALIZABLE_INTERFACE_TYPE)

			assert.Implements(t, (*Serializable)(nil), as)
			assert.Same(t, as, as.(asInterface).as(SERIALIZABLE_INTERFACE_TYPE))
		})

		t.Run("if the first value is not serializable the multivalue is not serializable", func(t *testing.T) {
			mv := NewMultivalue(ANY_LTHREAD, ANY_INT)
			_, ok := mv.as(SERIALIZABLE_INTERFACE_TYPE).(Iterable)
			assert.False(t, ok)
		})

		t.Run("if the second value is not serializable the multivalue is not serializable", func(t *testing.T) {
			mv := NewMultivalue(ANY_INT, ANY_LTHREAD)
			_, ok := mv.as(SERIALIZABLE_INTERFACE_TYPE).(Iterable)
			assert.False(t, ok)
		})
	})

	t.Run("as Watchable", func(t *testing.T) {

		t.Run("if all values are watchable the multivalue is watchable", func(t *testing.T) {
			as := NewMultivalue(EMPTY_LIST, ANY_OBJ).as(WATCHABLE_INTERFACE_TYPE)

			assert.Implements(t, (*Watchable)(nil), as)
			assert.Same(t, as, as.(asInterface).as(WATCHABLE_INTERFACE_TYPE))
		})

		t.Run("if the first value is not watchable the multivalue is not watchable", func(t *testing.T) {
			mv := NewMultivalue(ANY_INT, ANY_OBJ)
			_, ok := mv.as(WATCHABLE_INTERFACE_TYPE).(Iterable)
			assert.False(t, ok)
		})

		t.Run("if the second value is not watchable the multivalue is not watchable", func(t *testing.T) {
			mv := NewMultivalue(EMPTY_LIST, ANY_INT)
			_, ok := mv.as(WATCHABLE_INTERFACE_TYPE).(Iterable)
			assert.False(t, ok)
		})
	})

	t.Run("Iprops", func(t *testing.T) {

		t.Run("if all values are iprops the multivalue is an iprops", func(t *testing.T) {
			as := NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
				NewInexactObject(map[string]Serializable{"b": ANY_INT}, nil, nil),
			).as(IPROPS_INTERFACE_TYPE)

			assert.Implements(t, (*IProps)(nil), as)
			assert.Same(t, as, as.(asInterface).as(IPROPS_INTERFACE_TYPE))
		})

		t.Run("if the first value is not an iprops the multivalue is not an iprops", func(t *testing.T) {
			mv := NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
				ANY_BOOL,
			)
			_, ok := mv.as(IPROPS_INTERFACE_TYPE).(IProps)
			assert.False(t, ok)
		})

		t.Run("if the second value is not an iprops the multivalue is not an iprops", func(t *testing.T) {
			mv := NewMultivalue(
				ANY_BOOL,
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil),
			)
			_, ok := mv.as(IPROPS_INTERFACE_TYPE).(IProps)
			assert.False(t, ok)
		})

	})
}
