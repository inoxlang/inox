package symbolic

import (
	"testing"

	"github.com/inoxlang/inox/internal/parse"
)

func TestSymbolicInoxFunction(t *testing.T) {
	t.Run("Test()", func(t *testing.T) {

		fnWithAssignment := &InoxFunction{
			parameters:     []SymbolicValue{},
			parameterNames: []string{},
			node:           parse.MustParseExpression(`fn(){a = 1}`),
			result:         ANY,
		}

		fnWithOnlyIntLiteral := &InoxFunction{
			parameters:     []SymbolicValue{},
			parameterNames: []string{},
			node:           parse.MustParseExpression(`fn(){1}`),
			result:         ANY,
		}

		fnOnlyAllowingIntLiteralsInBody := &InoxFunction{
			parameters:     []SymbolicValue{},
			parameterNames: []string{},
			result:         ANY,
			visitCheckNode: func(visit visitArgs, globalsAtCreation map[string]SymbolicValue) (parse.TraversalAction, bool, error) {
				_, ok := visit.node.(*parse.IntLiteral)
				return parse.Continue, ok, nil
			},
		}

		assertTestFalse(t, fnOnlyAllowingIntLiteralsInBody, fnWithAssignment)
		assertTest(t, fnOnlyAllowingIntLiteralsInBody, fnWithOnlyIntLiteral)
	})
}
