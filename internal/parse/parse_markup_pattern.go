package parse

import (
	"strings"

	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mimeconsts"
)

func (p *parser) parseMarkupPatternExpression(prefixed bool) *MarkupPatternExpression {
	p.panicIfContextDone()

	start := p.i
	if prefixed {
		start = p.i - 1
		p.tokens = append(p.tokens, Token{
			Type: PERCENT_SYMBOL,
			Span: NodeSpan{p.i - 1, p.i},
		})
	}

	if !p.inPattern {
		p.inPattern = true
		defer func() {
			p.inPattern = false
		}()
	}

	//we do not increment because we keep the '<' for parsing the top element

	if p.i+1 >= p.len || !isAlpha(p.s[p.i+1]) {
		p.tokens = append(p.tokens, Token{Type: LESS_THAN, SubType: MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

		return &MarkupPatternExpression{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_MARKUP_PATTERN_EXPRESSION_MISSING_TOP_ELEM_NAME},
				false,
			},
		}
	}

	topElem, _ := p.parseMarkupPatternElement(p.i)

	return &MarkupPatternExpression{
		NodeBase: NodeBase{Span: NodeSpan{start, p.i}},
		Element:  topElem,
	}
}

func (p *parser) parseMarkupPatternElement(start int32) (_ *MarkupPatternElement, noOrExpectedClosingTag bool) {
	p.panicIfContextDone()

	noOrExpectedClosingTag = true

	var parsingErr *ParsingError
	p.tokens = append(p.tokens, Token{Type: LESS_THAN, SubType: MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{start, start + 1}})
	p.i++

	//Parse opening tag.

	var openingIdent *PatternIdentifierLiteral
	{
		start := p.i
		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		name := string(p.s[start:p.i])
		openingIdent = &PatternIdentifierLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Name:       name,
			Unprefixed: true,
		}
	}

	// openingIdent, ok := openingName.(*IdentifierLiteral)
	// if !ok {
	// 	parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	// }

	openingTag := &MarkupPatternOpeningTag{
		NodeBase: NodeBase{
			Span: NodeSpan{start, p.i},
		},
		Name: openingIdent,
	}

	tagName := openingIdent.Name
	singleBracketInterpolations := true
	rawTextElement := false

	if tagName == SCRIPT_TAG_NAME || tagName == STYLE_TAG_NAME {
		singleBracketInterpolations = false
		rawTextElement = true
	}

	isHyperscriptScript := false

	//Quantifier

	spaceCount := p.eatSpace()

	if p.i < p.len && isMarkupPatternElementQuantifier(p.s[p.i]) {
		if spaceCount == 0 {
			switch p.s[p.i] {
			case OPTIONAL_MARKUP_ELEMENT_QUANTIFIER_RUNE:
				openingTag.Quantifier = OptionalMarkupElement
			case ONE_OR_MORE_MARKUP_ELEMENT_QUANTIFIER_RUNE:
				openingTag.Quantifier = OneOrMoreMarkupElements
			case ZERO_OR_MORE_MARKUP_ELEMENT_QUANTIFIER_RUNE:
				openingTag.Quantifier = ZeroOrMoreMarkupElements
			}
		} else {
			p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
			openingTag.Err = &ParsingError{UnspecifiedParsingError, THERE_SHOULD_NOT_BE_SPACE_BETWEEN_THE_TAG_NAME_AND_THE_QUANTIFIER}
		}
		p.i++
		openingTag.Span.End = p.i
	}

	p.eatSpaceNewlineComment()

	//Attributes
	for p.i < p.len && p.s[p.i] != '>' && p.s[p.i] != '/' &&
		/*not start of another element*/ (p.s[p.i] != '<' || (p.i < p.len-1 && p.s[p.i+1] == '{')) {

		p.inPattern = false
		name, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForExpr: true})
		p.inPattern = true

		if isMissingExpr {
			openingTag.Attributes = append(openingTag.Attributes, &MarkupPatternAttribute{
				NodeBase: NodeBase{
					Span: name.Base().Span,
				},
				Name: name,
			})
			break
		}

		attrName := ""

		switch name := name.(type) {
		case *IdentifierLiteral:
			if name.Name == inoxconsts.HYPERSCRIPT_SCRIPT_MARKER {
				isHyperscriptScript = true
				break
			}
			attrName = name.Name
		case *UnquotedRegion:
			//ok
		default:
			if name.Base().Err == nil {
				name.BasePtr().Err = &ParsingError{UnspecifiedParsingError, MARKUP_ATTRIBUTE_NAME_SHOULD_BE_IDENT}
			}
		}

		if p.i < p.len && p.s[p.i] == '=' {
			//Parse value.

			p.tokens = append(p.tokens, Token{Type: EQUAL, SubType: MARKUP_ATTR_EQUAL, Span: NodeSpan{p.i, p.i + 1}})
			p.i++

			value, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForExpr: true})

			openingTag.Attributes = append(openingTag.Attributes, &MarkupPatternAttribute{
				NodeBase: NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name: name,
				Type: value,
			})

			if isMissingExpr {
				break
			}

			if attrName == "type" {
				strLit, ok := value.(*DoubleQuotedStringLiteral)
				isHyperscriptScript = ok && strLit.Value == mimeconsts.HYPERSCRIPT_CTYPE
			}
		} else {

			openingTag.Attributes = append(openingTag.Attributes, &MarkupPatternAttribute{
				NodeBase: NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name: name,
			})
			openingTag.Span.End = p.i
		}

		p.eatSpaceNewlineComment()
	}

	//Determine the element type.

	var estimatedRawElementType RawElementType

	switch {
	case isHyperscriptScript:
		estimatedRawElementType = HyperscriptScript
	case tagName == SCRIPT_TAG_NAME:
		estimatedRawElementType = JsScript
	case tagName == STYLE_TAG_NAME:
		estimatedRawElementType = CssStyleElem
	}

	//Handle unterminated opening tags.

	if p.i >= p.len || (p.s[p.i] != '>' && p.s[p.i] != '/') {
		openingTag.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_MARKUP_TAG_MISSING_CLOSING}

		return &MarkupPatternElement{
			NodeBase:                NodeBase{Span: NodeSpan{start, p.i}},
			Opening:                 openingTag,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	//Check for end of self closing tag.

	selfClosing := p.s[p.i] == '/'

	if selfClosing {
		if p.i >= p.len-1 || p.s[p.i+1] != '>' {
			p.tokens = append(p.tokens, Token{Type: SLASH, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			openingTag.Span.End = p.i

			openingTag.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_SELF_CLOSING_MARKUP_TAG_MISSING_CLOSING}

			return &MarkupPatternElement{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					parsingErr,
					false,
				},
				Opening:                 openingTag,
				EstimatedRawElementType: estimatedRawElementType,
			}, noOrExpectedClosingTag
		}

		p.tokens = append(p.tokens, Token{Type: SELF_CLOSING_TAG_TERMINATOR, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2

		openingTag.Span.End = p.i

		return &MarkupPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
			Opening:                 openingTag,
			Closing:                 nil,
			Children:                nil,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	p.tokens = append(p.tokens, Token{Type: GREATER_THAN, SubType: MARKUP_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	openingTag.Span.End = p.i

	//Children

	var (
		children                          []Node
		regionHeaders                     []*AnnotatedRegionHeader
		allChildrenHaveMatchingClosingTag = true
	)

	var rawElementText string
	rawStart := int32(0)
	rawEnd := int32(0)

	if rawTextElement {
		rawStart = p.i
		for p.i < p.len {
			//closing tag
			if p.s[p.i] == '<' && p.i < p.len-1 && p.s[p.i+1] == '/' {
				break
			}
			p.i++
		}
		rawEnd = p.i
		rawElementText = string(p.s[rawStart:rawEnd])
	} else {
		var err *ParsingError
		children, regionHeaders, err, allChildrenHaveMatchingClosingTag = p.parseMarkupPatternChildren(singleBracketInterpolations)
		parsingErr = err
	}

	if p.i >= p.len || p.s[p.i] != '<' {

		var err *ParsingError
		if allChildrenHaveMatchingClosingTag {
			err = &ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		}

		return &MarkupPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				err,
				false,
			},
			Opening:                 openingTag,
			RegionHeaders:           regionHeaders,
			Children:                children,
			RawElementContent:       rawElementText,
			RawElementContentStart:  rawStart,
			RawElementContentEnd:    rawEnd,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	//Closing element.

	closingTagStart := p.i
	p.tokens = append(p.tokens, Token{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{p.i, p.i + 2}})
	p.i += 2

	closingName, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForExpr: true})

	closingTag := &MarkupPatternClosingTag{
		NodeBase: NodeBase{
			Span: NodeSpan{closingTagStart, p.i},
		},
		Name: closingName,
	}

	if closing, ok := closingName.(*PatternIdentifierLiteral); !ok {
		closingTag.Err = &ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	} else if closing.Name != openingIdent.Name {
		closingTag.Err = &ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		noOrExpectedClosingTag = false
	}

	if p.i >= p.len || p.s[p.i] != '>' {
		if closingTag.Err == nil {
			closingTag.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_CLOSING_MARKUP_TAG_MISSING_CLOSING_DELIM}
		}

		return &MarkupPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
			Opening:                 openingTag,
			Closing:                 closingTag,
			RegionHeaders:           regionHeaders,
			Children:                children,
			RawElementContent:       rawElementText,
			RawElementContentStart:  rawStart,
			RawElementContentEnd:    rawEnd,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	p.tokens = append(p.tokens, Token{Type: GREATER_THAN, SubType: MARKUP_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	closingTag.Span.End = p.i

	result := &MarkupPatternElement{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Opening:                 openingTag,
		Closing:                 closingTag,
		RegionHeaders:           regionHeaders,
		Children:                children,
		RawElementContent:       rawElementText,
		RawElementContentStart:  rawStart,
		RawElementContentEnd:    rawEnd,
		EstimatedRawElementType: estimatedRawElementType,
	}

	return result, noOrExpectedClosingTag
}

func (p *parser) parseMarkupPatternChildren(singleBracketInterpolations bool) (_ []Node, _ []*AnnotatedRegionHeader, _ *ParsingError, allChildrenHaveMatchingClosingTag bool) {
	p.panicIfContextDone()

	allChildrenHaveMatchingClosingTag = true
	inInterpolation := false
	interpolationStart := int32(-1)
	children := make([]Node, 0)
	childStart := p.i
	var regionHeaders []*AnnotatedRegionHeader

	bracketPairDepthWithinInterpolation := 0

	var parsingErr *ParsingError

children_parsing_loop:
	for p.i < p.len && (inInterpolation || (p.s[p.i] != '<' || (p.i < p.len-1 && p.s[p.i+1] != '/'))) {

		//interpolation
		switch {
		case p.s[p.i] == '{' && singleBracketInterpolations:
			if inInterpolation {
				bracketPairDepthWithinInterpolation++
				p.i++
				continue
			}
			p.tokens = append(p.tokens, Token{
				Type:    OPENING_CURLY_BRACKET,
				SubType: MARKUP_INTERP_OPENING_BRACE,
				Span:    NodeSpan{p.i, p.i + 1},
			})

			// add previous slice
			raw := string(p.s[childStart:p.i])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			children = append(children, &MarkupText{
				NodeBase: NodeBase{
					NodeSpan{childStart, p.i},
					sliceErr,
					false,
				},
				Raw:   raw,
				Value: value,
			})

			inInterpolation = true
			p.i++
			interpolationStart = p.i
		case inInterpolation && p.s[p.i] == '}': //potential end of interpolation
			if bracketPairDepthWithinInterpolation > 0 {
				//still in interpolation
				bracketPairDepthWithinInterpolation--
				p.i++
				continue
			}

			closingBracketToken := Token{
				Type:    CLOSING_CURLY_BRACKET,
				SubType: MARKUP_INTERP_CLOSING_BRACE,
				Span:    NodeSpan{p.i, p.i + 1},
			}
			interpolationExclEnd := p.i
			inInterpolation = false
			p.i++
			childStart = p.i

			var interpParsingErr *ParsingError
			var expr Node

			interpolation := p.s[interpolationStart:interpolationExclEnd]

			if strings.TrimSpace(string(interpolation)) == "" {
				interpParsingErr = &ParsingError{UnspecifiedParsingError, EMPTY_MARKUP_INTERP}
			} else {
				//ignore leading & trailing space
				relativeExprStart := int32(0)
				relativeInclusiveExprEnd := int32(len32(interpolation) - 1)

				for relativeExprStart < len32(interpolation) && interpolation[relativeExprStart] == '\n' || isSpaceNotLF(interpolation[relativeExprStart]) {
					if interpolation[relativeExprStart] == '\n' {
						pos := interpolationStart + relativeExprStart
						p.tokens = append(p.tokens, Token{
							Type: NEWLINE,
							Span: NodeSpan{Start: pos, End: pos + 1},
						})
					}
					relativeExprStart++
				}

				for relativeInclusiveExprEnd > 0 && interpolation[relativeInclusiveExprEnd] == '\n' || isSpaceNotLF(interpolation[relativeInclusiveExprEnd]) {
					if interpolation[relativeInclusiveExprEnd] == '\n' {
						pos := interpolationStart + relativeInclusiveExprEnd
						p.tokens = append(p.tokens, Token{
							Type: NEWLINE,
							Span: NodeSpan{Start: pos, End: pos + 1},
						})
					}
					relativeInclusiveExprEnd--
				}

				var e Node

				unexpectedRestStart := int32(-1)
				missingExpr := false

				func() {
					//Modify the state of the parser to make it parse the interpolation.

					indexSave := p.i
					sourceSave := p.s
					lenSave := p.len

					p.i = interpolationStart + relativeExprStart
					p.s = p.s[:interpolationExclEnd]
					p.len = len32(p.s)

					defer func() {
						p.i = indexSave
						p.s = sourceSave
						p.len = lenSave
					}()

					e, missingExpr = p.parseExpression()

					p.eatSpaceNewlineComment()
					if p.i != p.len {
						unexpectedRestStart = p.i
					}
				}()

				if e != nil {
					expr = e
					if unexpectedRestStart > 0 {
						p.tokens = append(p.tokens, Token{
							Type: INVALID_INTERP_SLICE,
							Span: NodeSpan{unexpectedRestStart, interpolationExclEnd},
							Raw:  string(p.s[unexpectedRestStart:interpolationExclEnd]),
						})
						if !missingExpr {
							interpParsingErr = &ParsingError{UnspecifiedParsingError, MARKUP_INTERP_SHOULD_CONTAIN_A_SINGLE_EXPR}
						}
					}
				} else {
					interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_MARKUP_INTERP}
				}
			}
			p.tokens = append(p.tokens, closingBracketToken)

			interpolationNode := &MarkupPatternInterpolation{
				NodeBase: NodeBase{
					NodeSpan{interpolationStart, interpolationExclEnd},
					interpParsingErr,
					false,
				},
				Expr: expr,
			}
			children = append(children, interpolationNode)
		case p.s[p.i] == '<' && !inInterpolation: //child element or unquoted region

			// Add previous slice
			raw := string(p.s[childStart:p.i])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			children = append(children, &MarkupText{
				NodeBase: NodeBase{
					NodeSpan{childStart, p.i},
					sliceErr,
					false,
				},
				Raw:   raw,
				Value: value,
			})

			if p.i < p.len-1 && p.s[p.i+1] == '{' {
				unquotedRegion := p.parseUnquotedRegion()
				children = append(children, unquotedRegion)
				childStart = p.i

				continue children_parsing_loop
			}

			//Child element

			child, noOrExpectedClosingTag := p.parseMarkupPatternElement(p.i)
			children = append(children, child)
			childStart = p.i

			if !noOrExpectedClosingTag {
				allChildrenHaveMatchingClosingTag = false
				continue children_parsing_loop
			}
		case p.s[p.i] == '@' && !inInterpolation && p.i < p.len-1 && p.s[p.i+1] == '\'' && isSpace(p.s[p.i-1]): //annotated region header
			// Add previous slice
			raw := string(p.s[childStart:p.i])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			children = append(children, &MarkupText{
				NodeBase: NodeBase{
					NodeSpan{childStart, p.i},
					sliceErr,
					false,
				},
				Raw:   raw,
				Value: value,
			})

			p.parseAnnotatedRegionHeadersInMarkup(&regionHeaders)
			childStart = p.i
		case p.s[p.i] == '*' && !inInterpolation:
			// Add previous slice
			raw := string(p.s[childStart:p.i])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			children = append(children,
				&MarkupText{
					NodeBase: NodeBase{
						NodeSpan{childStart, p.i},
						sliceErr,
						false,
					},
					Raw:   raw,
					Value: value,
				},
				&MarkupPatternWildcard{
					NodeBase: NodeBase{Span: NodeSpan{p.i, p.i + 1}},
					Wildcard: MarkupStarWildcard,
				},
			)
			p.i++ //eat '*'
			childStart = p.i
		default:
			p.i++
		}
	}

	if inInterpolation {
		raw := string(p.s[interpolationStart:p.i])
		value, _ := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		children = append(children, &MarkupText{
			NodeBase: NodeBase{
				NodeSpan{interpolationStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_MARKUP_INTERP},
				false,
			},
			Raw:   raw,
			Value: value,
		})
	} else {
		raw := string(p.s[childStart:p.i])
		value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		children = append(children, &MarkupText{
			NodeBase: NodeBase{
				NodeSpan{childStart, p.i},
				sliceErr,
				false,
			},
			Raw:   raw,
			Value: value,
		})
	}

	return children, regionHeaders, parsingErr, allChildrenHaveMatchingClosingTag
}
