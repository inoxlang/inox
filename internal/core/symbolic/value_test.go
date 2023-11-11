package symbolic

import (
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
)

func TestSymbolicAny(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		any := ANY

		assertTest(t, any, any)
		assertTest(t, any, &Int{})
	})

}

func TestSymbolicNil(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		_nil := &NilT{}

		assertTest(t, _nil, _nil)
		assertTestFalse(t, _nil, &Int{})
	})

}

func TestSymbolicBool(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		bool := ANY_BOOL

		assertTest(t, bool, bool)
		assertTest(t, bool, ANY_BOOL)
		assertTestFalse(t, bool, &Int{})
	})

}

func TestSymbolicFloat(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		float := &Float{}

		assertTest(t, float, float)
		assertTest(t, float, &Float{})
		assertTestFalse(t, float, &Int{})
	})

}

func TestSymbolicInt(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyInt := &Int{}
		anyIntMatchingSpecificPattern := &Int{matchingPattern: &IntRangePattern{
			intRange: NewIncludedEndIntRange(INT_1, INT_2),
		}}

		assertTest(t, anyInt, anyInt)
		assertTest(t, anyInt, &Int{})
		assertTest(t, anyInt, INT_1)
		assertTest(t, anyInt, anyIntMatchingSpecificPattern)
		assertTestFalse(t, anyInt, ANY_FLOAT)
	})

}

func TestSymbolicRune(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		rune := &Rune{}

		assertTest(t, rune, rune)
		assertTest(t, rune, &Rune{})
		assertTestFalse(t, rune, &Int{})
	})

}

func TestSymbolicPath(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyPath := &Path{}
		assertTest(t, anyPath, anyPath)
		assertTest(t, anyPath, &Path{})
		assertTestFalse(t, anyPath, &String{})
		assertTestFalse(t, anyPath, &Int{})

		anyAbsPath := ANY_ABS_PATH
		assertTest(t, anyAbsPath, anyAbsPath)
		assertTest(t, anyAbsPath, NewPath("/"))
		assertTest(t, anyAbsPath, NewPath("/1"))
		assertTestFalse(t, anyAbsPath, NewPath("./1"))
		assertTestFalse(t, anyAbsPath, anyPath)
		assertTestFalse(t, anyAbsPath, &String{})

		anyDirPath := ANY_DIR_PATH
		assertTest(t, anyDirPath, anyDirPath)
		assertTest(t, anyDirPath, NewPath("/"))
		assertTest(t, anyDirPath, NewPath("./"))
		assertTest(t, anyDirPath, NewPath("./dir/"))
		assertTestFalse(t, anyDirPath, NewPath("/1"))
		assertTestFalse(t, anyDirPath, NewPath("./1"))
		assertTestFalse(t, anyDirPath, anyPath)
		assertTestFalse(t, anyDirPath, anyAbsPath)

		pathWithValue := NewPath("/")
		assertTest(t, pathWithValue, pathWithValue)
		assertTest(t, pathWithValue, NewPath("/"))
		assertTestFalse(t, pathWithValue, NewPath("/1"))
		assertTestFalse(t, pathWithValue, NewPath("./"))
		assertTestFalse(t, pathWithValue, NewPathMatchingPattern(NewPathPattern("/...")))
		assertTestFalse(t, pathWithValue, anyDirPath)
		assertTestFalse(t, pathWithValue, anyPath)
		assertTestFalse(t, pathWithValue, anyAbsPath)

		pathMatchingPatternWithValue := NewPathMatchingPattern(NewPathPattern("/..."))
		assertTest(t, pathMatchingPatternWithValue, pathMatchingPatternWithValue)
		assertTest(t, pathMatchingPatternWithValue, NewPath("/"))
		assertTest(t, pathMatchingPatternWithValue, NewPath("/1"))
		assertTest(t, pathMatchingPatternWithValue, NewPath("/1/"))
		assertTestFalse(t, pathMatchingPatternWithValue, NewPath("./"))
		assertTestFalse(t, pathMatchingPatternWithValue, anyDirPath)
		assertTestFalse(t, pathMatchingPatternWithValue, anyPath)
		assertTestFalse(t, pathMatchingPatternWithValue, anyAbsPath)

		pathMatchingPatternWithNode := NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})
		assertTest(t, pathMatchingPatternWithNode, pathMatchingPatternWithNode)
		assertTestFalse(t, pathMatchingPatternWithNode, NewPath("/"))
		assertTestFalse(t, pathMatchingPatternWithNode, NewPath("/1"))
		assertTestFalse(t, pathMatchingPatternWithNode, NewPath("/1/"))
		assertTestFalse(t, pathMatchingPatternWithValue, NewPath("./"))
		assertTestFalse(t, pathMatchingPatternWithNode, anyPath)
		assertTestFalse(t, pathMatchingPatternWithNode, anyAbsPath)
		assertTestFalse(t, pathMatchingPatternWithNode, anyDirPath)
	})

}

