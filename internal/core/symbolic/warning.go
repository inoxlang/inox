package symbolic

import "github.com/inoxlang/inox/internal/parse"

type EvaluationWarning struct {
	Message        string
	LocatedMessage string
	Location       parse.SourcePositionStack
}
