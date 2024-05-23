package parse

import (
	"strings"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

func (p *parser) parseMarkupPatternExpression(prefixed bool) *ast.MarkupPatternExpression {
	p.panicIfContextDone()

	start := p.i
	if prefixed {
		start = p.i - 1
		p.tokens = append(p.tokens, ast.Token{
			Type: ast.PERCENT_SYMBOL,
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
		p.tokens = append(p.tokens, ast.Token{Type: ast.LESS_THAN, SubType: ast.MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

		return &ast.MarkupPatternExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MARKUP_PATTERN_EXPRESSION_MISSING_TOP_ELEM_NAME},
				false,
			},
		}
	}

	topElem, _ := p.parseMarkupPatternElement(p.i)

	return &ast.MarkupPatternExpression{
		NodeBase: ast.NodeBase{Span: NodeSpan{start, p.i}},
		Element:  topElem,
	}
}

func (p *parser) parseMarkupPatternElement(start int32) (_ *ast.MarkupPatternElement, noOrExpectedClosingTag bool) {
	p.panicIfContextDone()

	noOrExpectedClosingTag = true

	var parsingErr *sourcecode.ParsingError
	p.tokens = append(p.tokens, ast.Token{Type: ast.LESS_THAN, SubType: ast.MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{start, start + 1}})
	p.i++

	//Parse opening tag.

	var openingIdent *ast.PatternIdentifierLiteral
	{
		start := p.i
		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		name := string(p.s[start:p.i])
		openingIdent = &ast.PatternIdentifierLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Name:       name,
			Unprefixed: true,
		}
	}

	// openingIdent, ok := openingName.(*ast.IdentifierLiteral)
	// if !ok {
	// 	parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	// }

	openingTag := &ast.MarkupPatternOpeningTag{
		NodeBase: ast.NodeBase{
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

	//Quantifier

	spaceCount := p.eatSpace()

	if p.i < p.len && isMarkupPatternElementQuantifier(p.s[p.i]) {
		if spaceCount == 0 {
			switch p.s[p.i] {
			case ast.OPTIONAL_MARKUP_ELEMENT_QUANTIFIER_RUNE:
				openingTag.Quantifier = ast.OptionalMarkupElement
			case ast.ONE_OR_MORE_MARKUP_ELEMENT_QUANTIFIER_RUNE:
				openingTag.Quantifier = ast.OneOrMoreMarkupElements
			case ast.ZERO_OR_MORE_MARKUP_ELEMENT_QUANTIFIER_RUNE:
				openingTag.Quantifier = ast.ZeroOrMoreMarkupElements
			}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
			openingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, THERE_SHOULD_NOT_BE_SPACE_BETWEEN_THE_TAG_NAME_AND_THE_QUANTIFIER}
		}
		p.i++
		openingTag.Span.End = p.i
	}

	p.eatSpaceNewlineComment()

	//Attributes
	for p.i < p.len && p.s[p.i] != '>' && p.s[p.i] != '/' &&
		/*not start of another element*/ (p.s[p.i] != '<' || (p.i < p.len-1 && p.s[p.i+1] == '{')) {

		p.inPattern = false
		name, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		p.inPattern = true

		if isMissingExpr {
			openingTag.Attributes = append(openingTag.Attributes, &ast.MarkupPatternAttribute{
				NodeBase: ast.NodeBase{
					Span: name.Base().Span,
				},
				Name: name,
			})
			break
		}

		switch name := name.(type) {
		case *ast.IdentifierLiteral:
		case *ast.UnquotedRegion:
			//ok
		default:
			if name.Base().Err == nil {
				name.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, MARKUP_ATTRIBUTE_NAME_SHOULD_BE_IDENT}
			}
		}

		if p.i < p.len && p.s[p.i] == '=' {
			//Parse value.

			p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.MARKUP_ATTR_EQUAL, Span: NodeSpan{p.i, p.i + 1}})
			p.i++

			value, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

			openingTag.Attributes = append(openingTag.Attributes, &ast.MarkupPatternAttribute{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name: name,
				Type: value,
			})

			if isMissingExpr {
				break
			}

			// if attrName == "type" {
			// 	strLit, ok := value.(*ast.DoubleQuotedStringLiteral)
			// 	hasNonJsTypeAttr = ok && strLit.Value == mimeconsts.HYPERSCRIPT_CTYPE
			// }
		} else {

			openingTag.Attributes = append(openingTag.Attributes, &ast.MarkupPatternAttribute{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name: name,
			})
			openingTag.Span.End = p.i
		}

		p.eatSpaceNewlineComment()
	}

	//Determine the element type.

	var estimatedRawElementType ast.RawElementType

	switch {
	case tagName == SCRIPT_TAG_NAME:
		estimatedRawElementType = ast.JsScript
	case tagName == STYLE_TAG_NAME:
		estimatedRawElementType = ast.CssStyleElem
	}

	//Handle unterminated opening tags.

	if p.i >= p.len || (p.s[p.i] != '>' && p.s[p.i] != '/') {
		openingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_MARKUP_TAG_MISSING_CLOSING}

		return &ast.MarkupPatternElement{
			NodeBase:                ast.NodeBase{Span: NodeSpan{start, p.i}},
			Opening:                 openingTag,
			EstimatedRawElementType: estimatedRawElementType,
		}, noOrExpectedClosingTag
	}

	//Check for end of self closing tag.

	selfClosing := p.s[p.i] == '/'

	if selfClosing {
		if p.i >= p.len-1 || p.s[p.i+1] != '>' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.SLASH, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			openingTag.Span.End = p.i

			openingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_SELF_CLOSING_MARKUP_TAG_MISSING_CLOSING}

			return &ast.MarkupPatternElement{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					parsingErr,
					false,
				},
				Opening:                 openingTag,
				EstimatedRawElementType: estimatedRawElementType,
			}, noOrExpectedClosingTag
		}

		p.tokens = append(p.tokens, ast.Token{Type: ast.SELF_CLOSING_TAG_TERMINATOR, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2

		openingTag.Span.End = p.i

		return &ast.MarkupPatternElement{
			NodeBase: ast.NodeBase{
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

	p.tokens = append(p.tokens, ast.Token{Type: ast.GREATER_THAN, SubType: ast.MARKUP_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	openingTag.Span.End = p.i

	//Children

	var (
		children                          []ast.Node
		regionHeaders                     []*ast.AnnotatedRegionHeader
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
		var err *sourcecode.ParsingError
		children, regionHeaders, err, allChildrenHaveMatchingClosingTag = p.parseMarkupPatternChildren(singleBracketInterpolations)
		parsingErr = err
	}

	if p.i >= p.len || p.s[p.i] != '<' {

		var err *sourcecode.ParsingError
		if allChildrenHaveMatchingClosingTag {
			err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		}

		return &ast.MarkupPatternElement{
			NodeBase: ast.NodeBase{
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
	p.tokens = append(p.tokens, ast.Token{Type: ast.END_TAG_OPEN_DELIMITER, Span: NodeSpan{p.i, p.i + 2}})
	p.i += 2

	closingName, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	closingTag := &ast.MarkupPatternClosingTag{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{closingTagStart, p.i},
		},
		Name: closingName,
	}

	if closing, ok := closingName.(*ast.PatternIdentifierLiteral); !ok {
		closingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	} else if closing.Name != openingIdent.Name {
		closingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		noOrExpectedClosingTag = false
	}

	if p.i >= p.len || p.s[p.i] != '>' {
		if closingTag.Err == nil {
			closingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_CLOSING_MARKUP_TAG_MISSING_CLOSING_DELIM}
		}

		return &ast.MarkupPatternElement{
			NodeBase: ast.NodeBase{
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

	p.tokens = append(p.tokens, ast.Token{Type: ast.GREATER_THAN, SubType: ast.MARKUP_TAG_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	closingTag.Span.End = p.i

	result := &ast.MarkupPatternElement{
		NodeBase: ast.NodeBase{
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

func (p *parser) parseMarkupPatternChildren(singleBracketInterpolations bool) (_ []ast.Node, _ []*ast.AnnotatedRegionHeader, _ *sourcecode.ParsingError, allChildrenHaveMatchingClosingTag bool) {
	p.panicIfContextDone()

	allChildrenHaveMatchingClosingTag = true
	inInterpolation := false
	interpolationStart := int32(-1)
	children := make([]ast.Node, 0)
	childStart := p.i
	var regionHeaders []*ast.AnnotatedRegionHeader

	bracketPairDepthWithinInterpolation := 0

	var parsingErr *sourcecode.ParsingError

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
			p.tokens = append(p.tokens, ast.Token{
				Type:    ast.OPENING_CURLY_BRACKET,
				SubType: ast.MARKUP_INTERP_OPENING_BRACE,
				Span:    NodeSpan{p.i, p.i + 1},
			})

			// add previous slice
			raw := string(p.s[childStart:p.i])
			children = append(children, ast.NewMarkupText(NodeSpan{childStart, p.i}, raw))

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

			closingBracketToken := ast.Token{
				Type:    ast.CLOSING_CURLY_BRACKET,
				SubType: ast.MARKUP_INTERP_CLOSING_BRACE,
				Span:    NodeSpan{p.i, p.i + 1},
			}
			interpolationExclEnd := p.i
			inInterpolation = false
			p.i++
			childStart = p.i

			var interpParsingErr *sourcecode.ParsingError
			var expr ast.Node

			interpolation := p.s[interpolationStart:interpolationExclEnd]

			if strings.TrimSpace(string(interpolation)) == "" {
				interpParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, EMPTY_MARKUP_INTERP}
			} else {
				//ignore leading & trailing space
				relativeExprStart := int32(0)
				relativeInclusiveExprEnd := int32(len32(interpolation) - 1)

				for relativeExprStart < len32(interpolation) && interpolation[relativeExprStart] == '\n' || isSpaceNotLF(interpolation[relativeExprStart]) {
					if interpolation[relativeExprStart] == '\n' {
						pos := interpolationStart + relativeExprStart
						p.tokens = append(p.tokens, ast.Token{
							Type: ast.NEWLINE,
							Span: NodeSpan{Start: pos, End: pos + 1},
						})
					}
					relativeExprStart++
				}

				for relativeInclusiveExprEnd > 0 && interpolation[relativeInclusiveExprEnd] == '\n' || isSpaceNotLF(interpolation[relativeInclusiveExprEnd]) {
					if interpolation[relativeInclusiveExprEnd] == '\n' {
						pos := interpolationStart + relativeInclusiveExprEnd
						p.tokens = append(p.tokens, ast.Token{
							Type: ast.NEWLINE,
							Span: NodeSpan{Start: pos, End: pos + 1},
						})
					}
					relativeInclusiveExprEnd--
				}

				var e ast.Node

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
						p.tokens = append(p.tokens, ast.Token{
							Type: ast.INVALID_INTERP_SLICE,
							Span: NodeSpan{unexpectedRestStart, interpolationExclEnd},
							Raw:  string(p.s[unexpectedRestStart:interpolationExclEnd]),
						})
						if !missingExpr {
							interpParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, MARKUP_INTERP_SHOULD_CONTAIN_A_SINGLE_EXPR}
						}
					}
				} else {
					interpParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_MARKUP_INTERP}
				}
			}
			p.tokens = append(p.tokens, closingBracketToken)

			interpolationNode := &ast.MarkupPatternInterpolation{
				NodeBase: ast.NodeBase{
					NodeSpan{interpolationStart, interpolationExclEnd},
					interpParsingErr,
					false,
				},
				Expr: expr,
			}
			children = append(children, interpolationNode)
		case p.s[p.i] == '<' && !inInterpolation: //child element or unquoted region

			// ast.Add previous slice
			raw := string(p.s[childStart:p.i])
			children = append(children, ast.NewMarkupText(NodeSpan{childStart, p.i}, raw))

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
			// ast.Add previous slice
			raw := string(p.s[childStart:p.i])
			children = append(children, ast.NewMarkupText(NodeSpan{childStart, p.i}, raw))

			p.parseAnnotatedRegionHeadersInMarkup(&regionHeaders)
			childStart = p.i
		case p.s[p.i] == '*' && !inInterpolation:
			// ast.Add previous slice
			raw := string(p.s[childStart:p.i])
			text := ast.NewMarkupText(NodeSpan{childStart, p.i}, raw)

			children = append(children,
				text,
				&ast.MarkupPatternWildcard{
					NodeBase: ast.NodeBase{Span: NodeSpan{p.i, p.i + 1}},
					Wildcard: ast.MarkupStarWildcard,
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
		text := ast.NewMarkupText(NodeSpan{Start: interpolationStart, End: p.i}, raw)
		text.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MARKUP_INTERP}

		children = append(children, text)
	} else {
		raw := string(p.s[childStart:p.i])
		children = append(children, ast.NewMarkupText(NodeSpan{childStart, p.i}, raw))
	}

	return children, regionHeaders, parsingErr, allChildrenHaveMatchingClosingTag
}
