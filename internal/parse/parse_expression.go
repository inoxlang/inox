package parse

// parseExpression parses any expression, if $expr is a *MissingExpression $isMissingExpr will be true.
// The term 'expression' is quite broad here, it refers to any standalone node type that is not a statement
func (p *parser) parseExpression(precededByOpeningParen ...bool) (expr Node, isMissingExpr bool) {
	p.panicIfContextDone()

	__start := p.i
	// these variables are only used for expressions that can be on the left side of a member/slice/index/call expression,
	// other expressions are directly returned.
	var (
		lhs   Node
		first Node
	)

	if p.i >= p.len {
		return &MissingExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{p.i - 1, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, fmtExprExpectedHere(p.s, p.i, false)},
			},
		}, true
	}

	switch p.s[p.i] {
	case '$': //normal & global variables
		start := p.i
		isGlobal := false
		p.i++

		if p.i < p.len && p.s[p.i] == '$' {
			isGlobal = true
			p.i++
		}

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		if isGlobal {
			lhs = &GlobalVariable{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
				},
				Name: string(p.s[start+2 : p.i]),
			}
		} else {
			lhs = &Variable{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
				},
				Name: string(p.s[start+1 : p.i]),
			}
		}

	case '!':
		p.i++
		operand, _ := p.parseExpression()
		p.tokens = append(p.tokens, Token{Type: EXCLAMATION_MARK, Span: NodeSpan{__start, __start + 1}})

		return &UnaryExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, operand.Base().Span.End},
			},
			Operator: BoolNegate,
			Operand:  operand,
		}, false
	case '~':
		p.i++
		expr, _ := p.parseExpression()
		p.tokens = append(p.tokens, Token{Type: TILDE, Span: NodeSpan{__start, __start + 1}})

		return &RuntimeTypeCheckExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, expr.Base().Span.End},
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
			return p.parseURLLike(p.i), false
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return p.parsePortLiteral(), false
		case '{':
			return p.parseDictionaryLiteral(), false
		}

	//TODO: refactor ?
	case '_', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		identStartingExpr := p.parseIdentStartingExpression(p.inPattern)
		var name string

		switch v := identStartingExpr.(type) {
		case *IdentifierLiteral:
			name = v.Name
			switch name {
			case tokenStrings[GO_KEYWORD]:
				return p.parseSpawnExpression(identStartingExpr), false
			case tokenStrings[FN_KEYWORD]:
				if p.inPattern {
					return p.parseFunctionPattern(identStartingExpr.Base().Span.Start, false), false
				}
				return p.parseFunction(identStartingExpr.Base().Span.Start), false

			case "s":
				if p.i < p.len && p.s[p.i] == '!' {
					p.i++
					return p.parseTopCssSelector(p.i - 2), false
				}
			case tokenStrings[MAPPING_KEYWORD]:
				return p.parseMappingExpression(v), false
			case tokenStrings[COMP_KEYWORD]:
				return p.parseComputeExpression(v), false
			case tokenStrings[TREEDATA_KEYWORD]:
				return p.parseTreedataLiteral(v), false
			case tokenStrings[CONCAT_KEYWORD]:
				return p.parseConcatenationExpression(v, len(precededByOpeningParen) > 0 && precededByOpeningParen[0]), false
			case tokenStrings[TESTSUITE_KEYWORD]:
				return p.parseTestSuiteExpression(v), false
			case tokenStrings[TESTCASE_KEYWORD]:
				return p.parseTestCaseExpression(v), false
			case tokenStrings[LIFETIMEJOB_KEYWORD]:
				return p.parseLifetimeJobExpression(v), false
			case tokenStrings[ON_KEYWORD]:
				return p.parseReceptionHandlerExpression(v), false
			case tokenStrings[SENDVAL_KEYWORD]:
				return p.parseSendValueExpression(v), false
			case tokenStrings[READONLY_KEYWORD]:
				if p.inPattern {
					return p.parseReadonlyPatternExpression(v), false
				}
			case NEW_KEYWORD_STRING:
				return p.parseNewExpression(v), false
			}
			if isKeyword(name) {
				return v, false
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
						return p.parsePatternCall(result), false
					case '?':
						p.i++
						return &OptionalPatternExpression{
							NodeBase: NodeBase{
								Span: NodeSpan{result.Base().Span.Start, p.i},
							},
							Pattern: result,
						}, false
					}
				}
				return result, false
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
						return p.parsePatternCall(result), false
					case '?':
						p.i++
						return &OptionalPatternExpression{
							NodeBase: NodeBase{
								Span: NodeSpan{result.Base().Span.Start, p.i},
							},
							Pattern: result,
						}, false
					}
				}
				return result, false
			}

			name = v.Left.Name
		case *SelfExpression, *MemberExpression:
			lhs = identStartingExpr
		default:
			return v, false
		}

		if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (p.i >= p.len-1 || p.s[p.i] != ':' || p.s[p.i+1] != ':')) {
			return identStartingExpr, false
		}

		if p.s[p.i] == '<' && NodeIs(identStartingExpr, (*IdentifierLiteral)(nil)) {
			return p.parseXMLExpression(identStartingExpr.(*IdentifierLiteral)), false
		}

		call := p.tryParseCall(identStartingExpr, name)
		if call != nil {
			identStartingExpr = call
		}

		lhs = identStartingExpr
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
			return p.parsePatternUnion(p.i, false, true), false
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
		return p.parseDashStartingExpression(len(precededByOpeningParen) > 0 && precededByOpeningParen[0]), false
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

		if p.i == __start+1 {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT}
		}

		return &UnambiguousIdentifierLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, p.i},
				Err:  parsingErr,
			},
			Name: string(p.s[__start+1 : p.i]),
		}, false
	case '@':
		return p.parseLazyAndHostAliasStuff(), false
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
		patt := p.parsePercentPrefixedPattern(len(precededByOpeningParen) > 0 && precededByOpeningParen[0])

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

		lhs = p.parseUnaryBinaryAndParenthesizedExpression(openingParenIndex, -1)
		if p.i >= p.len {
			return lhs, false
		}
	}

	first = lhs

