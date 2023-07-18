package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDictionaryClone(t *testing.T) {
	clone, err := NewDictionary(nil).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, NewDictionary(nil), clone)

	clone, err = NewDictionary(map[string]Serializable{"/": Int(1)}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, NewDictionary(map[string]Serializable{"/": Int(1)}), clone)

	// //not clonable
	// clone, err = NewDictionary(map[string]Serializable{"/": &ValueListIterator{}}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	// assert.Equal(t, ErrNotClonable, err)
	// assert.Nil(t, clone)

	//cycle
	dict := NewDictionary(nil)
	dict.entries["\"self\""] = dict
	dict.keys["\"self\""] = Str("self")
	clone, err = dict.PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.ErrorIs(t, err, ErrMaximumPseudoCloningDepthReached)
	assert.Nil(t, clone)
}

func TestValueListClone(t *testing.T) {
	clone, err := (&ValueList{}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ValueList{elements: []Serializable{}}, clone)

	clone, err = (&ValueList{elements: []Serializable{Int(1)}}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ValueList{elements: []Serializable{Int(1)}}, clone)

	// //not clonable
	// clone, err = (&ValueList{elements: []Serializable{&ValueListIterator{}}}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	// assert.Equal(t, ErrNotClonable, err)
	// assert.Nil(t, clone)

	//cycle
	list := &ValueList{elements: []Serializable{Int(0)}}
	list.elements[0] = list
	clone, err = list.PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.ErrorIs(t, err, ErrMaximumPseudoCloningDepthReached)
	assert.Nil(t, clone)
}

func TestRuneSliceClone(t *testing.T) {
	clone, err := (&RuneSlice{elements: []rune{}}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, (&RuneSlice{elements: []rune{}}), clone)

	clone, err = (&RuneSlice{elements: []rune{'a'}}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, (&RuneSlice{elements: []rune{'a'}}), clone)
}

func TestByteSliceClone(t *testing.T) {
	clone, err := (&ByteSlice{Bytes: []byte{}, IsDataMutable: true}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ByteSlice{Bytes: []byte{}, IsDataMutable: true}, clone)

	clone, err = (&ByteSlice{Bytes: []byte{'a'}, IsDataMutable: true}).PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ByteSlice{Bytes: []byte{'a'}, IsDataMutable: true}, clone)
}

func TestOptionClone(t *testing.T) {
	clone, err := Option{Name: "a", Value: Int(1)}.PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, Option{Name: "a", Value: Int(1)}, clone)

	clone, err = Option{Name: "a", Value: objFrom(ValMap{"a": Int(1)})}.PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.NoError(t, err)

	expectedObj := objFrom(ValMap{"a": Int(1)})
	Share(expectedObj, nil)
	assert.Equal(t, Option{Name: "a", Value: expectedObj}, clone)

	//not clonable
	clone, err = Option{Name: "a", Value: &ValueListIterator{}}.PseudoClone(nil, &[]PotentiallySharable{}, 0)
	assert.ErrorIs(t, err, ErrValueNotSharableNorClonable)
	assert.Nil(t, clone)
}
