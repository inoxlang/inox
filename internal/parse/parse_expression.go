package parse

type exprParsingConfig struct {
	precededByOpeningParen                  bool
	statement                               bool
	disallowUnparenthesizedBinExpr          bool
	disallowParsingSeveralPatternUnionCases bool
}

// parseExpression parses any expression, if $expr is a *MissingExpression $isMissingExpr will be true.
// The term 'expression' is quite broad here, it refers to any standalone node type that is not a statement
func (p *parser) parseExpression(config ...exprParsingConfig) (expr Node, isMissingExpr bool) {
	p.panicIfContextDone()

	precededByOpeningParen := len(config) > 0 && config[0].precededByOpeningParen
	isStmt := len(config) > 0 && config[0].statement

	defer func() {
		allowUnparenthesizedBinExpr := !p.inPattern && !isStmt && (len(config) == 0 || !config[0].disallowUnparenthesizedBinExpr)

		if expr != nil && !isMissingExpr && allowUnparenthesizedBinExpr {
			binExpr, ok := p.tryParseUnparenthesizedBinaryExpr(expr)
			if ok {
				expr = binExpr
				return
			}
		}

		//union pattern without leading pipe
		if p.inPattern && !isStmt && expr != nil && !isMissingExpr && (len(config) == 0 || !config[0].disallowParsingSeveralPatternUnionCases) {
			patternUnion, ok := p.tryParsePatternUnionWithoutLeadingPipe(expr, precededByOpeningParen)
			if ok {
				expr = patternUnion
				return
			}
		}
	}()

	exprStartIndex := p.i
	// these variables are only used for expressions that can be on the left side of a member/slice/index/call expression,
	// other expressions are directly returned.
	var (
		left  Node
		first Node
	)

	if p.i >= p.len {
		return &MissingExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{p.i - 1, p.i},
				Err:  &ParsingError{MissingExpr, fmtExprExpectedHere(p.s, p.i, false)},
			},
		}, true
	}

	switch p.s[p.i] {
	case '$': //variables and URL expressions
		start := p.i
		p.i++

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		variable := &Variable{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Name: string(p.s[start+1 : p.i]),
		}

		if p.i < p.len && p.s[p.i] == '/' {
			return p.parseURLLike(start, variable), false
		}

		left = variable
	case '!':
		p.i++
		operand, _ := p.parseExpression()
		p.tokens = append(p.tokens, Token{Type: EXCLAMATION_MARK, Span: NodeSpan{exprStartIndex, exprStartIndex + 1}})

		return &UnaryExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{exprStartIndex, operand.Base().Span.End},
			},
			Operator: BoolNegate,
			Operand:  operand,
		}, false
	case '~':
		p.i++
		expr, _ := p.parseExpression()
		p.tokens = append(p.tokens, Token{Type: TILDE, Span: NodeSpan{exprStartIndex, exprStartIndex + 1}})

		return &RuntimeTypeCheckExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{exprStartIndex, expr.Base().Span.End},
			},
			Expr: expr,
		}, false
	case ':':
		if p.i >= p.len-1 {
			break
		}

		switch p.s[p.i+1] {
		case '/':
			if p.i >= p.len-2 || p.s[p.i+2] != '/' {
				break
			}
			return p.parseURLLike(p.i, nil), false
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return p.parsePortLiteral(), false
		case '{':
			return p.parseDictionaryLiteral(), false
		}

	//TODO: refactor ?
	case '_', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		e, returnNow := p.parseUnderscoreAlphaStartingExpression(precededByOpeningParen, isStmt)
		if returnNow {
			return e, false
		}
		left = e
	case '0':
		if p.i < p.len-2 && isByteSliceBase(p.s[p.i+1]) && p.s[p.i+2] == '[' {
			return p.parseByteSlices(), false
		}
		return p.parseNumberAndRangeAndRateLiterals(), false
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.parseNumberAndRangeAndRateLiterals(), false
	case '{':
		if p.inPattern {
			return p.parseObjectRecordPatternLiteral(false, false), false
		}
		return p.parseObjectOrRecordLiteral(false), false
	case '[':
		if p.inPattern {
			return p.parseListTuplePatternLiteral(false, false), false
		}
		return p.parseListOrTupleLiteral(false), false
	case '|':
		if p.inPattern {
			caseBeforeFirstPipe := Node(nil)
			return p.parsePatternUnion(p.i, false, caseBeforeFirstPipe, precededByOpeningParen), false
		}
	case '\'':
		return p.parseRuneRuneRange(), false
	case '"':
		return p.parseQuotedStringLiteral(), false
	case '`':
		return p.parseStringTemplateLiteralOrMultilineStringLiteral(nil), false
	case '+':
		if p.i < p.len-1 && isDecDigit(p.s[p.i+1]) {
			break
		}
		start := p.i
		return p.parseUnquotedStringLiteral(start), false

	case '/':
		return p.parsePathLikeExpression(false), false
	case '.':
		return p.parseDotStartingExpression(), false
	case '-':
		return p.parseDashStartingExpression(precededByOpeningParen), false
	case '#':
		if p.i < p.len-1 {
			switch p.s[p.i+1] {
			case '{':
				if p.inPattern {
					return p.parseObjectRecordPatternLiteral(false, true), false
				}
				return p.parseObjectOrRecordLiteral(true), false
			case '[':
				if p.inPattern {
					return p.parseListTuplePatternLiteral(false, true), false
				}
				return p.parseListOrTupleLiteral(true), false
			}
		}
		p.i++

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		var parsingErr *ParsingError

		if p.i == exprStartIndex+1 {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT}
		}

		return &UnambiguousIdentifierLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{exprStartIndex, p.i},
				Err:  parsingErr,
			},
			Name: string(p.s[exprStartIndex+1 : p.i]),
		}, false
	case '@':
		return p.parseLazyAndCodegenStuff(), false
	case '*':
		start := p.i
		p.tokens = append(p.tokens, Token{Type: ASTERISK, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		if p.inPattern {
			typ, _ := p.parseExpression()
			return &PointerType{
				NodeBase:  NodeBase{Span: NodeSpan{start, typ.Base().Span.End}},
				ValueType: typ,
			}, false
		} else {
			pointer, _ := p.parseExpression()
			return &DereferenceExpression{
				NodeBase: NodeBase{Span: NodeSpan{start, pointer.Base().Span.End}},
				Pointer:  pointer,
			}, false
		}
	case '%':
		patt := p.parsePercentPrefixedPattern(precededByOpeningParen)

		switch patt.(type) {
		case *PatternIdentifierLiteral, *PatternNamespaceMemberExpression:
			if p.i < p.len && p.s[p.i] == '`' {
				return p.parseStringTemplateLiteralOrMultilineStringLiteral(patt), false
			}
		}
		return patt, false
	case '(': //parenthesized expression, unary expression, binary expression, pattern union
		openingParenIndex := p.i
		p.i++

		left = p.parseUnaryBinaryAndParenthesizedExpression(openingParenIndex, -1)
		if p.i >= p.len {
			return left, false
		}
	}

	first = left

loop:
	for left != nil && p.i < p.len && (!isUnpairedOrIsClosingDelim(p.s[p.i]) || (p.i < p.len-1 && p.s[p.i] == ':' && p.s[p.i+1] == ':')) {
		isDoubleColon := p.i < p.len-1 && p.s[p.i] == ':' && p.s[p.i+1] == ':'

		switch {
		case p.s[p.i] == '[' || p.s[p.i] == '.' || isDoubleColon:
			//member expressions, index/slice expressions, extraction expression & double-colon expressions.
			membLike, continueLoop := p.parseMemberLike(exprStartIndex, first, left, isDoubleColon)
			if continueLoop {
				left = membLike
				continue loop
			}
			return membLike, false
		case ((p.i < p.len && p.s[p.i] == '(') ||
			(p.i < p.len-1 && p.s[p.i] == '!' && p.s[p.i+1] == '(')): //call: <left> '(' ...

			must := false
			if p.s[p.i] == '!' {
				must = true
				p.i++
				p.tokens = append(p.tokens,
					Token{Type: EXCLAMATION_MARK, Span: NodeSpan{p.i - 1, p.i}},
					Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}},
				)
			} else {
				p.tokens = append(p.tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			}

			p.i++
			spanStart := left.Base().Span.Start

			if left == first {
				spanStart = exprStartIndex
			}

			call := &CallExpression{
				NodeBase: NodeBase{
					NodeSpan{spanStart, 0},
					nil,
					false,
				},
				Callee:    left,
				Arguments: nil,
				Must:      must,
			}

			left = p.parseParenthesizedCallArgs(call)
		case p.s[p.i] == '?':
			p.i++
			left = &BooleanConversionExpression{
				NodeBase: NodeBase{
					NodeSpan{exprStartIndex, p.i},
					nil,
					false,
				},
				Expr: left,
			}
		default:
			break loop
		}
	}

	if left != nil {
		return left, false
	}

	return &MissingExpression{
		NodeBase: NodeBase{
			Span: NodeSpan{p.i, p.i + 1},
			Err:  &ParsingError{MissingExpr, fmtExprExpectedHere(p.s, p.i, true)},
		},
	}, true
}

