package parse

import (
	"strconv"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

const (
	NO_OTHERPROPS_PATTERN_NAME = "no"
)

func (p *parser) parseObjectOrRecordLiteral(isRecord bool) ast.Node {
	p.panicIfContextDone()

	var (
		elementCount   = 0
		properties     []*ast.ObjectProperty
		metaProperties []*ast.ObjectMetaProperty
		spreadElements []*ast.PropertySpreadElement
		parsingErr     *sourcecode.ParsingError
	)

	openingBraceIndex := p.i

	if isRecord {
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_RECORD_BRACKET, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2
	} else {
		p.tokens = append(p.tokens, ast.Token{
			Type:    ast.OPENING_CURLY_BRACKET,
			SubType: ast.OBJECT_LIKE_OPENING_BRACE,
			Span:    NodeSpan{p.i, p.i + 1},
		})
		p.i++
	}

	p.eatSpaceNewlineCommaComment()

	//entry
	var (
		nextTokenIndex int
		key            ast.Node
		keyName        string
		keyOrVal       ast.Node
		noKey          bool
		type_          ast.Node
		v              ast.Node
		isMissingExpr  bool
		propSpanStart  int32
		propSpanEnd    int32
		propParsingErr *sourcecode.ParsingError
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
			p.tokens = append(p.tokens, ast.Token{Type: ast.THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}})

			p.i += 3
			p.eatSpace()

			expr, _ := p.parseExpression()

			_, ok := expr.(*ast.ExtractionExpression)
			if !ok {
				propParsingErr = &sourcecode.ParsingError{ExtractionExpressionExpected, fmtInvalidSpreadElemExprShouldBeExtrExprNot(expr)}
			}

			p.eatSpace()

			if p.i < p.len && !isValidEntryEnd(p.s, p.i) {
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_ENTRY_ENTRY_SEPARATION}
			}

			spreadElements = append(spreadElements, &ast.PropertySpreadElement{
				NodeBase: ast.NodeBase{
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
			propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectRecord(p.s[p.i])}
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Raw: string(char), Span: NodeSpan{p.i, p.i + 1}})

			p.i++
			properties = append(properties, &ast.ObjectProperty{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i - 1, p.i},
					Err:  propParsingErr,
				},
				Key:   nil,
				Value: nil,
			})
			goto step_end

		}

		propSpanStart = key.Base().Span.Start

		if nextTokenIndex < len(p.tokens) && p.tokens[nextTokenIndex].Type == ast.OPENING_PARENTHESIS {
			noKey = true
			keyName = "(element: " + strconv.Itoa(elementCount) + ")"
			v = key
			propSpanEnd = v.Base().Span.End
			key = nil
			elementCount++
		} else {
			switch k := key.(type) {
			case *ast.IdentifierLiteral:
				keyName = k.Name
			case *ast.DoubleQuotedStringLiteral:
				keyName = k.Value
			case *ast.UnquotedRegion:
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
			properties = append(properties, &ast.ObjectProperty{
				NodeBase: ast.NodeBase{
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
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, ONLY_KEYS_CAN_HAVE_A_TYPE_ANNOT}
				noKey = true
				type_ = p.parsePercentPrefixedPattern(false)
				propSpanEnd = type_.Base().Span.End

				properties = append(properties, &ast.ObjectProperty{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{propSpanStart, propSpanEnd},
						Err:  propParsingErr,
					},
					Value: keyOrVal,
					Type:  type_,
				})

				goto step_end
			case isRecord: //explicit key properties of record cannot be annotated
				properties = append(properties, &ast.ObjectProperty{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Key: keyOrVal,
				})
				goto step_end //the pattern is kept for the next iteration step
			case IsMetadataKey(keyName): //meta properties cannot be annotated
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, ONLY_KEYS_CAN_HAVE_A_TYPE_ANNOT}
				metaProperties = append(metaProperties, &ast.ObjectMetaProperty{
					NodeBase: ast.NodeBase{
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

			metaProperties = append(metaProperties, &ast.ObjectMetaProperty{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{propSpanStart, propSpanEnd},
					Err:  propParsingErr,
				},
				Key: key,
				Initialization: &ast.InitializationBlock{
					NodeBase:   block.NodeBase,
					Statements: block.Statements,
				},
			})

			p.eatSpace()

			goto step_end
		}

		if noKey { // no key property not followed by a valid entry end
			propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_ENTRY_ENTRY_SEPARATION}
			properties = append(properties, &ast.ObjectProperty{
				NodeBase: ast.NodeBase{
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
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyMissingColonAfterKey(keyName)}
			} else {
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidObjKeyMissingColonAfterTypeAnnotation(keyName)}
			}
			properties = append(properties, &ast.ObjectProperty{
				NodeBase: ast.NodeBase{
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
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtOnlyIdentsAndStringsValidObjRecordKeysNot(key)}
				noKey = false
			}

			p.tokens = append(p.tokens, ast.Token{Type: ast.COLON, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			p.eatSpace()

			if p.i < p.len-1 && p.s[p.i] == '#' && IsCommentFirstSpace(p.s[p.i+1]) {
				p.eatSpaceNewlineComment()
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyCommentBeforeValueOfKey(keyName)}
			}

			p.eatSpace()

			if p.i >= p.len || p.s[p.i] == '}' || p.s[p.i] == ',' { //missing value
				if propParsingErr == nil {
					propParsingErr = &sourcecode.ParsingError{MissingObjectPropertyValue, MISSING_PROPERTY_VALUE}
				}
				properties = append(properties, &ast.ObjectProperty{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{propSpanStart, p.i},
						Err:  propParsingErr,
					},
					Key:  key,
					Type: type_,
				})

				goto step_end
			}

			if p.s[p.i] == '\n' {
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON}
				properties = append(properties, &ast.ObjectProperty{
					NodeBase: ast.NodeBase{
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
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectRecord(p.s[p.i])}
					p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
					p.i++
				} else {
					v = nil
				}
			}

			p.eatSpace()

			if !isMissingExpr && p.i < p.len && !isValidEntryEnd(p.s, p.i) && !isClosingDelim(p.s[p.i]) {
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_ENTRY_ENTRY_SEPARATION}
			}

			properties = append(properties, &ast.ObjectProperty{
				NodeBase: ast.NodeBase{
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
		properties = append(properties, &ast.ObjectProperty{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{propSpanStart, propSpanEnd},
				Err:  propParsingErr,
			},
			Key:   key,
			Type:  type_,
			Value: v,
		})
	}

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{
			Type:    ast.CLOSING_CURLY_BRACKET,
			SubType: ast.OBJECT_LIKE_CLOSING_BRACE,
			Span:    NodeSpan{p.i, p.i + 1},
		})
		p.i++
	} else {
		errorMessage := UNTERMINATED_OBJ_MISSING_CLOSING_BRACE
		if isRecord {
			errorMessage = UNTERMINATED_REC_MISSING_CLOSING_BRACE
		}
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, errorMessage}
	}

	base := ast.NodeBase{
		Span: NodeSpan{openingBraceIndex, p.i},
		Err:  parsingErr,
	}

	if isRecord {
		return &ast.RecordLiteral{
			NodeBase:       base,
			Properties:     properties,
			SpreadElements: spreadElements,
		}
	}

	return &ast.ObjectLiteral{
		NodeBase:       base,
		Properties:     properties,
		MetaProperties: metaProperties,
		SpreadElements: spreadElements,
	}
}

