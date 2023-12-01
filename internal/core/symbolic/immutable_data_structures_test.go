package symbolic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicRecord(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {

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
				assert.Equal(t, testCase.ok, testCase.record1.Test(testCase.record2, RecTestCallState{}))
			})
		}

		t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			rec := &Record{}
			rec.entries = map[string]Serializable{
				"self": rec,
			}
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				rec.Test(rec, RecTestCallState{})
			})
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
				assert.Equal(t, testCase.ok, testCase.tuple1.Test(testCase.tuple2, RecTestCallState{}))
			})
		}

		t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			tuple1 := &Tuple{}
			tuple1.elements = []Serializable{tuple1}
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				tuple1.Test(tuple1, RecTestCallState{})
			})

			tuple2 := &Tuple{}
			tuple2.generalElement = tuple2
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				tuple2.Test(tuple2, RecTestCallState{})
			})
		})
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
				assert.Equal(t, testCase.ok, testCase.list1.Test(testCase.list2, RecTestCallState{}))
			})
		}
	})

}
