package parse

import "strconv"

type sequencePatternPieceKind int

const (
	rootSequencePatternPiece sequencePatternPieceKind = iota
	parenthesizedSequencePatternPiece
	unionCaseSequencePatternPiece
)

// parseComplexStringPatternPiece parses a piece (of string pattern) that can have one ore more elements.
func (p *parser) parseComplexStringPatternPiece(start int32, kind sequencePatternPieceKind, ident *PatternIdentifierLiteral) *ComplexStringPatternPiece {
	p.panicIfContextDone()

	unprefixed := false

	switch kind {
	case rootSequencePatternPiece:
		unprefixed = ident.Unprefixed

		if unprefixed {
			p.tokens = append(p.tokens,
				Token{Type: UNPREFIXED_STR, Span: ident.Span},
				Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{ident.Span.End, ident.Span.End + 1}},
			)
		} else {
			p.tokens = append(p.tokens,
				Token{Type: PERCENT_STR, Span: ident.Span},
				Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{ident.Span.End, ident.Span.End + 1}},
			)
		}
	case parenthesizedSequencePatternPiece:
		p.tokens = append(p.tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{start, start + 1}})
	}

	var parsingErr *ParsingError
	var elements []*PatternPieceElement

	for p.i < p.len && p.s[p.i] != ')' && (kind != unionCaseSequencePatternPiece || p.s[p.i] != '|') {

		p.eatSpaceNewlineComment()

		if p.i >= p.len || p.s[p.i] == ')' || (kind == unionCaseSequencePatternPiece && p.s[p.i] == '|') {
			break
		}

		if p.s[p.i] == '|' {
			union := p.parseComplexStringPatternUnion(p.i, true)
			elements = append(elements, &PatternPieceElement{
				NodeBase:   union.NodeBase,
				Quantifier: ExactlyOneOccurrence,
				Expr:       union,
			})
			break
		}

		elementStart := p.i
		ocurrenceModifier := ExactlyOneOccurrence
		count := 0
		elementEnd := int32(-1)
		var groupName *PatternGroupName
		var elemParsingErr *ParsingError
		var element Node

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
				groupName = &PatternGroupName{
					NodeBase: NodeBase{
						Span: NodeSpan{elementStart, p.i},
					},
					Name: string(p.s[elementStart:p.i]),
				}
				if groupName.Name[len(groupName.Name)-1] == '-' {
					groupName.Err = &ParsingError{UnspecifiedParsingError, INVALID_GROUP_NAME_SHOULD_NOT_END_WITH_DASH}
				}

				p.tokens = append(p.tokens, Token{Type: COLON, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				p.eatSpace()
			}
		}

		element = p.parseComplexStringPatternElement()
		elementEnd = p.i

		if p.i < p.len && (p.s[p.i] == '+' || p.s[p.i] == '*' || p.s[p.i] == '?' || p.s[p.i] == '=') {
			switch p.s[p.i] {
			case '+':
				ocurrenceModifier = AtLeastOneOccurrence
				elementEnd++
				p.i++

				p.tokens = append(p.tokens, Token{Type: OCCURRENCE_MODIFIER, Span: NodeSpan{p.i - 1, p.i}, Raw: "+"})
			case '*':
				ocurrenceModifier = ZeroOrMoreOccurrences
				elementEnd++
				p.i++

				p.tokens = append(p.tokens, Token{Type: OCCURRENCE_MODIFIER, Span: NodeSpan{p.i - 1, p.i}, Raw: "*"})
			case '?':
				ocurrenceModifier = OptionalOccurrence
				elementEnd++
				p.i++

				p.tokens = append(p.tokens, Token{Type: OCCURRENCE_MODIFIER, Span: NodeSpan{p.i - 1, p.i}, Raw: "?"})
			case '=':
				p.i++
				numberStart := p.i
				if p.i >= p.len || !isDecDigit(p.s[p.i]) {
					elemParsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT_UNTERMINATED_EXACT_OCURRENCE_COUNT}
					elementEnd = p.i
					goto after_ocurrence
				}

				for p.i < p.len && isDecDigit(p.s[p.i]) {
					p.i++
				}

				_count, err := strconv.ParseUint(string(p.s[numberStart:p.i]), 10, 32)
				if err != nil {
					elemParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_PATTERN_INVALID_OCCURENCE_COUNT}
				}
				count = int(_count)
				ocurrenceModifier = ExactOccurrenceCount
				elementEnd = p.i

				raw := string(p.s[numberStart-1 : p.i])
				p.tokens = append(p.tokens, Token{Type: OCCURRENCE_MODIFIER, Span: NodeSpan{numberStart - 1, p.i}, Raw: raw})
			}
		}

	after_ocurrence:

		elements = append(elements, &PatternPieceElement{
			NodeBase: NodeBase{
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
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_COMPLEX_STRING_PATT_MISSING_CLOSING_BRACKET}
		} else {
			p.tokens = append(p.tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	}

	return &ComplexStringPatternPiece{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Unprefixed: unprefixed,
		Elements:   elements,
	}
}

func (p *parser) parseComplexStringPatternElement() Node {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *ParsingError

	if p.i >= p.len || p.s[p.i] == ')' || p.s[p.i] == '|' {
		return &InvalidComplexStringPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, fmtAPatternWasExpectedHere(p.s, p.i)},
				false,
			},
		}
	}

	switch {
	case p.s[p.i] == '(':
		elemStart := p.i
		p.i++

		if p.i >= p.len || p.s[p.i] == ')' {
			return &InvalidComplexStringPatternElement{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_PATTERN_ELEMENT},
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
			disallowUnparenthesizedBinExpr:          true,
			disallowParsingSeveralPatternUnionCases: true,
		})
		return e
	case p.s[p.i] == '-' || isDecDigit(p.s[p.i]):
		e, _ := p.parseExpression(exprParsingConfig{
			disallowUnparenthesizedBinExpr:          true,
			disallowParsingSeveralPatternUnionCases: true,
		})
		switch e.(type) {
		case *IntegerRangeLiteral:
		default:
			return &InvalidComplexStringPatternElement{
				NodeBase: NodeBase{
					e.Base().Span,
					&ParsingError{UnspecifiedParsingError, INVALID_COMPLEX_PATTERN_ELEMENT},
					false,
				},
			}
		}
		return e
	case isAlpha(p.s[p.i]):
		patternIdent := &PatternIdentifierLiteral{
			NodeBase:   NodeBase{Span: NodeSpan{start, start + 1}},
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
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_COMPLEX_PATTERN_ELEMENT}
			}
			p.i++
		}
	}

	if parsingErr == nil && p.i == start {
		parsingErr = &ParsingError{UnspecifiedParsingError, fmtAPatternWasExpectedHere(p.s, p.i)}
		if p.s[p.i] != ')' {
			p.i++
		}
	}

	return &InvalidComplexStringPatternElement{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
	}
}