func (p *parser) parseObjectRecordPatternLiteral(percentPrefixed, isRecordPattern bool) ast.Node {
	p.panicIfContextDone()

	var (
		elementCount       = 0
		properties         []*ast.ObjectPatternProperty
		otherPropsExprs    []*ast.OtherPropsExpr
		spreadElements     []*ast.PatternPropertySpreadElement
		parsingErr         *sourcecode.ParsingError
		objectPatternStart int32
	)

	if percentPrefixed {
		if isRecordPattern {
			panic(ErrUnreachable)
		}
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{p.i - 1, p.i + 1}})
		objectPatternStart = p.i - 1
		p.i++
	} else {
		objectPatternStart = p.i
		if isRecordPattern {
			p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_RECORD_BRACKET, Span: NodeSpan{p.i, p.i + 2}})
			p.i += 2
		} else {
			p.tokens = append(p.tokens, ast.Token{
				Type:    ast.OPENING_CURLY_BRACKET,
				SubType: ast.OBJECT_LIKE_OPENING_BRACE, Span: NodeSpan{p.i, p.i + 1},
			})
			p.i++
		}
	}

	p.eatSpaceNewlineCommaComment()

	//entry
	var (
		key            ast.Node
		keyName        string
		keyOrVal       ast.Node
		isOptional     bool
		noKey          bool
		type_          ast.Node
		v              ast.Node
		isMissingExpr  bool
		propSpanStart  int32
		propSpanEnd    int32
		propParsingErr *sourcecode.ParsingError
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
			// 	tokens = append(tokens, ast.Token{Type: ast.THREE_DOTS, Span: NodeSpan{dotStart, dotStart + 3}})

			// 	exact = false

			// 	p.eatSpaceNewlineCommaComment(&tokens)
			// 	continue object_pattern_top_loop
			// }
			// p.eatSpace()

			expr, _ := p.parseExpression()

			var locationErr *sourcecode.ParsingError

			if len(properties) > 0 {
				locationErr = &sourcecode.ParsingError{UnspecifiedParsingError, SPREAD_SHOULD_BE_LOCATED_AT_THE_START}
			}

			p.tokens = append(p.tokens, ast.Token{Type: ast.THREE_DOTS, Span: NodeSpan{dotStart, dotStart + 3}})

			spreadElements = append(spreadElements, &ast.PatternPropertySpreadElement{
				NodeBase: ast.NodeBase{
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
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectPattern(p.s[p.i])}
				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Raw: string(char), Span: NodeSpan{p.i, p.i + 1}})

				p.i++
				properties = append(properties, &ast.ObjectPatternProperty{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{p.i - 1, p.i},
						Err:  propParsingErr,
					},
					Key:   nil,
					Value: nil,
				})
				goto step_end
			}

			if boolConvExpr, ok := key.(*ast.BooleanConversionExpression); ok {
				key = boolConvExpr.Expr
				keyOrVal = key
				isOptional = true
				p.tokens = append(p.tokens, ast.Token{Type: ast.QUESTION_MARK, Span: NodeSpan{p.i - 1, p.i}})
			}

			propSpanStart = key.Base().Span.Start

			if nextTokenIndex < len(p.tokens) && p.tokens[nextTokenIndex].Type == ast.OPENING_PARENTHESIS {
				noKey = true
				keyName = "(element: " + strconv.Itoa(elementCount) + ")"
				v = key
				propSpanEnd = v.Base().Span.End
				key = nil
				elementCount++
			} else {
				switch k := key.(type) {
				case *ast.IdentifierLiteral:
					keyName = k.Name

					if keyName == ast.OTHERPROPS_KEYWORD_STRING {
						expr := p.parseOtherProps(k)
						otherPropsExprs = append(otherPropsExprs, expr)
						goto step_end
					}
				case *ast.DoubleQuotedStringLiteral:
					keyName = k.Value
				case *ast.UnquotedRegion:
					noKey = !p.areNextSpacesFollowedBy(':') && !p.areNextSpacesFollowedBy('%')
					keyName = "(unquoted region)"
					if noKey {
						v = key
						key = nil
						propSpanEnd = v.Base().Span.End
					}
				default:
					noKey = true
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, A_KEY_IS_REQUIRED_FOR_EACH_VALUE_IN_OBJ_REC_PATTERNS}
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
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, A_KEY_IS_REQUIRED_FOR_EACH_VALUE_IN_OBJ_REC_PATTERNS}
				}
				properties = append(properties, &ast.ObjectPatternProperty{
					NodeBase: ast.NodeBase{
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
						propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, ONLY_KEYS_CAN_HAVE_A_TYPE_ANNOT}
					}
					noKey = true
					type_ = p.parsePercentPrefixedPattern(false)
					propSpanEnd = type_.Base().Span.End

					properties = append(properties, &ast.ObjectPatternProperty{
						NodeBase: ast.NodeBase{
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
				propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, METAPROPS_ARE_NOT_ALLOWED_IN_OBJECT_PATTERNS}
			}

			if noKey { // no key property not followed by a valid entry end
				if propParsingErr == nil {
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_OBJ_PATT_LIT_ENTRY_SEPARATION}
				}
				properties = append(properties, &ast.ObjectPatternProperty{
					NodeBase: ast.NodeBase{
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
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidObjPatternKeyMissingColonAfterKey(keyName)}
				} else {
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidObjKeyMissingColonAfterTypeAnnotation(keyName)}
				}
				properties = append(properties, &ast.ObjectPatternProperty{
					NodeBase: ast.NodeBase{
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
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtOnlyIdentsAndStringsValidObjPatternKeysNot(key)}
					noKey = false
				}

				p.tokens = append(p.tokens, ast.Token{Type: ast.COLON, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				p.eatSpace()

				if p.i < p.len-1 && p.s[p.i] == '#' && IsCommentFirstSpace(p.s[p.i+1]) {
					p.eatSpaceNewlineComment()
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidObjPatternKeyCommentBeforeValueOfKey(keyName)}
				}

				p.eatSpace()

				if p.i >= p.len || p.s[p.i] == '}' || p.s[p.i] == ',' { //missing value
					if propParsingErr == nil {
						propParsingErr = &sourcecode.ParsingError{MissingObjectPatternProperty, MISSING_PROPERTY_PATTERN}
					}
					properties = append(properties, &ast.ObjectPatternProperty{
						NodeBase: ast.NodeBase{
							Span: NodeSpan{propSpanStart, p.i},
							Err:  propParsingErr,
						},
						Key:  key,
						Type: type_,
					})

					goto step_end
				}

				if p.s[p.i] == '\n' {
					propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON}
					properties = append(properties, &ast.ObjectPatternProperty{
						NodeBase: ast.NodeBase{
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

				var annotations *ast.MetadataAnnotations

				if isMissingExpr {
					if p.i < p.len {
						propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectPattern(p.s[p.i])}
						p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
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
						propParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_OBJ_PATT_LIT_ENTRY_SEPARATION}
					}
				}

				properties = append(properties, &ast.ObjectPatternProperty{
					NodeBase: ast.NodeBase{
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
		properties = append(properties, &ast.ObjectPatternProperty{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{propSpanStart, propSpanEnd},
				Err:  propParsingErr,
			},
			Key:   key,
			Value: v,
		})
	}

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{
			Type:    ast.CLOSING_CURLY_BRACKET,
			SubType: ast.OBJECT_LIKE_CLOSING_BRACE,
			Span:    NodeSpan{p.i, p.i + 1},
		})
		p.i++
	} else {
		if isRecordPattern {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_REC_PATTERN_MISSING_CLOSING_BRACE}
		} else {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE}
		}
	}

	base := ast.NodeBase{
		Span: NodeSpan{objectPatternStart, p.i},
		Err:  parsingErr,
	}

	if isRecordPattern {
		return &ast.RecordPatternLiteral{
			NodeBase:        base,
			Properties:      properties,
			OtherProperties: otherPropsExprs,
			SpreadElements:  spreadElements,
		}
	}

	return &ast.ObjectPatternLiteral{
		NodeBase:        base,
		Properties:      properties,
		OtherProperties: otherPropsExprs,
		SpreadElements:  spreadElements,
	}
}

func (p *parser) parseOtherProps(key *ast.IdentifierLiteral) *ast.OtherPropsExpr {
	p.tokens = append(p.tokens, ast.Token{Type: ast.OTHERPROPS_KEYWORD, Span: key.Span})
	expr := &ast.OtherPropsExpr{
		NodeBase: ast.NodeBase{
			Span: key.Span,
		},
	}

	p.eatSpace()
	expr.Pattern, _ = p.parseExpression()

	if ident, ok := expr.Pattern.(*ast.PatternIdentifierLiteral); ok && ident.Name == NO_OTHERPROPS_PATTERN_NAME {
		expr.No = true
	}

	expr.NodeBase.Span.End = expr.Pattern.Base().Span.End
	return expr
}
