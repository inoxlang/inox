package parse

import (
	"strings"

	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mimeconsts"
)

const (
	SCRIPT_TAG_NAME = "script"
	STYLE_TAG_NAME  = "style"
)

func (p *parser) parseMarkupExpression(namespaceIdent *IdentifierLiteral /* can be nil */, start int32) *MarkupExpression {
	p.panicIfContextDone()

	var namespace Node
	if namespaceIdent != nil {
		namespace = namespaceIdent
	}

	//we do not increment because we keep the '<' for parsing the top element

	if p.i+1 >= p.len || !isAlpha(p.s[p.i+1]) {
		p.tokens = append(p.tokens, Token{Type: LESS_THAN, SubType: MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

		return &MarkupExpression{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_MARKUP_EXPRESSION_MISSING_TOP_ELEM_NAME},
				false,
			},
			Namespace: namespace,
		}
	}

	topElem, _ := p.parseMarkupElement(p.i)

	return &MarkupExpression{
		NodeBase:  NodeBase{Span: NodeSpan{start, p.i}},
		Namespace: namespace,
		Element:   topElem,
	}
}

func (p *parser) parseMarkupElement(start int32) (_ *MarkupElement, noOrExpectedClosingTag bool) {
	p.panicIfContextDone()

	noOrExpectedClosingTag = true

	var parsingErr *ParsingError
	p.tokens = append(p.tokens, Token{Type: LESS_THAN, SubType: MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{start, start + 1}})
	p.i++

	//Parse opening tag.

	var openingIdent *IdentifierLiteral
	{
		start := p.i
		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		name := string(p.s[start:p.i])
		openingIdent = &IdentifierLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Name: name,
		}
	}

	// openingIdent, ok := openingName.(*IdentifierLiteral)
	// if !ok {
	// 	parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	// }

	p.eatSpaceNewlineComment()

	openingElement := &MarkupOpeningTag{
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

	unterminatedHyperscriptAttribute := false //used to avoid reporting too many errors.
	isHyperscriptScript := false

	//Attributes
	for p.i < p.len && p.s[p.i] != '>' && p.s[p.i] != '/' &&
		/*not start of another element*/ (p.s[p.i] != '<' || (p.i < p.len-1 && p.s[p.i+1] == '{')) {
		if p.s[p.i] == '{' { //underscore attribute shortand
			attr, terminated := p.parseHyperscriptAttribute(p.i)
			if !terminated {
				unterminatedHyperscriptAttribute = true
			}
			openingElement.Attributes = append(openingElement.Attributes, attr)
			p.eatSpaceNewlineComment()
			openingElement.Span.End = p.i
			continue
		}
		unterminatedHyperscriptAttribute = false

		name, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

		if isMissingExpr {
			openingElement.Attributes = append(openingElement.Attributes, &MarkupAttribute{
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

			value, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

			openingElement.Attributes = append(openingElement.Attributes, &MarkupAttribute{
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

			openingElement.Attributes = append(openingElement.Attributes, &MarkupAttribute{
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
		if !unterminatedHyperscriptAttribute { //Avoid reporting two errors.
			openingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_MARKUP_TAG_MISSING_CLOSING}
		}

		return &MarkupElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
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

			openingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_SELF_CLOSING_MARKUP_TAG_MISSING_CLOSING}

			return &MarkupElement{
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

		return &MarkupElement{
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

	p.tokens = append(p.tokens, Token{Type: GREATER_THAN, SubType: MARKUP_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
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
		children, regionHeaders, err, allChildrenHaveMatchingClosingTag = p.parseXMLChildren(singleBracketInterpolations)
		parsingErr = err
	}

	if p.i >= p.len || p.s[p.i] != '<' {

		var err *ParsingError
		if allChildrenHaveMatchingClosingTag {
			err = &ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		}

		return &MarkupElement{
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

	closingElement := &MarkupClosingTag{
		NodeBase: NodeBase{
			Span: NodeSpan{closingElemStart, p.i},
		},
		Name: closingName,
	}

	if closing, ok := closingName.(*IdentifierLiteral); !ok {
		closingElement.Err = &ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	} else if closing.Name != openingIdent.Name {
		closingElement.Err = &ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		noOrExpectedClosingTag = false
	}

	if p.i >= p.len || p.s[p.i] != '>' {
		if closingElement.Err == nil {
			closingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_CLOSING_MARKUP_TAG_MISSING_CLOSING_DELIM}
		}

		return &MarkupElement{
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
			RawElementParsingResult: rawEnd,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	p.tokens = append(p.tokens, Token{Type: GREATER_THAN, SubType: MARKUP_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	closingElement.Span.End = p.i

	result := &MarkupElement{
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

	if rawElementText != "" {
		p.parseContentOfRawMarkupElement(result)
	}

	return result, noOrExpectedClosingTag
}

func (p *parser) parseHyperscriptAttribute(start int32) (attr *HyperscriptAttributeShorthand, terminated bool) {
	p.tokens = append(p.tokens, Token{
		Type:    OPENING_CURLY_BRACKET,
		SubType: UNDERSCORE_ATTR_SHORTHAND_OPENING_BRACE,
		Span:    NodeSpan{p.i, p.i + 1},
	})

	p.i++

	end := int32(-1)
	closingCurlyBracketPosition := int32(-1)

	for p.i < p.len {
		if p.s[p.i] == '}' {
			closingCurlyBracketPosition = p.i
			end = p.i + 1 //potential end
			p.i++
			p.eatSpaceNewline()
			if p.i >= p.len {
				break
			}

			r := p.s[p.i]

			if r == '.' || r == '>' /*end of opening tag*/ {
				p.tokens = append(p.tokens, Token{
					Type:    CLOSING_CURLY_BRACKET,
					SubType: UNDERSCORE_ATTR_SHORTHAND_CLOSING_BRACE,
					Span:    NodeSpan{closingCurlyBracketPosition, closingCurlyBracketPosition + 1},
				})

				if r == '.' {
					p.tokens = append(p.tokens, Token{
						Type: DOT,
						Span: NodeSpan{p.i, p.i + 1},
					})
					p.i++
				}
				break
			}
			end = -1
			closingCurlyBracketPosition = -1
		}
		p.i++
	}

	terminated = end >= 0
	if !terminated {
		end = p.i
	}

	codeEnd := end
	if closingCurlyBracketPosition > 0 {
		codeEnd = closingCurlyBracketPosition
	}

	value := string(p.s[start+1 : codeEnd])

	attr = &HyperscriptAttributeShorthand{
		NodeBase: NodeBase{
			Span: NodeSpan{start, end},
		},
		IsUnterminated: !terminated,
		Value:          value,
	}

	if !terminated {
		attr.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_HYPERSCRIPT_ATTRIBUTE_SHORTHAND}
	}

	if terminated && p.parseHyperscript != nil {
		result, parsingErr, err := p.parseHyperscript(p.context, value)

		if attr.Err == nil {
			if err != nil {
				attr.Err = &ParsingError{UnspecifiedParsingError, HYPERSCRIPT_PARSING_ERROR_PREFIX + err.Error()}
			}
			if parsingErr != nil {
				attr.HyperscriptParsingError = parsingErr
			}
		}

		if result != nil {
			attr.HyperscriptParsingResult = result
		}
	}

	return
}

func (p *parser) parseXMLChildren(singleBracketInterpolations bool) (_ []Node, _ []*AnnotatedRegionHeader, _ *ParsingError, allChildrenHaveMatchingClosingTag bool) {
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

					switch {
					case p.i < p.len-2 && p.s[p.i] == 'i' && p.s[p.i+1] == 'f' && !IsIdentChar(p.s[p.i+2]):
						//Parse if expression without parentheses.
						ifKeywordStart := p.i
						p.i += 2
						p.eatSpace()
						e = p.parseIfExpression(-1, ifKeywordStart)
					case p.i < p.len-3 && p.s[p.i] == 'f' && p.s[p.i+1] == 'o' && p.s[p.i+2] == 'r' && !IsIdentChar(p.s[p.i+3]):
						//Parse for expression without parentheses.
						forKeywordStart := p.i
						p.i += 3
						p.eatSpace()
						e = p.parseForExpression(-1, forKeywordStart)
					default:
						e, missingExpr = p.parseExpression()
					}

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

			interpolationNode := &MarkupInterpolation{
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

			child, noOrExpectedClosingTag := p.parseMarkupElement(p.i)
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
