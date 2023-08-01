package symbolic

import (
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicAny(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		any := ANY

		assert.True(t, any.Test(any))
		assert.True(t, any.Test(&Int{}))
	})

}

func TestSymbolicNil(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		_nil := &NilT{}

		assert.True(t, _nil.Test(_nil))
		assert.False(t, _nil.Test(&Int{}))
	})

}

func TestSymbolicBool(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		bool := ANY_BOOL

		assert.True(t, bool.Test(bool))
		assert.True(t, bool.Test(ANY_BOOL))
		assert.False(t, bool.Test(&Int{}))
	})

}

func TestSymbolicFloat(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		float := &Float{}

		assert.True(t, float.Test(float))
		assert.True(t, float.Test(&Float{}))
		assert.False(t, float.Test(&Int{}))
	})

}

func TestSymbolicInt(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		int := &Int{}

		assert.True(t, int.Test(int))
		assert.True(t, int.Test(&Int{}))
		assert.False(t, int.Test(&Float{}))
	})

}

func TestSymbolicRune(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		rune := &Rune{}

		assert.True(t, rune.Test(rune))
		assert.True(t, rune.Test(&Rune{}))
		assert.False(t, rune.Test(&Int{}))
	})

}

func TestSymbolicPath(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyPath := &Path{}
		assert.True(t, anyPath.Test(anyPath))
		assert.True(t, anyPath.Test(&Path{}))
		assert.False(t, anyPath.Test(&String{}))
		assert.False(t, anyPath.Test(&Int{}))

		anyAbsPath := &Path{absoluteness: AbsolutePath}
		assert.True(t, anyAbsPath.Test(anyAbsPath))
		assert.True(t, anyAbsPath.Test(&Path{absoluteness: AbsolutePath}))
		assert.True(t, anyPath.Test(anyAbsPath))
		assert.False(t, anyAbsPath.Test(anyPath))

		anyDirPath := &Path{dirConstraint: DirPath}
		assert.True(t, anyDirPath.Test(anyDirPath))
		assert.True(t, anyDirPath.Test(&Path{dirConstraint: DirPath}))
		assert.True(t, anyPath.Test(anyDirPath))
		assert.False(t, anyDirPath.Test(anyPath))
		assert.False(t, anyDirPath.Test(anyAbsPath))
	})

}

func TestSymbolicURL(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		url := &URL{}

		assert.True(t, url.Test(url))
		assert.True(t, url.Test(&URL{}))
		assert.False(t, url.Test(&String{}))
		assert.False(t, url.Test(&Int{}))
	})

}

func TestSymbolicHost(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		host := &Host{}

		assert.True(t, host.Test(host))
		assert.True(t, host.Test(&Host{}))
		assert.False(t, host.Test(&String{}))
		assert.False(t, host.Test(&Int{}))
	})

}

func TestSymbolicIdentifier(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		specificIdent := &Identifier{name: "foo"}
		ident := &Identifier{}

		assert.True(t, specificIdent.Test(specificIdent))
		assert.False(t, specificIdent.Test(ident))

		assert.True(t, ident.Test(ident))
		assert.True(t, ident.Test(specificIdent))
	})

}

func TestSymbolicOption(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		option := &Option{}

		assert.True(t, option.Test(option))
		assert.True(t, option.Test(&Option{}))
		assert.False(t, option.Test(&String{}))
		assert.False(t, option.Test(&Int{}))
	})

}

func TestSymbolicNode(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		specificNode := &AstNode{Node: &parse.ContinueStatement{}}
		node := &AstNode{}

		assert.True(t, specificNode.Test(specificNode))
		assert.False(t, specificNode.Test(node))

		assert.True(t, node.Test(node))
		assert.True(t, node.Test(specificNode))
	})

}

func TestSymbolicError(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		err := &Error{data: &Int{}}

		assert.True(t, err.Test(err))
		assert.True(t, err.Test(&Error{data: &Int{}}))
		assert.False(t, err.Test(&Int{}))
	})

}

func TestSymbolicGoFunction(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		{
			anyFunc := &GoFunction{}
			assert.True(t, anyFunc.Test(anyFunc))
			assert.True(t, anyFunc.Test(&GoFunction{}))
			assert.False(t, anyFunc.Test(&Int{}))
		}

		{
			specificFunc := &GoFunction{fn: symbolicGoFn}
			assert.True(t, specificFunc.Test(specificFunc))
			assert.False(t, specificFunc.Test(&GoFunction{}))
			assert.False(t, specificFunc.Test(&Int{}))
		}
	})

}

func TestSymbolicRuneSlice(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		slice := &RuneSlice{}

		assert.True(t, slice.Test(slice))
		assert.True(t, slice.Test(&RuneSlice{}))
		assert.False(t, slice.Test(&String{}))
		assert.False(t, slice.Test(&Int{}))
	})

}

func TestSymbolicQuantityRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyQtyRange := &QuantityRange{element: ANY_SERIALIZABLE}

		assert.True(t, anyQtyRange.Test(anyQtyRange))
		assert.True(t, anyQtyRange.Test(&QuantityRange{element: ANY_SERIALIZABLE}))
		assert.True(t, anyQtyRange.Test(NewQuantityRange(ANY_BYTECOUNT)))
		assert.False(t, anyQtyRange.Test(ANY_STR))
		assert.False(t, anyQtyRange.Test(ANY_INT))

		qtyRange := NewQuantityRange(ANY_BYTECOUNT)

		assert.True(t, qtyRange.Test(qtyRange))
		assert.True(t, qtyRange.Test(NewQuantityRange(ANY_BYTECOUNT)))
		assert.False(t, qtyRange.Test(&QuantityRange{element: ANY_SERIALIZABLE}))
		assert.False(t, qtyRange.Test(ANY_STR))
		assert.False(t, qtyRange.Test(ANY_INT))
	})

}

func TestSymbolicIntRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		intRange := &IntRange{}

		assert.True(t, intRange.Test(intRange))
		assert.True(t, intRange.Test(&IntRange{}))
		assert.False(t, intRange.Test(&String{}))
		assert.False(t, intRange.Test(&Int{}))
	})

}

func TestSymbolicRuneRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		runeRange := &RuneRange{}

		assert.True(t, runeRange.Test(runeRange))
		assert.True(t, runeRange.Test(&RuneRange{}))
		assert.False(t, runeRange.Test(&String{}))
		assert.False(t, runeRange.Test(&Int{}))
	})

}

func TestSymbolicAnyIterable(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyIterable := &AnyIterable{}

		assert.True(t, anyIterable.Test(anyIterable))
		assert.True(t, anyIterable.Test(NewList()))
		assert.True(t, anyIterable.Test(NewListOf(ANY_SERIALIZABLE)))
		assert.True(t, anyIterable.Test(NewListOf(&Int{})))
		assert.False(t, anyIterable.Test(&Int{}))
	})

}

func symbolicGoFn(ctx *Context, list *List, args ...SymbolicValue) *List {
	return list
}
