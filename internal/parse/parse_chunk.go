package parse

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"slices"
	"unicode"
)

func MustParseChunk(str string, opts ...ParserOptions) (result *Chunk) {
	n, err := ParseChunk(str, "<chunk>", opts...)
	if err != nil {
		panic(err)
	}
	return n
}

// parses an Inox file, resultErr is either a non-syntax error or an aggregation of syntax errors (*ParsingErrorAggregation).
// result and resultErr can be both non-nil at the same time because syntax errors are also stored in nodes.
func ParseChunk(str string, fpath string, opts ...ParserOptions) (result *Chunk, resultErr error) {
	_, result, resultErr = ParseChunk2(str, fpath, opts...)
	return
}

// ParseChunk2 has the same behavior as ParseChunk2 but returns the rune slice created for parsing.
func ParseChunk2(str string, fpath string, opts ...ParserOptions) (runes []rune, result *Chunk, resultErr error) {

	if int32(len(str)) > MAX_MODULE_BYTE_LEN {
		return nil, nil, &ParsingError{UnspecifiedParsingError, fmt.Sprintf("module'p.s code is too long (%d bytes)", len(str))}
	}

	//check that the passed context is not done.
	if len(opts) > 0 {
		ctx := opts[0].Context
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
			resultErr = fmt.Errorf("%s: %s", resultErr.Error(), debug.Stack())
		}

		if result != nil {
			//we walk the AST and adds each node'p.s error to resultErr

			var aggregation *ParsingErrorAggregation

			Walk(result, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
				if reflect.ValueOf(node).IsNil() {
					return ContinueTraversal, nil
				}

				nodeBase := node.Base()

				parsingErr := nodeBase.Err
				if parsingErr == nil {
					return ContinueTraversal, nil
				}

				if aggregation == nil {
					aggregation = &ParsingErrorAggregation{}
				}

				//add location in error message
				line := int32(1)
				col := int32(1)
				i := int32(0)

				for i < nodeBase.Span.Start {
					if p.s[i] == '\n' {
						line++
						col = 1
					} else {
						col++
					}

					i++
				}

				endLine := line
				endCol := col

				for i < nodeBase.Span.End {
					if p.s[i] == '\n' {
						endLine++
						endCol = 1
					} else {
						endCol++
					}
					i++
				}

				aggregation.Errors = append(aggregation.Errors, parsingErr)
				aggregation.ErrorPositions = append(aggregation.ErrorPositions, SourcePositionRange{
					SourceName:  fpath,
					StartLine:   line,
					StartColumn: col,
					EndLine:     endLine,
					EndColumn:   endCol,
					Span:        nodeBase.Span,
				})

				aggregation.Message = fmt.Sprintf("%s\n%s:%d:%d: %s", aggregation.Message, fpath, line, col, parsingErr.Message)
				resultErr = aggregation
				return ContinueTraversal, nil
			}, nil)
		}

	}()

	result, resultErr = p.parseChunk()
	return
}

func (p *parser) parseChunk() (*Chunk, error) {
	p.panicIfContextDone()

	chunk := &Chunk{
		NodeBase: NodeBase{
			Span: NodeSpan{Start: 0, End: p.len},
		},
		Statements: nil,
	}

	var (
		stmts []Node
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

	var preinit *PreinitStatement
	var manifest *Manifest

	if includableChunkDesc == nil {
		p.eatSpaceNewlineSemicolonComment()
		preinit = p.parsePreInitIfPresent()

		p.eatSpaceNewlineSemicolonComment()
		manifest = p.parseManifestIfPresent()
	}

	prevStmtEndIndex := int32(-1)
	var prevStmtErrKind ParsingErrorKind

	if p.onlyChunkStart {
		goto finalize_chunk_node
	}

	p.eatSpaceNewlineSemicolonComment()

	//parse statements

	for p.i < p.len {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {
			p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			stmts = append(stmts, &UnknownNode{
				NodeBase: NodeBase{
					Span: NodeSpan{p.i, p.i + 1},
					Err:  &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i])},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment()
			continue
		}

		var stmtErr *ParsingError

		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		stmt := p.parseStatement()
		prevStmtEndIndex = p.i

		if _, isMissingExpr := stmt.(*MissingExpression); isMissingExpr {
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
	chunk.Statements = stmts
	chunk.GlobalConstantDeclarations = globalConstDecls
	chunk.Tokens = p.tokens
	slices.SortFunc(chunk.Tokens, func(a, b Token) int {
		return int(a.Span.Start) - int(b.Span.Start)
	})

	return chunk, nil
}
