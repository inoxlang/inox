package internal

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicEval(t *testing.T) {

	_makeStateAndChunk := func(code string) (*parse.Chunk, *State, error) {
		chunk, err := parse.ParseChunkSource(parse.InMemorySource{
			NameString: "",
			CodeString: code,
		})

		state := newSymbolicState(NewSymbolicContext(), chunk)
		state.ctx.AddNamedPattern("int", &TypePattern{
			val: &Int{},
			call: func(ctx *Context, values []SymbolicValue) (Pattern, error) {
				if len(values) == 0 {
					return nil, errors.New("missing argument")
				}
				return &IntRangePattern{}, nil
			},
		})
		state.ctx.AddNamedPattern("str", &TypePattern{val: &String{}})
		state.ctx.AddNamedPattern("obj", &TypePattern{val: NewAnyObject()})
		state.ctx.AddNamedPattern("list", &TypePattern{val: NewListOf(ANY)})
		state.ctx.AddPatternNamespace("myns", NewPatternNamespace(map[string]Pattern{
			"int": state.ctx.ResolveNamedPattern("int"),
		}))
		state.Module = &Module{
			MainChunk: chunk,
		}

		return chunk.Node, state, err
	}

	makeStateAndChunk := func(code string) (*parse.Chunk, *State) {
		node, state, err := _makeStateAndChunk(code)
		if err != nil {
			panic(err)
		}
		return node, state
	}

	t.Run("quoted string literal", func(t *testing.T) {
		n, state := makeStateAndChunk(`""`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &String{}, res)
	})

	t.Run("multiline string literal", func(t *testing.T) {
		n, state := makeStateAndChunk("``")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &String{}, res)
	})

	t.Run("byte slice literal", func(t *testing.T) {
		n, state := makeStateAndChunk("0x[01]")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &ByteSlice{}, res)
	})

	t.Run("integer literal", func(t *testing.T) {
		n, state := makeStateAndChunk("1")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &Int{}, res)
	})

	t.Run("date literal", func(t *testing.T) {
		n, state := makeStateAndChunk("2020y-UTC")
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &Date{}, res)
	})

	t.Run("list literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, state := makeStateAndChunk("[]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &List{elements: []SymbolicValue{}}, res)
		})

		t.Run("singe element", func(t *testing.T) {
			n, state := makeStateAndChunk("[1]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(&Int{}), res)
		})

		t.Run("two elements of different type", func(t *testing.T) {
			n, state := makeStateAndChunk(`[1, "a'"]`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(ANY_INT, ANY_STR), res)
		})

		t.Run("type annotation and element of invalid type", func(t *testing.T) {
			n, state := makeStateAndChunk("[]%int[true]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInListAnnotated(ANY_BOOL, state.ctx.ResolveNamedPattern("int"))),
			}, state.errors)
			assert.Equal(t, NewListOf(ANY_INT), res)
		})

		t.Run("type annotation and element of valid type", func(t *testing.T) {
			n, state := makeStateAndChunk("[]%int[1]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewListOf(ANY_INT), res)
		})
	})

	t.Run("tuple literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, state := makeStateAndChunk("#[]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Tuple{generalElement: ANY}, res)
		})

		t.Run("singe element", func(t *testing.T) {
			n, state := makeStateAndChunk("#[1]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewTuple(&Int{}), res)
		})

		t.Run("mutable element", func(t *testing.T) {
			n, state := makeStateAndChunk("#[{}]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			elemNode := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(elemNode, state, ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE),
			}, state.errors)
			assert.Equal(t, NewTuple(ANY), res)
		})

		t.Run("type annotation and element of invalid type", func(t *testing.T) {
			n, state := makeStateAndChunk("#[]%int[true]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			elemNode := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(elemNode, state, fmtUnexpectedElemInTupleAnnotated(ANY_BOOL, state.ctx.ResolveNamedPattern("int"))),
			}, state.errors)
			assert.Equal(t, NewTupleOf(ANY_INT), res)
		})

		t.Run("type annotation and element of valid type", func(t *testing.T) {
			n, state := makeStateAndChunk("#[]%int[1]")
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewTupleOf(ANY_INT), res)
		})

	})

	t.Run("local variable declaration", func(t *testing.T) {
		t.Run("no type annotation", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				var a = 1; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("value not assignable to type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				var a %str = 1; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(&Int{}, &TypePattern{val: &String{}})),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("object (ability to hold static data) is not assignable to type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				var a %str = {}; 
				return a
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			decl := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(decl.Right, state,
					fmtNotAssignableToVarOftype(NewEmptyObject(), &TypePattern{val: &String{}}),
				),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("value assignable to type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				var obj %{name: %| %str | %int} = {name: 1}; 
				return obj
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)

			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &Int{},
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
			n, state := makeStateAndChunk(`
				return fn(v %| %[]%int | %[]%str){
					var a %list = v; 
					return a
				}
			`)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)

			argType := NewMultivalue(
				NewListOf(&Int{}), NewListOf(&String{}),
			)

			expectedFn := &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{argType},
				parameterNames: []string{"v"},
				returnType:     argType,
			}
			assert.Equal(t, expectedFn, res)
		})
	})

	t.Run("global variable defintion", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			$$v = []
			return $$v
		`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &List{elements: []SymbolicValue{}}, res)
	})

	t.Run("variable assignment", func(t *testing.T) {

		t.Run("local variable", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = []
				return $v
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &List{elements: []SymbolicValue{}}, res)
		})

		t.Run("RHS has incompatible type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = []
				v = 3
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)

			type_ := &TypePattern{val: &List{generalElement: ANY}}
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(&Int{}, type_)),
			}, state.errors)
			assert.Equal(t, &List{elements: []SymbolicValue{}}, res)
		})

		t.Run("RHS has type incompatible with static type of the variable", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%p = %| %int | %str
				var v %p = 1
				v = true
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[2]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(&Bool{}, &UnionPattern{
					Cases: []Pattern{
						state.ctx.ResolveNamedPattern("int"),
						state.ctx.ResolveNamedPattern("str"),
					},
				})),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("+= assignment, LHS has incompatible type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = "a"
				v += 1
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT),
			}, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("+= assignment, RHS has incompatible type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = 1
				v += "a"
				return v
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1].(*parse.Assignment)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement.Right, state, INVALID_INT_OPER_ASSIGN_RHS_NOT_INT),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("variable LHS in function: a local variable outside of the function already has the same name", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				a = ""
				fn f(){
					a = 3
					return a
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("multi value RHS", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(v %| %[]%int | %[]%str){
					list = []
					list = v
				}
			`)
			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
		})
	})

	t.Run("property assignement", func(t *testing.T) {
		t.Run("set new property of an object", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				obj = {}
				$obj.name = "foo"
				return obj
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
			}, res)
		})

		t.Run("existing property: RHS has incompatible type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				obj = {name: "foo"}
				$obj.name = 1
				return obj
			`)
			assignment := n.Statements[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(&Int{}, &TypePattern{val: &String{}})),
			}, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
			}, res)
		})

		t.Run("existing property: RHS has type compatible with static type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				var obj %{name: %| %str | %int } = {name: "foo"}
				$obj.name = 1
				return obj
			`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &Int{},
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

		t.Run("existing property: RHS has type incompatible with static type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
				makeSymbolicEvalError(assignment, state, fmtNotAssignableToPropOfType(&Bool{}, propType)),
			}, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
				static: map[string]Pattern{
					"name": propType,
				},
			}, res)
		})

		t.Run("+= assignment, LHS has incompatible type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				obj = {name: "foo"}
				$obj.name += 1
				return obj
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement, state, INVALID_INT_OPER_ASSIGN_LHS_NOT_INT),
			}, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
			}, res)
		})

		t.Run("+= assignment, RHS has incompatible type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				obj = {count: 1}
				$obj.count += "a"
				return obj
			`)
			res, err := symbolicEval(n, state)
			assignement := n.Statements[1].(*parse.Assignment)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(assignement.Right, state, INVALID_INT_OPER_ASSIGN_RHS_NOT_INT),
			}, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"count": &Int{},
				},
			}, res)
		})

		t.Run("object's property LHS (identifier member): new property", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				obj = {}
				obj.name = "foo"
				return obj
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
			}, res)
		})

	})

	t.Run("object literal", func(t *testing.T) {
		t.Run("basic", func(t *testing.T) {

			n, state := makeStateAndChunk(`{"name": "foo"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
			}, res)
		})

		t.Run("type annotation", func(t *testing.T) {
			n, state := makeStateAndChunk(`{"name" %| %str | %int : "foo"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
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
			n, state := makeStateAndChunk(`{"name" %str : 1}`)
			res, err := symbolicEval(n, state)

			valueNode := parse.FindNode(state.Module.MainChunk.Node, (*parse.IntLiteral)(nil), nil)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(valueNode, state, fmtNotAssignableToPropOfType(&Int{}, state.ctx.ResolveNamedPattern("str"))),
			}, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
				static: map[string]Pattern{
					"name": state.ctx.ResolveNamedPattern("str"),
				},
			}, res)
		})

		t.Run("object in object", func(t *testing.T) {

			n, state := makeStateAndChunk(`{v: {}}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"v": &Object{
						entries: map[string]SymbolicValue{},
					},
				},
			}, res)
		})

		t.Run("_constraints_", func(t *testing.T) {
			n, state := makeStateAndChunk(`{
				a: 1
				b: 2

				_constraints_ {
					(self.a < self.b)
				}
			}`)
			res, err := symbolicEval(n, state)

			binExpr := parse.FindNode(state.Module.MainChunk.Node, (*parse.BinaryExpression)(nil), nil)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"a": &Int{},
					"b": &Int{},
				},
				complexPropertyConstraints: []*ComplexPropertyConstraint{
					{
						Properties: []string{"a", "b"},
						Expr:       binExpr,
					},
				},
			}, res)
		})
	})

	t.Run("record", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := makeStateAndChunk(`#{"name": "foo"}`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Record{
				entries: map[string]SymbolicValue{
					"name": &String{},
				},
			}, res)
		})

		t.Run("mutable value", func(t *testing.T) {
			n, state := makeStateAndChunk(`#{"a": {}}`)
			res, err := symbolicEval(n, state)
			valueNode := n.Statements[0].(*parse.RecordLiteral).Properties[0].Value

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(valueNode, state, fmtValuesOfRecordShouldBeImmutablePropHasMutable("a")),
			}, state.errors)
			assert.Equal(t, &Record{
				entries: map[string]SymbolicValue{
					"a": ANY,
				},
			}, res)
		})

	})
	t.Run("member expression", func(t *testing.T) {
		t.Run("object", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = {"name": "foo"}
				return $v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("record", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = #{"name": "foo"}
				return $v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("inexisting field of GoValue", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return v.XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("existing method of GoValue", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return v.cancel
			`)

			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.NotNil(t, res)
		})

		t.Run("inexisting method of GoValue", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return v.XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("multivalue: 2 objects with same property type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return fn(v %| %{a: %int} | %{a: %int, b: %str}) {
					return v.a
				}
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res.(*InoxFunction).returnType)
		})

		t.Run("multivalue: 2 objects with different property type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return fn(v %| %{a: %int} | %{a: %str}) {
					return v.a
				}
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewMultivalue(&Int{}, &String{}), res.(*InoxFunction).returnType)
		})

		t.Run("unterminated", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = {"name": "foo"}
				return $v.
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, ANY, res)
		})

	})

	t.Run("dynamic member expression", func(t *testing.T) {
		t.Run("object", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = {"name": "foo"}
				return $v.<name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewDynamicValue(&String{}), res)
		})

		t.Run("record", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = #{"name": "foo"}
				return $v.<name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewDynamicValue(&String{}), res)
		})

		t.Run("dynamic value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = {a: {b: 1}}
				return $v.<a.b
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewDynamicValue(&Int{}), res)
		})

		t.Run("inexisting field of GoValue", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return v.<XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
			}, state.errors)
			assert.Equal(t, NewAnyDynamicValue(), res)
		})

		t.Run("existing method of GoValue", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return v.<cancel
			`)

			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.NotNil(t, res)
		})

		t.Run("inexisting method of GoValue", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return v.<XYZ
			`)
			memberExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			goVal := &Routine{}
			state.setGlobal("v", goVal, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(memberExpr, state, fmtPropOfSymbolicDoesNotExist("XYZ", goVal, "")),
			}, state.errors)
			assert.Equal(t, NewAnyDynamicValue(), res)
		})
	})

	t.Run("identifier member expression", func(t *testing.T) {
		t.Run("object", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = {"name": "foo"}
				return v.name
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("unterminated (0 property names)", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = {"name": "foo"}
				return v.
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("unterminated (1 property names)", func(t *testing.T) {
			n, state, _ := _makeStateAndChunk(`
				v = {"a": {"b": "foo"}}
				return v.a.
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, ANY, res)
		})
	})

	t.Run("index expression", func(t *testing.T) {
		t.Run("index is not an integer", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = ["a"]
				return $v["0"]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtIndexIsNotAnIntButA(&String{})),
			}, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("indexed is not indexable", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = 0
				return $v[0]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtXisNotIndexable(&Int{})),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("ok", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = ["a"]
				return $v[0]
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("list of unknown length", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return $$v[0]
			`)
			state.setGlobal("v", &List{generalElement: &String{}}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("rune slice", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return $$v[0]
			`)
			state.setGlobal("v", &RuneSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Rune{}, res)
		})

		t.Run("byte slice", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return $$v[0]
			`)
			state.setGlobal("v", &ByteSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Byte{}, res)
		})

	})

	t.Run("slice expression", func(t *testing.T) {
		t.Run("start index is not an integer", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = ["a"]
				return $v["0":]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtStartIndexIsNotAnIntButA(&String{})),
			}, state.errors)
			assert.Equal(t, NewListOf(ANY), res)
		})

		t.Run("end index is not an integer", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = ["a"]
				return $v[0:"1"]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtEndIndexIsNotAnIntButA(&String{})),
			}, state.errors)
			assert.Equal(t, NewListOf(ANY), res)
		})

		t.Run("indexed it not a sequence", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = 0
				return $v[0:]
			`)
			indexExpr := n.Statements[1].(*parse.ReturnStatement).Expr
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(indexExpr, state, fmtSequenceExpectedButIs(&Int{})),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("ok", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				v = ["a"]
				return $v[0:]
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewListOf(ANY), res)
		})

		t.Run("list of unknown length", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return $$v[0:]
			`)
			state.setGlobal("v", &List{generalElement: &String{}}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &List{generalElement: &String{}}, res)
		})

		t.Run("rune slice", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return $$v[0:]
			`)
			state.setGlobal("v", &RuneSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &RuneSlice{}, res)
		})

		t.Run("byte slice", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return $$v[0:]
			`)
			state.setGlobal("v", &ByteSlice{}, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &ByteSlice{}, res)
		})

	})

	t.Run("extraction expression", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			v = {a: 1, b: true, c: "a"}
			return $v.{a, b}
		`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &Object{
			entries: map[string]SymbolicValue{
				"a": &Int{},
				"b": &Bool{},
			},
		}, res)
	})

	t.Run("binary expression", func(t *testing.T) {
		t.Run("+: left operand is a string", func(t *testing.T) {
			n, state := makeStateAndChunk(`("a" + 1)`)
			res, err := symbolicEval(n, state)

			leftOperand := n.Statements[0].(*parse.BinaryExpression).Left

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(leftOperand, state, fmtLeftOperandOfBinaryShouldBe(parse.Add, "int or float", "%string")),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("<: left operand is a string", func(t *testing.T) {
			n, state := makeStateAndChunk(`("a" < 1)`)
			res, err := symbolicEval(n, state)

			leftOperand := n.Statements[0].(*parse.BinaryExpression).Left

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(leftOperand, state, fmtLeftOperandOfBinaryShouldBe(parse.LessThan, "int or float", "%string")),
			}, state.errors)
			assert.Equal(t, &Bool{}, res)
		})

		t.Run("+: right operand is a string", func(t *testing.T) {
			n, state := makeStateAndChunk(`(1 + "a")`)
			res, err := symbolicEval(n, state)

			rightOperand := n.Statements[0].(*parse.BinaryExpression).Right

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(rightOperand, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "%string")),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("<: Right operand is a string", func(t *testing.T) {
			n, state := makeStateAndChunk(`(1 < "a")`)
			res, err := symbolicEval(n, state)

			RightOperand := n.Statements[0].(*parse.BinaryExpression).Right

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(RightOperand, state, fmtRightOperandOfBinaryShouldBe(parse.LessThan, "int", "%string")),
			}, state.errors)
			assert.Equal(t, &Bool{}, res)
		})

		t.Run("substrof: left operand is an int", func(t *testing.T) {
			n, state := makeStateAndChunk(`(1 substrof "1")`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Left, state, fmtLeftOperandOfBinaryShouldBe(parse.Substrof, "string-like", "%int")),
			}, state.errors)
			assert.Equal(t, &Bool{}, res)
		})

		t.Run("substrof: right operand is an int", func(t *testing.T) {
			n, state := makeStateAndChunk(`("1" substrof 1)`)
			res, err := symbolicEval(n, state)

			expr := n.Statements[0].(*parse.BinaryExpression)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(expr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Substrof, "string-like", "%int")),
			}, state.errors)
			assert.Equal(t, &Bool{}, res)
		})

		t.Run("match: right operand is a path pattern", func(t *testing.T) {
			n, state := makeStateAndChunk(`(/home/user/ match %/home/user/...)`)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Bool{}, res)
		})

		t.Run("match: right operand is a regex pattern", func(t *testing.T) {
			n, state := makeStateAndChunk("(\"\" match %`.*`)")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Bool{}, res)
		})

		t.Run("match: right operand is an object pattern", func(t *testing.T) {
			n, state := makeStateAndChunk("({} match %{})")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Bool{}, res)
		})

		t.Run("set difference: right operand is a pattern", func(t *testing.T) {
			n, state := makeStateAndChunk("((%| 1 | 2 | 3) \\ %| 1 | 2)")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &DifferencePattern{
				Base:    &AnyPattern{},
				Removed: &AnyPattern{},
			}, res)
		})

		t.Run("set difference: right operand is an integer", func(t *testing.T) {
			n, state := makeStateAndChunk("((%| 1 | 2) \\ 1)")
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &DifferencePattern{
				Base:    &AnyPattern{},
				Removed: &AnyPattern{},
			}, res)
		})
	})

	t.Run("unary expression: !: operand is a string", func(t *testing.T) {
		n, state := makeStateAndChunk(`!"string"`)
		res, err := symbolicEval(n, state)

		assert.NoError(t, err)
		assert.Equal(t, []SymbolicEvaluationError{
			makeSymbolicEvalError(n, state, fmtOperandOfBoolNegateShouldBeBool(&String{})),
		}, state.errors)
		assert.Equal(t, &Bool{}, res)
	})

	t.Run("function declaration", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(){
	
				}
				return $$f
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{node: fnExpr, returnType: Nil}, res)
		})

		t.Run("single parameter", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(a){
					return a
				}
				return $$f
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{ANY},
				parameterNames: []string{"a"},
				returnType:     ANY,
			}, res)
		})

		t.Run("no params, single captured local", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				a = 1
				fn[a] f(){
					return a
				}
				return $$f
			`)
			fnExpr := n.Statements[1].(*parse.FunctionDeclaration).Function

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				returnType:     &Int{},
				capturedLocals: map[string]SymbolicValue{"a": &Int{}},
			}, res)
		})

		t.Run("no params, two captured locals", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				a = 1
				b = "1"
				fn[a, b] f(){
					return [a, b]
				}
				return $$f
			`)
			fnExpr := n.Statements[2].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{
				node:       fnExpr,
				returnType: NewList(&Int{}, &String{}),
				capturedLocals: map[string]SymbolicValue{
					"a": &Int{},
					"b": &String{},
				},
			}, res)
		})

		t.Run("missing return", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f() %int {
					
				}
			`)
			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnExpr, state, MISSING_RETURN_IN_FUNCTION),
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("invalid return value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f() %int {
					return "a"
				}
			`)

			returnStmt := parse.FindNode(n, (*parse.ReturnStatement)(nil), nil)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(returnStmt, state, fmtInvalidReturnValue(&String{}, &Int{})),
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("patterns should be accessible from the body", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%p = 1
				fn f(){
					[%p, %int]
				}
				return $$f
			`)
			fnExpr := n.Statements[1].(*parse.FunctionDeclaration).Function
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &InoxFunction{node: fnExpr, returnType: Nil}, res)
		})

	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("patterns should be accessible from the body of a function expression within a function declaration", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%p = 1
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
			n, state := makeStateAndChunk(`
				return %fn(){}
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				node:       fnPatt,
				returnType: Nil,
			}, res)
		})

		t.Run("single parameter", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %fn(a){}
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, &FunctionPattern{
				node:       fnPatt,
				returnType: Nil,
			}, res)
		})

		t.Run("missing return", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %fn() %int {
					
				}
			`)
			fnPatt := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.FunctionPatternExpression)
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnPatt, state, MISSING_RETURN_IN_FUNCTION_PATT),
			}, state.errors)
			assert.Equal(t, &FunctionPattern{
				node:       fnPatt,
				returnType: &Int{},
			}, res)
		})

		t.Run("invalid return value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %fn() %int {
					return "a"
				}
			`)

			fnPatt := parse.FindNode(n, (*parse.FunctionPatternExpression)(nil), nil)
			innerReturnStmt := parse.FindNodes(n, (*parse.ReturnStatement)(nil), nil)[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(innerReturnStmt, state, fmtInvalidReturnValue(&String{}, &Int{})),
			}, state.errors)
			assert.Equal(t, &FunctionPattern{
				node:       fnPatt,
				returnType: &Int{},
			}, res)
		})

	})

	t.Run("methods", func(t *testing.T) {
		t.Run("method returning a property", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				{
					f: fn() => self.a,
					a: 1,
				}
			`)

			fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"a": &Int{},
					"f": &InoxFunction{
						node:       fnExpr,
						returnType: &Int{},
					},
				},
			}, res)
		})

		t.Run("method returning a dynamic member", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				{
					f: fn() => self.<a,
					a: 1,
				}
			`)

			fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"a": &Int{},
					"f": &InoxFunction{
						node:       fnExpr,
						returnType: NewDynamicValue(ANY_INT),
					},
				},
			}, res)
		})

		t.Run("method calling another method : caller declared first", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				{
					a: 1,
					f: fn() => self.g,
					g: fn() => self.a,
				}
			`)

			fFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[0]
			gFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[1]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)

			obj := res.(*Object)
			g, _, _ := obj.GetProperty("g")

			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"a": &Int{},
					"f": &InoxFunction{
						node:       fFnExpr,
						returnType: g,
					},
					"g": &InoxFunction{
						node:       gFnExpr,
						returnType: &Int{},
					},
				},
			}, obj)
		})

		t.Run("method calling another method : callee declared first", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				{
					a: 1,
					g: fn() => self.a,
					f: fn() => self.g,
				}
			`)

			gFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[0]
			fFnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[1]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)

			obj := res.(*Object)
			g, _, _ := obj.GetProperty("g")

			assert.Equal(t, &Object{
				entries: map[string]SymbolicValue{
					"a": &Int{},
					"f": &InoxFunction{
						node:       fFnExpr,
						returnType: g,
					},
					"g": &InoxFunction{
						node:       gFnExpr,
						returnType: &Int{},
					},
				},
			}, obj)
		})

		t.Run("cycle of two methods", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			}, state.errors)
			assert.Equal(t, &Object{}, res)
		})
		t.Run("cycle of three methods", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			}, state.errors)
			assert.Equal(t, &Object{}, res)
		})
	})

	t.Run("call Inox function", func(t *testing.T) {
		t.Run("empty function (identifier)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(){
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &NilT{}, res)
		})

		t.Run("empty function (member)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				o = {
					f: fn(){}
				}
	
				return $o.f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &NilT{}, res)
		})

		t.Run("empty function (identifier member)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				o = {
					f: fn(){}
				}
	
				return o.f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &NilT{}, res)
		})

		t.Run("function always return an integer", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(){
					return 1
				}
	
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("local variable defined outside of a function is not accessible from inside the function", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("function return its first argument: type of result should be the type of the arg", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(x){
					return x
				}
	
				return f(1)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("function returning its variadic argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(...rest){
					return $rest
				}
	
				return f(1)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(&Int{}), res)
		})

		t.Run("no variadic parameter: spread argument (known length)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			}, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("no variadic parameter: spread argument (unknown length)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(a){
					return $a
				}
	
				return f(...list)
			`)

			state.setGlobal("list", &List{generalElement: ANY}, GlobalConst)

			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("single, variadic parameter: spread argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(...rest){
					return $rest
				}
	
				list = ["2", true]
				return f(1, ...list)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(&Int{}, &String{}, &Bool{}), res)
		})

		t.Run("non variadic parameter + variadic parameter: spread argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(first, ...rest){
					return [first, $rest]
				}
	
				list = ["2", true]
				return f(1, ...list)
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(
				&Int{},
				NewList(&String{}, &Bool{}),
			), res)
		})

		t.Run("function declaration + call: %int return type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f() %int {
					return 1
				}
				return f()
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("function declaration + call: invalid return value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f() %int {
					return "a"
				}
				return f()
			`)

			fnReturnStmt := parse.FindNodes(n, (*parse.ReturnStatement)(nil), nil)[0]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(fnReturnStmt, state, fmtInvalidReturnValue(&String{}, &Int{})),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("invalid argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(x %int){
					return 1
				}
	
				return f("a")
			`)

			call := n.Statements[1].(*parse.ReturnStatement).Expr.(*parse.CallExpression)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call.Arguments[0], state, FmtInvalidArg(0, &String{}, &Int{})),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("valid argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(x %int){
					return 1
				}
	
				return f(0)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("multi value argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return fn(list %| %[]%int | %[]%str){
					f = fn(list %list){
						
					}
					return f(list)
				}
			`)

			fnExpr := parse.FindNodes(n, &parse.FunctionExpression{}, nil)[0]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{NewMultivalue(NewListOf(&Int{}), NewListOf(&String{}))},
				parameterNames: []string{"list"},
				returnType:     Nil,
			}, res)
		})

		t.Run("non-variadic function: not enough arguments", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(a, b %int, c){
					return [a, b, c]
				}
	
				return f(1)
			`)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(1, 3)),
			}, state.errors)

			assert.Equal(t, NewList(&Int{}, &Int{}, ANY), res)
		})

		t.Run("non-variadic function: too many arguments", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(a){
					return a
				}
	
				return f(1, 2)
			`)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(2, 1)),
			}, state.errors)

			assert.Equal(t, &Int{}, res)
		})

		t.Run("variadic function fn(a, ...rest): not enough arguments", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(a, ...rest){
					return [a, rest]
				}
	
				return f()
			`)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(0, 1)),
			}, state.errors)

			assert.Equal(t, NewList(ANY, NewList()), res)
		})

		t.Run("variadic function fn(a, b, ...rest): not enough arguments", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(a, b, ...rest){
					return [a, b, rest]
				}
	
				return f(1)
			`)
			call := parse.FindNode(n, (*parse.CallExpression)(nil), nil)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(1, 2)),
			}, state.errors)

			assert.Equal(t, NewList(&Int{}, ANY, NewList()), res)
		})

		t.Run("direct recursion of a function with no return type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn factorial(i %int) {
					if (i == 0) {
						return 1
					}
					return (i * factorial( (i - 1) ))
				}
				return factorial(3)
			`)
			call := parse.FindNodes(n, (*parse.CallExpression)(nil), nil)[0]

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, FUNCS_CALLED_RECU_SHOULD_HAVE_RET_TYPE),
				makeSymbolicEvalError(call, state, fmtRightOperandOfBinaryShouldBe(parse.Mul, "int", "%any")),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("direct recursion of a function with a return type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn factorial(i %int) %int {
					if (i == 0) {
						return 1
					}
					return (i * factorial( (i - 1) ))
				}
				return factorial(3)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

	})

	t.Run("call Go function", func(t *testing.T) {
		t.Run("call Go function: signature is func(*Context) int", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) *Int {
					return &Int{}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call Go function: signature is func(*Context) *List", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			assert.Empty(t, state.errors)
			assert.Equal(t, &List{generalElement: ANY}, res)
		})

		t.Run("call Go function: signature is func(*Context) (int, int)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) (*Int, *Int) {
					return &Int{}, &Int{}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(&Int{}, &Int{}), res)
		})

		t.Run("call Go function: signature is func(*Context, *Int) *Int: missing argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f()
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, *Int) *Int {
					return &Int{}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(0, 1)),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call Go function: signature is func(*Context, *Int) *Int: bad argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f("a")
			`)

			argNode := n.Statements[0].(*parse.ReturnStatement).Expr.(*parse.CallExpression).Arguments[0]

			goFunc := &GoFunction{
				fn: func(*Context, *Int) *Int {
					return &Int{}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(argNode, state, FmtInvalidArg(0, &String{}, &Int{})),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call Go function: signature is func(*Context, *Int) *Int: too many arguments", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f(1, 2)
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, *Int) *Int {
					return &Int{}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfArgs(2, 1)),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call Go function: signature is func(*Context, *List) *Int: pass multivalue of 2 lists", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return fn(list %| %[]%str | %[]%int){
					return f(list)
				}
			`)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(ctx *Context, list *List) *List {
					return list
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{NewMultivalue(NewListOf(&String{}), NewListOf(&Int{}))},
				parameterNames: []string{"list"},
				returnType:     NewListOf(ANY),
			}, res)
		})

		t.Run("call Go function: signature is func(*Context, ...*List) *Int: pass multivalue of 2 lists", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return fn(list %| %[]%str | %[]%int){
					return f(list)
				}
			`)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(ctx *Context, lists ...*List) *List {
					return lists[0]
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{NewMultivalue(NewListOf(&String{}), NewListOf(&Int{}))},
				parameterNames: []string{"list"},
				returnType:     NewListOf(ANY),
			}, res)
		})

		t.Run("call Go function: signature is func(*Context, ...*Int) *Int: bad first variadic argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f("a")
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, ...*Int) *Int {
					return &Int{}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, FmtInvalidArg(0, &String{}, &Int{})),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call Go function: signature is func(*Context, *Int, ...*Int) *Int: missing argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f()
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, *Int, ...*Int) *Int {
					return &Int{}
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(0, 1)),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call Go function: signature is func(*Context, ...*Int) *Int: bad second variadic argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f(1, "a")
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			goFunc := &GoFunction{
				fn: func(*Context, ...*Int) *Int {
					return &Int{}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, FmtInvalidArg(1, &String{}, &Int{})),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call Go function: no concrete value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f()
			`)

			state.setGlobal("f", &GoFunction{}, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0].(*parse.ReturnStatement).Expr, state, CANNOT_CALL_GO_FUNC_NO_CONCRETE_VALUE),
			}, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("call variadic Go function: spread argument (unknown length), missing non variadic argument", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f(...list)
			`)

			call := n.Statements[0].(*parse.ReturnStatement).Expr

			state.setGlobal("list", &List{generalElement: ANY}, GlobalConst)
			goFunc := &GoFunction{
				fn: func(*Context, *Any, ...*Int) *Int {
					return &Int{}
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(call, state, fmtInvalidNumberOfNonSpreadArgs(0, 1)),
			}, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("call variadic Go function: spread argument (unknown length)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f(...list)
			`)

			state.setGlobal("list", &List{generalElement: ANY}, GlobalConst)
			goFunc := &GoFunction{
				fn: func(*Context, ...*Int) *Int {
					return &Int{}
				},
			}
			state.setGlobal("f", goFunc, GlobalConst)

			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("'must' call Go function: error is not returned", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return f!()
			`)

			goFunc := &GoFunction{
				fn: func(*Context) (*Int, *Error) {
					//TODO: replace error with symbolic Nil error
					return &Int{}, &Error{data: ANY}
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})
	})

	t.Run("call abstract function", func(t *testing.T) {

		//TODO: add more tests
		t.Run("fn() %int", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(func %fn() %int){
					return func()
				}
				return f
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)

			fnExpr := n.Statements[0].(*parse.FunctionDeclaration).Function
			fnPatt := n.Statements[0].(*parse.FunctionDeclaration).Function.Parameters[0].Type

			expectedFn := &InoxFunction{
				node: fnExpr,
				parameters: []SymbolicValue{
					&Function{
						pattern: &FunctionPattern{
							node:       fnPatt.(*parse.FunctionPatternExpression),
							returnType: ANY_INT,
						},
					},
				},
				parameterNames: []string{"func"},
				returnType:     &Int{},
			}
			assert.Equal(t, expectedFn, res)
		})

	})
	t.Run("call pattern", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			%mypattern()
		`)

		state.ctx.AddNamedPattern("mypattern", &TypePattern{
			call: func(ctx *Context, values []SymbolicValue) (Pattern, error) {
				return &ExactValuePattern{value: &Int{}}, nil
			},
		})

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &ExactValuePattern{value: &Int{}}, res)
	})

	t.Run("pipe statement", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			fn one(){
				return 1
			}

			fn addOne(i %int){
				$$result = (i + 1)
			}

			one | addOne $
			return $$result
		`)
		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &Int{}, res)
	})

	t.Run("pipe statement: $ is an invalid argument in second call", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			fn one(){
				return "1"
			}

			fn addOne(i %int){
				$$result = (i + 1)
			}

			one | addOne $
			return $$result
		`)

		secondCall := parse.FindNodes(n, (*parse.CallExpression)(nil), nil)[1]

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Equal(t, []SymbolicEvaluationError{
			makeSymbolicEvalError(secondCall.Arguments[0], state, FmtInvalidArg(0, &String{}, &Int{})),
		}, state.errors)
		assert.Equal(t, &Int{}, res)
	})

	t.Run("if statement", func(t *testing.T) {

		t.Run("test is a boolean", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("test is not a boolean", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				if 1 {
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
				makeSymbolicEvalError(ifStmt, state, fmtIfStmtTestNotBoolBut(&Int{})),
				makeSymbolicEvalError(idents[0], state, fmtVarIsNotDeclared("a")),
				makeSymbolicEvalError(idents[1], state, fmtVarIsNotDeclared("b")),
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("join", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				var a %obj = {}
				if true {
					a = {a: 1}
				} else {
					a = {b: 1}
				}
				return a
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewMultivalue(
				NewEmptyObject(),
				NewObject(map[string]SymbolicValue{"a": ANY_INT}, nil),
				NewObject(map[string]SymbolicValue{"b": ANY_INT}, nil),
			), res)
		})

		//TODO: add test about joining that checks that the state in alternate is not already joined with the consequent's fork

		t.Run("truthiness narrowing", func(t *testing.T) {

			t.Run("parameter", func(t *testing.T) {
				n, state := makeStateAndChunk(`
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
				assert.Empty(t, state.errors)
			})

			t.Run("parameter field", func(t *testing.T) {
				n, state := makeStateAndChunk(`
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
				assert.Empty(t, state.errors)
			})

			t.Run("variable of static type %int? and nil value", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					var v %int? = nil

					if v? {
						var a %int = v
					} else {
						
					}
				`)

				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors)
			})

			t.Run("non existing variable (identifier)", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					if v? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors)
			})

			t.Run("non existing variable (local var)", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					if $v? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors)
			})

			t.Run("non existing variable (global var)", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					if $$v? {

					} else {
						
					}
				`)
				_, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors)
			})
		})
	})

	t.Run("if expression", func(t *testing.T) {

		t.Run("no else", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				(if true 1)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, ANY_INT, res)
		})

		t.Run("if-else", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				(if false 1 else false)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewMultivalue(ANY_INT, ANY_BOOL), res)
		})

		t.Run("truthiness narrowing", func(t *testing.T) {

			t.Run("parameter", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					return fn(arg %int?){
						return (if arg? arg else false)
					}
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors)
				assert.Equal(t, NewMultivalue(ANY_INT, ANY_BOOL), res.(*InoxFunction).returnType)
			})

			t.Run("parameter field", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					return fn(arg %{prop: %int?}){
						return (if arg.prop? arg.prop else false)
					}
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors)
				assert.Equal(t, NewMultivalue(ANY_INT, ANY_BOOL), res.(*InoxFunction).returnType)
			})

			t.Run("variable of static type %int? and nil value", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					var v %int? = nil

					return (if v? v else false)
				`)

				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.Empty(t, state.errors)
				assert.Equal(t, NewMultivalue(ANY_INT, ANY_BOOL), res)
			})

			t.Run("non existing variable (identifier)", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					return (if v? v else false)
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors)
				assert.Equal(t, ANY, res)
			})

			t.Run("non existing variable (local var)", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					return (if $v? $v else false)
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors)
				assert.Equal(t, ANY, res)
			})

			t.Run("non existing variable (global var)", func(t *testing.T) {
				n, state := makeStateAndChunk(`
					return (if $$v? $$v else false)
				`)
				res, err := symbolicEval(n, state)
				assert.NoError(t, err)
				assert.NotEmpty(t, state.errors)
				assert.Equal(t, ANY, res)
			})
		})
	})

	t.Run("assignment", func(t *testing.T) {
		// TODO
	})

	t.Run("multi assignment", func(t *testing.T) {
		// TODO
	})

	t.Run("assignment in if statement: variable LHS: RHS has incompatible type", func(t *testing.T) {
		n, state := makeStateAndChunk(`
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
			makeSymbolicEvalError(assignement, state, fmtNotAssignableToVarOftype(&Int{}, &TypePattern{val: &List{generalElement: ANY}})),
		}, state.errors)
		assert.Equal(t, &List{elements: []SymbolicValue{}}, res)
	})

	t.Run("for statement", func(t *testing.T) {
		t.Run("iterated value is not iterable", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for i, e in 1 {
	
				} 
			`)
			res, err := symbolicEval(n, state)
			forStmt := n.Statements[0]
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(forStmt, state, fmtXisNotIterable(&Int{})),
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("object iteration: keys are strings", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for k, v in {a: 1} {
					return k
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &String{}, res)
		})

		t.Run("list iteration: keys are integers and values have type of element", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for i, e in [["a"], [0]] {
					return [i, e]
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(
				&Int{}, NewMultivalue(NewList(&String{}), NewList(&Int{})),
			), res)
		})

		t.Run("empty dictionary iteration: keys should be any", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for k, v in :{} {
					return k
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("path dictionary iteration: keys should be paths", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for k, v in :{./a: 1, ./b: 2} {
					return k
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Path{}, res)
		})

		t.Run("int range iteration: keys and values are integers", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for i, e in 1..3 {
					return [i, e]
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(&Int{}, &Int{}), res)
		})

		t.Run("rune range iteration: keys are integers and values are runes", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for i, r in 'a'..'z' {
					return [i, r]
				} 
			`)
			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(&Int{}, &Rune{}), res)
		})

		t.Run("streamable iteration", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for e in streamable {
					return e
				} 
			`)
			state.setGlobal("streamable", &AnyStreamSource{}, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, ANY, res)
		})

		t.Run("key variable should not be provided when iterating over a streamable", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			}, state.errors)
		})

		t.Run("chunked streamable iteration", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				for chunked chunk in streamable {
					return chunk
				} 
			`)
			state.setGlobal("streamable", NewWatcher(ANY_STR_PATTERN_ELEM), GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewTupleOf(&String{}), res)
		})

	})

	t.Run("walk statement", func(t *testing.T) {
		t.Run("walked value is not walkable", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				path = 1
				walk $path entry {
	
				}
			`)

			walkStmt := n.Statements[1]
			res, err := symbolicEval(n, state)

			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(walkStmt, state, fmtXisNotWalkable(&Int{})),
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("entries have right value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				walk ./ entry {
					return entry
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, WALK_ELEM, res)
		})

		t.Run("meta", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				walk ./ meta, entry {
					return [meta, entry]
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, NewList(ANY, WALK_ELEM), res)
		})
	})

	t.Run("switch statement: error in every block", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			v = 1
			switch v {
				0 {
					!"s"
				}
				1 {
					!"s"
				}
			}
		`)
		unaryExprs := parse.FindNodes(n, (*parse.UnaryExpression)(nil), nil)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Equal(t, []SymbolicEvaluationError{
			makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(&String{})),
			makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(&String{})),
		}, state.errors)
		assert.Nil(t, res)
	})

	t.Run("match statement", func(t *testing.T) {
		t.Run("join", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				var v %obj = {}
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
			assert.Empty(t, state.errors)
			assert.Equal(t, NewMultivalue(
				NewEmptyObject(),
				NewObject(map[string]SymbolicValue{"a": ANY_INT}, nil),
				NewObject(map[string]SymbolicValue{"b": ANY_INT}, nil),
			), res)
		})

		t.Run("error in every block", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
				makeSymbolicEvalError(unaryExprs[0], state, fmtOperandOfBoolNegateShouldBeBool(&String{})),
				makeSymbolicEvalError(unaryExprs[1], state, fmtOperandOfBoolNegateShouldBeBool(&String{})),
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("single group matching case", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				match /home/user {
					%/home/{:username} m {
						m.username
					}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Nil(t, res)
		})

		t.Run("two group matching cases with same variable", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			assert.Empty(t, state.errors)
			assert.Nil(t, res)
		})

		t.Run("narrowing of variable's value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			assert.Empty(t, state.errors)
			assert.Nil(t, res)
		})

		t.Run("narrowing of property", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			assert.Empty(t, state.errors)
			assert.Nil(t, res)
		})
	})

	t.Run("regex literal", func(t *testing.T) {
		n, state := makeStateAndChunk("%`a`")

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &RegexPattern{}, res)
	})

	t.Run("object pattern literal", func(t *testing.T) {

		t.Run("spread pattern that is not an object pattern", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{...1}
			`)

			spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, fmtPatternSpreadInObjectPatternShouldBeAnObjectPatternNot(&ExactValuePattern{value: &Int{}})),
			}, state.errors)
			assert.Equal(t, &ObjectPattern{entries: map[string]Pattern{}}, res)
		})

		t.Run("spread inexact object pattern", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{...%{...}}
			`)

			//spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)

			// assert.Equal(t, []SymbolicEvaluationError{
			// 	makeSymbolicEvalError(spreadElem, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT),
			// }, state.errors)
			assert.Equal(t, &ObjectPattern{entries: map[string]Pattern{}}, res)
		})

		t.Run("spread object pattern matching all objects", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{...%p}
			`)

			state.ctx.AddNamedPattern("p", &ObjectPattern{})

			spreadElem := parse.FindNode(n, (*parse.PatternPropertySpreadElement)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, CANNOT_SPREAD_OBJ_PATTERN_THAT_MATCHES_ANY_OBJECT),
			}, state.errors)
			assert.Equal(t, &ObjectPattern{entries: map[string]Pattern{}}, res)
		})

		t.Run("spread valid object pattern", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{...%{name: %str}}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
			}, res)
		})

		t.Run("pattern call", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{a: %int(0..1)}
			`)

			patt, _ := state.ctx.ResolveNamedPattern("int").Call(nil, []SymbolicValue{&IntRange{}})

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": patt},
			}, res)
		})

		t.Run("pattern call: invalid/missing arguments", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{a: %int()}
			`)

			patternCallExpr := parse.FindNode(n, (*parse.PatternCallExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(patternCallExpr, state, "missing argument"),
			}, state.errors)
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": ANY_PATTERN},
			}, res)
		})

		t.Run("pattern namespace's member", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{a: %myns.int}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{"a": state.ctx.ResolveNamedPattern("int")},
			}, res)
		})

		t.Run("deep object pattern", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return %{
					a: %{name: %str}
					b: %{
						c: %{count: %int}
						d: 1
					}
					e: 2
					f: {}
				}
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &ObjectPattern{
				entries: map[string]Pattern{
					"a": &ObjectPattern{
						entries: map[string]Pattern{"name": state.ctx.ResolveNamedPattern("str")},
					},
					"b": &ObjectPattern{
						entries: map[string]Pattern{
							"c": &ObjectPattern{
								entries: map[string]Pattern{
									"count": state.ctx.ResolveNamedPattern("int"),
								},
							},
							"d": NewExactValuePattern(&Int{}),
						},
					},
					"e": NewExactValuePattern(&Int{}),
					"f": NewExactValuePattern(NewEmptyObject()),
				},
			}, res)
		})

	})

	t.Run("list pattern literal", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			return %[ %{} ]
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &ListPattern{
			elements: []Pattern{
				&ObjectPattern{
					entries: map[string]Pattern{},
				},
			},
		}, res)
	})

	t.Run("union pattern", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			return %| 1 | "1"
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &UnionPattern{
			Cases: []Pattern{
				&ExactValuePattern{value: &Int{}},
				&ExactValuePattern{value: &String{}},
			},
		}, res)
	})

	t.Run("union pattern", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			return %| %int | %str
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &UnionPattern{
			Cases: []Pattern{
				state.ctx.ResolveNamedPattern("int"),
				state.ctx.ResolveNamedPattern("str"),
			},
		}, res)
	})

	t.Run("option pattern", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			return %--name=%str
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &OptionPattern{}, res)
	})

	t.Run("string pattern", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			return %str( "a" )
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &SequenceStringPattern{}, res)
	})

	t.Run("pattern definition: object pattern literal", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			%p = %{list: %[1]}
			return %p
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, &ObjectPattern{
			entries: map[string]Pattern{
				"list": &ListPattern{
					elements: []Pattern{
						&ExactValuePattern{value: &Int{}},
					},
				},
			},
		}, res)
	})

	t.Run("pattern namespace definition: object pattern literal", func(t *testing.T) {
		t.Run("RHS is an object pattern literal", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%namespace. = {}
				return %namespace.
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &PatternNamespace{
				entries: nil,
			}, res)
		})

		t.Run("RHS is invalid", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%namespace. = 1
				return %namespace.
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0], state, fmtPatternNamespaceShouldBeInitWithNot(&Int{})),
			}, state.errors)
			assert.Equal(t, &PatternNamespace{
				entries: nil,
			}, res)
		})

	})

	t.Run("optional pattern", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := makeStateAndChunk(`%int?`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &OptionalPattern{
				pattern: state.ctx.ResolveNamedPattern("int"),
			}, res)
		})

		t.Run("pattern already matches nil", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%p = nil
				return %p?
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[1].(*parse.ReturnStatement).Expr, state, CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL),
			}, state.errors)
			assert.Equal(t, &AnyPattern{}, res)
		})
	})

	t.Run("host alias definition: RHS is not a host", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			@h = 1
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Equal(t, []SymbolicEvaluationError{
			makeSymbolicEvalError(n.Statements[0], state, fmtCannotCreateHostAliasWithA(&Int{})),
		}, state.errors)
		assert.Nil(t, res)
	})

	t.Run("assertion statement", func(t *testing.T) {
		t.Run("value is a boolean", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				assert (true or false)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Nil(t, res)
		})

		t.Run("value is not a boolean", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				assert (1 + 1)
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(n.Statements[0], state, fmtAssertedValueShouldBeBoolNot(&Int{})),
			}, state.errors)
			assert.Nil(t, res)
		})

		t.Run("binary match expression narrows the type of a variable (%int)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				assert (a match %int)
				return (1 + a)
			`)

			state.setGlobal("a", ANY, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Int{}, res)
		})

		t.Run("binary match expression narrows the type of a variable: (object pattern literal)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				assert (a match %{a: 1, b: [3]})
			`)

			state.setGlobal("a", ANY, GlobalConst)

			res, err := symbolicEval(n, state)

			varInfo, _ := state.get("a")
			expectedObject := &Object{
				entries: map[string]SymbolicValue{
					"a": &Int{},
					"b": NewList(&Int{}),
				},
				static: map[string]Pattern{
					"a": &ExactValuePattern{value: &Int{}},
					"b": &ExactValuePattern{value: NewList(&Int{})},
				},
			}
			assert.Equal(t, expectedObject, varInfo.value)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Nil(t, res)
		})

		t.Run("binary match expression narrows the type of a variable: (list pattern literal)", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				assert (a match %[]%obj)
			`)

			state.setGlobal("a", ANY, GlobalConst)

			res, err := symbolicEval(n, state)

			varInfo, _ := state.get("a")
			expectedObject := &List{
				generalElement: &Object{},
			}
			assert.Equal(t, varInfo.value, expectedObject)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Nil(t, res)
		})

	})

	t.Run("runtime typecheck expression", func(t *testing.T) {
		t.Run("argument of Go function", func(t *testing.T) {
			n, state := makeStateAndChunk(`f ~arg`)

			goFunc := &GoFunction{
				fn: func(*Context, *Int) {
				},
			}

			state.setGlobal("f", goFunc, GlobalConst)
			state.setGlobal("arg", ANY, GlobalConst)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, Nil, res)
		})

		t.Run("argument of Inox function", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			assert.Empty(t, state.errors)
			assert.Equal(t, ANY_INT, res)
		})
	})

	t.Run("testsuite expression", func(t *testing.T) {
		t.Run("empty module", func(t *testing.T) {
			n, state := makeStateAndChunk(`testsuite "name" {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &TestSuite{}, res)
		})

		t.Run("error in module", func(t *testing.T) {
			n, state := makeStateAndChunk(`testsuite "name" {
				(1 + true)
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "%boolean")),
			}, state.errors)
			assert.Equal(t, &TestSuite{}, res)
		})
	})

	t.Run("testcase expression", func(t *testing.T) {
		t.Run("empty module", func(t *testing.T) {
			n, state := makeStateAndChunk(`testcase "name" {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &TestCase{}, res)
		})

		t.Run("error in module", func(t *testing.T) {
			n, state := makeStateAndChunk(`testcase "name" {
				(1 + true)
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "%boolean")),
			}, state.errors)
			assert.Equal(t, &TestCase{}, res)
		})
	})

	t.Run("lifetimejob expression", func(t *testing.T) {
		t.Run("should have access to implicit subject properties defined before and after the jobs", func(t *testing.T) {
			n, state := makeStateAndChunk(`{ 
				a: 1
				lifetimejob "name" { [self.a, self.b] } 
				b: 2
			}`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
		})

		t.Run("accessing a non existing property of the subject should cause an error", func(t *testing.T) {
			n, state := makeStateAndChunk(`{ 
				lifetimejob "name" { self.a } 
			}`)

			_, err := symbolicEval(n, state)

			membExpr := parse.FindNode(n, &parse.MemberExpression{}, nil)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(membExpr, state, fmtPropOfSymbolicDoesNotExist("a", NewEmptyObject(), "")),
			}, state.errors)
		})

		t.Run("implicit subject: error in module", func(t *testing.T) {
			n, state := makeStateAndChunk(`{ 
				a: 1
				lifetimejob "name" { (1 + true) } 
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "%boolean")),
			}, state.errors)
		})

		t.Run("explicit subject", func(t *testing.T) {
			n, state := makeStateAndChunk(`lifetimejob "name" for %list {}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &LifetimeJob{subjectPattern: state.ctx.ResolveNamedPattern("list")}, res)
		})

		t.Run("explicit subject: error in module", func(t *testing.T) {
			n, state := makeStateAndChunk(`lifetimejob "name" for %list {
				(1 + true)
			}`)

			binExpr := parse.FindNode(n, &parse.BinaryExpression{}, nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "%boolean")),
			}, state.errors)
			assert.Equal(t, &LifetimeJob{subjectPattern: state.ctx.ResolveNamedPattern("list")}, res)
		})

		t.Run("explicit subject: not matched by self", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				{
					lifetimejob "name" for %{a: %int} {}
				}
			`)

			lifetimeJobExpr := parse.FindNode(n, &parse.LifetimejobExpression{}, nil)
			subjectPattern := newExactObjectPattern(map[string]Pattern{
				"a": state.ctx.ResolveNamedPattern("int"),
			})

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(lifetimeJobExpr, state, fmtSelfShouldMatchLifetimeJobSubjectPattern(subjectPattern)),
			}, state.errors)
		})

		t.Run("lifetime job within an object literal should have access to patterns defined in parent state", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%p = 1
				{ 
					a: 1
					lifetimejob "name" { [%p, %int]  } 
				}`,
			)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
		})

		t.Run("lifetime job within a function should have access to patterns defined in top level state", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				%p = 1
				fn createJob(){
					return lifetimejob "name" for %obj { [%p, %int]  } 
				}
			`)

			_, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
		})

		//TODO: add tests on globals

	})

	t.Run("spawn_expression", func(t *testing.T) {
		t.Run("single expression", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(){ }
				return go {globals: .{}} do f()
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.IsType(t, &Routine{}, res)
		})

		t.Run("single expression without meta", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				fn f(){ }
				return go do f()
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.IsType(t, &Routine{}, res)
		})

		t.Run("provided group is not a routine group", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				group = 0
				return go {group: group, globals: .{}} do { }
			`)

			res, err := symbolicEval(n, state)
			obj := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(obj, state, fmtGroupPropertyNotRoutineGroup(&Int{})),
			}, state.errors)
			assert.IsType(t, &Routine{}, res)
		})

		t.Run("error in embedded module", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return go {globals: .{}} do { return (1 + "a") }
			`)

			binExpr := parse.FindNode(n, (*parse.BinaryExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(binExpr.Right, state, fmtRightOperandOfBinaryShouldBe(parse.Add, "int", "%string")),
			}, state.errors)
			assert.IsType(t, &Routine{}, res)
		})

		t.Run("call provided function", func(t *testing.T) {
			n, state := makeStateAndChunk(`
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
			assert.Empty(t, state.errors)
			assert.IsType(t, ANY, res)
		})

	})

	t.Run("reception handler expression", func(t *testing.T) {
		n, state := makeStateAndChunk(`
			{
				on received %{} {}
			}
		`)

		res, err := symbolicEval(n, state)
		assert.NoError(t, err)
		assert.Empty(t, state.errors)
		assert.Equal(t, NewObject(map[string]SymbolicValue{
			"0": ANY_SYNC_MSG_HANDLER,
		}, nil), res)

	})

	t.Run("sendvalue expression", func(t *testing.T) {
		t.Run("in method", func(t *testing.T) {
			n, state := makeStateAndChunk(`{
				f: fn(){ 
					sendval 1 to supersys
				}
			}`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.IsType(t, &Object{}, res)
		})
	})

	t.Run("mapping expression", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				Mapping { 0 => 1  1 => comp 0 }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Mapping{}, res)
		})

		t.Run("key variable & group matching variable should be accessible in right side", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				Mapping { p %/{:name} m => [p, m] }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Mapping{}, res)
		})

		t.Run("key variable should be accessible in right side and have right type", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				Mapping { n 1 => (n - 1) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Mapping{}, res)
		})

		t.Run("key variable should be accessible in right side and have right type: case pattern key", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				Mapping { n %int => (n - 1) }
			`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &Mapping{}, res)
		})

	})

	t.Run("compute expression", func(t *testing.T) {

		t.Run("argument is not a simple value", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				Mapping { 0 => comp {} }
			`)

			computeExpr := parse.FindNode(n, (*parse.ComputeExpression)(nil), nil)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(computeExpr.Arg, state, INVALID_KEY_IN_COMPUTE_EXPRESSION_ONLY_SIMPLE_VALUE_ARE_SUPPORTED),
			}, state.errors)

			assert.Equal(t, &Mapping{}, res)
		})
	})
	t.Run("concatenation expression", func(t *testing.T) {
		t.Run("single string element", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat "a"`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, &String{}, res)
		})

		t.Run("two string-like elements", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat "a" "b"`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, &StringConcatenation{}, res)
		})

		t.Run("single byteslice element", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat 0d[12]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, &ByteSlice{}, res)
		})

		t.Run("two bytes-like elements", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat 0d[12] 0d[34]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, &BytesConcatenation{}, res)
		})

		t.Run("two tuples with known elements", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat #[1] #["a"]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, NewTuple(ANY_INT, ANY_STR), res)
		})

		t.Run("two tuples with unknown elements, different general elements", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return fn(a %int_tuple, b %str_tuple){
					return concat a b
				}`,
			)
			state.ctx.AddNamedPattern("int_tuple", &TypePattern{val: NewTupleOf(ANY_INT)})
			state.ctx.AddNamedPattern("str_tuple", &TypePattern{val: NewTupleOf(ANY_STR)})

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			expectedFn := &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{NewTupleOf(&Int{}), NewTupleOf(&String{})},
				parameterNames: []string{"a", "b"},
				returnType:     NewTupleOf(NewMultivalue(ANY_INT, ANY_STR)),
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("two tuples with unknown elements, same general element", func(t *testing.T) {
			n, state := makeStateAndChunk(`
				return fn(a %int_tuple, b %int_tuple){
					return concat a b
				}`,
			)
			state.ctx.AddNamedPattern("int_tuple", &TypePattern{val: NewTupleOf(ANY_INT)})

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)

			fnExpr := n.Statements[0].(*parse.ReturnStatement).Expr
			expectedFn := &InoxFunction{
				node:           fnExpr,
				parameters:     []SymbolicValue{NewTupleOf(&Int{}), NewTupleOf(&Int{})},
				parameterNames: []string{"a", "b"},
				returnType:     NewTupleOf(ANY_INT),
			}
			assert.Equal(t, expectedFn, res)
		})

		t.Run("spread string list", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat ...["a"]`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, &StringConcatenation{}, res)
		})

		t.Run("spread list with invalid values", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat ...[1]`)
			res, err := symbolicEval(n, state)

			spreadElem := parse.FindNode(n, (*parse.ElementSpreadElement)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION),
			}, state.errors)
			assert.Empty(t, err)
			assert.Equal(t, ANY, res)
		})

		t.Run("string followed by a spread list with invalid values", func(t *testing.T) {
			n, state := makeStateAndChunk(`concat "a" ...[1]`)
			res, err := symbolicEval(n, state)

			spreadElem := parse.FindNode(n, (*parse.ElementSpreadElement)(nil), nil)

			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(spreadElem, state, CONCATENATION_SUPPORTED_TYPES_EXPLANATION),
			}, state.errors)
			assert.Empty(t, err)
			assert.Equal(t, &StringConcatenation{}, res)
		})

		t.Run("non iterable spread element", func(t *testing.T) {
			n, state := makeStateAndChunk(`return concat ...1`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, ANY, res)
		})

		t.Run("string followed by a non iterable spread element", func(t *testing.T) {
			n, state := makeStateAndChunk(`return concat "a" ...1`)

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, err)
			assert.Equal(t, &StringConcatenation{}, res)
		})
	})

	t.Run("string template literal", func(t *testing.T) {

		replace := func(s string) string {
			return strings.ReplaceAll(s, "|", "`")
		}

		t.Run("no interpolation", func(t *testing.T) {
			n, state := makeStateAndChunk(replace(`
				%digit = %str('0'..'9')
				return %digit|3|
			`))

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Empty(t, state.errors)
			assert.Equal(t, &CheckedString{}, res)
		})

		t.Run("interpolation & non-namespaced pattern", func(t *testing.T) {
			n, state := makeStateAndChunk(replace(`
				%sql = %str( %|.*| )
				unsanitized_id = "5"
				return %sql|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit, state, STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX),
			}, state.errors)
			assert.Equal(t, &CheckedString{}, res)
		})

		t.Run("interpolation pattern does not exist", func(t *testing.T) {
			n, state := makeStateAndChunk(replace(`
				%sql. = {stmt: %str( %|.*| )}
				unsanitized_id = "5"
				return %sql.stmt|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit, state, fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist("int", "sql")),
			}, state.errors)
			assert.Equal(t, &CheckedString{}, res)
		})

		t.Run("interpolation expression is not a string", func(t *testing.T) {
			n, state := makeStateAndChunk(replace(`
				%sql. = {
					stmt: %str( %|.*| ),
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = {}
				return %sql.stmt|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`))

			templateLit := n.Statements[2].(*parse.ReturnStatement).Expr

			res, err := symbolicEval(n, state)
			assert.NoError(t, err)
			assert.Equal(t, []SymbolicEvaluationError{
				makeSymbolicEvalError(templateLit, state, fmtInterpolationIsNotStringBut(&Object{entries: map[string]SymbolicValue{}})),
			}, state.errors)
			assert.Equal(t, &CheckedString{}, res)
		})
	})

}

