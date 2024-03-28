package parse

import "log"

func (p *parser) parseStatement() Node {
	// no p.panicIfContextDone() call because there is one in the following statement.

	expr, _ := p.parseExpression(exprParsingConfig{statement: true})

	var b rune
	followedBySpace := false
	isAKeyword := false

	switch e := expr.(type) {
	case *IdentifierLiteral, *IdentifierMemberExpression: //funcname <no args>
		if expr.Base().IsParenthesized {
			break
		}

		if idnt, isIdentLiteral := expr.(*IdentifierLiteral); isIdentLiteral && isKeyword(idnt.Name) {
			isAKeyword = true
			break
		}

		prevI := p.i
		p.eatSpace()

		//function call with command-line syntax and no arguments
		if p.i < p.len && p.s[p.i] == ';' {
			if p.i < p.len {
				p.i++
			}
			return &CallExpression{
				NodeBase: NodeBase{
					Span: NodeSpan{expr.Base().Span.Start, p.i},
				},
				Callee:            expr,
				Arguments:         nil,
				Must:              true,
				CommandLikeSyntax: true,
			}
		} else {
			p.i = prevI
		}
	case *MissingExpression:
		if p.i >= p.len {
			break
		}
		p.i++
		p.tokens = append(p.tokens, Token{
			Type: UNEXPECTED_CHAR,
			Raw:  string(p.s[p.i-1]),
			Span: NodeSpan{p.i - 1, p.i},
		})

		return &UnknownNode{
			NodeBase: NodeBase{
				NodeSpan{expr.Base().Span.Start, p.i},
				&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i-1])},
				false,
			},
		}
	case *TestSuiteExpression:
		if expr.Base().IsParenthesized {
			break
		}

		e.IsStatement = true
	case *TestCaseExpression:
		if expr.Base().IsParenthesized {
			break
		}

		e.IsStatement = true
	}

	if p.i >= p.len {
		if !isAKeyword {
			return expr
		}
	} else {
		b = p.s[p.i]
		followedBySpace = b == ' '
	}

	switch ev := expr.(type) {
	case *CallExpression:
		return ev
	case *IdentifierLiteral:
		switch ev.Name {
		case ASSERT_KEYWORD_STRING:
			p.eatSpace()

			expr, _ := p.parseExpression()
			p.tokens = append(p.tokens, Token{Type: ASSERT_KEYWORD, Span: ev.Span})

			return &AssertionStatement{
				NodeBase: NodeBase{
					NodeSpan{ev.Span.Start, expr.Base().Span.End},
					nil,
					false,
				},
				Expr: expr,
			}
		case IF_KEYWORD_STRING:
			return p.parseIfStatement(ev)
		case FOR_KEYWORD_STRING:
			return p.parseForStatement(ev)
		case tokenStrings[WALK_KEYWORD]:
			return p.parseWalkStatement(ev)
		case SWITCH_KEYWORD_STRING, MATCH_KEYWORD_STRING:
			return p.parseSwitchMatchStatement(ev)
		case tokenStrings[FN_KEYWORD]:
			log.Panic("invalid state: function parsing should be hanlded by p.parseExpression")
			return nil
		case tokenStrings[DROP_PERMS_KEYWORD]:
			return p.parsePermissionDroppingStatement(ev)
		case tokenStrings[IMPORT_KEYWORD]:
			return p.parseImportStatement(ev)
		case tokenStrings[RETURN_KEYWORD]:
			return p.parseReturnStatement(ev)
		case COYIELD_KEYWORD_STRING:
			return p.parseCoyieldStatement(ev)
		case YIELD_KEYWORD_STRING:
			return p.parseYieldStatement(ev)
		case tokenStrings[BREAK_KEYWORD]:
			p.tokens = append(p.tokens, Token{Type: BREAK_KEYWORD, Span: ev.Span})
			return &BreakStatement{
				NodeBase: NodeBase{
					Span: ev.Span,
				},
				Label: nil,
			}
		case tokenStrings[CONTINUE_KEYWORD]:
			p.tokens = append(p.tokens, Token{Type: CONTINUE_KEYWORD, Span: ev.Span})

			return &ContinueStatement{
				NodeBase: NodeBase{
					Span: ev.Span,
				},
				Label: nil,
			}
		case tokenStrings[PRUNE_KEYWORD]:
			p.tokens = append(p.tokens, Token{Type: PRUNE_KEYWORD, Span: ev.Span})

			return &PruneStatement{
				NodeBase: NodeBase{
					Span: ev.Span,
				},
			}
		case tokenStrings[ASSIGN_KEYWORD]:
			return p.parseMultiAssignmentStatement(ev)
		case tokenStrings[VAR_KEYWORD]:
			return p.parseLocalVariableDeclarations(ev.Base())
		case tokenStrings[GLOBALVAR_KEYWORD]:
			return p.parseGlobalVariableDeclarations(ev.Base())
		case tokenStrings[SYNCHRONIZED_KEYWORD]:
			return p.parseSynchronizedBlock(ev)
		case tokenStrings[PATTERN_KEYWORD]:
			return p.parsePatternDefinition(ev)
		case tokenStrings[PNAMESPACE_KEYWORD]:
			return p.parsePatternNamespaceDefinition(ev)
		case tokenStrings[EXTEND_KEYWORD]:
			return p.parseExtendStatement(ev)
		case STRUCT_KEYWORD_STRING:
			return p.parseStructDefinition(ev)
		}

	}

	p.eatSpace()

	if p.i >= p.len {
		return expr
	}

	switch p.s[p.i] {
	case '=': //assignment
		return p.parseAssignment(expr)
	case ';':
		return expr
	case '+', '-', '*', '/':
		if p.i < p.len-1 && p.s[p.i+1] == '=' {
			return p.parseAssignment(expr)
		}

		if followedBySpace && !expr.Base().IsParenthesized {
			return p.parseCommandLikeStatement(expr)
		}
	default:
		if expr.Base().IsParenthesized {
			break
		}

		switch expr.(type) {
		case *IdentifierLiteral, *IdentifierMemberExpression:
			if !followedBySpace ||
				(isUnpairedOrIsClosingDelim(p.s[p.i]) && p.s[p.i] != '(' && p.s[p.i] != '|' && p.s[p.i] != '\n' && p.s[p.i] != ':') {
				break
			}
			return p.parseCommandLikeStatement(expr)
		}
	}
	return expr
}
