package symbolic

import (
	"fmt"
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicAnyPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := ANY_PATTERN

		assertTest(t, pattern, &RegexPattern{})
		assertTestFalse(t, pattern, ANY_INT)
	})

	t.Run("TestValue() should return true for any symbolic value", func(t *testing.T) {
		pattern := ANY_PATTERN

		assertTestValue(t, pattern, &RegexPattern{})
		assertTestValue(t, pattern, ANY_INT)
	})

}

func TestSymbolicTypePattern(t *testing.T) {
	patt := &TypePattern{val: ANY_INT}

	assertTestValue(t, patt, INT_1)
	assertTestValue(t, patt, NewMultivalue(INT_1, INT_2))
}

func TestSymbolicPathPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyPathPattern := &PathPattern{}

		assertTest(t, anyPathPattern, &PathPattern{})
		assertTestFalse(t, anyPathPattern, ANY_INT)
		assertTestFalse(t, anyPathPattern, ANY_PATTERN)

		pathPatternWithValue := NewPathPattern("/...")
		assertTest(t, pathPatternWithValue, pathPatternWithValue)
		assertTestFalse(t, pathPatternWithValue, anyPathPattern)
		assertTestFalse(t, pathPatternWithValue, ANY_INT)
		assertTestFalse(t, pathPatternWithValue, ANY_PATTERN)

		pathPatternWithNode := &PathPattern{node: &parse.PathPatternExpression{}}
		assertTest(t, pathPatternWithNode, pathPatternWithNode)
		assertTestFalse(t, pathPatternWithNode, &PathPattern{node: &parse.PathPatternExpression{}})
		assertTestFalse(t, pathPatternWithNode, anyPathPattern)
		assertTestFalse(t, pathPatternWithNode, pathPatternWithValue)
		assertTestFalse(t, pathPatternWithNode, ANY_INT)
		assertTestFalse(t, pathPatternWithNode, ANY_PATTERN)
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyPathPattern := ANY_PATH_PATTERN

		assertTestValue(t, anyPathPattern, &Path{})
		assertTestValue(t, anyPathPattern, NewPath("/"))
		assertTestValue(t, anyPathPattern, NewPath("./"))
		assertTestValue(t, anyPathPattern, NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}}))
		assertTestValueFalse(t, anyPathPattern, ANY_INT)
		assertTestValueFalse(t, anyPathPattern, ANY_PATH_PATTERN)

		//same tests but with result of .SymbolicValue()
		anyPathPattern_val := anyPathPattern.SymbolicValue()
		assertTest(t, anyPathPattern_val, &Path{})
		assertTest(t, anyPathPattern_val, NewPath("/"))
		assertTest(t, anyPathPattern_val, NewPath("./"))
		assertTest(t, anyPathPattern_val, NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}}))
		assertTestFalse(t, anyPathPattern_val, ANY_INT)
		assertTestFalse(t, anyPathPattern_val, ANY_PATH_PATTERN)

		pathPatternWithValue := NewPathPattern("/...")
		assertTestValue(t, pathPatternWithValue, NewPath("/"))
		assertTestValue(t, pathPatternWithValue, NewPath("/1"))
		assertTestValue(t, pathPatternWithValue, NewPath("/1/"))
		assertTestValueFalse(t, pathPatternWithValue, NewPath("./"))
		assertTestValueFalse(t, pathPatternWithValue, NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}}))
		assertTestValueFalse(t, pathPatternWithValue, &Path{})
		assertTestValueFalse(t, pathPatternWithValue, ANY_INT)
		assertTestValueFalse(t, pathPatternWithValue, ANY_PATH_PATTERN)

		//same tests but with result of .SymbolicValue()
		pathPatternWithValue_val := pathPatternWithValue.SymbolicValue()
		assertTest(t, pathPatternWithValue_val, NewPath("/"))
		assertTest(t, pathPatternWithValue_val, NewPath("/1"))
		assertTest(t, pathPatternWithValue_val, NewPath("/1/"))
		assertTestFalse(t, pathPatternWithValue_val, NewPath("./"))
		assertTestFalse(t, pathPatternWithValue_val, NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}}))
		assertTestFalse(t, pathPatternWithValue_val, &Path{})
		assertTestFalse(t, pathPatternWithValue_val, ANY_INT)
		assertTestFalse(t, pathPatternWithValue_val, ANY_PATH_PATTERN)

		pathPatternWithNode := &PathPattern{node: &parse.PathPatternExpression{}}
		assertTestValue(t, pathPatternWithNode, NewPathMatchingPattern(pathPatternWithNode))
		assertTestValueFalse(t, pathPatternWithNode, NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}}))
		assertTestValueFalse(t, pathPatternWithNode, NewPath("/"))
		assertTestValueFalse(t, pathPatternWithNode, NewPath("./"))
		assertTestValueFalse(t, pathPatternWithNode, &Path{})
		assertTestValueFalse(t, pathPatternWithNode, ANY_INT)
		assertTestValueFalse(t, pathPatternWithNode, ANY_PATH_PATTERN)

		//same tests but with result of .SymbolicValue()
		pathPatternWithNode_val := pathPatternWithNode.SymbolicValue()
		assertTest(t, pathPatternWithNode_val, NewPathMatchingPattern(pathPatternWithNode))
		assertTestFalse(t, pathPatternWithNode_val, NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}}))
		assertTestFalse(t, pathPatternWithNode_val, NewPath("/"))
		assertTestFalse(t, pathPatternWithNode_val, NewPath("./"))
		assertTestFalse(t, pathPatternWithNode_val, &Path{})
		assertTestFalse(t, pathPatternWithNode_val, ANY_INT)
		assertTestFalse(t, pathPatternWithNode_val, ANY_PATH_PATTERN)
	})

}

func TestSymbolicUrlPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyUrlPattern := &URLPattern{}

		assertTest(t, anyUrlPattern, &URLPattern{})
		assertTestFalse(t, anyUrlPattern, ANY_INT)
		assertTestFalse(t, anyUrlPattern, ANY_PATTERN)

		urlPatternWithValue := NewUrlPattern("https://example.com/...")
		assertTest(t, urlPatternWithValue, urlPatternWithValue)
		assertTestFalse(t, urlPatternWithValue, anyUrlPattern)
		assertTestFalse(t, urlPatternWithValue, ANY_INT)
		assertTestFalse(t, urlPatternWithValue, ANY_PATTERN)

		urlPatternWithNode := NewUrlPatternFromNode(&parse.PathPatternExpression{}, &parse.Chunk{})
		assertTest(t, urlPatternWithNode, urlPatternWithNode)
		assertTestFalse(t, urlPatternWithNode, NewUrlPatternFromNode(&parse.PathPatternExpression{}, &parse.Chunk{}))
		assertTestFalse(t, urlPatternWithNode, anyUrlPattern)
		assertTestFalse(t, urlPatternWithNode, urlPatternWithValue)
		assertTestFalse(t, urlPatternWithNode, ANY_INT)
		assertTestFalse(t, urlPatternWithNode, ANY_PATTERN)
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyUrlPattern := ANY_URL_PATTERN

		assertTestValue(t, anyUrlPattern, &URL{})
		assertTestValue(t, anyUrlPattern, NewUrl("https://example.com/"))
		assertTestValue(t, anyUrlPattern, NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}, &parse.Chunk{})))
		assertTestValueFalse(t, anyUrlPattern, ANY_INT)
		assertTestValueFalse(t, anyUrlPattern, ANY_URL_PATTERN)

		//same tests but with result of .SymbolicValue()
		anyUrlPattern_val := anyUrlPattern.SymbolicValue()
		assertTest(t, anyUrlPattern_val, &URL{})
		assertTest(t, anyUrlPattern_val, NewUrl("https://example.com/"))
		assertTest(t, anyUrlPattern_val, NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}, &parse.Chunk{})))
		assertTestFalse(t, anyUrlPattern_val, ANY_INT)
		assertTestFalse(t, anyUrlPattern_val, ANY_URL_PATTERN)

		urlPatternWithValue := NewUrlPattern("https://example.com/...")
		assertTestValue(t, urlPatternWithValue, NewUrl("https://example.com/"))
		assertTestValue(t, urlPatternWithValue, NewUrl("https://example.com/1"))
		assertTestValue(t, urlPatternWithValue, NewUrl("https://example.com/1/"))
		assertTestValueFalse(t, urlPatternWithValue, NewUrl("https://localhost/"))
		assertTestValueFalse(t, urlPatternWithValue, NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}, &parse.Chunk{})))
		assertTestValueFalse(t, urlPatternWithValue, &URL{})
		assertTestValueFalse(t, urlPatternWithValue, ANY_INT)
		assertTestValueFalse(t, urlPatternWithValue, ANY_URL_PATTERN)

		//same tests but with result of .SymbolicValue()
		urlPatternWithValue_val := urlPatternWithValue.SymbolicValue()
		assertTest(t, urlPatternWithValue_val, NewUrl("https://example.com/"))
		assertTest(t, urlPatternWithValue_val, NewUrl("https://example.com/1"))
		assertTest(t, urlPatternWithValue_val, NewUrl("https://example.com/1/"))
		assertTestFalse(t, urlPatternWithValue_val, NewUrl("https://localhost/"))
		assertTestFalse(t, urlPatternWithValue_val, NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}, &parse.Chunk{})))
		assertTestFalse(t, urlPatternWithValue_val, &URL{})
		assertTestFalse(t, urlPatternWithValue_val, ANY_INT)
		assertTestFalse(t, urlPatternWithValue_val, ANY_URL_PATTERN)

		urlPatternWithNode := NewUrlPatternFromNode(&parse.URLPatternLiteral{}, &parse.Chunk{}) //the node will never be a parse.URLPatternLiteral
		assertTestValue(t, urlPatternWithNode, NewUrlMatchingPattern(urlPatternWithNode))
		assertTestValueFalse(t, urlPatternWithNode, NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}, &parse.Chunk{})))
		assertTestValueFalse(t, urlPatternWithNode, NewUrl("https://example.com/"))
		assertTestValueFalse(t, urlPatternWithNode, &URL{})
		assertTestValueFalse(t, urlPatternWithNode, ANY_INT)
		assertTestValueFalse(t, urlPatternWithNode, ANY_URL_PATTERN)

		//same tests but with result of .SymbolicValue()
		urlPatternWithNode_val := urlPatternWithNode.SymbolicValue()
		assertTest(t, urlPatternWithNode_val, NewUrlMatchingPattern(urlPatternWithNode))
		assertTestFalse(t, urlPatternWithNode_val, NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}, &parse.Chunk{})))
		assertTestFalse(t, urlPatternWithNode_val, NewUrl("https://example.com/"))
		assertTestFalse(t, urlPatternWithNode_val, &URL{})
		assertTestFalse(t, urlPatternWithNode_val, ANY_INT)
		assertTestFalse(t, urlPatternWithNode_val, ANY_URL_PATTERN)
	})

}

func TestSymbolicHostPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyHostPattern := &HostPattern{}

		assertTest(t, anyHostPattern, &HostPattern{})
		assertTestFalse(t, anyHostPattern, ANY_INT)
		assertTestFalse(t, anyHostPattern, ANY_PATTERN)

		hostPatternWithValue := NewHostPattern("https://example.com")
		assertTest(t, hostPatternWithValue, hostPatternWithValue)
		assertTestFalse(t, hostPatternWithValue, anyHostPattern)
		assertTestFalse(t, hostPatternWithValue, ANY_INT)
		assertTestFalse(t, hostPatternWithValue, ANY_PATTERN)

		hostPatternWithNode := NewHostPatternFromNode(&parse.PathPatternExpression{}, &parse.Chunk{})
		assertTest(t, hostPatternWithNode, hostPatternWithNode)
		assertTestFalse(t, hostPatternWithNode, NewHostPatternFromNode(&parse.PathPatternExpression{}, &parse.Chunk{}))
		assertTestFalse(t, hostPatternWithNode, anyHostPattern)
		assertTestFalse(t, hostPatternWithNode, hostPatternWithValue)
		assertTestFalse(t, hostPatternWithNode, ANY_INT)
		assertTestFalse(t, hostPatternWithNode, ANY_PATTERN)
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyHostPattern := ANY_HOST_PATTERN

		assertTestValue(t, anyHostPattern, &Host{})
		assertTestValue(t, anyHostPattern, NewHost("https://example.com"))
		assertTestValue(t, anyHostPattern, NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}, &parse.Chunk{})))
		assertTestValueFalse(t, anyHostPattern, ANY_INT)
		assertTestValueFalse(t, anyHostPattern, ANY_HOST_PATTERN)

		//same tests but with result of .SymbolicValue()
		anyHostPattern_val := anyHostPattern.SymbolicValue()
		assertTest(t, anyHostPattern_val, &Host{})
		assertTest(t, anyHostPattern_val, NewHost("https://example.com"))
		assertTest(t, anyHostPattern_val, NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}, &parse.Chunk{})))
		assertTestFalse(t, anyHostPattern_val, ANY_INT)
		assertTestFalse(t, anyHostPattern_val, ANY_HOST_PATTERN)

		hostPatternWithValue := NewHostPattern("https://example.com")
		assertTestValue(t, hostPatternWithValue, NewHost("https://example.com"))
		assertTestValueFalse(t, hostPatternWithValue, NewHost("https://localhost"))
		assertTestValueFalse(t, hostPatternWithValue, NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}, &parse.Chunk{})))
		assertTestValueFalse(t, hostPatternWithValue, &Host{})
		assertTestValueFalse(t, hostPatternWithValue, ANY_INT)
		assertTestValueFalse(t, hostPatternWithValue, ANY_HOST_PATTERN)

		//same tests but with result of .SymbolicValue()
		hostPatternWithValue_val := hostPatternWithValue.SymbolicValue()
		assertTest(t, hostPatternWithValue_val, NewHost("https://example.com"))
		assertTestFalse(t, hostPatternWithValue_val, NewHost("https://localhost"))
		assertTestFalse(t, hostPatternWithValue_val, NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}, &parse.Chunk{})))
		assertTestFalse(t, hostPatternWithValue_val, &Host{})
		assertTestFalse(t, hostPatternWithValue_val, ANY_INT)
		assertTestFalse(t, hostPatternWithValue_val, ANY_HOST_PATTERN)

		hostPatternWithNode := NewHostPatternFromNode(&parse.HostPatternLiteral{}, &parse.Chunk{}) //the node will never be a parse.HostPatternLiteral
		assertTestValue(t, hostPatternWithNode, NewHostMatchingPattern(hostPatternWithNode))
		assertTestValueFalse(t, hostPatternWithNode, NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}, &parse.Chunk{})))
		assertTestValueFalse(t, hostPatternWithNode, NewHost("https://example.com"))
		assertTestValueFalse(t, hostPatternWithNode, &Host{})
		assertTestValueFalse(t, hostPatternWithNode, ANY_INT)
		assertTestValueFalse(t, hostPatternWithNode, ANY_HOST_PATTERN)

		//same tests but with result of .SymbolicValue()
		hostPatternWithNode_val := hostPatternWithNode.SymbolicValue()
		assertTest(t, hostPatternWithNode_val, NewHostMatchingPattern(hostPatternWithNode))
		assertTestFalse(t, hostPatternWithNode_val, NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}, &parse.Chunk{})))
		assertTestFalse(t, hostPatternWithNode_val, NewHost("https://example.com"))
		assertTestFalse(t, hostPatternWithNode_val, &Host{})
		assertTestFalse(t, hostPatternWithNode_val, ANY_INT)
		assertTestFalse(t, hostPatternWithNode_val, ANY_HOST_PATTERN)
	})

}

func TestSymbolicNamedSegmentPathPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		namedPathPattern := &NamedSegmentPathPattern{}
		specificNamedPathPattern := &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}

		assertTest(t, namedPathPattern, &NamedSegmentPathPattern{})
		assertTest(t, namedPathPattern, specificNamedPathPattern)
		assertTestFalse(t, namedPathPattern, &Path{})
		assertTestFalse(t, namedPathPattern, ANY_INT)
		assertTestFalse(t, namedPathPattern, ANY_PATTERN)

		assertTestFalse(t, specificNamedPathPattern, &NamedSegmentPathPattern{})
		assertTest(t, specificNamedPathPattern, specificNamedPathPattern)
		assertTestFalse(t, specificNamedPathPattern, &Path{})
		assertTestFalse(t, specificNamedPathPattern, ANY_INT)
		assertTestFalse(t, specificNamedPathPattern, ANY_PATTERN)
	})

	t.Run("TestValue() should return true for any symbolic path", func(t *testing.T) {
		namedPathPattern := &NamedSegmentPathPattern{}
		assertTestValue(t, namedPathPattern, &Path{})
		assertTestValueFalse(t, namedPathPattern, ANY_INT)
		assertTestValueFalse(t, namedPathPattern, &NamedSegmentPathPattern{})
		assertTestValueFalse(t, namedPathPattern, &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}})

		specificNamedPathPattern := &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}
		assertTestValue(t, specificNamedPathPattern, &Path{})
		assertTestValueFalse(t, specificNamedPathPattern, ANY_INT)
		assertTestValueFalse(t, specificNamedPathPattern, &NamedSegmentPathPattern{})
		assertTestValueFalse(t, specificNamedPathPattern, &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}})
	})

}

