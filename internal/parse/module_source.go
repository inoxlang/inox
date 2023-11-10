package parse

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

type ParsedChunk struct {
	Node      *Chunk
	Source    ChunkSource
	runes     []rune
	runesLock sync.Mutex
}

func (c *ParsedChunk) Name() string {
	return c.Source.Name()
}

// result should not be modified.
func (c *ParsedChunk) Runes() []rune {
	c.runesLock.Lock()
	defer c.runesLock.Unlock()

	runes := c.runes
	if c.Source.Code() != "" && len(runes) == 0 {
		c.runes = []rune(c.Source.Code())
	}
	return c.runes
}

type ChunkSource interface {
	Name() string             //unique name | URL | path
	UserFriendlyName() string //same as name but path values may be relative
	Code() string
}

type SourceFile struct {
	NameString             string
	UserFriendlyNameString string
	Resource               string //path or url
	ResourceDir            string //path or url
	IsResourceURL          bool
	CodeString             string
}

func (f SourceFile) Name() string {
	return f.NameString
}

func (f SourceFile) UserFriendlyName() string {
	if f.UserFriendlyNameString == "" {
		return f.NameString
	}
	return f.UserFriendlyNameString
}

func (f SourceFile) Code() string {
	return f.CodeString
}

type InMemorySource struct {
	NameString string
	CodeString string
}

func (s InMemorySource) Name() string {
	return s.NameString
}

func (s InMemorySource) UserFriendlyName() string {
	return s.NameString
}

func (s InMemorySource) Code() string {
	return s.CodeString
}

func ParseChunkSource(src ChunkSource) (*ParsedChunk, error) {
	runes, chunk, err := ParseChunk2(src.Code(), src.Name())

	if chunk == nil {
		return nil, err
	}

	return &ParsedChunk{
		Node:   chunk,
		Source: src,
		runes:  runes,
	}, err
}

func NewParsedChunk(node *Chunk, src ChunkSource) *ParsedChunk {
	return &ParsedChunk{
		Node:   node,
		Source: src,
	}
}

func (chunk *ParsedChunk) GetLineColumn(node Node) (int32, int32) {
	return chunk.GetSpanLineColumn(node.Base().Span)
}

func (chunk *ParsedChunk) FormatNodeSpanLocation(w io.Writer, nodeSpan NodeSpan) (int, error) {
	line, col := chunk.GetSpanLineColumn(nodeSpan)
	return fmt.Fprintf(w, "%s:%d:%d:", chunk.Name(), line, col)
}

func (chunk *ParsedChunk) GetFormattedNodeLocation(node Node) string {
	buf := bytes.NewBuffer(nil)
	chunk.FormatNodeSpanLocation(buf, node.Base().Span)
	return buf.String()
}

func (chunk *ParsedChunk) GetSpanLineColumn(span NodeSpan) (int32, int32) {
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

func (chunk *ParsedChunk) GetIncludedEndSpanLineColumn(span NodeSpan) (int32, int32) {
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

func (chunk *ParsedChunk) GetEndSpanLineColumn(span NodeSpan) (int32, int32) {
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

func (chunk *ParsedChunk) GetLineColumnSingeCharSpan(line, column int32) NodeSpan {
	pos := chunk.GetLineColumnPosition(line, column)
	return NodeSpan{
		Start: pos,
		End:   pos + 1,
	}
}

func (chunk *ParsedChunk) GetLineColumnPosition(line, column int32) int32 {
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

func (chunk *ParsedChunk) GetSourcePosition(span NodeSpan) SourcePositionRange {
	line, col := chunk.GetSpanLineColumn(span)
	endLine, endCol := chunk.GetEndSpanLineColumn(span)

	return SourcePositionRange{
		SourceName:  chunk.Name(),
		StartLine:   line,
		StartColumn: col,
		EndLine:     endLine,
		EndColumn:   endCol,
		Span:        span,
	}
}

// GetNodeAndChainAtSpan searches for the deepest node that includes the provided span.
// Spans of length 0 are supported, nodes whose exclusive range end is equal to the start of the provided span
// are ignored.
func (chunk *ParsedChunk) GetNodeAndChainAtSpan(target NodeSpan) (foundNode Node, ancestors []Node, ok bool) {

	Walk(chunk.Node, func(node, _, _ Node, chain []Node, _ bool) (TraversalAction, error) {
		span := node.Base().Span

		//if the cursor is not in the node's span we don't check the descendants of the node
		if span.Start >= target.End || span.End <= target.Start {
			return Prune, nil
		}

		if foundNode == nil || node.Base().IncludedIn(foundNode) {
			foundNode = node
			ancestors = chain
			ok = true
		}

		return ContinueTraversal, nil
	}, nil)

	return
}

// GetNodeAtSpan calls .GetNodeAndChainAtSpan and returns the found node.
func (chunk *ParsedChunk) GetNodeAtSpan(target NodeSpan) (foundNode Node, ok bool) {
	node, _, ok := chunk.GetNodeAndChainAtSpan(target)
	return node, ok
}

func (chunk *ParsedChunk) FindFirstStatementAndChainOnLine(line int) (foundNode Node, ancestors []Node, ok bool) {
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

	//eat leading space
	for i < length && isSpaceNotLF(runes[i]) {
		i++
	}

	if i < length && runes[i] == '\n' { //empty line
		return nil, nil, false
	}

	pos := i

	span := NodeSpan{
		Start: pos,
		End:   pos + 1,
	}
	node, ancestors, found := chunk.GetNodeAndChainAtSpan(span)
	if len(ancestors) == 0 || IsScopeContainerNode(node) {
		return nil, nil, false
	}

	if found {
		//search for closest statement

		for i := len(ancestors) - 1; i >= 0; i-- {
			ancestor := ancestors[i]
			switch ancestor.(type) {
			case *Block, *Chunk, *EmbeddedModule:

				var (
					stmt          Node
					stmtAncestors []Node
				)

				if i == len(ancestors)-1 {
					stmt = node
					stmtAncestors = ancestors
				} else {
					stmt = ancestors[i+1]
					stmtAncestors = ancestors[:i+1]
				}

				//if the statement does not start on the line we return false
				if stmt.Base().Span.Start != pos {
					return nil, nil, false
				}

				return stmt, stmtAncestors, true
			}
		}

		return nil, nil, false
	}

	return nil, nil, false
}

func (c *ParsedChunk) EstimatedIndentationUnit() string {
	return EstimateIndentationUnit(c.Runes(), c.Node)
}

type SourcePositionRange struct {
	SourceName  string   `json:"sourceName"`
	StartLine   int32    `json:"line"`
	StartColumn int32    `json:"column"`
	EndLine     int32    `json:"endLine"`
	EndColumn   int32    `json:"endColumn"`
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
	Chunk           *ParsedChunk
	CurrentNodeSpan NodeSpan //zero for the last item
}

func (i ChunkStackItem) GetChunk() (*ParsedChunk, bool) {
	return i.Chunk, i.Chunk != nil
}

func (i ChunkStackItem) GetCurrentNodeSpan() (NodeSpan, bool) {
	return i.CurrentNodeSpan, i.CurrentNodeSpan != (NodeSpan{})
}

type StackItem interface {
	GetChunk() (*ParsedChunk, bool)
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
