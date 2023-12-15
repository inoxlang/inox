package symbolic

import (
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestInoxFunction(t *testing.T) {
	t.Run("Test()", func(t *testing.T) {

		//TODO: add more tests.

		inoxFnWithAssignment := &InoxFunction{
			parameters:     []Value{},
			parameterNames: []string{},
			node:           parse.MustParseExpression(`fn(){a = 1}`),
			result:         ANY,
		}

		inoxFnWithOnlyIntLiteral := &InoxFunction{
			parameters:     []Value{},
			parameterNames: []string{},
			node:           parse.MustParseExpression(`fn(){1}`),
			result:         ANY,
		}

		inoxFnOnlyAllowingIntLiteralsInBody := &InoxFunction{
			parameters:     []Value{},
			parameterNames: []string{},
			result:         ANY,
			visitCheckNode: func(visit visitArgs, globalsAtCreation map[string]Value) (parse.TraversalAction, bool, error) {
				_, ok := visit.node.(*parse.IntLiteral)
				return parse.ContinueTraversal, ok, nil
			},
		}

		assertTestFalse(t, inoxFnOnlyAllowingIntLiteralsInBody, inoxFnWithAssignment)
		assertTest(t, inoxFnOnlyAllowingIntLiteralsInBody, inoxFnWithOnlyIntLiteral)

		t.Run("args", func(t *testing.T) {
			integralArgInoxFn := &InoxFunction{
				parameters:     []Value{ANY_INTEGRAL},
				parameterNames: []string{"arg"},
				result:         ANY,
			}
			intArgInoxFn := &InoxFunction{
				parameters:     []Value{ANY_INT},
				parameterNames: []string{"arg"},
				result:         ANY,
			}

			integralArgFn := NewFunction([]Value{ANY_INTEGRAL}, []string{"arg"}, -1, false, []Value{ANY})
			intArgFn := NewFunction([]Value{ANY_INT}, []string{"arg"}, -1, false, []Value{ANY})
			integralArgGoFn := WrapGoFunction(func(ctx *Context, arg Integral) Value { return ANY })
			intArgGoFn := WrapGoFunction(func(ctx *Context, arg *Int) Value { return ANY })

			assertTest(t, intArgInoxFn, intArgInoxFn)
			assertTest(t, integralArgInoxFn, integralArgInoxFn)

			//a function accepting any integral value as a first argument can handle integer values.
			assertTest(t, intArgInoxFn, integralArgInoxFn)

			//a function accepting any integer value as a first argument cannot handle all integral values.
			assertTestFalse(t, integralArgInoxFn, intArgFn)

			//Inox functions should never match a non-Inox function.
			assertTestFalse(t, intArgInoxFn, intArgFn)
			assertTestFalse(t, intArgInoxFn, integralArgFn)
			assertTestFalse(t, intArgInoxFn, intArgGoFn)
			assertTestFalse(t, intArgInoxFn, integralArgGoFn)

			assertTestFalse(t, integralArgInoxFn, integralArgFn)
			assertTestFalse(t, integralArgInoxFn, intArgFn)
			assertTestFalse(t, integralArgInoxFn, integralArgGoFn)
			assertTestFalse(t, integralArgInoxFn, intArgGoFn)
		})

		t.Run("result", func(t *testing.T) {
			integralResultFn := NewFunction([]Value{}, []string{}, -1, false, []Value{ANY_INTEGRAL})
			intResultFn := NewFunction([]Value{}, []string{}, -1, false, []Value{ANY_INT})

			integralResultInoxFn := &InoxFunction{
				parameters:     []Value{},
				parameterNames: []string{},
				result:         ANY_INTEGRAL,
			}
			intResultInoxFn := &InoxFunction{
				parameters:     []Value{},
				parameterNames: []string{},
				result:         ANY_INT,
			}

			integralResultGoFn := WrapGoFunction(func(ctx *Context) Integral {
				return ANY_INTEGRAL
			})

			intResultGoFn := WrapGoFunction(func(ctx *Context) *Int {
				return ANY_INT
			})

			assertTest(t, integralResultInoxFn, integralResultInoxFn)
			assertTest(t, intResultInoxFn, intResultInoxFn)

			//a function returning an integer is a special case of a function returning an integral value.
			assertTest(t, integralResultInoxFn, intResultInoxFn)

			//a function returning an integral value does not necessarily return an integer.
			assertTestFalse(t, intResultInoxFn, integralResultInoxFn)

			//Inox functions should never match a non-Inox function.
			assertTestFalse(t, intResultInoxFn, intResultFn)
			assertTestFalse(t, intResultInoxFn, integralResultFn)
			assertTestFalse(t, intResultInoxFn, intResultGoFn)
			assertTestFalse(t, intResultInoxFn, integralResultGoFn)

			assertTestFalse(t, integralResultInoxFn, integralResultFn)
			assertTestFalse(t, integralResultInoxFn, intResultFn)
			assertTestFalse(t, integralResultInoxFn, integralResultGoFn)
			assertTestFalse(t, integralResultInoxFn, intResultGoFn)
		})

	})
}

func TestGoFunction(t *testing.T) {

	t.Run("only ctx param", func(t *testing.T) {
		goFunc := WrapGoFunction(func(ctx *Context) {})
		if !assert.NoError(t, goFunc.LoadSignatureData()) {
			return
		}

		assert.False(t, goFunc.hasOptionalParams)
	})

	//TODO: add more tests.
}

func TestFunction(t *testing.T) {
	t.Run("Test()", func(t *testing.T) {

		//TODO: add more tests.

		t.Run("args", func(t *testing.T) {
			integralArgFn := NewFunction([]Value{ANY_INTEGRAL}, []string{"arg"}, -1, false, []Value{ANY})
			intArgFn := NewFunction([]Value{ANY_INT}, []string{"arg"}, -1, false, []Value{ANY})

			integralArgInoxFn := &InoxFunction{
				parameters:     []Value{ANY_INTEGRAL},
				parameterNames: []string{"arg"},
				result:         ANY,
			}
			intArgInoxFn := &InoxFunction{
				parameters:     []Value{ANY_INT},
				parameterNames: []string{"arg"},
				result:         ANY,
			}

			//a function accepting any integeral value as a first argument can handle integer values.
			assertTest(t, intArgFn, integralArgFn)
			assertTest(t, intArgFn, integralArgInoxFn)

			//a function accepting any integer value as a first argument cannot handle all integral values.
			assertTestFalse(t, integralArgFn, intArgFn)
			assertTestFalse(t, integralArgFn, intArgInoxFn)
		})

		t.Run("args", func(t *testing.T) {
			integralArgFn := NewFunction([]Value{ANY_INTEGRAL}, []string{"arg"}, -1, false, []Value{ANY})
			intArgFn := NewFunction([]Value{ANY_INT}, []string{"arg"}, -1, false, []Value{ANY})

			integralArgInoxFn := &InoxFunction{
				parameters:     []Value{ANY_INTEGRAL},
				parameterNames: []string{"arg"},
				result:         ANY,
			}
			intArgInoxFn := &InoxFunction{
				parameters:     []Value{ANY_INT},
				parameterNames: []string{"arg"},
				result:         ANY,
			}

			integralArgGoFn := WrapGoFunction(func(ctx *Context, arg Integral) Value { return ANY })
			intArgGoFn := WrapGoFunction(func(ctx *Context, arg *Int) Value { return ANY })

			assertTest(t, intArgFn, intArgFn)
			assertTest(t, intArgFn, intArgInoxFn)
			assertTest(t, intArgFn, intArgGoFn)

			assertTest(t, integralArgFn, integralArgFn)
			assertTest(t, integralArgFn, integralArgInoxFn)
			assertTest(t, integralArgFn, integralArgGoFn)

			//a function accepting any integeral value as a first argument can handle integer values.
			assertTest(t, intArgFn, integralArgFn)
			assertTest(t, intArgFn, integralArgInoxFn)
			assertTest(t, intArgFn, integralArgGoFn)

			//a function accepting any integer value as a first argument cannot handle all integral values.
			assertTestFalse(t, integralArgFn, intArgFn)
			assertTestFalse(t, integralArgFn, intArgInoxFn)
			assertTestFalse(t, integralArgFn, intArgGoFn)
		})

		t.Run("single result", func(t *testing.T) {
			integralResultFn := NewFunction([]Value{}, []string{}, -1, false, []Value{ANY_INTEGRAL})
			intResultFn := NewFunction([]Value{}, []string{}, -1, false, []Value{ANY_INT})

			integralResultInoxFn := &InoxFunction{
				parameters:     []Value{},
				parameterNames: []string{},
				result:         ANY_INTEGRAL,
			}
			intResultInoxFn := &InoxFunction{
				parameters:     []Value{},
				parameterNames: []string{},
				result:         ANY_INT,
			}

			integralResultGoFn := WrapGoFunction(func(ctx *Context) Integral {
				return ANY_INTEGRAL
			})

			intResultGoFn := WrapGoFunction(func(ctx *Context) *Int {
				return ANY_INT
			})

			assertTest(t, integralResultFn, integralResultFn)
			assertTest(t, integralResultFn, integralResultInoxFn)
			assertTest(t, integralResultFn, integralResultGoFn)

			assertTest(t, intResultFn, intResultFn)
			assertTest(t, intResultFn, intResultInoxFn)
			assertTest(t, intResultFn, intResultGoFn)

			//a function returning an integer is a special case of a function returning an integral value.
			assertTest(t, integralResultFn, intResultFn)
			assertTest(t, integralResultFn, intResultInoxFn)
			assertTest(t, integralResultFn, intResultGoFn)

			//a function returning an integral value does not necessarily return an integer.
			assertTestFalse(t, intResultFn, integralResultFn)
			assertTestFalse(t, intResultFn, integralResultInoxFn)
			assertTestFalse(t, intResultFn, integralResultGoFn)
		})
	})
}