func (p *parser) parseComplexStringPatternUnion(start int32, isShorthandUnion bool) *PatternUnion {
	p.panicIfContextDone()

	var cases []Node

	if !isShorthandUnion {
		p.tokens = append(p.tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{start, start + 1}})
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

			return &PatternUnion{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, INVALID_PATT_UNION_ELEMENT_SEPARATOR_EXPLANATION},
					false,
				},
				Cases: cases,
			}
		}
		p.tokens = append(p.tokens, Token{Type: PIPE, SubType: STRING_PATTERN_UNION_PIPE, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpaceNewlineComment()

		if p.i >= p.len || p.s[p.i] == ')' || p.s[p.i] == '|' {
			cases = append(cases, &InvalidComplexStringPatternElement{
				NodeBase: NodeBase{
					NodeSpan{p.i, p.i},
					&ParsingError{UnspecifiedParsingError, fmtAPatternWasExpectedHere(p.s, p.i)},
					false,
				},
			})
		} else {
			pieceStart := p.i

			piece := p.parseComplexStringPatternPiece(pieceStart, unionCaseSequencePatternPiece, nil)
			var case_ Node = piece

			//Simplify if the piece contains a single element that does not have a name and is not parenthesized.
			if len(piece.Elements) == 1 && !piece.IsParenthesized {
				elem := piece.Elements[0]
				if elem.Quantifier == ExactlyOneOccurrence && elem.GroupName == nil && !elem.IsParenthesized {
					case_ = elem.Expr
				}
			}
			cases = append(cases, case_)
		}

	}

	if isShorthandUnion {
		return &PatternUnion{
			NodeBase: NodeBase{Span: NodeSpan{start, p.i}},
			Cases:    cases,
		}
	}

	var parsingErr *ParsingError

	if p.i >= p.len || p.s[p.i] != ')' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_UNION_MISSING_CLOSING_PAREN}
	} else {
		p.tokens = append(p.tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &PatternUnion{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Cases: cases,
	}
}
