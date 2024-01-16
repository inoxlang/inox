package parse

import (
	"bytes"
	"fmt"
)

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

type LocatedError interface {
	MessageWithoutLocation() string
	LocationStack() SourcePositionStack
}

type ChunkStackItem struct {
	Chunk           *ParsedChunkSource
	CurrentNodeSpan NodeSpan //zero for the last item
}

func (i ChunkStackItem) GetChunk() (*ParsedChunkSource, bool) {
	return i.Chunk, i.Chunk != nil
}

func (i ChunkStackItem) GetCurrentNodeSpan() (NodeSpan, bool) {
	return i.CurrentNodeSpan, i.CurrentNodeSpan != (NodeSpan{})
}

type StackItem interface {
	GetChunk() (*ParsedChunkSource, bool)
	GetCurrentNodeSpan() (NodeSpan, bool)
}

func GetSourcePositionStack[Item StackItem](nodeSpan NodeSpan, chunkStack []Item) (SourcePositionStack, string) {
	locationPartBuff := bytes.NewBuffer(nil)
	var positionStack SourcePositionStack

	//TODO: get whole position stack
	for i, item := range chunkStack {
		var span NodeSpan
		chunk, hasChunk := item.GetChunk()

		if !hasChunk {
			locationPartBuff.WriteString("??:")

			if i != len(chunkStack)-1 {
				locationPartBuff.WriteRune(' ')
			}
			continue
		}

		if i == len(chunkStack)-1 {
			span = nodeSpan
		} else {
			var ok bool
			span, ok = item.GetCurrentNodeSpan()
			if !ok {
				span = NodeSpan{Start: 0, End: 1}
			}
		}

		position := chunk.GetSourcePosition(span)
		positionStack = append(positionStack, position)

		chunk.FormatNodeSpanLocation(locationPartBuff, span) //TODO: fix

		if i != len(chunkStack)-1 {
			locationPartBuff.WriteRune(' ')
		}
	}
	return positionStack, locationPartBuff.String()
}
