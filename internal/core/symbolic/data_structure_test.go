package symbolic

import (
	"context"
	"fmt"
	"testing"

	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicObject(t *testing.T) {
	cases := []struct {
		object1 *Object
		object2 *Object
		ok      bool
	}{
		//an any object should match an any object
		{&Object{entries: nil}, &Object{entries: nil}, true},

		//an empty object should not match an any object
		{&Object{entries: map[string]Serializable{}}, &Object{entries: nil}, false},

		//an any object should match an empty object
		{&Object{entries: nil}, &Object{entries: map[string]Serializable{}}, true},

		//an empty object should match an empty object
		{&Object{entries: map[string]Serializable{}}, &Object{entries: map[string]Serializable{}}, true},

		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries: map[string]Serializable{},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{},
				exact:   true,
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			false,
		},
		{
			&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			&Object{
				entries: map[string]Serializable{},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			},
			&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			false,
		},
	}

	for _, testCase := range cases {
		t.Run(t.Name()+"_"+Stringify(testCase.object1)+"_"+Stringify(testCase.object2), func(t *testing.T) {
			assert.Equal(t, testCase.ok, testCase.object1.Test(testCase.object2))
		})
	}

	t.Run("SetProp", func(t *testing.T) {
		t.Run("should return an error if a new property is set in an  exact object", func(t *testing.T) {
			obj := NewExactObject(map[string]Serializable{}, nil, nil)
			updated, err := obj.SetProp("new-prop", NewInt(1))
			if !assert.ErrorContains(t, err, CANNOT_ADD_NEW_PROPERTY_TO_AN_EXACT_OBJECT) {
				return
			}
			assert.Nil(t, updated)
		})
	})

}

func TestSymbolicRecord(t *testing.T) {
	cases := []struct {
		record1 *Record
		record2 *Record
		ok      bool
	}{
		//an any record should match an any record
		{&Record{entries: nil}, &Record{entries: nil}, true},

		//an empty record should not match an any record
		{&Record{entries: map[string]Serializable{}}, &Record{entries: nil}, false},

		//an any record should match an empty record
		{&Record{entries: nil}, &Record{entries: map[string]Serializable{}}, true},

		//an empty record should match an empty record
		{&Record{entries: map[string]Serializable{}}, &Record{entries: map[string]Serializable{}}, true},

		{
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Record{
				entries: map[string]Serializable{},
			},
			false,
		},
		{
			&Record{
				entries: map[string]Serializable{},
			},
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{},
				exact:   true,
			},
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			false,
		},
		{
			&Record{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			&Record{
				entries: map[string]Serializable{},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			},
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Record{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			},
			&Record{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Record{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			false,
		},
	}

	for _, testCase := range cases {
		t.Run(t.Name()+"_"+Stringify(testCase.record1)+"_"+Stringify(testCase.record2), func(t *testing.T) {
			assert.Equal(t, testCase.ok, testCase.record1.Test(testCase.record2))
		})
	}

}

func TestSymbolicList(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			list1 *List
			list2 *List
			ok    bool
		}{
			{
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				&List{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				false,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: []Serializable{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				&List{elements: nil, generalElement: ANY_INT},
				false,
			},
			{
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				&List{elements: []Serializable{ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&List{elements: []Serializable{ANY_INT, ANY_INT}, generalElement: nil},
				&List{elements: []Serializable{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.list1, "_", testCase.list2), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.list1.Test(testCase.list2))
			})
		}
	})

	t.Run("Append()", func(t *testing.T) {
		t.Run("adding no elements to empty list", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList()
			list.Append(ctx)

			_, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
		})

		t.Run("adding element to empty list", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList()
			list.Append(ctx, NewInt(1))

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewList(NewInt(1)), updatedSelf)
		})

		t.Run("adding no element to list with single element", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList(NewInt(1))
			list.Append(ctx)

			_, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
		})

		t.Run("adding same type element to list with single element", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList(NewInt(1))
			list.Append(ctx, NewInt(2))

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewListOf(ANY_INT), updatedSelf)
		})
	})
}

