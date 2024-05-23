package parse

import (
	"strings"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

const (
	SCRIPT_TAG_NAME = "script"
	STYLE_TAG_NAME  = "style"
)

func (p *parser) parseMarkupExpression(namespaceIdent *ast.IdentifierLiteral /* can be nil */, start int32) *ast.MarkupExpression {
	p.panicIfContextDone()

	var namespace ast.Node
	if namespaceIdent != nil {
		namespace = namespaceIdent
	}

	//we do not increment because we keep the '<' for parsing the top element

	if p.i+1 >= p.len || !isAlpha(p.s[p.i+1]) {
		p.tokens = append(p.tokens, ast.Token{Type: ast.LESS_THAN, SubType: ast.MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

		return &ast.MarkupExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MARKUP_EXPRESSION_MISSING_TOP_ELEM_NAME},
				false,
			},
			Namespace: namespace,
		}
	}

	topElem, _ := p.parseMarkupElement(p.i)

	return &ast.MarkupExpression{
		NodeBase:  ast.NodeBase{Span: NodeSpan{start, p.i}},
		Namespace: namespace,
		Element:   topElem,
	}
}

func (p *parser) parseMarkupElement(start int32) (_ *ast.MarkupElement, noOrExpectedClosingTag bool) {
	p.panicIfContextDone()

	noOrExpectedClosingTag = true

	var parsingErr *sourcecode.ParsingError
	p.tokens = append(p.tokens, ast.Token{Type: ast.LESS_THAN, SubType: ast.MARKUP_TAG_OPENING_BRACKET, Span: NodeSpan{start, start + 1}})
	p.i++

	//Parse opening tag.

	var openingIdent *ast.IdentifierLiteral
	{
		start := p.i
		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		name := string(p.s[start:p.i])
		openingIdent = &ast.IdentifierLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Name: name,
		}
	}

	// openingIdent, ok := openingName.(*ast.IdentifierLiteral)
	// if !ok {
	// 	parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	// }

	p.eatSpaceNewlineComment()

	openingTag := &ast.MarkupOpeningTag{
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

	unterminatedHyperscriptAttribute := false //used to avoid reporting too many errors.

	//Attributes
	for p.i < p.len && p.s[p.i] != '>' && p.s[p.i] != '/' &&
		/*not start of another element*/ (p.s[p.i] != '<' || (p.i < p.len-1 && p.s[p.i+1] == '{')) {
		if p.s[p.i] == '{' { //underscore attribute shortand
			//TODO
			for p.i < p.len && p.s[p.i] != '}' {
				p.i++
			}
			if p.i < p.len && p.s[p.i] == '}' {
				p.i++
				continue
			}
			// terminated := false
			// if !terminated {
			// 	unterminatedHyperscriptAttribute = true
			// }
			// openingTag.Attributes = append(openingTag.Attributes, attr)
			// p.eatSpaceNewlineComment()
			// openingTag.Span.End = p.i
			// continue
		}
		unterminatedHyperscriptAttribute = false

		name, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		if isMissingExpr {
			openingTag.Attributes = append(openingTag.Attributes, &ast.MarkupAttribute{
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

			value, isMissingExpr := p.parseExpression(exprParsingConfig{
				disallowUnparenthesizedBinForPipelineExprs: true,
			})

			openingTag.Attributes = append(openingTag.Attributes, &ast.MarkupAttribute{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name:  name,
				Value: value,
			})

			if isMissingExpr {
				break
			}

		} else {

			openingTag.Attributes = append(openingTag.Attributes, &ast.MarkupAttribute{
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
		if !unterminatedHyperscriptAttribute { //Avoid reporting two errors.
			openingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_MARKUP_TAG_MISSING_CLOSING}
		}

		return &ast.MarkupElement{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
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

			return &ast.MarkupElement{
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

		return &ast.MarkupElement{
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
		children, regionHeaders, err, allChildrenHaveMatchingClosingTag = p.parseMarkupChildren(singleBracketInterpolations)
		parsingErr = err
	}

	if p.i >= p.len || p.s[p.i] != '<' {

		var err *sourcecode.ParsingError
		if allChildrenHaveMatchingClosingTag {
			err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		}

		return &ast.MarkupElement{
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

	closingTag := &ast.MarkupClosingTag{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{closingTagStart, p.i},
		},
		Name: closingName,
	}

	if closing, ok := closingName.(*ast.IdentifierLiteral); !ok {
		closingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	} else if closing.Name != openingIdent.Name {
		closingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
		noOrExpectedClosingTag = false
	}

	if p.i >= p.len || p.s[p.i] != '>' {
		if closingTag.Err == nil {
			closingTag.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_CLOSING_MARKUP_TAG_MISSING_CLOSING_DELIM}
		}

		return &ast.MarkupElement{
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

	result := &ast.MarkupElement{
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

func (p *parser) parseMarkupChildren(singleBracketInterpolations bool) (_ []ast.Node, _ []*ast.AnnotatedRegionHeader, _ *sourcecode.ParsingError, allChildrenHaveMatchingClosingTag bool) {
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

			interpolationNode := &ast.MarkupInterpolation{
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

			child, noOrExpectedClosingTag := p.parseMarkupElement(p.i)
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
		default:
			p.i++
		}
	}

	if inInterpolation {
		raw := string(p.s[interpolationStart:p.i])
		text := ast.NewMarkupText(NodeSpan{interpolationStart, p.i}, raw)
		text.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MARKUP_INTERP}

		children = append(children, text)
	} else {
		raw := string(p.s[childStart:p.i])
		children = append(children, ast.NewMarkupText(NodeSpan{childStart, p.i}, raw))
	}

	return children, regionHeaders, parsingErr, allChildrenHaveMatchingClosingTag
}
