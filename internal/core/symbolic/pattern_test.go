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

		assert.True(t, pattern.Test(&RegexPattern{}))
		assert.False(t, pattern.Test(ANY_INT))
	})

	t.Run("TestValue() should return true for any symbolic value", func(t *testing.T) {
		pattern := ANY_PATTERN

		assert.True(t, pattern.TestValue(&RegexPattern{}))
		assert.True(t, pattern.TestValue(ANY_INT))
	})

}

func TestSymbolicPathPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyPathPattern := &PathPattern{}

		assert.True(t, anyPathPattern.Test(&PathPattern{}))
		assert.False(t, anyPathPattern.Test(ANY_INT))
		assert.False(t, anyPathPattern.Test(ANY_PATTERN))

		pathPatternWithValue := NewPathPattern("/...")
		assert.True(t, pathPatternWithValue.Test(pathPatternWithValue))
		assert.False(t, pathPatternWithValue.Test(anyPathPattern))
		assert.False(t, pathPatternWithValue.Test(ANY_INT))
		assert.False(t, pathPatternWithValue.Test(ANY_PATTERN))

		pathPatternWithNode := &PathPattern{node: &parse.PathPatternExpression{}}
		assert.True(t, pathPatternWithNode.Test(pathPatternWithNode))
		assert.False(t, pathPatternWithNode.Test(&PathPattern{node: &parse.PathPatternExpression{}}))
		assert.False(t, pathPatternWithNode.Test(anyPathPattern))
		assert.False(t, pathPatternWithNode.Test(pathPatternWithValue))
		assert.False(t, pathPatternWithNode.Test(ANY_INT))
		assert.False(t, pathPatternWithNode.Test(ANY_PATTERN))
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyPathPattern := ANY_PATH_PATTERN

		assert.True(t, anyPathPattern.TestValue(&Path{}))
		assert.True(t, anyPathPattern.TestValue(NewPath("/")))
		assert.True(t, anyPathPattern.TestValue(NewPath("./")))
		assert.True(t, anyPathPattern.TestValue(NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})))
		assert.False(t, anyPathPattern.TestValue(ANY_INT))
		assert.False(t, anyPathPattern.TestValue(ANY_PATH_PATTERN))

		//same tests but with result of .SymbolicValue()
		anyPathPattern_val := anyPathPattern.SymbolicValue()
		assert.True(t, anyPathPattern_val.Test(&Path{}))
		assert.True(t, anyPathPattern_val.Test(NewPath("/")))
		assert.True(t, anyPathPattern_val.Test(NewPath("./")))
		assert.True(t, anyPathPattern_val.Test(NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})))
		assert.False(t, anyPathPattern_val.Test(ANY_INT))
		assert.False(t, anyPathPattern_val.Test(ANY_PATH_PATTERN))

		pathPatternWithValue := NewPathPattern("/...")
		assert.True(t, pathPatternWithValue.TestValue(NewPath("/")))
		assert.True(t, pathPatternWithValue.TestValue(NewPath("/1")))
		assert.True(t, pathPatternWithValue.TestValue(NewPath("/1/")))
		assert.False(t, pathPatternWithValue.TestValue(NewPath("./")))
		assert.False(t, pathPatternWithValue.TestValue(NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})))
		assert.False(t, pathPatternWithValue.TestValue(&Path{}))
		assert.False(t, pathPatternWithValue.TestValue(ANY_INT))
		assert.False(t, pathPatternWithValue.TestValue(ANY_PATH_PATTERN))

		//same tests but with result of .SymbolicValue()
		pathPatternWithValue_val := pathPatternWithValue.SymbolicValue()
		assert.True(t, pathPatternWithValue_val.Test(NewPath("/")))
		assert.True(t, pathPatternWithValue_val.Test(NewPath("/1")))
		assert.True(t, pathPatternWithValue_val.Test(NewPath("/1/")))
		assert.False(t, pathPatternWithValue_val.Test(NewPath("./")))
		assert.False(t, pathPatternWithValue_val.Test(NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})))
		assert.False(t, pathPatternWithValue_val.Test(&Path{}))
		assert.False(t, pathPatternWithValue_val.Test(ANY_INT))
		assert.False(t, pathPatternWithValue_val.Test(ANY_PATH_PATTERN))

		pathPatternWithNode := &PathPattern{node: &parse.PathPatternExpression{}}
		assert.True(t, pathPatternWithNode.TestValue(NewPathMatchingPattern(pathPatternWithNode)))
		assert.False(t, pathPatternWithNode.TestValue(NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})))
		assert.False(t, pathPatternWithNode.TestValue(NewPath("/")))
		assert.False(t, pathPatternWithNode.TestValue(NewPath("./")))
		assert.False(t, pathPatternWithNode.TestValue(&Path{}))
		assert.False(t, pathPatternWithNode.TestValue(ANY_INT))
		assert.False(t, pathPatternWithNode.TestValue(ANY_PATH_PATTERN))

		//same tests but with result of .SymbolicValue()
		pathPatternWithNode_val := pathPatternWithNode.SymbolicValue()
		assert.True(t, pathPatternWithNode_val.Test(NewPathMatchingPattern(pathPatternWithNode)))
		assert.False(t, pathPatternWithNode_val.Test(NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})))
		assert.False(t, pathPatternWithNode_val.Test(NewPath("/")))
		assert.False(t, pathPatternWithNode_val.Test(NewPath("./")))
		assert.False(t, pathPatternWithNode_val.Test(&Path{}))
		assert.False(t, pathPatternWithNode_val.Test(ANY_INT))
		assert.False(t, pathPatternWithNode_val.Test(ANY_PATH_PATTERN))
	})

}

func TestSymbolicUrlPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyUrlPattern := &URLPattern{}

		assert.True(t, anyUrlPattern.Test(&URLPattern{}))
		assert.False(t, anyUrlPattern.Test(ANY_INT))
		assert.False(t, anyUrlPattern.Test(ANY_PATTERN))

		urlPatternWithValue := NewUrlPattern("https://example.com/...")
		assert.True(t, urlPatternWithValue.Test(urlPatternWithValue))
		assert.False(t, urlPatternWithValue.Test(anyUrlPattern))
		assert.False(t, urlPatternWithValue.Test(ANY_INT))
		assert.False(t, urlPatternWithValue.Test(ANY_PATTERN))

		urlPatternWithNode := NewUrlPatternFromNode(&parse.PathPatternExpression{})
		assert.True(t, urlPatternWithNode.Test(urlPatternWithNode))
		assert.False(t, urlPatternWithNode.Test(NewUrlPatternFromNode(&parse.PathPatternExpression{})))
		assert.False(t, urlPatternWithNode.Test(anyUrlPattern))
		assert.False(t, urlPatternWithNode.Test(urlPatternWithValue))
		assert.False(t, urlPatternWithNode.Test(ANY_INT))
		assert.False(t, urlPatternWithNode.Test(ANY_PATTERN))
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyUrlPattern := ANY_URL_PATTERN

		assert.True(t, anyUrlPattern.TestValue(&URL{}))
		assert.True(t, anyUrlPattern.TestValue(NewUrl("https://example.com/")))
		assert.True(t, anyUrlPattern.TestValue(NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}))))
		assert.False(t, anyUrlPattern.TestValue(ANY_INT))
		assert.False(t, anyUrlPattern.TestValue(ANY_URL_PATTERN))

		//same tests but with result of .SymbolicValue()
		anyUrlPattern_val := anyUrlPattern.SymbolicValue()
		assert.True(t, anyUrlPattern_val.Test(&URL{}))
		assert.True(t, anyUrlPattern_val.Test(NewUrl("https://example.com/")))
		assert.True(t, anyUrlPattern_val.Test(NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}))))
		assert.False(t, anyUrlPattern_val.Test(ANY_INT))
		assert.False(t, anyUrlPattern_val.Test(ANY_URL_PATTERN))

		urlPatternWithValue := NewUrlPattern("https://example.com/...")
		assert.True(t, urlPatternWithValue.TestValue(NewUrl("https://example.com/")))
		assert.True(t, urlPatternWithValue.TestValue(NewUrl("https://example.com/1")))
		assert.True(t, urlPatternWithValue.TestValue(NewUrl("https://example.com/1/")))
		assert.False(t, urlPatternWithValue.TestValue(NewUrl("https://localhost/")))
		assert.False(t, urlPatternWithValue.TestValue(NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}))))
		assert.False(t, urlPatternWithValue.TestValue(&URL{}))
		assert.False(t, urlPatternWithValue.TestValue(ANY_INT))
		assert.False(t, urlPatternWithValue.TestValue(ANY_URL_PATTERN))

		//same tests but with result of .SymbolicValue()
		urlPatternWithValue_val := urlPatternWithValue.SymbolicValue()
		assert.True(t, urlPatternWithValue_val.Test(NewUrl("https://example.com/")))
		assert.True(t, urlPatternWithValue_val.Test(NewUrl("https://example.com/1")))
		assert.True(t, urlPatternWithValue_val.Test(NewUrl("https://example.com/1/")))
		assert.False(t, urlPatternWithValue_val.Test(NewUrl("https://localhost/")))
		assert.False(t, urlPatternWithValue_val.Test(NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}))))
		assert.False(t, urlPatternWithValue_val.Test(&URL{}))
		assert.False(t, urlPatternWithValue_val.Test(ANY_INT))
		assert.False(t, urlPatternWithValue_val.Test(ANY_URL_PATTERN))

		urlPatternWithNode := NewUrlPatternFromNode(&parse.URLPatternLiteral{}) //the node will never be a parse.URLPatternLiteral
		assert.True(t, urlPatternWithNode.TestValue(NewUrlMatchingPattern(urlPatternWithNode)))
		assert.False(t, urlPatternWithNode.TestValue(NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}))))
		assert.False(t, urlPatternWithNode.TestValue(NewUrl("https://example.com/")))
		assert.False(t, urlPatternWithNode.TestValue(&URL{}))
		assert.False(t, urlPatternWithNode.TestValue(ANY_INT))
		assert.False(t, urlPatternWithNode.TestValue(ANY_URL_PATTERN))

		//same tests but with result of .SymbolicValue()
		urlPatternWithNode_val := urlPatternWithNode.SymbolicValue()
		assert.True(t, urlPatternWithNode_val.Test(NewUrlMatchingPattern(urlPatternWithNode)))
		assert.False(t, urlPatternWithNode_val.Test(NewUrlMatchingPattern(NewUrlPatternFromNode(&parse.URLPatternLiteral{}))))
		assert.False(t, urlPatternWithNode_val.Test(NewUrl("https://example.com/")))
		assert.False(t, urlPatternWithNode_val.Test(&URL{}))
		assert.False(t, urlPatternWithNode_val.Test(ANY_INT))
		assert.False(t, urlPatternWithNode_val.Test(ANY_URL_PATTERN))
	})

}

func TestSymbolicHostPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyHostPattern := &HostPattern{}

		assert.True(t, anyHostPattern.Test(&HostPattern{}))
		assert.False(t, anyHostPattern.Test(ANY_INT))
		assert.False(t, anyHostPattern.Test(ANY_PATTERN))

		hostPatternWithValue := NewHostPattern("https://example.com")
		assert.True(t, hostPatternWithValue.Test(hostPatternWithValue))
		assert.False(t, hostPatternWithValue.Test(anyHostPattern))
		assert.False(t, hostPatternWithValue.Test(ANY_INT))
		assert.False(t, hostPatternWithValue.Test(ANY_PATTERN))

		hostPatternWithNode := NewHostPatternFromNode(&parse.PathPatternExpression{})
		assert.True(t, hostPatternWithNode.Test(hostPatternWithNode))
		assert.False(t, hostPatternWithNode.Test(NewHostPatternFromNode(&parse.PathPatternExpression{})))
		assert.False(t, hostPatternWithNode.Test(anyHostPattern))
		assert.False(t, hostPatternWithNode.Test(hostPatternWithValue))
		assert.False(t, hostPatternWithNode.Test(ANY_INT))
		assert.False(t, hostPatternWithNode.Test(ANY_PATTERN))
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyHostPattern := ANY_HOST_PATTERN

		assert.True(t, anyHostPattern.TestValue(&Host{}))
		assert.True(t, anyHostPattern.TestValue(NewHost("https://example.com")))
		assert.True(t, anyHostPattern.TestValue(NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}))))
		assert.False(t, anyHostPattern.TestValue(ANY_INT))
		assert.False(t, anyHostPattern.TestValue(ANY_HOST_PATTERN))

		//same tests but with result of .SymbolicValue()
		anyHostPattern_val := anyHostPattern.SymbolicValue()
		assert.True(t, anyHostPattern_val.Test(&Host{}))
		assert.True(t, anyHostPattern_val.Test(NewHost("https://example.com")))
		assert.True(t, anyHostPattern_val.Test(NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}))))
		assert.False(t, anyHostPattern_val.Test(ANY_INT))
		assert.False(t, anyHostPattern_val.Test(ANY_HOST_PATTERN))

		hostPatternWithValue := NewHostPattern("https://example.com")
		assert.True(t, hostPatternWithValue.TestValue(NewHost("https://example.com")))
		assert.False(t, hostPatternWithValue.TestValue(NewHost("https://localhost")))
		assert.False(t, hostPatternWithValue.TestValue(NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}))))
		assert.False(t, hostPatternWithValue.TestValue(&Host{}))
		assert.False(t, hostPatternWithValue.TestValue(ANY_INT))
		assert.False(t, hostPatternWithValue.TestValue(ANY_HOST_PATTERN))

		//same tests but with result of .SymbolicValue()
		hostPatternWithValue_val := hostPatternWithValue.SymbolicValue()
		assert.True(t, hostPatternWithValue_val.Test(NewHost("https://example.com")))
		assert.False(t, hostPatternWithValue_val.Test(NewHost("https://localhost")))
		assert.False(t, hostPatternWithValue_val.Test(NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}))))
		assert.False(t, hostPatternWithValue_val.Test(&Host{}))
		assert.False(t, hostPatternWithValue_val.Test(ANY_INT))
		assert.False(t, hostPatternWithValue_val.Test(ANY_HOST_PATTERN))

		hostPatternWithNode := NewHostPatternFromNode(&parse.HostPatternLiteral{}) //the node will never be a parse.HostPatternLiteral
		assert.True(t, hostPatternWithNode.TestValue(NewHostMatchingPattern(hostPatternWithNode)))
		assert.False(t, hostPatternWithNode.TestValue(NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}))))
		assert.False(t, hostPatternWithNode.TestValue(NewHost("https://example.com")))
		assert.False(t, hostPatternWithNode.TestValue(&Host{}))
		assert.False(t, hostPatternWithNode.TestValue(ANY_INT))
		assert.False(t, hostPatternWithNode.TestValue(ANY_HOST_PATTERN))

		//same tests but with result of .SymbolicValue()
		hostPatternWithNode_val := hostPatternWithNode.SymbolicValue()
		assert.True(t, hostPatternWithNode_val.Test(NewHostMatchingPattern(hostPatternWithNode)))
		assert.False(t, hostPatternWithNode_val.Test(NewHostMatchingPattern(NewHostPatternFromNode(&parse.HostPatternLiteral{}))))
		assert.False(t, hostPatternWithNode_val.Test(NewHost("https://example.com")))
		assert.False(t, hostPatternWithNode_val.Test(&Host{}))
		assert.False(t, hostPatternWithNode_val.Test(ANY_INT))
		assert.False(t, hostPatternWithNode_val.Test(ANY_HOST_PATTERN))
	})

}

func TestSymbolicNamedSegmentPathPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		namedPathPattern := &NamedSegmentPathPattern{}
		specificNamedPathPattern := &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}

		assert.True(t, namedPathPattern.Test(&NamedSegmentPathPattern{}))
		assert.True(t, namedPathPattern.Test(specificNamedPathPattern))
		assert.False(t, namedPathPattern.Test(&Path{}))
		assert.False(t, namedPathPattern.Test(ANY_INT))
		assert.False(t, namedPathPattern.Test(ANY_PATTERN))

		assert.False(t, specificNamedPathPattern.Test(&NamedSegmentPathPattern{}))
		assert.True(t, specificNamedPathPattern.Test(specificNamedPathPattern))
		assert.False(t, specificNamedPathPattern.Test(&Path{}))
		assert.False(t, specificNamedPathPattern.Test(ANY_INT))
		assert.False(t, specificNamedPathPattern.Test(ANY_PATTERN))
	})

	t.Run("TestValue() should return true for any symbolic path", func(t *testing.T) {
		namedPathPattern := &NamedSegmentPathPattern{}
		assert.True(t, namedPathPattern.TestValue(&Path{}))
		assert.False(t, namedPathPattern.TestValue(ANY_INT))
		assert.False(t, namedPathPattern.TestValue(&NamedSegmentPathPattern{}))
		assert.False(t, namedPathPattern.TestValue(&NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}))

		specificNamedPathPattern := &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}
		assert.True(t, specificNamedPathPattern.TestValue(&Path{}))
		assert.False(t, specificNamedPathPattern.TestValue(ANY_INT))
		assert.False(t, specificNamedPathPattern.TestValue(&NamedSegmentPathPattern{}))
		assert.False(t, specificNamedPathPattern.TestValue(&NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}))
	})

}

func TestSymbolicExactValuePattern(t *testing.T) {
	t.Run("Test()", func(t *testing.T) {
		pattern := &ExactValuePattern{value: ANY_INT}

		assert.True(t, pattern.Test(pattern))
		assert.False(t, pattern.Test(ANY_INT))
		assert.False(t, pattern.Test(ANY_PATTERN))
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := &ExactValuePattern{value: ANY_INT}

		assert.True(t, pattern.TestValue(ANY_INT))
		assert.False(t, pattern.TestValue(ANY_SERIALIZABLE))
		assert.False(t, pattern.TestValue(pattern))
	})

}

func TestSymbolicRegexPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &RegexPattern{}

		assert.True(t, pattern.Test(&RegexPattern{}))
		assert.False(t, pattern.Test(ANY_INT))
		assert.False(t, pattern.Test(ANY_PATTERN))

		patternWithRegex := NewRegexPattern("(a|b)")
		assert.True(t, patternWithRegex.Test(patternWithRegex))
		assert.True(t, patternWithRegex.Test(NewRegexPattern("(a|b)")))
		assert.True(t, patternWithRegex.Test(NewRegexPattern("[ab]")))
		assert.False(t, patternWithRegex.Test(&RegexPattern{}))
		assert.False(t, patternWithRegex.Test(ANY_INT))
		assert.False(t, patternWithRegex.Test(ANY_PATTERN))
	})

	t.Run("TestValue() & SymbolicValue()", func(t *testing.T) {
		pattern := &RegexPattern{}

		assert.True(t, pattern.TestValue(ANY_STR))
		assert.False(t, pattern.TestValue(ANY_INT))
		assert.False(t, pattern.TestValue(&RegexPattern{}))

		patternWithRegex := NewRegexPattern("(a|b)")
		val := patternWithRegex.SymbolicValue()

		assert.True(t, patternWithRegex.TestValue(NewString("a")))
		assert.True(t, val.Test(NewString("a")))

		assert.True(t, patternWithRegex.TestValue(NewString("b")))
		assert.True(t, val.Test(NewString("b")))

		assert.False(t, patternWithRegex.TestValue(NewString("c")))
		assert.False(t, val.Test(NewString("c")))

		assert.False(t, patternWithRegex.TestValue(&RegexPattern{}))
		assert.False(t, val.Test(&RegexPattern{}))

		assert.False(t, patternWithRegex.TestValue(ANY_INT))
		assert.False(t, val.Test(ANY_INT))

		assert.False(t, patternWithRegex.TestValue(ANY_PATTERN))
		assert.False(t, val.Test(ANY_PATTERN))
	})

}

func TestSymbolicObjectPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {

		t.Run("objects should never be matched", func(t *testing.T) {
			pattern := &ObjectPattern{entries: nil}

			assert.False(t, pattern.Test(&Object{entries: nil}))
			assert.False(t, pattern.Test(&Object{entries: map[string]Serializable{}}))
		})

		t.Run("if entries is nil any other object pattern of the same 'readonlyness' should be matched", func(t *testing.T) {
			pattern := &ObjectPattern{entries: nil}
			assert.True(t, pattern.Test(&ObjectPattern{entries: nil}))
			assert.True(t, pattern.Test(&ObjectPattern{inexact: true, entries: map[string]Pattern{}}))
			assert.True(t, pattern.Test(&ObjectPattern{entries: map[string]Pattern{}}))
			assert.True(t, pattern.Test(&ObjectPattern{inexact: true, entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}}))
			assert.True(t, pattern.Test(&ObjectPattern{entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}}))

			assert.False(t, pattern.Test(&ObjectPattern{readonly: true, entries: nil}))
			assert.False(t, pattern.Test(&ObjectPattern{readonly: true, entries: map[string]Pattern{}, inexact: true}))
			assert.False(t, pattern.Test(&ObjectPattern{readonly: true, entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}}))
		})

		t.Run("exact empty object pattern should match any other exact object pattern of the same 'readonlyness'", func(t *testing.T) {
			pattern := &ObjectPattern{entries: map[string]Pattern{}}
			assert.True(t, pattern.Test(&ObjectPattern{entries: map[string]Pattern{}}))

			assert.False(t, pattern.Test(&ObjectPattern{readonly: true, entries: map[string]Pattern{}}))
			assert.False(t, pattern.Test(&ObjectPattern{entries: nil}))
			assert.False(t, pattern.Test(&ObjectPattern{entries: map[string]Pattern{}, inexact: true}))
			assert.False(t, pattern.Test(&ObjectPattern{entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}}))
		})

		t.Run("inexact empty object pattern should match any other non-any object pattern of the same 'readonlyness'", func(t *testing.T) {
			pattern := &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}
			assert.True(t, pattern.Test(&ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}))
			assert.True(t, pattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}))

			assert.False(t, pattern.Test(&ObjectPattern{readonly: true, entries: nil}))
			assert.False(t, pattern.Test(&ObjectPattern{readonly: true, entries: map[string]Pattern{}, inexact: true}))
			assert.False(t, pattern.Test(&ObjectPattern{readonly: true, entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}}}))
		})

		t.Run("exact object pattern with a single prop should match any other exact object pattern "+
			"with the same single prop (same readonlyness)", func(t *testing.T) {
			singleIntPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}
			assert.True(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{},
			}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: true,
			}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries:  map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				readonly: true,
			}))

			singleAnyPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY}},
			}

			assert.True(t, singleAnyPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}))

			assert.False(t, singleIntPropPattern.Test(singleAnyPropPattern))
		})

		t.Run("inexact object pattern with a single prop should match any other exact object pattern", func(t *testing.T) {
			singleIntPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: true,
			}
			assert.True(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}))
			assert.True(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{
					"a": &TypePattern{val: ANY_INT},
					"b": &TypePattern{val: ANY_INT},
				},
			}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{},
			}))

			assert.False(t, singleIntPropPattern.Test(&ObjectPattern{
				entries:  map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				readonly: true,
			}))

			singleAnyPropPattern := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY}},
				inexact: true,
			}

			assert.True(t, singleAnyPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
			}))

			assert.True(t, singleAnyPropPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{
					"a": &TypePattern{val: ANY_INT},
					"b": &TypePattern{val: ANY_INT},
				},
			}))

			assert.False(t, singleIntPropPattern.Test(singleAnyPropPattern))
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
			}))

			//same as propAwithBdep but with a pattern in the dependency
			assert.True(t, propAwithBdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b"},
						pattern:      ANY_OBJECT_PATTERN,
					},
				},
			}))

			assert.False(t, propAwithBdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				inexact: true,
			}))

			assert.False(t, propAwithBdep.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				readonly: true,
			}))

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
			}))

			//pattern is missing
			assert.False(t, propAwithBdepAndPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {
						requiredKeys: []string{"b"},
					},
				},
			}))

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
			}))

			//one of the required key is missing
			assert.False(t, propAwithBdepAndPattern.Test(&ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			}))
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
			if !assert.False(t, patt.TestValue(&ObjectPattern{entries: nil})) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&ObjectPattern{entries: nil})) {
				return
			}

			//should never match another object pattern (empty)
			if !assert.False(t, patt.TestValue(&ObjectPattern{entries: map[string]Pattern{}})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&ObjectPattern{entries: map[string]Pattern{}})) {
				return
			}

			//should match object 'any'
			if !assert.True(t, patt.TestValue(&Object{entries: nil})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{entries: nil})) {
				return
			}

			//should not match readonly object 'any'
			if !assert.False(t, patt.TestValue(&Object{entries: nil, readonly: true})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil, readonly: true})) {
				return
			}

			//should match empty objects (inexact)
			if !assert.True(t, patt.TestValue(&Object{entries: map[string]Serializable{}})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{entries: map[string]Serializable{}})) {
				return
			}

			//should match empty objects (exact)
			if !assert.True(t, patt.TestValue(&Object{entries: map[string]Serializable{}, exact: true})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{entries: map[string]Serializable{}, exact: true})) {
				return
			}
		})

		t.Run("empty exact object pattern", func(t *testing.T) {
			patt := &ObjectPattern{entries: map[string]Pattern{}}

			//should not match object 'any'
			if !assert.False(t, patt.TestValue(&Object{entries: nil})) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}

			//should not match an exact object with a property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			})) {
				return
			}
		})

		t.Run("empty inexact object pattern", func(t *testing.T) {
			patt := &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}

			//should not match object 'any'
			if !assert.False(t, patt.TestValue(&Object{entries: nil})) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil})) {
				return
			}

			//should match an empty inexact object
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}

			//should match an inexact object with a property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   false,
			})) {
				return
			}

			//should match an empty exact object
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}

			//should match an exact object with a property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}
		})

		t.Run("inexact object pattern with a single prop", func(t *testing.T) {
			patt := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: true,
			}

			//should not match object 'any'
			if !assert.False(t, patt.TestValue(&Object{entries: nil})) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}

			//should match an inexact object with the same property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}

			//should not match an exact object with the same property but optional
			if !assert.False(t, patt.TestValue(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}

			//should not match an exact object with the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			})) {
				return
			}

			//should not match an exact object with the same property but different type
			if !assert.False(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_BOOL},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_BOOL},
			})) {
				return
			}

			//should match an exact object with the same property + an additional property
			if !assert.True(t, patt.TestValue(&Object{
				exact: true,
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact: true,
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}

			//should match an inexact object with the same property + an additional property
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}

			//should match an exact object with the same property
			if !assert.True(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
		})

		t.Run("exact object pattern with a single prop", func(t *testing.T) {
			patt := &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_INT}},
				inexact: false,
			}

			//should not match object 'any'
			if !assert.False(t, patt.TestValue(&Object{entries: nil})) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}

			//should not match an inexact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}

			//should not match an inexact object with the same property but optional
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}

			//should not match an inexact object with the same property if super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			})) {
				return
			}

			//should not match an inexact object with the same property if different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
			})) {
				return
			}

			//should match an inexact object with the same property + an additional property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}

			//should match an exact object with the same property
			if !assert.True(t, patt.TestValue(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact:   true,
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:           true,
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
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
			if !assert.False(t, patt.TestValue(&Object{entries: nil})) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}

			//should not match an empty inexact object with the same dependency
			if !assert.False(t, patt.TestValue(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}

			//TODO: empty exact object with the same dependency
			//does that even make sense ? should some dependencies be forbidden for exact objects & exact object patterns ?

			//should not match an inexact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}

			//should not match an inexact object having the same property (but optional) and dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}

			//should not match an inexact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}

			//should not match an inexact object having the same property but optional and the same dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an inexact object with the same property if super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an inexact object with the same property if different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
			})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an inexact object with the same property + validating the dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}

			//should not match an inexact object with the same property + an additional property unrelated to the dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			})) {
				return
			}

			//should not match an exact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
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
			if !assert.False(t, patt.TestValue(&Object{entries: nil})) {
				return
			}
			val := patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{entries: nil})) {
				return
			}

			//should not match an empty inexact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   false,
			})) {
				return
			}

			//should match an empty inexact object with the same dependency
			if !assert.True(t, patt.TestValue(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				exact:   false,
				entries: map[string]Serializable{},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an empty exact object
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{},
				exact:   true,
			})) {
				return
			}

			//should not match an inexact object with the same property but not validating the dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}

			//should match an inexact object having the same property (but optional) and dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}

			//should not match an inexact object with the same property but optional
			if !assert.False(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			})) {
				return
			}

			//should match an inexact object having the same property but optional and the same dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an inexact object with the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but super type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an inexact object with the same property if different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
			})) {
				return
			}

			//should not match an inexact object having the same dependency and the same property but different type
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_BOOL},
				dependencies: map[string]propertyDependencies{
					"a": {requiredKeys: []string{"b"}},
				},
			})) {
				return
			}

			//should not match an inexact object with the same property + validating the dependency
			if !assert.True(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.True(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": ANY_INT,
				},
			})) {
				return
			}

			//should not match an inexact object with the same property + an additional property unrelated to the dependency
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"c": ANY_INT,
				},
			})) {
				return
			}

			//should not match an exact object with the same property
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}

			//should not match an exact object with the same property if optional
			if !assert.False(t, patt.TestValue(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}
			val = patt.SymbolicValue()
			if !assert.False(t, val.Test(&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			})) {
				return
			}
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
			value   SymbolicValue
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
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value))
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
			testedValue SymbolicValue
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
				if !assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue)) {
					return
				}
				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.testedValue))
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
			value   SymbolicValue
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
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value))
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
			testedValue SymbolicValue
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
				if !assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue)) {
					return
				}
				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.testedValue))

				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue))
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
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value))
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
				if !assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.testedValue)) {
					return
				}
				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.testedValue))
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
			value   SymbolicValue
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
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value))
			})
		}
	})

	t.Run("TestValue()", func(t *testing.T) {
		cases := []struct {
			pattern *UnionPattern
			value   SymbolicValue
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
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))

				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.value))
			})
		}
	})

}