func TestWidenValues(t *testing.T) {

	cases := []struct {
		input  []SymbolicValue
		output SymbolicValue
	}{
		{[]SymbolicValue{&Int{}}, &Int{}},
		{[]SymbolicValue{&Int{}, &Int{}}, &Int{}},
		{[]SymbolicValue{&Int{}, &String{}}, NewMultivalue(&Int{}, &String{})},
		{[]SymbolicValue{&String{}, &Int{}}, NewMultivalue(&String{}, &Int{})},
		{[]SymbolicValue{&Identifier{"foo"}, &Identifier{}}, &Identifier{}},
		{[]SymbolicValue{&Identifier{}, &Identifier{"foo"}}, &Identifier{}},
		{
			[]SymbolicValue{
				NewObject(map[string]SymbolicValue{"a": &Int{}}, nil),
				NewObject(map[string]SymbolicValue{}, nil),
			},
			NewMultivalue(
				NewObject(map[string]SymbolicValue{"a": &Int{}}, nil),
				NewObject(map[string]SymbolicValue{}, nil),
			),
		},
		{
			[]SymbolicValue{
				NewObject(map[string]SymbolicValue{}, nil),
				NewObject(map[string]SymbolicValue{"a": &Int{}}, nil),
			},
			NewMultivalue(
				NewObject(map[string]SymbolicValue{}, nil),
				NewObject(map[string]SymbolicValue{"a": &Int{}}, nil),
			),
		},
		{
			[]SymbolicValue{
				NewObject(map[string]SymbolicValue{"a": ANY}, nil),
				NewObject(map[string]SymbolicValue{"a": &Int{}}, nil),
			},
			NewObject(map[string]SymbolicValue{"a": ANY}, nil),
		},
		{
			[]SymbolicValue{
				NewList(&String{}),
				NewList(&Int{}),
			},
			NewMultivalue(
				NewList(&String{}),
				NewList(&Int{}),
			),
		},
		{
			[]SymbolicValue{
				NewList(&String{}, &String{}),
				NewList(&Int{}, &String{}),
			},
			NewMultivalue(
				NewList(&String{}, &String{}),
				NewList(&Int{}, &String{}),
			),
		},
	}
	for _, testCase := range cases {
		t.Run(t.Name()+"_"+fmt.Sprint(testCase.input), func(t *testing.T) {
			output := joinValues(testCase.input)
			assert.Equal(t, testCase.output, output, fmt.Sprint(output))
		})
	}
}
