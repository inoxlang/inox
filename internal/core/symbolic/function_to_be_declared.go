package symbolic

import (
	"github.com/inoxlang/inox/internal/ast"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	_ = Value((*inoxFunctionToBeDeclared)(nil))
)

type inoxFunctionToBeDeclared struct {
	decl *ast.FunctionDeclaration
}

func (*inoxFunctionToBeDeclared) IsMutable() bool {
	return false
}

func (*inoxFunctionToBeDeclared) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("function-not-declared-yet")
}

func (*inoxFunctionToBeDeclared) Test(v Value, state RecTestCallState) bool {
	panic("unimplemented")
}

func (*inoxFunctionToBeDeclared) WidestOfType() Value {
	return ANY_INOX_FUNC
}