func TestSymbolicIntersectionPattern(t *testing.T) {
	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			name    string
			pattern *IntersectionPattern
			value   SymbolicValue
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
				assert.Equal(t, testCase.ok, testCase.pattern.Test(testCase.value))
			})
		}
	})

	t.Run("TestValue()", func(t *testing.T) {
		cases := []struct {
			name    string
			pattern *IntersectionPattern
			value   SymbolicValue
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
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))

				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.value))
			})
		}
	})

}

func TestSymbolicOptionPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := NewOptionPattern("a", ANY_STR_PATTERN)

		assert.True(t, pattern.Test(NewOptionPattern("a", ANY_STR_PATTERN)))
		assert.False(t, pattern.Test(NewOptionPattern("b", ANY_PATTERN)))
		assert.False(t, pattern.Test(ANY_OPTION_PATTERN))
		assert.False(t, pattern.Test(ANY_INT))
		assert.False(t, pattern.Test(NewOption("x", EMPTY_STRING)))

		anyOptionPattern := ANY_OPTION_PATTERN
		assert.True(t, anyOptionPattern.Test(NewOptionPattern("a", ANY_STR_PATTERN)))
		assert.True(t, anyOptionPattern.Test(NewOptionPattern("b", ANY_PATTERN)))
		assert.True(t, anyOptionPattern.Test(ANY_OPTION_PATTERN))
		assert.False(t, anyOptionPattern.Test(ANY_INT))
		assert.False(t, anyOptionPattern.Test(NewOption("x", EMPTY_STRING)))
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := NewOptionPattern("a", ANY_STR_PATTERN)

		assert.True(t, pattern.TestValue(NewOption("a", EMPTY_STRING)))
		assert.False(t, pattern.TestValue(NewOption("a", NewInt(1))))
		assert.False(t, pattern.TestValue(NewOption("b", EMPTY_STRING)))
		assert.False(t, pattern.TestValue(ANY_INT))
		assert.False(t, pattern.TestValue(ANY_OPTION_PATTERN))

		anyOptionPattern := ANY_OPTION_PATTERN

		assert.True(t, anyOptionPattern.TestValue(ANY_OPTION))
		assert.True(t, anyOptionPattern.TestValue(NewOption("a", EMPTY_STRING)))
		assert.True(t, anyOptionPattern.TestValue(NewOption("a", NewInt(1))))
		assert.True(t, anyOptionPattern.TestValue(NewOption("b", EMPTY_STRING)))
		assert.False(t, anyOptionPattern.TestValue(ANY_INT))
		assert.False(t, anyOptionPattern.TestValue(ANY_OPTION_PATTERN))

	})

}

