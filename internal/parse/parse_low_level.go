package parse

import (
	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

func (p *parser) parseStructDefinition(extendIdent *ast.IdentifierLiteral) *ast.StructDefinition {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, ast.Token{Type: ast.STRUCT_KEYWORD, Span: extendIdent.Span})

	p.eatSpace()

	def := &ast.StructDefinition{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{extendIdent.Span.Start, p.i},
		},
	}

	if p.i >= p.len || !isAlphaOrUndescore(p.s[p.i]) {
		def.Err = &sourcecode.ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_DEF_MISSING_NAME_AFTER_KEYWORD}
		return def
	}

	name := &ast.PatternIdentifierLiteral{
		NodeBase:   ast.NodeBase{Span: NodeSpan{p.i, p.i + 1}},
		Unprefixed: true,
	}

	nameStart := p.i
	for p.i < p.len && isAlphaOrUndescore(p.s[p.i]) {
		p.i++
		name.Span.End = p.i
	}

	name.Name = string(p.s[nameStart:p.i])

	def.Name = name
	def.NodeBase.Span.End = def.Name.Base().Span.End

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '{' {
		def.Err = &sourcecode.ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_DEF_MISSING_BODY}
		return def
	}

	//parse body

	body := &ast.StructBody{
		NodeBase: ast.NodeBase{Span: NodeSpan{Start: p.i}},
	}
	def.Body = body

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	p.eatSpaceNewlineSemicolonComment()

	for p.i < p.len && p.s[p.i] != '}' {
		expr, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		if isMissingExpr {
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			expr = &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i, p.i + 1},
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInStructBody(p.s[p.i])},
					false,
				},
			}
			p.i++

			end := p.i
			body.NodeBase.Span.End = end
			def.NodeBase.Span.End = end

			body.Definitions = append(body.Definitions, expr)

			p.eatSpaceNewlineSemicolonComment()
			continue
		}

		switch e := expr.(type) {
		case *ast.IdentifierLiteral: //field name
			fieldDef := &ast.StructFieldDefinition{
				Name:     e,
				NodeBase: e.NodeBase,
			}

			p.eatSpace()

			//parse field type

			func() {
				prev := p.inPattern
				p.inPattern = true
				defer func() {
					p.inPattern = prev
				}()

				fieldDef.Type, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
				fieldDef.Span.End = p.i
			}()

			body.Definitions = append(body.Definitions, fieldDef)
		case *ast.FunctionDeclaration: //method
			body.Definitions = append(body.Definitions, expr)
		default:
			body.Definitions = append(body.Definitions, expr)
			basePtr := expr.BasePtr()
			if basePtr.Err == nil {
				basePtr.Err = &sourcecode.ParsingError{UnspecifiedParsingError, ONLY_FIELD_AND_METHOD_DEFINITIONS_ARE_ALLOWED_IN_STRUCT_BODY}
			}
		}

		body.NodeBase.Span.End = p.i
		def.NodeBase.Span.End = p.i
		p.eatSpaceNewlineSemicolonComment()
	}

	if p.i >= p.len || p.s[p.i] != '}' {
		def.Err = &sourcecode.ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_BODY_MISSING_CLOSING_BRACE}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	body.NodeBase.Span.End = p.i
	def.NodeBase.Span.End = p.i

	return def
}

func (p *parser) parseNewExpression(newIdent *ast.IdentifierLiteral) *ast.NewExpression {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, ast.Token{Type: ast.NEW_KEYWORD, Span: newIdent.Span})

	p.eatSpace()

	newExpr := &ast.NewExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{newIdent.Span.Start, p.i},
		},
	}

	if p.i >= p.len || !isAlphaOrUndescore(p.s[p.i]) {
		newExpr.Err = &sourcecode.ParsingError{UnterminatedStructDefinition, UNTERMINATED_NEW_EXPR_MISSING_TYPE_AFTER_KEYWORD}
		return newExpr
	}

	//parse type
	{
		inPatternSave := p.inPattern
		p.inPattern = true

		newExpr.Type, _ = p.parseExpression()
		newExpr.NodeBase.Span.End = newExpr.Type.Base().Span.End

		p.inPattern = inPatternSave
	}

	p.eatSpace()

	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		//no body
		return newExpr
	}

	if p.s[p.i] != '{' {
		newExpr.Initialization, _ = p.parseExpression()
	} else {
		newExpr.Initialization = p.parseStructInitializationLiteral()
	}

	newExpr.NodeBase.Span.End = p.i

	return newExpr
}

func (p *parser) parseStructInitializationLiteral() *ast.StructInitializationLiteral {
	structInit := &ast.StructInitializationLiteral{
		NodeBase: ast.NodeBase{Span: NodeSpan{p.i, p.i + 1}},
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	p.eatSpaceNewlineCommaComment()

loop:
	for p.i < p.len && p.s[p.i] != '}' {
		expr, isMissingExpr := p.parseExpression()

		if isMissingExpr {
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			expr = &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i, p.i + 1},
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInStructInitLiteral(p.s[p.i])},
					false,
				},
			}
			p.i++

			end := p.i
			structInit.NodeBase.Span.End = end
			structInit.Fields = append(structInit.Fields, expr)

			p.eatSpaceNewlineCommaComment()
			continue
		}

		switch e := expr.(type) {
		case *ast.IdentifierLiteral: //field name
			fieldInit := &ast.StructFieldInitialization{
				Name:     e,
				NodeBase: e.NodeBase,
			}
			structInit.Fields = append(structInit.Fields, fieldInit)

			p.eatSpace()

			if p.i >= p.len || p.s[p.i] == '}' {
				break loop
			}

			if p.s[p.i] == ':' {
				p.tokens = append(p.tokens, ast.Token{Type: ast.COLON, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
				p.i++

				p.eatSpace()

				fieldInit.Value, _ = p.parseExpression()
				fieldInit.Span.End = p.i
			} else {
				fieldInit.Err = &sourcecode.ParsingError{UnspecifiedParsingError, MISSING_COLON_AFTER_FIELD_NAME}
			}
		default:
			structInit.Fields = append(structInit.Fields, expr)
			basePtr := expr.BasePtr()
			if basePtr.Err == nil {
				basePtr.Err = &sourcecode.ParsingError{UnspecifiedParsingError, ONLY_FIELD_INIT_PAIRS_ALLOWED}
			}
		}

		structInit.NodeBase.Span.End = p.i
		p.eatSpaceNewlineCommaComment()
	}

	if p.i >= p.len || p.s[p.i] != '}' {
		structInit.Err = &sourcecode.ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_INIT_LIT_MISSING_CLOSING_BRACE}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}
	structInit.NodeBase.Span.End = p.i

	return structInit
}
