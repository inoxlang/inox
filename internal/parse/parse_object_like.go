package parse

import "strconv"

const (
	NO_OTHERPROPS_PATTERN_NAME = "no"
)

func (p *parser) parseObjectOrRecordLiteral(isRecord bool) Node {
	p.panicIfContextDone()

	var (
		elementCount   = 0
		properties     []*ObjectProperty
		metaProperties []*ObjectMetaProperty
		spreadElements []*PropertySpreadElement
		parsingErr     *ParsingError
	)

	openingBraceIndex := p.i

	if isRecord {
		p.tokens = append(p.tokens, Token{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2
	} else {
		p.tokens = append(p.tokens, Token{
			Type:    OPENING_CURLY_BRACKET,
			SubType: OBJECT_LIKE_OPENING_BRACE,
			Span:    NodeSpan{p.i, p.i + 1},
		})
		p.i++
	}

	p.eatSpaceNewlineCommaComment()

	//entry
	var (
		nextTokenIndex int
		key            Node
		keyName        string
		keyOrVal       Node
		noKey          bool
		type_          Node
		v              Node
		isMissingExpr  bool
		propSpanStart  int32
		propSpanEnd    int32
		propParsingErr *ParsingError
	)

object_literal_top_loop:
	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) { //one iteration == one entry or spread element (that can be invalid)
		propParsingErr = nil
		nextTokenIndex = -1
		key = nil
		keyOrVal = nil
		isMissingExpr = false
		propSpanStart = 0
		propSpanEnd = 0
		keyName = ""
		type_ = nil
		v = nil
		propParsingErr = nil
		noKey = false

		if p.i >= p.len || p.s[p.i] == '}' {
			break object_literal_top_loop
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' { //spread element
			spreadStart := p.i
			p.tokens = append(p.tokens, Token{Type: THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}})

			p.i += 3
			p.eatSpace()

			expr, _ := p.parseExpression()

			_, ok := expr.(*ExtractionExpression)
			if !ok {
				propParsingErr = &ParsingError{ExtractionExpressionExpected, fmtInvalidSpreadElemExprShouldBeExtrExprNot(expr)}
			}

			p.eatSpace()

			if p.i < p.len && !isValidEntryEnd(p.s, p.i) {
				propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_ENTRY_ENTRY_SEPARATION}
			}

			spreadElements = append(spreadElements, &PropertySpreadElement{
				NodeBase: NodeBase{
					NodeSpan{spreadStart, expr.Base().Span.End},
					propParsingErr,
					false,
				},
				Expr: expr,
			})

			goto step_end
		}

		nextTokenIndex = len(p.tokens)
		key, isMissingExpr = p.parseExpression()
		keyOrVal = key

		//if missing expression we report an error and we continue the main loop
		if isMissingExpr {
			char := p.s[p.i]
			propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectRecord(p.s[p.i])}
			p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(char), Span: NodeSpan{p.i, p.i + 1}})

			p.i++
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span: NodeSpan{p.i - 1, p.i},
					Err:  propParsingErr,
				},
				Key:   nil,
				Value: nil,
			})
			goto step_end

		}

		propSpanStart = key.Base().Span.Start

		if nextTokenIndex < len(p.tokens) && p.tokens[nextTokenIndex].Type == OPENING_PARENTHESIS {
			noKey = true
			keyName = "(element: " + strconv.Itoa(elementCount) + ")"
			v = key
			propSpanEnd = v.Base().Span.End
			key = nil
			elementCount++
		} else {
			switch k := key.(type) {
			case *IdentifierLiteral:
				keyName = k.Name
			case *DoubleQuotedStringLiteral:
				keyName = k.Value
			case *UnquotedRegion:
				noKey = !p.areNextSpacesFollowedBy(':') && !p.areNextSpacesFollowedBy('%')
				keyName = "(unquoted region)"
				if noKey {
					v = key
					key = nil
					propSpanEnd = v.Base().Span.End
				}
			default:
				noKey = true
				keyName = "(element: " + strconv.Itoa(elementCount) + ")"
				v = key
				propSpanEnd = v.Base().Span.End
				key = nil
			}
			elementCount++
		}

		p.eatSpace()
		if p.i >= p.len {
			break object_literal_top_loop
		}

		switch {
		case isValidEntryEnd(p.s, p.i):
			noKey = true
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span: NodeSpan{propSpanStart, p.i},
					Err:  propParsingErr,
				},
				Value: keyOrVal,
			})
			goto step_end
		case p.s[p.i] == ':':
			goto at_colon
		case p.s[p.i] == '%': // type annotation
			switch {
			case noKey: // no key properties cannot be annotated
				propParsingErr = &ParsingError{UnspecifiedParsingError, ONLY_KEYS_CAN_HAVE_A_TYPE_ANNOT}
				noKey = true
				type_ = p.parsePercentPrefixedPattern(false)
				propSpanEnd = type_.Base().Span.End

				properties = append(properties, &ObjectProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, propSpanEnd},
						Err:  propParsingErr,
					},
					Value: keyOrVal,
					Type:  type_,
				})

				goto step_end
			case isRecord: //explicit key properties of record cannot be annotated
				properties = append(properties, &ObjectProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Key: keyOrVal,
				})
				goto step_end //the pattern is kept for the next iteration step
			case IsMetadataKey(keyName): //meta properties cannot be annotated
				propParsingErr = &ParsingError{UnspecifiedParsingError, ONLY_KEYS_CAN_HAVE_A_TYPE_ANNOT}
				metaProperties = append(metaProperties, &ObjectMetaProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Key: keyOrVal,
				})
				goto step_end //the pattern is kept for the next iteration step
			default: //explicit key property
			}

			type_ = p.parsePercentPrefixedPattern(false)
			propSpanEnd = type_.Base().Span.End

			p.eatSpace()
			if p.i >= p.len {
				break object_literal_top_loop
			}

			goto explicit_key
		default:

		}

		// if meta property we parse it and continue to next iteration step
		if !noKey && IsMetadataKey(keyName) && !isRecord && p.i < p.len && p.s[p.i] != ':' {
			block := p.parseBlock()
			propSpanEnd = block.Span.End

			metaProperties = append(metaProperties, &ObjectMetaProperty{
				NodeBase: NodeBase{
					Span: NodeSpan{propSpanStart, propSpanEnd},
					Err:  propParsingErr,
				},
				Key: key,
				Initialization: &InitializationBlock{
					NodeBase:   block.NodeBase,
					Statements: block.Statements,
				},
			})

			p.eatSpace()

			goto step_end
		}

		if noKey { // no key property not followed by a valid entry end
			propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_ENTRY_ENTRY_SEPARATION}
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span: NodeSpan{propSpanStart, p.i},
					Err:  propParsingErr,
				},
				Value: keyOrVal,
			})
			goto step_end
		}

	explicit_key:

		if p.s[p.i] != ':' { //we add the property and we keep the current character for the next iteration step
			if type_ == nil {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyMissingColonAfterKey(keyName)}
			} else {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjKeyMissingColonAfterTypeAnnotation(keyName)}
			}
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span: NodeSpan{propSpanStart, p.i},
					Err:  propParsingErr,
				},
				Key:  key,
				Type: type_,
			})
			goto step_end
		}

	at_colon:
		{
			if noKey {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtOnlyIdentsAndStringsValidObjRecordKeysNot(key)}
				noKey = false
			}

			p.tokens = append(p.tokens, Token{Type: COLON, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			p.eatSpace()

			if p.i < p.len-1 && p.s[p.i] == '#' && IsCommentFirstSpace(p.s[p.i+1]) {
				p.eatSpaceNewlineComment()
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyCommentBeforeValueOfKey(keyName)}
			}

			p.eatSpace()

			if p.i >= p.len || p.s[p.i] == '}' || p.s[p.i] == ',' { //missing value
				if propParsingErr == nil {
					propParsingErr = &ParsingError{MissingObjectPropertyValue, MISSING_PROPERTY_VALUE}
				}
				properties = append(properties, &ObjectProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Key:  key,
					Type: type_,
				})

				goto step_end
			}

			if p.s[p.i] == '\n' {
				propParsingErr = &ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON}
				properties = append(properties, &ObjectProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Key:  key,
					Type: type_,
				})
				goto step_end
			}

			v, isMissingExpr = p.parseExpression()
			propSpanEnd = p.i

			if isMissingExpr {
				if p.i < p.len {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectRecord(p.s[p.i])}
					p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
					p.i++
				} else {
					v = nil
				}
			}

			p.eatSpace()

			if !isMissingExpr && p.i < p.len && !isValidEntryEnd(p.s, p.i) && !isClosingDelim(p.s[p.i]) {
				propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_ENTRY_ENTRY_SEPARATION}
			}

			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span: NodeSpan{propSpanStart, propSpanEnd},
					Err:  propParsingErr,
				},
				Key:   key,
				Type:  type_,
				Value: v,
			})
		}

	step_end:
		keyName = ""
		key = nil
		keyOrVal = nil
		v = nil
		noKey = false
		type_ = nil
		p.eatSpaceNewlineCommaComment()
	}

	if !noKey && keyName != "" || v != nil {
		properties = append(properties, &ObjectProperty{
			NodeBase: NodeBase{
				Span: NodeSpan{propSpanStart, propSpanEnd},
				Err:  propParsingErr,
			},
			Key:   key,
			Type:  type_,
			Value: v,
		})
	}

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, Token{
			Type:    CLOSING_CURLY_BRACKET,
			SubType: OBJECT_LIKE_CLOSING_BRACE,
			Span:    NodeSpan{p.i, p.i + 1},
		})
		p.i++
	} else {
		errorMessage := UNTERMINATED_OBJ_MISSING_CLOSING_BRACE
		if isRecord {
			errorMessage = UNTERMINATED_REC_MISSING_CLOSING_BRACE
		}
		parsingErr = &ParsingError{UnspecifiedParsingError, errorMessage}
	}

	base := NodeBase{
		Span: NodeSpan{openingBraceIndex, p.i},
		Err:  parsingErr,
	}

	if isRecord {
		return &RecordLiteral{
			NodeBase:       base,
			Properties:     properties,
			SpreadElements: spreadElements,
		}
	}

	return &ObjectLiteral{
		NodeBase:       base,
		Properties:     properties,
		MetaProperties: metaProperties,
		SpreadElements: spreadElements,
	}
}

