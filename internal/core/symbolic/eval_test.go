package symbolic

import (
	"context"
	"strings"
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicEval(t *testing.T) {
	enableMultivalueCaching = false

	symbolicMap := func(ctx *Context, iterable Iterable, mapper SymbolicValue) *List {
		var MAP_PARAM_NAMES = []string{"iterable", "mapper"}

		makeParams := func(result SymbolicValue) *[]SymbolicValue {
			return &[]SymbolicValue{iterable, NewFunction(
				[]SymbolicValue{iterable.IteratorElementValue()}, nil, false,
				[]SymbolicValue{result},
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

			InitializeObject(obj, entries, nil)
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

	t.Run("byte slice literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("0x[01]")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &ByteSlice{}, res)
	})

	t.Run("integer literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("1")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &Int{hasValue: true, value: 1}, res)
	})

	t.Run("integer range literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("1..2")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, ANY_INT_RANGE, res)
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
		extData.ToSymbolicValue = func(v any, wide bool) (SymbolicValue, error) {
			return v.(SymbolicValue), nil
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

	t.Run("date literal", func(t *testing.T) {
		n, state := MakeTestStateAndChunk("2020y-UTC")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		expectedDate, _ := parse.ParseDateLiteral([]byte("2020y-UTC"))
		assert.Equal(t, NewDate(expectedDate), res)
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
		assert.Equal(t, NewPathPatternFromNode(pathPatternExpr), res)
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
					fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR_LIKE)),
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
					fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR_LIKE)),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("value not assignable to type (deep mismatch: dictionary entry)", func(t *testing.T) {
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
						Cases: []Pattern{
							state.ctx.ResolveNamedPattern("str"),
							state.ctx.ResolveNamedPattern("int"),
						},
					},
				},
			}, res)
		})

		t.Run("multivalue LHS", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return fn(v %| %[]%int | %[]%str){
					var a %| %[]%int | %[]%str = v; 
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
				parameters:     []SymbolicValue{argType},
				parameterNames: []string{"v"},
				result:         argType,
			}
			assert.Equal(t, expectedFn, res)
		})
	})

	t.Run("global variable defintion", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			$$v = []
			return $$v
		`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &List{elements: []Serializable{}}, res)

		//check definition position data
		idents, ancestorChains := parse.FindNodesAndChains(n, (*parse.GlobalVariable)(nil), nil)
		definitionIdent := idents[0]
		returnIdent := idents[1]
		returnIdentAncestors := ancestorChains[1]

		pos, ok := state.symbolicData.GetVariableDefinitionPosition(returnIdent, returnIdentAncestors)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, definitionIdent.Span, pos.Span)
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

		t.Run("global variable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				$$v = []
				return $$v
			`)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(), res)

			//check scope data
			stmt, ancestors := parse.FindNodeAndChain(n, (*parse.ReturnStatement)(nil), nil)
			data, ok := state.symbolicData.GetGlobalScopeData(stmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			for _, varData := range data.Variables {
				if varData.Name == "v" {
					return
				}
			}

			assert.Fail(t, "variable not found in scope data")
		})

		t.Run("RHS has type incompatible with explicit static type of the variable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				%p = %| %int | %str
				var v %p = int
				v = true
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[2]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(TRUE, &UnionPattern{
					Cases: []Pattern{
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
				routine = go do {}
				$obj.routine = routine
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
					"routine": ANY_SERIALIZABLE,
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
				routine = go do {}
				obj.routine = routine
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
				obj.routine = val
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
					"routine": ANY_SERIALIZABLE,
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
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(ANY_INT, &TypePattern{val: ANY_STR})),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STR.Static(),
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
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(ANY_INT, &TypePattern{val: ANY_STR})),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STR.Static(),
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
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(ANY_INT, &TypePattern{val: ANY_STR})),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"name": NewString("foo"),
				},
				static: map[string]Pattern{
					"name": ANY_STR.Static(),
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
						Cases: []Pattern{
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
				Cases: []Pattern{
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

		t.Run("+= assignment, LHS has incompatible type", func(t *testing.T) {
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
					"name": ANY_STR.Static(),
				},
			}, res)
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
					"name": ANY_STR.Static(),
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
						Cases: []Pattern{
							state.ctx.ResolveNamedPattern("str"),
							state.ctx.ResolveNamedPattern("int"),
						},
					},
				},
			}, res)
		})

		t.Run("type annotation with incompatible value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{"name" %str : $$int}`)
			res, err := symbolicEval(n, state)

			valueNode := parse.FindNode(state.Module.mainChunk.Node, (*parse.GlobalVariable)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(valueNode, state, fmtNotAssignableToPropOfType(ANY_INT, state.ctx.ResolveNamedPattern("str"))),
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

		t.Run("non-serializable values not allowed in initialization", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{routine: go do {}}`)
			propNode := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propNode, state, NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"routine": ANY_SERIALIZABLE,
				},
				static: map[string]Pattern{
					"routine": getStatic(ANY_SERIALIZABLE),
				},
			}, res)
		})

		t.Run("non-watchable mutable values not allowed in initialization", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{routine: val}`)
			state.setGlobal("val", ANY_SERIALIZABLE, GlobalConst)
			propNode := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propNode, state, MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE),
			}, state.errors())
			assert.Equal(t, &Object{
				entries: map[string]Serializable{
					"routine": ANY_SERIALIZABLE,
				},
				static: map[string]Pattern{
					"routine": getStatic(ANY_SERIALIZABLE),
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

		t.Run("readonly objects should not have lifetime jobs", func(t *testing.T) {
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

			prop := parse.FindNode(n, (*parse.ObjectProperty)(nil), func(n *parse.ObjectProperty, isUnique bool) bool {
				return n.Name() == "b"
			})

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

		t.Run("mutable value", func(t *testing.T) {
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

		t.Run("non-serializable values not allowed in initialization", func(t *testing.T) {
			//TODO
		})

	})
	t.Run("member expression", func(t *testing.T) {
		t.Run("object", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {"name": "foo"}
				return $v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("foo"), res)
		})

		t.Run("record", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = #{"name": "foo"}
				return $v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("foo"), res)
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
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("name", NewEmptyObject(), "")),
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

			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
			}, state.errors())
			assert.Equal(t, ANY, res)
		})

		t.Run("existing method of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.cancel
			`)

			goVal := &Routine{}
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
			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
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
				return v.($$int)
			`)
			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}

			propNameNode := parse.FindNode(n, (*parse.GlobalVariable)(nil), nil)

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

			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
			}, state.errors())
			assert.Equal(t, NewAnyDynamicValue(), res)
		})

		t.Run("existing method of GoValue", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return v.<cancel
			`)

			goVal := &Routine{}
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
			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
			}, state.errors())
			assert.Equal(t, NewAnyDynamicValue(), res)
		})
	})

	t.Run("identifier member expression", func(t *testing.T) {
		t.Run("object", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				v = {"name": "foo"}
				return v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("foo"), res)
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
				return $$v[0]
			`)
			state.setGlobal("v", &List{generalElement: &String{}}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &String{}, res)
		})

		t.Run("rune slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $$v[0]
			`)
			state.setGlobal("v", &RuneSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Rune{}, res)
		})

		t.Run("byte slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $$v[0]
			`)
			state.setGlobal("v", &ByteSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Byte{}, res)
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
				return $$v[0:]
			`)
			state.setGlobal("v", &List{generalElement: &String{}}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &List{generalElement: &String{}}, res)
		})

		t.Run("rune slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $$v[0:]
			`)
			state.setGlobal("v", &RuneSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &RuneSlice{}, res)
		})

		t.Run("byte slice", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return $$v[0:]
			`)
			state.setGlobal("v", &ByteSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ByteSlice{}, res)
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
				makeSymbolicEvalError(leftOperand, state, fmtLeftOperandOfBinaryShouldBe(parse.Add, "int or float", "\"a\"")),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("<: left operand is a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`("a" < int)`)
			res, err := symbolicEval(n, state)

			leftOperand := n.Statements[0].(*parse.BinaryExpression).Left

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(leftOperand, state, fmtLeftOperandOfBinaryShouldBe(parse.LessThan, "int or float", "\"a\"")),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
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

		t.Run("<: Right operand is a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(int < "a")`)
			res, err := symbolicEval(n, state)

			RightOperand := n.Statements[0].(*parse.BinaryExpression).Right

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(RightOperand, state, fmtRightOperandOfBinaryShouldBe(parse.LessThan, "int", "\"a\"")),
			}, state.errors())
			assert.Equal(t, ANY_BOOL, res)
		})

		t.Run("substrof: left operand is an int", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`(int substrof "1")`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Left, state, fmtLeftOperandOfBinaryShouldBe(parse.Substrof, "string-like", "%int")),
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
				Base:    &AnyPattern{},
				Removed: &AnyPattern{},
			}, res)
		})

		t.Run("set difference: right operand is an integer", func(t *testing.T) {
			n, state := MakeTestStateAndChunk("((%| int | 2) \\ int)")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &DifferencePattern{
				Base:    &AnyPattern{},
				Removed: &AnyPattern{},
			}, res)
		})

		t.Run("binary in/not-in", func(t *testing.T) {

			t.Run("right operand is not a container", func(t *testing.T) {
				n, state := MakeTestStateAndChunk("(1 in [])")
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
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

	t.Run("function declaration", func(t *testing.T) {

		t.Run("missing body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				fn f()
				return f
			`, nil)

			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{node: fnExpr, result: nil}, res)

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
			assert.Equal(t, &InoxFunction{node: fnExpr, result: Nil}, res)

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
				parameters:     []SymbolicValue{ANY},
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
				parameters:     []SymbolicValue{NewListOf(ANY_SERIALIZABLE)},
				parameterNames: []string{"a"},
				result:         NewListOf(ANY_SERIALIZABLE),
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
				result:         ANY_INT,
				capturedLocals: map[string]SymbolicValue{"a": ANY_INT},
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
				node:   fnExpr,
				result: NewList(ANY_INT, NewString("1")),
				capturedLocals: map[string]SymbolicValue{
					"a": ANY_INT,
					"b": NewString("1"),
				},
			}, res)
		})

		t.Run("missing return", func(t *testing.T) {
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

		t.Run("invalid return value (annotation is a unprefixed named pattern)", func(t *testing.T) {
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
				makeSymbolicEvalError(objectProp.Value, state, fmtNotAssignableToPropOfExpectedValue(NewString("a"), ANY_INT)),
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
				%p = int
				fn f(){
					[%p, %int]
				}
				return $$f
			`)
			fnExpr := n.Statements[1].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{node: fnExpr, result: Nil}, res)
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
			assert.Equal(t, &InoxFunction{node: fnExpr, result: nil}, res)

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
				%p = int
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
				return %fn(){}
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				node:           fnPatt,
				returnType:     Nil,
				parameters:     []SymbolicValue{},
				parameterNames: []string{},
			}, res)
		})

		t.Run("single parameter", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %fn(a){}
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				node:           fnPatt,
				returnType:     Nil,
				parameters:     []SymbolicValue{ANY},
				parameterNames: []string{"a"},
			}, res)
		})

		t.Run("missing return", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %fn() %int {
					
				}
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnPatt, state, MISSING_RETURN_IN_FUNCTION_PATT),
			}, state.errors())
			assert.Equal(t, &FunctionPattern{
				node:           fnPatt,
				returnType:     ANY_INT,
				parameters:     []SymbolicValue{},
				parameterNames: []string{},
			}, res)
		})

		t.Run("invalid return value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %fn() %int {
					return "a"
				}
			`)

			fnPatt := parse.FindNode(n, (*parse.FunctionPatternExpression)(nil), nil)
			innerReturnStmt := parse.FindNodes(n, (*parse.ReturnStatement)(nil), nil)[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(innerReturnStmt, state, fmtInvalidReturnValue(NewString("a"), ANY_INT)),
			}, state.errors())
			assert.Equal(t, &FunctionPattern{
				node:           fnPatt,
				returnType:     ANY_INT,
				parameters:     []SymbolicValue{},
				parameterNames: []string{},
			}, res)
		})

	})

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
				node:   fnExpr,
				result: ANY_INT,
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
				node:   fnExpr,
				result: NewDynamicValue(ANY_INT),
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
				node:   fFnExpr,
				result: g,
			}

			expectedG := &InoxFunction{
				node:   gFnExpr,
				result: ANY_INT,
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
				node:   fFnExpr,
				result: g,
			}

			expectedG := &InoxFunction{
				node:   gFnExpr,
				result: ANY_INT,
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
			assert.Equal(t, &Object{}, res)
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
			assert.Equal(t, &Object{}, res)
		})

		t.Run("invalid mutation", func(t *testing.T) {
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

		t.Run("valid mutation", func(t *testing.T) {
			t.Run("", func(t *testing.T) {
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

			t.Run("", func(t *testing.T) {
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

			t.Run("", func(t *testing.T) {
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
			assert.Equal(t, &NilT{}, res)
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
			assert.Equal(t, &NilT{}, res)
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
			assert.Equal(t, &NilT{}, res)
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
				makeSymbolicEvalError(propertyValue, state, fmtNotAssignableToPropOfExpectedValue(NewString("a"), ANY_INT)),
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
				parameters:     []SymbolicValue{NewMultivalue(NewListOf(ANY_INT), NewListOf(ANY_STR_LIKE))},
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

		t.Run("method returning a property (identifier member expression with single property)", func(t *testing.T) {
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

		t.Run("method returning a property (member expression)", func(t *testing.T) {
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

		t.Run("method returning a property (identifier member expression with two properties)", func(t *testing.T) {
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
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(0, 1)),
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
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(2, 1)),
			}, state.errors())
			assert.Equal(t, ANY_INT, res)
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
				parameters:     []SymbolicValue{NewMultivalue(NewListOf(ANY_STR_LIKE), NewListOf(ANY_INT))},
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
				parameters:     []SymbolicValue{NewMultivalue(NewListOf(ANY_STR_LIKE), NewListOf(ANY_INT))},
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
				fn: func(*Context, SymbolicValue, ...*Int) *Int {
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
				return f(#b)
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg SymbolicValue) *Int {
					if _, ok := arg.(*Identifier); ok {
						ctx.SetSymbolicGoFunctionParameters(&[]SymbolicValue{&Identifier{name: "a"}}, []string{"arg"})
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

		t.Run("specific Go function with non-empty object parameter, missing property in argument", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return f({})
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			param := NewInexactObject(map[string]Serializable{
				"prop": ANY_INT,
			}, nil, nil)

			goFunc := &GoFunction{
				fn: func(ctx *Context, arg SymbolicValue) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]SymbolicValue{param}, []string{"arg"})
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
				fn: func(ctx *Context, arg SymbolicValue) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]SymbolicValue{param}, []string{"arg"})
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
				fn: func(ctx *Context, arg SymbolicValue) *Int {
					ctx.SetSymbolicGoFunctionParameters(&[]SymbolicValue{param}, []string{"arg"})
					return ANY_INT
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(propertyValue, state, fmtNotAssignableToPropOfExpectedValue(NewString("a"), ANY_INT)),
			}, state.errors())

			assert.Equal(t, ANY_INT, res)
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
			fnPatt := n.Statements[0].(*parse.FunctionDeclaration).Function.Parameters[0].Type

			expectedFn := &InoxFunction{
				node: fnExpr,
				parameters: []SymbolicValue{
					&Function{
						pattern: &FunctionPattern{
							node:           fnPatt.(*parse.FunctionPatternExpression),
							returnType:     ANY_INT,
							parameters:     []SymbolicValue{},
							parameterNames: []string{},
						},
						parameters:     []SymbolicValue{},
						parameterNames: []string{},
					},
				},
				parameterNames: []string{"func"},
				result:         ANY_INT,
			}
			assert.Equal(t, expectedFn, res)
		})

	})
	t.Run("call pattern", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			%mypattern()
		`)

		state.ctx.AddNamedPattern("mypattern", &TypePattern{
			call: func(ctx *Context, values []SymbolicValue) (Pattern, error) {
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
			n, state := MakeTestStateAndChunk(`%p = readonly {}; return %p`)

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
			n, state := MakeTestStateAndChunk(`%p = readonly #{}; return %p`)

			res, err := symbolicEval(n, state)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, state.errors())

			expectedRecordPattern := NewInexactRecordPattern(map[string]Pattern{}, nil)
			assert.Equal(t, expectedRecordPattern, res)
		})

		t.Run("pattern not convertible to readonly", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`%p = readonly {x: not-convertible}; return %p`)
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
				fn one(){
					return int
				}
	
				fn addOne(i %int){
					$$result = (i + int)
				}
	
				one | addOne $
				return $$result
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("$ is an invalid argument in second call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn one(){
					return "1"
				}
	
				fn addOne(i %int){
					$$result = (i + int)
				}
	
				one | addOne $
				return $$result
			`)

			secondCall := parse.FindNodes(n, (*parse.CallExpression)(nil), nil)[1]

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
					return arg
				}

				idt int | idt $
				result = | idt "a" | idt $

				return result
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewString("a"), res)
		})

		t.Run("anonymous variable should not be defined after pipe statement", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn idt(arg){
					return arg
				}

				idt int | idt $

				return $
			`)

			varIdent := parse.FindNodes(n, (*parse.Variable)(nil), nil)[1]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(varIdent, state, fmtLocalVarIsNotDeclared("")),
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
				makeSymbolicEvalError(ifStmt.Test, state, fmtIfStmtTestNotBoolBut(ANY_INT)),
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
				makeSymbolicEvalError(ifStmt.Test, state, fmtIfStmtTestNotBoolBut(ANY_INT)),
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
				makeSymbolicEvalError(ifStmt.Test, state, fmtIfStmtTestNotBoolBut(ANY_INT)),
			}, state.errors())
			assert.Nil(t, res)
		})

		t.Run("join", func(t *testing.T) {
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
				NewEmptyObject(),
				NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"b": ANY_INT}, nil, map[string]Pattern{"b": ANY_INT.Static()}),
			), res)
		})

		t.Run("join else-if", func(t *testing.T) {
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
				NewEmptyObject(),
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
					makeSymbolicEvalError(membExpr, state, fmtPropOfSymbolicDoesNotExist("prop", NewEmptyObject(), "")),
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
					if $$v? {

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
					if $$v.a? {

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
					} else {
						var b %bool = a.prop
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializable(NewMultivalue(NewInt(1), TRUE)).(Serializable)}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("negated binary == expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if !(a.prop == 1) {
						var b %bool = a.prop
					} else {
						var b %int = a.prop
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializable(NewMultivalue(NewInt(1), TRUE)).(Serializable)}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("binary != expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if (a.prop != 1) {
						var b %bool = a.prop
					} else {
						var b %int = a.prop
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializable(NewMultivalue(NewInt(1), TRUE)).(Serializable)}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})

			t.Run("negated binary != expression narrows the type of a property (%int)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					if !(a.prop != 1) {
						var b %int = a.prop
					} else {
						var b %bool = a.prop
					}
				`)

				object := NewInexactObject(map[string]Serializable{
					"prop": AsSerializable(NewMultivalue(NewInt(1), TRUE)).(Serializable)}, nil, nil)

				state.setGlobal("a", object, GlobalConst)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors())
			})
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

			ifStmt := n.Statements[0]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ifStmt, state, fmtIfExprTestNotBoolBut(ANY_INT)),
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

			ifStmt := n.Statements[0]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(ifStmt, state, fmtIfExprTestNotBoolBut(ANY_INT)),
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
					parameters:     []SymbolicValue{NewMultivalue(ANY_INT, Nil)},
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
					return (if $$v? $$v else false)
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors())
				assert.Equal(t, ANY, res)
			})
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
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR_LIKE)),
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
					$a = 0
					$a += 1
					return $a
				`)
				res, err := symbolicEval(n, state)

				assert.NoError(t, err)
				assert.Empty(t, state.errors())
				assert.Equal(t, ANY_INT, res)
			})
		})

		t.Run("global variable LHS", func(t *testing.T) {

			t.Run("value not assignable to type (deep mismatch: object property)", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$$a = {a: "x"}; 
					$$a = {a: 1}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)

				objectProp := parse.FindNode(n.Statements[1], (*parse.ObjectProperty)(nil), nil)

				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(objectProp.Value, state,
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR)),
				}, state.errors())
			})

			t.Run("+= assignment: invalid RHS", func(t *testing.T) {
				n, state := MakeTestStateAndChunk(`
					$$a = 0
					$$a += true
					return $$a
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
					$$a = 0
					$$a += 1
					return $$a
				`)
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
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR_LIKE)),
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
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR_LIKE)),
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
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR_LIKE)),
				}, state.errors())
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
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), NewString("x"))),
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
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool) bool {
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
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool) bool {
					return n.Value == -1
				})

				assert.NoError(t, err)
				assert.Equal(t, []SymbolicEvaluationError{
					makeSymbolicEvalError(index, state, INDEX_IS_OUT_OF_BOUNDS),
				}, state.errors())
				assert.Equal(t, NewList(NewInt(0)), res)
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
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), NewString("x"))),
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
						fmtNotAssignableToPropOfExpectedValue(NewInt(1), ANY_STR)),
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
				state.setGlobal("Array", WrapGoFunction(func(ctx *Context, elements ...SymbolicValue) *Array {
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
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool) bool {
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
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool) bool {
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
				index := parse.FindNode(n.Statements[1], (*parse.IntLiteral)(nil), func(n *parse.IntLiteral, _ bool) bool {
					return n.Value == 0
				})

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
				makeSymbolicEvalError(stmt, state, fmtListShouldHaveLengthGreaterOrEqualTo(1)),
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
				makeSymbolicEvalError(stmt, state, fmtListShouldHaveLengthGreaterOrEqualTo(2)),
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
				AsSerializable(NewMultivalue(ANY_INT, Nil)).(Serializable),
				AsSerializable(NewMultivalue(ANY_INT, Nil)).(Serializable),
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
	})

	t.Run("for statement", func(t *testing.T) {
		t.Run("iterated value is not iterable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				for i, e in int {
	
				} 
			`)
			res, err := symbolicEval(n, state)
			forStmt := n.Statements[0]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(forStmt, state, fmtXisNotIterable(ANY_INT)),
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
			assert.Equal(t, NewMultivalue(ANY_STR, Nil), res)
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
				ANY_INT, AsSerializable(NewMultivalue(NewList(NewString("a")), NewList(ANY_INT))).(Serializable),
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
				assert.Equal(t, NewMultivalue(AsSerializable(NewMultivalue(NewPath("./a"), NewPath("./b"))), Nil), res)
			} else {
				assert.Equal(t, NewMultivalue(AsSerializable(NewMultivalue(NewPath("./b"), NewPath("./a"))), Nil), res)
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

			expectedResultFromForStmt := NewList(ANY_INT, ANY_INT)
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
			state.setGlobal("streamable", &AnyStreamSource{}, GlobalConst)

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
			state.setGlobal("streamable", &AnyStreamSource{}, GlobalConst)

			_, err := symbolicEval(n, state)
			keyVar := parse.FindNode(n, &parse.IdentifierLiteral{}, func(n *parse.IdentifierLiteral, isUnique bool) bool {
				return n.Name == "k"
			})
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

			expectedResultFromForStmt := NewTupleOf(ANY_STR)
			assert.Equal(t, NewMultivalue(expectedResultFromForStmt, Nil), res)
		})

		t.Run("error in head + missing body", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				for i, e in int
			`, nil)
			res, err := symbolicEval(n, state)
			forStmt := n.Statements[0]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(forStmt, state, fmtXisNotIterable(ANY_INT)),
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
	})

	t.Run("walk statement", func(t *testing.T) {
		t.Run("walked value is not walkable", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				path = int
				walk $path entry {
	
				}
			`)

			walkStmt := n.Statements[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(walkStmt, state, fmtXisNotWalkable(ANY_INT)),
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

			walkStmt := n.Statements[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(walkStmt, state, fmtXisNotWalkable(ANY_INT)),
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
					match v {
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

	})

	t.Run("match statement", func(t *testing.T) {
		t.Run("join", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var v %object = {}
				match 1 {
					%int {
						v = {a: 1}
					}
					%str {
						v = {b: 1}
					}
				}

				return v
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewMultivalue(
				NewEmptyObject(),
				NewInexactObject(map[string]Serializable{"a": NewInt(1)}, nil, map[string]Pattern{"a": ANY_INT.Static()}),
				NewInexactObject(map[string]Serializable{"b": NewInt(1)}, nil, map[string]Pattern{"b": ANY_INT.Static()}),
			), res)
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

		t.Run("narrowing of variable's value (no default case)", func(t *testing.T) {
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

		t.Run("narrowing of variable's value (with default case)", func(t *testing.T) {
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
		})

		t.Run("spread object pattern matching all objects", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{...%p}
			`)

			state.ctx.AddNamedPattern("p", &ObjectPattern{}, false)

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
		})

		t.Run("pattern call", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				return %{a: %int(0..1)}
			`)

			patt, _ := state.ctx.ResolveNamedPattern("int").Call(nil, []SymbolicValue{&IntRange{}})

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": patt},
				inexact: true,
			}, res)
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
				entries: map[string]Pattern{"a": ANY_PATTERN},
				inexact: true,
			}, res)
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

			state.ctx.AddNamedPattern("p", &RecordPattern{}, false)

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

			objectPatternLiteral := parse.FindNode(n, (*parse.ObjectPatternLiteral)(nil), func(lit *parse.ObjectPatternLiteral, _ bool) bool {
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

			patt, _ := state.ctx.ResolveNamedPattern("int").Call(nil, []SymbolicValue{&IntRange{}})

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

	})

	t.Run("list pattern literal", func(t *testing.T) {
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

	t.Run("tuple pattern literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
			%p = #[ #[] ]
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
				%p = #[ %{} ]
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
				%p = #[]%{}
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
	})

	t.Run("union pattern", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			return %| 1 | "1"
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())
		assert.Equal(t, &UnionPattern{
			Cases: []Pattern{
				&ExactValuePattern{value: &Int{hasValue: true, value: 1}},
				&ExactValuePattern{value: NewString("1")},
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
			Cases: []Pattern{
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
		assert.Equal(t, NewSequenceStringPattern(complexStringPatternPiece), res)
	})

	t.Run("pattern definition", func(t *testing.T) {

		t.Run("object pattern literal", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				%p = %{list: %[1]}
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
				%p = {list: [1]}
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

		t.Run("in preinit block", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				preinit {
					%p = %{}
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
				%namespace. = {patt: %str}
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
				%namespace. = {patt: #a}
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
				%namespace. = int
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
				%p = nil
				return %p?
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[1].(*parse.ReturnStatement).Expr, state, CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL),
			}, state.errors())
			assert.Equal(t, &AnyPattern{}, res)
		})
	})

	t.Run("host alias definition: RHS is not a host", func(t *testing.T) {
		n, state := MakeTestStateAndChunk(`
			@h = int
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Equal(t, []SymbolicEvaluationError{
			makeSymbolicEvalError(n.Statements[0], state, fmtCannotCreateHostAliasWithA(ANY_INT)),
		}, state.errors())
		assert.Nil(t, res)
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
				generalElement: &Object{},
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
				fn: func(*Context, *Int) {
				},
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

			goFunc := &GoFunction{
				fn: func(*Context, *Int) {
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
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
			assert.Equal(t, &TestSuite{}, res)
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
			assert.Equal(t, &TestSuite{}, res)
		})
	})

	t.Run("testcase expression", func(t *testing.T) {
		t.Run("empty module", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`testcase "name" {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &TestCase{}, res)
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
			assert.Equal(t, &TestCase{}, res)
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
				makeSymbolicEvalError(membExpr, state, fmtPropOfSymbolicDoesNotExist("a", NewEmptyObject(), "")),
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
				%p = int
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
				%p = int
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
		t.Run("single expression", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){ }
				return go {globals: .{}} do f()
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, &Routine{}, res)
		})

		t.Run("single expression without meta", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){ }
				return go do f()
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, &Routine{}, res)
		})

		t.Run("provided group is not a routine group", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				group = int
				return go {group: group, globals: .{}} do { }
			`)

			res, err := symbolicEval(n, state)
			obj := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(obj, state, fmtGroupPropertyNotRoutineGroup(ANY_INT)),
			}, state.errors())
			assert.IsType(t, &Routine{}, res)
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
			assert.IsType(t, &Routine{}, res)
		})

		t.Run("call provided function", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				fn f(){
					return 2
				}
				rt = go {globals: {f: f}} do {
					return f() # f is external for the embedded module
				}
				return rt.wait_result!()
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
				makeSymbolicEvalWarning(metadataNode, state, fmtUnknownSectionInCoroutineMetadata("x")),
			) //we use contains because there is also a warning about a missing permission
		})

		t.Run("allow section", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				rt = go {allow: {}} do {
					return int
				}
				return rt.wait_result!()
			`)

			metadataNode := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			assert.NotContains(t,
				state.warnings(),
				makeSymbolicEvalWarning(metadataNode, state, fmtUnknownSectionInCoroutineMetadata("allow")),
			) //we use contains because there is also a warning about a missing permission
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
		assert.Equal(t, NewInexactObject(map[string]Serializable{
			"0": ANY_SYNC_MSG_HANDLER,
		}, nil, map[string]Pattern{"0": getStatic(ANY_SYNC_MSG_HANDLER)}), res)

	})

	t.Run("sendvalue expression", func(t *testing.T) {
		t.Run("in method", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`{
				f: fn(){ 
					sendval int to supersys
				}
			}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.IsType(t, &Object{}, res)
		})
	})

	t.Run("mapping expression", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { 0 => 1  1 => comp 0 }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Mapping{}, res)
		})

		t.Run("key variable & group matching variable should be accessible in right side", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { p %/{:name} m => [p, m] }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Mapping{}, res)
		})

		t.Run("key variable should be accessible in right side and have right type", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { n int => (n - int) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Mapping{}, res)
		})

		t.Run("key variable should be accessible in right side and have right type: case pattern key", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				Mapping { n %int => (n - int) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Mapping{}, res)
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

			assert.Equal(t, &Mapping{}, res)
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
			assert.Equal(t, ANY_STR_CONCAT, res)
		})

		t.Run("first element is a multivalue implementing string like", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var elem %str = "a"
				if g {
					elem = concat elem "b"
				}
				# at this point elem is a %string | %string-concatenation
				return [elem, concat elem "x"]
			`)

			state.setGlobal("g", ANY_BOOL, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(
				//we also check that elem has the right because the test case depends on that
				AsSerializable(NewMultivalue(NewString("a"), ANY_STR_CONCAT)).(Serializable),
				ANY_STR_CONCAT,
			), res)
		})

		t.Run("second element is a multivalue implementing string like", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var elem %str = "a"
				if g {
					elem = concat elem "b"
				}
				# at this point elem is a %string | %string-concatenation
				return [elem, concat "x" elem]
			`)

			state.setGlobal("g", ANY_BOOL, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(
				//we also check that elem has the right because the test case depends on that
				AsSerializable(NewMultivalue(NewString("a"), ANY_STR_CONCAT)).(Serializable),
				ANY_STR_CONCAT,
			), res)
		})

		t.Run("single byteslice element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat 0d[12]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &ByteSlice{}, res)
		})

		t.Run("two bytes-like elements", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`concat 0d[12] 0d[34]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_BYTES_CONCAT, res)
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
			state.ctx.AddNamedPattern("str_tuple", &TypePattern{val: NewTupleOf(ANY_STR)}, false)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			expectedFn := &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{NewTupleOf(ANY_INT), NewTupleOf(&String{})},
				parameterNames: []string{"a", "b"},
				result:         NewTupleOf(AsSerializable(NewMultivalue(ANY_INT, ANY_STR)).(Serializable)),
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
				parameters:     []SymbolicValue{NewTupleOf(ANY_INT), NewTupleOf(ANY_INT)},
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
			assert.Equal(t, &StringConcatenation{}, res)
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
			assert.Equal(t, &StringConcatenation{}, res)
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
			assert.Equal(t, &StringConcatenation{}, res)
		})
	})

	t.Run("string template literal", func(t *testing.T) {

		replace := func(s string) string {
			return strings.ReplaceAll(s, "|", "`")
		}

		t.Run("no interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				%digit = %str('0'..'9')
				return %digit|3|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &CheckedString{}, res)
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
				%sql = %str( %|.*| )
				unsanitized_id = "5"
				return %sql|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit, state, STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX),
			}, state.errors())
			assert.Equal(t, &CheckedString{}, res)
		})

		t.Run("interpolation pattern does not exist", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				%sql. = {stmt: %str( %|.*| )}
				unsanitized_id = "5"
				return %sql.stmt|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr.(*parse.StringTemplateLiteral)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit.Slices[1], state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist("int", "sql")),
			}, state.errors())
			assert.Equal(t, &CheckedString{}, res)
		})

		t.Run("interpolation expression is not a string", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				%sql. = {
					stmt: %str( %|.*| ),
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = {}
				return %sql.stmt|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr.(*parse.StringTemplateLiteral)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit.Slices[1], state, fmtInterpolationIsNotStringlikeOrIntBut(&Object{entries: map[string]Serializable{}})),
			}, state.errors())
			assert.Equal(t, &CheckedString{}, res)
		})

		t.Run("no pattern, leading interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				s = "1"
				return |{{s}}2|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_STR, res)
		})

		t.Run("no pattern, trailing interpolation", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(replace(`
				s = "2"
				return |int{{s}}|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, ANY_STR, res)
		})

		t.Run("no pattern, multivalue interpolation implementing string like", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`
				var elem %str = "a"
				if g {
					elem = concat elem "b"
				}
				# at this point elem is a %string | %string-concatenation
				return [elem,` + "`x{{elem}}`" + `]
			`)

			state.setGlobal("g", ANY_BOOL, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, NewList(
				//we also check that elem has the right because the test case depends on that
				AsSerializable(NewMultivalue(NewString("a"), ANY_STR_CONCAT)).(Serializable),
				ANY_STR,
			), res)
		})
	})

	t.Run("URL expressions", func(t *testing.T) {

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
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{}), GlobalConst)
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
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{
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
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{
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
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *Identifier {
					return &Identifier{name: elem.name}
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &Identifier{name: "div"}, res)
		})

		t.Run("self-closing element", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`html<div/>`)
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{
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
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *XMLElement {
					return elem
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &XMLElement{
				name:     "div",
				children: []SymbolicValue{ANY_STR, ANY_INT, ANY_STR},
			}, res)
		})

		t.Run("attribute with value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`a = "a"; return html<div a=a></div>`)
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *XMLElement {
					return elem
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &XMLElement{
				name:       "div",
				attributes: map[string]SymbolicValue{"a": NewString("a")},
				children:   []SymbolicValue{ANY_STR},
			}, res)
		})

		t.Run("attribute without value", func(t *testing.T) {
			n, state := MakeTestStateAndChunk(`a = "a"; return html<div a></div>`)
			state.setGlobal("html", NewNamespace(map[string]SymbolicValue{
				FROM_XML_FACTORY_NAME: WrapGoFunction(func(ctx *Context, elem *XMLElement) *XMLElement {
					return elem
				}),
			}), GlobalConst)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors())
			assert.Equal(t, &XMLElement{
				name:       "div",
				attributes: map[string]SymbolicValue{"a": ANY_STR},
				children:   []SymbolicValue{ANY_STR},
			}, res)
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
		`, "args", extData.MOD_ARGS_VARNAME))

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors())

		if !assert.IsType(t, (*Struct)(nil), res) {
			return
		}

		structVal := res.(*Struct)
		structType := structVal.structType

		assert.Equal(t, NewStructPattern("", structType.tempId, []string{"a", "b", "c"}, []Pattern{
			state.ctx.ResolveNamedPattern("bool"),
			state.ctx.ResolveNamedPattern("str"),
			state.ctx.ResolveNamedPattern("int"),
		}), structType)

		assert.Equal(t, NewStruct(structType, map[string]SymbolicValue{
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
			state.baseGlobals = map[string]SymbolicValue{
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
					ParsedChunk: utils.Must(parse.ParseChunkSource(parse.SourceFile{
						NameString:  "/included.ix",
						Resource:    "/included.ix",
						ResourceDir: "/",
						CodeString:  "includable-chunk\n(1+\"a\")",
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
				Span:        parse.NodeSpan{Start: 20, End: 23},
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

	t.Run("the evaluation should stop if the context context is done AND there is no remaining no-check fuel", func(t *testing.T) {
		nodeCount := parse.CountNodes(parse.MustParseChunk("[]"))

		n, state := MakeTestStateAndChunk("[" + strings.Repeat("1,", INITIAL_NO_CHECK_FUEL-nodeCount+1) + "]")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		state.ctx.startingConcreteContext = testConcreteContext{ctx}
		_, err := symbolicEval(n, state)
		assert.ErrorContains(t, err, "stopped symbolic evaluation because context is done")
	})
	t.Run("the evaluation should not stop if the context context is done but there is remaining no-check fuel", func(t *testing.T) {
		nodeCount := parse.CountNodes(parse.MustParseChunk("[]"))

		n, state := MakeTestStateAndChunk("[" + strings.Repeat("1,", INITIAL_NO_CHECK_FUEL-nodeCount) + "]")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		state.ctx.startingConcreteContext = testConcreteContext{ctx}
		_, err := symbolicEval(n, state)
		assert.NoError(t, err)
	})
}
