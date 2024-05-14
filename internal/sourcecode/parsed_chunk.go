package sourcecode

import (
	"fmt"
	"io"
	"sync"
)

// ParsedChunkSource contains an AST and the ChunkSource that was parsed to obtain it.
// ParsedChunkSource provides helper methods to find nodes in the AST and to get positions.
type ParsedChunkSource interface {
	ChunkSource() ChunkSource

	// unique name | URL | path
	Name() string

	// result should not be modified.
	Runes() []rune

	GetSpanLineColumn(span NodeSpan) (int32, int32)

	GetSourcePosition(span NodeSpan) PositionRange

	FormatNodeSpanLocation(w io.Writer, nodeSpan NodeSpan) (int, error)

	GetLineCut(cutIndex int32) (beforeSpan string, afterSpan string)
}

type ParsedChunkSourceBase struct {
	Source    ChunkSource
	runes     []rune
	runesLock sync.Mutex
}

func MakeParsedChunkSourceBaseWithRunes(source ChunkSource, runes []rune) ParsedChunkSourceBase {
	return ParsedChunkSourceBase{
		Source: source,
		runes:  runes,
	}
}

func (c *ParsedChunkSourceBase) ChunkSource() ChunkSource {
	return c.Source
}

// unique name | URL | path
func (c *ParsedChunkSourceBase) Name() string {
	return c.Source.Name()
}

// result should not be modified.
func (c *ParsedChunkSourceBase) Runes() []rune {
	c.runesLock.Lock()
	defer c.runesLock.Unlock()

	runes := c.runes
	if c.Source.Code() != "" && len(runes) == 0 {
		c.runes = []rune(c.Source.Code())
	}
	return c.runes
}

func (chunk *ParsedChunkSourceBase) FormatNodeSpanLocation(w io.Writer, nodeSpan NodeSpan) (int, error) {
	line, col := chunk.GetSpanLineColumn(nodeSpan)
	return fmt.Fprintf(w, "%s:%d:%d:", chunk.Name(), line, col)
}

func (chunk *ParsedChunkSourceBase) GetSpanLineColumn(span NodeSpan) (int32, int32) {
	line := int32(1)
	col := int32(1)
	i := 0

	runes := chunk.Runes()

	for i < int(span.Start) && i < len(runes) {
		if runes[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}

		i++
	}

	return line, col
}

func (chunk *ParsedChunkSourceBase) GetIncludedEndSpanLineColumn(span NodeSpan) (int32, int32) {
	line := int32(1)
	col := int32(1)
	i := 0

	runes := chunk.Runes()

	for i < int(span.End-1) && i < len(runes) {
		if runes[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}

		i++
	}

	return line, col
}

func (chunk *ParsedChunkSourceBase) GetEndSpanLineColumn(span NodeSpan) (int32, int32) {
	line := int32(1)
	col := int32(1)
	i := 0

	runes := chunk.Runes()

	for i < int(span.End) && i < len(runes) {
		if runes[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}

		i++
	}

	return line, col
}

func (chunk *ParsedChunkSourceBase) GetLineCut(cutIndex int32) (beforeSpan string, afterSpan string) {
	runes := chunk.Runes()

	i := cutIndex

	for i > 0 && runes[i] != '\n' {
		i--
	}

	beforeSpan = string(runes[i:cutIndex])

	i = cutIndex

	for i < len32(runes) && runes[i] != '\n' {
		i++
	}

	afterSpan = string(runes[cutIndex:i])

	return
}

func (chunk *ParsedChunkSourceBase) GetLineColumnSingeCharSpan(line, column int32) NodeSpan {
	pos := chunk.GetLineColumnPosition(line, column)
	return NodeSpan{
		Start: pos,
		End:   pos + 1,
	}
}

func (chunk *ParsedChunkSourceBase) GetLineColumnPosition(line, column int32) int32 {
	i := int32(0)
	runes := chunk.Runes()
	length := len32(runes)

	line -= 1

	for i < length && line > 0 {
		if runes[i] == '\n' {
			line--
		}
		i++
	}

	pos := i + column - 1
	return pos
}

func (chunk *ParsedChunkSourceBase) GetSourcePosition(span NodeSpan) PositionRange {
	line, col := chunk.GetSpanLineColumn(span)
	endLine, endCol := chunk.GetEndSpanLineColumn(span)

	return PositionRange{
		SourceName:  chunk.Name(),
		StartLine:   line,
		StartColumn: col,
		EndLine:     endLine,
		EndColumn:   endCol,
		Span:        span,
	}
}

func len32[E any](s []E) int32 {
	return int32(len(s))
}