loop:
	for lhs != nil && p.i < p.len && (!isUnpairedOrIsClosingDelim(p.s[p.i]) || (p.i < p.len-1 && p.s[p.i] == ':' && p.s[p.i+1] == ':')) {
		isDoubleColon := p.i < p.len-1 && p.s[p.i] == ':' && p.s[p.i+1] == ':'

		switch {
		//member expressions, index/slice expressions, extraction expression & double-colon expressions
		case p.s[p.i] == '[' || p.s[p.i] == '.' || isDoubleColon:
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
					return &MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left:     lhs,
						Optional: isOptional,
					}, false
				}
				if isDoubleColon {
					p.tokens = append(p.tokens, Token{Type: DOUBLE_COLON, Span: NodeSpan{tokenStart, tokenStart + 2}})

					return &DoubleColonExpression{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnterminatedDoubleColonExpr, UNTERMINATED_DOUBLE_COLON_EXPR},
							false,
						},
						Left: lhs,
					}, false
				}
				return &InvalidMemberLike{
					NodeBase: NodeBase{
						NodeSpan{first.Base().Span.Start, p.i},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_MEMB_OR_INDEX_EXPR},
						false,
					},
					Left: lhs,
				}, false
			}

			switch {
			case isBracket: //index/slice expression
				p.eatSpace()

				if p.i >= p.len {
					return &InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
							false,
						},
						Left: lhs,
					}, false
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
					return &InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
							false,
						},
						Left: lhs,
					}, false
				}

				if p.s[p.i] == ':' {
					if isSliceExpr {
						return &SliceExpression{
							NodeBase: NodeBase{
								NodeSpan{first.Base().Span.Start, p.i},
								&ParsingError{UnspecifiedParsingError, INVALID_SLICE_EXPR_SINGLE_COLON},
								false,
							},
							Indexed:    lhs,
							StartIndex: startIndex,
							EndIndex:   endIndex,
						}, false
					}
					isSliceExpr = true
					p.i++
				}

				p.eatSpace()

				if isSliceExpr && startIndex == nil && (p.i >= p.len || p.s[p.i] == ']') {
					return &SliceExpression{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_SLICE_EXPR_MISSING_END_INDEX},
							false,
						},
						Indexed:    lhs,
						StartIndex: startIndex,
						EndIndex:   endIndex,
					}, false
				}

				if p.i < p.len && p.s[p.i] != ']' && isSliceExpr {
					endIndex, _ = p.parseExpression()
				}

				p.eatSpace()

				if p.i >= p.len || p.s[p.i] != ']' {
					return &InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR_MISSING_CLOSING_BRACKET},
							false,
						},
						Left: lhs,
					}, false
				}

				p.i++

				spanStart := lhs.Base().Span.Start
				if lhs == first {
					spanStart = __start
				}

				if isSliceExpr {
					lhs = &SliceExpression{
						NodeBase: NodeBase{
							NodeSpan{spanStart, p.i},
							nil,
							false,
						},
						Indexed:    lhs,
						StartIndex: startIndex,
						EndIndex:   endIndex,
					}
					continue loop
				}

				lhs = &IndexExpression{
					NodeBase: NodeBase{
						NodeSpan{spanStart, p.i},
						nil,
						false,
					},
					Indexed: lhs,
					Index:   startIndex,
				}
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

				spanStart := lhs.Base().Span.Start
				if lhs == first {
					spanStart = __start
				}

				elementName := string(p.s[elementNameStart:p.i])
				if lhs == first {
					spanStart = __start
				}

				element := &IdentifierLiteral{
					NodeBase: NodeBase{
						NodeSpan{elementNameStart, p.i},
						nil,
						false,
					},
					Name: elementName,
				}

				lhs = &DoubleColonExpression{
					NodeBase: NodeBase{
						Span: NodeSpan{spanStart, p.i},
						Err:  parsingErr,
					},
					Left:    lhs,
					Element: element,
				}
			case p.s[p.i] == '{': //extraction expression (result is returned, the loop is not continued)
				p.i--
				keyList := p.parseKeyList()

				lhs = &ExtractionExpression{
					NodeBase: NodeBase{
						NodeSpan{lhs.Base().Span.Start, keyList.Span.End},
						nil,
						false,
					},
					Object: lhs,
					Keys:   keyList,
				}
				continue loop
			default:
				isDynamic := false
				isComputed := false
				spanStart := lhs.Base().Span.Start
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
							Left:         lhs,
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
							Left:         lhs,
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
						Left:         lhs,
						PropertyName: propertyNameIdent,
						Optional:     isOptional,
					}
				}

				if !isComputed {
					if isDynamic && p.i >= p.len {
						return newMemberExpression(&ParsingError{UnspecifiedParsingError, UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR}), false
					}

					//member expression with invalid property name
					if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
						return newMemberExpression(&ParsingError{UnspecifiedParsingError, fmtPropNameShouldStartWithAletterNot(p.s[p.i])}), false
					}

					for p.i < p.len && IsIdentChar(p.s[p.i]) {
						p.i++
					}

					propName := string(p.s[propNameStart:p.i])
					if lhs == first {
						spanStart = __start
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

				lhs = newMemberExpression(nil)
			}
		case ((p.i < p.len && p.s[p.i] == '(') ||
			(p.i < p.len-1 && p.s[p.i] == '!' && p.s[p.i+1] == '(')): //call: <lhs> '(' ...

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
			spanStart := lhs.Base().Span.Start

			if lhs == first {
				spanStart = __start
			}

			call := &CallExpression{
				NodeBase: NodeBase{
					NodeSpan{spanStart, 0},
					nil,
					false,
				},
				Callee:    lhs,
				Arguments: nil,
				Must:      must,
			}

			lhs = p.parseParenthesizedCallArgs(call)
		case p.s[p.i] == '?':
			p.i++
			lhs = &BooleanConversionExpression{
				NodeBase: NodeBase{
					NodeSpan{__start, p.i},
					nil,
					false,
				},
				Expr: lhs,
			}
		default:
			break loop
		}
	}

	if lhs != nil {
		return lhs, false
	}

	return &MissingExpression{
		NodeBase: NodeBase{
			Span: NodeSpan{p.i, p.i + 1},
			Err:  &ParsingError{UnspecifiedParsingError, fmtExprExpectedHere(p.s, p.i, true)},
		},
	}, true
}
