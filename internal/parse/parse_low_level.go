package parse

func (p *parser) parseStructDefinition(extendIdent *IdentifierLiteral) *StructDefinition {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, Token{Type: STRUCT_KEYWORD, Span: extendIdent.Span})

	p.eatSpace()

	def := &StructDefinition{
		NodeBase: NodeBase{
			Span: NodeSpan{extendIdent.Span.Start, p.i},
		},
	}

	if p.i >= p.len || !isAlphaOrUndescore(p.s[p.i]) {
		def.Err = &ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_DEF_MISSING_NAME_AFTER_KEYWORD}
		return def
	}

	name := &PatternIdentifierLiteral{
		NodeBase:   NodeBase{Span: NodeSpan{p.i, p.i + 1}},
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
		def.Err = &ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_DEF_MISSING_BODY}
		return def
	}

	//parse body

	body := &StructBody{
		NodeBase: NodeBase{Span: NodeSpan{Start: p.i}},
	}
	def.Body = body

	p.tokens = append(p.tokens, Token{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	p.eatSpaceNewlineSemicolonComment()

	for p.i < p.len && p.s[p.i] != '}' {
		expr, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForExpr: true})

		if isMissingExpr {
			p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			expr = &UnknownNode{
				NodeBase: NodeBase{
					NodeSpan{p.i, p.i + 1},
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInStructBody(p.s[p.i])},
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
		case *IdentifierLiteral: //field name
			fieldDef := &StructFieldDefinition{
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

				fieldDef.Type, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForExpr: true})
				fieldDef.Span.End = p.i
			}()

			body.Definitions = append(body.Definitions, fieldDef)
		case *FunctionDeclaration: //method
			body.Definitions = append(body.Definitions, expr)
		default:
			body.Definitions = append(body.Definitions, expr)
			basePtr := expr.BasePtr()
			if basePtr.Err == nil {
				basePtr.Err = &ParsingError{UnspecifiedParsingError, ONLY_FIELD_AND_METHOD_DEFINITIONS_ARE_ALLOWED_IN_STRUCT_BODY}
			}
		}

		body.NodeBase.Span.End = p.i
		def.NodeBase.Span.End = p.i
		p.eatSpaceNewlineSemicolonComment()
	}

	if p.i >= p.len || p.s[p.i] != '}' {
		def.Err = &ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_BODY_MISSING_CLOSING_BRACE}
	} else {
		p.tokens = append(p.tokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	body.NodeBase.Span.End = p.i
	def.NodeBase.Span.End = p.i

	return def
}

func (p *parser) parseNewExpression(newIdent *IdentifierLiteral) *NewExpression {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, Token{Type: NEW_KEYWORD, Span: newIdent.Span})

	p.eatSpace()

	newExpr := &NewExpression{
		NodeBase: NodeBase{
			Span: NodeSpan{newIdent.Span.Start, p.i},
		},
	}

	if p.i >= p.len || !isAlphaOrUndescore(p.s[p.i]) {
		newExpr.Err = &ParsingError{UnterminatedStructDefinition, UNTERMINATED_NEW_EXPR_MISSING_TYPE_AFTER_KEYWORD}
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

func (p *parser) parseStructInitializationLiteral() *StructInitializationLiteral {
	structInit := &StructInitializationLiteral{
		NodeBase: NodeBase{Span: NodeSpan{p.i, p.i + 1}},
	}

	p.tokens = append(p.tokens, Token{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	p.eatSpaceNewlineCommaComment()

loop:
	for p.i < p.len && p.s[p.i] != '}' {
		expr, isMissingExpr := p.parseExpression()

		if isMissingExpr {
			p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			expr = &UnknownNode{
				NodeBase: NodeBase{
					NodeSpan{p.i, p.i + 1},
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInStructInitLiteral(p.s[p.i])},
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
		case *IdentifierLiteral: //field name
			fieldInit := &StructFieldInitialization{
				Name:     e,
				NodeBase: e.NodeBase,
			}
			structInit.Fields = append(structInit.Fields, fieldInit)

			p.eatSpace()

			if p.i >= p.len || p.s[p.i] == '}' {
				break loop
			}

			if p.s[p.i] == ':' {
				p.tokens = append(p.tokens, Token{Type: COLON, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
				p.i++

				p.eatSpace()

				fieldInit.Value, _ = p.parseExpression()
				fieldInit.Span.End = p.i
			} else {
				fieldInit.Err = &ParsingError{UnspecifiedParsingError, MISSING_COLON_AFTER_FIELD_NAME}
			}
		default:
			structInit.Fields = append(structInit.Fields, expr)
			basePtr := expr.BasePtr()
			if basePtr.Err == nil {
				basePtr.Err = &ParsingError{UnspecifiedParsingError, ONLY_FIELD_INIT_PAIRS_ALLOWED}
			}
		}

		structInit.NodeBase.Span.End = p.i
		p.eatSpaceNewlineCommaComment()
	}

	if p.i >= p.len || p.s[p.i] != '}' {
		structInit.Err = &ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_INIT_LIT_MISSING_CLOSING_BRACE}
	} else {
		p.tokens = append(p.tokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}
	structInit.NodeBase.Span.End = p.i

	return structInit
}
