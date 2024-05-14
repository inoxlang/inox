package position

import (
	"bytes"
	"fmt"
)

type NodeSpan struct {
	Start int32 `json:"start"` //0-indexed
	End   int32 `json:"end"`   //exclusive end, 0-indexed
}

func (s NodeSpan) HasPositionEndIncluded(i int32) bool {
	return i >= s.Start && i <= s.End
}

func (s NodeSpan) Len() int32 {
	return s.End - s.Start
}

type SourcePositionRange struct {
	SourceName  string   `json:"sourceName"`
	StartLine   int32    `json:"line"`      //1-indexed
	StartColumn int32    `json:"column"`    //1-indexed
	EndLine     int32    `json:"endLine"`   //1-indexed
	EndColumn   int32    `json:"endColumn"` //1-indexed
	Span        NodeSpan `json:"span"`
}

func (pos SourcePositionRange) String() string {
	return fmt.Sprintf("%s:%d:%d:", pos.SourceName, pos.StartLine, pos.StartColumn)
}

type SourcePositionStack []SourcePositionRange

func (stack SourcePositionStack) String() string {
	buff := bytes.NewBuffer(nil)
	for _, pos := range stack {
		buff.WriteString(pos.String())
		buff.WriteRune(' ')
	}
	return buff.String()
}

type StackLocatedError interface {
	error
	MessageWithoutLocation() string
	LocationStack() SourcePositionStack
}

type LocatedError interface {
	error
	MessageWithoutLocation() string
	LocationRange() SourcePositionRange
}