func TestSymbolicTuple(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			tuple1 *Tuple
			tuple2 *Tuple
			ok     bool
		}{
			{
				&Tuple{elements: nil, generalElement: ANY_SERIALIZABLE},
				&Tuple{elements: nil, generalElement: ANY_SERIALIZABLE},
				true,
			},
			{
				&Tuple{elements: nil, generalElement: ANY_SERIALIZABLE},
				&Tuple{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&Tuple{elements: nil, generalElement: ANY_INT},
				&Tuple{elements: nil, generalElement: ANY_SERIALIZABLE},
				false,
			},
			{
				&Tuple{elements: nil, generalElement: ANY_INT},
				&Tuple{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&Tuple{elements: nil, generalElement: ANY_INT},
				&Tuple{elements: []Serializable{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&Tuple{elements: nil, generalElement: ANY_INT},
				&Tuple{elements: []Serializable{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&Tuple{elements: []Serializable{ANY_INT}, generalElement: nil},
				&Tuple{elements: nil, generalElement: ANY_INT},
				false,
			},
			{
				&Tuple{elements: []Serializable{ANY_INT}, generalElement: nil},
				&Tuple{elements: []Serializable{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&Tuple{elements: []Serializable{ANY_INT}, generalElement: nil},
				&Tuple{elements: []Serializable{ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&Tuple{elements: []Serializable{ANY_INT, ANY_INT}, generalElement: nil},
				&Tuple{elements: []Serializable{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.tuple1, "_", testCase.tuple2), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.tuple1.Test(testCase.tuple2))
			})
		}
	})

}

func TestSymbolicKeyList(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			list1 *KeyList
			list2 *KeyList
			ok    bool
		}{
			{
				&KeyList{Keys: nil},
				&KeyList{Keys: nil},
				true,
			},
			{
				&KeyList{Keys: nil},
				&KeyList{Keys: []string{"name"}},
				true,
			},
			{
				&KeyList{Keys: []string{"name"}},
				&KeyList{Keys: nil},
				false,
			},
			{
				&KeyList{Keys: []string{"name"}},
				&KeyList{Keys: []string{"name"}},
				true,
			},
			{
				&KeyList{Keys: []string{"name"}},
				&KeyList{Keys: []string{"name", "age"}},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.list1, "_", testCase.list2), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.list1.Test(testCase.list2))
			})
		}
	})

}

func TestSymbolicDictionary(t *testing.T) {
	cases := []struct {
		dict1            *Dictionary
		dict2            *Dictionary
		oneTestTwoResult bool
	}{
		{
			&Dictionary{entries: nil},
			&Dictionary{entries: nil},
			true,
		},
		{
			&Dictionary{entries: map[string]Serializable{}},
			&Dictionary{entries: nil},
			false,
		},
		{
			&Dictionary{entries: nil},
			&Dictionary{entries: map[string]Serializable{}},
			true,
		},
		{
			&Dictionary{entries: map[string]Serializable{}, keys: map[string]Serializable{}},
			&Dictionary{entries: map[string]Serializable{}, keys: map[string]Serializable{}},
			true,
		},
		{
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_INT},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			&Dictionary{
				entries: map[string]Serializable{},
			},
			false,
		},
		{
			&Dictionary{
				entries: map[string]Serializable{},
			},
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_INT},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			false,
		},
		{
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_INT},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_INT},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			true,
		},
		{
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_SERIALIZABLE},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_INT},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			true,
		},
		{
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_INT},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			&Dictionary{
				entries: map[string]Serializable{"./a": ANY_SERIALIZABLE},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			false,
		},
	}

	for _, testCase := range cases {
		t.Run(t.Name()+"_"+fmt.Sprint(testCase.dict1, "_", testCase.dict2), func(t *testing.T) {
			assert.Equal(t, testCase.oneTestTwoResult, testCase.dict1.Test(testCase.dict2))
		})
	}

}

type testConcreteContext struct {
	context.Context
}

func (ctx testConcreteContext) HasPermissionUntyped(perm any) bool {
	return false
}
func (ctx testConcreteContext) HasAPermissionWithKindAndType(kind permkind.PermissionKind, typename permkind.InternalPermissionTypename) bool {
	return false
}
