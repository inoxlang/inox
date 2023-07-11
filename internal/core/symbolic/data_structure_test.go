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

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			object  *Object
			widened *Object
			ok      bool
		}{
			{
				&Object{},
				nil,
				false,
			},
			{
				&Object{
					entries: make(map[string]Serializable),
				},
				&Object{},
				true,
			},
			{
				&Object{
					entries: map[string]Serializable{
						"name": &String{},
					},
				},
				&Object{},
				true,
			},
			{
				&Object{
					entries: map[string]Serializable{
						"any": ANY_SERIALIZABLE,
					},
				},
				&Object{},
				true,
			},
			{
				&Object{
					entries: map[string]Serializable{
						"list": &List{generalElement: ANY_SERIALIZABLE},
					},
				},
				&Object{},
				true,
			},
			{
				&Object{
					entries: map[string]Serializable{
						"list": ANY_SERIALIZABLE,
					},
				},
				&Object{},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.object), func(t *testing.T) {

				widened, ok := testCase.object.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.object.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
	})
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

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			record  *Record
			widened *Record
			ok      bool
		}{
			{
				&Record{},
				nil,
				false,
			},
			{
				&Record{
					entries: make(map[string]Serializable),
				},
				&Record{},
				true,
			},
			{
				&Record{
					entries: map[string]Serializable{
						"name": &String{},
					},
				},
				&Record{},
				true,
			},
			{
				&Record{
					entries: map[string]Serializable{
						"any": ANY_SERIALIZABLE,
					},
				},
				&Record{},
				true,
			},
			{
				&Record{
					entries: map[string]Serializable{
						"list": &List{generalElement: ANY_SERIALIZABLE},
					},
				},
				&Record{},
				true,
			},
			{
				&Record{
					entries: map[string]Serializable{
						"list": ANY_SERIALIZABLE,
					},
				},
				&Record{},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.record), func(t *testing.T) {

				widened, ok := testCase.record.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.record.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
	})
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
				&List{elements: []Serializable{&Int{}, &Bool{}}, generalElement: nil},
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
				&List{elements: []Serializable{&Bool{}}, generalElement: nil},
				false,
			},
			{
				&List{elements: []Serializable{&Int{}, &Int{}}, generalElement: nil},
				&List{elements: []Serializable{&Int{}, &Bool{}}, generalElement: nil},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.list1, "_", testCase.list2), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.list1.Test(testCase.list2))
			})
		}
	})

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			list    *List
			widened *List
			ok      bool
		}{
			{
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				nil,
				false,
			},
			{
				&List{elements: nil, generalElement: &Int{}},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				true,
			},
			{
				&List{elements: []Serializable{&Int{}}, generalElement: nil},
				&List{elements: nil, generalElement: &Int{}},
				true,
			},
			{
				&List{elements: []Serializable{&Int{}, &String{}}, generalElement: nil},
				&List{elements: nil, generalElement: asSerializable(NewMultivalue(&Int{}, &String{})).(Serializable)},
				true,
			},
			{
				&List{
					elements: []Serializable{
						&List{elements: []Serializable{&Int{}}},
						&String{},
					},
					generalElement: nil,
				},
				&List{
					elements: []Serializable{
						&List{generalElement: &Int{}},
						&String{},
					},
					generalElement: nil,
				},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.list), func(t *testing.T) {

				widened, ok := testCase.list.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.list.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
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
				&Tuple{elements: []Serializable{&Int{}, &Bool{}}, generalElement: nil},
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
				&Tuple{elements: []Serializable{&Bool{}}, generalElement: nil},
				false,
			},
			{
				&Tuple{elements: []Serializable{&Int{}, &Int{}}, generalElement: nil},
				&Tuple{elements: []Serializable{&Int{}, &Bool{}}, generalElement: nil},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.tuple1, "_", testCase.tuple2), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.tuple1.Test(testCase.tuple2))
			})
		}
	})

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			tuple   *Tuple
			widened *Tuple
			ok      bool
		}{
			{
				&Tuple{elements: nil, generalElement: ANY_SERIALIZABLE},
				nil,
				false,
			},
			{
				&Tuple{elements: nil, generalElement: &Int{}},
				&Tuple{elements: nil, generalElement: ANY_SERIALIZABLE},
				true,
			},
			{
				&Tuple{elements: []Serializable{&Int{}}, generalElement: nil},
				&Tuple{elements: nil, generalElement: &Int{}},
				true,
			},
			{
				&Tuple{elements: []Serializable{&Int{}, &String{}}, generalElement: nil},
				&Tuple{elements: nil, generalElement: asSerializable(NewMultivalue(&Int{}, &String{})).(Serializable)},
				true,
			},
			{
				&Tuple{
					elements: []Serializable{
						&Tuple{elements: []Serializable{&Int{}}},
						&String{},
					},
					generalElement: nil,
				},
				&Tuple{
					elements: []Serializable{
						&Tuple{generalElement: &Int{}},
						&String{},
					},
					generalElement: nil,
				},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprintf("%#v", testCase.tuple), func(t *testing.T) {

				widened, ok := testCase.tuple.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.tuple.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
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

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			list    *KeyList
			widened *KeyList
			ok      bool
		}{
			{
				&KeyList{Keys: nil},
				nil,
				false,
			},
			{
				&KeyList{Keys: []string{"name"}},
				&KeyList{Keys: nil},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.list), func(t *testing.T) {

				widened, ok := testCase.list.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.list.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
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
			&Dictionary{Entries: nil},
			&Dictionary{Entries: nil},
			true,
		},
		{
			&Dictionary{Entries: map[string]SymbolicValue{}},
			&Dictionary{Entries: nil},
			false,
		},
		{
			&Dictionary{Entries: nil},
			&Dictionary{Entries: map[string]SymbolicValue{}},
			true,
		},
		{
			&Dictionary{Entries: map[string]SymbolicValue{}, Keys: map[string]SymbolicValue{}},
			&Dictionary{Entries: map[string]SymbolicValue{}, Keys: map[string]SymbolicValue{}},
			true,
		},
		{
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": &Int{}},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			&Dictionary{
				Entries: map[string]SymbolicValue{},
			},
			false,
		},
		{
			&Dictionary{
				Entries: map[string]SymbolicValue{},
			},
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": &Int{}},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			false,
		},
		{
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": &Int{}},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": &Int{}},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			true,
		},
		{
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": ANY},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": &Int{}},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			true,
		},
		{
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": &Int{}},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			&Dictionary{
				Entries: map[string]SymbolicValue{"./a": ANY},
				Keys:    map[string]SymbolicValue{"./a": &Path{}},
			},
			false,
		},
	}

	for _, testCase := range cases {
		t.Run(t.Name()+"_"+fmt.Sprint(testCase.dict1, "_", testCase.dict2), func(t *testing.T) {
			assert.Equal(t, testCase.oneTestTwoResult, testCase.dict1.Test(testCase.dict2))
		})
	}

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			dict    *Dictionary
			widened *Dictionary
			ok      bool
		}{
			{
				&Dictionary{},
				nil,
				false,
			},
			{
				&Dictionary{
					Entries: make(map[string]SymbolicValue),
					Keys:    make(map[string]SymbolicValue),
				},
				&Dictionary{},
				true,
			},
			{
				&Dictionary{
					Entries: map[string]SymbolicValue{
						"\"name\"": &String{},
					},
					Keys: map[string]SymbolicValue{
						"\"name\"": &String{},
					},
				},
				&Dictionary{},
				true,
			},
			{
				&Dictionary{
					Entries: map[string]SymbolicValue{
						"\"any\"": ANY,
					},
					Keys: map[string]SymbolicValue{
						"\"any\"": &String{},
					},
				},
				&Dictionary{},
				true,
			},
			{
				&Dictionary{
					Entries: map[string]SymbolicValue{
						"\"list\"": &List{generalElement: ANY_SERIALIZABLE},
					},
					Keys: map[string]SymbolicValue{
						"\"list\"": &String{},
					},
				},
				&Dictionary{},
				true,
			},
			{
				&Dictionary{
					Entries: map[string]SymbolicValue{
						"\"list\"": ANY,
					},
					Keys: map[string]SymbolicValue{
						"\"list\"": &String{},
					},
				},
				&Dictionary{},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.dict), func(t *testing.T) {

				widened, ok := testCase.dict.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.dict.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
	})
}
