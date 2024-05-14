package inoxjs

import (
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
)

type Error struct {
	Message string

	Location                  parse.SourcePositionRange
	LocatedMessage            string
	IsHyperscriptParsingError bool
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
