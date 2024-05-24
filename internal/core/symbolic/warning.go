package symbolic

import "github.com/inoxlang/inox/internal/sourcecode"

type EvaluationWarning struct {
	Message        string
	LocatedMessage string
	Location       sourcecode.PositionStack
}
