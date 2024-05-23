package parse

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

// ParsedChunkSource contains an AST and the ChunkSource that was parsed to obtain it.
// ParsedChunkSource provides helper methods to find nodes in the AST and to get positions.
type ParsedChunkSource struct {
	Node *ast.Chunk
	sourcecode.ParsedChunkSourceBase
}

func MustParseChunkSource(src ChunkSource, options ...ParserOptions) *ParsedChunkSource {
	return utils.Must(ParseChunkSource(src, options...))
}

// ParseChunkSource parses an Inox chunk. The returned error is either a non-syntax error or an aggregation of
// syntax errors (*ParsingErrorAggregation). On a critical error (nil, error) is returned.
//
// === Caching ===
// Contrary to ParseChunk, ParseChunkSource uses the cache provided  in the options. The cache is only used for
// *SourceFile sources. SourceFile.Location is used as the 'path' for the cache entry.
func ParseChunkSource(src ChunkSource, options ...ParserOptions) (parsed *ParsedChunkSource, resultErr error) {

	sourceCode := src.Code()
	sourceName := src.Name()

	//Check the cache if the code source is a file.
	var (
		cache            *ChunkCache
		resourceLocation string
	)
	if srcFile, ok := src.(SourceFile); ok && len(options) > 0 && options[0].ParsedFileCache != nil {
		cache = options[0].ParsedFileCache
		resourceLocation = srcFile.Resource
		parsedChunk, err, ok := cache.GetResultAndDataByPathSourcePair(resourceLocation, sourceCode)

		if ok {
			return parsedChunk, err
		}
	}

	runes, chunk, err := ParseChunk2(sourceCode, sourceName, options...)

	resultErr = err

	if chunk != nil { //No critical error.
		parsed = &ParsedChunkSource{
			Node:                  chunk,
			ParsedChunkSourceBase: sourcecode.MakeParsedChunkSourceBaseWithRunes(src, runes),
		}
	}

	//Update the cache.
	if cache != nil {
		cache.Put(resourceLocation, sourceCode, parsed, resultErr)
	}

	return
}

func NewParsedChunkSource(node *ast.Chunk, src ChunkSource) *ParsedChunkSource {
	return &ParsedChunkSource{
		Node: node,
		ParsedChunkSourceBase: sourcecode.ParsedChunkSourceBase{
			Source: src,
		},
	}
}

func (chunk *ParsedChunkSource) GetLineColumn(node ast.Node) (int32, int32) {
	return chunk.GetSpanLineColumn(node.Base().Span)
}

func (chunk *ParsedChunkSource) FormatNodeSpanLocation(w io.Writer, nodeSpan NodeSpan) (int, error) {
	line, col := chunk.GetSpanLineColumn(nodeSpan)
	return fmt.Fprintf(w, "%s:%d:%d:", chunk.Name(), line, col)
}

func (chunk *ParsedChunkSource) GetFormattedNodeLocation(node ast.Node) string {
	buf := bytes.NewBuffer(nil)
	chunk.FormatNodeSpanLocation(buf, node.Base().Span)
	return buf.String()
}

func (chunk *ParsedChunkSource) GetSpanLineColumn(span NodeSpan) (int32, int32) {
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

func (chunk *ParsedChunkSource) GetIncludedEndSpanLineColumn(span NodeSpan) (int32, int32) {
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

func (chunk *ParsedChunkSource) GetEndSpanLineColumn(span NodeSpan) (int32, int32) {
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

func (chunk *ParsedChunkSource) GetLineCut(cutIndex int32) (beforeSpan string, afterSpan string) {
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

func (chunk *ParsedChunkSource) GetLineCutWithTrimmedSpace(cutIndex int32) (beforeSpan string, afterSpan string) {
	beforeSpan, afterSpan = chunk.GetLineCut(cutIndex)
	beforeSpan = strings.TrimFunc(beforeSpan, isSpaceNotLF)
	afterSpan = strings.TrimFunc(afterSpan, isSpaceNotLF)
	return
}

func (chunk *ParsedChunkSource) GetLineColumnSingeCharSpan(line, column int32) NodeSpan {
	pos := chunk.GetLineColumnPosition(line, column)
	return NodeSpan{
		Start: pos,
		End:   pos + 1,
	}
}

func (chunk *ParsedChunkSource) GetLineColumnPosition(line, column int32) int32 {
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

func (chunk *ParsedChunkSource) GetSourcePosition(span NodeSpan) SourcePositionRange {
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
func (chunk *ParsedChunkSource) GetNodeAndChainAtSpan(target NodeSpan) (foundNode ast.Node, ancestors []ast.Node, ok bool) {

	ast.Walk(chunk.Node, func(node, _, _ ast.Node, chain []ast.Node, _ bool) (ast.TraversalAction, error) {
		span := node.Base().Span

		//if the cursor is not in the node's span we don't check the descendants of the node
		if span.Start >= target.End || span.End <= target.Start {
			return ast.Prune, nil
		}

		if foundNode == nil || node.Base().IncludedIn(foundNode) {
			foundNode = node
			ancestors = chain
			ok = true
		}

		return ast.ContinueTraversal, nil
	}, nil)

	return
}

// GetNodeAtSpan calls .GetNodeAndChainAtSpan and returns the found node.
func (chunk *ParsedChunkSource) GetNodeAtSpan(target NodeSpan) (foundNode ast.Node, ok bool) {
	node, _, ok := chunk.GetNodeAndChainAtSpan(target)
	return node, ok
}

func (chunk *ParsedChunkSource) FindFirstStatementAndChainOnLine(line int) (foundNode ast.Node, ancestors []ast.Node, ok bool) {
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
	if len(ancestors) == 0 || ast.IsScopeContainerNode(node) {
		return nil, nil, false
	}

	if found {
		//search for closest statement

		for i := len(ancestors) - 1; i >= 0; i-- {
			ancestor := ancestors[i]
			switch ancestor.(type) {
			case *ast.Block, *ast.Chunk, *ast.EmbeddedModule:

				var (
					stmt          ast.Node
					stmtAncestors []ast.Node
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

func (c *ParsedChunkSource) EstimatedIndentationUnit() string {
	return EstimateIndentationUnit(c.Runes(), c.Node)
}