func (p *parser) parseUnderscoreAlphaStartingExpression(precededByOpeningParen bool, stmt bool) (node Node, returnNow bool) {
	returnNow = true
	identStartingExpr := p.parseIdentStartingExpression(p.inPattern)

	var name string

	switch v := identStartingExpr.(type) {
	case *IdentifierLiteral:
		name = v.Name
		switch name {
		case tokenStrings[GO_KEYWORD]:
			node = p.parseSpawnExpression(identStartingExpr)
			return
		case tokenStrings[FN_KEYWORD]:
			if p.inPattern {
				node = p.parseFunctionPattern(identStartingExpr.Base().Span.Start, false)
				return
			}
			node = p.parseFunction(identStartingExpr.Base().Span.Start)
			return
		case "s":
			if p.i < p.len && p.s[p.i] == '!' {
				p.i++
				node = p.parseTopCssSelector(p.i - 2)
				return
			}
		case tokenStrings[MAPPING_KEYWORD]:
			node = p.parseMappingExpression(v)
			return
		case tokenStrings[COMP_KEYWORD]:
			node = p.parseComputeExpression(v)
			return
		case tokenStrings[TREEDATA_KEYWORD]:
			node = p.parseTreedataLiteral(v)
			return
		case tokenStrings[CONCAT_KEYWORD]:
			node = p.parseConcatenationExpression(v, precededByOpeningParen)
			return
		case tokenStrings[TESTSUITE_KEYWORD]:
			node = p.parseTestSuiteExpression(v)
			return
		case tokenStrings[TESTCASE_KEYWORD]:
			node = p.parseTestCaseExpression(v)
			return
		case tokenStrings[LIFETIMEJOB_KEYWORD]:
			node = p.parseLifetimeJobExpression(v)
			return
		case tokenStrings[ON_KEYWORD]:
			node = p.parseReceptionHandlerExpression(v)
			return
		case tokenStrings[SENDVAL_KEYWORD]:
			node = p.parseSendValueExpression(v)
			return
		case tokenStrings[READONLY_KEYWORD]:
			if p.inPattern {
				node = p.parseReadonlyPatternExpression(v)
				return
			}
		case NEW_KEYWORD_STRING:
			node = p.parseNewExpression(v)
			return
		default:
			if !stmt && (name == SWITCH_KEYWORD_STRING || name == MATCH_KEYWORD_STRING) {
				node = p.parseSwitchMatchExpression(v)
				return
			}
		}
		if isKeyword(name) {
			node = v
			return
		}
		if p.inPattern {
			result := &PatternIdentifierLiteral{
				NodeBase:   v.NodeBase,
				Unprefixed: true,
				Name:       v.Name,
			}
			if p.i < p.len {
				switch p.s[p.i] {
				case '(', '{':
					if v.Name == "str" && p.s[p.i] == '(' {
						p.i++
						node = p.parseComplexStringPatternPiece(result.Span.Start, rootSequencePatternPiece, result)
						return
					}

					node = p.parsePatternCall(result)
					return
				case '?':
					p.i++
					node = &OptionalPatternExpression{
						NodeBase: NodeBase{
							Span: NodeSpan{result.Base().Span.Start, p.i},
						},
						Pattern: result,
					}
					return
				}
			}
			node = result
			return
		}
	case *IdentifierMemberExpression:
		if p.inPattern && len(v.PropertyNames) == 1 {
			base := v.Left.NodeBase
			base.Span.End += 1 //add one for the dot

			result := &PatternNamespaceMemberExpression{
				NodeBase: v.NodeBase,
				Namespace: &PatternNamespaceIdentifierLiteral{
					NodeBase:   base,
					Unprefixed: true,
					Name:       v.Left.Name,
				},
				MemberName: v.PropertyNames[0],
			}
			if p.i < p.len {
				switch p.s[p.i] {
				case '(', '{':
					node = p.parsePatternCall(result)
					return
				case '?':
					p.i++
					node = &OptionalPatternExpression{
						NodeBase: NodeBase{
							Span: NodeSpan{result.Base().Span.Start, p.i},
						},
						Pattern: result,
					}
					return
				}
			}
			node = result
			return
		}

		name = v.Left.Name
	case *SelfExpression, *MemberExpression:
		node = identStartingExpr
		returnNow = false
	default:
		node = v
		return
	}

	if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (p.i >= p.len-1 || p.s[p.i] != ':' || p.s[p.i+1] != ':')) {
		node = identStartingExpr
		return
	}

	if p.s[p.i] == '<' && NodeIs(identStartingExpr, (*IdentifierLiteral)(nil)) {
		ident := identStartingExpr.(*IdentifierLiteral)
		node = p.parseXMLExpression(ident, ident.Span.Start)
		return
	}

	call := p.tryParseCall(identStartingExpr, name)
	if call != nil {
		identStartingExpr = call
	}

	node = identStartingExpr
	returnNow = false
	return
}

