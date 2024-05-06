package hsanalysis

import (
	"fmt"

	"github.com/inoxlang/inox/internal/codebase/analysis/text"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
)

type Error struct {
	Message string

	Location       parse.SourcePositionRange
	LocatedMessage string
}

func MakeError(msg string, location parse.SourcePositionRange) Error {
	return Error{
		Message:        msg,
		Location:       location,
		LocatedMessage: fmt.Sprintf("%s: %s", location, msg),
	}
}

func (e Error) Error() string {
	return e.LocatedMessage
}

type Warning struct {
	Message string

	Location       parse.SourcePositionStack
	LocatedMessage string
}

func (c *analyzer) addError(node hscode.JSONMap, msg string) {

	relativeNodeStart, relativeNodeEnd := hscode.GetNodeSpan(node)

	inoxNodeSpan := c.parameters.InoxNodePosition.Span
	absoluteNodeStart := inoxNodeSpan.Start + relativeNodeStart
	absoluteNodeEnd := inoxNodeSpan.End + relativeNodeEnd

	location := c.parameters.Chunk.GetSourcePosition(parse.NodeSpan{absoluteNodeStart, absoluteNodeEnd})

	c.errors = append(c.errors, Error{
		Message:        text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD,
		Location:       location,
		LocatedMessage: fmt.Sprintf("%s: %s", location, msg),
	})
}
