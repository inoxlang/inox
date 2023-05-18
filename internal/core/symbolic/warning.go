package internal

import parse "github.com/inoxlang/inox/internal/parse"

type SymbolicEvaluationWarning struct {
	Message        string
	LocatedMessage string
	Location       parse.SourcePositionStack
}
