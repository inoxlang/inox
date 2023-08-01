package symbolic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicObject(t *testing.T) {
	cases := []struct {
		object1 *Object
		object2 *Object
		ok      bool
	}{
		{&Object{entries: nil}, &Object{entries: nil}, true},
		{&Object{entries: map[string]Serializable{}}, &Object{entries: nil}, false},
		{&Object{entries: nil}, &Object{entries: map[string]Serializable{}}, true},

		{&Object{entries: map[string]Serializable{}}, &Object{entries: map[string]Serializable{}}, true},
		{
			&Object{
				entries: map[string]Serializable{"a": &Int{}},
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
				entries: map[string]Serializable{"a": &Int{}},
			},
			false,
		},
		{
			&Object{
				entries:         map[string]Serializable{"a": &Int{}},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			&Object{
				entries: map[string]Serializable{},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": &Int{}},
			},
			&Object{
				entries: map[string]Serializable{"a": &Int{}},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": &Int{}},
			},
			&Object{
				entries:         map[string]Serializable{"a": &Int{}},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			&Object{
				entries: map[string]Serializable{"a": &Int{}},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": &Int{}},
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

}

func TestSymbolicRecord(t *testing.T) {
	cases := []struct {
		record1 *Record
		record2 *Record
		ok      bool
	}{
		{&Record{entries: nil}, &Record{entries: nil}, true},
		{&Record{entries: map[string]Serializable{}}, &Record{entries: nil}, false},
		{&Record{entries: nil}, &Record{entries: map[string]Serializable{}}, true},

		{&Record{entries: map[string]Serializable{}}, &Record{entries: map[string]Serializable{}}, true},
		{
			&Record{
				entries: map[string]Serializable{"a": &Int{}},
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
				entries: map[string]Serializable{"a": &Int{}},
			},
			false,
		},
		{
			&Record{
				entries:         map[string]Serializable{"a": &Int{}},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			&Record{
				entries: map[string]Serializable{},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": &Int{}},
			},
			&Record{
				entries: map[string]Serializable{"a": &Int{}},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": &Int{}},
			},
			&Record{
				entries:         map[string]Serializable{"a": &Int{}},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			&Record{
				entries: map[string]Serializable{"a": &Int{}},
			},
			true,
		},
		{
			&Record{
				entries: map[string]Serializable{"a": &Int{}},
			},
			&Record{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			false,
		},
	}

	for _, testCase := range cases {
		t.Run(t.Name()+"_"+fmt.Sprint(testCase.record1, "_", testCase.record2), func(t *testing.T) {
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
				&List{elements: nil, generalElement: &Int{}},
				true,
			},
			{
				&List{elements: nil, generalElement: &Int{}},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				false,
			},
			{
				&List{elements: nil, generalElement: &Int{}},
				&List{elements: nil, generalElement: &Int{}},
				true,
			},
			{
				&List{elements: nil, generalElement: &Int{}},
				&List{elements: []Serializable{&Int{}}, generalElement: nil},
				true,
			},
			{
				&List{elements: nil, generalElement: &Int{}},
				&List{elements: []Serializable{&Int{}, ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&List{elements: []Serializable{&Int{}}, generalElement: nil},
				&List{elements: nil, generalElement: &Int{}},
				false,
			},
			{
				&List{elements: []Serializable{&Int{}}, generalElement: nil},
				&List{elements: []Serializable{&Int{}}, generalElement: nil},
				true,
			},
			{
				&List{elements: []Serializable{&Int{}}, generalElement: nil},
				&List{elements: []Serializable{ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&List{elements: []Serializable{&Int{}, &Int{}}, generalElement: nil},
				&List{elements: []Serializable{&Int{}, ANY_BOOL}, generalElement: nil},
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
				&Tuple{elements: nil, generalElement: &Int{}},
				true,
			},
			{
				&Tuple{elements: nil, generalElement: &Int{}},
				&Tuple{elements: nil, generalElement: ANY_SERIALIZABLE},
				false,
			},
			{
				&Tuple{elements: nil, generalElement: &Int{}},
				&Tuple{elements: nil, generalElement: &Int{}},
				true,
			},
			{
				&Tuple{elements: nil, generalElement: &Int{}},
				&Tuple{elements: []Serializable{&Int{}}, generalElement: nil},
				true,
			},
			{
				&Tuple{elements: nil, generalElement: &Int{}},
				&Tuple{elements: []Serializable{&Int{}, ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&Tuple{elements: []Serializable{&Int{}}, generalElement: nil},
				&Tuple{elements: nil, generalElement: &Int{}},
				false,
			},
			{
				&Tuple{elements: []Serializable{&Int{}}, generalElement: nil},
				&Tuple{elements: []Serializable{&Int{}}, generalElement: nil},
				true,
			},
			{
				&Tuple{elements: []Serializable{&Int{}}, generalElement: nil},
				&Tuple{elements: []Serializable{ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&Tuple{elements: []Serializable{&Int{}, &Int{}}, generalElement: nil},
				&Tuple{elements: []Serializable{&Int{}, ANY_BOOL}, generalElement: nil},
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
				entries: map[string]Serializable{"./a": &Int{}},
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
				entries: map[string]Serializable{"./a": &Int{}},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			false,
		},
		{
			&Dictionary{
				entries: map[string]Serializable{"./a": &Int{}},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			&Dictionary{
				entries: map[string]Serializable{"./a": &Int{}},
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
				entries: map[string]Serializable{"./a": &Int{}},
				keys:    map[string]Serializable{"./a": &Path{}},
			},
			true,
		},
		{
			&Dictionary{
				entries: map[string]Serializable{"./a": &Int{}},
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