func TestSymbolicExactValuePattern(t *testing.T) {
	t.Run("Test()", func(t *testing.T) {
		pattern := &ExactValuePattern{value: ANY_INT}

		assertTest(t, pattern, pattern)
		assertTestFalse(t, pattern, ANY_INT)
		assertTestFalse(t, pattern, ANY_PATTERN)
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := &ExactValuePattern{value: ANY_INT}

		assertTestValue(t, pattern, ANY_INT)
		assertTestValueFalse(t, pattern, ANY_SERIALIZABLE)
		assertTestValueFalse(t, pattern, pattern)
	})

}

func TestSymbolicRegexPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &RegexPattern{}

		assertTest(t, pattern, &RegexPattern{})
		assertTestFalse(t, pattern, ANY_INT)
		assertTestFalse(t, pattern, ANY_PATTERN)

		patternWithRegex := NewRegexPattern("(a|b)")
		assertTest(t, patternWithRegex, patternWithRegex)
		assertTest(t, patternWithRegex, NewRegexPattern("(a|b)"))
		assertTest(t, patternWithRegex, NewRegexPattern("[ab]"))
		assertTestFalse(t, patternWithRegex, &RegexPattern{})
		assertTestFalse(t, patternWithRegex, ANY_INT)
		assertTestFalse(t, patternWithRegex, ANY_PATTERN)
	})

	t.Run("TestValue() & SymbolicValue()", func(t *testing.T) {
		pattern := &RegexPattern{}

		assertTestValue(t, pattern, ANY_STR)
		assertTestValueFalse(t, pattern, ANY_INT)
		assertTestValueFalse(t, pattern, &RegexPattern{})

		patternWithRegex := NewRegexPattern("(a|b)")
		val := patternWithRegex.SymbolicValue()

		assertTestValue(t, patternWithRegex, NewString("a"))
		assertTest(t, val, NewString("a"))

		assertTestValue(t, patternWithRegex, NewString("b"))
		assertTest(t, val, NewString("b"))

		assertTestValueFalse(t, patternWithRegex, NewString("c"))
		assertTestFalse(t, val, NewString("c"))

		assertTestValueFalse(t, patternWithRegex, &RegexPattern{})
		assertTestFalse(t, val, &RegexPattern{})

		assertTestValueFalse(t, patternWithRegex, ANY_INT)
		assertTestFalse(t, val, ANY_INT)

		assertTestValueFalse(t, patternWithRegex, ANY_PATTERN)
		assertTestFalse(t, val, ANY_PATTERN)
	})

}

func TestSymbolicObjectPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {

		t.Run("objects should never be matched", func(t *testing.T) {
			pattern := &ObjectPattern{entries: nil}

			assertTestFalse(t, pattern, &Object{entries: nil})
			assertTestFalse(t, pattern, &Object{entries: map[string]Serializable{}})
		})

		t.Run("if entries is nil any other object pattern of the same 'readonlyness' should be matched", func(t *testing.T) {
			pattern := &ObjectPattern{entries: nil}
			assertTest(t, pattern, &ObjectPattern{entries: nil})
			assertTest(t, pattern, &ObjectPattern{inexact: true, entries: map[string]Pattern{}})
			assertTest(t, pattern, &ObjectPattern{entries: map[string]Pattern{}})
			assertTest(t, pattern, &ObjectPattern{inexact: true, entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}})
			assertTest(t, pattern, &ObjectPattern{entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}})

			assertTestFalse(t, pattern, &ObjectPattern{readonly: true, entries: nil})
			assertTestFalse(t, pattern, &ObjectPattern{readonly: true, entries: map[string]Pattern{}, inexact: true})
			assertTestFalse(t, pattern, &ObjectPattern{readonly: true, entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}})
		})

		t.Run("exact empty object pattern should match any other exact object pattern of the same 'readonlyness'", func(t *testing.T) {
			pattern := &ObjectPattern{entries: map[string]Pattern{}}
			assertTest(t, pattern, &ObjectPattern{entries: map[string]Pattern{}})

			assertTestFalse(t, pattern, &ObjectPattern{readonly: true, entries: map[string]Pattern{}})
			assertTestFalse(t, pattern, &ObjectPattern{entries: nil})
			assertTestFalse(t, pattern, &ObjectPattern{entries: map[string]Pattern{}, inexact: true})
			assertTestFalse(t, pattern, &ObjectPattern{entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}})
		})

		t.Run("inexact empty object pattern should match any other non-any object pattern of the same 'readonlyness'", func(t *testing.T) {
			pattern := &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}
			assert.True(t, pattern.Test(&ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, RecTestCallState{}))
			assert.True(t, pattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}, RecTestCallState{}))

			assertTestFalse(t, pattern, &ObjectPattern{readonly: true, entries: nil})
			assertTestFalse(t, pattern, &ObjectPattern{readonly: true, entries: map[string]Pattern{}, inexact: true})
			assertTestFalse(t, pattern, &ObjectPattern{readonly: true, entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}})
		})

		t.Run("exact object pattern with a single prop should match any other exact object pattern "+
			"with the same single prop (same readonlyness)", func(t *testing.T) {
			singleIntPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}
			assert.True(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}, RecTestCallState{}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{},
			}, RecTestCallState{}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: true,
			}, RecTestCallState{}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries:  map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				readonly: true,
			}, RecTestCallState{}))

			singleAnyPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY}},
			}

			assert.True(t, singleAnyPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}, RecTestCallState{}))

			assertTestFalse(t, singleIntPropPattern, singleAnyPropPattern)
		})

		t.Run("inexact object pattern with a single prop should match any other exact object pattern", func(t *testing.T) {
			singleIntPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: true,
			}
			assert.True(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}, RecTestCallState{}))
			assert.True(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{
					"a": &TypePattern{val: ANY_INT},
					"b": &TypePattern{val: ANY_INT},
				},
			}, RecTestCallState{}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{},
			}, RecTestCallState{}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries:  map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				readonly: true,
			}, RecTestCallState{}))

			singleAnyPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY}},
				inexact: true,
			}

			assert.True(t, singleAnyPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}, RecTestCallState{}))

			assert.True(t, singleAnyPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{
					"a": &TypePattern{val: ANY_INT},
					"b": &TypePattern{val: ANY_INT},
				},
			}, RecTestCallState{}))

			assertTestFalse(t, singleIntPropPattern, singleAnyPropPattern)
		})

		t.Run("inexact object pattern with a dependency should match any other inexact object"+
			"with the same dependency and readonlyness", func(t *testing.T) {
			propAwithBdep := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}

			//matches itself
			assert.True(t, propAwithBdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{}))

			//same as propAwithBdep but with a pattern in the dependency
			assert.True(t, propAwithBdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b"},
						pattern:      ANY_OBJECT_PATTERN,
					},
				},
			}, RecTestCallState{}))

			assert.False(t, propAwithBdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				inexact: true,
			}, RecTestCallState{}))

			assert.False(t, propAwithBdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				readonly: true,
			}, RecTestCallState{}))

			//same
			propAwithBdepAndPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b"},
						pattern: NewInexactObjectPattern(map[string]Pattern{
							"b": &TypePattern{val: ANY_INT},
						}, nil),
					},
				},
			}
			//matches itself
			assert.True(t, propAwithBdepAndPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b"},
						pattern: NewInexactObjectPattern(map[string]Pattern{
							"b": &TypePattern{val: ANY_INT},
						}, nil),
					},
				},
			}, RecTestCallState{}))

			//pattern is missing
			assert.False(t, propAwithBdepAndPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b"},
					},
				},
			}, RecTestCallState{}))

			//same
			propAwithBCdep := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b", "c"},
					},
				},
			}
			//matches itself
			assert.True(t, propAwithBCdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b", "c"},
					},
				},
			}, RecTestCallState{}))

			//one of the required key is missing
			assert.False(t, propAwithBdepAndPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{}))
		})

		t.Run("an infinite recursion should cause raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			pattern := &ObjectPattern{}
			pattern.entries = map[string]Pattern{
				"self": pattern,
			}
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				pattern.Test(pattern, RecTestCallState{})
			})
		})
	})

	t.Run("TestValue() & SymbolicValue()", func(t *testing.T) {

		//SymbolicValue() only tests

		t.Run("empty exact", func(t *testing.T) {
			patt := NewExactObjectPattern(map[string]Pattern{}, nil)

			val := patt.SymbolicValue()
			assert.Equal(t, NewExactObject(map[string]Serializable{}, nil, map[string]Pattern{}), val)
		})

		t.Run("readonly empty exact", func(t *testing.T) {
			patt := NewExactObjectPattern(map[string]Pattern{}, nil)
			patt.readonly = true

			val := patt.SymbolicValue()
			expected := NewExactObject(map[string]Serializable{}, nil, map[string]Pattern{})
			expected.readonly = true
			assert.Equal(t, expected, val)
		})

		t.Run("empty inexact", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{}, nil)

			val := patt.SymbolicValue()
			assert.Equal(t, NewInexactObject(map[string]Serializable{}, nil, map[string]Pattern{}), val)
		})

		t.Run("readonly empty inexact", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{}, nil)
			patt.readonly = true

			val := patt.SymbolicValue()
			expected := NewInexactObject(map[string]Serializable{}, nil, map[string]Pattern{})
			expected.readonly = true
			assert.Equal(t, expected, val)
		})

		//TestValue() + SymbolicValue() tests

		t.Run("object pattern 'any'", func(t *testing.T) {
			patt := &ObjectPattern{entries: nil}

			//should never match another object pattern (any)
			if !assertTestValueFalse(t, patt, &ObjectPattern{entries: nil}) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&ObjectPattern{entries: nil}, RecTestCallState{})) {
				return
			}

			//should never match another object pattern (empty)
			if !assertTestValueFalse(t, patt, &ObjectPattern{entries: map[string]Pattern{}}) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&ObjectPattern{entries: map[string]Pattern{}}, RecTestCallState{})) {
				return
			}

			//should match object 'any'
			if !assertTestValue(t, patt, &Object{entries: nil}) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{entries: nil}, RecTestCallState{})) {
				return
			}

			//should not match readonly object 'any'
			if !assertTestValueFalse(t, patt, &Object{entries: nil, readonly: true}) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil, readonly: true}, RecTestCallState{})) {
				return
			}

			//should match empty objects (inexact)
			if !assertTestValue(t, patt, &Object{entries: map[string]Serializable{}}) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{entries: map[string]Serializable{}}, RecTestCallState{})) {
				return
			}

			//should match empty objects (exact)
			if !assertTestValue(t, patt, &Object{entries: map[string]Serializable{}, exact: true}) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{entries: map[string]Serializable{}, exact: true}, RecTestCallState{})) {
				return
			}
		})

		t.Run("empty exact object pattern", func(t *testing.T) {
			patt := &ObjectPattern{entries: map[string]Pattern{}}

			//should not match object 'any'
			if !assertTestValueFalse(t, patt, &Object{entries: nil}) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil}, RecTestCallState{})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with a property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
		})

		t.Run("empty inexact object pattern", func(t *testing.T) {
			patt := &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}

			//should not match object 'any'
			if !assertTestValueFalse(t, patt, &Object{entries: nil}) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil}, RecTestCallState{})) {
				return
			}

			//should match an empty inexact object
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}

			//should match an inexact object with a property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}

			//should match an empty exact object
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}

			//should match an exact object with a property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
		})

		t.Run("inexact object pattern with a single prop", func(t *testing.T) {
			patt := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: true,
			}

			//should not match object 'any'
			if !assertTestValueFalse(t, patt, &Object{entries: nil}) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil}, RecTestCallState{})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}

			//should match an inexact object with the same property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property but optional
			if !assert.False(t, patt.TestValue(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property but different type
			if !assert.False(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_BOOL},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_BOOL},
			}, RecTestCallState{})) {
				return
			}

			//should match an exact object with the same property + an additional property
			if !assert.True(t, patt.TestValue(&Object{
				exact: true,
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact: true,
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}

			//should match an inexact object with the same property + an additional property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}

			//should match an exact object with the same property
			if !assert.True(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
		})

		t.Run("exact object pattern with a single prop", func(t *testing.T) {
			patt := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: false,
			}

			//should not match object 'any'
			if !assertTestValueFalse(t, patt, &Object{entries: nil}) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil}, RecTestCallState{})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property but optional
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property if super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property if different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
			}, RecTestCallState{})) {
				return
			}

			//should match an inexact object with the same property + an additional property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}

			//should match an exact object with the same property
			if !assert.True(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
		})

		t.Run("inexact object pattern with a single prop + a dependency with a required key", func(t *testing.T) {
			patt := &ObjectPattern{
				inexact: true,
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}

			//should not match object 'any'
			if !assertTestValueFalse(t, patt, &Object{entries: nil}) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil}, RecTestCallState{})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}

			//should not match an empty inexact object with the same dependency
			if !assert.False(t, patt.TestValue(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}

			//TODO: empty exact object with the same dependency
			//does that even make sense ? should some dependencies be forbidden for exact objects & exact object patterns ?

			//should not match an inexact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object having the same property (but optional) and dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object having the same property but optional and the same dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property if super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property if different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property + validating the dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property + an additional property unrelated to the dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
		})

		t.Run("inexact object pattern with a single optional prop + a dependency with a required key", func(t *testing.T) {
			patt := &ObjectPattern{
				inexact:         true,
				entries:         map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}

			//should not match object 'any'
			if !assertTestValueFalse(t, patt, &Object{entries: nil}) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil}, RecTestCallState{})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			}, RecTestCallState{})) {
				return
			}

			//should match an empty inexact object with the same dependency
			if !assert.True(t, patt.TestValue(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property but not validating the dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}

			//should match an inexact object having the same property (but optional) and dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property but optional
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			}, RecTestCallState{})) {
				return
			}

			//should match an inexact object having the same property but optional and the same dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property if different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property + validating the dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an inexact object with the same property + an additional property unrelated to the dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			}, RecTestCallState{})) {
				return
			}
		})

		t.Run("an infinite recursion should cause raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			//TODO
		})
	})

	t.Run("MigrationInitialValue()", func(t *testing.T) {
		t.Run("empty exact", func(t *testing.T) {
			patt := NewExactObjectPattern(map[string]Pattern{}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewExactObject(map[string]Serializable{}, nil, map[string]Pattern{}), initialValue)
		})

		t.Run("empty inexact", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewInexactObject(map[string]Serializable{}, nil, map[string]Pattern{}), initialValue)
		})

		t.Run("property pattern with initial value", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{
				"inner": NewInexactObjectPattern(map[string]Pattern{}, nil),
			}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewInexactObject(map[string]Serializable{
				"inner": NewInexactObject(map[string]Serializable{}, nil, map[string]Pattern{}),
			}, nil, map[string]Pattern{
				"inner": NewInexactObjectPattern(map[string]Pattern{}, nil),
			}), initialValue)
		})

		t.Run("property pattern without initial value", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{
				"inner": ANY_SERIALIZABLE_PATTERN,
			}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if assert.False(t, ok) {
				return
			}
			assert.Nil(t, initialValue)
		})
	})

	t.Run("ToReadonlyPattern()", func(t *testing.T) {

		t.Run("already readonly", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{}, nil)
			patt.readonly = true

			result, err := patt.ToReadonlyPattern()
			if !assert.NoError(t, err) {
				return
			}
			assert.Same(t, patt, result)
		})

		t.Run("empty", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{}, nil)

			result, err := patt.ToReadonlyPattern()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewInexactObjectPattern(map[string]Pattern{}, nil)
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("immutable property", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{
				"x": ANY_RECORD_PATTERN,
			}, nil)

			result, err := patt.ToReadonlyPattern()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewInexactObjectPattern(map[string]Pattern{
				"x": ANY_RECORD_PATTERN,
			}, nil)
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("an error should be returned if a property pattern is not convertible to readonly", func(t *testing.T) {
			patt := NewInexactObjectPattern(map[string]Pattern{
				"x": ANY_SERIALIZABLE_PATTERN,
			}, nil)

			result, err := patt.ToReadonlyPattern()
			if !assert.ErrorIs(t, err, ErrNotConvertibleToReadonly) {
				return
			}

			assert.Nil(t, result)
		})
	})
}

func TestSymbolicRecordPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			pattern *RecordPattern
			value   Value
			ok      bool
		}{
			//symbolic object
			{&RecordPattern{entries: nil}, &Object{entries: nil}, false},
			{&RecordPattern{entries: nil}, &Object{entries: map[string]Serializable{}}, false},

			//symbolic record pattern
			{&RecordPattern{entries: nil}, &RecordPattern{entries: nil}, true},
			{&RecordPattern{entries: map[string]Pattern{}}, &RecordPattern{entries: nil}, false},
			{&RecordPattern{entries: nil}, &RecordPattern{entries: map[string]Pattern{}, inexact: true}, true},
			{&RecordPattern{entries: nil}, &RecordPattern{entries: map[string]Pattern{}}, true},

			{&RecordPattern{entries: map[string]Pattern{}}, &RecordPattern{entries: map[string]Pattern{}}, true},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				&RecordPattern{
					entries: map[string]Pattern{},
				},
				false,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{},
				},
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				false,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": ANY_PATTERN},
				},
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				&RecordPattern{
					entries: map[string]Pattern{"a": ANY_PATTERN},
				},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

	t.Run("TestValue() & SymbolicValue()", func(t *testing.T) {
		t.Run("empty exact", func(t *testing.T) {
			patt := NewExactRecordPattern(map[string]Pattern{}, nil)

			val := patt.SymbolicValue()
			assert.Equal(t, NewExactRecord(map[string]Serializable{}, nil), val)
		})

		t.Run("empty inexact", func(t *testing.T) {
			patt := NewInexactRecordPattern(map[string]Pattern{}, nil)

			val := patt.SymbolicValue()
			assert.Equal(t, NewInexactRecord(map[string]Serializable{}, nil), val)
		})

		cases := []struct {
			pattern     *RecordPattern
			testedValue Value
			ok          bool
		}{
			{&RecordPattern{entries: nil}, &RecordPattern{entries: nil}, false},
			{&RecordPattern{entries: nil}, &RecordPattern{entries: map[string]Pattern{}}, false},

			//symbolic object
			{&RecordPattern{entries: nil}, &Record{entries: nil}, true},
			{&RecordPattern{entries: map[string]Pattern{}}, &Record{entries: nil}, false},
			{&RecordPattern{entries: nil}, &Record{entries: map[string]Serializable{}}, true},

			{
				&RecordPattern{entries: map[string]Pattern{}},
				&Record{entries: map[string]Serializable{}},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				&Record{
					entries: map[string]Serializable{},
				},
				false,
			},
			{
				&RecordPattern{
					entries:         map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
					optionalEntries: map[string]struct{}{"a": {}},
				},
				&Record{
					entries: map[string]Serializable{},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{},
				},
				&Record{
					entries: map[string]Serializable{"a": ANY_INT},
				},
				false,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				&Record{
					entries: map[string]Serializable{"a": ANY_INT},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": ANY_SERIALIZABLE_PATTERN},
				},
				&Record{
					entries: map[string]Serializable{"a": ANY_INT},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				},
				&Record{
					entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
				},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+Stringify(testCase.pattern)+"_"+Stringify(testCase.testedValue), func(t *testing.T) {
				if !assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue, RecTestCallState{})) {
					return
				}
				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.testedValue, RecTestCallState{}))
			})
		}
	})

	t.Run("MigrationInitialValue()", func(t *testing.T) {
		t.Run("empty exact", func(t *testing.T) {
			patt := NewExactRecordPattern(map[string]Pattern{}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewExactRecord(map[string]Serializable{}, nil), initialValue)
		})

		t.Run("empty inexact", func(t *testing.T) {
			patt := NewInexactRecordPattern(map[string]Pattern{}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewInexactRecord(map[string]Serializable{}, nil), initialValue)
		})

		t.Run("property pattern with initial value", func(t *testing.T) {
			patt := NewInexactRecordPattern(map[string]Pattern{
				"inner": NewInexactRecordPattern(map[string]Pattern{}, nil),
			}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewInexactRecord(map[string]Serializable{
				"inner": NewInexactRecord(map[string]Serializable{}, nil),
			}, nil), initialValue)
		})

		t.Run("property pattern without initial value", func(t *testing.T) {
			patt := NewInexactRecordPattern(map[string]Pattern{
				"inner": ANY_SERIALIZABLE_PATTERN,
			}, nil)

			initialValue, ok := patt.MigrationInitialValue()
			if assert.False(t, ok) {
				return
			}
			assert.Nil(t, initialValue)
		})
	})
}

func TestSymbolicListPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			pattern *ListPattern
			value   Value
			ok      bool
		}{

			{
				&ListPattern{generalElement: ANY_PATTERN},
				&List{generalElement: ANY_SERIALIZABLE},
				false,
			},

			{
				&ListPattern{generalElement: ANY_PATTERN},
				&ListPattern{generalElement: ANY_PATTERN},
				true,
			},
			{
				&ListPattern{generalElement: ANY_PATTERN, readonly: true},
				&ListPattern{generalElement: ANY_PATTERN},
				false,
			},
			{
				&ListPattern{generalElement: ANY_PATTERN},
				&ListPattern{generalElement: ANY_PATTERN, readonly: true},
				false,
			},
			{
				&ListPattern{generalElement: ANY_PATTERN},
				&ListPattern{generalElement: &TypePattern{val: ANY_INT}},
				true,
			},
			{
				&ListPattern{generalElement: ANY_PATTERN},
				&ListPattern{elements: []Pattern{}},
				true,
			},
			{
				&ListPattern{generalElement: ANY_PATTERN, readonly: true},
				&ListPattern{elements: []Pattern{}},
				false,
			},
			{
				&ListPattern{generalElement: ANY_PATTERN},
				&ListPattern{elements: []Pattern{}, readonly: true},
				false,
			},
			{
				&ListPattern{elements: []Pattern{}},
				&ListPattern{generalElement: ANY_PATTERN},
				false,
			},
			{
				&ListPattern{elements: []Pattern{}},
				&ListPattern{elements: []Pattern{ANY_PATTERN}},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

	t.Run("TestValue() & SymbolicValue()", func(t *testing.T) {
		t.Run("readonly", func(t *testing.T) {
			patt := NewListPatternOf(ANY_SERIALIZABLE_PATTERN)
			patt.readonly = true

			val := patt.SymbolicValue()
			expected := NewListOf(ANY_SERIALIZABLE)
			expected.readonly = true
			assert.Equal(t, expected, val)
		})

		t.Run("multivalue as general element", func(t *testing.T) {
			patt := NewListPatternOf(&TypePattern{val: NewMultivalue(ANY_INT, ANY_STR)})

			val := patt.SymbolicValue()
			serializableMv := AsSerializableChecked(NewMultivalue(ANY_INT, ANY_STR))
			expected := NewListOf(serializableMv)
			assert.Equal(t, expected, val)
		})

		t.Run("multivalue as element", func(t *testing.T) {
			patt := NewListPattern([]Pattern{&TypePattern{val: NewMultivalue(ANY_INT, ANY_STR)}})

			val := patt.SymbolicValue()
			serializableMv := AsSerializableChecked(NewMultivalue(ANY_INT, ANY_STR))
			expected := NewList(serializableMv)
			assert.Equal(t, expected, val)
		})

		cases := []struct {
			pattern     *ListPattern
			testedValue Value
			ok          bool
		}{
			//[]any
			{
				&ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&List{elements: []Serializable{}}, //empty list
				true,
			},
			{
				&ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN, readonly: true},
				&List{elements: []Serializable{}}, //empty list
				false,
			},
			{
				&ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&List{elements: []Serializable{}, readonly: true}, //empty list
				false,
			},
			{
				&ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&List{generalElement: ANY_INT}, //[]int
				true,
			},
			{
				&ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&List{generalElement: ANY_SERIALIZABLE}, //[]any
				true,
			},
			{
				&ListPattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&List{elements: []Serializable{ANY_SERIALIZABLE}}, //[any]
				true,
			},

			//[any]
			{
				&ListPattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}},
				&List{generalElement: ANY_SERIALIZABLE}, //[any]
				false,
			},
			{
				&ListPattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}},
				&List{elements: []Serializable{ANY_INT}}, //[string]
				true,
			},
			{
				&ListPattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}, readonly: true},
				&List{elements: []Serializable{ANY_INT}}, //[string]
				false,
			},
			{
				&ListPattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}},
				&List{elements: []Serializable{ANY_INT}, readonly: true}, //[string]
				false,
			},
			{
				&ListPattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}},
				&List{elements: []Serializable{}}, //empty list
				false,
			},

			//[]int
			{
				&ListPattern{generalElement: &TypePattern{val: ANY_INT}},
				&List{elements: []Serializable{}}, //empty list
				true,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: ANY_INT}},
				&List{generalElement: ANY_INT}, //[]int
				true,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: ANY_INT}},
				&List{elements: []Serializable{ANY_INT}}, //[int]
				true,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: ANY_INT}},
				&List{generalElement: ANY_SERIALIZABLE}, //[]any
				false,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: ANY_INT}},
				&List{generalElement: ANY_STR}, //[]string
				false,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: ANY_INT}},
				&List{elements: []Serializable{ANY_INT, ANY_STR}}, //[int, string]
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+Stringify(testCase.pattern)+"_"+Stringify(testCase.testedValue), func(t *testing.T) {
				if !assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue, RecTestCallState{})) {
					return
				}
				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.testedValue, RecTestCallState{}))

				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue, RecTestCallState{}))
			})
		}
	})

	t.Run("MigrationInitialValue()", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			patt := NewListPattern([]Pattern{})

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, EMPTY_LIST, initialValue)
		})

		t.Run("general element pattern with initial value", func(t *testing.T) {
			patt := NewListPatternOf(NewListPattern([]Pattern{}))

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewListOf(NewList()), initialValue)
		})

		t.Run("general element pattern without initial value", func(t *testing.T) {
			patt := NewListPatternOf(ANY_SERIALIZABLE_PATTERN)

			initialValue, ok := patt.MigrationInitialValue()
			if assert.False(t, ok) {
				return
			}
			assert.Nil(t, initialValue)
		})

		t.Run("single element pattern with initial value", func(t *testing.T) {
			patt := NewListPattern([]Pattern{NewListPattern([]Pattern{})})

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewList(NewList()), initialValue)
		})

		t.Run("single element pattern without initial value", func(t *testing.T) {
			patt := NewListPattern([]Pattern{ANY_SERIALIZABLE_PATTERN})

			initialValue, ok := patt.MigrationInitialValue()
			if assert.False(t, ok) {
				return
			}
			assert.Nil(t, initialValue)
		})
	})

	t.Run("ToReadonlyPattern()", func(t *testing.T) {

		t.Run("already readonly", func(t *testing.T) {
			patt := NewListPattern(nil)
			patt.readonly = true

			result, err := patt.ToReadonlyPattern()
			if !assert.NoError(t, err) {
				return
			}
			assert.Same(t, patt, result)
		})

		t.Run("empty", func(t *testing.T) {
			patt := NewListPattern(nil)

			result, err := patt.ToReadonlyPattern()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewListPattern(nil)
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("immutable general element", func(t *testing.T) {
			patt := NewListPatternOf(ANY_RECORD_PATTERN)

			result, err := patt.ToReadonlyPattern()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewListPatternOf(ANY_RECORD_PATTERN)
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("an error should be returned if the general element pattern is not convertible to readonly", func(t *testing.T) {
			patt := NewListPatternOf(ANY_PATTERN)

			result, err := patt.ToReadonlyPattern()
			if !assert.ErrorIs(t, err, ErrNotConvertibleToReadonly) {
				return
			}

			assert.Nil(t, result)
		})

		t.Run("immutable element", func(t *testing.T) {
			patt := NewListPattern([]Pattern{ANY_RECORD_PATTERN})

			result, err := patt.ToReadonlyPattern()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewListPattern([]Pattern{ANY_RECORD_PATTERN})
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("an error should be returned if an element pattern is not convertible to readonly", func(t *testing.T) {
			patt := NewListPattern([]Pattern{ANY_PATTERN})

			result, err := patt.ToReadonlyPattern()
			if !assert.ErrorIs(t, err, ErrNotConvertibleToReadonly) {
				return
			}

			assert.Nil(t, result)
		})
	})
}

func TestSymbolicTuplePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			pattern *TuplePattern
			value   Serializable
			ok      bool
		}{

			{
				&TuplePattern{generalElement: ANY_PATTERN},
				&Tuple{generalElement: ANY_SERIALIZABLE},
				false,
			},

			{
				&TuplePattern{generalElement: ANY_PATTERN},
				&TuplePattern{generalElement: ANY_PATTERN},
				true,
			},
			{
				&TuplePattern{generalElement: ANY_PATTERN},
				&TuplePattern{generalElement: &TypePattern{val: ANY_INT}},
				true,
			},
			{
				&TuplePattern{generalElement: ANY_PATTERN},
				&TuplePattern{elements: []Pattern{}},
				true,
			},
			{
				&TuplePattern{elements: []Pattern{}},
				&TuplePattern{generalElement: ANY_PATTERN},
				false,
			},
			{
				&TuplePattern{elements: []Pattern{}},
				&TuplePattern{elements: []Pattern{ANY_PATTERN}},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

	t.Run("TestValue() & SymbolicValue()", func(t *testing.T) {
		t.Run("multivalue as general element", func(t *testing.T) {
			patt := NewTuplePatternOf(&TypePattern{val: NewMultivalue(ANY_INT, ANY_STR)})

			val := patt.SymbolicValue()
			serializableMv := AsSerializableChecked(NewMultivalue(ANY_INT, ANY_STR))
			expected := NewTupleOf(serializableMv)
			assert.Equal(t, expected, val)
		})

		t.Run("multivalue as element", func(t *testing.T) {
			patt := NewTuplePattern([]Pattern{&TypePattern{val: NewMultivalue(ANY_INT, ANY_STR)}})

			val := patt.SymbolicValue()
			serializableMv := AsSerializableChecked(NewMultivalue(ANY_INT, ANY_STR))
			expected := NewTuple(serializableMv)
			assert.Equal(t, expected, val)
		})

		cases := []struct {
			pattern     *TuplePattern
			testedValue Serializable
			ok          bool
		}{
			//[]any
			{
				&TuplePattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&Tuple{elements: []Serializable{}}, //empty tuple
				true,
			},
			{
				&TuplePattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&Tuple{generalElement: ANY_INT}, //[]int
				true,
			},
			{
				&TuplePattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&Tuple{generalElement: ANY_SERIALIZABLE}, //[]any
				true,
			},
			{
				&TuplePattern{generalElement: ANY_SERIALIZABLE_PATTERN},
				&Tuple{elements: []Serializable{ANY_SERIALIZABLE}}, //[any]
				true,
			},

			//[any]
			{
				&TuplePattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}},
				&Tuple{generalElement: ANY_SERIALIZABLE}, //[any]
				false,
			},
			{
				&TuplePattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}},
				&Tuple{elements: []Serializable{ANY_INT}}, //[string]
				true,
			},
			{
				&TuplePattern{elements: []Pattern{ANY_SERIALIZABLE_PATTERN}},
				&Tuple{elements: []Serializable{}}, //empty tuple
				false,
			},

			//[]int
			{
				&TuplePattern{generalElement: &TypePattern{val: ANY_INT}},
				&Tuple{elements: []Serializable{}}, //empty tuple
				true,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: ANY_INT}},
				&Tuple{generalElement: ANY_INT}, //[]int
				true,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: ANY_INT}},
				&Tuple{elements: []Serializable{ANY_INT}}, //[int]
				true,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: ANY_INT}},
				&Tuple{generalElement: ANY_SERIALIZABLE}, //[]any
				false,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: ANY_INT}},
				&Tuple{generalElement: ANY_STR}, //[]string
				false,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: ANY_INT}},
				&Tuple{elements: []Serializable{ANY_INT, ANY_STR}}, //[int, string]
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+Stringify(testCase.pattern)+"_"+Stringify(testCase.testedValue), func(t *testing.T) {
				if !assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue, RecTestCallState{})) {
					return
				}
				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.testedValue, RecTestCallState{}))
			})
		}
	})

	t.Run("MigrationInitialValue()", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			patt := NewTuplePattern([]Pattern{})

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, EMPTY_TUPLE, initialValue)
		})

		t.Run("general element pattern with initial value", func(t *testing.T) {
			patt := NewTuplePatternOf(NewTuplePattern([]Pattern{}))

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewTupleOf(NewTuple()), initialValue)
		})

		t.Run("general element pattern without initial value", func(t *testing.T) {
			patt := NewTuplePatternOf(ANY_SERIALIZABLE_PATTERN)

			initialValue, ok := patt.MigrationInitialValue()
			if assert.False(t, ok) {
				return
			}
			assert.Nil(t, initialValue)
		})

		t.Run("single element pattern with initial value", func(t *testing.T) {
			patt := NewTuplePattern([]Pattern{NewTuplePattern([]Pattern{})})

			initialValue, ok := patt.MigrationInitialValue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, NewTuple(NewTuple()), initialValue)
		})

		t.Run("single element pattern without initial value", func(t *testing.T) {
			patt := NewTuplePattern([]Pattern{ANY_SERIALIZABLE_PATTERN})

			initialValue, ok := patt.MigrationInitialValue()
			if assert.False(t, ok) {
				return
			}
			assert.Nil(t, initialValue)
		})
	})
}

