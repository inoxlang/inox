package parse

import (
	"strconv"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

type sequencePatternPieceKind int

const (
	rootSequencePatternPiece sequencePatternPieceKind = iota
	parenthesizedSequencePatternPiece
	unionCaseSequencePatternPiece
)

// parseComplexStringPatternPiece parses a piece (of string pattern) that can have one ore more elements.
func (p *parser) parseComplexStringPatternPiece(start int32, kind sequencePatternPieceKind, ident *ast.PatternIdentifierLiteral) *ast.ComplexStringPatternPiece {
	p.panicIfContextDone()

	unprefixed := false

	switch kind {
	case rootSequencePatternPiece:
		unprefixed = ident.Unprefixed

		if unprefixed {
			p.tokens = append(p.tokens,
				ast.Token{Type: ast.UNPREFIXED_STR, Span: ident.Span},
				ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{Start: ident.Span.End, End: ident.Span.End + 1}},
			)
		} else {
			p.tokens = append(p.tokens,
				ast.Token{Type: ast.PERCENT_STR, Span: ident.Span},
				ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{ident.Span.End, ident.Span.End + 1}},
			)
		}
	case parenthesizedSequencePatternPiece:
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{start, start + 1}})
	}

	var parsingErr *sourcecode.ParsingError
	var elements []*ast.PatternPieceElement

	for p.i < p.len && p.s[p.i] != ')' && (kind != unionCaseSequencePatternPiece || p.s[p.i] != '|') {

		p.eatSpaceNewlineComment()

		if p.i >= p.len || p.s[p.i] == ')' || (kind == unionCaseSequencePatternPiece && p.s[p.i] == '|') {
			break
		}

		if p.s[p.i] == '|' {
			union := p.parseComplexStringPatternUnion(p.i, true)
			elements = append(elements, &ast.PatternPieceElement{
				NodeBase:   union.NodeBase,
				Quantifier: ast.ExactlyOneOccurrence,
				Expr:       union,
			})
			break
		}

		elementStart := p.i
		ocurrenceModifier := ast.ExactlyOneOccurrence
		count := 0
		elementEnd := int32(-1)
		var groupName *ast.PatternGroupName
		var elemParsingErr *sourcecode.ParsingError
		var element ast.Node

		if isAlpha(p.s[p.i]) { //group name or pattern name
			isGroupName := false
			j := int32(p.i + 1)

			for ; j < p.len; j++ {
				if isAlpha(p.s[j]) || isDecDigit(p.s[j]) || p.s[j] == '_' || p.s[j] == '-' {
					continue
				}
				if p.s[j] == ':' {
					isGroupName = true
				}
				break
			}

			if isGroupName {

				p.i = j
				groupName = &ast.PatternGroupName{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{elementStart, p.i},
					},
					Name: string(p.s[elementStart:p.i]),
				}
				if groupName.Name[len(groupName.Name)-1] == '-' {
					groupName.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_GROUP_NAME_SHOULD_NOT_END_WITH_DASH}
				}

				p.tokens = append(p.tokens, ast.Token{Type: ast.COLON, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				p.eatSpace()
			}
		}

		element = p.parseComplexStringPatternElement()
		elementEnd = p.i

		if p.i < p.len && (p.s[p.i] == '+' || p.s[p.i] == '*' || p.s[p.i] == '?' || p.s[p.i] == '=') {
			switch p.s[p.i] {
			case '+':
				ocurrenceModifier = ast.AtLeastOneOccurrence
				elementEnd++
				p.i++

				p.tokens = append(p.tokens, ast.Token{Type: ast.OCCURRENCE_MODIFIER, Span: NodeSpan{p.i - 1, p.i}, Raw: "+"})
			case '*':
				ocurrenceModifier = ast.ZeroOrMoreOccurrences
				elementEnd++
				p.i++

				p.tokens = append(p.tokens, ast.Token{Type: ast.OCCURRENCE_MODIFIER, Span: NodeSpan{p.i - 1, p.i}, Raw: "*"})
			case '?':
				ocurrenceModifier = ast.OptionalOccurrence
				elementEnd++
				p.i++

				p.tokens = append(p.tokens, ast.Token{Type: ast.OCCURRENCE_MODIFIER, Span: NodeSpan{p.i - 1, p.i}, Raw: "?"})
			case '=':
				p.i++
				numberStart := p.i
				if p.i >= p.len || !isDecDigit(p.s[p.i]) {
					elemParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT_UNTERMINATED_EXACT_OCURRENCE_COUNT}
					elementEnd = p.i
					goto after_ocurrence
				}

				for p.i < p.len && isDecDigit(p.s[p.i]) {
					p.i++
				}

				_count, err := strconv.ParseUint(string(p.s[numberStart:p.i]), 10, 32)
				if err != nil {
					elemParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_PATTERN_INVALID_OCCURENCE_COUNT}
				}
				count = int(_count)
				ocurrenceModifier = ast.ExactOccurrenceCount
				elementEnd = p.i

				raw := string(p.s[numberStart-1 : p.i])
				p.tokens = append(p.tokens, ast.Token{Type: ast.OCCURRENCE_MODIFIER, Span: NodeSpan{numberStart - 1, p.i}, Raw: raw})
			}
		}

	after_ocurrence:

		elements = append(elements, &ast.PatternPieceElement{
			NodeBase: ast.NodeBase{
				NodeSpan{elementStart, elementEnd},
				elemParsingErr,
				false,
			},
			Quantifier:          ocurrenceModifier,
			ExactOcurrenceCount: int(count),
			Expr:                element,
			GroupName:           groupName,
		})

	}

	if kind != unionCaseSequencePatternPiece {
		if p.i >= p.len || p.s[p.i] != ')' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_COMPLEX_STRING_PATT_MISSING_CLOSING_BRACKET}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	}

	return &ast.ComplexStringPatternPiece{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Unprefixed: unprefixed,
		Elements:   elements,
	}
}