func (p *parser) parseObjectRecordPatternLiteral(percentPrefixed, isRecordPattern bool) Node {
	p.panicIfContextDone()

	var (
		elementCount       = 0
		properties         []*ObjectPatternProperty
		otherPropsExprs    []*OtherPropsExpr
		spreadElements     []*PatternPropertySpreadElement
		parsingErr         *ParsingError
		objectPatternStart int32
	)

	if percentPrefixed {
		if isRecordPattern {
			panic(ErrUnreachable)
		}
		p.tokens = append(p.tokens, Token{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{p.i - 1, p.i + 1}})
		objectPatternStart = p.i - 1
		p.i++
	} else {
		objectPatternStart = p.i
		if isRecordPattern {
			p.tokens = append(p.tokens, Token{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{p.i, p.i + 2}})
			p.i += 2
		} else {
			p.tokens = append(p.tokens, Token{
				Type:    OPENING_CURLY_BRACKET,
				SubType: OBJECT_LIKE_OPENING_BRACE, Span: NodeSpan{p.i, p.i + 1},
			})
			p.i++
		}
	}

	p.eatSpaceNewlineCommaComment()

	//entry
	var (
		key            Node
		keyName        string
		keyOrVal       Node
		isOptional     bool
		noKey          bool
		type_          Node
		v              Node
		isMissingExpr  bool
		propSpanStart  int32
		propSpanEnd    int32
		propParsingErr *ParsingError
	)

object_pattern_top_loop:
	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) { //one iteration == one entry or spread element (that can be invalid)
		propParsingErr = nil
		key = nil
		isMissingExpr = false
		propSpanStart = 0
		propSpanEnd = 0
		keyName = ""
		v = nil
		propParsingErr = nil
		noKey = false

		if p.s[p.i] == '}' {
			break object_pattern_top_loop
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' { //spread element
			spreadStart := p.i
			dotStart := p.i

			p.i += 3

			p.eatSpace()

			// //inexact pattern
			// if p.i < p.len && (p.s[p.i] == '}' || p.s[p.i] == ',' || p.s[p.i] == '\n') {
			// 	tokens = append(tokens, Token{Type: THREE_DOTS, Span: NodeSpan{dotStart, dotStart + 3}})

			// 	exact = false

			// 	p.eatSpaceNewlineCommaComment(&tokens)
			// 	continue object_pattern_top_loop
			// }
			// p.eatSpace()

			expr, _ := p.parseExpression()

			var locationErr *ParsingError

			if len(properties) > 0 {
				locationErr = &ParsingError{UnspecifiedParsingError, SPREAD_SHOULD_BE_LOCATED_AT_THE_START}
			}

			p.tokens = append(p.tokens, Token{Type: THREE_DOTS, Span: NodeSpan{dotStart, dotStart + 3}})

			spreadElements = append(spreadElements, &PatternPropertySpreadElement{
				NodeBase: NodeBase{
					NodeSpan{spreadStart, expr.Base().Span.End},
					locationErr,
					false,
				},
				Expr: expr,
			})

			goto step_end
		} else {
			prev := p.inPattern
			p.inPattern = false

			nextTokenIndex := len(p.tokens)
			key, isMissingExpr = p.parseExpression()
			keyOrVal = key

			p.inPattern = prev

			//if missing expression we report an error and we continue the main loop
			if isMissingExpr {
				char := p.s[p.i]
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectPattern(p.s[p.i])}
				p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(char), Span: NodeSpan{p.i, p.i + 1}})

				p.i++
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{p.i - 1, p.i},
						Err:  propParsingErr,
					},
					Key:   nil,
					Value: nil,
				})
				goto step_end
			}

			if boolConvExpr, ok := key.(*BooleanConversionExpression); ok {
				key = boolConvExpr.Expr
				keyOrVal = key
				isOptional = true
				p.tokens = append(p.tokens, Token{Type: QUESTION_MARK, Span: NodeSpan{p.i - 1, p.i}})
			}

			propSpanStart = key.Base().Span.Start

			if nextTokenIndex < len(p.tokens) && p.tokens[nextTokenIndex].Type == OPENING_PARENTHESIS {
				noKey = true
				keyName = "(element: " + strconv.Itoa(elementCount) + ")"
				v = key
				propSpanEnd = v.Base().Span.End
				key = nil
				elementCount++
			} else {
				switch k := key.(type) {
				case *IdentifierLiteral:
					keyName = k.Name

					if keyName == OTHERPROPS_KEYWORD_STRING {
						expr := p.parseOtherProps(k)
						otherPropsExprs = append(otherPropsExprs, expr)
						goto step_end
					}
				case *DoubleQuotedStringLiteral:
					keyName = k.Value
				case *UnquotedRegion:
					noKey = !p.areNextSpacesFollowedBy(':') && !p.areNextSpacesFollowedBy('%')
					keyName = "(unquoted region)"
					if noKey {
						v = key
						key = nil
						propSpanEnd = v.Base().Span.End
					}
				default:
					noKey = true
					propParsingErr = &ParsingError{UnspecifiedParsingError, A_KEY_IS_REQUIRED_FOR_EACH_VALUE_IN_OBJ_REC_PATTERNS}
					keyName = "(element: " + strconv.Itoa(elementCount) + ")"
					v = key
					propSpanEnd = v.Base().Span.End
					key = nil
					elementCount++
				}
			}

			p.eatSpace()
			if p.i >= p.len {
				break object_pattern_top_loop
			}

			switch {
			case isValidEntryEnd(p.s, p.i):
				noKey = true
				if propParsingErr == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, A_KEY_IS_REQUIRED_FOR_EACH_VALUE_IN_OBJ_REC_PATTERNS}
				}
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Value: keyOrVal,
				})
				goto step_end
			case p.s[p.i] == ':':
				goto at_colon
			case p.s[p.i] == '%': // type annotation
				switch {
				case noKey: // no key properties cannot be annotated
					if propParsingErr == nil {
						propParsingErr = &ParsingError{UnspecifiedParsingError, ONLY_KEYS_CAN_HAVE_A_TYPE_ANNOT}
					}
					noKey = true
					type_ = p.parsePercentPrefixedPattern(false)
					propSpanEnd = type_.Base().Span.End

					properties = append(properties, &ObjectPatternProperty{
						NodeBase: NodeBase{
							Span: NodeSpan{propSpanStart, propSpanEnd},
							Err:  propParsingErr,
						},
						Value: keyOrVal,
						Type:  type_,
					})

					goto step_end
				default: //explicit key property
				}

				type_ = p.parsePercentPrefixedPattern(false)
				propSpanEnd = type_.Base().Span.End

				p.eatSpace()
				if p.i >= p.len {
					break object_pattern_top_loop
				}

				goto explicit_key
			default:

			}

			// if meta property we add an error
			if IsMetadataKey(keyName) && propParsingErr == nil {
				propParsingErr = &ParsingError{UnspecifiedParsingError, METAPROPS_ARE_NOT_ALLOWED_IN_OBJECT_PATTERNS}
			}

			if noKey { // no key property not followed by a valid entry end
				if propParsingErr == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_PATT_LIT_ENTRY_SEPARATION}
				}
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Value: keyOrVal,
				})
				goto step_end
			}

		explicit_key:
			if p.s[p.i] != ':' { //we add the property and we keep the current character for the next iteration step
				if type_ == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjPatternKeyMissingColonAfterKey(keyName)}
				} else {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjKeyMissingColonAfterTypeAnnotation(keyName)}
				}
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Key:  key,
					Type: type_,
				})
				goto step_end
			}

		at_colon:
			{
				if noKey {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtOnlyIdentsAndStringsValidObjPatternKeysNot(key)}
					noKey = false
				}

				p.tokens = append(p.tokens, Token{Type: COLON, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				p.eatSpace()

				if p.i < p.len-1 && p.s[p.i] == '#' && IsCommentFirstSpace(p.s[p.i+1]) {
					p.eatSpaceNewlineComment()
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjPatternKeyCommentBeforeValueOfKey(keyName)}
				}

				p.eatSpace()

				if p.i >= p.len || p.s[p.i] == '}' || p.s[p.i] == ',' { //missing value
					if propParsingErr == nil {
						propParsingErr = &ParsingError{MissingObjectPatternProperty, MISSING_PROPERTY_PATTERN}
					}
					properties = append(properties, &ObjectPatternProperty{
						NodeBase: NodeBase{
							Span: NodeSpan{propSpanStart, p.i},
							Err:  propParsingErr,
						},
						Key:  key,
						Type: type_,
					})

					goto step_end
				}

				if p.s[p.i] == '\n' {
					propParsingErr = &ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON}
					properties = append(properties, &ObjectPatternProperty{
						NodeBase: NodeBase{
							Span: NodeSpan{propSpanStart, p.i},
							Err:  propParsingErr,
						},
						Key:  key,
						Type: type_,
					})
					goto step_end
				}

				v, isMissingExpr = p.parseExpression()
				propSpanEnd = p.i

				var annotations *MetadataAnnotations

				if isMissingExpr {
					if p.i < p.len {
						propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectPattern(p.s[p.i])}
						p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
						p.i++
					} else {
						v = nil
					}
				} else {
					p.eatSpace()

					annotations = p.tryParseMetadaAnnotationsAfterProperty()
					if annotations != nil {
						propSpanEnd = p.i
					}

					p.eatSpace()

					if p.i < p.len && !isValidEntryEnd(p.s, p.i) && !isClosingDelim(p.s[p.i]) {
						propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_PATT_LIT_ENTRY_SEPARATION}
					}
				}

				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span: NodeSpan{propSpanStart, propSpanEnd},
						Err:  propParsingErr,
					},
					Key:         key,
					Type:        type_,
					Value:       v,
					Annotations: annotations,
					Optional:    isOptional,
				})
			}
		}

	step_end:
		keyName = ""
		key = nil
		keyOrVal = nil
		isOptional = false
		v = nil
		noKey = false
		type_ = nil
		p.eatSpaceNewlineCommaComment()
	}

	if !noKey && keyName != "" || (keyName == "" && key != nil) {
		properties = append(properties, &ObjectPatternProperty{
			NodeBase: NodeBase{
				Span: NodeSpan{propSpanStart, propSpanEnd},
				Err:  propParsingErr,
			},
			Key:   key,
			Value: v,
		})
	}

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, Token{
			Type:    CLOSING_CURLY_BRACKET,
			SubType: OBJECT_LIKE_CLOSING_BRACE,
			Span:    NodeSpan{p.i, p.i + 1},
		})
		p.i++
	} else {
		if isRecordPattern {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_REC_PATTERN_MISSING_CLOSING_BRACE}
		} else {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE}
		}
	}

	base := NodeBase{
		Span: NodeSpan{objectPatternStart, p.i},
		Err:  parsingErr,
	}

	if isRecordPattern {
		return &RecordPatternLiteral{
			NodeBase:        base,
			Properties:      properties,
			OtherProperties: otherPropsExprs,
			SpreadElements:  spreadElements,
		}
	}

	return &ObjectPatternLiteral{
		NodeBase:        base,
		Properties:      properties,
		OtherProperties: otherPropsExprs,
		SpreadElements:  spreadElements,
	}
}

func (p *parser) parseOtherProps(key *IdentifierLiteral) *OtherPropsExpr {
	p.tokens = append(p.tokens, Token{Type: OTHERPROPS_KEYWORD, Span: key.Span})
	expr := &OtherPropsExpr{
		NodeBase: NodeBase{
			Span: key.Span,
		},
	}

	p.eatSpace()
	expr.Pattern, _ = p.parseExpression()

	if ident, ok := expr.Pattern.(*PatternIdentifierLiteral); ok && ident.Name == NO_OTHERPROPS_PATTERN_NAME {
		expr.No = true
	}

	expr.NodeBase.Span.End = expr.Pattern.Base().Span.End
	return expr
}
