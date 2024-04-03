package parse

import "unicode"

func (p *parser) parseQuotedAndMetaStuff() (result Node, returnImmediately bool) {
	p.panicIfContextDone()

	start := p.i
	p.i++

	returnImmediately = true

	if p.i >= p.len {
		p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"})
		result = &UnknownNode{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
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

			metaIdent := &MetaIdentifier{
				NodeBase: NodeBase{Span: NodeSpan{start, p.i}},
				Name:     string(p.s[start+1 : p.i]),
			}

			if metaIdent.Name[len(metaIdent.Name)-1] == '-' {
				metaIdent.Err = &ParsingError{UnspecifiedParsingError, META_IDENTIFIER_MUST_NO_END_WITH_A_HYPHEN}
			}

			result = metaIdent
			returnImmediately = false
			return
		}

		p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"})

		result = &UnknownNode{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
			},
		}
		return
	}
}

func (p *parser) parseQuotedExpression() *QuotedExpression {
	start := p.i - 1
	p.tokens = append(p.tokens, Token{Type: AT_SIGN, Span: NodeSpan{start, start + 1}})

	// The opening parenthesis is not eaten because the expression is parsed as a parenthesized expression.

	var parsingErr *ParsingError

	if p.inQuotedRegion {
		parsingErr = &ParsingError{UnspecifiedParsingError, NESTED_QUOTED_REGIONS_NOT_ALLOWED}
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

	return &QuotedExpression{
		NodeBase: NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Expression: e,
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

	if p.inQuotedRegion {
		parsingErr = &ParsingError{UnspecifiedParsingError, NESTED_QUOTED_REGIONS_NOT_ALLOWED}
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

		annotations, moveForward := p.parseMetadaAnnotationsBeforeStatement(&stmts)

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
		p.tokens = append(p.tokens, Token{
			Type:    CLOSING_CURLY_BRACKET,
			SubType: QUOTED_STMTS_CLOSING_BRACE,
			Span:    NodeSpan{closingBraceIndex, closingBraceIndex + 1},
		})
		p.i++
	} else {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STATEMENTS_REGION_MISSING_CLOSING_DELIM}
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

func (p *parser) parseUnquotedRegion() *UnquotedRegion {
	p.panicIfContextDone()

	startIndex := p.i
	var parsingErr *ParsingError

	p.tokens = append(p.tokens, Token{
		Type: UNQUOTED_REGION_OPENING_DELIM,
		Span: NodeSpan{startIndex, p.i + 2},
	})

	p.i += 2

	//Eat '...' if present.

	spread := p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.'

	if spread {
		p.tokens = append(p.tokens, Token{Type: THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}})
		p.i += 3
	}

	if p.inQuotedRegion {
		if p.inUnquotedRegion {
			parsingErr = &ParsingError{UnspecifiedParsingError, NESTED_UNQUOTED_REGIONS_NOT_ALLOWED}
		} else {
			p.inUnquotedRegion = true
			defer func() {
				p.inUnquotedRegion = false
			}()
		}
	} else {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNQUOTED_REGIONS_ONLY_ALLOWED_INSIDE_QUOTED_REGIONS}
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
		p.tokens = append(p.tokens, Token{
			Type: UNQUOTED_REGION_CLOSING_DELIM,
			Span: NodeSpan{p.i, p.i + 2},
		})
		p.i += 2
	case p.i >= p.len:
		parsingErr = &ParsingError{UnterminatedUnquotedRegion, UNTERMINATED_UNQUOTED_REGION_MISSING_CLOSING_DELIM}
	default:
		parsingErr = &ParsingError{UnspecifiedParsingError, UNQUOTED_REGION_SHOULD_CONTAIN_A_SINGLE_EXPR}

		//Eat until EOF or '}>'
		extraStartIndex := p.i
		for p.i < p.len && (p.s[p.i] != '}' || (p.i < p.len-1 && p.s[p.i+1] != '>')) {
			p.i++
		}
		p.tokens = append(p.tokens, Token{
			Type: UNQUOTED_REGION_CLOSING_DELIM,
			Span: NodeSpan{extraStartIndex, p.i},
		})

		if p.i < p.len-1 && p.s[p.i] == '}' && p.s[p.i+1] == '>' {
			p.tokens = append(p.tokens, Token{
				Type: UNQUOTED_REGION_CLOSING_DELIM,
				Span: NodeSpan{p.i, p.i + 2},
			})
			p.i += 2
		}
	}

	return &UnquotedRegion{
		NodeBase: NodeBase{
			Span: NodeSpan{startIndex, p.i},
			Err:  parsingErr,
		},
		Spread:     spread,
		Expression: e,
	}
}
