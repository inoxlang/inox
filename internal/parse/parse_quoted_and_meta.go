package parse

import "unicode"

func (p *parser) parseQuotedAndMetaStuff() Node {
	p.panicIfContextDone()

	start := p.i
	p.i++

	if p.i >= p.len {
		p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"})
		return &UnknownNode{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
			},
		}
	}

	switch p.s[p.i] {
	case '(': //lazy expression
		p.tokens = append(p.tokens, Token{Type: AT_SIGN, Span: NodeSpan{start, start + 1}})

		// The opening parenthesis is not eaten because the expression is parsed as a parenthesized expression.

		e, _ := p.parseExpression()

		return &QuotedExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Expression: e,
		}
	case '{':
		return p.parseQuotedStatements()
	default:
		// if IsFirstIdentChar(p.s[p.i]) {
		// 	j := p.i
		// 	p.i--

		// 	for j < p.len && IsIdentChar(p.s[j]) {
		// 		j++
		// 	}

		// 	for j < p.len && isSpaceNotLF(p.s[j]) {
		// 		j++
		// 	}
		// }

		// p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"})

		return &UnknownNode{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
			},
		}
	}
}

func (p *parser) parseQuotedStatements() *QuotedStatements {
	p.panicIfContextDone()

	openingBraceIndex := p.i
	startIndex := p.i - 1

	p.i++

	p.tokens = append(p.tokens, Token{
		Type: OPENING_QUOTED_STMTS_REGION_BRACE,
		Span: NodeSpan{startIndex, openingBraceIndex + 1},
	})

	var (
		prevStmtEndIndex = int32(-1)
		prevStmtErrKind  ParsingErrorKind

		parsingErr *ParsingError
		stmts      []Node
	)

	p.eatSpaceNewlineSemicolonComment()

	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {

			p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})

			stmts = append(stmts, &UnknownNode{
				NodeBase: NodeBase{
					Span: NodeSpan{p.i, p.i + 1},
					Err:  &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInQuotedStatements(p.s[p.i])},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment()
			continue
		}

		var stmtErr *ParsingError

		if p.i >= p.len || p.s[p.i] == '}' {
			break
		}

		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		stmt := p.parseStatement()

		prevStmtEndIndex = p.i
		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.Kind
		}

		if stmtErr != nil && (stmt.Base().Err == nil || stmt.Base().Err.Kind != InvalidNext) {
			stmt.BasePtr().Err = stmtErr
		}

		stmts = append(stmts, stmt)
		p.eatSpaceNewlineSemicolonComment()
	}

	closingBraceIndex := p.i

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, Token{
			Type:    CLOSING_CURLY_BRACKET,
			SubType: QUOTED_STMTS_CLOSING_BRACE,
			Span:    NodeSpan{closingBraceIndex, closingBraceIndex + 1},
		})
		p.i++
	} else {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STATEMENTS_REGION_MISSING_CLOSING_BRACE}
	}

	end := p.i

	return &QuotedStatements{
		NodeBase: NodeBase{
			Span: NodeSpan{startIndex, end},
			Err:  parsingErr,
		},
		Statements: stmts,
	}

}
