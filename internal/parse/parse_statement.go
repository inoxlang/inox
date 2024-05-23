package parse

import (
	"log"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

func (p *parser) parseStatement() ast.Node {
	// no p.panicIfContextDone() call because there is one in the following statement.

	expr, _ := p.parseExpression(exprParsingConfig{statement: true})

	var b rune
	followedBySpace := false
	isAKeyword := false

	switch e := expr.(type) {
	case *ast.IdentifierLiteral, *ast.IdentifierMemberExpression: //funcname <no args>
		if expr.Base().IsParenthesized {
			break
		}

		if idnt, isIdentLiteral := expr.(*ast.IdentifierLiteral); isIdentLiteral && isKeyword(idnt.Name) {
			isAKeyword = true
			break
		}

		prevI := p.i
		p.eatSpace()

		//function call with command-line syntax and no arguments
		if p.i < p.len && p.s[p.i] == ';' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.SEMICOLON, Span: NodeSpan{p.i, p.i + 1}})
			p.i++

			return &ast.CallExpression{
				NodeBase: ast.NodeBase{
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
	case *ast.MissingExpression:
		if p.i >= p.len {
			break
		}
		p.i++
		p.tokens = append(p.tokens, ast.Token{
			Type: ast.UNEXPECTED_CHAR,
			Raw:  string(p.s[p.i-1]),
			Span: NodeSpan{p.i - 1, p.i},
		})

		return &ast.UnknownNode{
			NodeBase: ast.NodeBase{
				NodeSpan{expr.Base().Span.Start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i-1])},
				false,
			},
		}
	case *ast.TestSuiteExpression:
		if expr.Base().IsParenthesized {
			break
		}

		e.IsStatement = true
	case *ast.TestCaseExpression:
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
	case *ast.CallExpression:
		return ev
	case *ast.IdentifierLiteral:
		switch ev.Name {
		case ast.ASSERT_KEYWORD_STRING:
			p.eatSpace()

			expr, _ := p.parseExpression()
			p.tokens = append(p.tokens, ast.Token{Type: ast.ASSERT_KEYWORD, Span: ev.Span})

			return &ast.AssertionStatement{
				NodeBase: ast.NodeBase{
					NodeSpan{ev.Span.Start, expr.Base().Span.End},
					nil,
					false,
				},
				Expr: expr,
			}
		case ast.IF_KEYWORD_STRING:
			return p.parseIfStatement(ev)
		case ast.FOR_KEYWORD_STRING:
			return p.parseForStatement(ev)
		case ast.WALK_KEYWORD_STRING:
			return p.parseWalkStatement(ev)
		case ast.SWITCH_KEYWORD_STRING, ast.MATCH_KEYWORD_STRING:
			return p.parseSwitchMatchStatement(ev)
		case ast.TOKEN_STRINGS[ast.FN_KEYWORD]:
			log.Panic("invalid state: function parsing should be hanlded by p.parseExpression")
			return nil
		case ast.TOKEN_STRINGS[ast.DROP_PERMS_KEYWORD]:
			return p.parsePermissionDroppingStatement(ev)
		case ast.TOKEN_STRINGS[ast.IMPORT_KEYWORD]:
			return p.parseImportStatement(ev)
		case ast.TOKEN_STRINGS[ast.RETURN_KEYWORD]:
			return p.parseReturnStatement(ev)
		case ast.COYIELD_KEYWORD_STRING:
			return p.parseCoyieldStatement(ev)
		case ast.YIELD_KEYWORD_STRING:
			return p.parseYieldStatement(ev)
		case ast.TOKEN_STRINGS[ast.BREAK_KEYWORD]:
			p.tokens = append(p.tokens, ast.Token{Type: ast.BREAK_KEYWORD, Span: ev.Span})
			return &ast.BreakStatement{
				NodeBase: ast.NodeBase{
					Span: ev.Span,
				},
				Label: nil,
			}
		case ast.TOKEN_STRINGS[ast.CONTINUE_KEYWORD]:
			p.tokens = append(p.tokens, ast.Token{Type: ast.CONTINUE_KEYWORD, Span: ev.Span})

			return &ast.ContinueStatement{
				NodeBase: ast.NodeBase{
					Span: ev.Span,
				},
				Label: nil,
			}
		case ast.TOKEN_STRINGS[ast.PRUNE_KEYWORD]:
			p.tokens = append(p.tokens, ast.Token{Type: ast.PRUNE_KEYWORD, Span: ev.Span})

			return &ast.PruneStatement{
				NodeBase: ast.NodeBase{
					Span: ev.Span,
				},
			}
		case ast.TOKEN_STRINGS[ast.ASSIGN_KEYWORD]:
			return p.parseMultiAssignmentStatement(ev)
		case ast.TOKEN_STRINGS[ast.VAR_KEYWORD]:
			return p.parseLocalVariableDeclarations(ev.Base())
		case ast.TOKEN_STRINGS[ast.GLOBALVAR_KEYWORD]:
			return p.parseGlobalVariableDeclarations(ev.Base())
		case ast.TOKEN_STRINGS[ast.SYNCHRONIZED_KEYWORD]:
			return p.parseSynchronizedBlock(ev)
		case ast.TOKEN_STRINGS[ast.PATTERN_KEYWORD]:
			return p.parsePatternDefinition(ev)
		case ast.TOKEN_STRINGS[ast.PNAMESPACE_KEYWORD]:
			return p.parsePatternNamespaceDefinition(ev)
		case ast.TOKEN_STRINGS[ast.EXTEND_KEYWORD]:
			return p.parseExtendStatement(ev)
		case ast.STRUCT_KEYWORD_STRING:
			return p.parseStructDefinition(ev)
		}

	}

	p.eatSpace()

	if p.i >= p.len {
		return expr
	}

	isAllowedCommandCallee := false

	switch expr.(type) {
	case *ast.IdentifierLiteral, *ast.IdentifierMemberExpression:
		isAllowedCommandCallee = true
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

		if isAllowedCommandCallee && followedBySpace && !expr.Base().IsParenthesized {
			return p.parseCommandLikeStatement(expr)
		}
	default:
		if expr.Base().IsParenthesized {
			break
		}

		if isAllowedCommandCallee {
			if !followedBySpace ||
				(isUnpairedOrIsClosingDelim(p.s[p.i]) && p.s[p.i] != '(' && p.s[p.i] != '|' && p.s[p.i] != '\n' && p.s[p.i] != ':') {
				break
			}
			return p.parseCommandLikeStatement(expr)
		}
	}
	return expr
}
