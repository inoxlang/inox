package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestSimpleValueCloning(t *testing.T) {
	assert.Equal(t, True, utils.Must(True.Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, Int(1), utils.Must(Int(1).Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, Float(1), utils.Must(Float(1).Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, Str("a"), utils.Must(Str("a").Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, Identifier("a"), utils.Must(Identifier("a").Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, EmailAddress("a@a.com"), utils.Must(EmailAddress("a@a.com").Clone(map[uintptr]map[int]Value{}, 0)))

	assert.Equal(t, Path("/"), utils.Must(Path("/").Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, PathPattern("/"), utils.Must(PathPattern("/").Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, Host("https://example.com"), utils.Must(Host("https://example.com").Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, HostPattern("https://**.com"), utils.Must(HostPattern("https://**.com").Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, URL("https://example.com/"), utils.Must(URL("https://example.com/").Clone(map[uintptr]map[int]Value{}, 0)))
	assert.Equal(t, URLPattern("https://example.com/..."), utils.Must(URLPattern("https://example.com/...").Clone(map[uintptr]map[int]Value{}, 0)))
}

func TestObjectClone(t *testing.T) {
	clone, err := objFrom(nil).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &Object{}, clone)

	clone, err = objFrom(ValMap{"a": Int(1)}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, objFrom(ValMap{"a": Int(1)}), clone)

	// //not clonable
	// clone, err = objFrom(ValMap{"a": &ValueListIterator{}}).Clone(map[uintptr]map[int]Value{}, 0)
	// assert.Equal(t, ErrNotClonable, err)
	// assert.Nil(t, clone)

	//cycle
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	obj := &Object{}
	obj.SetProp(ctx, "self", obj)
	clone, err = obj.Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.IsType(t, &Object{}, clone)
	objectClone := clone.(*Object)
	assert.Equal(t, 1, len(objectClone.keys))
	assert.True(t, SamePointer(objectClone, objectClone.Prop(ctx, "self").(*Object)))
}

func TestDictionaryClone(t *testing.T) {
	clone, err := NewDictionary(nil).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, NewDictionary(nil), clone)

	clone, err = NewDictionary(map[string]Serializable{"/": Int(1)}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, NewDictionary(map[string]Serializable{"/": Int(1)}), clone)

	// //not clonable
	// clone, err = NewDictionary(map[string]Serializable{"/": &ValueListIterator{}}).Clone(map[uintptr]map[int]Value{}, 0)
	// assert.Equal(t, ErrNotClonable, err)
	// assert.Nil(t, clone)

	//cycle
	dict := NewDictionary(nil)
	dict.entries["\"self\""] = dict
	dict.keys["\"self\""] = Str("self")
	clone, err = dict.Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.IsType(t, &Dictionary{}, clone)
	dictClone := clone.(*Dictionary)
	assert.Equal(t, 1, len(dictClone.entries))
	assert.True(t, SamePointer(dictClone, dictClone.entries["\"self\""].(*Dictionary)))
}

func TestValueListClone(t *testing.T) {
	clone, err := (&ValueList{}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ValueList{elements: []Serializable{}}, clone)

	clone, err = (&ValueList{elements: []Serializable{Int(1)}}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ValueList{elements: []Serializable{Int(1)}}, clone)

	// //not clonable
	// clone, err = (&ValueList{elements: []Serializable{&ValueListIterator{}}}).Clone(map[uintptr]map[int]Value{}, 0)
	// assert.Equal(t, ErrNotClonable, err)
	// assert.Nil(t, clone)

	//cycle
	list := &ValueList{elements: []Serializable{Int(0)}}
	list.elements[0] = list
	clone, err = list.Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.IsType(t, &ValueList{}, clone)
	listClone := clone.(*ValueList)
	assert.Equal(t, 1, len(listClone.elements))
	elem := listClone.elements[0].(*ValueList)
	assert.Equal(t, 1, len(listClone.elements))
	assert.Equal(t, 1, len(elem.elements))
	assert.True(t, SamePointer(listClone, elem))
}

func TestKeyListClone(t *testing.T) {
	clone, err := KeyList{}.Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, KeyList{}, clone)

	clone, err = KeyList{"a"}.Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, KeyList{"a"}, clone)
}

func TestRuneSliceClone(t *testing.T) {
	clone, err := (&RuneSlice{elements: []rune{}}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, (&RuneSlice{elements: []rune{}}), clone)

	clone, err = (&RuneSlice{elements: []rune{'a'}}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, (&RuneSlice{elements: []rune{'a'}}), clone)
}

func TestByteSliceClone(t *testing.T) {
	clone, err := (&ByteSlice{Bytes: []byte{}, IsDataMutable: true}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ByteSlice{Bytes: []byte{}, IsDataMutable: true}, clone)

	clone, err = (&ByteSlice{Bytes: []byte{'a'}, IsDataMutable: true}).Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, &ByteSlice{Bytes: []byte{'a'}, IsDataMutable: true}, clone)
}

func TestOptionClone(t *testing.T) {
	clone, err := Option{Name: "a", Value: Int(1)}.Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, Option{Name: "a", Value: Int(1)}, clone)

	clone, err = Option{Name: "a", Value: objFrom(ValMap{"a": Int(1)})}.Clone(map[uintptr]map[int]Value{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, Option{Name: "a", Value: objFrom(ValMap{"a": Int(1)})}, clone)

	//not clonable
	clone, err = Option{Name: "a", Value: &ValueListIterator{}}.Clone(map[uintptr]map[int]Value{}, 0)
	assert.ErrorIs(t, err, ErrNotClonable)
	assert.Nil(t, clone)
}
