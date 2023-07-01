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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (ANY).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		any := ANY

		assert.False(t, any.IsWidenable())

		widened, ok := any.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicNil(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		_nil := &NilT{}

		assert.True(t, _nil.Test(_nil))
		assert.False(t, _nil.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&NilT{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		_nil := &NilT{}

		assert.False(t, _nil.IsWidenable())

		widened, ok := _nil.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicBool(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		bool := &Bool{}

		assert.True(t, bool.Test(bool))
		assert.True(t, bool.Test(&Bool{}))
		assert.False(t, bool.Test(&Int{}))
	})
	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Bool{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		_nil := &Bool{}

		assert.False(t, _nil.IsWidenable())

		widened, ok := _nil.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicFloat(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		float := &Float{}

		assert.True(t, float.Test(float))
		assert.True(t, float.Test(&Float{}))
		assert.False(t, float.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Float{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		_nil := &Float{}

		assert.False(t, _nil.IsWidenable())

		widened, ok := _nil.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicInt(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		int := &Int{}

		assert.True(t, int.Test(int))
		assert.True(t, int.Test(&Int{}))
		assert.False(t, int.Test(&Float{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Int{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		int := &Int{}

		assert.False(t, int.IsWidenable())

		widened, ok := int.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicRune(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		rune := &Rune{}

		assert.True(t, rune.Test(rune))
		assert.True(t, rune.Test(&Rune{}))
		assert.False(t, rune.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Rune{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		rune := &Rune{}

		assert.False(t, rune.IsWidenable())

		widened, ok := rune.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Path{}).IsWidenable())
		assert.True(t, (&Path{dirConstraint: DirPath}).IsWidenable())
		assert.True(t, (&Path{absoluteness: AbsolutePath}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		t.Run("non specific", func(t *testing.T) {
			path := &Path{}

			assert.False(t, path.IsWidenable())

			widened, ok := path.Widen()
			assert.False(t, ok)
			assert.Nil(t, widened)
		})

		t.Run("dir path", func(t *testing.T) {
			path := &Path{dirConstraint: DirPath}

			assert.True(t, path.IsWidenable())

			widened, ok := path.Widen()
			assert.True(t, ok)
			assert.Equal(t, ANY_PATH, widened)
		})

		t.Run("absolute path", func(t *testing.T) {
			path := &Path{absoluteness: AbsolutePath}

			assert.True(t, path.IsWidenable())

			widened, ok := path.Widen()
			assert.True(t, ok)
			assert.Equal(t, ANY_PATH, widened)
		})
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&URL{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		url := &URL{}

		assert.False(t, url.IsWidenable())

		widened, ok := url.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Host{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		host := &Host{}

		assert.False(t, host.IsWidenable())

		widened, ok := host.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		specificIdent := &Identifier{name: "foo"}
		ident := &Identifier{}

		assert.True(t, specificIdent.IsWidenable())
		assert.False(t, ident.IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		specificIdent := &Identifier{name: "foo"}
		ident := &Identifier{}

		widened, ok := specificIdent.Widen()
		assert.True(t, ok)
		assert.Equal(t, &Identifier{}, widened)

		widened, ok = ident.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Option{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		option := &Option{}

		assert.False(t, option.IsWidenable())

		widened, ok := option.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		specificNode := &AstNode{Node: &parse.ContinueStatement{}}
		node := &AstNode{}

		assert.True(t, specificNode.IsWidenable())
		assert.False(t, node.IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		specificNode := &AstNode{Node: &parse.ContinueStatement{}}
		node := &AstNode{}

		widened, ok := specificNode.Widen()
		assert.True(t, ok)
		assert.Equal(t, &AstNode{}, widened)

		widened, ok = node.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicError(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		err := &Error{data: &Int{}}

		assert.True(t, err.Test(err))
		assert.True(t, err.Test(&Error{data: &Int{}}))
		assert.False(t, err.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Error{data: ANY}).IsWidenable())
		assert.True(t, (&Error{data: &Identifier{name: "i"}}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		anyDataErr := &Error{data: ANY}

		assert.False(t, anyDataErr.IsWidenable())

		widened, ok := anyDataErr.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)

		//

		intDataErr := &Error{data: &Identifier{name: "i"}}

		assert.True(t, intDataErr.IsWidenable())

		widened, ok = intDataErr.Widen()
		assert.True(t, ok)
		assert.Equal(t, &Error{data: &Identifier{}}, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&GoFunction{}).IsWidenable())
		assert.True(t, (&GoFunction{fn: symbolicGoFn}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		{
			anyFunc := &GoFunction{}

			assert.False(t, anyFunc.IsWidenable())
			widened, ok := anyFunc.Widen()
			assert.False(t, ok)
			assert.Nil(t, widened)
		}

		{
			specificFunc := &GoFunction{fn: symbolicGoFn}

			assert.True(t, specificFunc.IsWidenable())
			widened, ok := specificFunc.Widen()
			assert.True(t, ok)
			assert.Equal(t, &GoFunction{}, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&RuneSlice{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		slice := &RuneSlice{}

		assert.False(t, slice.IsWidenable())
		widened, ok := slice.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicQuantityRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		qtyRange := &QuantityRange{}

		assert.True(t, qtyRange.Test(qtyRange))
		assert.True(t, qtyRange.Test(&QuantityRange{}))
		assert.False(t, qtyRange.Test(&String{}))
		assert.False(t, qtyRange.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&QuantityRange{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		qtyRange := &QuantityRange{}

		assert.False(t, qtyRange.IsWidenable())
		widened, ok := qtyRange.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&IntRange{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		intRange := &IntRange{}

		assert.False(t, intRange.IsWidenable())
		widened, ok := intRange.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&RuneRange{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		runeRange := &RuneRange{}

		assert.False(t, runeRange.IsWidenable())
		widened, ok := runeRange.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicAnyIterable(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyIterable := &AnyIterable{}

		assert.True(t, anyIterable.Test(anyIterable))
		assert.True(t, anyIterable.Test(NewList()))
		assert.True(t, anyIterable.Test(NewListOf(ANY)))
		assert.True(t, anyIterable.Test(NewListOf(&Int{})))
		assert.False(t, anyIterable.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&AnyIterable{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		anyIterable := ANY

		assert.False(t, anyIterable.IsWidenable())

		widened, ok := anyIterable.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func symbolicGoFn(ctx *Context, list *List, args ...SymbolicValue) *List {
	return list
}