func TestSymbolicAnyStringPatternElement(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &AnyStringPattern{}

		assert.True(t, pattern.Test(&AnyStringPattern{}))
		assert.False(t, pattern.Test(ANY_INT))
		assert.False(t, pattern.Test(ANY_STR))
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := &AnyStringPattern{}

		assert.True(t, pattern.TestValue(ANY_STR))
		assert.False(t, pattern.TestValue(ANY_INT))
		assert.False(t, pattern.TestValue(&AnyStringPattern{}))
	})

}

func TestTypePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		{
			_any := &TypePattern{val: ANY}

			assert.True(t, _any.Test(_any))
			assert.True(t, _any.Test(&TypePattern{val: ANY_INT}))
			assert.False(t, _any.Test(ANY_INT))
			assert.False(t, _any.Test(ANY_STR))
		}

		{
			specific := &TypePattern{val: ANY_STR}

			assert.True(t, specific.Test(specific))
			assert.True(t, specific.Test(&TypePattern{val: ANY_STR}))
			assert.False(t, specific.Test(&TypePattern{val: ANY_INT}))
			assert.False(t, specific.Test(ANY_INT))
			assert.False(t, specific.Test(ANY_STR))
		}

	})

	t.Run("TestValue()", func(t *testing.T) {
		_any := &TypePattern{val: ANY}
		specific := &TypePattern{val: ANY_STR}

		assert.True(t, _any.TestValue(ANY_STR))
		assert.True(t, _any.TestValue(ANY_INT))

		assert.True(t, specific.TestValue(ANY_STR))
		assert.False(t, specific.TestValue(ANY_INT))
	})

}

func TestFunctionPattern(t *testing.T) {

	t.Run("any function pattern", func(t *testing.T) {
		t.Run("Test()", func(t *testing.T) {
			anyFnPatt := &FunctionPattern{}

			assert.True(t, anyFnPatt.Test(anyFnPatt))
			assert.True(t, anyFnPatt.Test(&FunctionPattern{
				node: &parse.FunctionPatternExpression{},
			}))
			assert.False(t, anyFnPatt.Test(ANY_INT))
			assert.False(t, anyFnPatt.Test(ANY_STR))
		})

		t.Run("TestValue()", func(t *testing.T) {
			anyFnPatt := &FunctionPattern{}

			assert.True(t, anyFnPatt.TestValue(&Function{pattern: anyFnPatt}))
			assert.True(t, anyFnPatt.TestValue(&InoxFunction{
				node: &parse.FunctionPatternExpression{},
			}))
			assert.False(t, anyFnPatt.TestValue(ANY_STR))
			assert.False(t, anyFnPatt.TestValue(anyFnPatt))
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

				assert.True(t, fnPatt.Test(fnPatt))
				assert.True(t, fnPatt.Test(&FunctionPattern{
					node: node.(*parse.FunctionPatternExpression),
				}))
				assert.False(t, fnPatt.Test(&FunctionPattern{
					node: &parse.FunctionPatternExpression{},
				}))
				assert.False(t, fnPatt.Test(anyFnPatt))
				assert.False(t, fnPatt.Test(ANY_INT))
				assert.False(t, fnPatt.Test(ANY_STR))
			})

			t.Run("TestValue()", func(t *testing.T) {
				node, _ := parse.ParseExpression(pattCode)
				fnPatt := utils.Must(symbolicEval(node, makeState())).(*FunctionPattern)

				for _, s := range testCase.matchingFnExprs {
					node, _ := parse.ParseExpression(s)
					matchingFn := utils.Must(symbolicEval(node, makeState())).(*InoxFunction)

					assert.True(t, fnPatt.TestValue(matchingFn), "should match "+s)
				}

				for _, s := range testCase.notMatchingFnExprs {
					node, _ := parse.ParseExpression(s)
					notMatchingFn := utils.Must(symbolicEval(node, makeState())).(*InoxFunction)

					assert.False(t, fnPatt.TestValue(notMatchingFn), "should not match "+s)
				}

				assert.False(t, fnPatt.TestValue(fnPatt))
				assert.False(t, fnPatt.TestValue(ANY_STR))
			})

		})
	}

}
