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
		pattern := &AnyPattern{}

		assert.True(t, pattern.Test(&RegexPattern{}))
		assert.False(t, pattern.Test(&Int{}))
	})

	t.Run("TestValue() should return true for any symbolic value", func(t *testing.T) {
		pattern := &AnyPattern{}

		assert.True(t, pattern.TestValue(&RegexPattern{}))
		assert.True(t, pattern.TestValue(&Int{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &AnyPattern{}

		assert.False(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicPathPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &PathPattern{}

		assert.True(t, pattern.Test(&PathPattern{}))
		assert.False(t, pattern.Test(&Int{}))
		assert.False(t, pattern.Test(&AnyPattern{}))
	})

	t.Run("TestValue() should return true for any symbolic path", func(t *testing.T) {
		pattern := &PathPattern{}

		assert.True(t, pattern.TestValue(&Path{}))
		assert.False(t, pattern.TestValue(&Int{}))
		assert.False(t, pattern.TestValue(&PathPattern{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &PathPattern{}

		assert.False(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicURLPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &URLPattern{}

		assert.True(t, pattern.Test(&URLPattern{}))
		assert.False(t, pattern.Test(&URL{}))
		assert.False(t, pattern.Test(&Int{}))
		assert.False(t, pattern.Test(&AnyPattern{}))
	})

	t.Run("TestValue() should return true for any symbolic URL", func(t *testing.T) {
		pattern := &URLPattern{}

		assert.True(t, pattern.TestValue(&URL{}))
		assert.False(t, pattern.TestValue(&Int{}))
		assert.False(t, pattern.TestValue(&URLPattern{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &URLPattern{}

		assert.False(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicHostPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &HostPattern{}

		assert.True(t, pattern.Test(&HostPattern{}))
		assert.False(t, pattern.Test(&Host{}))
		assert.False(t, pattern.Test(&Int{}))
		assert.False(t, pattern.Test(&AnyPattern{}))
	})

	t.Run("TestValue() should return true for any symbolic Host", func(t *testing.T) {
		pattern := &HostPattern{}

		assert.True(t, pattern.TestValue(&Host{}))
		assert.False(t, pattern.TestValue(&Int{}))
		assert.False(t, pattern.TestValue(&HostPattern{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &HostPattern{}

		assert.False(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicNamedSegmentPathPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		namedPathPattern := &NamedSegmentPathPattern{}
		specificNamedPathPattern := &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}

		assert.True(t, namedPathPattern.Test(&NamedSegmentPathPattern{}))
		assert.True(t, namedPathPattern.Test(specificNamedPathPattern))
		assert.False(t, namedPathPattern.Test(&Path{}))
		assert.False(t, namedPathPattern.Test(&Int{}))
		assert.False(t, namedPathPattern.Test(&AnyPattern{}))

		assert.False(t, specificNamedPathPattern.Test(&NamedSegmentPathPattern{}))
		assert.True(t, specificNamedPathPattern.Test(specificNamedPathPattern))
		assert.False(t, specificNamedPathPattern.Test(&Path{}))
		assert.False(t, specificNamedPathPattern.Test(&Int{}))
		assert.False(t, specificNamedPathPattern.Test(&AnyPattern{}))
	})

	t.Run("TestValue() should return true for any symbolic path", func(t *testing.T) {
		namedPathPattern := &NamedSegmentPathPattern{}
		assert.True(t, namedPathPattern.TestValue(&Path{}))
		assert.False(t, namedPathPattern.TestValue(&Int{}))
		assert.False(t, namedPathPattern.TestValue(&NamedSegmentPathPattern{}))
		assert.False(t, namedPathPattern.TestValue(&NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}))

		specificNamedPathPattern := &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}
		assert.True(t, specificNamedPathPattern.TestValue(&Path{}))
		assert.False(t, specificNamedPathPattern.TestValue(&Int{}))
		assert.False(t, specificNamedPathPattern.TestValue(&NamedSegmentPathPattern{}))
		assert.False(t, specificNamedPathPattern.TestValue(&NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		namedPathPattern := &NamedSegmentPathPattern{}
		assert.False(t, namedPathPattern.IsWidenable())
		widened, ok := namedPathPattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)

		specificNamedPathPattern := &NamedSegmentPathPattern{node: &parse.NamedSegmentPathPatternLiteral{}}
		assert.True(t, specificNamedPathPattern.IsWidenable())
		widened, ok = specificNamedPathPattern.Widen()
		assert.True(t, ok)
		assert.Equal(t, &NamedSegmentPathPattern{node: nil}, widened)
	})
}

func TestSymbolicExactValuePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &ExactValuePattern{value: &Int{}}

		assert.True(t, pattern.Test(pattern))
		assert.False(t, pattern.Test(&Int{}))
		assert.False(t, pattern.Test(&AnyPattern{}))
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := &ExactValuePattern{value: &Int{}}

		assert.True(t, pattern.TestValue(&Int{}))
		assert.False(t, pattern.TestValue(ANY))
		assert.False(t, pattern.TestValue(pattern))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &ExactValuePattern{value: &Int{}}
		assert.True(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.True(t, ok)
		assert.Equal(t, &ExactValuePattern{value: ANY}, widened)
	})
}

func TestSymbolicRegexPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &RegexPattern{}

		assert.True(t, pattern.Test(&RegexPattern{}))
		assert.False(t, pattern.Test(&Int{}))
		assert.False(t, pattern.Test(&AnyPattern{}))
	})

	t.Run("TestValue() should return true for any string", func(t *testing.T) {
		pattern := &RegexPattern{}

		assert.True(t, pattern.TestValue(&String{}))
		assert.False(t, pattern.TestValue(&Int{}))
		assert.False(t, pattern.TestValue(&RegexPattern{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &RegexPattern{}

		assert.False(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicObjectPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			pattern *ObjectPattern
			value   SymbolicValue
			ok      bool
		}{
			//symbolic object
			{&ObjectPattern{entries: nil}, &Object{entries: nil}, false},
			{&ObjectPattern{entries: nil}, &Object{entries: map[string]SymbolicValue{}}, false},

			//symbolic object pattern
			{&ObjectPattern{entries: nil}, &ObjectPattern{entries: nil}, true},
			{&ObjectPattern{entries: map[string]Pattern{}}, &ObjectPattern{entries: nil}, false},
			{&ObjectPattern{entries: nil}, &ObjectPattern{entries: map[string]Pattern{}}, true},

			{&ObjectPattern{entries: map[string]Pattern{}}, &ObjectPattern{entries: map[string]Pattern{}}, true},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&ObjectPattern{
					entries: map[string]Pattern{},
				},
				false,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{},
				},
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				false,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				true,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &AnyPattern{}},
				},
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				true,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&ObjectPattern{
					entries: map[string]Pattern{"a": &AnyPattern{}},
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
			pattern *ObjectPattern
			value   SymbolicValue
			ok      bool
		}{
			{&ObjectPattern{entries: nil}, &ObjectPattern{entries: nil}, false},
			{&ObjectPattern{entries: nil}, &ObjectPattern{entries: map[string]Pattern{}}, false},

			//symbolic object
			{&ObjectPattern{entries: nil}, &Object{entries: nil}, true},
			{&ObjectPattern{entries: map[string]Pattern{}}, &Object{entries: nil}, false},
			{&ObjectPattern{entries: nil}, &Object{entries: map[string]SymbolicValue{}}, true},

			{
				&ObjectPattern{entries: map[string]Pattern{}},
				&Object{entries: map[string]SymbolicValue{}},
				true,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&Object{
					entries: map[string]SymbolicValue{},
				},
				false,
			},
			{
				&ObjectPattern{
					entries:         map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
					optionalEntries: map[string]struct{}{"a": {}},
				},
				&Object{
					entries: map[string]SymbolicValue{},
				},
				true,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{},
				},
				&Object{
					entries: map[string]SymbolicValue{"a": &Int{}},
				},
				false,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&Object{
					entries: map[string]SymbolicValue{"a": &Int{}},
				},
				true,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &AnyPattern{}},
				},
				&Object{
					entries: map[string]SymbolicValue{"a": &Int{}},
				},
				true,
			},
			{
				&ObjectPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&Object{
					entries: map[string]SymbolicValue{"a": ANY},
				},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))
			})
		}
	})

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			pattern *ObjectPattern
			widened *ObjectPattern
			ok      bool
		}{
			{
				&ObjectPattern{},
				nil,
				false,
			},
			{
				&ObjectPattern{
					inexact: true,
					entries: make(map[string]Pattern),
				},
				&ObjectPattern{},
				true,
			},
			{
				&ObjectPattern{
					inexact: false,
					entries: make(map[string]Pattern),
				},
				&ObjectPattern{},
				true,
			},
			{
				&ObjectPattern{
					inexact: false,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: &Int{}},
					},
				},
				//the entries can be widened
				&ObjectPattern{
					inexact: false,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: ANY},
					},
				},
				true,
			},
			{
				&ObjectPattern{
					inexact: true,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: &Int{}},
					},
				},
				//the entries can be widened
				&ObjectPattern{
					inexact: true,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: ANY},
					},
				}, true,
			},
			{
				&ObjectPattern{
					inexact: true,
					entries: map[string]Pattern{
						"any": &ExactValuePattern{value: ANY},
					},
				},
				//entries cannot be widened and the pattern is already inexact
				&ObjectPattern{},
				true,
			},
			{
				&ObjectPattern{
					inexact: false,
					entries: map[string]Pattern{
						"any": &ExactValuePattern{value: ANY},
					},
				},
				//entries cannot be widened so the object pattern becomes inexact
				&ObjectPattern{
					inexact: true,
					entries: map[string]Pattern{
						"any": &ExactValuePattern{value: ANY},
					},
				},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.pattern), func(t *testing.T) {

				widened, ok := testCase.pattern.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.pattern.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
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
			{&RecordPattern{entries: nil}, &Object{entries: map[string]SymbolicValue{}}, false},

			//symbolic object pattern
			{&RecordPattern{entries: nil}, &RecordPattern{entries: nil}, true},
			{&RecordPattern{entries: map[string]Pattern{}}, &RecordPattern{entries: nil}, false},
			{&RecordPattern{entries: nil}, &RecordPattern{entries: map[string]Pattern{}}, true},

			{&RecordPattern{entries: map[string]Pattern{}}, &RecordPattern{entries: map[string]Pattern{}}, true},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
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
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				false,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &AnyPattern{}},
				},
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&RecordPattern{
					entries: map[string]Pattern{"a": &AnyPattern{}},
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
			pattern *RecordPattern
			value   SymbolicValue
			ok      bool
		}{
			{&RecordPattern{entries: nil}, &RecordPattern{entries: nil}, false},
			{&RecordPattern{entries: nil}, &RecordPattern{entries: map[string]Pattern{}}, false},

			//symbolic object
			{&RecordPattern{entries: nil}, &Record{entries: nil}, true},
			{&RecordPattern{entries: map[string]Pattern{}}, &Record{entries: nil}, false},
			{&RecordPattern{entries: nil}, &Record{entries: map[string]SymbolicValue{}}, true},

			{
				&RecordPattern{entries: map[string]Pattern{}},
				&Record{entries: map[string]SymbolicValue{}},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&Record{
					entries: map[string]SymbolicValue{},
				},
				false,
			},
			{
				&RecordPattern{
					entries:         map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
					optionalEntries: map[string]struct{}{"a": {}},
				},
				&Record{
					entries: map[string]SymbolicValue{},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{},
				},
				&Record{
					entries: map[string]SymbolicValue{"a": &Int{}},
				},
				false,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&Record{
					entries: map[string]SymbolicValue{"a": &Int{}},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &AnyPattern{}},
				},
				&Record{
					entries: map[string]SymbolicValue{"a": &Int{}},
				},
				true,
			},
			{
				&RecordPattern{
					entries: map[string]Pattern{"a": &ExactValuePattern{value: &Int{}}},
				},
				&Record{
					entries: map[string]SymbolicValue{"a": ANY},
				},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))
			})
		}
	})

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			pattern *RecordPattern
			widened *RecordPattern
			ok      bool
		}{
			{
				&RecordPattern{},
				nil,
				false,
			},
			{
				&RecordPattern{
					inexact: true,
					entries: make(map[string]Pattern),
				},
				&RecordPattern{},
				true,
			},
			{
				&RecordPattern{
					inexact: false,
					entries: make(map[string]Pattern),
				},
				&RecordPattern{},
				true,
			},
			{
				&RecordPattern{
					inexact: false,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: &Int{}},
					},
				},
				//the entries can be widened
				&RecordPattern{
					inexact: false,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: ANY},
					},
				},
				true,
			},
			{
				&RecordPattern{
					inexact: true,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: &Int{}},
					},
				},
				//the entries can be widened
				&RecordPattern{
					inexact: true,
					entries: map[string]Pattern{
						"name": &ExactValuePattern{value: ANY},
					},
				}, true,
			},
			{
				&RecordPattern{
					inexact: true,
					entries: map[string]Pattern{
						"any": &ExactValuePattern{value: ANY},
					},
				},
				//entries cannot be widened and the pattern is already inexact
				&RecordPattern{},
				true,
			},
			{
				&RecordPattern{
					inexact: false,
					entries: map[string]Pattern{
						"any": &ExactValuePattern{value: ANY},
					},
				},
				//entries cannot be widened so the object pattern becomes inexact
				&RecordPattern{
					inexact: true,
					entries: map[string]Pattern{
						"any": &ExactValuePattern{value: ANY},
					},
				},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.pattern), func(t *testing.T) {

				widened, ok := testCase.pattern.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.pattern.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
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
				&ListPattern{generalElement: &AnyPattern{}},
				&List{generalElement: ANY},
				false,
			},

			{
				&ListPattern{generalElement: &AnyPattern{}},
				&ListPattern{generalElement: &AnyPattern{}},
				true,
			},
			{
				&ListPattern{generalElement: &AnyPattern{}},
				&ListPattern{generalElement: &ExactValuePattern{value: &Int{}}},
				true,
			},
			{
				&ListPattern{generalElement: &AnyPattern{}},
				&ListPattern{elements: []Pattern{}},
				true,
			},
			{
				&ListPattern{elements: []Pattern{}},
				&ListPattern{generalElement: &AnyPattern{}},
				false,
			},
			{
				&ListPattern{elements: []Pattern{}},
				&ListPattern{elements: []Pattern{&AnyPattern{}}},
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
			pattern *ListPattern
			value   SymbolicValue
			ok      bool
		}{
			//[]any
			{
				&ListPattern{generalElement: &AnyPattern{}},
				&List{elements: []SymbolicValue{}}, //empty list
				true,
			},
			{
				&ListPattern{generalElement: &AnyPattern{}},
				&List{generalElement: &Int{}}, //[]int
				true,
			},
			{
				&ListPattern{generalElement: &AnyPattern{}},
				&List{generalElement: ANY}, //[]any
				true,
			},
			{
				&ListPattern{generalElement: &AnyPattern{}},
				&List{elements: []SymbolicValue{ANY}}, //[any]
				true,
			},

			//[any]
			{
				&ListPattern{elements: []Pattern{&AnyPattern{}}},
				&List{generalElement: ANY}, //[any]
				false,
			},
			{
				&ListPattern{elements: []Pattern{&AnyPattern{}}},
				&List{elements: []SymbolicValue{&Int{}}}, //[string]
				true,
			},
			{
				&ListPattern{elements: []Pattern{&AnyPattern{}}},
				&List{elements: []SymbolicValue{}}, //empty list
				false,
			},

			//[]int
			{
				&ListPattern{generalElement: &TypePattern{val: &Int{}}},
				&List{elements: []SymbolicValue{}}, //empty list
				true,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: &Int{}}},
				&List{generalElement: &Int{}}, //[]int
				true,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: &Int{}}},
				&List{elements: []SymbolicValue{&Int{}}}, //[int]
				true,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: &Int{}}},
				&List{generalElement: ANY}, //[]any
				false,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: &Int{}}},
				&List{generalElement: &String{}}, //[]string
				false,
			},
			{
				&ListPattern{generalElement: &TypePattern{val: &Int{}}},
				&List{elements: []SymbolicValue{&Int{}, &String{}}}, //[int, string]
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))
			})
		}
	})

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			pattern *ListPattern
			widened *ListPattern
			ok      bool
		}{
			{
				&ListPattern{generalElement: &AnyPattern{}},
				nil,
				false,
			},
			{
				&ListPattern{elements: []Pattern{&AnyPattern{}}},
				&ListPattern{generalElement: &AnyPattern{}},
				false,
			},

			{
				&ListPattern{elements: []Pattern{&ExactValuePattern{value: &Int{}}}},
				&ListPattern{elements: []Pattern{&ExactValuePattern{value: ANY}}},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.pattern), func(t *testing.T) {

				widened, ok := testCase.pattern.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.pattern.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
	})
}

func TestSymbolicTuplePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			pattern *TuplePattern
			value   SymbolicValue
			ok      bool
		}{

			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&Tuple{generalElement: ANY},
				false,
			},

			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&TuplePattern{generalElement: &AnyPattern{}},
				true,
			},
			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&TuplePattern{generalElement: &ExactValuePattern{value: &Int{}}},
				true,
			},
			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&TuplePattern{elements: []Pattern{}},
				true,
			},
			{
				&TuplePattern{elements: []Pattern{}},
				&TuplePattern{generalElement: &AnyPattern{}},
				false,
			},
			{
				&TuplePattern{elements: []Pattern{}},
				&TuplePattern{elements: []Pattern{&AnyPattern{}}},
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
			pattern *TuplePattern
			value   SymbolicValue
			ok      bool
		}{
			//[]any
			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&Tuple{elements: []SymbolicValue{}}, //empty tuple
				true,
			},
			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&Tuple{generalElement: &Int{}}, //[]int
				true,
			},
			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&Tuple{generalElement: ANY}, //[]any
				true,
			},
			{
				&TuplePattern{generalElement: &AnyPattern{}},
				&Tuple{elements: []SymbolicValue{ANY}}, //[any]
				true,
			},

			//[any]
			{
				&TuplePattern{elements: []Pattern{&AnyPattern{}}},
				&Tuple{generalElement: ANY}, //[any]
				false,
			},
			{
				&TuplePattern{elements: []Pattern{&AnyPattern{}}},
				&Tuple{elements: []SymbolicValue{&Int{}}}, //[string]
				true,
			},
			{
				&TuplePattern{elements: []Pattern{&AnyPattern{}}},
				&Tuple{elements: []SymbolicValue{}}, //empty tuple
				false,
			},

			//[]int
			{
				&TuplePattern{generalElement: &TypePattern{val: &Int{}}},
				&Tuple{elements: []SymbolicValue{}}, //empty tuple
				true,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: &Int{}}},
				&Tuple{generalElement: &Int{}}, //[]int
				true,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: &Int{}}},
				&Tuple{elements: []SymbolicValue{&Int{}}}, //[int]
				true,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: &Int{}}},
				&Tuple{generalElement: ANY}, //[]any
				false,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: &Int{}}},
				&Tuple{generalElement: &String{}}, //[]string
				false,
			},
			{
				&TuplePattern{generalElement: &TypePattern{val: &Int{}}},
				&Tuple{elements: []SymbolicValue{&Int{}, &String{}}}, //[int, string]
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))
			})
		}
	})

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			pattern *TuplePattern
			widened *TuplePattern
			ok      bool
		}{
			{
				&TuplePattern{generalElement: &AnyPattern{}},
				nil,
				false,
			},
			{
				&TuplePattern{elements: []Pattern{&AnyPattern{}}},
				&TuplePattern{generalElement: &AnyPattern{}},
				false,
			},

			{
				&TuplePattern{elements: []Pattern{&ExactValuePattern{value: &Int{}}}},
				&TuplePattern{elements: []Pattern{&ExactValuePattern{value: ANY}}},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.pattern), func(t *testing.T) {

				widened, ok := testCase.pattern.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.pattern.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
	})
}

func TestSymbolicUnionPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			pattern *UnionPattern
			value   SymbolicValue
			ok      bool
		}{
			{
				&UnionPattern{
					Cases: []Pattern{
						&AnyPattern{},
						&AnyPattern{},
					},
				},
				&UnionPattern{
					Cases: []Pattern{
						&ExactValuePattern{value: &Int{}},
						&ExactValuePattern{value: &String{}},
					},
				},
				true,
			},
			{
				&UnionPattern{
					Cases: []Pattern{
						&AnyPattern{},
						&AnyPattern{},
					},
				},
				&UnionPattern{
					Cases: []Pattern{
						&ExactValuePattern{value: &Int{}},
						&ExactValuePattern{value: &String{}},
						&ExactValuePattern{value: &Bool{}},
					},
				},
				false,
			},
			{
				&UnionPattern{
					Cases: []Pattern{
						&ExactValuePattern{value: &Int{}},
						&ExactValuePattern{value: &String{}},
					},
				},
				&UnionPattern{
					Cases: []Pattern{
						&AnyPattern{},
						&AnyPattern{},
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
					Cases: []Pattern{
						&ExactValuePattern{value: &Int{}},
						&ExactValuePattern{value: &String{}},
					},
				},
				&Int{},
				true,
			},
			{
				&UnionPattern{
					Cases: []Pattern{
						&ExactValuePattern{value: &Int{}},
						&ExactValuePattern{value: &String{}},
					},
				},
				&String{},
				true,
			},
			{
				&UnionPattern{
					Cases: []Pattern{
						&ExactValuePattern{value: &Int{}},
						&ExactValuePattern{value: &String{}},
					},
				},
				ANY,
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.pattern, "_", testCase.value), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))
			})
		}
	})

	t.Run("Widen() and IsWidenable()", func(t *testing.T) {
		cases := []struct {
			pattern *UnionPattern
			widened *UnionPattern
			ok      bool
		}{
			{
				&UnionPattern{
					Cases: []Pattern{
						&ExactValuePattern{value: &Int{}},
						&ExactValuePattern{value: &String{}},
					},
				},
				&UnionPattern{
					Cases: nil,
				},
				true,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.pattern), func(t *testing.T) {

				widened, ok := testCase.pattern.Widen()
				assert.Equal(t, testCase.ok, ok)
				assert.Equal(t, testCase.pattern.IsWidenable(), ok, "widen() is inconsistent with IsWidenable()")

				if !ok {
					assert.Nil(t, widened)
				} else {
					assert.Equal(t, testCase.widened, widened)
				}
			})
		}
	})
}

