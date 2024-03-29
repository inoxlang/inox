package symbolic

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
)

func TestSymbolicEval(t *testing.T) {
	prev := enableMultivalueCaching
	enableMultivalueCaching = false
	defer func() {
		enableMultivalueCaching = prev
	}()

	symbolicMap := func(ctx *Context, iterable Iterable, mapper Value) *List {
		var MAP_PARAM_NAMES = []string{"iterable", "mapper"}

		makeParams := func(result Value) *[]Value {
			return &[]Value{iterable, NewFunction(
				[]Value{iterable.IteratorElementValue()},
				nil,
				-1,
				false,
				[]Value{result},
			)}
		}

		switch m := mapper.(type) {
		case parse.Node:

		case *KeyList:
			obj := NewUnitializedObject()
			entries := map[string]Serializable{}
			for _, key := range m.Keys {
				entries[key] = ANY_SERIALIZABLE
			}

			InitializeObject(obj, entries, nil, false)
			return NewListOf(obj)
		case *PropertyName:
		case *GoFunction:
			result := m.Result().(Serializable) //not necessarily seriali
			ctx.SetSymbolicGoFunctionParameters(makeParams(result), MAP_PARAM_NAMES)
			return NewListOf(result)
		case *InoxFunction:
			result := m.Result()
			ctx.SetSymbolicGoFunctionParameters(makeParams(result), MAP_PARAM_NAMES)
			return NewListOf(m.Result().(Serializable))
		case *AstNode:
		case *Mapping:
		default:
			ctx.AddSymbolicGoFunctionError("invalid mapper argument")
		}

		return NewListOf(ANY_SERIALIZABLE)
	}

	t.Run("empty", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(``)
		_, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		//check scope data
		data, ok := state.symbolicData.GetGlobalScopeData(n, nil)
		if !assert.True(t, ok) {
			return
		}

		assert.Len(t, data.Variables, 1)

		// check context data
		{
			pattern := state.ctx.ResolveNamedPattern("int")

			data, ok := state.symbolicData.GetContextData(n, nil)
			if !assert.True(t, ok) {
				return
			}

			//ignore definition positions
			for i := range data.Patterns {
				data.Patterns[i].DefinitionPosition = parse.SourcePositionRange{}
			}

			assert.Contains(t, data.Patterns, NamedPatternData{
				Name:  "int",
				Value: pattern,
			})
		}
	})

	t.Run("quoted string literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`""`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, EMPTY_STRING, res)
	})

	t.Run("multiline string literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("``")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, EMPTY_STRING, res)
	})

	t.Run("flag literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("--verbose")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewOption("verbose", TRUE), res)
	})

	t.Run("option expression", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`--name="foo"`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewOption("name", NewString("foo")), res)
	})

	t.Run("property name literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(".name")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewPropertyName("name"), res)
	})

	t.Run("long value-path literal", func(t *testing.T) {
		t.Run("2 segments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(".name.len")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewLongValuePath(NewPropertyName("name"), NewPropertyName("len")), res)
		})
		t.Run("unterminated", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(".name.")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_LONG_VALUE_PATH, res)
		})
	})

	t.Run("byte slice literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("0x[01]")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, ANY_BYTE_SLICE, res)
	})

	t.Run("integer literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("1")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &Int{hasValue: true, value: 1}, res)
	})

	t.Run("integer range literal", func(t *testing.T) {
		t.Run("", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("1..2")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewIntRange(INT_1, INT_2, false), res)
		})

		t.Run("no upper bound", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("1..")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewIntRange(INT_1, MAX_INT, false), res)
		})
	})

	t.Run("float range literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("1.0..2.0")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewIncludedEndFloatRange(FLOAT_1, FLOAT_2), res)
	})

	t.Run("quantity range literal", func(t *testing.T) {
		getQuantity := extData.GetQuantity
		ToSymbolicValue := extData.ToSymbolicValue

		defer func() {
			extData.GetQuantity = getQuantity
			extData.ToSymbolicValue = ToSymbolicValue
		}()
		extData.GetQuantity = func(values []float64, units []string) (any, error) {
			if units[0] == "x" {
				return NewInt(int64(values[0])), nil
			}
			if units[0] == "B" {
				return NewByteCount(int64(values[0])), nil
			}
			panic(ErrUnreachable)
		}
		extData.ToSymbolicValue = func(_ ConcreteContext, v any, wide bool) (Value, error) {
			return v.(Value), nil
		}

		t.Run("no upper bound", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("1B..")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewQuantityRange(ANY_BYTECOUNT), res)
		})

		t.Run("upper bound has invalid type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("1B..1x")
			res, err := symbolicEval(n, state)

			upperBound := parse.FindNodes(n, (*parse.QuantityLiteral)(nil), nil)[1]

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(upperBound, state, UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_OF_SAME_TYPE_AS_LOWER_BOUND),
			}, state.errors())
			assert.Equal(t, NewQuantityRange(ANY_BYTECOUNT), res)
		})
	})

	t.Run("year literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("2020y-UTC")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		expectedYear, _, _ := parse.ParseDateLikeLiteral([]byte("2020y-UTC"))
		assert.Equal(t, NewYear(expectedYear), res)
	})

	t.Run("date literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("2020y-1mt-1d-UTC")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		expectedDate, _, _ := parse.ParseDateLikeLiteral([]byte("2020y-1mt-1d-UTC"))
		assert.Equal(t, NewDate(expectedDate), res)
	})

	t.Run("datetime literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("2020y-1mt-1d-5h-3m-UTC")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		expectedDateTime, _, _ := parse.ParseDateLikeLiteral([]byte("2020y-1mt-1d-5h-3m-UTC"))
		assert.Equal(t, NewDateTime(expectedDateTime), res)
	})

	t.Run("path literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("/")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewPath("/"), res)
	})

	t.Run("path pattern literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("%/...")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewPathPattern("/..."), res)
	})

	t.Run("path pattern expression", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("a = 1; return %/{a}/...")
		res, err := symbolicEval(n, state)

		pathPatternExpr := parse.FindNode(n, (*parse.PathPatternExpression)(nil), nil)
		if !assert.NotNil(t, pathPatternExpr) {
			return
		}

		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewPathPatternFromNode(pathPatternExpr, &parse.Chunk{}), res)
	})

	t.Run("scheme literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("https://")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewScheme("https://"), res)
	})

	t.Run("url literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("https://example.com/")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewUrl("https://example.com/"), res)
	})

	t.Run("url pattern literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("%https://example.com/...")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewUrlPattern("https://example.com/..."), res)
	})

	t.Run("host literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("https://example.com")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewHost("https://example.com"), res)
	})

	t.Run("host pattern literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("%https://**.com")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewHostPattern("https://**.com"), res)
	})

	t.Run("list literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("[]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &List{elements: []Serializable{}}, res)
		})

		t.Run("singe element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("[int]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(ANY_INT), res)
		})

		t.Run("readonly", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(list readonly []int){
					return list
				}
				return f([])
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, EMPTY_READONLY_LIST, res)
		})

		t.Run("readonly lists should not have non-readonly elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(list readonly []{}){
					return list
				}
				obj = {}
				return f([obj])
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			identLiteral := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), func(n *parse.IdentifierLiteral) bool {
				return n.Name == "obj"
			})[1]

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(identLiteral, state, fmtUnexpectedElemInListofValues(EMPTY_OBJECT, EMPTY_READONLY_OBJECT)),
			}, state.errors())

			if !assert.IsType(t, (*List)(nil), res) {
				return
			}

			list := res.(*List)
			assert.True(t, list.readonly)
		})

		t.Run("non-serializable element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("[go do {}]")
			elemNode := parse.FindNode(n, (*parse.SpawnExpression)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(elemNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, NewList(ANY_SERIALIZABLE), res)
		})

		t.Run("non-watchable mutable element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("[val]")
			state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
			elemNode := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(elemNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE),
			}, state.errors())
			assert.Equal(t, NewList(ANY_SERIALIZABLE), res)
		})

		t.Run("two elements of different type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`[int, "a"]`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(ANY_INT, NewString("a")), res)
		})

		t.Run("type annotation", func(t *testing.T) {

			t.Run("element of invalid type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("[]%int[true]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListAnnotated(TRUE, state.ctx.ResolveNamedPattern("int"))),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("spread element should be a list", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("[]%int[...true]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(elemNode, state, SPREAD_ELEMENT_SHOULD_BE_A_LIST),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("spread element of invalid type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("l = [true]; return []%int[...l]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				elemNode := parse.FindNode(n, (*parse.ElementSpreadElement)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListAnnotated(TRUE, state.ctx.ResolveNamedPattern("int"))),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("element of valid type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("[]%int[1]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})
		})

		t.Run("spread element should be a list", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("[1, ...true, 2]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(elemNode, state, SPREAD_ELEMENT_SHOULD_BE_A_LIST),
			}, state.errors())
			assert.Equal(t, NewList(NewInt(1), NewInt(2)), res)
		})

		t.Run("spread element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("l = [true]; return [1, ...l]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(NewInt(1), TRUE), res)
		})

	})

	t.Run("tuple literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("#[]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Tuple{elements: []Serializable{}}, res)
		})

		t.Run("singe element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("#[int]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewTuple(ANY_INT), res)
		})

		t.Run("non-serializable element", func(t *testing.T) {
			//TODO
		})

		t.Run("mutable element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("#[{}]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			elemNode := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE),
			}, state.errors())
			assert.Equal(t, NewTuple(ANY_SERIALIZABLE), res)
		})

		t.Run("type annotation", func(t *testing.T) {
			t.Run("type annotation and element of invalid type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("#[]%int[true]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInTupleAnnotated(TRUE, state.ctx.ResolveNamedPattern("int"))),
				}, state.errors())
				assert.Equal(t, NewTupleOf(ANY_INT), res)
			})

			t.Run("spread element should be a tuple", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("#[]%int[...true]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(elemNode, state, SPREAD_ELEMENT_SHOULD_BE_A_TUPLE),
				}, state.errors())
				assert.Equal(t, NewTupleOf(ANY_INT), res)
			})

			t.Run("spread element of invalid type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("l = #[true]; return #[]%int[...l]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				elemNode := parse.FindNode(n, (*parse.ElementSpreadElement)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInTupleAnnotated(TRUE, state.ctx.ResolveNamedPattern("int"))),
				}, state.errors())
				assert.Equal(t, NewTupleOf(ANY_INT), res)
			})

			t.Run("element of valid type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("#[]%int[1]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewTupleOf(ANY_INT), res)
			})
		})

		t.Run("without type annotation", func(t *testing.T) {
			t.Run("spread element should be a tuple", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("#[...true]")
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(elemNode, state, SPREAD_ELEMENT_SHOULD_BE_A_TUPLE),
				}, state.errors())
				assert.Equal(t, NewTuple(), res)
			})
		})

	})

	t.Run("dictionary literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(":{}")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewDictionary(map[string]Serializable{}, map[string]Serializable{}), res)
		})

		t.Run("single entry", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`:{./a: "b"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewDictionary(map[string]Serializable{
				"./a": NewString("b"),
			}, map[string]Serializable{
				"./a": NewPath("./a"),
			}), res)
		})

		t.Run("non-serializable value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`:{./a: go do {}}`)
			entryValueNode := parse.FindNode(n, (*parse.SpawnExpression)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(entryValueNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, NewDictionary(map[string]Serializable{
				"./a": ANY_SERIALIZABLE,
			}, map[string]Serializable{
				"./a": NewPath("./a"),
			}), res)
		})

		t.Run("non-watchable mutable value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`:{./a: val}`)
			state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
			entryValueNode := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(entryValueNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE),
			}, state.errors())
			assert.Equal(t, NewDictionary(map[string]Serializable{
				"./a": ANY_SERIALIZABLE,
			}, map[string]Serializable{
				"./a": NewPath("./a"),
			}), res)
		})

		t.Run("variable key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`v = ./a; return :{v: "b"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewDictionary(map[string]Serializable{
				"v": NewString("b"),
			}, map[string]Serializable{
				"v": NewPath("./a"),
			}), res)
		})

		t.Run("multivalue key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`:{v: "b"}`)
			state.setGlobal("v", NewMultivalue(ANY_INT, ANY_BOOL), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewDictionary(map[string]Serializable{
				"v": NewString("b"),
			}, map[string]Serializable{
				"v": AsSerializableChecked(NewMultivalue(ANY_INT, ANY_BOOL)),
			}), res)
		})
	})

	t.Run("constant declarations", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			const (
				A = int
			)

			return A
		`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, ANY_INT, res)

		//check definition position data
		idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.IdentifierLiteral)(nil), nil)
		definitionIdent := idents[0]
		returnIdent := idents[2]
		returnIdentAncestors := ancestorChains[2]

		pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, definitionIdent.Span, pos.Span)
	})

	t.Run("local variable declaration", func(t *testing.T) {
		t.Run("no type annotation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a = int; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			//check definition position data
			idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.IdentifierLiteral)(nil), nil)
			definitionIdent := idents[0]
			returnIdent := idents[2]
			returnIdentAncestors := ancestorChains[2]

			pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, definitionIdent.Span, pos.Span)
		})

		t.Run("value not assignable to type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %str = int; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(ANY_INT, &TypePattern{val: ANY_STR_LIKE})),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("missing value", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var a 
				return a
			`, nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors()) //there is already a parsing error
			assert.Equal(t, ANY, res)
		})

		t.Run("missing value after annotation", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var a int
				return a
			`, nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors()) //there is already a parsing error
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("annotation should be a pattern", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				myint = 1
				var a ($myint) = 1
				return a
			`, nil)

			typeAnnotation := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil).Type

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, NewInt(1), res)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(typeAnnotation, state, VARIABLE_DECL_ANNOTATION_MUST_BE_A_PATTERN),
			}, state.errors())
		})

		t.Run("value not assignable to type (deep mismatch: object property)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %{a: str} = {a: 1}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			objectProp := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectProp.Value, state,
					fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: record property)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a #{a: str} = #{a: 1}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			objectProp := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectProp.Value, state,
					fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: dictionary entry)", func(t *testing.T) {
			t.Skip("TODO: rewrite type annotation with a dictionary pattern")

			n, state := MakeTestStateAndChunk(`
				var a %(:{"a": "str"}) = :{"a": 1}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			intLiteral := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiteral, state,
					fmtNotAssignableToEntryOfExpectedValue(NewInt(1), NewString("str"))),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: list element)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a []str = [1]; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			intLiteral := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiteral, state,
					fmtUnexpectedElemInListofValues(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: tuple element)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a #[]str = #[1]; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			intLiteral := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiteral, state,
					fmtUnexpectedElemInListofValues(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (unprefixed named pattern)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a str = int; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(ANY_INT, &TypePattern{val: ANY_STR_LIKE})),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("object (ability to hold static data) is not assignable to type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %str = {}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(NewEmptyObject(), &TypePattern{val: ANY_STR_LIKE}),
				),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value assignable to type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var obj %{name: %| %str | %int} = {name: int}; 
				return obj
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": ANY_INT,
				},
				static: map[string]Pattern{
					"name": &UnionPattern{
						cases: []Pattern{
							state.ctx.ResolveNamedPattern("str"),
							state.ctx.ResolveNamedPattern("int"),
						},
					},
				},
			}, res)
		})

		t.Run("object assignable to wide type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var obj object = {name: str}; 
				return obj
			`)
			state.setGlobal("str", ANY_STR_LIKE, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": ANY_STR_LIKE,
				},
				static: map[string]Pattern{
					"name": state.ctx.ResolveNamedPattern("str"),
				},
			}, res)
		})

		t.Run("multivalue LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(v []int | []str){
					var a []int | []str = v; 
					return a
				}
			`)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			argType := NewMultivalue(
				NewListOf(ANY_INT), NewListOf(ANY_STR_LIKE),
			)

			expectedFn := &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{argType},
				parameterNames: []string{"v"},
				result:         argType,
			}
			assert.Equal(t, expectedFn, res)
		})
	})

	t.Run("global variable declaration", func(t *testing.T) {
		t.Run("no type annotation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a = int; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			//check definition position data
			idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.IdentifierLiteral)(nil), nil)
			definitionIdent := idents[0]
			returnIdent := idents[2]
			returnIdentAncestors := ancestorChains[2]

			pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, definitionIdent.Span, pos.Span)
		})

		t.Run("value not assignable to type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a %str = int; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.GlobalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(ANY_INT, &TypePattern{val: ANY_STR_LIKE})),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("missing value", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				globalvar a 
				return a
			`, nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors()) //there is already a parsing error
			assert.Equal(t, ANY, res)
		})

		t.Run("missing value after annotation", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				globalvar a int
				return a
			`, nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors()) //there is already a parsing error
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("annotation should be a pattern", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				myint = 1
				globalvar a ($myint) = 1
				return a
			`, nil)

			typeAnnotation := parse.FindNode(n, (*parse.GlobalVariableDeclaration)(nil), nil).Type

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, NewInt(1), res)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(typeAnnotation, state, VARIABLE_DECL_ANNOTATION_MUST_BE_A_PATTERN),
			}, state.errors())
		})

		t.Run("value not assignable to type (deep mismatch: object property)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a %{a: str} = {a: 1}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			objectProp := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectProp.Value, state,
					fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: record property)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a #{a: str} = #{a: 1}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			objectProp := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectProp.Value, state,
					fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: dictionary entry)", func(t *testing.T) {
			t.Skip("TODO: rewrite type annotation with a dictionary pattern")

			n, state := MakeTestStateAndChunk(`
				globalvar a %(:{"a": "str"}) = :{"a": 1}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			intLiteral := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiteral, state,
					fmtNotAssignableToEntryOfExpectedValue(NewInt(1), NewString("str"))),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: list element)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a []str = [1]; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			intLiteral := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiteral, state,
					fmtUnexpectedElemInListofValues(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: tuple element)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a #[]str = #[1]; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			intLiteral := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiteral, state,
					fmtUnexpectedElemInListofValues(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (unprefixed named pattern)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a str = int; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.GlobalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(ANY_INT, &TypePattern{val: ANY_STR_LIKE})),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("object (ability to hold static data) is not assignable to type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar a %str = {}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.GlobalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(NewEmptyObject(), &TypePattern{val: ANY_STR_LIKE}),
				),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value assignable to type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar obj %{name: %| %str | %int} = {name: int}; 
				return obj
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": ANY_INT,
				},
				static: map[string]Pattern{
					"name": &UnionPattern{
						cases: []Pattern{
							state.ctx.ResolveNamedPattern("str"),
							state.ctx.ResolveNamedPattern("int"),
						},
					},
				},
			}, res)
		})

		t.Run("object assignable to wide type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar obj object = {name: str}; 
				return obj
			`)
			state.setGlobal("str", ANY_STR_LIKE, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": ANY_STR_LIKE,
				},
				static: map[string]Pattern{
					"name": state.ctx.ResolveNamedPattern("str"),
				},
			}, res)
		})

		t.Run("multivalue LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(v %| %[]%int | %[]%str){
					globalvar a %| %[]%int | %[]%str = v; 
					return a
				}
			`)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			argType := NewMultivalue(
				NewListOf(ANY_INT), NewListOf(ANY_STR_LIKE),
			)

			expectedFn := &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{argType},
				parameterNames: []string{"v"},
				result:         argType,
			}
			assert.Equal(t, expectedFn, res)
		})
	})

	t.Run("variable assignment", func(t *testing.T) {

		t.Run("local variable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = []
				return $v
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(), res)
		})

		t.Run("RHS has type incompatible with explicit static type of the variable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = %| %int | %str
				var v p = int
				v = true
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[2]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(TRUE, &UnionPattern{
					cases: []Pattern{
						state.ctx.ResolveNamedPattern("int"),
						state.ctx.ResolveNamedPattern("str"),
					},
				})),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("RHS has type incompatible with implicit static type of the variable (int)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var v = 1
				v = true
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(TRUE, &TypePattern{val: ANY_INT})),
			}, state.errors())
			assert.Equal(t, NewInt(1), res)
		})

		t.Run("RHS has type incompatible with implicit static type of the variable ([]int)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var v = [1, 2, 3]
				v = true
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)

			staticType := NewListPatternOf(&TypePattern{val: ANY_INT})

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(TRUE, staticType)),
			}, state.errors())
			assert.Equal(t, NewList(NewInt(1), NewInt(2), NewInt(3)), res)
		})

		t.Run("+= assignment, LHS has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = "a"
				v += int
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT),
			}, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("+= assignment, RHS has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				v += "a"
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1].(*parse.Assignment)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("variable LHS in function: a local variable outside of the function already has the same name", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = ""
				fn f(){
					a = int
					return a
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("multi value RHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v %| %int | %str){
					var list %| %int | %str = 1
					list = v
				}
			`)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})
	})

	t.Run("property assignement", func(t *testing.T) {
		t.Run("set new property of an object: member expr LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {}
				$obj.name = "foo"
				return obj
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
			}, res)
		})

		t.Run("set new property of an object: identifier member LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {}
				obj.name = "foo"
				return obj
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
			}, res)
		})

		t.Run("set new property of an object with non-serializable value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {}
				lthread = go do {}
				$obj.lthread = lthread
				return obj
			`)
			assignment := n.Statements[2]

			res, err := symbolicEval(n, state)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"lthread": ANY_SERIALIZABLE,
				},
			}, res)
		})

		t.Run("set new property of an object with non-watchable mutable value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {}
				$obj.prop = val
				return obj
			`)
			state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
			assignment := n.Statements[1]

			res, err := symbolicEval(n, state)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"prop": ANY_SERIALIZABLE,
				},
			}, res)
		})

		t.Run("set new property of an object with non-serializable value: identifier member LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {}
				lthread = go do {}
				obj.lthread = lthread
				return obj
			`)
			assignment := n.Statements[2]

			res, err := symbolicEval(n, state)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, &Object{entries: map[string]Serializable{}}, res)
		})

		t.Run("set new property of an object with non-watchable mutable value: identifier member LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {}
				obj.lthread = val
				return obj
			`)
			state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
			assignment := n.Statements[1]

			res, err := symbolicEval(n, state)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"lthread": ANY_SERIALIZABLE,
				},
			}, res)
		})

		t.Run("existing property of an object: RHS has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {name: "foo"}
				$obj.name = int
				return obj
			`)
			assignment := n.Statements[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(ANY_INT, &TypePattern{val: ANY_STRING})),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STRING.Static(),
				},
			}, res)
		})

		t.Run("existing property of an object: RHS has incompatible type, identifier member LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {name: "foo"}
				obj.name = int
				return obj
			`)
			assignment := n.Statements[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(ANY_INT, &TypePattern{val: ANY_STRING})),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STRING.Static(),
				},
			}, res)
		})

		t.Run("existing property of an object: RHS has incompatible type, identifier member LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {name: "foo"}
				obj.name = int
				return obj
			`)
			assignment := n.Statements[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(ANY_INT, &TypePattern{val: ANY_STRING})),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STRING.Static(),
				},
			}, res)
		})

		t.Run("existing property of an object: RHS has type compatible with static type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var obj %{name: %| %str | %int } = {name: "foo"}
				$obj.name = int
				return obj
			`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": ANY_INT,
				},
				static: map[string]Pattern{
					"name": &UnionPattern{
						cases: []Pattern{
							state.ctx.ResolveNamedPattern("str"),
							state.ctx.ResolveNamedPattern("int"),
						},
					},
				},
			}, res)
		})

		t.Run("existing property of an object: RHS has type incompatible with static type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var obj %{name: %| %str | %int } = {name: "foo"}
				$obj.name = true
				return obj
			`)
			res, err := symbolicEval(n, state)

			assignment := n.Statements[1]

			propType := &UnionPattern{
				cases: []Pattern{
					state.ctx.ResolveNamedPattern("str"),
					state.ctx.ResolveNamedPattern("int"),
				},
			}

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(TRUE, propType)),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": propType,
				},
			}, res)
		})

		t.Run("+= assignment, propert LHS (member expression) has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {name: "foo"}
				$obj.name += int
				return obj
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STRING.Static(),
				},
			}, res)
		})

		t.Run("+= assignment, propert LHS (ident member expression) has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {name: "foo"}
				obj.name += int
				return obj
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STRING.Static(),
				},
			}, res)
		})

		t.Run("+= assignment, field LHS (ident member expression) has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a bool
				}
				ptr = new MyStruct
				ptr.a += int
			`)
			_, err := symbolicEval(n, state)
			assignement := n.Statements[2]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT),
			}, state.errors())
		})

		t.Run("+= assignment, field LHS (member expression) has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a bool
				}
				ptr = new MyStruct
				$ptr.a += int
			`)
			_, err := symbolicEval(n, state)
			assignement := n.Statements[2]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT),
			}, state.errors())
		})

		t.Run("+= assignment, RHS has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {count: int}
				$obj.count += "a"
				return obj
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1].(*parse.Assignment)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"count": ANY_INT,
				},
				static: map[string]Pattern{
					"count": ANY_INT.Static(),
				},
			}, res)
		})

	})

	t.Run("object literal", func(t *testing.T) {
		t.Run("basic", func(t *testing.T) {

			n, state := MakeTestStateAndChunk(`{"name": "foo"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STRING.Static(),
				},
			}, res)
		})

		t.Run("type annotation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{"name" %| %str | %int : "foo"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": &UnionPattern{
						cases: []Pattern{
							state.ctx.ResolveNamedPattern("str"),
							state.ctx.ResolveNamedPattern("int"),
						},
					},
				},
			}, res)
		})

		t.Run("type annotation with incompatible value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{"name" %str : $int}`)
			res, err := symbolicEval(n, state)

			valueNode := parse.FindNode(state.Module.mainChunk.Node, (*parse.Variable)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(valueNode, state, fmtNotAssignableToPropOfType(ANY_INT, state.ctx.ResolveNamedPattern("str").SymbolicValue())),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": ANY_STR_LIKE,
				},
				static: map[string]Pattern{
					"name": state.ctx.ResolveNamedPattern("str"),
				},
			}, res)
		})

		t.Run("object in object", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{v: {}}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"v": &Object{
						entries: map[string]Serializable{},
					},
				},
				static: map[string]Pattern{
					"v": NewEmptyObject().Static(),
				},
			}, res)
		})

		t.Run("one property without a key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{1}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"": NewList(INT_1),
				},
				static: map[string]Pattern{
					"": NewListPatternOf(&TypePattern{val: ANY_INT}),
				},
			}, res)
		})

		t.Run("two properties without a key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{1, 2}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"": NewList(INT_1, INT_2),
				},
				static: map[string]Pattern{
					"": NewListPatternOf(&TypePattern{val: ANY_INT}),
				},
			}, res)
		})

		t.Run("property with serializable multivalue value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{a: mv}`)

			mv := NewMultivalue(INT_1, INT_2)
			state.setGlobal("mv", mv, GlobalVar)

			serializableMv := AsSerializable(mv).(Serializable)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"a": serializableMv,
				},
				static: map[string]Pattern{
					"a": &TypePattern{val: serializableMv},
				},
			}, res)
		})

		t.Run("missing value of property", func(t *testing.T) {
			n, state, err := _makeStateAndChunk(`{v:}`, nil)

			if !assert.Error(t, err) {
				return
			}

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"v": ANY_SERIALIZABLE,
				},
				static: map[string]Pattern{
					"v": getStatic(ANY_SERIALIZABLE),
				},
			}, res)
		})

		t.Run("invalid spread element", func(t *testing.T) {
			n, state, err := _makeStateAndChunk(`obj = {b: 2}; return {a: 1, ...obj, c: 3}`, nil)
			if !assert.Error(t, err) {
				return
			}

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"a": NewInt(1),
					"c": NewInt(3),
				},
				static: map[string]Pattern{
					"a": ANY_INT.Static(),
					"c": ANY_INT.Static(),
				},
			}, res)
		})

		t.Run("non-serializable values not allowed in initialization", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{lthread: go do {}}`)
			propNode := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"lthread": ANY_SERIALIZABLE,
				},
				static: map[string]Pattern{
					"lthread": getStatic(ANY_SERIALIZABLE),
				},
			}, res)
		})

		t.Run("non-watchable mutable values not allowed in initialization", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{lthread: val}`)
			state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
			propNode := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"lthread": ANY_SERIALIZABLE,
				},
				static: map[string]Pattern{
					"lthread": getStatic(ANY_SERIALIZABLE),
				},
			}, res)
		})

		t.Run("_constraints_", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{
				a: 1
				b: 2

				_constraints_ {
					(self.a < self.b)
				}
			}`)
			res, err := symbolicEval(n, state)

			binExpr := parse.FindNode(state.Module.mainChunk.Node, (*parse.BinaryExpression)(nil), nil)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"a": NewInt(1),
					"b": NewInt(2),
				},
				static: map[string]Pattern{
					"a": ANY_INT.Static(),
					"b": ANY_INT.Static(),
				},
				complexPropertyConstraints: []*ComplexPropertyConstraint{
					{
						Properties: []string{"a", "b"},
						Expr:       binExpr,
					},
				},
			}, res)
		})

		t.Run("readonly", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(obj readonly {}){
					return obj
				}
				return f({})
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries:  map[string]Serializable{},
				readonly: true,
			}, res)
		})

		t.Run("readonly objects should not have lifetime jobs", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(obj readonly {}){
					return obj
				}
				return f({
					lifetimejob #job {

					}
				})
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			lifetimeJobExpr := parse.FindNode(n, (*parse.LifetimejobExpression)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(lifetimeJobExpr, state, LIFETIME_JOBS_NOT_ALLOWED_IN_READONLY_OBJECTS),
			}, state.errors())

			if !assert.IsType(t, (*Object)(nil), res) {
				return
			}

			obj := res.(*Object)
			assert.True(t, obj.readonly)
		})

		t.Run("readonly objects should not have non-readonly property values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(obj readonly {}){
					return obj
				}
				list = []
				return f({
					a: #{}
					b: list # not readonly
				})
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			prop := parse.FindObjPropWithName(n, "b")

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(prop.Key, state, PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE),
			}, state.errors())

			if !assert.IsType(t, (*Object)(nil), res) {
				return
			}

			obj := res.(*Object)
			assert.True(t, obj.readonly)
		})
	})

	t.Run("record", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewEmptyRecord(), res)
		})

		t.Run("property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{"name": "foo"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
			}, res)
		})

		t.Run("one property without a key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{1}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"": NewTuple(INT_1),
				},
			}, res)
		})

		t.Run("two properties without a key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{1, 2}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"": NewTuple(INT_1, INT_2),
				},
			}, res)
		})

		t.Run("property with serializable multivalue value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{a: mv}`)

			mv := NewMultivalue(INT_1, INT_2)
			state.setGlobal("mv", mv, GlobalVar)

			serializableMv := AsSerializable(mv).(Serializable)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"a": serializableMv,
				},
			}, res)
		})

		t.Run("missing value of property", func(t *testing.T) {
			n, state, err := _makeStateAndChunk(`#{v:}`, nil)

			if !assert.Error(t, err) {
				return
			}

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"v": ANY_SERIALIZABLE,
				},
			}, res)
		})

		t.Run("mutable property value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{"a": {}}`)
			res, err := symbolicEval(n, state)
			valueNode := n.Statements[0].(*parse.RecordLiteral).Properties[0].Value

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(valueNode, state, fmtValuesOfRecordShouldBeImmutablePropHasMutable("a")),
			}, state.errors())
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"a": ANY_SERIALIZABLE,
				},
			}, res)
		})

		t.Run("mutable element value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{{}}`)
			res, err := symbolicEval(n, state)
			valueNode := n.Statements[0].(*parse.RecordLiteral).Properties[0].Value

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(valueNode, state, INVALID_ELEM_ELEMS_OF_RECORD_SHOULD_BE_IMMUTABLE),
			}, state.errors())
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"": NewTuple(ANY_SERIALIZABLE),
				},
			}, res)
		})

		t.Run("non-serializable values not allowed in initialization", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`#{suite: testsuite {}}`)
			propNode := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, &Record{
				entries: map[string]Serializable{
					"suite": ANY_SERIALIZABLE,
				},
			}, res)
		})

	})
	t.Run("member expression", func(t *testing.T) {
		t.Run("object property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {"name": "foo"}
				return $v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("foo"), res)
		})

		t.Run("record property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = #{"name": "foo"}
				return $v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("foo"), res)
		})

		t.Run("struct field", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a int
				}

				ptr = new MyStruct
				return $ptr.a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("inexisting property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {}
				return v.name
			`)
			memberExpr := n.Statements[1].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfDoesNotExist("name", NewEmptyObject(), "")),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("inexisting property, optional member expression", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {}
				return v.?name
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("optional member expression are not allowed on struct field", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a int
				}

				ptr = new MyStruct
				return $ptr.?a
			`)
			memberExpr := n.Statements[2].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, OPTIONAL_MEMBER_EXPRS_NOT_ALLOWED_FOR_STRUCT_FIELDS),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("optional property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg %{name?: %str}){
					return $arg.name
				}
			`)
			memberExpr := parse.FindNode(n, (*parse.MemberExpression)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropertyIsOptionalUseOptionalMembExpr("name")),
			}, state.errors())
		})

		t.Run("inexisting property of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			goVal := ANY_LTHREAD
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfDoesNotExist("XYZ", goVal, "")),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("existing method of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.cancel
			`)

			goVal := ANY_LTHREAD
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.NotNil(t, res)
		})

		t.Run("inexisting method of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			goVal := ANY_LTHREAD
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfDoesNotExist("XYZ", goVal, "")),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("multivalue: 2 objects with same property type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(v %| %{a: %int} | %{a: %int, b: %str}) {
					return v.a
				}
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res.(*InoxFunction).result)
		})

		t.Run("multivalue: 2 objects with different property type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(v %| %{a: %int} | %{a: %str}) {
					return v.a
				}
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(ANY_INT, ANY_STR_LIKE), res.(*InoxFunction).result)
		})

		t.Run("unterminated", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = {"name": "foo"}
				return $v.
			`, nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

	})

	t.Run("computed member expression", func(t *testing.T) {
		t.Run("property name is not a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {"name": "foo"}
				return v.($int)
			`)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			propNameNode := parse.FindNode(n, (*parse.Variable)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propNameNode, state, fmtComputedPropNameShouldBeAStringNotA(ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})
	})

	t.Run("dynamic member expression", func(t *testing.T) {
		t.Run("object", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {"name": "foo"}
				return $v.<name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewDynamicValue(NewString("foo")), res)
		})

		t.Run("record", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = #{"name": "foo"}
				return $v.<name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewDynamicValue(NewString("foo")), res)
		})

		t.Run("dynamic value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {a: {b: int}}
				return $v.<a.b
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewDynamicValue(ANY_INT), res)
		})

		t.Run("inexisting field of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.<XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			goVal := ANY_LTHREAD
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfDoesNotExist("XYZ", goVal, "")),
			}, state.errors())
			assert.Equal(t, NewAnyDynamicValue(), res)
		})

		t.Run("existing method of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.<cancel
			`)

			goVal := ANY_LTHREAD
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.NotNil(t, res)
		})

		t.Run("inexisting method of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.<XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			goVal := ANY_LTHREAD
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfDoesNotExist("XYZ", goVal, "")),
			}, state.errors())
			assert.Equal(t, NewAnyDynamicValue(), res)
		})
	})

	t.Run("identifier member expression", func(t *testing.T) {
		t.Run("object property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {"name": "foo"}
				return v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("foo"), res)
		})

		t.Run("struct field", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a int
				}

				ptr = new MyStruct
				return ptr.a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("unterminated (0 property names)", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = {"name": "foo"}
				return v.
			`, nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("unterminated (int property names)", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = {"a": {"b": "foo"}}
				return v.a.
			`, nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("optional property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg %{name?: %str}){
					return arg.name
				}
			`)
			memberExpr := parse.FindNode(n, (*parse.IdentifierMemberExpression)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropertyIsOptionalUseOptionalMembExpr("name")),
			}, state.errors())
		})
	})

	t.Run("index expression", func(t *testing.T) {
		t.Run("index is not an integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v["0"]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtIndexIsNotAnIntButA(NewString("0"))),
			}, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("indexed is not indexable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = 0
				return $v[0]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtXisNotIndexable(NewInt(0))),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v[0]
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("start index is out of bounds (negative)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v[-1]
			`)
			res, err := symbolicEval(n, state)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, INDEX_IS_OUT_OF_BOUNDS),
			}, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("start index is out of bounds (positive)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v[1]
			`)
			res, err := symbolicEval(n, state)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, INDEX_IS_OUT_OF_BOUNDS),
			}, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("list of unknown length", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $v[0]
			`)
			state.setGlobal("v", &List{generalElement: ANY_STRING}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_STRING, res)
		})

		t.Run("rune slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $v[0]
			`)
			state.setGlobal("v", ANY_RUNE_SLICE, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_RUNE, res)
		})

		t.Run("byte slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $v[0]
			`)
			state.setGlobal("v", ANY_BYTE_SLICE, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BYTE, res)
		})

	})

	t.Run("slice expression", func(t *testing.T) {
		t.Run("start index is not an integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v["0":]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtStartIndexIsNotAnIntButA(NewString("0"))),
			}, state.errors())
			assert.Equal(t, NewListOf(ANY_SERIALIZABLE), res)
		})

		t.Run("end index is not an integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v[0:"1"]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtEndIndexIsNotAnIntButA(NewString("1"))),
			}, state.errors())
			assert.Equal(t, NewListOf(ANY_SERIALIZABLE), res)
		})

		t.Run("indexed it not a sequence", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = 0
				return $v[0:]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtSequenceExpectedButIs(NewInt(0))),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v[0:]
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewListOf(ANY_SERIALIZABLE), res)
		})

		t.Run("start index is out of bounds (negative)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v[-1:]
			`)
			res, err := symbolicEval(n, state)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, START_INDEX_IS_OUT_OF_BOUNDS),
			}, state.errors())
			assert.Equal(t, NewListOf(ANY_SERIALIZABLE), res)
		})

		t.Run("start index is out of bounds (positive)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a"]
				return $v[1:]
			`)
			res, err := symbolicEval(n, state)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, START_INDEX_IS_OUT_OF_BOUNDS),
			}, state.errors())
			assert.Equal(t, NewListOf(ANY_SERIALIZABLE), res)
		})

		t.Run("end index should less or equal to start index", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = ["a", "b"]
				return $v[1:0]
			`)
			res, err := symbolicEval(n, state)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, END_INDEX_SHOULD_BE_LESS_OR_EQUAL_START_INDEX),
			}, state.errors())
			assert.Equal(t, NewListOf(ANY_SERIALIZABLE), res)
		})

		t.Run("list of unknown length", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $v[0:]
			`)
			state.setGlobal("v", &List{generalElement: ANY_STRING}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &List{generalElement: ANY_STRING}, res)
		})

		t.Run("rune slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $v[0:]
			`)
			state.setGlobal("v", ANY_RUNE_SLICE, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_RUNE_SLICE, res)
		})

		t.Run("byte slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $v[0:]
			`)
			state.setGlobal("v", ANY_BYTE_SLICE, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BYTE_SLICE, res)
		})

	})

	t.Run("extraction expression", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {a: int, b: true, c: "a"}
				return $v.{a, b}
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"a": ANY_INT,
					"b": TRUE,
				},
				static: map[string]Pattern{
					"a": &TypePattern{val: ANY_INT},
					"b": &TypePattern{val: ANY_BOOL},
				},
			}, res)
		})
		t.Run("dynamic values are not supported", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {a: {b: 1}}
				return $v.<a.{b}
			`)
			dynamicMemberExpr := parse.FindNode(n, (*parse.DynamicMemberExpression)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(dynamicMemberExpr, state, EXTRACTION_DOES_NOT_SUPPORT_DYNAMIC_VALUES),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{"b": ANY_SERIALIZABLE},
				static:  map[string]Pattern{"b": &TypePattern{val: ANY_SERIALIZABLE}},
			}, res)
		})
	})

	t.Run("binary expression", func(t *testing.T) {
		t.Run("+: left operand is a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("a" + int)`)
			res, err := symbolicEval(n, state)

			leftOperand := n.Statements[0].(*parse.BinaryExpression).Left

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(leftOperand, state, fmtExpectedLeftOperandForArithmetic(NewString("a"), parse.Add)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("+: (duration, duration)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(d1 + d2)`)
			duration := NewDuration(time.Hour)
			state.setGlobal("d1", duration, GlobalConst)
			state.setGlobal("d2", duration, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, ANY_DURATION, res)
		})

		t.Run("+: (datetime, duration)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(t + d)`)
			goTime := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
			datetime := NewDateTime(goTime)
			duration := NewDuration(time.Hour)

			state.setGlobal("t", datetime, GlobalConst)
			state.setGlobal("d", duration, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, ANY_DATETIME, res)
		})

		t.Run("+: (duration, datetime)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(t + d)`)
			goTime := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
			datetime := NewDateTime(goTime)
			duration := NewDuration(time.Hour)

			state.setGlobal("t", datetime, GlobalConst)
			state.setGlobal("d", duration, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, ANY_DATETIME, res)
		})

		t.Run("-: (duration, datetime)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(d - t)`)
			goTime := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
			datetime := NewDateTime(goTime)
			duration := NewDuration(time.Hour)

			state.setGlobal("t", datetime, GlobalConst)
			state.setGlobal("d", duration, GlobalConst)

			rightOperand := n.Statements[0].(*parse.BinaryExpression).Right

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(rightOperand, state, A_DURATION_CAN_BE_SUBSTRACTED_FROM_A_DATETIME),
			}, state.errors())
			assert.Equal(t, ANY_DATETIME, res)
		})

		t.Run("-: (datetime, duration)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(d - t)`)
			goTime := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
			datetime := NewDateTime(goTime)
			duration := NewDuration(time.Hour)

			state.setGlobal("t", datetime, GlobalConst)
			state.setGlobal("d", duration, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, ANY_DATETIME, res)
		})

		t.Run("+: right operand is a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(int + "a")`)
			res, err := symbolicEval(n, state)

			rightOperand := n.Statements[0].(*parse.BinaryExpression).Right

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(rightOperand, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "\"a\"")),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("<: left operand is a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("a" < int)`)
			res, err := symbolicEval(n, state)

			binExpr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr, state, OPERANDS_NOT_COMPARABLE_BECAUSE_DIFFERENT_TYPES),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("<: right operand is a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(int < "a")`)
			res, err := symbolicEval(n, state)

			binExpr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr, state, OPERANDS_NOT_COMPARABLE_BECAUSE_DIFFERENT_TYPES),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("<: left operand does not implement comparable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`({} < 1)`)
			res, err := symbolicEval(n, state)

			binExpr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr, state, LEFT_OPERAND_DOES_NOT_IMPL_COMPARABLE_),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("<: right operand does not implement comparable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(1 < {})`)
			res, err := symbolicEval(n, state)

			binExpr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr, state, RIGHT_OPERAND_DOES_NOT_IMPL_COMPARABLE_),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("substrof: left operand is an int", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(int substrof "1")`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Left, state, fmtLeftOperandOfBinaryShouldBe(parse.Substrof, "string-like or bytes-like", "%int")),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("substrof: right operand is an int", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("1" substrof int)`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Substrof, "string-like", "%int")),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("substrof: (string, string)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("A" substrof 0d[65])`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("substrof: (byte-slice, byte-slice)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("A" substrof 0d[65])`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("substrof: (string, byte-slice)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("A" substrof 0d[65])`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("substrof: (byte-slice, string)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("A" substrof 0d[65])`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("match: right operand is a path pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(/home/user/ match %/home/user/...)`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("match: right operand is a regex pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("(\"\" match %`.*`)")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("match: right operand is an object pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("({} match %{})")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("set difference: right operand is a pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("((%| int | 2 | 3) \\ %| int | 2)")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &DifferencePattern{
				Base:    ANY_PATTERN,
				Removed: ANY_PATTERN,
			}, res)
		})

		t.Run("set difference: right operand is an integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("((%| int | 2) \\ int)")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &DifferencePattern{
				Base:    ANY_PATTERN,
				Removed: ANY_PATTERN,
			}, res)
		})

		t.Run("binary in/not-in", func(t *testing.T) {

			t.Run("base case", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("(1 in [])")
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_BOOL, res)
			})

			t.Run("left operand is not serializable", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("((go do {}) in [])")
				res, err := symbolicEval(n, state)

				spawnExpr := parse.FindNode(n, (*parse.SpawnExpression)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(spawnExpr, state, fmtLeftOperandOfBinaryShouldBe(parse.In, "serializable", Stringify(ANY_LTHREAD))),
				}, state.errors())
				assert.Equal(t, ANY_BOOL, res)
			})

			t.Run("right operand is not a container", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("(1 in true)")
				res, err := symbolicEval(n, state)

				booleanLit := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(booleanLit, state, fmtRightOperandOfBinaryShouldBe(parse.In, "container", Stringify(TRUE))),
				}, state.errors())
				assert.Equal(t, ANY_BOOL, res)
			})
		})

		t.Run("range expression", func(t *testing.T) {
			getQuantity := extData.GetQuantity
			ToSymbolicValue := extData.ToSymbolicValue

			defer func() {
				extData.GetQuantity = getQuantity
				extData.ToSymbolicValue = ToSymbolicValue
			}()
			extData.GetQuantity = func(values []float64, units []string) (any, error) {
				if units[0] == "B" {
					return NewByteCount(int64(values[0])), nil
				}
				panic(ErrUnreachable)
			}
			extData.ToSymbolicValue = func(_ ConcreteContext, v any, wide bool) (Value, error) {
				return v.(Value), nil
			}

			testCases := []struct {
				code   string
				result Value
				err    bool
			}{
				{"(1 .. 2)", NewIntRange(INT_1, INT_2, false), false},
				{"(1 ..< 2)", NewIntRange(INT_1, INT_1, false), false},
				{"(1.0 .. 2.0)", NewIncludedEndFloatRange(FLOAT_1, FLOAT_2), false},
				{"(1.0 ..< 2.0)", NewExcludedEndFloatRange(FLOAT_1, FLOAT_2), false},
				{"(1B .. 2B)", &QuantityRange{element: ANY_BYTECOUNT}, false},
				{"(1B ..< 2B)", &QuantityRange{element: ANY_BYTECOUNT}, false},

				//cases with error
				{"(1 .. 2.0)", ANY_INT_RANGE, true},
				{"(1.0 .. 2)", ANY_FLOAT_RANGE, true},
				{"(1 .. 2B)", ANY_INT_RANGE, true},
				{"(1B .. 2)", &QuantityRange{element: ANY_BYTECOUNT}, true},
				{"((go do {}) .. 2)", ANY_QUANTITY_RANGE, true},
			}

			for _, testCase := range testCases {
				t.Run(testCase.code, func(t *testing.T) {
					n, state := MakeTestStateAndChunk(testCase.code)
					res, err := symbolicEval(n, state)

					if !assert.NoError(t, err) {
						return
					}

					assert.Equal(t, testCase.result, res)

					if testCase.err {
						assert.Len(t, state.errors(), 1)
					}
				})
			}
		})

		t.Run("pair: left operand should be be serializable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(go do {} , 1)`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Left, state, fmtLeftOperandOfBinaryShouldBe(parse.PairComma, "serializable", "%lthread")),
			}, state.errors())
			assert.Equal(t, NewOrderedPair(ANY_SERIALIZABLE, INT_1), res)
		})

		t.Run("pair: right operand should be be serializable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(1 , go do {})`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.PairComma, "serializable", "%lthread")),
			}, state.errors())
			assert.Equal(t, NewOrderedPair(INT_1, ANY_SERIALIZABLE), res)
		})

		t.Run("pair: left operand should be be immutable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`({} , 1)`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Left, state, fmtLeftOperandOfBinaryShouldBeImmutable(parse.PairComma)),
			}, state.errors())
			assert.Equal(t, NewOrderedPair(ANY_SERIALIZABLE, INT_1), res)
		})

		t.Run("pair: right operand should be be immutable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(1 , {})`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Right, state, fmtRightOperandOfBinaryShouldBeImmutable(parse.PairComma)),
			}, state.errors())
			assert.Equal(t, NewOrderedPair(INT_1, ANY_SERIALIZABLE), res)
		})

	})

	t.Run("unary expression: !: operand is a string", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`!"string"`)
		res, err := symbolicEval(n, state)

		assert.NoError(t, err)
		assert.Equal(t, []SymbolicEvaluationError{
			makeSymbolicEvalError(n, state, fmtOperandOfBoolNegateShouldBeBool(NewString("string"))),
		}, state.errors())
		assert.Equal(t, ANY_BOOL, res)
	})

	t.Run("unary expression: number negation", func(t *testing.T) {
		t.Run("invalid operand", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(- true)`)
			unaryExpr := parse.FindNode(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExpr, state, fmtOperandOfNumberNegateShouldBeIntOrFloat(TRUE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("int multitvalue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(- a)`)

			state.setGlobal("a", NewMultivalue(INT_1, INT_2), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("float multitvalue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(- a)`)

			state.setGlobal("a", NewMultivalue(FLOAT_1, FLOAT_2), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_FLOAT, res)
		})
	})

	t.Run("function declaration", func(t *testing.T) {

		t.Run("missing body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				fn f()
				return f
			`, nil)

			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:      fnExpr,
				nodeChunk: n,
				result:    nil,
			}, res)

			//check definition position data
			idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.IdentifierLiteral)(nil), nil)
			definitionIdent := idents[0]
			returnIdent := idents[1]
			returnIdentAncestors := ancestorChains[1]

			pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, definitionIdent.Span, pos.Span)
		})

		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
	
				}
				return f
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:      fnExpr,
				nodeChunk: n,
				result:    Nil,
			}, res)

			//check definition position data
			idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.IdentifierLiteral)(nil), nil)
			definitionIdent := idents[0]
			returnIdent := idents[1]
			returnIdentAncestors := ancestorChains[1]

			pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, definitionIdent.Span, pos.Span)
		})

		t.Run("single parameter", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a){
					return a
				}
				return f
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{ANY},
				parameterNames: []string{"a"},
				result:         ANY,
			}, res)
		})

		t.Run("variadic parameter", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...a){
					return a
				}
				return f
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{ANY_ARRAY},
				parameterNames: []string{"a"},
				result:         ANY_ARRAY,
			}, res)
		})

		t.Run("variadic parameter with specific element tyoe", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...integers int){
					return integers
				}
				return f
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{NewArrayOf(ANY_INT)},
				parameterNames: []string{"integers"},
				result:         NewArrayOf(ANY_INT),
			}, res)
		})

		t.Run("no params, single captured local", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = int
				fn[a] f(){
					return a
				}
				return f
			`)
			fnExpr := n.Statements[1].(*parse.FunctionDeclaration).Function

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				result:         ANY_INT,
				capturedLocals: map[string]Value{"a": ANY_INT},
			}, res)
		})

		t.Run("no params, two captured locals", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = int
				b = "1"
				fn[a, b] f(){
					return [a, b]
				}
				return f
			`)
			fnExpr := n.Statements[2].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:      fnExpr,
				nodeChunk: n,
				result:    NewList(ANY_INT, NewString("1")),
				capturedLocals: map[string]Value{
					"a": ANY_INT,
					"b": NewString("1"),
				},
			}, res)
		})

		t.Run("return type specified but missing return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() %int {
					
				}
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnExpr, state, MISSING_RETURN_IN_FUNCTION),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("invalid return value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() %int {
					return "a"
				}
			`)

			returnStmt := parse.FindNode(n, (*parse.ReturnStatement)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(returnStmt, state, fmtInvalidReturnValue(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("invalid return value: arrow syntax", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() %int => "a"
			`)

			strLit := parse.FindNode(n, (*parse.DoubleQuotedStringLiteral)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(strLit, state, fmtInvalidReturnValue(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("invalid return value (annotation is an unprefixed named pattern)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() int {
					return "a"
				}
			`)

			returnStmt := parse.FindNode(n, (*parse.ReturnStatement)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(returnStmt, state, fmtInvalidReturnValue(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("invalid return value (deep mismatch)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() ({a: %int}) {
					return {
						a: "a"
					}
				}
			`)

			objectProp := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectProp.Value, state, fmtNotAssignableToPropOfType(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("missing unconditional return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a) %int {
					if a? {
						return int
					}
				}
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnExpr, state, MISSING_UNCONDITIONAL_RETURN_IN_FUNCTION),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("invalid conditional return value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a) %int {
					if a? {
						return "a"
					}
					return int
				}
			`)

			returnStmts := parse.FindNodes(n, (*parse.ReturnStatement)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(returnStmts[0], state, fmtInvalidReturnValue(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("invalid nested conditional return value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a) %int {
					if a? {
						if a? {
							return "a"
						}
					}
					return int
				}
			`)

			returnStmts := parse.FindNodes(n, (*parse.ReturnStatement)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(returnStmts[0], state, fmtInvalidReturnValue(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("patterns should be accessible from the body", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = int
				fn f(){
					[%p, %int]
				}
				return $f
			`)
			fnExpr := n.Statements[1].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:      fnExpr,
				result:    Nil,
				nodeChunk: n,
			}, res)
		})

		t.Run("a function that does not capture locals nor access globals is callable anywhere", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return (g() + f())

				fn g(){
					return f()
				}

				fn f(){
					return 1
				}
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("a function that captures a local variable is callable after the definition of the variable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				x = 1

				val = f()

				fn g(){
					return f()
				}

				fn[x] f(){
					return x
				}

				return (val + g())
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, ANY_INT, res)
		})
	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("missing body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				f = fn()
				return f
			`, nil)

			fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:      fnExpr,
				nodeChunk: n,
				result:    nil,
			}, res)

			//check definition position data
			idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.IdentifierLiteral)(nil), nil)
			definitionIdent := idents[0]
			returnIdent := idents[1]
			returnIdentAncestors := ancestorChains[1]

			pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, definitionIdent.Span, pos.Span)
		})

		t.Run("patterns should be accessible from the body of a function expression within a function declaration", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = int
				fn f(){
					return fn(){
						[%p, %int]
					}
				}
				`)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})
	})

	t.Run("function pattern", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %fn()
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				function: &Function{
					patternNode:                  fnPatt,
					patternNodeChunk:             n,
					formattedPatternNodeLocation: ":2:12:",

					results:                 []Value{Nil},
					parameters:              []Value{},
					parameterNames:          []string{},
					firstOptionalParamIndex: -1,
				},
			}, res)
		})

		t.Run("single parameter", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %fn(a %int)
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				function: &Function{
					patternNode:                  fnPatt,
					patternNodeChunk:             n,
					formattedPatternNodeLocation: ":2:12:",
					results:                      []Value{Nil},
					parameters:                   []Value{ANY_INT},
					parameterNames:               []string{"a"},
					firstOptionalParamIndex:      -1,
				},
			}, res)
		})

		t.Run("parameter with no name and a prefixed named pattern as type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %fn(%int)
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				function: &Function{
					patternNode:                  fnPatt,
					patternNodeChunk:             n,
					formattedPatternNodeLocation: ":2:12:",

					results:                 []Value{Nil},
					parameters:              []Value{ANY_INT},
					parameterNames:          []string{"_"},
					firstOptionalParamIndex: -1,
				},
			}, res)
		})

		t.Run("parameter with no name and a unprefixed named pattern as type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern f = fn(int)
				return %f
			`)
			fnPatt := n.Statements[0].(*parse.PatternDefinition).Right.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				function: &Function{
					patternNode:                  fnPatt,
					patternNodeChunk:             n,
					formattedPatternNodeLocation: ":2:17:",

					results:                 []Value{Nil},
					parameters:              []Value{ANY_INT},
					parameterNames:          []string{"_"},
					firstOptionalParamIndex: -1,
				},
			}, res)
		})
	})

	t.Run("call undefined function", func(t *testing.T) {
		t.Run("regular call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			ident := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("f")),
				makeSymbolicEvalError(ident, state, fmtCannotCall(ANY)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("regular call: undefined variable as single argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(x)
			`)

			ident := parse.FindIdentWithName(n, "f")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				//makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("x")),
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("f")),
				makeSymbolicEvalError(ident, state, fmtCannotCall(ANY)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("must call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f!()
			`)
			ident := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("f")),
				makeSymbolicEvalError(ident, state, fmtCannotCall(ANY)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})
	})

	t.Run("call Inox function", func(t *testing.T) {
		t.Run("empty function (identifier)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, Nil, res)

			callee := parse.FindNode(n, (*parse.CallExpression)(nil), nil).Callee

			calleeValue, ok := state.symbolicData.GetMostSpecificNodeValue(callee)
			if !assert.True(t, ok) {
				return
			}

			assert.IsType(t, (*InoxFunction)(nil), calleeValue)
		})

		t.Run("empty function (member)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				o = {
					f: fn(){}
				}
	
				return $o.f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, Nil, res)
		})

		t.Run("empty function (identifier member)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				o = {
					f: fn(){}
				}
	
				return o.f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, Nil, res)
		})

		t.Run("function always return an integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return int
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("local variable defined outside of a function should not be accessible from inside the function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = ""
				fn f(){
					return a
				}
				return f()
			`)

			identifier := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[2]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(identifier, state, fmtVarIsNotDeclared("a")),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("function return its first argument: type of result should be the type of the arg", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(x){
					return x
				}
	
				return f(int)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			//check definition position data
			idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.IdentifierLiteral)(nil), func(n *parse.IdentifierLiteral) bool {
				return n.Name == "x"
			})
			definitionIdent := idents[0]
			returnIdent := idents[1]
			returnIdentAncestors := ancestorChains[1]

			pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, definitionIdent.Span, pos.Span)
		})

		t.Run("function returning its variadic argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest){
					return $rest
				}
	
				return f(int)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(ANY_INT), res)
		})

		t.Run("no variadic parameter: list spread argument (known length)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a){
					return $a
				}
	
				list = ["2"]
				return f(...list)
			`)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS),
			}, state.errors())
			assert.Equal(t, NewString("2"), res)
		})

		t.Run("no variadic parameter: array spread argument (known length)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a){
					return $a
				}
	
				array = Array("2")
				return f(...array)
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS),
			}, state.errors())
			assert.Equal(t, NewString("2"), res)
		})

		t.Run("no variadic parameter: spread argument (unknown length)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a){
					return $a
				}
	
				return f(...list)
			`)

			state.setGlobal("list", &List{generalElement: ANY_SERIALIZABLE}, GlobalConst)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS),
			}, state.errors())
			assert.Equal(t, ANY_SERIALIZABLE, res)
		})

		t.Run("single, variadic parameter: list spread argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest){
					return $rest
				}
	
				list = ["2", true]
				return f(int, ...list)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(ANY_INT, NewString("2"), TRUE), res)
		})

		t.Run("single, variadic parameter: array spread argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest){
					return $rest
				}
	
				array = Array("2", true)
				return f(int, ...array)
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(ANY_INT, NewString("2"), TRUE), res)
		})

		t.Run("single, variadic parametert: no arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest){
					return $rest
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(), res)
		})

		t.Run("single, variadic parameter of element type 'int': no arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest int){
					return $rest
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(), res)
		})

		t.Run("single, variadic parameter of element type 'int': single integer argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest int){
					return $rest
				}
	
				return f(1)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(INT_1), res)
		})

		t.Run("single, variadic parameter of element type 'int': single non-integer argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest int){
					return $rest
				}
	
				return f("a")
			`)
			strLit := parse.FindNode(n, (*parse.DoubleQuotedStringLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(strLit, state, FmtInvalidArg(0, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, NewArray(ANY_INT), res)
		})

		t.Run("single, variadic parameter of element type 'int': two integer arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest int){
					return $rest
				}
	
				return f(1, 2)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(INT_1, INT_2), res)
		})

		t.Run("single, variadic parameter of element type 'int': integer arg followed by a non-integer argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(...rest int){
					return $rest
				}
	
				return f(1, "a")
			`)
			strLit := parse.FindNode(n, (*parse.DoubleQuotedStringLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(strLit, state, FmtInvalidArg(1, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, NewArray(INT_1, ANY_INT), res)
		})

		t.Run("non variadic parameter + variadic parameter: spread argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(first, ...rest){
					return Array(first, $rest)
				}
	
				list = ["2", true]
				return f(int, ...list)
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(
				ANY_INT,
				NewArray(NewString("2"), TRUE),
			), res)
		})

		t.Run("function declaration + call: %int return type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() %int {
					return int
				}
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("function declaration with the arrow syntax + call: %int return type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() %int => int
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("function declaration + call: invalid return value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f() %int {
					return "a"
				}
				return f()
			`)

			fnReturnStmt := parse.FindNodes(n, (*parse.ReturnStatement)(nil), nil)[0]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnReturnStmt, state, fmtInvalidReturnValue(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("invalid argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(x %int){
					return int
				}
	
				return f("a")
			`)

			call := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call.Arguments[0], state, FmtInvalidArg(0, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("invalid argument (unprefixed pattern)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(x int){
					return int
				}
	
				return f("a")
			`)

			call := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call.Arguments[0], state, FmtInvalidArg(0, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("missing property in object argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg {a: int}){
					return int
				}
				return f({})
			`)

			argNode := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			param := NewInexactObject(map[string]Serializable{
				"a": ANY_INT,
			}, nil, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, NewEmptyObject(), param)),
			}, state.errors())

			allowedProps, ok := state.symbolicData.GetAllowedNonPresentProperties(argNode)
			assert.True(t, ok)
			assert.Equal(t, []string{"a"}, allowedProps)

			assert.Equal(t, ANY_INT, res)
		})

		t.Run("missing property in object argument and optional property in parameter", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg {a: int, b: int}){
					return int
				}
				return f({})
			`)

			argNode := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			param := NewInexactObject(map[string]Serializable{
				"a": ANY_INT,
				"b": ANY_INT,
			}, nil, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, NewEmptyObject(), param)),
			}, state.errors())

			allowedProps, ok := state.symbolicData.GetAllowedNonPresentProperties(argNode)
			assert.True(t, ok)
			assert.Equal(t, []string{"a", "b"}, allowedProps)

			assert.Equal(t, ANY_INT, res)
		})

		t.Run("invalid type of property in object argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg {a: int}){
					return int
				}
				return f({a: "a"})
			`)

			argNode := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]
			propertyValue := argNode.(*parse.ObjectLiteral).Properties[0].Value

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propertyValue, state, fmtNotAssignableToPropOfType(NewString("a"), ANY_INT)),
			}, state.errors())

			assert.Equal(t, ANY_INT, res)
		})

		t.Run("valid argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(x %int){
					return int
				}
	
				return f(0)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("valid argument (unprefixed pattern)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(x int){
					return int
				}
	
				return f(0)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("multi value argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(list %| %[]%int | %[]%str){
					f = fn(v %| %[]%int | %[]%str){
						
					}
					return f(list)
				}
			`)

			fnExpr := parse.FindNodes(n, &parse.FunctionExpression{}, nil)[0]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{NewMultivalue(NewListOf(ANY_INT), NewListOf(ANY_STR_LIKE))},
				parameterNames: []string{"list"},
				result:         Nil,
			}, res)
		})

		t.Run("non-variadic function: not enough arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a, b %int, c){
					return Array(a, b, c)
				}
	
				return f(int)
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(1, 3)),
			}, state.errors())

			assert.Equal(t, NewArray(ANY_INT, ANY_INT, ANY), res)
		})

		t.Run("non-variadic function: too many arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a){
					return a
				}
	
				return f(int, 2)
			`)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(2, 1)),
			}, state.errors())

			assert.Equal(t, ANY_INT, res)
		})

		t.Run("variadic function fn(a, ...rest): not enough arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a, ...rest){
					return Array(a, rest)
				}
	
				return f()
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(0, 1)),
			}, state.errors())

			assert.Equal(t, NewArray(ANY, NewArray()), res)
		})

		t.Run("variadic function fn(a, b, ...rest): not enough arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(a, b, ...rest){
					return Array(a, b, rest)
				}
	
				return f(int)
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(1, 2)),
			}, state.errors())

			assert.Equal(t, NewArray(ANY_INT, ANY, NewArray()), res)
		})

		t.Run("direct recursion of a function with no return type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn factorial(i %int) {
					if (i == 0) {
						return int
					}
					return (i * factorial( (i - int) ))
				}
				return factorial(3)
			`)
			call := parse.FindNodes(n, (*parse.CallExpression)(nil), nil)[0]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, FUNCS_CALLED_RECU_SHOULD_HAVE_RET_TYPE),
				makeSymbolicEvalError(call, state, fmtRightOperandOfBinaryShouldBe(parse.Mul, "int", "%any")),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("direct recursion of a function with a return type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn factorial(i %int) %int {
					if (i == 0) {
						return int
					}
					return (i * factorial( (i - int) ))
				}
				return factorial(3)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("extension's method returning a property (double colon expression)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern o = {
					a: 1
				}
				extend o {
					f: fn() => self.a
				}

				var o o = {
					a: 1
				}

				return o::f()
			`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInt(1), res)
		})

		t.Run("'must' call: function always returns an error", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return err
				}
				return f!()
			`)
			state.setGlobal("err", ANY_ERR, GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.NotEmpty(t, state.warnings())
			assert.Equal(t, Nil, res)
		})

		t.Run("'must' call: function always returns nil", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return nil
				}
				return f!()
			`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.NotEmpty(t, state.warnings())
			assert.Equal(t, Nil, res)
		})

		t.Run("'must' call: function always returns (error|nil)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg bool){
					if arg {
						return err
					}
					return nil
				}
				return f!(bool)
			`)
			state.setGlobal("err", ANY_ERR, GlobalConst)
			state.setGlobal("bool", ANY_BOOL, GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Empty(t, state.warnings())
			assert.Equal(t, Nil, res)
		})

		t.Run("'must' call: function always return an array of length-2 with an error", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return Array(1, err)
				}
				return f!()
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			state.setGlobal("err", ANY_ERR, GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInt(1), res)
		})

		t.Run("'must' call: function always return an array of length-2 with an (error|nil)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return Array(1, err_or_nil)
				}
				return f!()
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			state.setGlobal("err_or_nil", NewMultivalue(ANY_ERR, Nil), GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInt(1), res)
		})

		t.Run("'must' call: function should not return an empty array", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return Array()
				}
				return f!()
			`)
			fnIdent := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Callee
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnIdent, state, INVALID_MUST_CALL_OF_AN_INOX_FN_RETURN_TYPE_MUST_BE_XXX),
			}, state.errors())
			assert.Empty(t, state.warnings())
			assert.Equal(t, NewArray(), res)
		})

		t.Run("'must' call: function should not return an array of length 1", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return Array(1)
				}
				return f!()
			`)
			fnIdent := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Callee
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnIdent, state, INVALID_MUST_CALL_OF_AN_INOX_FN_RETURN_TYPE_MUST_BE_XXX),
			}, state.errors())
			assert.Empty(t, state.warnings())
			assert.Equal(t, NewArray(INT_1), res)
		})

		t.Run("'must' call: function should not return a value that is not nil, nor err, nor an array", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return 1
				}
				return f!()
			`)
			fnIdent := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Callee
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnIdent, state, INVALID_MUST_CALL_OF_AN_INOX_FN_RETURN_TYPE_MUST_BE_XXX),
			}, state.errors())
			assert.Empty(t, state.warnings())
			assert.Equal(t, INT_1, res)
		})
	})

	t.Run("call Go function", func(t *testing.T) {
		t.Run("signature is func(*Context) int", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) *Int {
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context) *List", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) *List {
					return nil
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &List{generalElement: ANY_SERIALIZABLE}, res)
		})

		t.Run("signature is func(*Context) (int, 1)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) (*Int, *Int) {
					return ANY_INT, ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewArray(ANY_INT, ANY_INT), res)
		})

		t.Run("signature is func(*Context, *Int) *Int: missing argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, *Int) *Int {
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtNotEnoughArgs(0, 1)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, *Int) *Int: bad argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f("a")
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			goFunc := &GoFunction{
				fn: func(*Context, *Int) *Int {
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, *Int) *Int: too many arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(int, 2)
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, *Int) *Int {
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtTooManyArgs(2, 1)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, OptionalParam[*Int]) *Int: no provided argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(ctx *Context, i *OptionalParam[*Int]) *Int {
					if i.Value != nil {
						ctx.AddSymbolicGoFunctionError("argument should not have been provided")
					}
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, OptionalParam[*Int]) *Int: provided argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1)
			`)

			goFunc := &GoFunction{
				fn: func(_ *Context, i *OptionalParam[*Int]) *Int {
					return *i.Value
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInt(1), res)
		})

		t.Run("signature is func(*Context, OptionalParam[*Int]) *Int: bad argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f("a")
			`)
			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			goFunc := &GoFunction{
				fn: func(_ *Context, i *OptionalParam[*Int]) *Int {
					if i.Value == nil {
						return ANY_INT
					}
					return *i.Value
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, *Int, OptionalParam[*Int]) *Int: no provided arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(ctx *Context, _ *Int, i *OptionalParam[*Int]) *Int {
					if i.Value != nil {
						ctx.AddSymbolicGoFunctionError("argument should not have been provided")
					}
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtNotEnoughArgsAtLeastMandatoryMax(0, 1, 2)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, *Int, OptionalParam[*Int]) *Int: second argument not provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1)
			`)

			goFunc := &GoFunction{
				fn: func(ctx *Context, _ *Int, i *OptionalParam[*Int]) *Int {
					if i.Value != nil {
						ctx.AddSymbolicGoFunctionError("argument should not have been provided")
					}
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, *Int, OptionalParam[*Int]) *Int: second argument not provided and function is specific", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1)
			`)

			goFunc := &GoFunction{
				fn: func(ctx *Context, _ *Int, i *OptionalParam[*Int]) *Int {
					if i.Value != nil {
						ctx.AddSymbolicGoFunctionError("argument should not have been provided")
					}
					ctx.SetSymbolicGoFunctionParameters(&[]Value{ANY_INT, NewInt(2)}, []string{"a", "b"})
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, *Int, OptionalParam[*Int]) *Int: second argument provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1, 2)
			`)

			goFunc := &GoFunction{
				fn: func(_ *Context, a *Int, b *OptionalParam[*Int]) *Int {
					return NewInt(a.value + (*b.Value).value)
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInt(1+2), res)
		})

		t.Run("signature is func(*Context, *Int, OptionalParam[*Int]) *Int: bad second argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1, "a")
			`)
			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[1]

			goFunc := &GoFunction{
				fn: func(ctx *Context, a *Int, b *OptionalParam[*Int]) *Int {
					if b.Value != nil {
						ctx.AddSymbolicGoFunctionError("argument should not have been provided")
					}
					return a
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(1, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, NewInt(1), res)
		})

		t.Run("signature is func(*Context, *List) *Int: passing multivalue of 2 lists should be an error", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(list %| %[]%str | %[]%int){
					return f(list)
				}
			`)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			goFunc := &GoFunction{
				fn: func(ctx *Context, list *List) *List {
					return list
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call.Arguments[0], state,
					FmtInvalidArg(
						0,
						NewMultivalue(NewListOf(ANY_STR_LIKE), NewListOf(ANY_INT)),
						NewListOf(ANY_SERIALIZABLE),
					),
				),
			}, state.errors())

			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{NewMultivalue(NewListOf(ANY_STR_LIKE), NewListOf(ANY_INT))},
				parameterNames: []string{"list"},
				result:         NewListOf(ANY_SERIALIZABLE),
			}, res)
		})

		t.Run("signature is func(*Context, ...*List) *Int: pass multivalue of 2 lists should be an error", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(list %| %[]%str | %[]%int){
					return f(list)
				}
			`)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			goFunc := &GoFunction{
				fn: func(ctx *Context, list ...*List) Iterable {
					return list[0]
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state,
					FmtInvalidArg(
						0,
						NewMultivalue(NewListOf(ANY_STR_LIKE), NewListOf(ANY_INT)),
						NewListOf(ANY_SERIALIZABLE),
					),
				),
			}, state.errors())

			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{NewMultivalue(NewListOf(ANY_STR_LIKE), NewListOf(ANY_INT))},
				parameterNames: []string{"list"},
				result:         NewListOf(ANY_SERIALIZABLE),
			}, res)
		})

		t.Run("signature is func(*Context, ...*Int) *Int: bad first variadic argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f("a")
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, ...*Int) *Int {
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, FmtInvalidArg(0, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, *Int, ...*Int) *Int: missing argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, *Int, ...*Int) *Int {
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(0, 1)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(*Context, ...*Int) *Int: bad second variadic argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(int, "a")
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, ...*Int) *Int {
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, FmtInvalidArg(1, NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("signature is func(...*Int) *Int: no argument provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)
			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			goFunc := &GoFunction{
				fn: func(args ...*Int) *Int {
					assert.Empty(t, args)
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: -1,
				parameters:              []Value{NewArray(ANY_INT)},
				parameterNames:          []string{"_"},
				results:                 []Value{ANY_INT},
				originGoFunction:        goFunc,
				variadic:                true,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.False(t, fn.HasOptionalParams())
			assert.Equal(t, ANY_INT, fn.VariadicParamElem())
		})

		t.Run("signature is func(...*Int) *Int: one argument provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1)
			`)
			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			goFunc := &GoFunction{
				fn: func(args ...*Int) *Int {
					if assert.Equal(t, []*Int{INT_1}, args) {
						return INT_1
					}
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: -1,
				parameters:              []Value{NewArray(ANY_INT)},
				parameterNames:          []string{"_"},
				results:                 []Value{INT_1},
				originGoFunction:        goFunc,
				variadic:                true,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.False(t, fn.HasOptionalParams())
			assert.Equal(t, ANY_INT, fn.VariadicParamElem())
		})

		t.Run("signature is func(...*Int) *Int: two arguments provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1, 2)
			`)
			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			goFunc := &GoFunction{
				fn: func(args ...*Int) *Int {
					if assert.Equal(t, []*Int{INT_1, INT_2}, args) {
						return INT_3
					}
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_3, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: -1,
				parameters:              []Value{NewArray(ANY_INT)},
				parameterNames:          []string{"_"},
				results:                 []Value{INT_3},
				originGoFunction:        goFunc,
				variadic:                true,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.False(t, fn.HasOptionalParams())
			assert.Equal(t, ANY_INT, fn.VariadicParamElem())
		})

		t.Run("signature is func(...Value) Value: no argument provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(args ...Value) Value {
					if assert.Empty(t, args) {
						return INT_0
					}
					return ANY
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_0, res)
		})
		t.Run("signature is func(...Value) Value: one argument provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1)
			`)

			goFunc := &GoFunction{
				fn: func(args ...Value) Value {
					if assert.Equal(t, []Value{INT_1}, args) {
						return INT_1
					}
					return ANY
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1, res)
		})

		t.Run("signature is func(...Value) Value: two arguments provided", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1, 2)
			`)

			goFunc := &GoFunction{
				fn: func(args ...Value) Value {
					if assert.Equal(t, []Value{INT_1, INT_2}, args) {
						return INT_3
					}
					return ANY
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_3, res)
		})

		t.Run("signature is func(*Context) error", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) *Error {
					return nil
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(ANY_ERR, Nil), res)
		})
		t.Run("no concrete value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f()
			`)

			state.setGlobal("f", &GoFunction{}, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0].(*parse.ReturnStatement).Expr, state, CANNOT_CALL_GO_FUNC_NO_CONCRETE_VALUE),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("call variadic Go function: spread argument (unknown length), missing non variadic argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(...list)
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			state.setGlobal("list", &List{generalElement: ANY_SERIALIZABLE}, GlobalConst)
			goFunc := &GoFunction{
				fn: func(*Context, Value, ...*Int) *Int {
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(0, 1)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("call variadic Go function: spread argument (unknown length)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(...list)
			`)

			state.setGlobal("list", &List{generalElement: ANY_INT}, GlobalConst)
			goFunc := &GoFunction{
				fn: func(*Context, ...*Int) *Int {
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("'must' call Go function: error is not returned", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f!()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) (*Int, *Error) {
					//TODO: replace error with symbolic Nil error
					return ANY_INT, &Error{data: ANY}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("simple specific Go function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(#a)
			`)

			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg Value) *Int {
					if _, ok := arg.(*Identifier); ok {
						ctx.SetSymbolicGoFunctionParameters(&[]Value{&Identifier{name: "a"}}, []string{"arg"})
					}
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: -1,
				parameters:              []Value{NewIdentifier("a")},
				parameterNames:          []string{"arg"},
				results:                 []Value{ANY_INT},
				originGoFunction:        goFunc,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.False(t, fn.HasOptionalParams())
		})

		t.Run("simple specific Go function: invalid argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(#b)
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg Value) *Int {
					if _, ok := arg.(*Identifier); ok {
						ctx.SetSymbolicGoFunctionParameters(&[]Value{&Identifier{name: "a"}}, []string{"arg"})
					}
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, &Identifier{name: "b"}, &Identifier{name: "a"})),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("specific Go function with optional parameter", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(#b)
			`)

			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)
			argNode := callExprNode.Arguments[0]

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg *OptionalParam[Value]) *Int {
					if _, ok := (*arg.Value).(*Identifier); ok {
						ctx.SetSymbolicGoFunctionParameters(&[]Value{NewIdentifier("a")}, []string{"arg"})
					}
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, NewIdentifier("b"), NewIdentifier("a"))),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: 0,
				parameters:              []Value{NewIdentifier("a")},
				parameterNames:          []string{"arg"},
				results:                 []Value{ANY_INT},
				originGoFunction:        goFunc,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.True(t, fn.HasOptionalParams())
		})

		t.Run("specific variadic Go function: single argument (variadic)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1)
			`)

			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			goFunc := &GoFunction{
				fn: func(ctx *Context, ints ...*Int) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]Value{ANY_INT}, []string{"ints"})
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: -1,
				parameters:              []Value{ANY_INT},
				parameterNames:          []string{"ints"},
				results:                 []Value{ANY_INT},
				variadic:                false,
				originGoFunction:        goFunc,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.False(t, fn.HasOptionalParams())
		})

		t.Run("specific variadic Go function: two arguments (variadic)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(1, 2)
			`)

			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			goFunc := &GoFunction{
				fn: func(ctx *Context, ints ...*Int) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]Value{ANY_INT}, []string{"ints"})
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: -1,
				parameters:              []Value{ANY_INT},
				parameterNames:          []string{"ints"},
				results:                 []Value{ANY_INT},
				variadic:                false,
				originGoFunction:        goFunc,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.False(t, fn.HasOptionalParams())
		})

		t.Run("specific variadic Go function: spread argument of unknown length", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(...list)
			`)

			callExprNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			goFunc := &GoFunction{
				fn: func(ctx *Context, ints ...*Int) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]Value{ANY_INT}, []string{"ints"})
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			state.setGlobal("list", NewListOf(ANY_INT), GlobalVar)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			calleeData, ok := state.symbolicData.GetMostSpecificNodeValue(callExprNode.Callee)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Equal(t, &Function{
				firstOptionalParamIndex: -1,
				parameters:              []Value{ANY_INT},
				parameterNames:          []string{"ints"},
				results:                 []Value{ANY_INT},
				variadic:                false,
				originGoFunction:        goFunc,
			}, calleeData) {
				return
			}

			fn := calleeData.(*Function)
			assert.False(t, fn.HasOptionalParams())
		})

		t.Run("specific Go function with non-empty object parameter, missing property in argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f({})
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			param := NewInexactObject(map[string]Serializable{
				"prop": ANY_INT,
			}, nil, nil)

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg Value) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]Value{param}, []string{"arg"})
					return ANY_INT
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, NewEmptyObject(), param)),
			}, state.errors())

			allowedProps, ok := state.symbolicData.GetAllowedNonPresentProperties(argNode)
			assert.True(t, ok)
			assert.Equal(t, []string{"prop"}, allowedProps)

			assert.Equal(t, ANY_INT, res)
		})

		t.Run("specific Go function with non-empty record parameter, missing property in argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f(#{})
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			param := NewInexactRecord(map[string]Serializable{
				"prop": ANY_INT,
			}, nil)

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg Value) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]Value{param}, []string{"arg"})
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, NewEmptyRecord(), param)),
			}, state.errors())

			allowedProps, ok := state.symbolicData.GetAllowedNonPresentProperties(argNode)
			assert.True(t, ok)
			assert.Equal(t, []string{"prop"}, allowedProps)

			assert.Equal(t, ANY_INT, res)
		})

		t.Run("specific Go function with non-empty object parameter, invalid property value in argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f({a: "a"})
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]
			propertyValue := argNode.(*parse.ObjectLiteral).Properties[0].Value

			param := NewInexactObject(map[string]Serializable{
				"a": ANY_INT,
			}, nil, nil)

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg Value) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]Value{param}, []string{"arg"})
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propertyValue, state, fmtNotAssignableToPropOfType(NewString("a"), ANY_INT)),
			}, state.errors())

			assert.Equal(t, ANY_INT, res)
		})

		t.Run("object literal arguments without methods or lifetime jobs should be evaluated as exact objects", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f({a: "a"})
			`)
			objLiteral := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg Value) Value {
					return arg
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			expectedObject := NewExactObject(map[string]Serializable{
				"a": NewString("a"),
			}, nil, map[string]Pattern{
				"a": &TypePattern{val: ANY_STRING},
			})

			assert.Equal(t, expectedObject, res)
			val, ok := state.symbolicData.GetMostSpecificNodeValue(objLiteral)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, expectedObject, val)
		})

		t.Run("complex specific Go function with invalid argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return map([int, 2, 3], fn(arg %str){
					return arg
				})
			`)

			state.setGlobal("map", WrapGoFunction(symbolicMap), GlobalConst)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			//TODO: check error
		})

		t.Run("complex specific Go function with valid arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return map([int, 2, 3], fn(arg %int){
					return arg
				})
			`)

			state.setGlobal("map", WrapGoFunction(symbolicMap), GlobalConst)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("complex specific Go function within a recursive Inox function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn rec(list %list){
				    assert (list match %[]%list)
					return map(list, rec)
				}

				return rec([ [ [], [] ], [ [], [] ]])
			`)

			state.setGlobal("map", WrapGoFunction(symbolicMap), GlobalConst)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("a Go method should be able to update its receiver to a compatible value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				l = [1]
				l.append(2)
				return l
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewListOf(ANY_INT), res)
		})

		t.Run("it should be an error for a Go method to update its receiver to an incompatible value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var l [1] = [1]
				l.append(2)
				return l
			`)
			callNode := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(callNode, state, INVALID_MUTATION),
			}, state.errors())
			assert.Equal(t, NewList(NewInt(1)), res)
		})

		t.Run("no error should be shown if a Go method updates its receiver to an incompatible value but args have errors", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var l [1] = [1]
				l.append(#{a: {}})
				return l
			`)
			recordLiteral := parse.FindNode(n, (*parse.RecordLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(recordLiteral.Properties[0].Value, state, fmtValuesOfRecordShouldBeImmutablePropHasMutable("a")),
			}, state.errors())
			assert.Equal(t, NewList(NewInt(1)), res)
		})

		t.Run("useless deep mutation of a shared object property's value should be an error - member expression", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				shared.list.append(1)
			`)
			sharedObject := NewInexactObject(map[string]Serializable{
				"list": NewListOf(ANY_INT),
			}, nil, map[string]Pattern{
				"list": NewListPatternOf(&TypePattern{val: ANY_INT}),
			})
			sharedObject = sharedObject.Share(state).(*Object)

			state.setGlobal("shared", sharedObject, GlobalConst)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			propIdent := parse.FindIdentWithName(n, "list")

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propIdent, state, fmtUselessMutationInClonedPropValue("list")),
			}, state.errors())
		})

		t.Run("useless deep mutation of a shared object property's value should be an error - ident memb expression", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				shared.list.append(1)
			`)
			sharedObject := NewInexactObject(map[string]Serializable{
				"list": NewListOf(ANY_INT),
			}, nil, map[string]Pattern{
				"list": NewListPatternOf(&TypePattern{val: ANY_INT}),
			})
			sharedObject = sharedObject.Share(state).(*Object)

			state.setGlobal("shared", sharedObject, GlobalConst)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			propIdent := parse.FindIdentWithName(n, "list")

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propIdent, state, fmtUselessMutationInClonedPropValue("list")),
			}, state.errors())
		})
	})

	t.Run("call abstract function", func(t *testing.T) {

		//TODO: add more tests
		t.Run("fn() %int", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() %int){
					return func()
				}
				return f
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			fnPatt := parse.FindNode(n, (*parse.FunctionPatternExpression)(nil), nil)

			expectedFn := &InoxFunction{
				node:      fnExpr,
				nodeChunk: n,
				parameters: []Value{
					&Function{
						patternNode:                  fnPatt,
						patternNodeChunk:             n,
						formattedPatternNodeLocation: ":2:15:",
						parameters:                   []Value{},
						parameterNames:               []string{},
						firstOptionalParamIndex:      -1,
						results:                      []Value{ANY_INT},
					},
				},
				parameterNames: []string{"func"},
				result:         ANY_INT,
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("'must' call: fn() %error", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() %error){
					return func!()
				}
				return f
			`)
			state.ctx.AddNamedPattern("error", &TypePattern{val: ANY_ERR}, false)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.NotEmpty(t, state.warnings())
			if !assert.IsType(t, (*InoxFunction)(nil), res) {
				return
			}

			result := res.(*InoxFunction).Result()
			assert.Equal(t, Nil, result)
		})

		t.Run("'must' call: fn() %nil", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() %nil){
					return func!()
				}
				return f
			`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.NotEmpty(t, state.warnings())
			if !assert.IsType(t, (*InoxFunction)(nil), res) {
				return
			}

			result := res.(*InoxFunction).Result()
			assert.Equal(t, Nil, result)
		})

		t.Run("'must' call: %fn() (error|nil)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() (| %error | %nil)){
					return func!()
				}
				return f
			`)
			state.ctx.AddNamedPattern("error", &TypePattern{val: ANY_ERR}, false)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Empty(t, state.warnings())
			if !assert.IsType(t, (*InoxFunction)(nil), res) {
				return
			}

			result := res.(*InoxFunction).Result()
			assert.Equal(t, Nil, result)
		})

		t.Run("'must' call: %fn() Array(1, %err)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() array2_with_err){
					return func!()
				}
				return f
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			state.ctx.AddNamedPattern("error", &TypePattern{val: ANY_ERR}, false)
			state.ctx.AddNamedPattern("array2_with_err", &TypePattern{
				val: NewArray(INT_1, ANY_ERR),
			}, false)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			if !assert.IsType(t, (*InoxFunction)(nil), res) {
				return
			}

			result := res.(*InoxFunction).Result()
			assert.Equal(t, INT_1, result)
		})

		t.Run("'must' call: %fn() Array(1, | %err | %nil)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() array2_with_err_or_nil){
					return func!()
				}
				return f
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			state.ctx.AddNamedPattern("error", &TypePattern{val: ANY_ERR}, false)
			state.ctx.AddNamedPattern("array2_with_err_or_nil", &TypePattern{
				val: NewArray(INT_1, NewMultivalue(ANY_ERR, Nil)),
			}, false)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			if !assert.IsType(t, (*InoxFunction)(nil), res) {
				return
			}

			result := res.(*InoxFunction).Result()
			assert.Equal(t, INT_1, result)
		})

		t.Run("'must' call: function should not return an empty array", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() empty_array){
					return func!()
				}
				return f
			`)
			fnIdent := parse.FindNode(n, (*parse.CallExpression)(nil), nil).Callee
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			state.ctx.AddNamedPattern("error", &TypePattern{val: ANY_ERR}, false)
			state.ctx.AddNamedPattern("empty_array", &TypePattern{
				val: NewArray(),
			}, false)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnIdent, state, INVALID_MUST_CALL_OF_AN_INOX_FN_RETURN_TYPE_MUST_BE_XXX),
			}, state.errors())
			if !assert.IsType(t, (*InoxFunction)(nil), res) {
				return
			}

			result := res.(*InoxFunction).Result()
			assert.Equal(t, NewArray(), result)
		})

		t.Run("'must' call: function should not return a value that is not nil, nor err, nor an array", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(func %fn() int){
					return func!()
				}
				return f
			`)
			fnIdent := parse.FindNode(n, (*parse.CallExpression)(nil), nil).Callee
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnIdent, state, INVALID_MUST_CALL_OF_AN_INOX_FN_RETURN_TYPE_MUST_BE_XXX),
			}, state.errors())
			assert.Empty(t, state.warnings())
			if !assert.IsType(t, (*InoxFunction)(nil), res) {
				return
			}

			result := res.(*InoxFunction).Result()
			assert.Equal(t, ANY_INT, result)
		})
	})
	t.Run("call pattern", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			%mypattern()
		`)

		state.ctx.AddNamedPattern("mypattern", &TypePattern{
			call: func(ctx *Context, values []Value) (Pattern, error) {
				return &ExactValuePattern{value: ANY_INT}, nil
			},
		}, false)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &ExactValuePattern{value: ANY_INT}, res)
	})

	t.Run("readonly pattern", func(t *testing.T) {
		t.Run("pattern convertible to readonly", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`pattern p = readonly {}; return %p`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())

			expectedObjectPattern := NewInexactObjectPattern(map[string]Pattern{}, nil)
			expectedObjectPattern.readonly = true

			assert.Equal(t, expectedObjectPattern, res)
		})

		t.Run("pattern of immutable value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`pattern p = readonly #{}; return %p`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())

			expectedRecordPattern := NewInexactRecordPattern(map[string]Pattern{}, nil)
			assert.Equal(t, expectedRecordPattern, res)
		})

		t.Run("pattern not convertible to readonly", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`pattern p = readonly {x: not-convertible}; return %p`)
			state.ctx.AddNamedPattern("not-convertible", ANY_SERIALIZABLE_PATTERN, true)

			pattern := parse.FindNode(n, (*parse.ObjectPatternLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(pattern, state, FmtPropertyPatternError("x", ErrNotConvertibleToReadonly).Error()),
			}, state.errors())

			expectedObjectPattern := NewInexactObjectPattern(map[string]Pattern{
				"x": ANY_SERIALIZABLE_PATTERN,
			}, nil)
			assert.Equal(t, expectedObjectPattern, res)
		})
	})

	t.Run("pipe statement", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar result = {value: 0}

				fn get_int(){
					return Array(int, nil)
				}

	
				fn addOne(i %int){
					result.value = (i + int)
				}
	
				get_int | addOne $
				return result.value
			`)

			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("$ is an invalid argument in second call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				globalvar result = {value: 0}

				fn one(){
					return Array("1", nil)
				}
	
				fn addOne(i %int){
					result.value = (i + int)
				}
	
				one | addOne $
				return $result.value
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			secondCall := parse.FindNodes(n.Statements[3], (*parse.CallExpression)(nil), nil)[1]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(secondCall.Arguments[0], state, FmtInvalidArg(0, NewString("1"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("pipe statement should not be impacted by previous pipe statements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn idt(arg){
					return Array(arg, nil)
				}

				idt int | idt $
				result = | idt "a" | idt $

				return result
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("anonymous variable should not be defined after pipe statement", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn idt(arg){
					return Array(arg, nil)
				}

				idt int | idt $

				return $
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			varIdent := parse.FindNodes(n, (*parse.Variable)(nil), nil)[1]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(varIdent, state, fmtVarIsNotDeclared("")),
			}, state.errors())

			assert.Equal(t, ANY, res)
		})

	})

	t.Run("if statement", func(t *testing.T) {

		t.Run("test is a boolean", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if true {
					a
				} else {
					b
				}
			`)

			ifStmt := n.Statements[0]
			idents := parse.FindNodes(ifStmt, &parse.IdentifierLiteral{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(idents[0], state, fmtVarIsNotDeclared("a")),
				makeSymbolicEvalError(idents[1], state, fmtVarIsNotDeclared("b")),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("test is not a boolean", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if int {
					a
				} else {
					b
				}
			`)

			ifStmt := n.Statements[0].(*parse.IfStatement)
			idents := parse.FindNodes(ifStmt, &parse.IdentifierLiteral{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ifStmt.Test, state, fmtIfStmtTestShouldBeBoolBut(ANY_INT)),
				makeSymbolicEvalError(idents[1], state, fmtVarIsNotDeclared("a")),
				makeSymbolicEvalError(idents[2], state, fmtVarIsNotDeclared("b")),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("error in test + missing consequent block", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				if int
			`, nil)

			ifStmt := n.Statements[0].(*parse.IfStatement)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ifStmt.Test, state, fmtIfStmtTestShouldBeBoolBut(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("error in test + missing alternate block", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				if int {

				} else
			`, nil)

			ifStmt := n.Statements[0].(*parse.IfStatement)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ifStmt.Test, state, fmtIfStmtTestShouldBeBoolBut(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("join if: variable update", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				if true {
					a = {a: int}
				}
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{}, nil, nil),
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
			), res)
		})

		t.Run("join if: return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if true {
					return 1
				}
				return 2
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1_OR_2, res)
		})

		t.Run("join if-else: variable updated in all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				if true {
					a = {a: int}
				} else {
					a = {b: int}
				}
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"b": ANY_INT}, nil, map[string]Pattern{"b": ANY_INT.Static()}),
			), res)
		})

		t.Run("join if-else: return in all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if true {
					return 1
				} else {
					return 2
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1_OR_2, res)
		})

		t.Run("join if-else-if", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				if true {
					a = {a: int}
				} else if true {
					a = {b: int}
				} else {
					a = {c: int}
				}
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"b": ANY_INT}, nil, map[string]Pattern{"b": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"c": ANY_INT}, nil, map[string]Pattern{"c": ANY_INT.Static()}),
			), res)
		})

		//TODO: add test about joining that checks that the state in alternate is not already joined with the consequent's fork

		t.Run("truthiness narrowing", func(t *testing.T) {

			t.Run("parameter", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %int?){
						if arg? {
							var a %int = arg
						} else {
							//TODO
						}
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("parameter (negated)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %int?){
						if !arg? {
							//TODO
						} else {
							var a %int = arg
						}
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("parameter's property", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %{prop: %int?}){
						if arg.prop? {
							var a %int = arg.prop
						} else {
							//TODO
						}
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("parameter's optional property", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %{prop?: %int}){
						if arg.?prop? {
							var a %int = arg.prop
						} else {
							//TODO
						}
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("inexisting parameter's property", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %{}){
						if arg.prop? {
							
						} else {
							
						}
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				membExpr := parse.FindNode(n, (*parse.IdentifierMemberExpression)(nil), nil)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(membExpr, state, fmtPropOfDoesNotExist("prop", NewEmptyObject(), "")),
				}, state.errors())
			})

			t.Run("variable of static type %int? with nil value", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var v %int? = nil

					if v? {
						var a %never = v
					} else {
						var a %(nil) = v
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("variable of static type %int? with unknown value", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					fn(v %int?){
						if v? {
							var a %int = v
						} else {
							# TODO var a %(nil) = v
						}
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("non existing variable (identifier)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if v? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
			})

			t.Run("non existing variable (local var)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if $v? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
			})

			t.Run("non existing variable (global var)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if $v? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
			})

			t.Run("property of non existing variable (identifier)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if v.a? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
			})

			t.Run("property of non existing variable (local var)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if $v.a? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
			})

			t.Run("property of non existing variable (global var)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if $v.a? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
			})
		})

		t.Run("type narrowing", func(t *testing.T) {

			t.Run("binary match expression narrows the type of a variable (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a match %int) {
						var b %int = a
					} else {
						var b %bool = a
					}
				`)

				state.setGlobal("a", NewMultivalue(ANY_INT, ANY_BOOL), GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary match expression narrows the type of a variable (%(1))", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a match %(1)) {
						var b %int = a
					} else {
						var b %| int | bool = a
					}
				`)

				state.setGlobal("a", NewMultivalue(ANY_INT, ANY_BOOL), GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("negated binary match expression narrows the type of a variable (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if !(a match %int) {
						var b %bool = a
					} else {
						var b %int = a
					}
				`)

				state.setGlobal("a", NewMultivalue(ANY_INT, ANY_BOOL), GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary match expression narrows the type of a variable: (object pattern literal)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a match %{a: int, b: [3]}){
						var b %{a: int, b: [3]} = a
					}
				`)

				state.setGlobal("a", ANY, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary match expression narrows the type of a variable: (list pattern literal)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a match %[]%object){
						var b %[]%object = a
					}
				`)

				state.setGlobal("a", ANY, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary match expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a.prop match %int) {
						var b %int = a.prop
					}
				`)

				object := NewInexactObject(map[string]Serializable{"prop": ANY_SERIALIZABLE}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary == expression narrows the type of a variable (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a == 1) {
						var b %int = a
					} else {
						var b %bool = a
					}
				`)

				state.setGlobal("a", NewMultivalue(NewInt(1), TRUE), GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("negated binary == expression narrows the type of a variable (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a != 1) {
						var b %bool = a
					} else {
						var b %int = a
					}
				`)

				state.setGlobal("a", NewMultivalue(NewInt(1), TRUE), GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary != expression narrows the type of a variable (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a != 1) {
						var b %bool = a
					} else {
						var b %int = a
					}
				`)

				state.setGlobal("a", NewMultivalue(NewInt(1), TRUE), GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("negated binary != expression narrows the type of a variable (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if !(a != 1) {
						var b %int = a
					} else {
						var b %bool = a
					}
				`)

				state.setGlobal("a", NewMultivalue(NewInt(1), TRUE), GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary == expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a.prop == 1) {
						var b %int = a.prop
						var obj {prop: int} = a
					} else {
						var b %bool = a.prop
						var obj {prop: bool} = a
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE)),
				}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("negated binary == expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if !(a.prop == 1) {
						var b %bool = a.prop
						var obj {prop: bool} = a
					} else {
						var b %int = a.prop
						var obj {prop: int} = a
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE))}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary != expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a.prop != 1) {
						var b %bool = a.prop
						var obj {prop: bool} = a
					} else {
						var b %int = a.prop
						var obj {prop: int} = a
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE))}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("negated binary != expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if !(a.prop != 1) {
						var b %int = a.prop
						var obj {prop: int} = a
					} else {
						var b %bool = a.prop
						var obj {prop: bool} = a
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE))}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})
		})

		t.Run("binary == expression narrows the type of a property of a property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if (a.inner.prop == 1) {
					var b %int = a.inner.prop
					var obj {inner: object} = a
				} else {
					var b %bool = a.inner.prop
					var obj {inner: object} = a
				}
			`)

			object := NewInexactObject2(map[string]Serializable{
				"inner": NewInexactObject2(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE)),
				}),
			})

			state.setGlobal("a", object, GlobalConst)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("negated binary == expression narrows the type of a property of a property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if !(a.inner.prop == 1) {
					var b %bool = a.inner.prop
					var obj {inner: object} = a
				} else {
					var b %int = a.inner.prop
					var obj {inner: object} = a
				}
			`)

			object := NewInexactObject2(map[string]Serializable{
				"inner": NewInexactObject2(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE)),
				}),
			})

			state.setGlobal("a", object, GlobalConst)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("binary != expression narrows the type of a property of a property of a property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if (a.inner.prop != 1) {
					var b %bool = a.inner.prop
					var obj {inner: object} = a
				} else {
					var b %int = a.inner.prop
					var obj {inner: object} = a
				}
			`)

			object := NewInexactObject2(map[string]Serializable{
				"inner": NewInexactObject2(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE)),
				}),
			})

			state.setGlobal("a", object, GlobalConst)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("negated binary != expression narrows the type of a property of a property of a property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if !(a.inner.prop != 1) {
					var b %int = a.inner.prop
					var obj {inner: object} = a
				} else {
					var b %bool = a.inner.prop
					var obj {inner: object} = a
				}
			`)

			object := NewInexactObject2(map[string]Serializable{
				"inner": NewInexactObject2(map[string]Serializable{
					"prop": AsSerializableChecked(NewMultivalue(NewInt(1), TRUE)),
				}),
			})

			state.setGlobal("a", object, GlobalConst)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("retrieving the value of nodes should work", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = 1
				b = 2
				if true {
					a
				} else {
					b
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Nil(t, res)

			identA := parse.FindIdentWithName(n, "a")

			calleeValue, ok := state.symbolicData.GetMostSpecificNodeValue(identA)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, INT_1, calleeValue)
		})

		t.Run("global scope data should be preserved inside the statement", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){

				}
				if true {
					f()
				} else {
					f()	
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Nil(t, res)

			callExprs, ancestors := parse.FindNodesAndChains(n, (*parse.CallExpression)(nil), nil)

			for i, callExpr := range callExprs {
				callExprAncestors := ancestors[i]
				callee := callExpr.Callee
				calleeAncestors := append(slices.Clone(callExprAncestors), callExpr)

				scopeData, ok := state.symbolicData.GetGlobalScopeData(callee, calleeAncestors)
				if !assert.True(t, ok) {
					return
				}

				var data VarData

				for _, varInfo := range scopeData.Variables {
					if varInfo.Name == "f" {
						data = varInfo
						break
					}
				}

				if !assert.NotZero(t, data) {
					return
				}

				assert.IsType(t, (*InoxFunction)(nil), data.Value)
				assert.NotZero(t, data.DefinitionPosition)
			}
		})

		t.Run("local scope data should be preserved inside the statement", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = 1
				if true {
					$a
				} else {
					$a
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Nil(t, res)

			blocks, blockAncestors := parse.FindNodesAndChains(n, (*parse.Block)(nil), nil)
			vars, varAncestors := parse.FindNodesAndChains(n, (*parse.Variable)(nil), nil)

			//Check the local scope data for the if clause at the block.

			data, ok := state.symbolicData.GetLocalScopeData(blocks[0], blockAncestors[0])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the if clause at the statement.

			data, ok = state.symbolicData.GetLocalScopeData(vars[0], varAncestors[0])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the else clause at the block.

			data, ok = state.symbolicData.GetLocalScopeData(blocks[1], blockAncestors[1])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the else clause at the statement.

			data, ok = state.symbolicData.GetLocalScopeData(vars[1], varAncestors[1])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)
		})
	})

	t.Run("if expression", func(t *testing.T) {

		t.Run("no else", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				(if true int)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("error in test + missing consequent", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				(if int)
			`, nil)

			ifExpr := n.Statements[0].(*parse.IfExpression)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ifExpr.Test, state, fmtIfExprTestShouldBeBoolBut(ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("if-else", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				(if false int else false)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(ANY_INT, FALSE), res)
		})

		t.Run("error in test + missing alternate", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				(if int int else)
			`, nil)

			ifExpr := n.Statements[0].(*parse.IfExpression)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ifExpr.Test, state, fmtIfExprTestShouldBeBoolBut(ANY_INT)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("truthiness narrowing", func(t *testing.T) {

			t.Run("parameter", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %int?){
						return (if arg? arg else false)
					}
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewMultivalue(ANY_INT, FALSE), res.(*InoxFunction).result)
			})

			t.Run("parameter (negated)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %int?){
						return (if !arg? false else arg)
					}
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewMultivalue(FALSE, ANY_INT), res.(*InoxFunction).result)
			})

			t.Run("parameter field", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(arg %{prop: %int?}){
						return (if arg.prop? arg.prop else false)
					}
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewMultivalue(ANY_INT, FALSE), res.(*InoxFunction).result)
			})

			t.Run("variable of static type %int? with nil value", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var v %int? = nil

					return (if v? v else false)
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewMultivalue(NEVER, FALSE), res)
			})

			t.Run("variable of static type %int? with unknown value", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return fn(v %int?){
						return (if v? v else false)
					}
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
				expectedFn := &InoxFunction{
					node:           fnExpr,
					nodeChunk:      n,
					parameters:     []Value{NewMultivalue(ANY_INT, Nil)},
					parameterNames: []string{"v"},
					result:         NewMultivalue(ANY_INT, FALSE),
				}
				assert.Equal(t, expectedFn, res)
			})

			t.Run("non existing variable (identifier)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return (if v? v else false)
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
				assert.Equal(t, ANY, res)
			})

			t.Run("non existing variable (local var)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return (if $v? $v else false)
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
				assert.Equal(t, ANY, res)
			})

			t.Run("non existing variable (global var)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					return (if $v? $v else false)
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
				assert.Equal(t, ANY, res)
			})
		})

		t.Run("the expected value constraint should be passed to the consequent and the alternate", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var b ({a: int}) = (if true {a: true} else {a: false})
			`, nil)

			boolLiterals := parse.FindNodes(n, (*parse.BooleanLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(boolLiterals[1], state, fmtNotAssignableToPropOfType(TRUE, ANY_INT)),
				makeSymbolicEvalError(boolLiterals[2], state, fmtNotAssignableToPropOfType(FALSE, ANY_INT)),
			}, state.errors())
		})

		t.Run("mismatches should be reported at the positions of the consequent and alternate", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var b bool = (if true 1 else 2)
			`, nil)

			intLiterals := parse.FindNodes(n, (*parse.IntLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiterals[0], state, fmtValueIsAnXButYWasExpected(INT_1, ANY_BOOL)),
				makeSymbolicEvalError(intLiterals[1], state, fmtValueIsAnXButYWasExpected(INT_2, ANY_BOOL)),
			}, state.errors())
		})
	})

	t.Run("assignment", func(t *testing.T) {
		t.Run("assignment in if statement: variable LHS: RHS has incompatible type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = []
				if true {
					v = 3
				}
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(
					&Int{hasValue: true, value: 3}, &TypePattern{val: &List{generalElement: ANY_SERIALIZABLE}},
				)),
			}, state.errors())
			assert.Equal(t, &List{elements: []Serializable{}}, res)
		})

		t.Run("local variable LHS", func(t *testing.T) {

			t.Run("value not assignable to type (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var a %{a: str} = {a: "x"}; 
					$a = {a: 1}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
				}, state.errors())
			})

			t.Run("+= assignment: invalid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$a = 0
					$a += true
					return $a
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT),
				}, state.errors())
				assert.Equal(t, NewInt(0), res)
			})

			t.Run("+= assignment: valid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$a = 2
					$a += 1
					return $a
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})

			t.Run("+= assignment: known integer + unknown integer", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$a = 1
					$a += int
					return $a
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})

			t.Run("+= assignment: unknown integer + known integer", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$a = int
					$a += 1
					return $a
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})

			t.Run("+= assignment: known integer + integer multivalue", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$a = 1
					$a += multi_int
					return $a
				`)
				state.setGlobal("multi_int", NewMultivalue(INT_1, INT_2), GlobalConst)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})

			t.Run("+= assignment: integer multivalue + known integer", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var a int = multi_int
					$a += 1
					return $a
				`)
				state.setGlobal("multi_int", NewMultivalue(INT_1, INT_2), GlobalConst)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})

			t.Run("+= assignment: inside a loop, RHS is an integer multivalue", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$a = 10
					for n in [1, 2, 3] {
						$a += n
					}
					return $a
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})

			t.Run("+= assignment: inside a loop, RHS is an integer multivalue", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$a = 1
					for n in list {
						$a += n
					}
					return $a
				`)
				state.setGlobal("list", NewListOf(AsSerializableChecked(NewMultivalue(INT_1, INT_2))), GlobalConst)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})
		})

		t.Run("variable LHS", func(t *testing.T) {

			t.Run("value not assignable to type (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var a %{a: str} = {a: "x"}; 
					a = {a: 1}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
				}, state.errors())
			})

			t.Run("+= assignment: invalid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					a = 0
					a += true
					return a
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT),
				}, state.errors())
				assert.Equal(t, NewInt(0), res)
			})

			t.Run("+= assignment: valid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					a = 0
					a += 1
					return a
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})
		})

		t.Run("member expression LHS", func(t *testing.T) {
			t.Run("value not assignable to type (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var o %{a: {b: str}} = {a: {b: "x"}}; 
					$o.a = {b: 1}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
				}, state.errors())
			})

			t.Run("value not assignable to type of struct field", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					struct MyStruct {
						a int
					}

					ptr = new MyStruct
					$ptr.a = true
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				assignment := parse.FindNode(n, (*parse.Assignment)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignment, state, fmtNotAssignableToFieldOfType(ANY_BOOL, BUILTIN_COMPTIME_TYPES["int"])),
				}, state.errors())
			})

			t.Run("struct field with builtin type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					struct MyStruct {
						a int
					}

					ptr = new MyStruct
					$ptr.a = 1
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("struct field with struct pointer type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					struct Inner {
						a int
					}

					struct MyStruct {
						inner *Inner
					}

					ptr = new MyStruct
					$ptr.inner = new Inner
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("property of shared object", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					shared.a = 1
				`)
				sharedObject := NewInexactObject(map[string]Serializable{
					"a": ANY_INT,
				}, nil, map[string]Pattern{
					"list": &TypePattern{val: ANY_INT},
				})
				sharedObject = sharedObject.Share(state).(*Object)

				state.setGlobal("shared", sharedObject, GlobalConst)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("useless deep mutation of a shared object property's value should be an error", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$shared.list[0].a = 1
				`)
				sharedObject := NewInexactObject(map[string]Serializable{
					"list": NewListOf(NewInexactObject(map[string]Serializable{
						"a": ANY_INT,
					}, nil, map[string]Pattern{
						"a": &TypePattern{val: ANY_INT},
					})),
				}, nil, map[string]Pattern{
					"list": NewListPatternOf(&TypePattern{val: ANY_INT}),
				})
				sharedObject = sharedObject.Share(state).(*Object)

				state.setGlobal("shared", sharedObject, GlobalConst)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				propIdent := parse.FindIdentWithName(n, "list")

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(propIdent, state, fmtUselessMutationInClonedPropValue("list")),
				}, state.errors())
			})

		})

		t.Run("identifier member expression LHS", func(t *testing.T) {
			t.Run("value not assignable to type (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var o %{a: {b: str}} = {a: {b: "x"}}; 
					o.a = {b: 1}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
				}, state.errors())
			})

			t.Run("value not assignable to type of struct field", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					struct MyStruct {
						a int
					}

					ptr = new MyStruct
					ptr.a = true
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				assignment := parse.FindNode(n, (*parse.Assignment)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignment, state, fmtNotAssignableToFieldOfType(ANY_BOOL, BUILTIN_COMPTIME_TYPES["int"])),
				}, state.errors())
			})

			t.Run("struct field", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					struct MyStruct {
						a int
					}

					ptr = new MyStruct
					ptr.a = 1
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("property of shared object", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					shared.a = 1
				`)
				sharedObject := NewInexactObject(map[string]Serializable{
					"a": ANY_INT,
				}, nil, map[string]Pattern{
					"list": &TypePattern{val: ANY_INT},
				})
				sharedObject = sharedObject.Share(state).(*Object)

				state.setGlobal("shared", sharedObject, GlobalConst)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("1 property: value assignable to type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var o {a: int} = {a: 1}; 
					o.a = 2
					return o
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				if !assert.IsType(t, (*Object)(nil), res) {
					return
				}

				obj := res.(*Object)
				assert.Equal(t, NewInt(2), obj.Prop("a"))
			})

			t.Run("1 property: useless deep mutation of a shared object property's value should be an error", func(t *testing.T) {
				//TODO
			})

			t.Run("2 properties: value assignable to type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var o {a: {b: int}} = {a: {b: 1}}; 
					o.a.b = 2
					return o
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				if !assert.IsType(t, (*Object)(nil), res) {
					return
				}

				obj := res.(*Object)
				assert.Equal(t, NewInt(2), obj.Prop("a").(*Object).Prop("b"))
			})

			t.Run("3 properties: value assignable to type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var o {a: {b: {c: int}}} = {a: {b: {c: 1}}}; 
					o.a.b.c = 2
					return o
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				if !assert.IsType(t, (*Object)(nil), res) {
					return
				}

				obj := res.(*Object)
				assert.Equal(t, NewInt(2), obj.Prop("a").(*Object).Prop("b").(*Object).Prop("c"))
			})
		})

		t.Run("index expression LHS with known index & element", func(t *testing.T) {
			t.Run("same type RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[0] = 0
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("super type RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[0] = int
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("sub type RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [int]
					list[0] = 1
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewList(ANY_INT), res)
			})

			t.Run("RHS of same static type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[0] = 1
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("two elements: RHS equal to first element", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0, 1]
					list[0] = 0
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewList(NewInt(0), NewInt(1)), res)
			})

			t.Run("two elements: RHS equal to second element", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0, 1]
					list[0] = 1
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("RHS of invalid type relative to static type of LHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list [0] = [0]
					list[0] = 1
					return list
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, fmtNotAssignableToElementOfValue(NewInt(1), NewInt(0))),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("RHS of invalid type relative to element", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[0] = 'a'
					return list
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, fmtNotAssignableToElementOfValue(NewRune('a'), ANY_INT)),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("RHS of invalid type relative to element (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [{a: "x"}]
					list[0] = {a: 1}
					return list
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfType(NewInt(1), NewString("x"))),
				}, state.errors())
			})

			t.Run("readonly LHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					fn f(list readonly [int]){
						list[0] = 2
					}
					return f([1])
				`)
				assignement := parse.FindNode(n, (*parse.Assignment)(nil), nil)

				_, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Left, state, ErrReadonlyValueCannotBeMutated.Error()),
				}, state.errors())
			})

			t.Run("non-serializable RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list = [serializable]
					list[0] = go do {}
					return list
				`)
				state.setGlobal("serializable", ANY_INT, GlobalConst)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE),
				}, state.errors())
				assert.Equal(t, NewList(ANY_INT), res)
			})

			t.Run("non-watchable mutable RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list = [serializable]
					list[0] = val
					return list
				`)
				state.setGlobal("serializable", ANY_INT, GlobalConst)
				state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE),
				}, state.errors())
				assert.Equal(t, NewList(ANY_INT), res)
			})

			t.Run("+= assignment: invalid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					a = [0]
					a[0] += true
					return a
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("+= assignment: valid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					a = [0]
					a[0] += 1
					return a
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("index is out of bounds (negative)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[-1] = 0
					return list
				`)
				res, err := symbolicEval(n, state)

				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool, _ []parse.Node) bool {
					return n.Value == -1
				})

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(index, state, INDEX_IS_OUT_OF_BOUNDS),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("index is out of bounds (positive)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[-1] = 0
					return list
				`)
				res, err := symbolicEval(n, state)
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool, _ []parse.Node) bool {
					return n.Value == -1
				})

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(index, state, INDEX_IS_OUT_OF_BOUNDS),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("useless deep mutation of a shared object property's value should be an error - same value", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					shared.list[0] = 0
				`)
				sharedObject := NewInexactObject(map[string]Serializable{
					"list": NewList(ANY_INT),
				}, nil, map[string]Pattern{
					"list": NewListPatternOf(&TypePattern{val: ANY_INT}),
				})
				sharedObject = sharedObject.Share(state).(*Object)

				state.setGlobal("shared", sharedObject, GlobalConst)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				propIdent := parse.FindIdentWithName(n, "list")

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(propIdent, state, fmtUselessMutationInClonedPropValue("list")),
				}, state.errors())
			})

			t.Run("useless deep mutation of a shared object property's value should be an error - diffrent value", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					shared.list[0] = 1
				`)
				sharedObject := NewInexactObject(map[string]Serializable{
					"list": NewList(ANY_INT),
				}, nil, map[string]Pattern{
					"list": NewListPatternOf(&TypePattern{val: ANY_INT}),
				})
				sharedObject = sharedObject.Share(state).(*Object)

				state.setGlobal("shared", sharedObject, GlobalConst)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				propIdent := parse.FindIdentWithName(n, "list")

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(propIdent, state, fmtUselessMutationInClonedPropValue("list")),
				}, state.errors())
			})
		})

		t.Run("index expression LHS with unknown index & several elements", func(t *testing.T) {
			t.Run("same type RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0, 1]
					list[int] = 0
					return list
				`)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("super type RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0, 1]
					list[0] = int
					return list
				`)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("sub type RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [int, int]
					list[int] = 1
					return list
				`)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("RHS of same static type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0, 1]
					list[int] = 1
					return list
				`)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("RHS of invalid type relative to static type of LHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list [0, 1] = [0, 1]
					list[int] = 0
					return list
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENT),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0), NewInt(1)), res)
			})

			t.Run("RHS of invalid type relative to element", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0, 1]
					list[int] = 'a'
					return list
				`)

				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, fmtNotAssignableToElementOfValue(NewRune('a'), ANY_INT)),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0), NewInt(1)), res)
			})

			t.Run("RHS of invalid type relative to element (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [{a: "x"}, {a: "x"}]
					list[int] = {a: 1}
					return list
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfType(NewInt(1), NewString("x"))),
				}, state.errors())
			})

			t.Run("non-serializable RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list = [serializable, serializable]
					list[int] = go do {}
					return list
				`)

				state.setGlobal("serializable", ANY_INT, GlobalConst)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE),
				}, state.errors())
				assert.Equal(t, NewList(ANY_INT, ANY_INT), res)
			})

			t.Run("non-watchable mutable RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list = [serializable, serializable]
					list[int] = val
					return list
				`)

				state.setGlobal("serializable", ANY_INT, GlobalConst)
				state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE),
				}, state.errors())
				assert.Equal(t, NewList(ANY_INT, ANY_INT), res)
			})

			t.Run("+= assignment: invalid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					a = [0, 1]
					a[int] += true
					return a
				`)

				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("+= assignment: valid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					a = [0, 1]
					a[int] += 1
					return a
				`)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("useless deep mutation of a shared object property's value should be an error", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					shared.list[0:1] = [1]
				`)
				sharedObject := NewInexactObject(map[string]Serializable{
					"list": NewListOf(ANY_INT),
				}, nil, map[string]Pattern{
					"list": NewListPatternOf(&TypePattern{val: ANY_INT}),
				})
				sharedObject = sharedObject.Share(state).(*Object)

				state.setGlobal("shared", sharedObject, GlobalConst)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				propIdent := parse.FindIdentWithName(n, "list")

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(propIdent, state, fmtUselessMutationInClonedPropValue("list")),
				}, state.errors())
			})
		})

		t.Run("slice expression LHS with known indexes", func(t *testing.T) {
			t.Run("RHS should be a sequence", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[0:1] = {}
					return list
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state,
						fmtSequenceExpectedButIs(NewEmptyObject())),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("super type RHS element", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[0:1] = [int]
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("sub type RHS element", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [int]
					list[0:1] = [1]
					return list
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("RHS element of invalid type", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					slice = ['a'] # we define a variable in order to avoid mismatches inside the list literal
					list[0:1] = slice
					return list
				`)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[2], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state,
						fmtSeqOfXNotAssignableToSliceOfTheValue(NewRune('a'), NewListOf(ANY_INT))),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("RHS element of invalid type (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [{a: "x"}]
					list[0:1] = [{a: 1}]
					return list
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfType(NewInt(1), ANY_STRING)),
				}, state.errors())
			})

			t.Run("static type LHS with known length should conservatively make the assignment invalid", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list [0, 1] = [0, 1]
					list[0:1] = [0]
					return list
				`)
				state.setGlobal("int2", ANY_INT, GlobalConst)
				res, err := symbolicEval(n, state)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				msg := fmtSeqOfXNotAssignableToSliceOfTheValue(NewInt(0), NewList(NewInt(0), NewInt(1)))
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Right, state, msg),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0), NewInt(1)), res)
			})

			t.Run("readonly LHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					fn f(list readonly [0, 1]){
						list[0:1] = [0]
					}
				`)
				state.setGlobal("int2", ANY_INT, GlobalConst)
				_, err := symbolicEval(n, state)
				assignement := parse.FindNode(n, (*parse.Assignment)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement.Left, state, ErrReadonlyValueCannotBeMutated.Error()),
				}, state.errors())
			})

			t.Run("non-serializable RHS element", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					var list = [serializable]
					list[0:1] = Array(go do {})
					return list
				`)
				state.setGlobal("serializable", ANY_INT, GlobalConst)
				state.setGlobal("Array", WrapGoFunction(func(ctx *Context, elements ...Value) *Array {
					return NewArray()
				}), GlobalConst)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE),
				}, state.errors())
				assert.Equal(t, NewList(ANY_INT), res)
			})

			t.Run("non-watchable mutable RHS element", func(t *testing.T) {
				//TODO: fix
				t.Skip()
				n, state := MakeTestStateAndChunk(`
					var list = [serializable]
					list[0:1] = [val]
					return list
				`)
				state.setGlobal("serializable", ANY_INT, GlobalConst)
				state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
				assignement := parse.FindNode(n.Statements[1], (*parse.Assignment)(nil), nil)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(assignement, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE),
				}, state.errors())
				assert.Equal(t, NewList(ANY_INT), res)
			})

			t.Run("RHS sequence has less than the expected number of elements", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [int, int]
					list[0:2] = [0]
					return list
				`)
				res, err := symbolicEval(n, state)
				listLit := parse.FindNode(n.Statements[1], (*parse.ListLiteral)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(listLit, state, fmtRHSSequenceShouldHaveLenOf(2)),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("RHS sequence has more than the expected number of elements", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list[0:1] = [0, 1, 0]
					return list
				`)
				state.setGlobal("list", NewListOf(ANY_INT), GlobalConst)
				res, err := symbolicEval(n, state)
				listLit := parse.FindNode(n, (*parse.ListLiteral)(nil), nil)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(listLit, state, fmtRHSSequenceShouldHaveLenOf(1)),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})

			t.Run("start index is out of bounds (negative)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[-1:1] = [0]
					return list
				`)
				res, err := symbolicEval(n, state)
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool, _ []parse.Node) bool {
					return n.Value == -1
				})

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(index, state, START_INDEX_IS_OUT_OF_BOUNDS),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("start index is out of bounds (positive)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0]
					list[2:3] = [0]
					return list
				`)
				res, err := symbolicEval(n, state)
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool, _ []parse.Node) bool {
					return n.Value == 2
				})

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(index, state, START_INDEX_IS_OUT_OF_BOUNDS),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
			})

			t.Run("end index should be less than start index", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					list = [0, 1]
					list[1:0] = [1]
					return list
				`)
				res, err := symbolicEval(n, state)
				index := parse.FindIntLiteralWithValue(n.Statements[1], 0)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(index, state, END_INDEX_SHOULD_BE_LESS_OR_EQUAL_START_INDEX),
				}, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)
			})
		})
	})

	t.Run("multi assignment", func(t *testing.T) {

		t.Run("RHS is too short (int variable)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assign first = []
			`)
			res, err := symbolicEval(n, state)
			stmt := n.Statements[0]

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(stmt, state, fmtSequenceShouldHaveLengthGreaterOrEqualTo(1)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("RHS is too short (2 variables)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assign first second = [int]
			`)
			res, err := symbolicEval(n, state)
			stmt := n.Statements[0]

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(stmt, state, fmtSequenceShouldHaveLengthGreaterOrEqualTo(2)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("RHS is too short (2 variables) but nillable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assign? first second = [int]
			`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("unknown-length list of integers : nillable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assign? first second = list
				return [first, second]
			`)
			state.setGlobal("list", NewListOf(ANY_INT), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(
				AsSerializableChecked(NewMultivalue(ANY_INT, Nil)),
				AsSerializableChecked(NewMultivalue(ANY_INT, Nil)),
			), res)
		})

		t.Run("RHS is not a sequence", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assign? first second = int
				return Array(first, second)
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			multiAssignment := parse.FindNode(n, (*parse.MultiAssignment)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(multiAssignment, state, fmtSeqExpectedButIs(ANY_INT)),
			}, state.errors())
			assert.Equal(t, NewArray(ANY, ANY), res)
		})

		t.Run("RHS is an array", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assign? first second = array
				return [first, second]
			`)
			state.setGlobal("array", NewArray(ANY_INT, ANY_INT), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(ANY_INT, ANY_INT), res)
		})

		t.Run("RHS is an array returned by a function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return Array(1, 2)
				}
				assign first second = f()
				return [first, second]
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(INT_1, INT_2), res)
		})

		t.Run("RHS is an array ending with an error returned by a function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return Array(1, Error("x"))
				}
				assign first second = f()
				return [first, second]
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)
			state.setGlobal("Error", WrapGoFunction(NewError), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(INT_1, NewError(NewString("x"))), res)
		})
	})

	t.Run("for statement", func(t *testing.T) {
		t.Run("iterated value is not iterable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for i, e in int {
	
				} 
			`)
			res, err := symbolicEval(n, state)
			forStmt := n.Statements[0].(*parse.ForStatement)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(forStmt.IteratedValue, state, fmtXisNotIterable(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("object iteration: keys are strings", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for k, v in {a: int} {
					return k
				}
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(ANY_STRING, Nil), res)
		})

		t.Run("key & element variables should be present in local scope data", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for k, v in {a: int} {
					return k
				} 
			`)

			symbolicEval(n, state)

			stmt, chain := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)
			data, ok := state.symbolicData.GetLocalScopeData(stmt, chain)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 2) {
				return
			}

			if data.Variables[0].Name == "k" {
				assert.Equal(t, "v", data.Variables[1].Name)
			} else {
				assert.Equal(t, "v", data.Variables[0].Name)
				assert.Equal(t, "k", data.Variables[1].Name)
			}
		})

		t.Run("key & element variables should be present in local scope data: statement preceded by assignment", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = 1
				for k, v in {a: int} {
					return k
				} 
			`)

			symbolicEval(n, state)

			stmt, chain := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)
			data, ok := state.symbolicData.GetLocalScopeData(stmt, chain)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 3) {
				return
			}
		})

		t.Run("list iteration: keys are integers and values have type of element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for i, e in [["a"], [int]] {
					return [i, e]
				} 
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			expectedResultFromForStmt := NewList(
				ANY_INT, AsSerializableChecked(NewMultivalue(NewList(NewString("a")), NewList(ANY_INT))),
			)

			assert.Equal(t, NewMultivalue(expectedResultFromForStmt, Nil), res)
		})

		t.Run("empty dictionary iteration: keys should be any", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for k, v in :{} {
					return k
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("path dictionary iteration: keys should be paths", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for k, v in :{./a: int, ./b: 2} {
					return k
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			s := Stringify(res)
			if strings.Index(s, "./a") < strings.Index(s, "./b") {
				assert.Equal(t, NewMultivalue(NewPath("./a"), NewPath("./b"), Nil), res)
			} else {
				assert.Equal(t, NewMultivalue(NewPath("./b"), NewPath("./a"), Nil), res)
			}
		})

		t.Run("int range iteration: keys and values are integers", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for i, e in 1..3 {
					return [i, e]
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			pattern := NewIntRangePattern(NewIntRange(INT_1, INT_3, false))

			expectedResultFromForStmt := NewList(ANY_INT, ANY_INT.WithMatchingPattern(pattern))
			assert.Equal(t, NewMultivalue(expectedResultFromForStmt, Nil), res)
		})

		t.Run("rune range iteration: keys are integers and values are runes", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for i, r in 'a'..'z' {
					return [i, r]
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			expectedResultFromForStmt := NewList(ANY_INT, ANY_RUNE)
			assert.Equal(t, NewMultivalue(expectedResultFromForStmt, Nil), res)
		})

		t.Run("streamable iteration", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for e in streamable {
					return e
				} 
			`)
			state.setGlobal("streamable", ANY_STREAM_SOURCE, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("key variable should not be provided when iterating over a streamable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for k, e in streamable {
					
				} 
			`)
			state.setGlobal("streamable", ANY_STREAM_SOURCE, GlobalConst)

			_, err := symbolicEval(n, state)
			keyVar := parse.FindIdentWithName(n, "k")

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(keyVar, state, KEY_VAR_SHOULD_BE_PROVIDED_ONLY_WHEN_ITERATING_OVER_AN_ITERABLE),
			}, state.errors())
		})

		t.Run("chunked streamable iteration", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for chunked chunk in streamable {
					return chunk
				} 
			`)
			state.setGlobal("streamable", NewWatcher(ANY_STR_PATTERN), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			expectedResultFromForStmt := NewTupleOf(ANY_STRING)
			assert.Equal(t, NewMultivalue(expectedResultFromForStmt, Nil), res)
		})

		t.Run("error in head + missing body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				for i, e in int
			`, nil)
			res, err := symbolicEval(n, state)
			forStmt := n.Statements[0].(*parse.ForStatement)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(forStmt.IteratedValue, state, fmtXisNotIterable(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("state should be forked", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = #a
				for int..2 {
					a = #b
				} 
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(&Identifier{name: "a"}, &Identifier{name: "b"}), res)
		})

		t.Run("no iterated value", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				(for e in)
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})
	})

	t.Run("for expression", func(t *testing.T) {

		t.Run("iterated value is not iterable", func(t *testing.T) {

			n, state := MakeTestStateAndChunk(`
				(for i, e in int {}) 
			`)
			res, err := symbolicEval(n, state)
			forStmt := n.Statements[0].(*parse.ForExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(forStmt.IteratedValue, state, fmtXisNotIterable(ANY_INT)),
			}, state.errors())
			assert.Equal(t, EMPTY_LIST, res)
		})

		t.Run("arrow: key & element variables should be present in local scope data", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				(for k, v in {a: int} => [k, v])
			`)

			_, err := symbolicEval(n, state)

			if !assert.NoError(t, err) {
				return
			}

			arrayLiteral, chain := parse.FindNodeAndChain(n, (*parse.ListLiteral)(nil), nil)
			data, ok := state.symbolicData.GetLocalScopeData(arrayLiteral, chain)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 2) {
				return
			}

			if data.Variables[0].Name == "k" {
				assert.Equal(t, "v", data.Variables[1].Name)
			} else {
				assert.Equal(t, "v", data.Variables[0].Name)
				assert.Equal(t, "k", data.Variables[1].Name)
			}
		})

		t.Run("arrow: the result should be a list containing the produced elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(for r in 'a'..'z' => r)`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewListOf(ANY_RUNE), res)
		})

		t.Run("arrow: the produced elements should be serializable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(for i in 1..3 => go do {})`)

			spawnExpr := parse.FindNode(n, (*parse.SpawnExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spawnExpr, state, ELEMENTS_PRODUCED_BY_A_FOR_EXPR_SHOULD_BE_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, NewListOf(ANY_SERIALIZABLE), res)
		})

		t.Run("body: empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(for r in 'a'..'z' {})`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, EMPTY_LIST, res)
		})

		t.Run("body: direct yielding of a value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(for r in 'a'..'z' { yield r })`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewListOf(ANY_RUNE), res)
		})

		t.Run("body: key & element variables should be present in local scope data", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				(for k, v in {a: int} { yield [k, v] })
			`)

			res, err := symbolicEval(n, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t,
				NewListOf(NewList(ANY_STRING, ANY_SERIALIZABLE)),
				res,
			)

			listLiteral, chain := parse.FindNodeAndChain(n, (*parse.ListLiteral)(nil), nil)
			data, ok := state.symbolicData.GetLocalScopeData(listLiteral, chain)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 2) {
				return
			}

			if data.Variables[0].Name == "k" {
				assert.Equal(t, "v", data.Variables[1].Name)
			} else {
				assert.Equal(t, "v", data.Variables[0].Name)
				assert.Equal(t, "k", data.Variables[1].Name)
			}
		})

		t.Run("body: single yield: conditional", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(for r in 'a'..'z' { if r != 'a' { yield r } })`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewListOf(ANY_RUNE), res)
		})

		t.Run("body: two yields value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(
				for r in 'a'..'z' { 
					if r != 'a' { yield r } 
					yield 1 
				}
			)`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewListOf(AsSerializableChecked(NewMultivalue(ANY_RUNE, INT_1))), res)
		})

		t.Run("yield inside a for expression in the body of another for expression without a yield statement", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(
				for r in 'a'..'z' { 
					# The inner for expression should have no effect on the outer expression.
					(for r in 'a'..'z' { 
						yield 1
					})
				}
			)`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, EMPTY_LIST, res)
		})

		t.Run("yield inside a for expression in the body of another for expression with a yield statement", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(
				for r in 'a'..'z' { 
					# The inner for expression should have no effect on the outer expression.
					(for r in 'a'..'z' { 
						yield 1
					})
					yield 2
				}
			)`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewListOf(AsSerializableChecked(INT_2)), res)
		})

		t.Run("no iterated value", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				(for e in)
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})

		t.Run("arrow: missing value", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				(for e in [] =>)
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})

		t.Run("no body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				(for e in [])
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})
	})

	t.Run("walk statement", func(t *testing.T) {
		t.Run("walked value is not walkable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				path = int
				walk $path entry {
	
				}
			`)

			walkStmt := n.Statements[1].(*parse.WalkStatement)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(walkStmt.Walked, state, fmtXisNotWalkable(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("entries have right value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				walk ./ entry {
					return entry
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			assert.Equal(t, NewMultivalue(WALK_ELEM, Nil), res)
		})

		t.Run("meta", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				walk ./ meta, entry {
					return Array(meta, entry)
				}
			`)
			state.setGlobal("Array", WrapGoFunction(NewArray), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			expectedResultFromWalkStmt := NewArray(ANY, WALK_ELEM)
			assert.Equal(t, NewMultivalue(expectedResultFromWalkStmt, Nil), res)
		})

		t.Run("error in head + missing body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				path = int
				walk $path entry
			`, nil)

			walkStmt := n.Statements[1].(*parse.WalkStatement)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(walkStmt.Walked, state, fmtXisNotWalkable(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("state should be forked", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = #a
				walk ./ entry {
					a = #b
				} 
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(&Identifier{name: "a"}, &Identifier{name: "b"}), res)
		})

		t.Run("no walked value", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				walk
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})

		t.Run("no entry variable", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				walk ./
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})

		t.Run("no body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				walk ./ e
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})
	})

	t.Run("switch statement", func(t *testing.T) {

		t.Run("error in every block (no default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				switch v {
					0 {
						!"s"
					}
					int {
						!"s"
					}
				}
			`)
			unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("error in every block (with default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				switch v {
					0 {
						!"s"
					}
					int {
						!"s"
					}
					defaultcase {
						!"s"
					}
				}
			`)
			unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[2], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("block with an error + missing block", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
			v = int
			switch v {
				0 {
					!"s"
				}
				int
			}
		`, nil)
			unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("join single non-default case: variable update", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				switch int {
					1 {
						a = {a: int}
					}
				}
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{}, nil, nil),
				NewInexactObject(map[string]Serializable{"a": INT_1}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
			), res)
		})

		t.Run("join single non-default case: return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				switch int {
					0 {
						return 1
					}
				}
				return 2
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1_OR_2, res)
		})

		t.Run("join default case: variable update", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				switch int {
					defaultcase {
						a = {a: int}
					}
				}
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, map[string]Pattern{"a": ANY_INT.Static()}), res)
		})

		t.Run("join default case: return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				switch int {
					defaultcase {
						return 1
					}
				}
				return 2
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1, res)
		})

		t.Run("join non-default + default case: variable update in all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				switch int {
					1 {
						a = {a: int}
					}
					defaultcase {
						a = {b: int}
					}
				}
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": INT_1}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"b": ANY_INT}, nil, map[string]Pattern{"b": ANY_INT.Static()}),
			), res)
		})

		t.Run("join single non-default case: return in all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				switch int {
					0 {
						return 1
					}
					defaultcase {
						return 2
					}
				}
				return 3
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1_OR_2, res)
		})

		t.Run("narrowing of variable's value (no default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v){
					switch v {
						1 {
							var int %(1) = v
						}
						"1" {
							var string %("1") = v
						}
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("narrowing of variable's value (with default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v %| int | str | bool){
					switch v {
						1 {
							var int %(1) = v
						}
						"1" {
							var string %("1") = v
						}
						defaultcase {
							var bool %| int | str | bool = v
						}
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("local scope data should be preserved inside all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = 1
				switch int {
					1 {
						$a
					}
					defaultcase {
						$a
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Nil(t, res)

			blocks, blockAncestors := parse.FindNodesAndChains(n, (*parse.Block)(nil), nil)
			vars, varAncestors := parse.FindNodesAndChains(n, (*parse.Variable)(nil), nil)

			//Check the local scope data for the case 1 at the block.

			data, ok := state.symbolicData.GetLocalScopeData(blocks[0], blockAncestors[0])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the case 1 at the statement.

			data, ok = state.symbolicData.GetLocalScopeData(vars[0], varAncestors[0])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the default case at the block.

			data, ok = state.symbolicData.GetLocalScopeData(blocks[1], blockAncestors[1])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the default case at the statement.

			data, ok = state.symbolicData.GetLocalScopeData(vars[1], varAncestors[1])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)
		})

		t.Run("no discriminant", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				switch
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})

	})

	t.Run("switch expression", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				return switch v {}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, Nil, res)
		})

		t.Run("several cases, no default case", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				return switch v {
					1 => 1
					2 => 2
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, NewMultivalue(INT_1, INT_2, Nil), res)
		})

		t.Run("several cases + default case", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				return switch v {
					1 => 1
					2 => 2
					defaultcase => 3
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, NewMultivalue(INT_1, INT_2, INT_3), res)
		})

		t.Run("only default case", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				return switch v {
					defaultcase => 3
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, INT_3, res)
		})

		t.Run("error in every block (no default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				return switch v {
					0 => !"s"
					int => !"s"
				}
			`)
			unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Equal(t, NewMultivalue(ANY_BOOL, Nil), res)
		})

		t.Run("error in every block (with default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = int
				return switch v {
					0 => !"s"
					int => !"s"
					defaultcase => !"s"
				}
			`)
			unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[2], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("the expected value constraint should be passed to the cases", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var b ({a: int}) = switch 1 { 
					1 => {a: true} 
					defaultcase => {a: false}
				)
			`, nil)

			boolLiterals := parse.FindNodes(n, (*parse.BooleanLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(boolLiterals[0], state, fmtNotAssignableToPropOfType(TRUE, ANY_INT)),
				makeSymbolicEvalError(boolLiterals[1], state, fmtNotAssignableToPropOfType(FALSE, ANY_INT)),
			}, state.errors())
		})

		t.Run("mismatches should be reported at the cases's results", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var b bool = switch "a" { 
					"a" => 1
					defaultcase => 2
				)
			`, nil)

			intLiterals := parse.FindNodes(n, (*parse.IntLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiterals[0], state, fmtValueIsAnXButYWasExpected(INT_1, ANY_BOOL)),
				makeSymbolicEvalError(intLiterals[1], state, fmtValueIsAnXButYWasExpected(INT_2, ANY_BOOL)),
			}, state.errors())
		})

		t.Run("no discriminant", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				return switch
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, DEFAULT_SWITCH_MATCH_EXPR_RESULT, res)
		})

	})

	t.Run("match statement", func(t *testing.T) {
		t.Run("join single non-default case: variable update", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				match int_or_str {
					%int {
						a = {a: int_or_str}
					}
				}
				return a
			`)
			state.setGlobal("int_or_str", NewMultivalue(ANY_INT, ANY_STR_LIKE), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{}, nil, nil),
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
			), res)
		})

		t.Run("join single non-default case: return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				match int_or_str {
					%int {
						return 1
					}
				}
				return 2
			`)
			state.setGlobal("int_or_str", NewMultivalue(ANY_INT, ANY_STR_LIKE), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1_OR_2, res)
		})

		t.Run("join default case: variable update", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				match int_or_str {
					defaultcase {
						a = {a: 1}
					}
				}
				return a
			`)
			state.setGlobal("int_or_str", NewMultivalue(ANY_INT, ANY_STR_LIKE), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInexactObject(map[string]Serializable{"a": INT_1}, nil, map[string]Pattern{"a": ANY_INT.Static()}), res)
		})

		t.Run("join default case: return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				match int_or_str {
					defaultcase {
						return 1
					}
				}
				return 2
			`)
			state.setGlobal("int_or_str", NewMultivalue(ANY_INT, ANY_STR_LIKE), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1, res)
		})

		t.Run("join non-default + default case: variable update in all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var a %object = {}
				match int_or_str {
					%int {
						a = {a: 1}
					}
					defaultcase {
						a = {b: 2}
					}
				}
				return a
			`)
			state.setGlobal("int_or_str", NewMultivalue(ANY_INT, ANY_STR_LIKE), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": INT_1}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"b": INT_2}, nil, map[string]Pattern{"b": ANY_INT.Static()}),
			), res)
		})

		t.Run("join single non-default case: return in all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				match int_or_str {
					%int {
						return 1
					}
					defaultcase {
						return 2
					}
				}
				return 3
			`)
			state.setGlobal("int_or_str", NewMultivalue(ANY_INT, ANY_STR_LIKE), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_1_OR_2, res)
		})

		t.Run("error in a case value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = /path
				match v {
					undefined_var {}
				}
			`)

			ident := parse.FindIdentWithName(n, "undefined_var")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("undefined_var")),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("an exact value used as a match case should be serializable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = /path
				non_serializable_value = go do {}

				match v {
					non_serializable_value {}
				}
			`)

			ident := parse.FindIdentWithName(n, "non_serializable_value")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, AN_EXACT_VALUE_USED_AS_MATCH_CASE_SHOULD_BE_SERIALIZABLE),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("error in every block", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = /path
				match v {
					/ {
						!"s"
					}
					/... {
						!"s"
					}
				}
			`)

			unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("single group matching case", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				match /home/user {
					%/home/{:username} m {
						m.username
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("two group matching cases with same variable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				match /home/user {
					%/home/{:username} m {
						m.username
					}
					%/x/{:username} m {
						m.username
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("narrowing of a variable's value (no default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v){
					match v {
						%int {
							var int %int = v
						}
						%str {
							var string %str = v
						}
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("narrowing of a variable's value (with default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v %| int | str | bool){
					match v {
						%int {
							var int %int = v
						}
						%str {
							var string %str = v
						}
						defaultcase {
							var bool %bool = v
						}
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("narrowing of a variable's value (multivalue)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v (| int | str)){
					match v {
						%int {
							var int %int = v
						}
						%str {
							var string %str = v
						}
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("narrowing of property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v %{a: %| %int | %str}){
					match v.a {
						%int {
							var int %int = v.a
						}
						%str {
							var string %str = v.a
						}
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("error in one block + missing block", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = /path
				match v {
					/ {
						!"s"
					}
					/...
				}
			`, nil)

			unaryExpr := parse.FindNode(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExpr, state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("local scope data should be preserved inside all cases", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = 1
				match int {
					1 {
						$a
					}
					defaultcase {
						$a
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Nil(t, res)

			blocks, blockAncestors := parse.FindNodesAndChains(n, (*parse.Block)(nil), nil)
			vars, varAncestors := parse.FindNodesAndChains(n, (*parse.Variable)(nil), nil)

			//Check the local scope data for the case 1 at the block.

			data, ok := state.symbolicData.GetLocalScopeData(blocks[0], blockAncestors[0])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the case 1 at the statement.

			data, ok = state.symbolicData.GetLocalScopeData(vars[0], varAncestors[0])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the default case at the block.

			data, ok = state.symbolicData.GetLocalScopeData(blocks[1], blockAncestors[1])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)

			//Check the local scope data for the default case at the statement.

			data, ok = state.symbolicData.GetLocalScopeData(vars[1], varAncestors[1])
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Variables, 1) {
				return
			}

			assert.Equal(t, "a", data.Variables[0].Name)
		})

		t.Run("no discriminant", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				match
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
		})
	})

	t.Run("match expression", func(t *testing.T) {
		t.Run("join", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return match 1 {
					%int => {a: 1}
					%str => {b: 1}
				}
				return v
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewInexactObject(map[string]Serializable{"a": NewInt(1)}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"b": NewInt(1)}, nil, map[string]Pattern{"b": ANY_INT.Static()}),
				Nil,
			), res)
		})

		t.Run("error in a case value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = /path
				return match v {
					undefined_var => 0
				}
			`)

			ident := parse.FindIdentWithName(n, "undefined_var")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("undefined_var")),
			}, state.errors())
			assert.Equal(t, Nil, res)
		})

		t.Run("an exact value used as a match case should be serializable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = /path
				non_serializable_value = go do {}

				return match v {
					non_serializable_value => 0
				}
			`)

			ident := parse.FindIdentWithName(n, "non_serializable_value")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, AN_EXACT_VALUE_USED_AS_MATCH_CASE_SHOULD_BE_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, Nil, res)
		})

		t.Run("error in every block", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = /path
				return match v {
					/ => !"s"
					/... => !"s"
				}
			`)

			unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
				makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Equal(t, NewMultivalue(ANY_BOOL, Nil), res)
		})

		t.Run("single group matching case", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return match /home/user {
					%/home/{:username} m => m.username
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(ANY_STRING, Nil), res)
		})

		t.Run("two group matching cases with same variable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return match /home/user {
					%/home/{:username} m => m.username
					%/x/{:username} m => m.username
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(ANY_STRING, Nil), res)
		})

		t.Run("narrowing of variable's value (no default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v){
					return match v {
						%int => (v + 1)
						%str => concat v "!"
					}
				}
				return f
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			expectedFn := &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{ANY},
				parameterNames: []string{"v"},
				result:         NewMultivalue(ANY_INT, ANY_STR_LIKE, Nil),
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("narrowing of variable's value (with default case)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v %| int | str | bool){
					return match v {
						%int => (v + 1)
						%str => concat v "!"
						defaultcase => !v # should be a boolean
					}
				}
				return f
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			expectedFn := &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{NewMultivalue(ANY_INT, ANY_STR_LIKE, ANY_BOOL)},
				parameterNames: []string{"v"},
				result:         NewMultivalue(ANY_INT, ANY_STR_LIKE, ANY_BOOL),
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("narrowing of property", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(v %{a: %| %int | %str}){
					return match v.a {
						%int => v.a
						%str => v.a
					}
				}
				return f
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			expectedFn := &InoxFunction{
				node:      fnExpr,
				nodeChunk: n,
				parameters: []Value{
					NewInexactObject(
						map[string]Serializable{
							"a": AsSerializableChecked(NewMultivalue(ANY_INT, ANY_STR_LIKE)),
						},
						nil,
						map[string]Pattern{
							"a": utils.Must(NewUnionPattern(
								[]Pattern{
									state.ctx.ResolveNamedPattern("int"),
									state.ctx.ResolveNamedPattern("str"),
								},
								false,
							)),
						},
					),
				},
				parameterNames: []string{"v"},
				result:         NewMultivalue(ANY_INT, ANY_STR_LIKE, Nil),
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("error in one block + missing block", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = /path
				return match v {
					/ => !"s"
					/...
				}
			`, nil)

			unaryExpr := parse.FindNode(n, (*parse.UnaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExpr, state, fmtOperandOfBoolNegateShouldBeBool(NewString("s"))),
			}, state.errors())
			assert.Equal(t, NewMultivalue(ANY_BOOL, Nil), res)
		})

		t.Run("the expected value constraint should be passed to the cases", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var b ({a: int}) = match 1 { 
					1 => {a: true} 
					defaultcase => {a: false}
				)
			`, nil)

			boolLiterals := parse.FindNodes(n, (*parse.BooleanLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(boolLiterals[0], state, fmtNotAssignableToPropOfType(TRUE, ANY_INT)),
				makeSymbolicEvalError(boolLiterals[1], state, fmtNotAssignableToPropOfType(FALSE, ANY_INT)),
			}, state.errors())
		})

		t.Run("mismatches should be reported at the cases's results", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				var b bool = match "a" { 
					"a" => 1
					defaultcase => 2
				)
			`, nil)

			intLiterals := parse.FindNodes(n, (*parse.IntLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLiterals[0], state, fmtValueIsAnXButYWasExpected(INT_1, ANY_BOOL)),
				makeSymbolicEvalError(intLiterals[1], state, fmtValueIsAnXButYWasExpected(INT_2, ANY_BOOL)),
			}, state.errors())
		})

		t.Run("no discriminant", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				return match
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, DEFAULT_SWITCH_MATCH_EXPR_RESULT, res)
		})
	})

	t.Run("regex literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("%`a`")

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewRegexPattern("a"), res)
	})

	t.Run("object pattern literal", func(t *testing.T) {

		t.Run("spread pattern that is not an object pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{...1}
			`)

			spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, fmtPatternSpreadInObjectPatternShouldBeAnObjectPatternNot(&ExactValuePattern{value: &Int{hasValue: true, value: 1}})),
			}, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("spread object pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{...%{}}
			`)

			//spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			// assert.Equal(t, []SymbolicEvaluationError{
			// 	makeSymbolicEvalError(spreadElem, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT),
			// }, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("spread object pattern matching all objects", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{...%p}
			`)

			state.ctx.AddNamedPattern("p", ANY_OBJECT_PATTERN, false)

			spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_MATCHES_ANY_OBJECT),
			}, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("spread valid object pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{...%{name: %str}}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("spread properties should be unique among spread patterns", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{...%{name: %str}, ...%{name: %int}}
			`)

			secondSpread := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(secondSpread, state, fmtPropertyShouldNotBePresentInSeveralSpreadPatterns("name")),
			}, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("visible properties should have higher priority that spread properties", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{...%{name: %str}, name: %int}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("int")},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("pattern call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{a: %int(0..1)}
			`)

			intRange := NewIntRange(INT_0, INT_1, false)
			patt, _ := state.ctx.ResolveNamedPattern("int").Call(nil, []Value{intRange})

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": patt},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("pattern call: invalid/missing arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{a: %int()}
			`)

			patternCallExpr := parse.FindNode(n, (*parse.PatternCallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(patternCallExpr, state, "missing argument"),
			}, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("pattern namespace's member", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{a: %myns.int}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": state.ctx.ResolveNamedPattern("int")},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("deep object pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{
					a: %{name: %str}
					b: %{
						c: %{count: %int}
						d: 1
					}
					e: 2
					f: %(#{})
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{
					"a": &ObjectPattern{
						entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
						inexact: true,
					},
					"b": &ObjectPattern{
						entries: map[string]Pattern{
							"c": &ObjectPattern{
								entries: map[string]Pattern{
									"count": state.ctx.ResolveNamedPattern("int"),
								},
								inexact: true,
							},
							"d": utils.Must(NewExactValuePattern(&Int{hasValue: true, value: 1})),
						},
						inexact: true,
					},
					"e": utils.Must(NewExactValuePattern(&Int{hasValue: true, value: 2})),
					"f": utils.Must(NewExactValuePattern(NewEmptyRecord())),
				},
				inexact: true,
			}, res)
		})

		t.Run("no other props", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{otherprops(no)}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: false,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("missing property pattern", func(t *testing.T) {
			n, state, err := _makeStateAndChunk(`
				return %{a:}
			`, nil)

			if !assert.Error(t, err) {
				return
			}

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("undefined named pattern as property pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{a: undefined}
			`)
			ident := parse.FindNode(n, (*parse.PatternIdentifierLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtPatternIsNotDeclared("undefined")),
			}, state.errors())

			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})

		t.Run("pattern with non-serializable values as property pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{a: nonserializable}
			`)
			ident := parse.FindNode(n, (*parse.PatternIdentifierLiteral)(nil), nil)

			state.ctx.AddNamedPattern("nonserializable", &TypePattern{val: ANY_LTHREAD}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, PROPERTY_PATTERNS_IN_OBJECT_AND_REC_PATTERNS_MUST_HAVE_SERIALIZABLE_VALUEs),
			}, state.errors())

			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*ObjectPattern).SymbolicValue()
			})
		})
	})

	t.Run("record pattern literal", func(t *testing.T) {

		t.Run("spread pattern that is not an record pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{...1}}
			`)

			spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, fmtPatternSpreadInRecordPatternShouldBeAnRecordPatternNot(&ExactValuePattern{value: &Int{hasValue: true, value: 1}})),
			}, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("spread record pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{...#{}}}
			`)

			//spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			// assert.Equal(t, []SymbolicEvaluationError{
			// 	makeSymbolicEvalError(spreadElem, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT),
			// }, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("spread object pattern matching all objects", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{...%p}}
			`)

			state.ctx.AddNamedPattern("p", ANY_RECORD_PATTERN, false)

			spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, CANNOT_SPREAD_REC_PATTERN_THAT_MATCHES_ANY_RECORD),
			}, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("spread valid record pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{...#{name: %str}}}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("spread properties should be unique among spread patterns", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{...#{name: %str}, ...#{name: %int}}}
			`)

			secondSpread := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(secondSpread, state, fmtPropertyShouldNotBePresentInSeveralSpreadPatterns("name")),
			}, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{
					"x": &RecordPattern{
						entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
						inexact: true,
					},
				},
				inexact: true,
			}, res)
		})

		t.Run("visible properties should have higher priority that spread properties", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{...#{name: %str}, name: %int}}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{
					"x": &RecordPattern{
						entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("int")},
						inexact: true,
					},
				},
				inexact: true,
			}, res)
		})

		t.Run("the entry patterns should match only immutable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{a: {}}}
			`)

			objectPatternLiteral := parse.FindNode(n, (*parse.ObjectPatternLiteral)(nil), func(lit *parse.ObjectPatternLiteral, _ bool, _ []parse.Node) bool {
				return len(lit.Properties) == 0
			})

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectPatternLiteral, state, fmtEntriesOfRecordPatternShouldMatchOnlyImmutableValues("a")),
			}, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("pattern call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{a: %int(0..1)}}
			`)

			intRange := NewIntRange(INT_0, INT_1, false)
			patt, _ := state.ctx.ResolveNamedPattern("int").Call(nil, []Value{intRange})

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"a": patt},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("pattern call: invalid/missing arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{a: %int()}}
			`)

			patternCallExpr := parse.FindNode(n, (*parse.PatternCallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(patternCallExpr, state, "missing argument"),
			}, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("pattern namespace's member", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{a: %myns.int}}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"a": state.ctx.ResolveNamedPattern("int")},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("deep record pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{x: #{
					a: #{name: %str}
					b: #{
						c: #{count: %int}
						d: 1
					}
					e: 2
					f: %(#{})
				}}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{
					"a": &RecordPattern{
						entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
						inexact: true,
					},
					"b": &RecordPattern{
						entries: map[string]Pattern{
							"c": &RecordPattern{
								entries: map[string]Pattern{
									"count": state.ctx.ResolveNamedPattern("int"),
								},
								inexact: true,
							},
							"d": utils.Must(NewExactValuePattern(&Int{hasValue: true, value: 1})),
						},
						inexact: true,
					},
					"e": utils.Must(NewExactValuePattern(&Int{hasValue: true, value: 2})),
					"f": utils.Must(NewExactValuePattern(NewEmptyRecord())),
				},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("missing property pattern", func(t *testing.T) {
			n, state, err := _makeStateAndChunk(`
				return %{x: #{a:}}
			`, nil)

			if !assert.Error(t, err) {
				return
			}

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res.(*ObjectPattern).entries["x"])
		})

		t.Run("undefined named pattern as property pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = #{a: undefined}
				return %p
			`)
			ident := parse.FindPatternIdentWithName(n, "undefined")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtPatternIsNotDeclared("undefined")),
			}, state.errors())

			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*RecordPattern).SymbolicValue()
			})
		})

		t.Run("pattern with non-serializable values as property pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = #{a: mutable_nonserializable}
				return %p
			`)

			ident := parse.FindPatternIdentWithName(n, "mutable_nonserializable")

			state.ctx.AddNamedPattern("mutable_nonserializable", &TypePattern{val: ANY_TEST_CASE}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, PROPERTY_PATTERNS_IN_OBJECT_AND_REC_PATTERNS_MUST_HAVE_SERIALIZABLE_VALUEs),
			}, state.errors())

			assert.Equal(t, &RecordPattern{
				entries: map[string]Pattern{"a": &TypePattern{val: ANY_SERIALIZABLE}},
				inexact: true,
			}, res)

			assert.NotPanics(t, func() {
				res.(*RecordPattern).SymbolicValue()
			})
		})
	})

	t.Run("list pattern literal", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %[ %{} ]
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ListPattern{
				elements: []Pattern{
					&ObjectPattern{
						entries: map[string]Pattern{},
						inexact: true,
					},
				},
			}, res)
		})

		t.Run("undefined general element pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %[]mytype
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.NotPanics(t, func() {
				res.(*ListPattern).SymbolicValue()
			})

			assert.Equal(t, &ListPattern{
				generalElement: &TypePattern{val: ANY_SERIALIZABLE},
			}, res)
		})

		t.Run("general element pattern matching non-serializable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %[]lthread
			`)
			state.ctx.AddNamedPattern("lthread", &TypePattern{val: ANY_LTHREAD}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.NotPanics(t, func() {
				res.(*ListPattern).SymbolicValue()
			})

			assert.Equal(t, &ListPattern{
				generalElement: &TypePattern{val: ANY_SERIALIZABLE},
			}, res)
		})

		t.Run("undefined element pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %[mytype]
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.NotPanics(t, func() {
				res.(*ListPattern).SymbolicValue()
			})

			assert.Equal(t, &ListPattern{
				elements: []Pattern{&TypePattern{val: ANY_SERIALIZABLE}},
			}, res)
		})

		t.Run("element pattern matching non-serializable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %[lthread]
			`)
			state.ctx.AddNamedPattern("lthread", &TypePattern{val: ANY_LTHREAD}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.NotPanics(t, func() {
				res.(*ListPattern).SymbolicValue()
			})

			assert.Equal(t, &ListPattern{
				elements: []Pattern{&TypePattern{val: ANY_SERIALIZABLE}},
			}, res)
		})
	})

	t.Run("tuple pattern literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
			pattern p = #[ #[] ]
			return %p
		`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &TuplePattern{
				elements: []Pattern{
					&TuplePattern{
						elements: []Pattern{},
					},
				},
			}, res)
		})

		t.Run("element patterns should match only immutable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = #[ %{} ]
				return %p
			`)

			objectPatternLit := parse.FindNode(n, (*parse.ObjectPatternLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectPatternLit, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES),
			}, state.errors())
			assert.Equal(t, &TuplePattern{
				elements: []Pattern{&TypePattern{val: ANY_SERIALIZABLE}},
			}, res)
		})

		t.Run("general element pattern should match only immutable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = #[]%{}
				return %p
			`)

			objectPatternLit := parse.FindNode(n, (*parse.ObjectPatternLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectPatternLit, state, ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES),
			}, state.errors())
			assert.Equal(t, &TuplePattern{
				generalElement: &TypePattern{val: ANY_SERIALIZABLE},
			}, res)
		})

		t.Run("general element pattern matching non-serializable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = #[]ns
				return %p
			`)

			_, ok := any(ANY_IMMUTABLE_NAMESPACE).(Serializable)
			if !assert.False(t, ok) {
				return
			}
			if !assert.False(t, ANY_IMMUTABLE_NAMESPACE.IsMutable()) {
				return
			}
			state.ctx.AddNamedPattern("ns", &TypePattern{val: ANY_IMMUTABLE_NAMESPACE}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.NotPanics(t, func() {
				res.(*TuplePattern).SymbolicValue()
			})

			assert.Equal(t, &TuplePattern{
				generalElement: &TypePattern{val: ANY_SERIALIZABLE},
			}, res)
		})

		t.Run("undefined element pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = #[mytype]
				return %p
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.NotPanics(t, func() {
				res.(*TuplePattern).SymbolicValue()
			})

			assert.Equal(t, &TuplePattern{
				elements: []Pattern{&TypePattern{val: ANY_SERIALIZABLE}},
			}, res)
		})

		t.Run("element pattern matchin non-serializable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = #[ns]
				return %p
			`)
			_, ok := any(ANY_IMMUTABLE_NAMESPACE).(Serializable)
			if !assert.False(t, ok) {
				return
			}
			if !assert.False(t, ANY_IMMUTABLE_NAMESPACE.IsMutable()) {
				return
			}
			state.ctx.AddNamedPattern("ns", &TypePattern{val: ANY_IMMUTABLE_NAMESPACE}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.NotPanics(t, func() {
				res.(*TuplePattern).SymbolicValue()
			})

			assert.Equal(t, &TuplePattern{
				elements: []Pattern{&TypePattern{val: ANY_SERIALIZABLE}},
			}, res)
		})

	})

	t.Run("union pattern", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			return %| 1 | "1"
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &UnionPattern{
			cases: []Pattern{
				utils.Must(NewExactValuePattern(INT_1)),
				NewExactStringPatternWithConcreteValue(NewString("1")),
			},
		}, res)
	})

	t.Run("union pattern", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			return %| %int | %str
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &UnionPattern{
			cases: []Pattern{
				state.ctx.ResolveNamedPattern("int"),
				state.ctx.ResolveNamedPattern("str"),
			},
		}, res)
	})

	t.Run("option pattern", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			return %--name=%str
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewOptionPattern("name", state.ctx.ResolveNamedPattern("str")), res)
	})

	t.Run("string pattern", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			return %str( "a" )
		`)
		complexStringPatternPiece := parse.FindNode(n, (*parse.ComplexStringPatternPiece)(nil), nil)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, NewSequenceStringPattern(complexStringPatternPiece, &parse.Chunk{}), res)
	})

	t.Run("pattern conversion expressions", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %(1)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, utils.Must(NewExactValuePattern(INT_1)), res)
		})

		t.Run("string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %("1")
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewExactStringPatternWithConcreteValue(NewString("1")), res)
		})

		t.Run("multivalue of serializable immutable values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %(v)
			`)
			state.setGlobal("v", NewMultivalue(INT_1, INT_2), GlobalVar)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			runTimeValue := NewRunTimeValue(AsSerializableChecked(NewMultivalue(INT_1, INT_2)))
			expectedPattern := utils.Must(NewExactValuePattern(AsSerializableChecked(runTimeValue)))
			assert.Equal(t, expectedPattern, res)
		})

		t.Run("immutable non-serializable value", func(t *testing.T) {
			if utils.Implements[Serializable](ANY_TEST_CASE) {
				assert.FailNow(t, "value in the test should not be serializable")
			}

			n, state := MakeTestStateAndChunk(`
				return %(c)
			`)

			state.setGlobal("c", ANY_TEST_CASE, GlobalVar)

			ident := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, ONLY_SERIALIZABLE_IMMUT_VALS_ALLOWED_IN_EXACT_VAL_PATTERN),
			}, state.errors())

			runTimeValue := AsSerializableChecked(NewRunTimeValue(ANY_SERIALIZABLE))
			assert.Equal(t, utils.Must(NewExactValuePattern(runTimeValue)), res)
		})

		t.Run("serializable mutable value", func(t *testing.T) {
			if utils.Implements[Serializable](ANY_TEST_CASE) {
				assert.FailNow(t, "value in the test should not be serializable")
			}

			n, state := MakeTestStateAndChunk(`
				return %({})
			`)

			objLit := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objLit, state, ONLY_SERIALIZABLE_IMMUT_VALS_ALLOWED_IN_EXACT_VAL_PATTERN),
			}, state.errors())

			runTimeValue := AsSerializableChecked(NewRunTimeValue(ANY_SERIALIZABLE))
			assert.Equal(t, utils.Must(NewExactValuePattern(runTimeValue)), res)
		})
	})

	t.Run("pattern definition", func(t *testing.T) {

		t.Run("duplicate definitions should be ignored", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = 1
				pattern p = 2

				return %p
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			//there is already a static check error
			assert.Empty(t, state.errors())
			assert.Equal(t, utils.Must(NewExactValuePattern(NewInt(1))), res)
		})

		t.Run("object pattern literal", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = %{list: %[1]}
				return %p
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{
					"list": &ListPattern{
						elements: []Pattern{
							&ExactValuePattern{value: &Int{hasValue: true, value: 1}},
						},
					},
				},
				inexact: true,
			}, res)

			//check context data

			pattern := state.ctx.ResolveNamedPattern("p")
			returnStmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)

			data, ok := state.symbolicData.GetContextData(returnStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			//ignore definition positions
			for i := range data.Patterns {
				data.Patterns[i].DefinitionPosition = parse.SourcePositionRange{}
			}

			assert.Contains(t, data.Patterns, NamedPatternData{
				Name:  "p",
				Value: pattern,
			})
		})

		t.Run("unprefixed object pattern literal", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = {list: [1]}
				return %p
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{
					"list": &ListPattern{
						elements: []Pattern{
							&ExactValuePattern{value: &Int{hasValue: true, value: 1}},
						},
					},
				},
				inexact: true,
			}, res)

			//check context data

			pattern := state.ctx.ResolveNamedPattern("p")
			returnStmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)

			data, ok := state.symbolicData.GetContextData(returnStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			//ignore definition positions
			for i := range data.Patterns {
				data.Patterns[i].DefinitionPosition = parse.SourcePositionRange{}
			}

			assert.Contains(t, data.Patterns, NamedPatternData{
				Name:  "p",
				Value: pattern,
			})
		})

		t.Run("exact value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = 1
				return %p
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, utils.Must(NewExactValuePattern(NewInt(1))), res)
		})

		t.Run("exact value: multivalue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = $val
				return %p
			`)
			state.setGlobal("val", NewMultivalue(NewInt(1), NewInt(2)), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			expectedValue := AsSerializableChecked(NewRunTimeValue(AsSerializableChecked(NewMultivalue(NewInt(1), NewInt(2)))))
			expectedPattern := utils.Must(NewExactValuePattern(expectedValue))
			assert.Equal(t, expectedPattern, res)
		})

		t.Run("exact value: not serializable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				lthread = go do {}
				pattern p = $lthread
				return %p
			`)

			variable := parse.FindNode(n, (*parse.Variable)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(variable, state, ONLY_SERIALIZABLE_IMMUT_VALS_ALLOWED_IN_EXACT_VAL_PATTERN),
			}, state.errors())

			expectedValue := AsSerializableChecked(NewRunTimeValue(ANY_SERIALIZABLE))
			expectedPattern := utils.Must(NewExactValuePattern(expectedValue))
			assert.Equal(t, expectedPattern, res)
		})

		t.Run("exact string value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = "a"
				return %p
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewExactStringPatternWithConcreteValue(NewString("a")), res)
		})

		t.Run("in preinit block", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				preinit {
					pattern p = %{}
				}
				return %p
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{},
				inexact: true,
			}, res)

			//check context data

			pattern := state.ctx.ResolveNamedPattern("p")
			returnStmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)

			data, ok := state.symbolicData.GetContextData(returnStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			//ignore definition positions
			for i := range data.Patterns {
				data.Patterns[i].DefinitionPosition = parse.SourcePositionRange{}
			}

			assert.Contains(t, data.Patterns, NamedPatternData{
				Name:  "p",
				Value: pattern,
			})
		})
	})

	t.Run("pattern namespace definition", func(t *testing.T) {
		t.Run("RHS is an object literal", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pnamespace namespace. = {patt: %str}
				return %namespace.
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &PatternNamespace{
				entries: map[string]Pattern{
					"patt": state.ctx.ResolveNamedPattern("str"),
				},
			}, res)

			//check context data

			namespace := state.ctx.ResolvePatternNamespace("namespace")
			returnStmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)

			data, ok := state.symbolicData.GetContextData(returnStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			//ignore definition positions
			for i := range data.PatternNamespaces {
				data.PatternNamespaces[i].DefinitionPosition = parse.SourcePositionRange{}
			}

			assert.Contains(t, data.PatternNamespaces, PatternNamespaceData{
				Name:  "namespace",
				Value: namespace,
			})
		})

		t.Run("RHS is an object literal with an exact value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pnamespace namespace. = {patt: #a}
				return %namespace.
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &PatternNamespace{
				entries: map[string]Pattern{
					"patt": utils.Must(NewExactValuePattern(&Identifier{name: "a"})),
				},
			}, res)

			//check context data

			namespace := state.ctx.ResolvePatternNamespace("namespace")
			returnStmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)

			data, ok := state.symbolicData.GetContextData(returnStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			//ignore definition positions
			for i := range data.PatternNamespaces {
				data.PatternNamespaces[i].DefinitionPosition = parse.SourcePositionRange{}
			}

			assert.Contains(t, data.PatternNamespaces, PatternNamespaceData{
				Name:  "namespace",
				Value: namespace,
			})
		})

		t.Run("RHS is invalid", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pnamespace namespace. = int
				return %namespace.
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0], state, fmtPatternNamespaceShouldBeInitWithNot(ANY_INT)),
			}, state.errors())
			assert.Equal(t, &PatternNamespace{
				entries: nil,
			}, res)

			//check context data

			namespace := state.ctx.ResolvePatternNamespace("namespace")
			returnStmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)

			data, ok := state.symbolicData.GetContextData(returnStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			//ignore definition positions
			for i := range data.PatternNamespaces {
				data.PatternNamespaces[i].DefinitionPosition = parse.SourcePositionRange{}
			}

			assert.Contains(t, data.PatternNamespaces, PatternNamespaceData{
				Name:  "namespace",
				Value: namespace,
			})
		})

		t.Run("duplicate definitions should be ignored", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pnamespace namespace. = {}
				pnamespace namespace. = {a: %int}

				return %namespace.
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			//there is already a static check error
			assert.Empty(t, state.errors())
			assert.Equal(t, &PatternNamespace{
				entries: nil,
			}, res)

			namespace := state.ctx.ResolvePatternNamespace("namespace")
			assert.Same(t, namespace, res)
		})

	})

	t.Run("pattern identifier", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`%int`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, state.ctx.ResolveNamedPattern("int"), res)
		})

		t.Run("non existing", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`%nonexisting`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0], state, fmtPatternIsNotDeclared("nonexisting")),
			}, state.errors())
			assert.Equal(t, ANY_PATTERN, res)
		})

		t.Run("non existing: name close to an existing patern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`%in`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0], state, fmtPatternIsNotDeclaredYouProbablyMeant("in", "int")),
			}, state.errors())
			assert.Equal(t, ANY_PATTERN, res)
		})
	})

	t.Run("pattern namespace's member", func(t *testing.T) {
		t.Run("undeclared namespace", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %nonexisting.int
			`)

			patternIdent := parse.FindNode(n, (*parse.PatternNamespaceIdentifierLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(patternIdent, state, fmtPatternNamespaceIsNotDeclared("nonexisting")),
			}, state.errors())
			assert.Equal(t, ANY_PATTERN, res)
		})

		t.Run("non existing member", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %myns.nonexisting
			`)

			memberIdent := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberIdent, state, fmtPatternNamespaceHasNotMember("myns", "nonexisting")),
			}, state.errors())
			assert.Equal(t, ANY_PATTERN, res)
		})
	})

	t.Run("exact value pattern", func(t *testing.T) {
		t.Run("if the value is concrete then only the same value should be matched", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = 1

				var a p = 1
				var b p = 2 

				fn f(arg int){
					var c p = arg
				}

				fn g(arg){
					var d p = arg
				}
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			bDecl := parse.FindLocalVarDeclWithName(n, "b")
			cDecl := parse.FindLocalVarDeclWithName(n, "c")
			dDecl := parse.FindLocalVarDeclWithName(n, "d")

			pattern := utils.Must(NewExactValuePattern(INT_1))

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(bDecl.Right, state, fmtNotAssignableToVarOftype(INT_2, pattern)),
				makeSymbolicEvalError(cDecl.Right, state, fmtNotAssignableToVarOftype(ANY_INT, pattern)),
				makeSymbolicEvalError(dDecl.Right, state, fmtNotAssignableToVarOftype(ANY, pattern)),
			}, state.errors())
		})

		t.Run("if the value is known at run time then no value should be matched: any int", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = $an_int

				var a p = 1

				fn f(arg int){
					var b p = arg
				}

				fn g(arg){
					var c p = arg
				}
			`)

			state.setGlobal("an_int", ANY_INT, GlobalVar)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			aDecl := parse.FindLocalVarDeclWithName(n, "a")
			bDecl := parse.FindLocalVarDeclWithName(n, "b")
			cDecl := parse.FindLocalVarDeclWithName(n, "c")

			pattern := utils.Must(NewExactValuePattern(AsSerializableChecked(NewRunTimeValue(ANY_INT))))

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(aDecl.Right, state, fmtNotAssignableToVarOftype(INT_1, pattern)),
				makeSymbolicEvalError(bDecl.Right, state, fmtNotAssignableToVarOftype(ANY_INT, pattern)),
				makeSymbolicEvalError(cDecl.Right, state, fmtNotAssignableToVarOftype(ANY, pattern)),
			}, state.errors())
		})

		t.Run("if the value is known at run time then no value should be matched: multivalue with concrete values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = $one_or_two

				var a p = 1

				fn f(arg int){
					var b p = arg
				}

				fn g(arg){
					var c p = arg
				}
			`)

			varValue := NewMultivalue(INT_1, INT_2)
			state.setGlobal("one_or_two", varValue, GlobalVar)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			aDecl := parse.FindLocalVarDeclWithName(n, "a")
			bDecl := parse.FindLocalVarDeclWithName(n, "b")
			cDecl := parse.FindLocalVarDeclWithName(n, "c")

			pattern := utils.Must(NewExactValuePattern(AsSerializableChecked(NewRunTimeValue(AsSerializableChecked(varValue)))))

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(aDecl.Right, state, fmtNotAssignableToVarOftype(INT_1, pattern)),
				makeSymbolicEvalError(bDecl.Right, state, fmtNotAssignableToVarOftype(ANY_INT, pattern)),
				makeSymbolicEvalError(cDecl.Right, state, fmtNotAssignableToVarOftype(ANY, pattern)),
			}, state.errors())
		})
	})

	t.Run("exact string value pattern", func(t *testing.T) {
		t.Run("if the value is concrete only the same value should be matched", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = "a"

				var a p = "a"
				var b p = "b"

				fn f(arg string){
					var c p = arg
				}

				fn g(arg){
					var d p = arg
				}
			`)

			state.ctx.AddNamedPattern("string", &TypePattern{val: ANY_STRING}, false)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			bDecl := parse.FindLocalVarDeclWithName(n, "b")
			cDecl := parse.FindLocalVarDeclWithName(n, "c")
			dDecl := parse.FindLocalVarDeclWithName(n, "d")

			pattern := NewExactStringPatternWithConcreteValue(NewString("a"))

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(bDecl.Right, state, fmtNotAssignableToVarOftype(NewString("b"), pattern)),
				makeSymbolicEvalError(cDecl.Right, state, fmtNotAssignableToVarOftype(ANY_STRING, pattern)),
				makeSymbolicEvalError(dDecl.Right, state, fmtNotAssignableToVarOftype(ANY, pattern)),
			}, state.errors())
		})

		t.Run("if the value is known at run time then no value should be matched: str like", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = $s

				var a p = "a"

				fn f(arg str){
					var b p = arg
				}

				fn g(arg){
					var c p = arg
				}
			`)

			state.setGlobal("s", ANY_STR_LIKE, GlobalVar)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			aDecl := parse.FindLocalVarDeclWithName(n, "a")
			bDecl := parse.FindLocalVarDeclWithName(n, "b")
			cDecl := parse.FindLocalVarDeclWithName(n, "c")

			pattern := NewExactStringPatternWithRunTimeValue(NewRunTimeValue(ANY_STR_LIKE).as(STRLIKE_INTERFACE_TYPE).(*strLikeRunTimeValue))

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(aDecl.Right, state, fmtNotAssignableToVarOftype(NewString("a"), pattern)),
				makeSymbolicEvalError(bDecl.Right, state, fmtNotAssignableToVarOftype(ANY_STR_LIKE, pattern)),
				makeSymbolicEvalError(cDecl.Right, state, fmtNotAssignableToVarOftype(ANY, pattern)),
			}, state.errors())
		})

		t.Run("if the value is known at run time then no value should be matched: string matching a pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = $s

				var a p = "a"

				fn f(arg str){
					var b p = arg
				}

				fn g(arg){
					var c p = arg
				}
			`)

			variableValue := NewStringMatchingPattern(NewRegexPattern("a+"))
			state.setGlobal("s", variableValue, GlobalVar)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			aDecl := parse.FindLocalVarDeclWithName(n, "a")
			bDecl := parse.FindLocalVarDeclWithName(n, "b")
			cDecl := parse.FindLocalVarDeclWithName(n, "c")

			pattern := NewExactStringPatternWithRunTimeValue(NewRunTimeValue(variableValue).as(STRLIKE_INTERFACE_TYPE).(*strLikeRunTimeValue))

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(aDecl.Right, state, fmtNotAssignableToVarOftype(NewString("a"), pattern)),
				makeSymbolicEvalError(bDecl.Right, state, fmtNotAssignableToVarOftype(ANY_STR_LIKE, pattern)),
				makeSymbolicEvalError(cDecl.Right, state, fmtNotAssignableToVarOftype(ANY, pattern)),
			}, state.errors())
		})
	})

	t.Run("optional pattern", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`%int?`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &OptionalPattern{
				pattern: state.ctx.ResolveNamedPattern("int"),
			}, res)
		})

		t.Run("pattern already matches nil", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = nil
				return %p?
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[1].(*parse.ReturnStatement).Expr, state, CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL),
			}, state.errors())
			assert.Equal(t, ANY_PATTERN, res)
		})
	})

	t.Run("assertion statement", func(t *testing.T) {
		t.Run("value is a boolean", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assert (true or false)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("value is not a boolean", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assert (int + int)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0], state, fmtAssertedValueShouldBeBoolNot(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("binary match expression narrows the type of a variable (%int)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assert (a match %int)
				return (int + a)
			`)

			state.setGlobal("a", ANY, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("binary match expression narrows the type of a variable: (object pattern literal)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assert (a match %{a: 1, b: [3]})
			`)

			state.setGlobal("a", ANY, GlobalConst)

			res, err := symbolicEval(n, state)

			varInfo, _ := state.get("a")
			expectedObject := &Object{
				entries: map[string]Serializable{
					"a": &Int{hasValue: true, value: 1},
					"b": NewList(&Int{hasValue: true, value: 3}),
				},
				static: map[string]Pattern{
					"a": &ExactValuePattern{value: &Int{hasValue: true, value: 1}},
					"b": NewListPattern([]Pattern{utils.Must(NewExactValuePattern(&Int{hasValue: true, value: 3}))}),
				},
			}
			assert.Equal(t, expectedObject, varInfo.value)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("binary match expression narrows the type of a variable: (list pattern literal)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				assert (a match %[]%object)
			`)

			state.setGlobal("a", ANY, GlobalConst)

			res, err := symbolicEval(n, state)

			varInfo, _ := state.get("a")
			expectedObject := &List{
				generalElement: ANY_OBJ,
			}
			assert.Equal(t, varInfo.value, expectedObject)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Nil(t, res)
		})

		t.Run("binary match expression narrows the type of a property (%int)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				if (a.prop match %int){
					var b %int = a.prop
				}
			`)

			object := NewInexactObject(map[string]Serializable{"prop": ANY_SERIALIZABLE}, nil, nil)
			state.setGlobal("a", object, GlobalConst)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})
	})

	t.Run("runtime typecheck expression", func(t *testing.T) {
		t.Run("argument of Go function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`f ~arg`)

			goFunc := &GoFunction{
				fn: func(*Context, *Int) {},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			state.setGlobal("arg", ANY, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, Nil, res)
		})

		t.Run("argument of Inox function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(n %int){
					return n
				}

				return f(~arg)
			`)

			state.setGlobal("arg", ANY, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})
	})

	t.Run("testsuite expression", func(t *testing.T) {
		t.Run("empty module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite "name" {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("tests suite should inherit patterns defined by the parent state", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = 1
				return testsuite "name" {
					val = %p
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("meta value should either be a string or a record: string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite "my test case" {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("meta value should either be a string or a record: record", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite({}) {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("meta value should either be a string or a record: invalid value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite 0 {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, META_VAL_OF_TEST_SUITE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD),
			}, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("name value in meta should be a string: string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite({name: "test"}) {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("name value in meta should be a string: integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite({name: 1}) {}`)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("program value in meta should be an absolute non-dir path: relative non-dir path", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite({program: ./mod.ix}) {}`)
			pathLit := parse.FindNode(n, (*parse.RelativePathLiteral)(nil), nil)

			fls := memfs.New()
			util.WriteFile(fls, "/mod.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(pathLit, state, fmtNotAssignableToPropOfType(NewPath("./mod.ix"), ANY_ABS_NON_DIR_PATH)),
			}, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("program value in meta should not be present if we are not in a project", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite({program: /mod.ix}) {}`)
			objectLit := parse.FindNode(n.Statements[0], (*parse.ObjectLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectLit, state, PROGRAM_TESTING_ONLY_SUPPORTED_IN_PROJECTS),
			}, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

		t.Run("program value in meta should not be present if we are not in a project: testsuite in imported module", func(t *testing.T) {
			n, state := MakeTestStateAndImportedModules(`
				manifest {}; 
				import res /lib.ix {}
			`, map[string]string{
				"/lib.ix": `
					manifest {}
					testsuite({program: /program.ix}) {}
				`,
			})

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			if !assert.Len(t, state.errors(), 1) {
				return
			}
			assert.ErrorContains(t, state.errors()[0], PROGRAM_TESTING_ONLY_SUPPORTED_IN_PROJECTS)
		})

		t.Run("program value is allowed in meta if we are in a project", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}; 
				testsuite({program: /program.ix}) {}
			`)

			fls := memfs.New()
			util.WriteFile(fls, "/program.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("program value is allowed in meta if we are in a project: testsuite in imported module", func(t *testing.T) {
			n, state := MakeTestStateAndImportedModules(`
				manifest {}; 
				import res /lib.ix {}
			`, map[string]string{
				"/lib.ix": `
					manifest {}
					testsuite({program: /program.ix}) {}
				`,
			})

			fls := memfs.New()
			util.WriteFile(fls, "/program.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("main-db-schema property in meta should not be present if the program property is not present", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}; 
				testsuite({main-db-schema: %{}}) {}
			`)

			objectLit := parse.FindNode(n.Statements[0], (*parse.ObjectLiteral)(nil), nil)

			fls := memfs.New()
			state.projectFilesystem = fls

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectLit, state, MAIN_DB_SCHEMA_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM),
			}, state.errors())
		})

		t.Run("main-db-migrations property in meta should not be present if the program property is not present", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}; 
				testsuite({main-db-migrations: {}}) {}
			`)

			objectLit := parse.FindNode(n.Statements[0], (*parse.ObjectLiteral)(nil), func(n *parse.ObjectLiteral, isUnique bool, _ []parse.Node) bool {
				return len(n.Properties) == 1
			})

			fls := memfs.New()
			state.projectFilesystem = fls

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectLit, state, MAIN_DB_MIGRATIONS_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM),
			}, state.errors())
		})

		t.Run("main-db-migrations property in meta should not be present if the program property is not present", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}; 
				testsuite({main-db-migrations: {}}) {}
			`)

			objectLit := parse.FindNode(n.Statements[0], (*parse.ObjectLiteral)(nil), func(n *parse.ObjectLiteral, isUnique bool, _ []parse.Node) bool {
				return len(n.Properties) == 1
			})

			fls := memfs.New()
			state.projectFilesystem = fls

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectLit, state, MAIN_DB_MIGRATIONS_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM),
			}, state.errors())
		})

		t.Run("error in module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testsuite "name" {
				(1 + true)
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "true")),
			}, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})
	})

	t.Run("testcase expression", func(t *testing.T) {
		t.Run("empty module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase "name" {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("meta value should either be a string or a record: string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase "my test case" {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("meta value should either be a string or a record: record", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase({}) {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("meta value should either be a string or a record: invalid value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase 0 {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, META_VAL_OF_TEST_CASE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD),
			}, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("name value in meta should be a string: string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase({name: "my test"}) {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("name value in meta should be a string: integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase({name: 1}) {}`)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, fmtNotAssignableToPropOfType(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("pass-fs-copy value in meta should be a boolean: boolean", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase({pass-live-fs-copy-to-subtests: true}) {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("pass-fs-copy value in meta should be a string: integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase({pass-live-fs-copy-to-subtests: 1}) {}`)
			intLit := parse.FindNode(n, (*parse.IntLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intLit, state, fmtNotAssignableToPropOfType(NewInt(1), ANY_BOOL)),
			}, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("program value in meta should be an absolute non-dir path: relative non-dir path", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase({program: ./mod.ix}) {}`)
			pathLit := parse.FindNode(n, (*parse.RelativePathLiteral)(nil), nil)

			fls := memfs.New()
			util.WriteFile(fls, "/mod.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(pathLit, state, fmtNotAssignableToPropOfType(NewPath("./mod.ix"), ANY_ABS_NON_DIR_PATH)),
			}, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("program value in meta should not be present if we are not in a project", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase({program: /mod.ix}) {}`)
			objectLit := parse.FindNode(n.Statements[0], (*parse.ObjectLiteral)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objectLit, state, PROGRAM_TESTING_ONLY_SUPPORTED_IN_PROJECTS),
			}, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("error in module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase "name" {
				(1 + true)
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "true")),
			}, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("a __test global with a program property should be defined within the testcase", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				testcase {
					$__test.program
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
		})

		t.Run("if the main-db-schema and main-db-migrations properties are present __test.program.dbs.main should be defined", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				testcase({
					program: /program.ix
					main-db-schema: %{
						user: {name: str}
					}
					main-db-migrations: {
						inclusions: :{
							%/user: {name: "foo"}
						}
					}
				}){
					check_databases(__test.program.dbs)
				}
			`)

			fls := memfs.New()
			util.WriteFile(fls, "/program.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			called := false
			state.setGlobal("check_databases", WrapGoFunction(func(_ *Context, ns *Namespace) {
				called = true
				if !assert.Equal(t, []string{"main"}, ns.PropertyNames()) {
					return
				}
				value := ns.Prop("main")
				if !assert.IsType(t, (*DatabaseIL)(nil), value) {
					return
				}

				db := value.(*DatabaseIL)
				assert.True(t, HasRequiredProperty(db, "user"))
			}), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_CASE, res)
			assert.True(t, called)
		})

		t.Run("if parent suite has database parameters and the testcase has no meta value, __test.program.dbs.main should be defined", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				testsuite({
					program: /program.ix
					main-db-schema: %{
						user: {name: str}
					}
					main-db-migrations: {
						inclusions: :{
							%/user: {name: "foo"}
						}
					}
				}){
					testcase {
						check_databases(__test.program.dbs)
					}
				}
			`)

			fls := memfs.New()
			util.WriteFile(fls, "/program.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			called := false
			state.setGlobal("check_databases", WrapGoFunction(func(_ *Context, ns *Namespace) {
				called = true
				if !assert.Equal(t, []string{"main"}, ns.PropertyNames()) {
					return
				}
				value := ns.Prop("main")
				if !assert.IsType(t, (*DatabaseIL)(nil), value) {
					return
				}

				db := value.(*DatabaseIL)
				assert.True(t, HasRequiredProperty(db, "user"))
			}), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
			assert.True(t, called)
		})

		t.Run("if parent suite has database parameters and the testcase has a string meta value, __test.program.dbs.main should be defined", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				testsuite({
					program: /program.ix
					main-db-schema: %{
						user: {name: str}
					}
					main-db-migrations: {
						inclusions: :{
							%/user: {name: "foo"}
						}
					}
				}){
					testcase "test" {
						check_databases(__test.program.dbs)
					}
				}
			`)

			fls := memfs.New()
			util.WriteFile(fls, "/program.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			called := false
			state.setGlobal("check_databases", WrapGoFunction(func(_ *Context, ns *Namespace) {
				called = true
				if !assert.Equal(t, []string{"main"}, ns.PropertyNames()) {
					return
				}
				value := ns.Prop("main")
				if !assert.IsType(t, (*DatabaseIL)(nil), value) {
					return
				}

				db := value.(*DatabaseIL)
				assert.True(t, HasRequiredProperty(db, "user"))
			}), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
			assert.True(t, called)
		})

		t.Run("if parent suite has database parameters and the testcase has an empty record as meta value, __test.program.dbs.main should be defined", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				testsuite({
					program: /program.ix
					main-db-schema: %{
						user: {name: str}
					}
					main-db-migrations: {
						inclusions: :{
							%/user: {name: "foo"}
						}
					}
				}){
					testcase({}){
						check_databases(__test.program.dbs)
					}
				}
			`)

			fls := memfs.New()
			util.WriteFile(fls, "/program.ix", []byte("manifest {}"), 0600)
			state.projectFilesystem = fls

			called := false
			state.setGlobal("check_databases", WrapGoFunction(func(_ *Context, ns *Namespace) {
				called = true
				if !assert.Equal(t, []string{"main"}, ns.PropertyNames()) {
					return
				}
				value := ns.Prop("main")
				if !assert.IsType(t, (*DatabaseIL)(nil), value) {
					return
				}

				db := value.(*DatabaseIL)
				assert.True(t, HasRequiredProperty(db, "user"))
			}), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
			assert.True(t, called)
		})

		t.Run("testcase should inherit patterns defined by the parent test suite", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return testsuite {
					pattern p = 1
					testcase {
						var val p = 1
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TEST_SUITE, res)
		})

	})

	t.Run("lifetimejob expression", func(t *testing.T) {
		t.Run("should have access to implicit subject properties defined before and after the jobs", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{ 
				a: int
				lifetimejob "name" { [self.a, self.b] } 
				b: 2
			}`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("accessing a non existing property of the subject should cause an error", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{ 
				lifetimejob "name" { self.a } 
			}`)

			_, err := symbolicEval(n, state)

			membExpr := parse.FindNode(n, &parse.MemberExpression{}, nil)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(membExpr, state, fmtPropOfDoesNotExist("a", NewEmptyObject(), "")),
			}, state.errors())
		})

		t.Run("implicit subject: error in module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{ 
				a: int
				lifetimejob "name" { (int + true) } 
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "true")),
			}, state.errors())
		})

		t.Run("explicit subject", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`lifetimejob "name" for %list {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &LifetimeJob{subjectPattern: state.ctx.ResolveNamedPattern("list")}, res)
		})

		t.Run("explicit subject: error in module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`lifetimejob "name" for %list {
				(int + true)
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "true")),
			}, state.errors())
			assert.Equal(t, &LifetimeJob{subjectPattern: state.ctx.ResolveNamedPattern("list")}, res)
		})

		t.Run("explicit subject: not matched by self", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				{
					lifetimejob "name" for %{a: %int} {}
				}
			`)

			lifetimeJobExpr := parse.FindNode(n, &parse.LifetimejobExpression{}, nil)
			subjectPattern := NewExactObjectPattern(map[string]Pattern{
				"a": state.ctx.ResolveNamedPattern("int"),
			}, nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(lifetimeJobExpr, state, fmtSelfShouldMatchLifetimeJobSubjectPattern(subjectPattern)),
			}, state.errors())
		})

		t.Run("lifetime job within an object literal should have access to patterns defined in parent state", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = int
				{ 
					a: int
					lifetimejob "name" { [%p, %int]  } 
				}`,
			)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("lifetime job within a function should have access to patterns defined in top level state", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern p = int
				fn createJob(){
					return lifetimejob "name" for %object { [%p, %int]  } 
				}
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("meta value should be immutable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				lifetimejob {} for %object {  } 
			`)
			objLit := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objLit, state, META_VAL_OF_LIFETIMEJOB_SHOULD_BE_IMMUTABLE),
			}, state.errors())
		})

		//TODO: add tests on globals

	})

	t.Run("spawn expression", func(t *testing.T) {
		t.Run("call expression: user defined function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){ }
				return go {globals: .{}} do f()
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("call expression: undefined function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return go {globals: .{}} do f()
			`)
			ident := parse.FindIdentWithName(n, "f")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("f")),
				makeSymbolicEvalError(ident, state, fmtCannotCall(ANY)),
			}, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("call expression: identifier member expr: namespace method", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return go {globals: .{}} do http.read(https://example.com/)
			`)

			namespace := NewNamespace(map[string]Value{
				"read": WrapGoFunction(func(*Context, *URL) *String {
					return ANY_STRING
				}),
			})
			state.setGlobal("http", namespace, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("call expression: identifier member expr: object is not a namespace", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				obj = {
					read: fn(){
						return 1
					}
				}
				return go {globals: .{}} do obj.read()
			`)

			objIdent := parse.FindIdentWithName(n.Statements[1], "obj")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.IsType(t, ANY_LTHREAD, res)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objIdent, state, INVALID_SPAWN_EXPR_WITH_SHORTHAND_SYNTAX_CALLEE_SHOULD_BE_AN_FN_IDENTIFIER_OR_A_NAMESPACE_METHOD),
			}, state.errors())
		})

		t.Run("call expression: global constants should be usable as arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg){ return arg }
				return go {globals: .{}} do f(myconst)
			`)
			state.setGlobal("myconst", INT_1, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("call expression: global variables should be usable as arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(arg){ return arg }
				return go {globals: .{}} do f(myglobal)
			`)
			ident := parse.FindIdentWithName(n.Statements[1], "myglobal")

			state.setGlobal("myglobal", INT_1, GlobalVar)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("myglobal")),
			}, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("call expression: local variables should not be usable as arguments", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = 1
				fn f(arg){ return arg }
				return go {globals: .{}} do f(v)
			`)

			ident := parse.FindIdentWithName(n.Statements[2], "v")

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ident, state, fmtVarIsNotDeclared("v")),
			}, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("call expression without meta", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){ }
				return go do f()
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("provided group is not a lthread group", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				group = int
				return go {group: group, globals: .{}} do { }
			`)

			res, err := symbolicEval(n, state)
			obj := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(obj, state, fmtGroupPropertyNotLThreadGroup(ANY_INT)),
			}, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("error in embedded module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return go {globals: .{}} do { return (int + "a") }
			`)

			binExpr := parse.FindNode(n, (*parse.BinaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "\"a\"")),
			}, state.errors())
			assert.IsType(t, ANY_LTHREAD, res)
		})

		t.Run("call provided function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return 2
				}
				lthread = go {globals: {f: f}} do {
					return f() # f is external for the embedded module
				}
				return lthread.wait_result!()
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, ANY, res)
		})

		t.Run("unknown section in metadata", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return go {x: int} do { return int }
			`)

			metadataNode := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Contains(t,
				state.warnings(),
				makeSymbolicEvalWarning(metadataNode, state, fmtUnknownSectionInLThreadMetadata("x")),
			) //we use contains because there is also a warning about a missing permission
		})

		t.Run("allow section", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				lthread = go {allow: {}} do {
					return int
				}
				return lthread.wait_result!()
			`)

			metadataNode := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			assert.NotContains(t,
				state.warnings(),
				makeSymbolicEvalWarning(metadataNode, state, fmtUnknownSectionInLThreadMetadata(LTHREAD_META_ALLOW_SECTION)),
			) //we use contains because there is also a warning about a missing permission
		})

		t.Run("if a variable is a passed as a global, the static type should be preserved", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				list = [1, 2, 3]

				lthread = go {
					globals: {list: list}
				} do {
					list[0] = 0
					list.append(1)
				}

				return list
			`)

			res, err := symbolicEval(n, state)

			//make sure the value of `list` is not equal to its static type.
			if !assert.Equal(t, NewList(INT_1, INT_2, INT_3), res) {
				return
			}

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("no error should be reported if the meta object has an element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				lthread = go {globals: {a: 1}, 2} do {
					b = a
				}
			`)

			_, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})

		t.Run("no error should be reported if the 'globals' object has an element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				lthread = go {
					globals: {a: 1, 2}
				} do {
					b = a
				}
			`)

			_, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
		})
	})

	t.Run("reception handler expression", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			{
				on received %{} {}
			}
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		static := map[string]Pattern{"": NewListPatternOf(getStatic(ANY_SYNC_MSG_HANDLER))}
		expectedObject := NewInexactObject(map[string]Serializable{
			"": NewList(ANY_SYNC_MSG_HANDLER),
		}, nil, static)
		assert.Equal(t, expectedObject, res)

	})

	t.Run("sendvalue expression", func(t *testing.T) {

	})

	t.Run("mapping expression", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { 0 => 1  1 => comp 0 }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_MAPPING, res)
		})

		t.Run("key variable & group matching variable should be accessible in right side", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { p %/{:name} m => [p, m] }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_MAPPING, res)
		})

		t.Run("key variable should be accessible in right side and have right type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { n int => (n - int) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_MAPPING, res)
		})

		t.Run("key variable should be accessible in right side and have right type: case pattern key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { n %int => (n - int) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_MAPPING, res)
		})

	})
	t.Run("treedata literal", func(t *testing.T) {

		t.Run("no children", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata "root" {}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("single child", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata "root" {"child"}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		//TODO: properly check errors

		t.Run("mutable value as root", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata ({}) {}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("immutable non-serializable value as root", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata (testsuite {}) {}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("mutable value as child", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata "root" { ({}) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("immutable non-serializable value as child", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata "root" { (testsuite {}) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("treedata pair with a mutable key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata "root" { {}: 1 }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("treedata pair with an immutable non-serializable key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				suite = testsuite {}
				return treedata "root" { suite: 1 }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("treedata pair with a mutable value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				treedata "root" { 1: {} }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})

		t.Run("treedata pair with an immutable non-serializable value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				suite = testsuite {}
				return treedata "root" { 1: suite }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_TREEDATA, res)
		})
	})

	t.Run("compute expression", func(t *testing.T) {

		t.Run("argument is not a simple value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { 0 => comp {} }
			`)

			computeExpr := parse.FindNode(n, (*parse.ComputeExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(computeExpr.Arg, state, INVALID_KEY_IN_COMPUTE_EXPRESSION_ONLY_SIMPLE_VALUE_ARE_SUPPORTED),
			}, state.errors())

			assert.Equal(t, ANY_MAPPING, res)
		})
	})
	t.Run("concatenation expression", func(t *testing.T) {
		t.Run("single string element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat "a"`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("two string-like elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat "a" "b"`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_STR_LIKE, res)
		})

		t.Run("first element is a multivalue implementing string like", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var elem %str = "a"
				if g {
					elem = concat elem "b"
				}
				return [elem, concat elem "x"]
			`)

			state.setGlobal("g", ANY_BOOL, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(
				//we also check that elem has the right type because the test case depends on that
				ANY_STR_LIKE,
				ANY_STR_LIKE,
			), res)
		})

		t.Run("second element is a multivalue implementing string like", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var elem %str = "a"
				if g {
					elem = concat elem "b"
				}
				return [elem, concat "x" elem]
			`)

			state.setGlobal("g", ANY_BOOL, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(
				//we also check that elem has the right because the test case depends on that
				ANY_STR_LIKE,
				ANY_STR_LIKE,
			), res)
		})

		t.Run("single byteslice element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat 0d[12]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BYTE_SLICE, res)
		})

		t.Run("two bytes-like elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat 0d[12] 0d[34]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BYTES_LIKE, res)
		})

		t.Run("two tuples with known elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat #[int] #["a"]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewTuple(ANY_INT, NewString("a")), res)
		})

		t.Run("two tuples with unknown elements, different general elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(a %int_tuple, b %str_tuple){
					return concat a b
				}`,
			)
			state.ctx.AddNamedPattern("int_tuple", &TypePattern{val: NewTupleOf(ANY_INT)}, false)
			state.ctx.AddNamedPattern("str_tuple", &TypePattern{val: NewTupleOf(ANY_STRING)}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			expectedFn := &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{NewTupleOf(ANY_INT), NewTupleOf(ANY_STRING)},
				parameterNames: []string{"a", "b"},
				result:         NewTupleOf(AsSerializableChecked(NewMultivalue(ANY_INT, ANY_STRING))),
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("two tuples with unknown elements, same general element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(a %int_tuple, b %int_tuple){
					return concat a b
				}`,
			)
			state.ctx.AddNamedPattern("int_tuple", &TypePattern{val: NewTupleOf(ANY_INT)}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			expectedFn := &InoxFunction{
				node:           fnExpr,
				nodeChunk:      n,
				parameters:     []Value{NewTupleOf(ANY_INT), NewTupleOf(ANY_INT)},
				parameterNames: []string{"a", "b"},
				result:         NewTupleOf(ANY_INT),
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("spread string list", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat ...["a"]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_STR_LIKE, res)
		})

		t.Run("spread list with invalid values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat ...[int]`)
			res, err := symbolicEval(n, state)

			spreadElem := parse.FindNode(n, (*parse.ElementSpreadElement)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("string followed by a spread list with invalid values", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat "a" ...[int]`)
			res, err := symbolicEval(n, state)

			spreadElem := parse.FindNode(n, (*parse.ElementSpreadElement)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION),
			}, state.errors())
			assert.Equal(t, ANY_STR_LIKE, res)
		})

		t.Run("non iterable spread element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`return concat ...int`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("string followed by a non iterable spread element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`return concat "a" ...int`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.errors())
			assert.Equal(t, ANY_STR_LIKE, res)
		})
	})

	t.Run("string template literal", func(t *testing.T) {

		replace := func(s string) string {
			return strings.ReplaceAll(s, "|", "`")
		}

		t.Run("no interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				pattern digit = str('0'..'9')
				return %digit|3|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_CHECKED_STRING, res)
		})

		t.Run("no pattern, no interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				return |3|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("3"), res)
		})

		t.Run("interpolation & non-namespaced pattern", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				pattern sql = str( %|.*| )
				unsanitized_id = "5"
				return %sql|SELECT * FROM users WHERE id = ${int:$unsanitized_id}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit, state, STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX),
			}, state.errors())
			assert.Equal(t, ANY_CHECKED_STRING, res)
		})

		t.Run("interpolation pattern does not exist", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				pnamespace sql. = {stmt: %str( %|.*| )}
				unsanitized_id = "5"
				return %sql.stmt|SELECT * FROM users WHERE id = ${int:$unsanitized_id}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr.(*parse.StringTemplateLiteral)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit.Slices[1], state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist("int", "sql")),
			}, state.errors())
			assert.Equal(t, ANY_CHECKED_STRING, res)
		})

		t.Run("interpolation expression is not a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				pnamespace sql. = {
					stmt: %str( %|.*| ),
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = {}
				return %sql.stmt|SELECT * FROM users WHERE id = ${int:$unsanitized_id}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr.(*parse.StringTemplateLiteral)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit.Slices[1], state, fmtInterpolationIsNotStringlikeOrIntBut(&Object{entries: map[string]Serializable{}})),
			}, state.errors())
			assert.Equal(t, ANY_CHECKED_STRING, res)
		})

		t.Run("no pattern, leading interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				s = "1"
				return |${s}2|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_STRING, res)
		})

		t.Run("no pattern, trailing interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				s = "2"
				return |int${s}|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_STRING, res)
		})

		t.Run("no pattern, interpolation value implementing string like", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var elem %str = "a"
				if g {
					elem = concat elem "b"
				}
				return [elem,` + "`x${elem}`" + `]
			`)

			state.setGlobal("g", ANY_BOOL, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(
				//we also check that elem has the right type because the test case depends on that
				ANY_STR_LIKE,
				ANY_STRING,
			), res)
		})
	})

	t.Run("URL expressions", func(t *testing.T) {

		t.Run("invalid value for the host part", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				a = 1
				return $a/index.html
			`)

			hostPart := parse.FindNode(n, (*parse.URLExpression)(nil), nil).HostPart

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(hostPart, state, HOST_PART_SHOULD_HAVE_A_HOST_VALUE),
			}, state.errors())
			assert.Equal(t, ANY_URL, res)
		})

		t.Run("invalid query parameter value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				param_value = {}
				return https://example.com/?x={param_value}
			`)

			queryParam := parse.FindNode(n, (*parse.URLQueryParameter)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(queryParam, state, fmtValueNotStringifiableToQueryParamValue(NewEmptyObject())),
			}, state.errors())
			assert.Equal(t, ANY_URL, res)
		})

	})

	t.Run("XML expression", func(t *testing.T) {

		t.Run("namespace not a record", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div></div>`)
			state.setGlobal("html", Nil, GlobalConst)
			res, err := symbolicEval(n, state)

			namespaceIdent := n.Statements[0].(*parse.XMLExpression).Namespace

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(namespaceIdent, state, NAMESPACE_APPLIED_TO_XML_ELEMENT_SHOUD_BE_A_RECORD),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("namespace has not the property for the factory", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div></div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{}), GlobalConst)
			res, err := symbolicEval(n, state)

			namespaceIdent := n.Statements[0].(*parse.XMLExpression).Namespace

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(namespaceIdent, state, MISSING_FACTORY_IN_NAMESPACE_APPLIED_TO_XML_ELEMENT),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("namespace's factory has not the right type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div></div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: Nil,
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			namespaceIdent := n.Statements[0].(*parse.XMLExpression).Namespace

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(namespaceIdent, state, FROM_XML_FACTORY_IS_NOT_A_GO_FUNCTION),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("namespace's factory has not the right signature", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div></div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context) *Object {
					return NewEmptyObject()
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			namespaceIdent := n.Statements[0].(*parse.XMLExpression).Namespace

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(namespaceIdent, state, FROM_XML_FACTORY_SHOULD_HAVE_AT_LEAST_ONE_NON_VARIADIC_PARAM),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("namespace's factory is valid", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div></div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *Identifier {
					return &Identifier{name: elem.name}
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Identifier{name: "div"}, res)
		})

		t.Run("implicit namespace is not defined", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(<div></div>)`)
			res, err := symbolicEval(n, state)

			xmlExpr := n.Statements[0].(*parse.XMLExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(xmlExpr, state, HTML_NS_IS_NOT_DEFINED),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("self-closing element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div/>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *Identifier {
					return &Identifier{name: elem.name}
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Identifier{name: "div"}, res)
		})

		t.Run("interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div>{int}</div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *XMLElement {
					return elem
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &XMLElement{
				name:       "div",
				children:   []Value{ANY_STRING, ANY_INT, ANY_STRING},
				sourceNode: parse.FindNode(n, (*parse.XMLElement)(nil), nil),
			}, res)
		})

		t.Run("interpolation with checking", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div>{int}</div>`)
			goFn := func(ctx *Context, elem *XMLElement) *XMLElement {
				return elem
			}

			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(goFn),
			}), GlobalConst)

			RegisterXMLInterpolationCheckingFunction(goFn, func(n parse.Node, value Value) (errorMsg string) {
				//no error
				return ""
			})
			defer UnregisterXMLCheckingFunction(goFn)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &XMLElement{
				name:       "div",
				children:   []Value{ANY_STRING, ANY_INT, ANY_STRING},
				sourceNode: parse.FindNode(n, (*parse.XMLElement)(nil), nil),
			}, res)
		})

		t.Run("interpolation with checking: unexpected value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div>{int}</div>`)
			goFn := func(ctx *Context, elem *XMLElement) *XMLElement {
				return elem
			}

			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(goFn),
			}), GlobalConst)

			RegisterXMLInterpolationCheckingFunction(goFn, func(n parse.Node, value Value) (errorMsg string) {
				return "integers not allowed"
			})
			defer UnregisterXMLCheckingFunction(goFn)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, &XMLElement{
				name:       "div",
				children:   []Value{ANY_STRING, ANY_INT, ANY_STRING},
				sourceNode: parse.FindNode(n, (*parse.XMLElement)(nil), nil),
			}, res)

			intIdent := parse.FindIdentWithName(n, "int")

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(intIdent, state, "integers not allowed"),
			}, state.errors())
		})

		t.Run("attribute with value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`a = "a"; return html<div a=a></div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *XMLElement {
					return elem
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &XMLElement{
				name:       "div",
				attributes: map[string]Value{"a": NewString("a")},
				children:   []Value{ANY_STRING},
				sourceNode: parse.FindNode(n, (*parse.XMLElement)(nil), nil),
			}, res)
		})

		t.Run("attribute without value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`a = "a"; return html<div a></div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *XMLElement {
					return elem
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &XMLElement{
				name:       "div",
				attributes: map[string]Value{"a": ANY_STRING},
				children:   []Value{ANY_STRING},
				sourceNode: parse.FindNode(n, (*parse.XMLElement)(nil), nil),
			}, res)
		})

		t.Run("error during factory call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`return html<div></div>`)
			state.setGlobal("html", NewNamespace(map[string]Value{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *XMLElement {
					ctx.AddSymbolicGoFunctionError("factory error")
					return elem
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, &XMLElement{
				name:       "div",
				children:   []Value{ANY_STRING},
				sourceNode: parse.FindNode(n, (*parse.XMLElement)(nil), nil),
			}, res)

			xmlExpr := parse.FindNode(n, (*parse.XMLExpression)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(xmlExpr, state, "factory error"),
			}, state.errors())
		})
	})

	t.Run("module parameters ", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(strings.ReplaceAll(`
			manifest {
				parameters: {
					{
						name: #a	
						pattern: %bool
					}
					b: %str
					c: {
						pattern: %int
					}
				}
			}

			return args
		`, "args", globalnames.MOD_ARGS_VARNAME))

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		if !assert.IsType(t, (*ModuleArgs)(nil), res) {
			return
		}

		structVal := res.(*ModuleArgs)
		structType := structVal.typ

		assert.Equal(t, NewModuleParamsPattern([]string{"a", "b", "c"}, []Pattern{
			state.ctx.ResolveNamedPattern("bool"),
			state.ctx.ResolveNamedPattern("str"),
			state.ctx.ResolveNamedPattern("int"),
		}), structType)

		assert.Equal(t, NewModuleArgs(structType, map[string]Value{
			"a": ANY_BOOL,
			"b": ANY_STR_LIKE,
			"c": ANY_INT,
		}), structVal)
	})
	t.Run("module import statement ", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}
				import lib ./lib.ix {}
				return lib
			`)
			importStmt := parse.FindNode(n, (*parse.ImportStatement)(nil), nil)
			state.Module.directlyImportedModules = map[*parse.ImportStatement]*Module{
				importStmt: {
					mainChunk: utils.Must(parse.ParseChunkSource(parse.SourceFile{
						NameString:  "/lib.ix",
						Resource:    "/lib.ix",
						ResourceDir: "/",
						CodeString:  "manifest {}",
					})),
				},
			}

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)

			//check scope data
			stmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)
			data, ok := state.symbolicData.GetGlobalScopeData(stmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			for _, varData := range data.Variables {
				if varData.Name == "lib" {
					return
				}
			}

			assert.Fail(t, "variable not found in scope data")
		})
		t.Run("base global", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}
				import lib ./lib.ix {}
				return lib
			`)
			importStmt := parse.FindNode(n, (*parse.ImportStatement)(nil), nil)
			state.Module.directlyImportedModules = map[*parse.ImportStatement]*Module{
				importStmt: {
					mainChunk: utils.Must(parse.ParseChunkSource(parse.SourceFile{
						NameString:  "/lib.ix",
						Resource:    "/lib.ix",
						ResourceDir: "/",
						CodeString:  "manifest {}; (1 + v)",
					})),
				},
			}
			state.baseGlobals = map[string]Value{
				"v": NewInt(1),
			}

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("error in imported module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}
				import lib ./lib.ix {}
				return lib
			`)
			importStmt := parse.FindNode(n, (*parse.ImportStatement)(nil), nil)
			state.Module.directlyImportedModules = map[*parse.ImportStatement]*Module{
				importStmt: {
					mainChunk: utils.Must(parse.ParseChunkSource(parse.SourceFile{
						NameString:  "/lib.ix",
						Resource:    "/lib.ix",
						ResourceDir: "/",
						CodeString:  "manifest {}\n(1 + \"a\")",
					})),
				},
			}

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			if !assert.Len(t, state.errors(), 1) {
				return
			}

			evalErr := state.errors()[0]

			assert.Equal(t, parse.SourcePositionRange{
				SourceName:  "",
				StartLine:   3,
				StartColumn: 5,
				EndLine:     3,
				EndColumn:   27,
				Span:        importStmt.Span,
			}, evalErr.Location[0])

			assert.Equal(t, parse.SourcePositionRange{
				SourceName:  "/lib.ix",
				StartLine:   2,
				StartColumn: 6,
				EndLine:     2,
				EndColumn:   9,
				Span:        parse.NodeSpan{Start: 17, End: 20},
			}, evalErr.Location[1])

			assert.Equal(t, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "\"a\""), evalErr.Message)

			assert.Equal(t, ANY, res)
		})

		t.Run("error in file included by imported module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				manifest {}
				import lib ./lib.ix {}
				return lib
			`)
			importStmt := parse.FindNode(n, (*parse.ImportStatement)(nil), nil)
			importedModule := &Module{
				mainChunk: utils.Must(parse.ParseChunkSource(parse.SourceFile{
					NameString:  "/lib.ix",
					Resource:    "/lib.ix",
					ResourceDir: "/",
					CodeString:  "manifest {}\nimport ./included.ix",
				})),
				inclusionStatementMap: map[*parse.InclusionImportStatement]*IncludedChunk{},
			}

			state.Module.directlyImportedModules = map[*parse.ImportStatement]*Module{importStmt: importedModule}
			importedModule.inclusionStatementMap = map[*parse.InclusionImportStatement]*IncludedChunk{
				parse.FindNode(importedModule.mainChunk.Node, (*parse.InclusionImportStatement)(nil), nil): {
					ParsedChunkSource: utils.Must(parse.ParseChunkSource(parse.SourceFile{
						NameString:  "/included.ix",
						Resource:    "/included.ix",
						ResourceDir: "/",
						CodeString:  "includable-file\n(1+\"a\")",
					})),
				},
			}

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			if !assert.Len(t, state.errors(), 1) {
				return
			}

			evalErr := state.errors()[0]

			assert.Equal(t, parse.SourcePositionRange{
				SourceName:  "",
				StartLine:   3,
				StartColumn: 5,
				EndLine:     3,
				EndColumn:   27,
				Span:        importStmt.Span,
			}, evalErr.Location[0])

			assert.Equal(t, parse.SourcePositionRange{
				SourceName:  "/lib.ix",
				StartLine:   2,
				StartColumn: 1,
				EndLine:     2,
				EndColumn:   21,
				Span:        parse.NodeSpan{Start: 12, End: 32},
			}, evalErr.Location[1])

			assert.Equal(t, parse.SourcePositionRange{
				SourceName:  "/included.ix",
				StartLine:   2,
				StartColumn: 4,
				EndLine:     2,
				EndColumn:   7,
				Span:        parse.NodeSpan{Start: 19, End: 22},
			}, evalErr.Location[2])

			assert.Equal(t, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "\"a\""), evalErr.Message)

			assert.Equal(t, ANY, res)
		})
	})

	t.Run("inclusion import statement ", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				manifest {}
				import ./lib.ix
				return a
			`, map[string]string{"./lib.ix": "a = int"})
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			//check scope data
			stmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)
			data, ok := state.symbolicData.GetLocalScopeData(stmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			for _, varData := range data.Variables {
				if varData.Name == "a" {
					return
				}
			}

			assert.Fail(t, "variable not found in scope data")
		})

		t.Run("file does not exist", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				manifest {}
				import ./lib.ix
				return int
			`, map[string]string{})
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("error in included file", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				manifest {}
				import ./lib.ix
				return a
			`, map[string]string{"./lib.ix": "a = b"})
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			if !assert.Len(t, state.errors(), 1) {
				return
			}
			evalErr := state.errors()[0]

			importStmt := parse.FindNode(n, (*parse.InclusionImportStatement)(nil), nil)

			assert.Equal(t, parse.SourcePositionRange{
				SourceName:  "",
				StartLine:   3,
				StartColumn: 5,
				EndLine:     3,
				EndColumn:   20,
				Span:        importStmt.Span,
			}, evalErr.Location[0])

			assert.Equal(t, parse.SourcePositionRange{
				SourceName:  "./lib.ix",
				StartLine:   1,
				StartColumn: 5,
				EndLine:     1,
				EndColumn:   6,
				Span:        parse.NodeSpan{Start: 4, End: 5},
			}, evalErr.Location[1])

			assert.Equal(t, ANY, res)
		})
	})

	t.Run("extend statement", func(t *testing.T) {
		t.Run("one property", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				pattern p = {a: 1}

				extend p {
					b: - self.a
				}
			`, nil)
			_, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			//check context data
			stmt, ancestors := parse.FindNodeAndChain(n, (*parse.ExtendStatement)(nil), nil)
			data, ok := state.symbolicData.GetContextData(stmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Extensions, 1) {
				return
			}
			extension := data.Extensions[0]
			if !assert.Len(t, extension.PropertyExpressions, 1) {
				return
			}

			propExpr := extension.PropertyExpressions[0]
			assert.NotNil(t, propExpr.Expression)
			assert.Nil(t, propExpr.Method)
		})

		t.Run("one method", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				pattern p = {a: 1}

				extend p {
					b: fn(){

					}
				}
			`, nil)
			_, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			//check context data
			stmt, ancestors := parse.FindNodeAndChain(n, (*parse.ExtendStatement)(nil), nil)
			data, ok := state.symbolicData.GetContextData(stmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			if !assert.Len(t, data.Extensions, 1) {
				return
			}
			extension := data.Extensions[0]
			if !assert.Len(t, extension.PropertyExpressions, 1) {
				return
			}

			propExpr := extension.PropertyExpressions[0]
			assert.Nil(t, propExpr.Expression)
			assert.NotNil(t, propExpr.Method)
		})

		t.Run("properties of the extension object should not have the same name as an existing property", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				pattern p = {a: 1}

				extend p {
					a: - self.a
				}
			`, nil)

			objProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objProp.Key, state, fmtExtendedValueAlreadyHasAnXProperty("a")),
			}, state.errors())
		})

		t.Run("extension object should not have elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				pattern p = {a: 1}

				extend p {
					- self.a
					"2": self.a
				}
			`, nil)

			objProps := parse.FindNodes(n.Statements[1], (*parse.ObjectProperty)(nil), nil)
			if !assert.Len(t, objProps, 2) {
				return
			}

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objProps[0], state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS),
				makeSymbolicEvalError(objProps[1].Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS),
			}, state.errors())
		})

		t.Run("properties of the extension object should be valid inox identifiers", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				pattern p = {a: 1}

				extend p {
					" b": - self.a
					"b ": - self.a
					"b-": - self.a
					"?": - self.a
					"": - self.a

					"ok": - self.a
				}
			`, nil)

			objProps := parse.FindNodes(n.Statements[1], (*parse.ObjectProperty)(nil), nil)
			if !assert.Len(t, objProps, 6) {
				return
			}

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(objProps[0].Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS),
				makeSymbolicEvalError(objProps[1].Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS),
				makeSymbolicEvalError(objProps[2].Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS),
				makeSymbolicEvalError(objProps[3].Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS),
				makeSymbolicEvalError(objProps[4].Key, state, KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS),
			}, state.errors())
		})

		t.Run("properties of the extension object should be type checked", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				pattern p = {val: true}

				extend p {
					negated: - self.val
				}
			`, nil)

			unaryExpr := parse.FindNode(n.Statements[1], (*parse.UnaryExpression)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(unaryExpr, state, fmtOperandOfNumberNegateShouldBeIntOrFloat(TRUE)),
			}, state.errors())
		})

		if true {
			return
		}

		//TODO: adapt following tests for extend statements

		t.Run("(move this test in `call Inox function`) method returning a property (identifier member expression with single property)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				o = {
					f: fn() => self.a,
					a: int,
				}

				return o.f()
			`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("(move this test in `call Inox function`) method returning a property (member expression)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				o = {
					f: fn() => self.a,
					a: int,
				}

				return $o.f()
			`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("(move this test in `call Inox function`) method returning a property (identifier member expression with two properties)", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				inner = {
					f: fn() => self.a,
					a: int,
				}


				o = {inner: inner}

				return o.inner.f()
			`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		//TODO: adapt following tests for extend statements

		t.Run("methods", func(t *testing.T) {
			t.Run("method returning a property", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					{
						f: fn() => self.a,
						a: int,
					}
				`)

				fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				expectedFunc := &InoxFunction{
					node:      fnExpr,
					nodeChunk: n,
					result:    ANY_INT,
				}

				assert.Equal(t, &Object{
					entries: map[string]Serializable{
						"a": ANY_INT,
						"f": expectedFunc,
					},
					static: map[string]Pattern{
						"a": ANY_INT.Static(),
						"f": getStatic(expectedFunc),
					},
				}, res)
			})

			t.Run("method returning a dynamic member", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					{
						f: fn() => self.<a,
						a: int,
					}
				`)

				fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)

				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				expectedFunction := &InoxFunction{
					node:      fnExpr,
					nodeChunk: n,
					result:    NewDynamicValue(ANY_INT),
				}

				assert.Equal(t, &Object{
					entries: map[string]Serializable{
						"a": ANY_INT,
						"f": expectedFunction,
					},
					static: map[string]Pattern{
						"a": ANY_INT.Static(),
						"f": getStatic(expectedFunction),
					},
				}, res)
			})

			t.Run("method calling another method : caller declared first", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					{
						a: int,
						f: fn() => self.g,
						g: fn() => self.a,
					}
				`)

				fFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[0]
				gFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[1]

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				obj := res.(*Object)
				g, _, _ := obj.GetProperty("g")

				expectedF := &InoxFunction{
					node:      fFnExpr,
					nodeChunk: n,
					result:    g,
				}

				expectedG := &InoxFunction{
					node:      gFnExpr,
					nodeChunk: n,
					result:    ANY_INT,
				}

				assert.Equal(t, &Object{
					entries: map[string]Serializable{
						"a": ANY_INT,
						"f": expectedF,
						"g": expectedG,
					},
					static: map[string]Pattern{
						"a": ANY_INT.Static(),
						"f": getStatic(expectedF),
						"g": getStatic(expectedG),
					},
				}, obj)
			})

			t.Run("method calling another method : callee declared first", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					{
						a: int,
						g: fn() => self.a,
						f: fn() => self.g,
					}
				`)

				gFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[0]
				fFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[1]

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				obj := res.(*Object)
				g, _, _ := obj.GetProperty("g")

				expectedF := &InoxFunction{
					node:      fFnExpr,
					nodeChunk: n,
					result:    g,
				}

				expectedG := &InoxFunction{
					node:      gFnExpr,
					nodeChunk: n,
					result:    ANY_INT,
				}

				assert.Equal(t, &Object{
					entries: map[string]Serializable{
						"a": ANY_INT,
						"f": expectedF,
						"g": expectedG,
					},
					static: map[string]Pattern{
						"a": ANY_INT.Static(),
						"f": getStatic(expectedF),
						"g": getStatic(expectedG),
					},
				}, obj)
			})

			t.Run("cycle of two methods", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					{
						f: fn() => self.g,
						g: fn() => self.f,
					}
				`)

				objExpr := n.Statements[0]
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objExpr, state, fmtMethodCyclesDetected([][]string{{".g", ".f"}})),
				}, state.errors())
				assert.Equal(t, ANY_OBJ, res)
			})
			t.Run("cycle of three methods", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					{
						f: fn() => self.g,
						g: fn() => self.h,
						h: fn() => self.f,
					}
				`)

				objExpr := n.Statements[0]
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objExpr, state, fmtMethodCyclesDetected([][]string{{".g", ".h", ".f"}})),
				}, state.errors())
				assert.Equal(t, ANY_OBJ, res)
			})

			t.Run("invalid mutation of a list", func(t *testing.T) {
				t.Run("", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l = [1]
						l.append(true)
						return l
					`)

					callExpr := n.Statements[1]
					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Equal(t, []SymbolicEvaluationError{
						makeSymbolicEvalError(callExpr, state, INVALID_MUTATION),
					}, state.errors())
					assert.Equal(t, NewList(NewInt(1)), res)
				})
				t.Run("", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l [1] = [1]
						l.append(1)
						return l
					`)

					callExpr := n.Statements[1]
					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Equal(t, []SymbolicEvaluationError{
						makeSymbolicEvalError(callExpr, state, INVALID_MUTATION),
					}, state.errors())
					assert.Equal(t, NewList(NewInt(1)), res)
				})

				t.Run("", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l []%(1) = [1]
						l.append(2)
						return l
					`)

					callExpr := n.Statements[1]
					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Equal(t, []SymbolicEvaluationError{
						makeSymbolicEvalError(callExpr, state, INVALID_MUTATION),
					}, state.errors())
					assert.Equal(t, NewList(NewInt(1)), res)
				})
			})

			t.Run("valid mutation of a list", func(t *testing.T) {
				t.Run("append an int to a list with a single int element", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l = [1]
						l.append(2)
						return l
					`)

					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Empty(t, state.errors())
					assert.Equal(t, NewListOf(ANY_INT), res)
				})

				t.Run("append an int to an empty list", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l = [1]
						l.append(2)
						return l
					`)

					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Empty(t, state.errors())
					assert.Equal(t, NewListOf(ANY_INT), res)
				})

				t.Run("append two different ints to an empty list", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l = []
						l.append(1, 2)
						return l
					`)

					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Empty(t, state.errors())
					assert.Equal(t, NewListOf(AsSerializableChecked(INT_1_OR_2)), res)
				})

				t.Run("append an int to a list with a single int element that has []int as static type", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l []int = [1]
						l.append(2)
						return l
					`)

					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Empty(t, state.errors())
					assert.Equal(t, NewListOf(ANY_INT), res)
				})

				t.Run("append an int to an empty list that has []int as static type", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l []int = []
						l.append(2)
						return l
					`)

					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Empty(t, state.errors())
					assert.Equal(t, NewListOf(INT_2), res)
				})

				t.Run("append two different ints to an empty list that has []int as static type", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l []int = []
						l.append(1, 2)
						return l
					`)

					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Empty(t, state.errors())
					assert.Equal(t, NewListOf(AsSerializableChecked(INT_1_OR_2)), res)
				})

				t.Run("append an int to a list with a single int element that has []%(1) as static type", func(t *testing.T) {
					n, state := MakeTestStateAndChunk(`
						var l []%(1) = [1]
						l.append(1)
						return l
					`)

					res, err := symbolicEval(n, state)

					assert.NoError(t, err)
					assert.Empty(t, state.errors())
					assert.Equal(t, NewListOf(NewInt(1)), res)
				})
			})
		})
	})

	t.Run("double-colon expression", func(t *testing.T) {

		t.Run("unterminated", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				obj = {
					list: []
				}
				obj::
				return obj
			`, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewInexactObject(map[string]Serializable{
				"list": NewList(),
			}, nil, map[string]Pattern{
				"list": NewListPatternOf(&TypePattern{val: ANY_SERIALIZABLE}),
			}), res)
		})

		t.Run("mutation of an object property", func(t *testing.T) {
			t.Run("call of a property's method", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					obj = {
						list: []
					}
					obj::list.append(1)
					return obj
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewInexactObject(map[string]Serializable{
					"list": NewListOf(INT_1),
				}, nil, map[string]Pattern{
					"list": NewListPatternOf(&TypePattern{val: ANY_SERIALIZABLE}),
				}), res)
			})

			t.Run("assignment of an index expression", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					obj = {
						list: [0]
					}
					obj::list[0] = 1
					return obj
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewInexactObject(map[string]Serializable{
					"list": NewListOf(ANY_INT),
				}, nil, map[string]Pattern{
					"list": NewListPatternOf(&TypePattern{val: ANY_INT}),
				}), res)
			})

			t.Run("assignment of the member of an index expression", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					obj = {
						list: [{a: 0}]
					}
					obj::list[0].a = 1
					return obj
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())

				_ = res
				//TODO

				// assert.Equal(t, NewInexactObject(map[string]Serializable{
				// 	"list": NewList(NewInexactObject(map[string]Serializable{
				// 		"a": NewInt(1),
				// 	}, nil, map[string]Pattern{
				// 		"a": &TypePattern{val: ANY_INT},
				// 	})),
				// }, nil, map[string]Pattern{
				// 	"list": NewListPatternOf(NewInexactObjectPattern(map[string]Pattern{
				// 		"a": &TypePattern{val: ANY_INT},
				// 	}, nil)),
				// }), res)
			})
			//TODO: support deeper accesses
		})

		t.Run("accessed object properties should never be stored", func(t *testing.T) {
			testCases := []struct {
				input string
			}{
				{
					input: `
						obj = {list: []}
						obj::list
					`,
				},
				{
					input: `
						obj = {list: []}
						a = obj::list
					`,
				},
				{
					input: `
						obj = {list: []}
						return obj::list
					`,
				},
				{
					input: `
						fn print(arg){}
						obj = {list: []}
						print(obj::list)
					`,
				},
				//index expression
				{
					input: `
						obj = {list: [0]}
						obj::list[0]
					`,
				},
				{
					input: `
						obj = {list: [0]}
						a = obj::list[0]
					`,
				},
				{
					input: `
						obj = {list: [0]}
						return obj::list[0]
					`,
				},
				{
					input: `
						fn print(arg){}
						obj = {list: [0]}
						print(obj::list[0])
					`,
				},
				//slice expression
				{
					input: `
						obj = {list: [0]}
						obj::list[0:1]
					`,
				},
				{
					input: `
						obj = {list: [0]}
						a = obj::list[0:1]
					`,
				},
				{
					input: `
						obj = {list: [0]}
						return obj::list[0:1]
					`,
				},
				{
					input: `
						fn print(arg){}
						obj = {list: [0]}
						print(obj::list[0:1])
					`,
				},
			}

			for _, testCase := range testCases {
				n, state := MakeTestStateAndChunk(testCase.input)
				_, err := symbolicEval(n, state)

				if !assert.NoError(t, err) {
					return
				}

				errs := state.errors()
				if !assert.Len(t, errs, 1) {
					return
				}

				evalErr := errs[0]
				assert.ErrorContains(t, evalErr, MISPLACED_DOUBLE_COLON_EXPR)
			}

		})

		t.Run("extension's property", func(t *testing.T) {
			t.Run("object", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					pattern o = {
						# we do not use "int" because it is not concretizable (concrete type pattern is not available)
						a: 1
					}

					extend o {
						b: - self.a
					}

					var o o = {
						a: 1
					}

					return o::b
				`)

				res, err := symbolicEval(n, state)
				if !assert.NoError(t, err) {
					return
				}
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)

				extension, ok := state.symbolicData.GetUsedTypeExtension(parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil))
				if !assert.True(t, ok) {
					return
				}

				assert.Len(t, extension.PropertyExpressions, 1)

				extensions, ok := state.symbolicData.GetAvailableTypeExtensions(parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil))
				if !assert.True(t, ok) {
					return
				}

				assert.Len(t, extensions, 1)
			})

		})

		t.Run("extension's method", func(t *testing.T) {

			base := `
				pattern o = {
					# we do not use "int" because it is not concretizable (concrete type pattern is not available)
					a: 1
				}

				extend o {
					f: fn(){
						return 1
					}
				}

				var o o = {
					a: 1
				}
			`
			t.Run("accessed methods should never be stored", func(t *testing.T) {
				testCases := []struct {
					input string
				}{
					{
						input: base + `
							o::f
						`,
					},
					{
						input: base + `
							a = o::f
						`,
					},
					{
						input: base + `
							return o::f
						`,
					},
					{
						input: base + `
							fn print(arg){}
							print(o::f)
						`,
					},
					//index expression
					{
						input: base + `
							o::f[0]
						`,
					},
					{
						input: base + `
							a = o::f[0]
						`,
					},
					{
						input: base + `
							return o::f[0]
						`,
					},
					{
						input: base + `
							fn print(arg){}
							print(o::f[0])
						`,
					},
					//slice expression
					{
						input: base + `
							o::f[0:1]
						`,
					},
					{
						input: base + `
							a = o::f[0:1]
						`,
					},
					{
						input: base + `
							return o::f[0:1]
						`,
					},
					{
						input: base + `
							fn print(arg){}
							print(o::f[0:1])
						`,
					},
				}

				for _, testCase := range testCases {
					n, state := MakeTestStateAndChunk(testCase.input)
					_, err := symbolicEval(n, state)

					if !assert.NoError(t, err) {
						return
					}

					errs := state.errors()
					if !assert.Greater(t, len(errs), 0) {
						return
					}
					if !assert.LessOrEqual(t, len(errs), 3) {
						return
					}

					evalErr := errs[0]
					assert.ErrorContains(t, evalErr, MISPLACED_DOUBLE_COLON_EXPR_EXT_METHOD_CAN_ONLY_BE_CALLED)
				}

			})

		})

		t.Run("extensions should be available inside functions: declarations and calls", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern o = {
					# we do not use "int" because it is not concretizable (concrete type pattern is not available)
					a: 1
				}

				extend o {
					b: - self.a
				}

				var o o = {
					a: 1
				}
				
				fn get_b(arg o){
					return arg::b
				}

				return get_b(o)
			`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)

			extension, ok := state.symbolicData.GetUsedTypeExtension(parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil))
			if !assert.True(t, ok) {
				return
			}

			assert.Len(t, extension.PropertyExpressions, 1)

			extensions, ok := state.symbolicData.GetAvailableTypeExtensions(parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil))
			if !assert.True(t, ok) {
				return
			}

			assert.Len(t, extensions, 1)
		})

		t.Run("retrieval of the property of a URL-referenced entity", func(t *testing.T) {

			userPattern := NewInexactObjectPattern(map[string]Pattern{"name": &TypePattern{val: ANY_STRING}}, nil)
			db := NewDatabaseIL(DatabaseILParams{
				Schema: NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
			})

			t.Run("base case", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					url = ldb://main/user
					return url::name
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)

				res, err := symbolicEval(n, state)
				if !assert.NoError(t, err) {
					return
				}
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_STRING, res)

				doubleColonExpr := parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil)
				_, ok := state.symbolicData.GetUsedTypeExtension(doubleColonExpr)
				assert.False(t, ok)

				value, ok := state.symbolicData.GetURLReferencedEntity(doubleColonExpr)
				if !assert.True(t, ok) {
					return
				}
				expected := utils.Must(ShareOrClone(userPattern.SymbolicValue(), state))
				assert.Equal(t, expected, value)
			})

			t.Run("missing property's name", func(t *testing.T) {
				n, state, _ := _makeStateAndChunk(`
					url = ldb://main/user
					return url::
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)
				doubleColonExpr := parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil)

				res, err := symbolicEval(n, state)
				if !assert.NoError(t, err) {
					return
				}
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_SERIALIZABLE, res)

				_, ok := state.symbolicData.GetUsedTypeExtension(doubleColonExpr)
				assert.False(t, ok)

				value, ok := state.symbolicData.GetURLReferencedEntity(doubleColonExpr)
				if !assert.True(t, ok) {
					return
				}
				expected := utils.Must(ShareOrClone(userPattern.SymbolicValue(), state))
				assert.Equal(t, expected, value)
			})

			t.Run("inexisting property", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					url = ldb://main/user
					return url::non_existing_prop
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)
				doubleColonExpr := parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil)

				res, err := symbolicEval(n, state)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(doubleColonExpr.Element, state, fmtValueAtURLDoesNotHavePropX(userPattern.SymbolicValue(), "non_existing_prop")),
				}, state.errors())
				assert.Equal(t, ANY_SERIALIZABLE, res)

				_, ok := state.symbolicData.GetUsedTypeExtension(doubleColonExpr)
				assert.False(t, ok)

				value, ok := state.symbolicData.GetURLReferencedEntity(doubleColonExpr)
				if !assert.True(t, ok) {
					return
				}
				expected := utils.Must(ShareOrClone(userPattern.SymbolicValue(), state))
				assert.Equal(t, expected, value)
			})

			t.Run("trailing slash", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					url = ldb://main/user/
					return url::name
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)

				res, err := symbolicEval(n, state)
				if !assert.NoError(t, err) {
					return
				}

				doubleColonExpr := parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(doubleColonExpr.Left, state, PATH_OF_URL_SHOULD_NOT_HAVE_A_TRAILING_SLASH),
				}, state.errors())
				assert.Equal(t, ANY_SERIALIZABLE, res)

				_, ok := state.symbolicData.GetUsedTypeExtension(doubleColonExpr)
				assert.False(t, ok)

				_, ok = state.symbolicData.GetURLReferencedEntity(doubleColonExpr)
				assert.False(t, ok)
			})

			t.Run("root", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					url = ldb://main/
					return url::name
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)

				res, err := symbolicEval(n, state)
				if !assert.NoError(t, err) {
					return
				}

				doubleColonExpr := parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(doubleColonExpr.Left, state, ROOT_PATH_NOT_ALLOWED_REFERS_TO_DB),
				}, state.errors())
				assert.Equal(t, ANY_SERIALIZABLE, res)

				_, ok := state.symbolicData.GetUsedTypeExtension(doubleColonExpr)
				assert.False(t, ok)

				_, ok = state.symbolicData.GetURLReferencedEntity(doubleColonExpr)
				assert.False(t, ok)
			})

			t.Run("inexisting entity", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					url = ldb://main/userx
					return url::name
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)

				res, err := symbolicEval(n, state)
				if !assert.NoError(t, err) {
					return
				}
				assert.NotEmpty(t, state.errors())
				assert.Equal(t, ANY_SERIALIZABLE, res)

				doubleColonExpr := parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil)
				_, ok := state.symbolicData.GetUsedTypeExtension(doubleColonExpr)
				assert.False(t, ok)

				_, ok = state.symbolicData.GetURLReferencedEntity(doubleColonExpr)
				assert.False(t, ok)
			})

		})

		t.Run("directly calling the method of a URL-referenced value is not allowed", func(t *testing.T) {
			listPropPattern := NewListPatternOf(&TypePattern{val: ANY_INT})
			userPattern := NewInexactObjectPattern(map[string]Pattern{
				"list": listPropPattern,
			}, nil)
			db := NewDatabaseIL(DatabaseILParams{
				Schema: NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
			})

			n, state := MakeTestStateAndChunk(`
					url = ldb://main/user/list
					url::append(1)
				`)
			state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)
			doubleColonExpr := parse.FindNode(n, (*parse.DoubleColonExpression)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(doubleColonExpr.Element, state, DIRECTLY_CALLING_METHOD_OF_URL_REF_ENTITY_NOT_ALLOWED),
			}, state.errors())

			value, ok := state.symbolicData.GetURLReferencedEntity(doubleColonExpr)
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, listPropPattern.SymbolicValue(), value)
		})

		t.Run("mutation of a URL-referenced entity", func(t *testing.T) {

			listPropPattern := NewListPatternOf(&TypePattern{val: ANY_INT})
			userPattern := NewInexactObjectPattern(map[string]Pattern{
				"name": &TypePattern{val: ANY_STRING},
				"list": listPropPattern,
			}, nil)
			db := NewDatabaseIL(DatabaseILParams{
				Schema: NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
			})

			t.Run("call of a property's method", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					url = ldb://main/user
					url::list.append(1)
					return url::list
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)

				doubleColonExprs := parse.FindNodes(n, (*parse.DoubleColonExpression)(nil), nil)
				if !assert.Len(t, doubleColonExprs, 2) {
					return
				}

				for _, expr := range doubleColonExprs {
					value, ok := state.symbolicData.GetURLReferencedEntity(expr)
					if !assert.True(t, ok) {
						return
					}
					expected := utils.Must(ShareOrClone(userPattern.SymbolicValue(), state))
					assert.Equal(t, expected, value)
				}
			})

			t.Run("assignment of an index expression", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					url = ldb://main/user
					url::list[0] = 1
					return url::list
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(ANY_INT), res)

				doubleColonExprs := parse.FindNodes(n, (*parse.DoubleColonExpression)(nil), nil)
				if !assert.Len(t, doubleColonExprs, 2) {
					return
				}

				for _, expr := range doubleColonExprs {
					value, ok := state.symbolicData.GetURLReferencedEntity(expr)
					if !assert.True(t, ok) {
						return
					}
					expected := utils.Must(ShareOrClone(userPattern.SymbolicValue(), state))
					assert.Equal(t, expected, value)
				}
			})

			t.Run("assignment of the member of an index expression", func(t *testing.T) {
				listPropPattern := NewListPatternOf(NewInexactObjectPattern(nil, nil))
				userPattern := NewInexactObjectPattern(map[string]Pattern{
					"name": &TypePattern{val: ANY_STRING},
					"list": listPropPattern,
				}, nil)
				db := NewDatabaseIL(DatabaseILParams{
					Schema: NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
				})

				n, state := MakeTestStateAndChunk(`
					url = ldb://main/user
					url::list[0].a = 1
					return url::list
				`)
				state.setGlobal(globalnames.DATABASES, NewNamespace(map[string]Value{"main": db}), GlobalConst)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, NewListOf(NewInexactObject2(nil)), res)

				doubleColonExprs := parse.FindNodes(n, (*parse.DoubleColonExpression)(nil), nil)
				if !assert.Len(t, doubleColonExprs, 2) {
					return
				}

				for _, expr := range doubleColonExprs {
					value, ok := state.symbolicData.GetURLReferencedEntity(expr)
					if !assert.True(t, ok) {
						return
					}
					expected := utils.Must(ShareOrClone(userPattern.SymbolicValue(), state))
					assert.Equal(t, expected, value)
				}
			})
		})

	})

	t.Run("struct definition", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {}
			`, nil)

			_, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())

			types, ok := state.symbolicData.GetComptimeTypes(n)
			if !assert.True(t, ok) {
				return
			}
			typ, ok := types.GetType("MyStruct")
			if !assert.True(t, ok) {
				return
			}
			_ = typ.(*StructType)
		})

		t.Run("field with builtin type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a int
				}
			`, nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			types, ok := state.symbolicData.GetComptimeTypes(n)
			if !assert.True(t, ok) {
				return
			}
			typ, ok := types.GetType("MyStruct")
			if !assert.True(t, ok) {
				return
			}
			structType := typ.(*StructType)
			if !assert.Equal(t, 1, structType.FieldCount()) {
				return
			}
			assert.Equal(t, StructField{Name: "a", Type: BUILTIN_COMPTIME_TYPES["int"]}, structType.Field(0))
		})

		t.Run("field with builtin pointer type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a *int
				}
			`, nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			types, ok := state.symbolicData.GetComptimeTypes(n)
			if !assert.True(t, ok) {
				return
			}
			typ, ok := types.GetType("MyStruct")
			if !assert.True(t, ok) {
				return
			}
			structType := typ.(*StructType)
			if !assert.Equal(t, 1, structType.FieldCount()) {
				return
			}
			expectedStructField := StructField{Name: "a", Type: newPointerType(BUILTIN_COMPTIME_TYPES["int"])}
			assert.Equal(t, expectedStructField, structType.Field(0))
		})

		t.Run("field with struct type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a Int
				}
				struct Int {
					val int
				}
			`, nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			types, ok := state.symbolicData.GetComptimeTypes(n)
			if !assert.True(t, ok) {
				return
			}
			typ, ok := types.GetType("MyStruct")
			if !assert.True(t, ok) {
				return
			}
			MyStructType := typ.(*StructType)
			if !assert.Equal(t, 1, MyStructType.FieldCount()) {
				return
			}

			IntStructType, ok := types.GetType("Int")
			if !assert.True(t, ok) {
				return
			}

			expectedFieldType := StructField{Name: "a", Type: IntStructType}
			assert.Equal(t, expectedFieldType, MyStructType.Field(0))
		})

		t.Run("field with struct type defined in included chunk", func(t *testing.T) {
			n, state := MakeTestStateAndChunks(`
				manifest {}
				import ./dep.ix

				struct MyStruct {
					a Int
				}
			`, map[string]string{
				"./dep.ix": `
					includable-file
					struct Int {
						val int
					}
				`,
			})

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			types, ok := state.symbolicData.GetComptimeTypes(n)
			if !assert.True(t, ok) {
				return
			}
			typ, ok := types.GetType("MyStruct")
			if !assert.True(t, ok) {
				return
			}
			MyStructType := typ.(*StructType)
			if !assert.Equal(t, 1, MyStructType.FieldCount()) {
				return
			}

			IntStructType, ok := types.GetType("Int")
			if !assert.True(t, ok) {
				return
			}

			expectedFieldType := StructField{Name: "a", Type: IntStructType}
			assert.Equal(t, expectedFieldType, MyStructType.Field(0))
		})

		t.Run("field with struct pointer type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {
					a *Int
				}
				struct Int {
					val int
				}
			`, nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			types, ok := state.symbolicData.GetComptimeTypes(n)
			if !assert.True(t, ok) {
				return
			}
			typ, ok := types.GetType("MyStruct")
			if !assert.True(t, ok) {
				return
			}
			MyStructType := typ.(*StructType)
			if !assert.Equal(t, 1, MyStructType.FieldCount()) {
				return
			}

			IntStructType, ok := types.GetPointerType("Int")
			if !assert.True(t, ok) {
				return
			}

			expectedFieldType := StructField{Name: "a", Type: IntStructType}
			assert.Equal(t, expectedFieldType, MyStructType.Field(0))
		})
	})

	t.Run("new expression", func(t *testing.T) {

		var (
			INT_TYPE     = BUILTIN_COMPTIME_TYPES["int"]
			INT_PTR_TYPE = newPointerType(INT_TYPE)
		)

		t.Run("builtin type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`new int`, nil)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, INT_PTR_TYPE.SymbolicValue(), res)
		})

		t.Run("struct type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				struct MyStruct {}
				return new MyStruct
			`, nil)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())

			types, ok := state.symbolicData.GetComptimeTypes(n)
			if !assert.True(t, ok) {
				return
			}

			MyStructType, ok := types.GetType("MyStruct")
			if !assert.True(t, ok) {
				return
			}

			ptrType := newPointerType(MyStructType)

			assert.Equal(t, ptrType.SymbolicValue(), res)
		})

		t.Run("pattern name", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				pattern o = {a: 1}
				return new o
			`, nil)
			newExpr := parse.FindNode(n, (*parse.NewExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(newExpr.Type, state, ONLY_COMPILE_TIME_TYPES_CAN_BE_USED_IN_NEW_EXPRS),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})
	})

	t.Run("the evaluation should stop if the context context is done AND there is no remaining no-check fuel", func(t *testing.T) {
		nodeCount := parse.CountNodes(parse.MustParseChunk("[]"))

		n, state := MakeTestStateAndChunk("[" + strings.Repeat("1,", INITIAL_NO_CHECK_FUEL-nodeCount+1) + "]")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		state.ctx.startingConcreteContext = dummyConcreteContext{ctx}
		_, err := symbolicEval(n, state)
		assert.ErrorContains(t, err, "stopped symbolic evaluation because context is done")
	})

	t.Run("the evaluation should not stop if the context context is done but there is remaining no-check fuel", func(t *testing.T) {
		nodeCount := parse.CountNodes(parse.MustParseChunk("[]"))

		n, state := MakeTestStateAndChunk("[" + strings.Repeat("1,", INITIAL_NO_CHECK_FUEL-nodeCount) + "]")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		state.ctx.startingConcreteContext = dummyConcreteContext{ctx}
		_, err := symbolicEval(n, state)
		assert.NoError(t, err)
	})
}
