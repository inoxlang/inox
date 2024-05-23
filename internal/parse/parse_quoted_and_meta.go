package parse

import (
	"unicode"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

func (p *parser) parseQuotedAndMetaStuff() (result ast.Node, returnImmediately bool) {
	p.panicIfContextDone()

	start := p.i
	p.i++

	returnImmediately = true

	if p.i >= p.len {
		p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"})
		result = &ast.UnknownNode{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
			},
		}
		return
	}

	switch p.s[p.i] {
	case '(':
		result = p.parseQuotedExpression()
		return
	case '{':
		result = p.parseQuotedStatements()
		return
	default:
		if IsFirstIdentChar(p.s[p.i]) {
			for p.i < p.len && IsIdentChar(p.s[p.i]) {
				p.i++
			}

			metaIdent := &ast.MetaIdentifier{
				NodeBase: ast.NodeBase{Span: NodeSpan{start, p.i}},
				Name:     string(p.s[start+1 : p.i]),
			}

			if metaIdent.Name[len(metaIdent.Name)-1] == '-' {
				metaIdent.Err = &sourcecode.ParsingError{UnspecifiedParsingError, META_IDENTIFIER_MUST_NO_END_WITH_A_HYPHEN}
			}

			result = metaIdent
			returnImmediately = false
			return
		}

		p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"})

		result = &ast.UnknownNode{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
			},
		}
		return
	}
}

func (p *parser) parseQuotedExpression() *ast.QuotedExpression {
	start := p.i - 1
	p.tokens = append(p.tokens, ast.Token{Type: ast.AT_SIGN, Span: NodeSpan{start, start + 1}})

	// The opening parenthesis is not eaten because the expression is parsed as a parenthesized expression.

	var parsingErr *sourcecode.ParsingError

	if p.inQuotedRegion {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, NESTED_QUOTED_REGIONS_NOT_ALLOWED}
	} else {
		p.inQuotedRegion = true
		defer func() {
			p.inQuotedRegion = false
		}()
	}

	if p.inPattern {
		p.inPattern = false
		defer func() {
			p.inPattern = true
		}()
	}

	e, _ := p.parseExpression()

	return &ast.QuotedExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Expression: e,
	}
}

func (p *parser) parseQuotedStatements() *ast.QuotedStatements {
	p.panicIfContextDone()

	openingBraceIndex := p.i
	startIndex := p.i - 1

	p.i++

	p.tokens = append(p.tokens, ast.Token{
		Type: ast.OPENING_QUOTED_STMTS_REGION_BRACE,
		Span: NodeSpan{startIndex, openingBraceIndex + 1},
	})

	var (
		prevStmtEndIndex = int32(-1)
		prevStmtErrKind  string

		parsingErr    *sourcecode.ParsingError
		stmts         []ast.Node
		regionHeaders []*ast.AnnotatedRegionHeader
	)

	if p.inQuotedRegion {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, NESTED_QUOTED_REGIONS_NOT_ALLOWED}
	} else {
		p.inQuotedRegion = true
		defer func() {
			p.inQuotedRegion = false
		}()
	}

	if p.inPattern {
		p.inPattern = false
		defer func() {
			p.inPattern = true
		}()
	}

	//Parse statements.

	p.eatSpaceNewlineSemicolonComment()

	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {

			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})

			stmts = append(stmts, &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, p.i + 1},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInQuotedStatements(p.s[p.i])},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment()
			continue
		}

		var stmtErr *sourcecode.ParsingError

		if p.i >= p.len || p.s[p.i] == '}' {
			break
		}

		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &sourcecode.ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		annotations, moveForward := p.parseMetadaAnnotationsBeforeStatement(&stmts, &regionHeaders)

		if !moveForward {
			break
		}

		stmt := p.parseStatement()

		prevStmtEndIndex = p.i
		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.Kind
		}

		if stmtErr != nil && (stmt.Base().Err == nil || stmt.Base().Err.Kind != InvalidNext) {
			stmt.BasePtr().Err = stmtErr
		}

		if missingStmt := p.addAnnotationsToNodeIfPossible(annotations, stmt); missingStmt != nil {
			stmts = append(stmts, missingStmt)
		}
		stmts = append(stmts, stmt)

		p.eatSpaceNewlineSemicolonComment()
	}

	//Parse closing delimiter.

	closingBraceIndex := p.i

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{
			Type:    ast.CLOSING_CURLY_BRACKET,
			SubType: ast.QUOTED_STMTS_CLOSING_BRACE,
			Span:    NodeSpan{closingBraceIndex, closingBraceIndex + 1},
		})
		p.i++
	} else {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STATEMENTS_REGION_MISSING_CLOSING_DELIM}
	}

	end := p.i

	return &ast.QuotedStatements{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{startIndex, end},
			Err:  parsingErr,
		},
		RegionHeaders: regionHeaders,
		Statements:    stmts,
	}

}