func TestSymbolicURL(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyURL := &URL{}
		assertTest(t, anyURL, anyURL)
		assertTest(t, anyURL, &URL{})
		assertTestFalse(t, anyURL, &String{})
		assertTestFalse(t, anyURL, &Int{})

		urlWithValue := NewUrl("https://example.com/")
		assertTest(t, urlWithValue, urlWithValue)
		assertTest(t, urlWithValue, NewUrl("https://example.com/"))
		assertTestFalse(t, urlWithValue, NewUrl("https://example.com/1"))
		assertTestFalse(t, urlWithValue, NewUrl("https://localhost/"))
		assertTestFalse(t, urlWithValue, NewUrlMatchingPattern(NewUrlPattern("https://example.com/")))

		urlMatchingPatternWithValue := NewUrlMatchingPattern(NewUrlPattern("https://example.com/..."))
		assertTest(t, urlMatchingPatternWithValue, urlMatchingPatternWithValue)
		assertTest(t, urlMatchingPatternWithValue, NewUrl("https://example.com/"))
		assertTest(t, urlMatchingPatternWithValue, NewUrl("https://example.com/1"))
		assertTestFalse(t, urlMatchingPatternWithValue, NewUrl("https://localhost/"))
		assertTestFalse(t, urlMatchingPatternWithValue, anyURL)
	})

}

func TestSymbolicHost(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyHost := &Host{}
		assertTest(t, anyHost, anyHost)
		assertTest(t, anyHost, &Host{})
		assertTestFalse(t, anyHost, &String{})
		assertTestFalse(t, anyHost, &Int{})

		hostWithValue := NewHost("https://example.com")
		assertTest(t, hostWithValue, hostWithValue)
		assertTest(t, hostWithValue, NewHost("https://example.com"))
		assertTestFalse(t, hostWithValue, NewHost("https://localhost"))
		assertTestFalse(t, hostWithValue, NewHostMatchingPattern(NewHostPattern("https://example.com")))

		hostMatchingPatternWithValue := NewHostMatchingPattern(NewHostPattern("https://example.com"))
		assertTest(t, hostMatchingPatternWithValue, hostMatchingPatternWithValue)
		assertTest(t, hostMatchingPatternWithValue, NewHost("https://example.com"))
		assertTestFalse(t, hostMatchingPatternWithValue, NewHost("https://exemple.com"))
		assertTestFalse(t, hostMatchingPatternWithValue, NewHost("https://localhost/"))
		assertTestFalse(t, hostMatchingPatternWithValue, anyHost)
	})

}
func TestSymbolicIdentifier(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		specificIdent := &Identifier{name: "foo"}
		ident := &Identifier{}

		assertTest(t, specificIdent, specificIdent)
		assertTestFalse(t, specificIdent, ident)

		assertTest(t, ident, ident)
		assertTest(t, ident, specificIdent)
	})

}

func TestSymbolicOption(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		option := NewOption("a", NewInt(1))

		assertTest(t, option, NewOption("a", NewInt(1)))
		assertTestFalse(t, option, NewOption("a", NewInt(2)))
		assertTestFalse(t, option, NewOption("b", NewInt(1)))
		assertTestFalse(t, option, &String{})
		assertTestFalse(t, option, &Int{})

		assertTest(t, ANY_OPTION, ANY_OPTION)
		assertTest(t, ANY_OPTION, NewOption("a", NewInt(1)))
		assertTest(t, ANY_OPTION, NewOption("a", NewInt(2)))
		assertTest(t, ANY_OPTION, NewOption("b", NewInt(1)))
		assertTestFalse(t, ANY_OPTION, &String{})
		assertTestFalse(t, ANY_OPTION, &Int{})
	})

}

func TestSymbolicNode(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		specificNode := &AstNode{Node: &parse.ContinueStatement{}}
		node := &AstNode{}

		assertTest(t, specificNode, specificNode)
		assertTestFalse(t, specificNode, node)

		assertTest(t, node, node)
		assertTest(t, node, specificNode)
	})

}

func TestSymbolicError(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		err := &Error{data: &Int{}}

		assertTest(t, err, err)
		assertTest(t, err, &Error{data: &Int{}})
		assertTestFalse(t, err, &Int{})
	})

}

func TestSymbolicGoFunction(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		{
			anyFunc := &GoFunction{}
			assertTest(t, anyFunc, anyFunc)
			assertTest(t, anyFunc, &GoFunction{})
			assertTestFalse(t, anyFunc, &Int{})
		}

		{
			specificFunc := &GoFunction{fn: symbolicGoFn}
			assertTest(t, specificFunc, specificFunc)
			assertTestFalse(t, specificFunc, &GoFunction{})
			assertTestFalse(t, specificFunc, &Int{})
		}
	})

}

func TestSymbolicRuneSlice(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		slice := &RuneSlice{}

		assertTest(t, slice, slice)
		assertTest(t, slice, &RuneSlice{})
		assertTestFalse(t, slice, &String{})
		assertTestFalse(t, slice, &Int{})
	})

}

func TestSymbolicQuantityRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyQtyRange := &QuantityRange{element: ANY_SERIALIZABLE}

		assertTest(t, anyQtyRange, anyQtyRange)
		assertTest(t, anyQtyRange, &QuantityRange{element: ANY_SERIALIZABLE})
		assertTest(t, anyQtyRange, NewQuantityRange(ANY_BYTECOUNT))
		assertTestFalse(t, anyQtyRange, ANY_STR)
		assertTestFalse(t, anyQtyRange, ANY_INT)

		qtyRange := NewQuantityRange(ANY_BYTECOUNT)

		assertTest(t, qtyRange, qtyRange)
		assertTest(t, qtyRange, NewQuantityRange(ANY_BYTECOUNT))
		assertTestFalse(t, qtyRange, &QuantityRange{element: ANY_SERIALIZABLE})
		assertTestFalse(t, qtyRange, ANY_STR)
		assertTestFalse(t, qtyRange, ANY_INT)
	})

}

func TestSymbolicIntRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyIntRange := &IntRange{}
		intRange1_2 := NewIncludedEndIntRange(INT_1, INT_2)
		intRange1_2UnsupportedStep := NewIncludedEndIntRange(INT_1, INT_2)
		intRange1_2UnsupportedStep.isStepNotOne = true
		intRangeExclusiveEnd1_2 := NewExcludedEndIntRange(INT_1, INT_2)

		assertTest(t, anyIntRange, anyIntRange)
		assertTest(t, anyIntRange, &IntRange{})
		assertTest(t, anyIntRange, intRange1_2)
		assertTest(t, anyIntRange, intRange1_2UnsupportedStep)
		assertTest(t, anyIntRange, intRangeExclusiveEnd1_2)
		assertTestFalse(t, anyIntRange, ANY_STR)
		assertTestFalse(t, anyIntRange, ANY_INT)

		//check intRange1_2
		assertTest(t, intRange1_2, intRange1_2)
		assertTestFalse(t, intRange1_2, anyIntRange)
		assertTestFalse(t, intRange1_2, intRangeExclusiveEnd1_2)
		assertTestFalse(t, intRange1_2, intRange1_2UnsupportedStep)
		assertTestFalse(t, intRange1_2, ANY_INT)

		//check intRange1_2UnsupportedStep
		assertTest(t, intRange1_2UnsupportedStep, intRange1_2UnsupportedStep)
		assertTestFalse(t, intRange1_2UnsupportedStep, anyIntRange)
		assertTestFalse(t, intRange1_2UnsupportedStep, intRangeExclusiveEnd1_2)
		assertTestFalse(t, intRange1_2UnsupportedStep, intRange1_2)
		assertTestFalse(t, intRange1_2UnsupportedStep, ANY_INT)

		//check intRangeExclusiveEnd1_2
		assertTest(t, intRangeExclusiveEnd1_2, intRangeExclusiveEnd1_2)
		assertTestFalse(t, intRangeExclusiveEnd1_2, anyIntRange)
		assertTestFalse(t, intRangeExclusiveEnd1_2, intRange1_2)
		assertTestFalse(t, intRangeExclusiveEnd1_2, intRange1_2UnsupportedStep)
		assertTestFalse(t, intRangeExclusiveEnd1_2, ANY_INT)
	})

	t.Run("Contains()", func(t *testing.T) {
		anyIntRange := &IntRange{}
		assertMayContain(t, anyIntRange, INT_0)
		assertMayContain(t, anyIntRange, INT_1)

		intRange1_2 := NewIncludedEndIntRange(INT_1, INT_2)
		assertContains(t, intRange1_2, INT_1)
		assertContains(t, intRange1_2, INT_2)
		assertCannotPossiblyContain(t, intRange1_2, INT_0)
		assertCannotPossiblyContain(t, intRange1_2, INT_3)

		intRange1_2ExcludedEnd := NewExcludedEndIntRange(INT_1, INT_2)
		assertContains(t, intRange1_2ExcludedEnd, INT_1)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, INT_0)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, INT_2)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, INT_3)

		intRangeUnsupportedStep := NewIncludedEndIntRange(INT_1, INT_2)
		intRangeUnsupportedStep.isStepNotOne = true
		assertMayContain(t, intRangeUnsupportedStep, INT_1)
		assertMayContain(t, intRangeUnsupportedStep, INT_2)
		assertCannotPossiblyContain(t, intRange1_2, INT_0)
		assertCannotPossiblyContain(t, intRange1_2, INT_3)
	})

}

func TestSymbolicRuneRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		runeRange := &RuneRange{}

		assertTest(t, runeRange, runeRange)
		assertTest(t, runeRange, &RuneRange{})
		assertTestFalse(t, runeRange, &String{})
		assertTestFalse(t, runeRange, &Int{})
	})

}

func TestSymbolicAnyIterable(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyIterable := &AnyIterable{}

		assertTest(t, anyIterable, anyIterable)
		assertTest(t, anyIterable, NewList())
		assertTest(t, anyIterable, NewListOf(ANY_SERIALIZABLE))
		assertTest(t, anyIterable, NewListOf(&Int{}))
		assertTestFalse(t, anyIterable, &Int{})
	})

}

func symbolicGoFn(ctx *Context, list *List, args ...Value) *List {
	return list
}