func (p *parser) parseMemberLike(_start int32, first, left Node, isDoubleColon bool) (result Node, continueLoop bool) {
	dot := p.s[p.i] == '.'
	isBracket := p.s[p.i] == '['
	tokenStart := p.i

	if isDoubleColon {
		p.i++
	}

	p.i++
	start := p.i
	isOptional := false

	isDot := p.s[p.i-1] == '.'

	if isDot && p.i < p.len && p.s[p.i] == '?' {
		isOptional = true
		p.i++
		start = p.i
	}

	if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (dot || (p.s[p.i] != ':' && p.s[p.i] != ']'))) {
		//unterminated member expression
		if isDot {
			result = &MemberExpression{
				NodeBase: NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
					false,
				},
				Left:     left,
				Optional: isOptional,
			}
			return
		}
		if isDoubleColon {
			p.tokens = append(p.tokens, Token{Type: DOUBLE_COLON, Span: NodeSpan{tokenStart, tokenStart + 2}})

			result = &DoubleColonExpression{
				NodeBase: NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&ParsingError{UnterminatedDoubleColonExpr, UNTERMINATED_DOUBLE_COLON_EXPR},
					false,
				},
				Left: left,
			}
			return
		}
		result = &InvalidMemberLike{
			NodeBase: NodeBase{
				NodeSpan{first.Base().Span.Start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_MEMB_OR_INDEX_EXPR},
				false,
			},
			Left: left,
		}
		return
	}

	switch {
	case isBracket: //index/slice expression
		p.eatSpace()

		if p.i >= p.len {
			result = &InvalidMemberLike{
				NodeBase: NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
					false,
				},
				Left: left,
			}
			return
		}

		var startIndex Node
		var endIndex Node
		isSliceExpr := p.s[p.i] == ':'

		if isSliceExpr {
			p.i++
		} else {
			startIndex, _ = p.parseExpression()
		}

		p.eatSpace()

		if p.i >= p.len {
			result = &InvalidMemberLike{
				NodeBase: NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
					false,
				},
				Left: left,
			}
			return
		}

		if p.s[p.i] == ':' {
			if isSliceExpr {
				result = &SliceExpression{
					NodeBase: NodeBase{
						Span: NodeSpan{first.Base().Span.Start, p.i},
						Err:  &ParsingError{UnspecifiedParsingError, INVALID_SLICE_EXPR_SINGLE_COLON},
					},
					Indexed:    left,
					StartIndex: startIndex,
					EndIndex:   endIndex,
				}
				return
			}
			isSliceExpr = true
			p.i++
		}

		p.eatSpace()

		if isSliceExpr && startIndex == nil && (p.i >= p.len || p.s[p.i] == ']') {
			result = &SliceExpression{
				NodeBase: NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_SLICE_EXPR_MISSING_END_INDEX},
					false,
				},
				Indexed:    left,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			}
			return
		}

		if p.i < p.len && p.s[p.i] != ']' && isSliceExpr {
			endIndex, _ = p.parseExpression()
		}

		p.eatSpace()

		if p.i >= p.len || p.s[p.i] != ']' {
			result = &InvalidMemberLike{
				NodeBase: NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR_MISSING_CLOSING_BRACKET},
					false,
				},
				Left: left,
			}
			return
		}

		p.i++

		spanStart := left.Base().Span.Start
		if left == first {
			spanStart = _start
		}

		if isSliceExpr {
			result = &SliceExpression{
				NodeBase: NodeBase{
					NodeSpan{spanStart, p.i},
					nil,
					false,
				},
				Indexed:    left,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			}
			continueLoop = true
			return
		}

		result = &IndexExpression{
			NodeBase: NodeBase{
				NodeSpan{spanStart, p.i},
				nil,
				false,
			},
			Indexed: left,
			Index:   startIndex,
		}
		continueLoop = true
		return
	case isDoubleColon: //double-colon expression
		p.tokens = append(p.tokens, Token{Type: DOUBLE_COLON, Span: NodeSpan{tokenStart, tokenStart + 2}})

		elementNameStart := p.i
		var parsingErr *ParsingError
		if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
			parsingErr = &ParsingError{UnspecifiedParsingError, fmtDoubleColonExpressionelementShouldStartWithAletterNot(p.s[p.i])}
		}

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		spanStart := left.Base().Span.Start
		if left == first {
			spanStart = _start
		}

		elementName := string(p.s[elementNameStart:p.i])
		if left == first {
			spanStart = _start
		}

		element := &IdentifierLiteral{
			NodeBase: NodeBase{
				NodeSpan{elementNameStart, p.i},
				nil,
				false,
			},
			Name: elementName,
		}

		result = &DoubleColonExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{spanStart, p.i},
				Err:  parsingErr,
			},
			Left:    left,
			Element: element,
		}
		continueLoop = true
		return
	case p.s[p.i] == '{': //extraction expression (result is returned, the loop is not continued)
		p.i--
		keyList := p.parseKeyList()

		result = &ExtractionExpression{
			NodeBase: NodeBase{
				NodeSpan{left.Base().Span.Start, keyList.Span.End},
				nil,
				false,
			},
			Object: left,
			Keys:   keyList,
		}
		continueLoop = true
		return
	default:
		isDynamic := false
		isComputed := false
		spanStart := left.Base().Span.Start
		var computedPropertyNode Node
		var propertyNameIdent *IdentifierLiteral
		propNameStart := start

		if !isOptional && p.i < p.len {
			switch p.s[p.i] {
			case '<':
				isDynamic = true
				p.i++
				propNameStart++
			case '(':
				isComputed = true
				p.i++
				computedPropertyNode = p.parseUnaryBinaryAndParenthesizedExpression(p.i-1, -1)
			}
		}

		newMemberExpression := func(err *ParsingError) Node {
			if isDynamic {
				return &DynamicMemberExpression{
					NodeBase: NodeBase{
						NodeSpan{spanStart, p.i},
						err,
						false,
					},
					Left:         left,
					PropertyName: propertyNameIdent,
					Optional:     isOptional,
				}
			}
			if isComputed {
				return &ComputedMemberExpression{
					NodeBase: NodeBase{
						NodeSpan{spanStart, p.i},
						err,
						false,
					},
					Left:         left,
					PropertyName: computedPropertyNode,
					Optional:     isOptional,
				}
			}
			return &MemberExpression{
				NodeBase: NodeBase{
					NodeSpan{spanStart, p.i},
					err,
					false,
				},
				Left:         left,
				PropertyName: propertyNameIdent,
				Optional:     isOptional,
			}
		}

		if !isComputed {
			if isDynamic && p.i >= p.len {
				result = newMemberExpression(&ParsingError{UnspecifiedParsingError, UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR})
				return
			}

			//member expression with invalid property name
			if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
				result = newMemberExpression(&ParsingError{UnspecifiedParsingError, fmtPropNameShouldStartWithAletterNot(p.s[p.i])})
				return
			}

			for p.i < p.len && IsIdentChar(p.s[p.i]) {
				p.i++
			}

			propName := string(p.s[propNameStart:p.i])
			if left == first {
				spanStart = _start
			}

			propertyNameIdent = &IdentifierLiteral{
				NodeBase: NodeBase{
					NodeSpan{propNameStart, p.i},
					nil,
					false,
				},
				Name: propName,
			}
		}

		result = newMemberExpression(nil)
		continueLoop = true
		return
	}
}
