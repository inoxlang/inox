package parse

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"slices"
	"unicode"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

func MustParseChunk(str string, opts ...ParserOptions) (result *ast.Chunk) {
	n, err := ParseChunk(str, "<chunk>", opts...)
	if err != nil {
		panic(err)
	}
	return n
}

// parses an Inox file, resultErr is either a non-syntax error or an aggregation of syntax errors (*ParsingErrorAggregation).
// result and resultErr can be both non-nil at the same time because syntax errors are also stored in nodes. The returned
// *ast.Chunk does not contain references to the source's location, but $resultErr may. ParseChunk does not use the cache provided
// in the options.
func ParseChunk(str string, sourceName string, opts ...ParserOptions) (result *ast.Chunk, resultErr error) {
	_, result, resultErr = ParseChunk2(str, sourceName, opts...)
	return
}

// ParseChunk2 has the same behavior as ParseChunk2 but also returns the rune slice created for parsing. The rune slice should not be modified.
// The returned *ast.Chunk does not contain references to the source's location, but $resultErr may. ParseChunk2 does not use the cache provided
// in the options.
func ParseChunk2(str string, sourceName string, opts ...ParserOptions) (runes []rune, result *ast.Chunk, resultErr error) {

	if int32(len(str)) > MAX_MODULE_BYTE_LEN {
		return nil, nil, &sourcecode.ParsingError{UnspecifiedParsingError, fmt.Sprintf("module'p.s code is too long (%d bytes)", len(str))}
	}

	if len(opts) > 0 {
		//Check that the passed context is not done.
		ctx := opts[0].ParentContext
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			default:
			}
		}
	}

	runes = []rune(str)
	p := newParser(runes, opts...)
	defer p.cancel()

	defer func() {
		v := recover()
		if err, ok := v.(error); ok {
			resultErr = err
		}

		if resultErr != nil {
			resultErr = fmt.Errorf("%w: %s", resultErr, debug.Stack())
		}

		if result != nil {
			//we walk the AST and adds each node'p.s error to resultErr

			var aggregation *sourcecode.ParsingErrorAggregation

			ast.Walk(result, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) (ast.TraversalAction, error) {
				if reflect.ValueOf(node).IsNil() {
					return ast.ContinueTraversal, nil
				}

				//Regular parsing error.

				nodeBase := node.Base()

				parsingErr := nodeBase.Err
				if parsingErr == nil {
					return ast.ContinueTraversal, nil
				}

				if aggregation == nil {
					aggregation = &sourcecode.ParsingErrorAggregation{}
					resultErr = aggregation
				}

				startLine, endLine, startCol, endCol := getLineColumns(p.s, nodeBase.Span.Start, nodeBase.Span.End)

				aggregation.Errors = append(aggregation.Errors, parsingErr)
				aggregation.ErrorPositions = append(aggregation.ErrorPositions, SourcePositionRange{
					SourceName:  sourceName,
					StartLine:   startLine,
					StartColumn: startCol,
					EndLine:     endLine,
					EndColumn:   endCol,
					Span:        nodeBase.Span,
				})

				aggregation.Message = fmt.Sprintf("%s\n%s:%d:%d: %s", aggregation.Message, sourceName, startLine, startCol, parsingErr.Message)
				return ast.ContinueTraversal, nil
			}, nil)
		}
	}()

	result, resultErr = p.parseChunk()
	return
}

func (p *parser) parseChunk() (*ast.Chunk, error) {
	p.panicIfContextDone()

	chunk := &ast.Chunk{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{Start: 0, End: p.len},
		},
		Statements: nil,
	}

	var (
		stmts         []ast.Node
		regionHeaders []*ast.AnnotatedRegionHeader
	)

	//shebang
	if p.i < p.len-1 && p.s[0] == '#' && p.s[1] == '!' {
		for p.i < p.len && p.s[p.i] != '\n' {
			p.i++
		}
	}

	p.eatSpaceNewlineSemicolonComment()
	includableChunkDesc := p.parseIncludaleChunkDescIfPresent()

	p.eatSpaceNewlineSemicolonComment()
	globalConstDecls := p.parseGlobalConstantDeclarations()

	var preinit *ast.PreinitStatement
	var manifest *ast.Manifest

	if includableChunkDesc == nil {
		p.eatSpaceNewlineSemicolonComment()
		preinit = p.parsePreInitIfPresent()

		p.eatSpaceNewlineSemicolonComment()
		manifest = p.parseManifestIfPresent()
	}

	prevStmtEndIndex := int32(-1)
	var prevStmtErrKind string

	if p.onlyChunkStart {
		goto finalize_chunk_node
	}

	p.eatSpaceNewlineSemicolonComment()

	//parse statements

	for p.i < p.len {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			stmts = append(stmts, &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, p.i + 1},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i])},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment()
			continue
		}

		var stmtErr *sourcecode.ParsingError

		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &sourcecode.ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		annotations, moveForward := p.parseMetadaAnnotationsBeforeStatement(&stmts, &regionHeaders)

		if !moveForward {
			break
		}

		stmt := p.parseStatement()
		prevStmtEndIndex = p.i

		if missingStmt := p.addAnnotationsToNodeIfPossible(annotations, stmt); missingStmt != nil {
			stmts = append(stmts, missingStmt)
		}

		if _, isMissingExpr := stmt.(*ast.MissingExpression); isMissingExpr {
			stmts = append(stmts, stmt)
			break
		}

		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.Kind
		} else if stmtErr != nil {
			stmt.BasePtr().Err = stmtErr
		}
		stmts = append(stmts, stmt)

		p.eatSpaceNewlineSemicolonComment()
	}

finalize_chunk_node:

	chunk.Preinit = preinit
	chunk.Manifest = manifest
	chunk.IncludableChunkDesc = includableChunkDesc
	chunk.RegionHeaders = regionHeaders
	chunk.Statements = stmts
	chunk.GlobalConstantDeclarations = globalConstDecls
	chunk.Tokens = p.tokens
	slices.SortFunc(chunk.Tokens, func(a, b ast.Token) int {
		return int(a.Span.Start) - int(b.Span.Start)
	})

	return chunk, nil
}

func getLineColumns(s []rune, start, end int32) (startLine, endLine int32, startColumn, endColumn int32) {
	//add location in error message
	startLine = int32(1)
	startColumn = int32(1)
	i := int32(0)

	for i < start {
		if s[i] == '\n' {
			startLine++
			startColumn = 1
		} else {
			startColumn++
		}

		i++
	}

	endLine = startLine
	endColumn = startColumn

	for i < end {
		if s[i] == '\n' {
			endLine++
			endColumn = 1
		} else {
			endColumn++
		}
		i++
	}

	return
}
