package hsanalysis

import (
	"fmt"

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

func (e Error) LocationRange() parse.SourcePositionRange {
	return e.Location
}

func (e Error) MessageWithoutLocation() string {
	return e.Message
}

type Warning struct {
	Message string

	Location       parse.SourcePositionRange
	LocatedMessage string
}

func (c *analyzer) addError(node hscode.JSONMap, msg string) {
	relativeNodeStart, relativeNodeEnd := hscode.GetNodeSpan(node)
	codeStartIndex := c.parameters.CodeStartIndex

	absoluteNodeStart := codeStartIndex + relativeNodeStart
	absoluteNodeEnd := codeStartIndex + relativeNodeEnd

	location := c.parameters.Chunk.GetSourcePosition(parse.NodeSpan{Start: absoluteNodeStart, End: absoluteNodeEnd})

	c.errors = append(c.errors, MakeError(msg, location))
}
