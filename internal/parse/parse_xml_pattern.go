package parse

import (
	"strings"

	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mimeconsts"
)

func (p *parser) parseXMLPatternExpression(prefixed bool) *XMLPatternExpression {
	p.panicIfContextDone()

	start := p.i
	if prefixed {
		start = p.i - 1
	}

	if !p.inPattern {
		p.inPattern = true
		defer func() {
			p.inPattern = false
		}()
	}

	//we do not increment because we keep the '<' for parsing the top element

	if p.i+1 >= p.len || !isAlpha(p.s[p.i+1]) {
		p.tokens = append(p.tokens, Token{Type: LESS_THAN, SubType: XML_TAG_OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

		return &XMLPatternExpression{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_XML_PATTERN_EXPRESSION_MISSING_TOP_ELEM_NAME},
				false,
			},
		}
	}

	topElem, _ := p.parseXMLPatternElement(p.i)

	return &XMLPatternExpression{
		NodeBase: NodeBase{Span: NodeSpan{start, p.i}},
		Element:  topElem,
	}
}

func (p *parser) parseXMLPatternElement(start int32) (_ *XMLPatternElement, noOrExpectedClosingTag bool) {
	p.panicIfContextDone()

	noOrExpectedClosingTag = true

	var parsingErr *ParsingError
	p.tokens = append(p.tokens, Token{Type: LESS_THAN, SubType: XML_TAG_OPENING_BRACKET, Span: NodeSpan{start, start + 1}})
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

	p.eatSpaceNewlineComment()

	openingElement := &XMLPatternOpeningElement{
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

	//Attributes
	for p.i < p.len && p.s[p.i] != '>' && p.s[p.i] != '/' &&
		/*not start of another element*/ (p.s[p.i] != '<' || (p.i < p.len-1 && p.s[p.i+1] == '{')) {

		p.inPattern = false
		name, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})
		p.inPattern = true

		if isMissingExpr {
			openingElement.Attributes = append(openingElement.Attributes, &XMLAttribute{
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
				name.BasePtr().Err = &ParsingError{UnspecifiedParsingError, XML_ATTRIBUTE_NAME_SHOULD_BE_IDENT}
			}
		}

		if p.i < p.len && p.s[p.i] == '=' {
			//Parse value.

			p.tokens = append(p.tokens, Token{Type: EQUAL, SubType: XML_ATTR_EQUAL, Span: NodeSpan{p.i, p.i + 1}})
			p.i++

			value, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

			openingElement.Attributes = append(openingElement.Attributes, &XMLAttribute{
				NodeBase: NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name:  name,
				Value: value,
			})

			if isMissingExpr {
				break
			}

			if attrName == "type" {
				strLit, ok := value.(*DoubleQuotedStringLiteral)
				isHyperscriptScript = ok && strLit.Value == mimeconsts.HYPERSCRIPT_CTYPE
			}
		} else {

			openingElement.Attributes = append(openingElement.Attributes, &XMLAttribute{
				NodeBase: NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name: name,
			})
			openingElement.Span.End = p.i
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
		openingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING}

		return &XMLPatternElement{
			NodeBase:                NodeBase{Span: NodeSpan{start, p.i}},
			Opening:                 openingElement,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	//Check for end of self closing tag.

	selfClosing := p.s[p.i] == '/'

	if selfClosing {
		if p.i >= p.len-1 || p.s[p.i+1] != '>' {
			p.tokens = append(p.tokens, Token{Type: SLASH, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			openingElement.Span.End = p.i

			openingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_SELF_CLOSING_XML_TAG_MISSING_CLOSING}

			return &XMLPatternElement{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					parsingErr,
					false,
				},
				Opening:                 openingElement,
				EstimatedRawElementType: estimatedRawElementType,
			}, noOrExpectedClosingTag
		}

		p.tokens = append(p.tokens, Token{Type: SELF_CLOSING_TAG_TERMINATOR, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2

		openingElement.Span.End = p.i

		return &XMLPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
			Opening:                 openingElement,
			Closing:                 nil,
			Children:                nil,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	p.tokens = append(p.tokens, Token{Type: GREATER_THAN, SubType: XML_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	openingElement.Span.End = p.i

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
		children, regionHeaders, err, allChildrenHaveMatchingClosingTag = p.parseXMLPatternChildren(singleBracketInterpolations)
		parsingErr = err
	}

	if p.i >= p.len || p.s[p.i] != '<' {

		var err *ParsingError
		if allChildrenHaveMatchingClosingTag {
			err = &ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		}

		return &XMLPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				err,
				false,
			},
			Opening:                 openingElement,
			RegionHeaders:           regionHeaders,
			Children:                children,
			RawElementContent:       rawElementText,
			RawElementContentStart:  rawStart,
			RawElementContentEnd:    rawEnd,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	//Closing element.

	closingElemStart := p.i
	p.tokens = append(p.tokens, Token{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{p.i, p.i + 2}})
	p.i += 2

	closingName, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

	closingElement := &XMLPatternClosingElement{
		NodeBase: NodeBase{
			Span: NodeSpan{closingElemStart, p.i},
		},
		Name: closingName,
	}

	if closing, ok := closingName.(*PatternIdentifierLiteral); !ok {
		closingElement.Err = &ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	} else if closing.Name != openingIdent.Name {
		closingElement.Err = &ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		noOrExpectedClosingTag = false
	}

	if p.i >= p.len || p.s[p.i] != '>' {
		if closingElement.Err == nil {
			closingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_CLOSING_XML_TAG_MISSING_CLOSING_DELIM}
		}

		return &XMLPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
			Opening:                 openingElement,
			Closing:                 closingElement,
			RegionHeaders:           regionHeaders,
			Children:                children,
			RawElementContent:       rawElementText,
			RawElementContentStart:  rawStart,
			RawElementContentEnd:    rawEnd,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	p.tokens = append(p.tokens, Token{Type: GREATER_THAN, SubType: XML_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	closingElement.Span.End = p.i

	result := &XMLPatternElement{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Opening:                 openingElement,
		Closing:                 closingElement,
		RegionHeaders:           regionHeaders,
		Children:                children,
		RawElementContent:       rawElementText,
		RawElementContentStart:  rawStart,
		RawElementContentEnd:    rawEnd,
		EstimatedRawElementType: estimatedRawElementType,
	}

	return result, noOrExpectedClosingTag
}

func (p *parser) parseXMLPatternChildren(singleBracketInterpolations bool) (_ []Node, _ []*AnnotatedRegionHeader, _ *ParsingError, allChildrenHaveMatchingClosingTag bool) {
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
				SubType: XML_INTERP_OPENING_BRACE,
				Span:    NodeSpan{p.i, p.i + 1},
			})

			// add previous slice
			raw := string(p.s[childStart:p.i])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			children = append(children, &XMLText{
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
				SubType: XML_INTERP_CLOSING_BRACE,
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
				interpParsingErr = &ParsingError{UnspecifiedParsingError, EMPTY_XML_INTERP}
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
							interpParsingErr = &ParsingError{UnspecifiedParsingError, XML_INTERP_SHOULD_CONTAIN_A_SINGLE_EXPR}
						}
					}
				} else {
					interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_XML_INTERP}
				}
			}
			p.tokens = append(p.tokens, closingBracketToken)

			interpolationNode := &XMLPatternInterpolation{
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
			children = append(children, &XMLText{
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

			child, noOrExpectedClosingTag := p.parseXMLPatternElement(p.i)
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
			children = append(children, &XMLText{
				NodeBase: NodeBase{
					NodeSpan{childStart, p.i},
					sliceErr,
					false,
				},
				Raw:   raw,
				Value: value,
			})

			p.parseAnnotatedRegionHeadersInXML(&regionHeaders)
			childStart = p.i
		default:
			p.i++
		}
	}

	if inInterpolation {
		raw := string(p.s[interpolationStart:p.i])
		value, _ := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		children = append(children, &XMLText{
			NodeBase: NodeBase{
				NodeSpan{interpolationStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_XML_INTERP},
				false,
			},
			Raw:   raw,
			Value: value,
		})
	} else {
		raw := string(p.s[childStart:p.i])
		value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		children = append(children, &XMLText{
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