func (p *parser) parseUnquotedRegion() *ast.UnquotedRegion {
	p.panicIfContextDone()

	startIndex := p.i
	var parsingErr *sourcecode.ParsingError

	p.tokens = append(p.tokens, ast.Token{
		Type: ast.UNQUOTED_REGION_OPENING_DELIM,
		Span: NodeSpan{startIndex, p.i + 2},
	})

	p.i += 2

	//Eat '...' if present.

	spread := p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.'

	if spread {
		p.tokens = append(p.tokens, ast.Token{Type: ast.THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}})
		p.i += 3
	}

	if p.inQuotedRegion {
		if p.inUnquotedRegion {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, NESTED_UNQUOTED_REGIONS_NOT_ALLOWED}
		} else {
			p.inUnquotedRegion = true
			defer func() {
				p.inUnquotedRegion = false
			}()
		}
	} else {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNQUOTED_REGIONS_ONLY_ALLOWED_INSIDE_QUOTED_REGIONS}
	}

	if p.inPattern {
		p.inPattern = false
		defer func() {
			p.inPattern = true
		}()
	}

	//Parse the expression.

	p.eatSpaceNewlineComment()

	e, _ := p.parseExpression()

	p.eatSpaceNewlineComment()

	switch {
	case p.i < p.len-1 && p.s[p.i] == '}' && p.s[p.i+1] == '>':
		p.tokens = append(p.tokens, ast.Token{
			Type: ast.UNQUOTED_REGION_CLOSING_DELIM,
			Span: NodeSpan{p.i, p.i + 2},
		})
		p.i += 2
	case p.i >= p.len:
		parsingErr = &sourcecode.ParsingError{UnterminatedUnquotedRegion, UNTERMINATED_UNQUOTED_REGION_MISSING_CLOSING_DELIM}
	default:
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNQUOTED_REGION_SHOULD_CONTAIN_A_SINGLE_EXPR}

		//Eat until EOF or '}>'
		extraStartIndex := p.i
		for p.i < p.len && (p.s[p.i] != '}' || (p.i < p.len-1 && p.s[p.i+1] != '>')) {
			p.i++
		}
		p.tokens = append(p.tokens, ast.Token{
			Type: ast.UNQUOTED_REGION_CLOSING_DELIM,
			Span: NodeSpan{extraStartIndex, p.i},
		})

		if p.i < p.len-1 && p.s[p.i] == '}' && p.s[p.i+1] == '>' {
			p.tokens = append(p.tokens, ast.Token{
				Type: ast.UNQUOTED_REGION_CLOSING_DELIM,
				Span: NodeSpan{p.i, p.i + 2},
			})
			p.i += 2
		}
	}

	return &ast.UnquotedRegion{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{startIndex, p.i},
			Err:  parsingErr,
		},
		Spread:     spread,
		Expression: e,
	}
}