func TestSymbolicIntersectionPattern(t *testing.T) {
	//TODO
}

func TestSymbolicOptionPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &OptionPattern{}

		assert.True(t, pattern.Test(&OptionPattern{}))
		assert.False(t, pattern.Test(&Int{}))
		assert.False(t, pattern.Test(&Option{}))
	})

	t.Run("TestValue()", func(t *testing.T) {
		pattern := &OptionPattern{}

		assert.True(t, pattern.TestValue(&Option{}))
		assert.False(t, pattern.TestValue(&Int{}))
		assert.False(t, pattern.TestValue(&OptionPattern{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &OptionPattern{}

		assert.False(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicAnyStringPatternElement(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		pattern := &AnyStringPatternElement{}

		assert.True(t, pattern.Test(&AnyStringPatternElement{}))
		assert.False(t, pattern.Test(&Int{}))
		assert.False(t, pattern.Test(&String{}))
	})

	t.Run("TestValue() should return true for any symbolic Host", func(t *testing.T) {
		pattern := &AnyStringPatternElement{}

		assert.True(t, pattern.TestValue(&String{}))
		assert.False(t, pattern.TestValue(&Int{}))
		assert.False(t, pattern.TestValue(&AnyStringPatternElement{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		pattern := &AnyStringPatternElement{}

		assert.False(t, pattern.IsWidenable())

		widened, ok := pattern.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestTypePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		{
			_any := &TypePattern{val: ANY}

			assert.True(t, _any.Test(_any))
			assert.True(t, _any.Test(&TypePattern{val: &Int{}}))
			assert.False(t, _any.Test(&Int{}))
			assert.False(t, _any.Test(&String{}))
		}

		{
			specific := &TypePattern{val: &String{}}

			assert.True(t, specific.Test(specific))
			assert.True(t, specific.Test(&TypePattern{val: &String{}}))
			assert.False(t, specific.Test(&TypePattern{val: &Int{}}))
			assert.False(t, specific.Test(&Int{}))
			assert.False(t, specific.Test(&String{}))
		}

	})

	t.Run("TestValue() should return true for any symbolic Host", func(t *testing.T) {
		_any := &TypePattern{val: ANY}
		specific := &TypePattern{val: &String{}}

		assert.True(t, _any.TestValue(&String{}))
		assert.True(t, _any.TestValue(&Int{}))

		assert.True(t, specific.TestValue(&String{}))
		assert.False(t, specific.TestValue(&Int{}))
	})

	t.Run("Widen() & IsWidenable()", func(t *testing.T) {
		{
			_any := &TypePattern{val: ANY}

			assert.False(t, _any.IsWidenable())
			widened, ok := _any.Widen()
			assert.False(t, ok)
			assert.Nil(t, widened)
		}

		{
			specific := &TypePattern{val: &String{}}

			assert.False(t, specific.IsWidenable())
			widened, ok := specific.Widen()
			assert.False(t, ok)
			assert.Nil(t, widened)
		}
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
			assert.False(t, anyFnPatt.Test(&Int{}))
			assert.False(t, anyFnPatt.Test(&String{}))
		})

		t.Run("TestValue()", func(t *testing.T) {
			anyFnPatt := &FunctionPattern{}

			assert.True(t, anyFnPatt.TestValue(&Function{pattern: anyFnPatt}))
			assert.True(t, anyFnPatt.TestValue(&InoxFunction{
				node: &parse.FunctionPatternExpression{},
			}))
			assert.False(t, anyFnPatt.TestValue(&String{}))
			assert.False(t, anyFnPatt.TestValue(anyFnPatt))
		})

		t.Run("Widen() & IsWidenable()", func(t *testing.T) {
			anyFnPatt := &FunctionPattern{}

			assert.False(t, anyFnPatt.IsWidenable())
			widened, ok := anyFnPatt.Widen()
			assert.False(t, ok)
			assert.Nil(t, widened)
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
		state := newSymbolicState(NewSymbolicContext(nil), nil)
		state.ctx.AddNamedPattern("int", &TypePattern{val: &Int{}}, false)
		state.ctx.AddNamedPattern("str", &TypePattern{val: &String{}}, false)
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
				assert.False(t, fnPatt.Test(&Int{}))
				assert.False(t, fnPatt.Test(&String{}))
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
				assert.False(t, fnPatt.TestValue(&String{}))
			})

			t.Run("Widen() & IsWidenable()", func(t *testing.T) {
				node, _ := parse.ParseExpression(pattCode)
				fnPatt := utils.Must(symbolicEval(node, makeState())).(*FunctionPattern)

				assert.True(t, fnPatt.IsWidenable())
				widened, ok := fnPatt.Widen()
				assert.True(t, ok)
				assert.Equal(t, &FunctionPattern{}, widened)
			})
		})
	}

}