func TestSymbolicUnionPattern(t *testing.T) {
	INT_PATTERN := ANY_INT.Static()
	FLOAT_PATTERN := ANY_FLOAT.Static()
	STR_PATTERN := ANY_STR.Static()
	BOOL_PATTERN := ANY_BOOL.Static()

	newUnionPattern := func(cases ...Pattern) *UnionPattern {
		return utils.Must(NewUnionPattern(cases, false))
	}
	newDisjointUnionPattern := func(cases ...Pattern) *UnionPattern {
		return utils.Must(NewUnionPattern(cases, true))
	}

	t.Run("NewUnionPattern", func(t *testing.T) {

		patt := newUnionPattern(INT_PATTERN, STR_PATTERN)
		assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN}, patt.Cases())

		t.Run("flattening", func(t *testing.T) {
			patt = newUnionPattern(INT_PATTERN, newUnionPattern(STR_PATTERN, BOOL_PATTERN))
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN}, patt.Cases())

			patt = newUnionPattern(INT_PATTERN, newDisjointUnionPattern(STR_PATTERN, BOOL_PATTERN))
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				newDisjointUnionPattern(STR_PATTERN, BOOL_PATTERN),
			}, patt.Cases())

			patt = newUnionPattern(
				INT_PATTERN,
				newUnionPattern(
					STR_PATTERN,
					newUnionPattern(BOOL_PATTERN, FLOAT_PATTERN),
				),
			)
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN, FLOAT_PATTERN}, patt.Cases())

			patt = newUnionPattern(
				INT_PATTERN,
				newUnionPattern(
					STR_PATTERN,
					newDisjointUnionPattern(BOOL_PATTERN, FLOAT_PATTERN),
				),
			)
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				STR_PATTERN,
				newDisjointUnionPattern(BOOL_PATTERN, FLOAT_PATTERN),
			}, patt.Cases())
		})

		t.Run("flattening disjoint cases", func(t *testing.T) {
			patt = newDisjointUnionPattern(
				INT_PATTERN,
				newDisjointUnionPattern(STR_PATTERN, BOOL_PATTERN),
			)
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN}, patt.Cases())

			patt = newDisjointUnionPattern(
				INT_PATTERN,
				newUnionPattern(STR_PATTERN, BOOL_PATTERN),
			)
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				newUnionPattern(STR_PATTERN, BOOL_PATTERN),
			}, patt.Cases())

			patt = newDisjointUnionPattern(
				INT_PATTERN,
				newDisjointUnionPattern(
					STR_PATTERN,
					newDisjointUnionPattern(BOOL_PATTERN, FLOAT_PATTERN),
				),
			)
			assert.Equal(t, []Pattern{INT_PATTERN, STR_PATTERN, BOOL_PATTERN, FLOAT_PATTERN}, patt.Cases())

			patt = newDisjointUnionPattern(
				INT_PATTERN,
				newDisjointUnionPattern(
					STR_PATTERN,
					newUnionPattern(BOOL_PATTERN, FLOAT_PATTERN),
				),
			)
			assert.Equal(t, []Pattern{
				INT_PATTERN,
				STR_PATTERN,
				newUnionPattern(BOOL_PATTERN, FLOAT_PATTERN),
			}, patt.Cases())
		})
	})

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			pattern *UnionPattern
			value   Value
			ok      bool
		}{
			{
				&UnionPattern{
					cases: []Pattern{
						ANY_PATTERN,
						ANY_PATTERN,
					},
				},
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						ANY_PATTERN,
						ANY_PATTERN,
					},
				},
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
						&TypePattern{val: ANY_BOOL},
					},
				},
				false,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				&UnionPattern{
					cases: []Pattern{
						ANY_PATTERN,
						ANY_PATTERN,
					},
				},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

	t.Run("TestValue()", func(t *testing.T) {
		cases := []struct {
			pattern *UnionPattern
			value   Value
			ok      bool
		}{
			{
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				ANY_INT,
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				ANY_STR,
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				NewMultivalue(ANY_INT, ANY_STR),
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				NewMultivalue(ANY_STR, ANY_INT),
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				NewMultivalue(ANY_STR, NewInt(1)),
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						&TypePattern{val: ANY_INT},
						&TypePattern{val: ANY_STR},
					},
				},
				ANY_SERIALIZABLE,
				false,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": utils.Must(NewExactValuePattern(NewInt(1)))}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": utils.Must(NewExactValuePattern(NewInt(2)))}, nil),
					},
				},
				NewExactObject(map[string]Serializable{"a": NewInt(1)}, nil, nil),
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": utils.Must(NewExactValuePattern(NewInt(1)))}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": utils.Must(NewExactValuePattern(NewInt(2)))}, nil),
					},
				},
				NewExactObject(map[string]Serializable{"b": NewInt(2)}, nil, nil),
				true,
			},
			{
				&UnionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": utils.Must(NewExactValuePattern(NewInt(1)))}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": utils.Must(NewExactValuePattern(NewInt(2)))}, nil),
					},
				},
				NewExactObject(map[string]Serializable{"a": NewInt(1), "b": NewInt(2)}, nil, nil),
				true,
			},
		}

		for _, testCase := range cases {
			s := " should match "
			if !testCase.ok {
				s = " should not match"
			}
			t.Run(t.Name()+"_"+fmt.Sprint(Stringify(testCase.pattern), s, Stringify(testCase.value)), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value, RecTestCallState{}))

				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

}

func TestSymbolicIntersectionPattern(t *testing.T) {
	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			name    string
			pattern *IntersectionPattern
			value   Value
			ok      bool
		}{
			{
				"an intersection pattern should include itself",
				&IntersectionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
					},
				},
				&IntersectionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
					},
				},
				true,
			},
			{
				"a narrow intersection should not include a less narrow intersection",
				&IntersectionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"c": ANY_INT.Static()}, nil),
					},
				},
				&IntersectionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
					},
				},
				false,
			},
			{
				"an intersection should include a narrower intersection",
				&IntersectionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
					},
				},
				&IntersectionPattern{
					cases: []Pattern{
						NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
						NewInexactObjectPattern(map[string]Pattern{"c": ANY_INT.Static()}, nil),
					},
				},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

	t.Run("TestValue()", func(t *testing.T) {
		cases := []struct {
			name    string
			pattern *IntersectionPattern
			value   Value
			ok      bool
		}{
			{
				"value matching all cases should match the pattern",
				utils.Must(NewIntersectionPattern([]Pattern{
					NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
					NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
				})),
				NewInexactObject(map[string]Serializable{"a": NewInt(1), "b": NewInt(2)}, nil, nil),
				true,
			},
			// {
			// 	"multivalue matching all cases should match the pattern",
			// 	utils.Must(NewIntersectionPattern([]Pattern{
			// 		NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
			// 		NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
			// 	})),
			// 	NewMultivalue(
			// 		NewInexactObject(map[string]Serializable{"a": NewInt(1), "b": NewInt(2)}, nil, nil),
			// 		NewInexactObject(map[string]Serializable{"a": NewInt(2), "b": NewInt(3)}, nil, nil),
			// 	),
			// 	true,
			// },
			{
				"value matching the first case only should not match the pattern",
				utils.Must(NewIntersectionPattern([]Pattern{
					NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
					NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
				})),
				NewInexactObject(map[string]Serializable{"a": NewInt(1)}, nil, nil),
				false,
			},
			{
				"value matching the second case only should not match the pattern",
				utils.Must(NewIntersectionPattern([]Pattern{
					NewInexactObjectPattern(map[string]Pattern{"a": ANY_INT.Static()}, nil),
					NewInexactObjectPattern(map[string]Pattern{"b": ANY_INT.Static()}, nil),
				})),
				NewInexactObject(map[string]Serializable{"b": NewInt(1)}, nil, nil),
				false,
			},
			//TODO: add more tests
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value, RecTestCallState{}))

				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

}

func TestSymbolicOptionPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := NewOptionPattern("a", ANY_STR_PATTERN)

		assertTest(t, pattern, NewOptionPattern("a", ANY_STR_PATTERN))
		assertTestFalse(t, pattern, NewOptionPattern("b", ANY_PATTERN))
		assertTestFalse(t, pattern, ANY_OPTION_PATTERN)
		assertTestFalse(t, pattern, ANY_INT)
		assertTestFalse(t, pattern, NewOption("x", EMPTY_STRING))

		anyOptionPattern := ANY_OPTION_PATTERN
		assertTest(t, anyOptionPattern, NewOptionPattern("a", ANY_STR_PATTERN))
		assertTest(t, anyOptionPattern, NewOptionPattern("b", ANY_PATTERN))
		assertTest(t, anyOptionPattern, ANY_OPTION_PATTERN)
		assertTestFalse(t, anyOptionPattern, ANY_INT)
		assertTestFalse(t, anyOptionPattern, NewOption("x", EMPTY_STRING))
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := NewOptionPattern("a", ANY_STR_PATTERN)

		assertTestValue(t, pattern, NewOption("a", EMPTY_STRING))
		assertTestValueFalse(t, pattern, NewOption("a", NewInt(1)))
		assertTestValueFalse(t, pattern, NewOption("b", EMPTY_STRING))
		assertTestValueFalse(t, pattern, ANY_INT)
		assertTestValueFalse(t, pattern, ANY_OPTION_PATTERN)

		anyOptionPattern := ANY_OPTION_PATTERN

		assertTestValue(t, anyOptionPattern, ANY_OPTION)
		assertTestValue(t, anyOptionPattern, NewOption("a", EMPTY_STRING))
		assertTestValue(t, anyOptionPattern, NewOption("a", NewInt(1)))
		assertTestValue(t, anyOptionPattern, NewOption("b", EMPTY_STRING))
		assertTestValueFalse(t, anyOptionPattern, ANY_INT)
		assertTestValueFalse(t, anyOptionPattern, ANY_OPTION_PATTERN)

	})

}

func TestSymbolicAnyStringPatternElement(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &AnyStringPattern{}

		assertTest(t, pattern, &AnyStringPattern{})
		assertTestFalse(t, pattern, ANY_INT)
		assertTestFalse(t, pattern, ANY_STR)
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := &AnyStringPattern{}

		assertTestValue(t, pattern, ANY_STR)
		assertTestValueFalse(t, pattern, ANY_INT)
		assertTestValueFalse(t, pattern, &AnyStringPattern{})
	})

}

func TestTypePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		{
			_any := &TypePattern{val: ANY}

			assertTest(t, _any, _any)
			assertTest(t, _any, &TypePattern{val: ANY_INT})
			assertTestFalse(t, _any, ANY_INT)
			assertTestFalse(t, _any, ANY_STR)
		}

		{
			specific := &TypePattern{val: ANY_STR}

			assertTest(t, specific, specific)
			assertTest(t, specific, &TypePattern{val: ANY_STR})
			assertTestFalse(t, specific, &TypePattern{val: ANY_INT})
			assertTestFalse(t, specific, ANY_INT)
			assertTestFalse(t, specific, ANY_STR)
		}

	})

	t.Run("TestValue()", func(t *testing.T) {
		_any := &TypePattern{val: ANY}
		specific := &TypePattern{val: ANY_STR}

		assertTestValue(t, _any, ANY_STR)
		assertTestValue(t, _any, ANY_INT)

		assertTestValue(t, specific, ANY_STR)
		assertTestValueFalse(t, specific, ANY_INT)
	})

}

func TestFunctionPattern(t *testing.T) {

	t.Run("any function pattern", func(t *testing.T) {
		t.Run("Test()", func(t *testing.T) {
			anyFnPatt := &FunctionPattern{}

			assertTest(t, anyFnPatt, anyFnPatt)
			assert.True(t, anyFnPatt.Test(&FunctionPattern{
				node: &parse.FunctionPatternExpression{},
			}, RecTestCallState{}))
			assertTestFalse(t, anyFnPatt, ANY_INT)
			assertTestFalse(t, anyFnPatt, ANY_STR)
		})

		t.Run("TestValue()", func(t *testing.T) {
			anyFnPatt := &FunctionPattern{}

			assertTestValue(t, anyFnPatt, &Function{pattern: anyFnPatt})
			assert.True(t, anyFnPatt.TestValue(&InoxFunction{
				node: &parse.FunctionPatternExpression{},
			}, RecTestCallState{}))
			assertTestValueFalse(t, anyFnPatt, ANY_STR)
			assertTestValueFalse(t, anyFnPatt, anyFnPatt)
		})
	})

	testCases := map[string]struct {
		matchingFnExprs    []string
		notMatchingFnExprs []string
	}{
		"%fn(){}": {
			[]string{"fn(){}"},
			[]string{"fn() %int { return 1 }", "fn() { return 1 }"},
		},
		"%fn() %int": {
			[]string{"fn() %int { return 1 }"},
			[]string{"fn(){}", "fn() %str { return \"\" }"},
		},
	}

	makeState := func() *State {
		emptyChunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "",
			CodeString: "",
		}))

		state := newSymbolicState(NewSymbolicContext(nil, nil, nil), emptyChunk)
		state.ctx.AddNamedPattern("int", &TypePattern{val: ANY_INT}, false)
		state.ctx.AddNamedPattern("str", &TypePattern{val: ANY_STR}, false)
		state.ctx.AddNamedPattern("obj", &TypePattern{val: NewAnyObject()}, false)
		state.pushScope()
		return state
	}

	for pattCode, testCase := range testCases {
		t.Run(pattCode, func(t *testing.T) {
			t.Run("Test()", func(t *testing.T) {
				anyFnPatt := &FunctionPattern{}

				node, _ := parse.ParseExpression(pattCode)
				fnPatt := utils.Must(symbolicEval(node, makeState())).(*FunctionPattern)

				assertTest(t, fnPatt, fnPatt)
				assert.True(t, fnPatt.Test(&FunctionPattern{
					node: node.(*parse.FunctionPatternExpression),
				}, RecTestCallState{}))
				assert.False(t, fnPatt.Test(&FunctionPattern{
					node: &parse.FunctionPatternExpression{},
				}, RecTestCallState{}))
				assertTestFalse(t, fnPatt, anyFnPatt)
				assertTestFalse(t, fnPatt, ANY_INT)
				assertTestFalse(t, fnPatt, ANY_STR)
			})

			t.Run("TestValue()", func(t *testing.T) {
				node, _ := parse.ParseExpression(pattCode)
				fnPatt := utils.Must(symbolicEval(node, makeState())).(*FunctionPattern)

				for _, s := range testCase.matchingFnExprs {
					node, _ := parse.ParseExpression(s)
					matchingFn := utils.Must(symbolicEval(node, makeState())).(*InoxFunction)

					assertTestValue(t, fnPatt, matchingFn, "should match "+s)
				}

				for _, s := range testCase.notMatchingFnExprs {
					node, _ := parse.ParseExpression(s)
					notMatchingFn := utils.Must(symbolicEval(node, makeState())).(*InoxFunction)

					assertTestValueFalse(t, fnPatt, notMatchingFn, "should not match "+s)
				}

				assertTestValueFalse(t, fnPatt, fnPatt)
				assertTestValueFalse(t, fnPatt, ANY_STR)
			})

		})
	}

}

func assertTestValue(t *testing.T, a Pattern, b Value, msg ...any) bool {
	t.Helper()
	return assert.True(t, a.TestValue(b, RecTestCallState{}), msg...)
}

func assertTestValueFalse(t *testing.T, a Pattern, b Value, msg ...any) bool {
	t.Helper()
	return assert.False(t, a.TestValue(b, RecTestCallState{}), msg...)
}

func assertTest(t *testing.T, a, b Value) bool {
	t.Helper()
	return assert.True(t, a.Test(b, RecTestCallState{}))
}

func assertTestFalse(t *testing.T, a, b Value) bool {
	t.Helper()
	return assert.False(t, a.Test(b, RecTestCallState{}))
}
