package parse

import (
	"bytes"

	"github.com/inoxlang/inox/internal/parse/position"
)

type NodeSpan = position.NodeSpan
type StackLocatedError = position.StackLocatedError
type LocatedError = position.LocatedError
type SourcePositionRange = position.SourcePositionRange
type SourcePositionStack = position.SourcePositionStack

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
