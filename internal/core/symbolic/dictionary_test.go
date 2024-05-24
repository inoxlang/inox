package symbolic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDictionary(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {

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
				assert.Equal(t, testCase.oneTestTwoResult, testCase.dict1.Test(testCase.dict2, RecTestCallState{}))
			})
		}

		t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			dict := &Dictionary{}

			dict.entries = map[string]Serializable{
				"./a": dict,
			}
			dict.keys = map[string]Serializable{
				"./a": NewPath("./a"),
			}

			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				dict.Test(dict, RecTestCallState{})
			})
		})
	})

}
