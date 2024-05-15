package hsanalysis

import (
	"fmt"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
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

func (a *analyzer) getNodeLocation(node hscode.JSONMap) sourcecode.PositionRange {
	return getNodeLocation(node, a.parameters.CodeStartIndex, a.parameters.Chunk)
}

func (a *analyzer) addError(node hscode.JSONMap, msg string) {
	location := a.getNodeLocation(node)
	a.errors = append(a.errors, MakeError(msg, location))
}

func getNodeLocation(node hscode.JSONMap, codeStartIndex int32, chunk sourcecode.ParsedChunkSource) sourcecode.PositionRange {
	relativeNodeStart, relativeNodeEnd := hscode.GetNodeSpan(node)
	absoluteNodeStart := codeStartIndex + relativeNodeStart
	absoluteNodeEnd := codeStartIndex + relativeNodeEnd

	return chunk.GetSourcePosition(parse.NodeSpan{Start: absoluteNodeStart, End: absoluteNodeEnd})
}
