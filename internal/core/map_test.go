package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {

	t.Run("property name mapper", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		// should work with any Iprops
		mapper := PropertyName("name")

		obj := NewObjectFromMap(ValMap{"name": Str("a")}, ctx)
		result := Map(ctx, NewWrappedValueList(obj), mapper)
		assert.Equal(t, NewWrappedValueList(Str("a")), result)

		fileInfo := FileInfo{Name_: "file.txt"}
		result = Map(ctx, NewWrappedValueList(fileInfo), mapper)
		assert.Equal(t, NewWrappedValueList(Str("file.txt")), result)
	})

	t.Run("key list mapper", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		// should work with any Iprops
		mapper := KeyList{"name"}

		obj := NewObjectFromMap(ValMap{"name": Str("a")}, ctx)
		result := Map(ctx, NewWrappedValueList(obj), mapper)
		assert.Equal(t, NewWrappedValueList(objFrom(ValMap{
			"name": Str("a"),
		})), result)

		fileInfo := FileInfo{Name_: "file.txt"}
		result = Map(ctx, NewWrappedValueList(fileInfo), mapper)
		assert.Equal(t, NewWrappedValueList(objFrom(ValMap{
			"name": Str("file.txt"),
		})), result)
	})

}