func (p *parser) parseComplexStringPatternElement() ast.Node {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *sourcecode.ParsingError

	if p.i >= p.len || p.s[p.i] == ')' || p.s[p.i] == '|' {
		return &ast.InvalidComplexStringPatternElement{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, fmtAPatternWasExpectedHere(p.s, p.i)},
				false,
			},
		}
	}

	switch {
	case p.s[p.i] == '(':
		elemStart := p.i
		p.i++

		if p.i >= p.len || p.s[p.i] == ')' {
			return &ast.InvalidComplexStringPatternElement{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_PATTERN_ELEMENT},
					false,
				},
			}
		}

		if p.s[p.i] == '|' { //parenthesized union
			element := p.parseComplexStringPatternUnion(elemStart, false)

			return element
		}

		return p.parseComplexStringPatternPiece(elemStart, parenthesizedSequencePatternPiece, nil)
	case p.s[p.i] == '"' || p.s[p.i] == '`' || p.s[p.i] == '\'': //string and rune literals
		e, _ := p.parseExpression(exprParsingConfig{
			disallowUnparenthesizedBinForPipelineExprs: true,
			disallowParsingSeveralPatternUnionCases:    true,
		})
		return e
	case p.s[p.i] == '-' || isDecDigit(p.s[p.i]):
		e, _ := p.parseExpression(exprParsingConfig{
			disallowUnparenthesizedBinForPipelineExprs: true,
			disallowParsingSeveralPatternUnionCases:    true,
		})
		switch e.(type) {
		case *ast.IntegerRangeLiteral:
		default:
			return &ast.InvalidComplexStringPatternElement{
				NodeBase: ast.NodeBase{
					e.Base().Span,
					&sourcecode.ParsingError{UnspecifiedParsingError, INVALID_COMPLEX_PATTERN_ELEMENT},
					false,
				},
			}
		}
		return e
	case isAlpha(p.s[p.i]):
		patternIdent := &ast.PatternIdentifierLiteral{
			NodeBase:   ast.NodeBase{Span: NodeSpan{start, start + 1}},
			Unprefixed: true,
		}

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		patternIdent.NodeBase.Span.End = p.i
		patternIdent.Name = string(p.s[start:p.i])
		return patternIdent
	case p.i < p.len-1 && p.s[p.i] == '%' && p.s[p.i+1] == '`': //regex literal
		return p.parsePercentPrefixedPattern(false)
	default:

		for p.i < p.len && !IsDelim(p.s[p.i]) && p.s[p.i] != '"' && p.s[p.i] != '\'' {
			if parsingErr == nil {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_COMPLEX_PATTERN_ELEMENT}
			}
			p.i++
		}
	}

	if parsingErr == nil && p.i == start {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtAPatternWasExpectedHere(p.s, p.i)}
		if p.s[p.i] != ')' {
			p.i++
		}
	}

	return &ast.InvalidComplexStringPatternElement{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
	}
}

func (p *parser) parseComplexStringPatternUnion(start int32, isShorthandUnion bool) *ast.PatternUnion {
	p.panicIfContextDone()

	var cases []ast.Node

	if !isShorthandUnion {
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{start, start + 1}})
	}

	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewlineComment()

		if p.i >= p.len || p.s[p.i] == ')' {
			break
		}

		if p.s[p.i] != '|' {

			for p.i < p.len && p.s[p.i] != ')' {
				p.i++
			}

			return &ast.PatternUnion{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, INVALID_PATT_UNION_ELEMENT_SEPARATOR_EXPLANATION},
					false,
				},
				Cases: cases,
			}
		}
		p.tokens = append(p.tokens, ast.Token{Type: ast.PIPE, SubType: ast.STRING_PATTERN_UNION_PIPE, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpaceNewlineComment()

		if p.i >= p.len || p.s[p.i] == ')' || p.s[p.i] == '|' {
			cases = append(cases, &ast.InvalidComplexStringPatternElement{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtAPatternWasExpectedHere(p.s, p.i)},
					false,
				},
			})
		} else {
			pieceStart := p.i

			piece := p.parseComplexStringPatternPiece(pieceStart, unionCaseSequencePatternPiece, nil)
			var case_ ast.Node = piece

			//Simplify if the piece contains a single element that does not have a name and is not parenthesized.
			if len(piece.Elements) == 1 && !piece.IsParenthesized {
				elem := piece.Elements[0]
				if elem.Quantifier == ast.ExactlyOneOccurrence && elem.GroupName == nil && !elem.IsParenthesized {
					case_ = elem.Expr
				}
			}
			cases = append(cases, case_)
		}

	}

	if isShorthandUnion {
		return &ast.PatternUnion{
			NodeBase: ast.NodeBase{Span: NodeSpan{start, p.i}},
			Cases:    cases,
		}
	}

	var parsingErr *sourcecode.ParsingError

	if p.i >= p.len || p.s[p.i] != ')' {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_UNION_MISSING_CLOSING_PAREN}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &ast.PatternUnion{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Cases: cases,
	}
}
