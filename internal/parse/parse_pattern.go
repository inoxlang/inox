package parse

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/exp/slices"
)

func (p *parser) parsePercentPrefixedPattern(precededByOpeningParen bool) ast.Node {
	p.panicIfContextDone()

	start := p.i
	p.i++

	percentSymbol := ast.Token{Type: ast.PERCENT_SYMBOL, Span: NodeSpan{start, p.i}}

	if p.i >= p.len {
		p.tokens = append(p.tokens, percentSymbol)

		return &ast.UnknownNode{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
				false,
			},
		}
	}

	switch p.s[p.i] {
	case '|':
		prev := p.inPattern
		defer func() {
			p.inPattern = prev
		}()
		p.inPattern = true

		caseBeforeFirstPipe := ast.Node(nil)

		union := p.parsePatternUnion(start, true, caseBeforeFirstPipe, precededByOpeningParen)
		p.eatSpace()

		return union
	case '.', '/':
		p.i--
		return p.parsePathLikeExpression(true)
	case ':': //scheme-less host pattern
		p.i++
		percentPrefixed := true
		return p.parseURLLikePattern(start, percentPrefixed)
	case '{':
		prev := p.inPattern
		defer func() {
			p.inPattern = prev
		}()
		p.inPattern = true

		return p.parseObjectRecordPatternLiteral(true, false)
	case '[':
		prev := p.inPattern
		defer func() {
			p.inPattern = prev
		}()
		p.inPattern = true

		return p.parseListTuplePatternLiteral(true, false)
	case '(': //pattern conversion expresison
		prev := p.inPattern
		p.inPattern = false
		e, _ := p.parseExpression(exprParsingConfig{
			disallowUnparenthesizedBinForPipelineExprs: true,
			disallowParsingSeveralPatternUnionCases:    true,
		})

		p.inPattern = prev
		p.tokens = append(p.tokens, percentSymbol)

		return &ast.PatternConversionExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, e.Base().Span.End},
			},
			Value: e,
		}
	case '<':
		prefixed := true
		return p.parseMarkupPatternExpression(prefixed)
	case '`':
		return p.parseRegularExpressionLiteral(true)
	case '-':
		return p.parseOptionPatternLiteral(start, "", false)
	default:
		if isAlpha(p.s[p.i]) {
			p.i--
			return p.parsePercentAlphaStartingExpr()
		}

		p.tokens = append(p.tokens, percentSymbol)

		//TODO: fix, error based on next char ?

		return &ast.UnknownNode{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
				false,
			},
		}
	}
}

func (p *parser) parseRegularExpressionLiteral(percentPrefixed bool) *ast.RegularExpressionLiteral {
	start := p.i

	if percentPrefixed {
		start = p.i - 1
	}

	p.i++
	for p.i < p.len && (p.s[p.i] != '`' || utils.CountPrevBackslashes(p.s, p.i)%2 == 1) {
		p.i++
	}

	raw := ""
	str := ""

	var parsingErr *sourcecode.ParsingError
	if p.i >= p.len {
		raw = string(p.s[start:p.i])
		if percentPrefixed {
			str = raw[2:]
		} else {
			str = raw[1:]
		}

		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_REGEX_LIT}
	} else {
		raw = string(p.s[start : p.i+1])
		if percentPrefixed {
			str = raw[2 : len(raw)-1]
		} else {
			str = raw[1 : len(raw)-1]
		}
		p.i++

		_, err := regexp.Compile(str)
		if err != nil {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidRegexLiteral(err.Error())}
		}
	}

	return &ast.RegularExpressionLiteral{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Value:      str,
		Raw:        raw,
		Unprefixed: !percentPrefixed,
	}
}

func (p *parser) parseOptionPatternLiteral(start int32, unprefixedOptionPatternName string, singleDashUnprefixedOptionPattern bool) *ast.OptionPatternLiteral {
	prev := p.inPattern
	defer func() {
		p.inPattern = prev
	}()
	p.inPattern = true

	name := unprefixedOptionPatternName
	unprefixed := unprefixedOptionPatternName != ""
	singleDash := singleDashUnprefixedOptionPattern

	if !unprefixed {

		p.i++
		if p.i >= p.len {
			return &ast.OptionPatternLiteral{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, DASH_SHOULD_BE_FOLLOWED_BY_OPTION_NAME},
				},
				SingleDash: true,
			}
		}

		singleDash = true

		if p.s[p.i] == '-' {
			singleDash = false
			p.i++
		}

		nameStart := p.i

		if p.i >= p.len {
			return &ast.OptionPatternLiteral{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, DOUBLE_DASH_SHOULD_BE_FOLLOWED_BY_OPTION_NAME},
				},
				SingleDash: singleDash,
			}
		}

		if !isAlpha(p.s[p.i]) && !isDecDigit(p.s[p.i]) {
			return &ast.OptionPatternLiteral{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, OPTION_NAME_CAN_ONLY_CONTAIN_ALPHANUM_CHARS},
				},
				SingleDash: singleDash,
			}
		}

		for p.i < p.len && (isAlpha(p.s[p.i]) || isDecDigit(p.s[p.i]) || p.s[p.i] == '-') {
			p.i++
		}

		name = string(p.s[nameStart:p.i])
	}

	if p.i >= p.len || p.s[p.i] != '=' {
		return &ast.OptionPatternLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_OPTION_PATTERN_A_VALUE_IS_EXPECTED_AFTER_EQUAKL_SIGN},
			},
			Name:       name,
			SingleDash: singleDash,
			Unprefixed: unprefixed,
		}
	}

	p.i++

	if p.i >= p.len {
		return &ast.OptionPatternLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_OPTION_PATT_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR},
			},
			Name:       name,
			SingleDash: singleDash,
			Unprefixed: unprefixed,
		}
	}

	value, _ := p.parseExpression()

	return &ast.OptionPatternLiteral{
		NodeBase:   ast.NodeBase{Span: NodeSpan{start, p.i}},
		Name:       name,
		Value:      value,
		SingleDash: singleDash,
		Unprefixed: unprefixed,
	}
}

// parseFunctionPattern parses function patterns
func (p *parser) parseFunctionPattern(start int32, percentPrefixed bool) ast.Node {
	p.panicIfContextDone()

	if percentPrefixed {
		p.tokens = append(p.tokens, ast.Token{Type: ast.PERCENT_FN, Span: NodeSpan{p.i - 3, p.i}})
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.FN_KEYWORD, Span: NodeSpan{p.i - 2, p.i}})
	}

	p.eatSpace()

	var (
		parsingErr     *sourcecode.ParsingError
		capturedLocals []ast.Node
	)

	createNodeWithError := func() ast.Node {
		fn := ast.FunctionExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			CaptureList: capturedLocals,
		}

		fn.Err = parsingErr
		return &fn
	}

	if p.i >= p.len || p.s[p.i] != '(' {
		parsingErr = &sourcecode.ParsingError{InvalidNext, PERCENT_FN_SHOULD_BE_FOLLOWED_BY_PARAMETERS}
		return createNodeWithError()
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	var parameters []*ast.FunctionParameter
	isVariadic := false

	inPatternSave := p.inPattern
	p.inPattern = true

	//we parse the parameters
	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewlineComma()
		var paramErr *sourcecode.ParsingError

		if p.i < p.len && p.s[p.i] == ')' {
			break
		}

		if isVariadic {
			paramErr = &sourcecode.ParsingError{UnspecifiedParsingError, VARIADIC_PARAM_IS_UNIQUE_AND_SHOULD_BE_LAST_PARAM}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			isVariadic = true
			p.i += 3
		}

		firstNodeInParam, isMissingExpr := p.parseExpression()

		var typ ast.Node
		if isMissingExpr {
			r := p.s[p.i]
			p.i++
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i - 1, p.i}, Raw: string(r)})

			parameters = append(parameters, &ast.FunctionParameter{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i - 1, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(r)},
					false,
				},
			})

		} else {
			switch firstNodeInParam := firstNodeInParam.(type) {
			case *ast.IdentifierLiteral: //keyword
				var varNode ast.Node = firstNodeInParam

				p.eatSpace()
				typ, isMissingExpr = p.parseExpression()

				if isMissingExpr {
					typ = nil
				}

				span := firstNodeInParam.Base().Span
				if typ != nil {
					span.End = typ.Base().Span.End
				}

				parameters = append(parameters, &ast.FunctionParameter{
					NodeBase: ast.NodeBase{
						span,
						&sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES},
						false,
					},
					Var:        varNode,
					Type:       typ,
					IsVariadic: isVariadic,
				})
			case *ast.PatternIdentifierLiteral: //parameter name or parameter type
				p.eatSpace()

				typ, isMissingExpr = p.parseExpression()
				var varNode ast.Node

				if !isMissingExpr {
					//If there is someting after the first node is the name of the paramter.

					varNode = &ast.IdentifierLiteral{NodeBase: firstNodeInParam.Base(), Name: firstNodeInParam.Name}
					if paramErr == nil && isKeyword(firstNodeInParam.Name) {
						paramErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES}
					}
				} else {
					typ = firstNodeInParam
				}

				span := firstNodeInParam.Base().Span
				if varNode != nil {
					span.End = typ.Base().Span.End
				}

				parameters = append(parameters, &ast.FunctionParameter{
					NodeBase: ast.NodeBase{
						span,
						paramErr,
						false,
					},
					Var:        varNode,
					Type:       typ,
					IsVariadic: isVariadic,
				})
			case *ast.PatternCallExpression, *ast.PatternNamespaceMemberExpression,
				*ast.ObjectPatternLiteral, *ast.ListPatternLiteral, *ast.RecordPatternLiteral,
				*ast.ComplexStringPatternPiece, *ast.RegularExpressionLiteral:

				typ = firstNodeInParam

				parameters = append(parameters, &ast.FunctionParameter{
					NodeBase: ast.NodeBase{
						typ.Base().Span,
						paramErr,
						false,
					},
					Type:       typ,
					IsVariadic: isVariadic,
				})

			default:
				firstNodeInParam.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, PARAM_LIST_OF_FUNC_PATT_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS}

				parameters = append(parameters, &ast.FunctionParameter{
					NodeBase: ast.NodeBase{
						firstNodeInParam.Base().Span,
						paramErr,
						false,
					},
					Var:        firstNodeInParam,
					IsVariadic: isVariadic,
				})
			}

		}

		p.eatSpaceNewlineComma()
	}

	p.inPattern = inPatternSave

	var (
		returnType ast.Node
		end        int32
	)

	if p.i >= p.len {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN}
		end = p.i
	} else if p.s[p.i] != ')' {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FUNC_SYNTAX}
		end = p.i
	} else { //')'
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		if p.i < p.len && isAcceptedReturnTypeStart(p.s, p.i) {
			inPatternSave := p.inPattern
			p.inPattern = true

			returnType, _ = p.parseExpression(exprParsingConfig{
				disallowUnparenthesizedBinForPipelineExprs: true,
				disallowParsingSeveralPatternUnionCases:    true,
			})

			p.inPattern = inPatternSave
		}
		end = p.i
	}

	fn := ast.FunctionPatternExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, end},
			Err:  parsingErr,
		},
		Parameters: parameters,
		ReturnType: returnType,
		IsVariadic: isVariadic,
	}

	return &fn
}

func (p *parser) parseNamedPatternSegment(interpolation string, startIndex, endIndex int32) ast.Node {
	//':' is at startIndex
	i := int32(1)
	onlyIdentChars := true

	//Check that there are only chars allowed in identifiers after ':'.
	for i < int32(len(interpolation)) {
		if IsIdentChar(rune(interpolation[i])) {
			i++
			continue
		}

		onlyIdentChars = false
		break
	}

	var err *sourcecode.ParsingError
	if len(interpolation) == 1 || !onlyIdentChars { //empty name or invalid characters
		err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_NAMED_SEGMENT_PATH_PATTERN_COLON_SHOULD_BE_FOLLOWED_BY_A_NAME}
	} else if interpolation[1] == '-' {
		err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_NAMED_SEGMENT_PATH_PATTERN_COLON_NAME_SHOULD_NOT_START_WITH_DASH}
	} else if interpolation[len(interpolation)-1] == '-' {
		err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_NAMED_SEGMENT_PATH_PATTERN_COLON_NAME_SHOULD_NOT_END_WITH_DASH}
	}

	return &ast.NamedPathSegment{
		NodeBase: ast.NodeBase{
			NodeSpan{startIndex, endIndex},
			err,
			false,
		},
		Name: interpolation[1:],
	}
}

func (p *parser) tryParsePatternUnionWithoutLeadingPipe(firstCase ast.Node, precededByOpeningParen bool) (*ast.PatternUnion, bool) {
	startIndex := firstCase.Base().Span.Start

	tempIndex := p.i

	if precededByOpeningParen {
		if !p.areNextSpacesNewlinesCommentsFollowedBy('|') {
			return nil, false
		}
		//Eat the spaces and comments because we know we are in a pattern union.
		p.eatSpaceNewlineComment()
	} else {
		//We can only eat regular space because the expression is not parenthesized.
		for tempIndex < p.len && isSpaceNotLF(p.s[tempIndex]) {
			tempIndex++
		}
		if tempIndex >= p.len || p.s[tempIndex] != '|' {
			return nil, false
		}
		//Eat the spaces because we know we are in a pattern union.
		p.eatSpaceNewline()
	}

	//The '|' token will be eaten by parsePatternUnion.
	isPercentPrefixed := false
	return p.parsePatternUnion(startIndex, isPercentPrefixed, firstCase, precededByOpeningParen), true
}

func (p *parser) parsePatternCall(callee ast.Node) *ast.PatternCallExpression {
	p.panicIfContextDone()

	var (
		args       []ast.Node
		parsingErr *sourcecode.ParsingError
	)

	inPatternSave := p.inPattern
	defer func() {
		p.inPattern = inPatternSave
	}()

	p.inPattern = true

	switch p.s[p.i] {
	case '(':
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		p.eatSpaceComma()

		for p.i < p.len && p.s[p.i] != ')' {
			arg, isMissingExpr := p.parseExpression()

			if isMissingExpr {
				span := NodeSpan{p.i, p.i + 1}

				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: span, Raw: string(p.s[p.i])})

				arg = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						span,
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInPatternCallArguments(p.s[p.i])},
						false,
					},
				}
				p.i++
			}

			args = append(args, arg)
			p.eatSpaceComma()
		}

		if p.i >= p.len || p.s[p.i] != ')' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATTERN_CALL_MISSING_CLOSING_PAREN}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	case '{':
		args = append(args, utils.Ret0(p.parseExpression()))
	default:
		panic(ErrUnreachable)
	}

	return &ast.PatternCallExpression{
		Callee: callee,
		NodeBase: ast.NodeBase{
			Span: NodeSpan{callee.Base().Span.Start, p.i},
			Err:  parsingErr,
		},
		Arguments: args,
	}
}

func (p *parser) parseListTuplePatternLiteral(percentPrefixed, isTuplePattern bool) ast.Node {
	p.panicIfContextDone()

	openingBracketIndex := p.i
	p.i++

	var (
		elements []ast.Node
		start    int32
	)

	if percentPrefixed {
		if isTuplePattern {
			panic(ErrUnreachable)
		}
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{openingBracketIndex - 1, openingBracketIndex + 1}})
		start = openingBracketIndex - 1
	} else {
		if isTuplePattern {
			p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_TUPLE_BRACKET, Span: NodeSpan{openingBracketIndex, openingBracketIndex + 2}})
			p.i++
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_BRACKET, Span: NodeSpan{openingBracketIndex, openingBracketIndex + 1}})
		}
		start = openingBracketIndex
	}

	for p.i < p.len && p.s[p.i] != ']' {
		p.eatSpaceNewlineCommaComment()

		if p.i < p.len && p.s[p.i] == ']' {
			break
		}

		e, isMissingExpr := p.parseExpression()
		if !isMissingExpr {
			elements = append(elements, e)
			if p.i >= p.len {
				break
			}
		} else if p.s[p.i] != ',' {
			break
		}

		p.eatSpaceNewlineCommaComment()
	}

	var parsingErr *sourcecode.ParsingError

	if p.i >= p.len || p.s[p.i] != ']' {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_TUPLE_PATT_LIT_MISSING_BRACE}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	var generalElement ast.Node
	if p.i < p.len && (p.s[p.i] == '%' || IsFirstIdentChar(p.s[p.i]) || isOpeningDelim(p.s[p.i]) || p.s[p.i] == '#') {
		if len32(elements) > 0 {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_LIST_TUPLE_PATT_GENERAL_ELEMENT_IF_ELEMENTS}
		} else {
			elements = nil
		}
		generalElement, _ = p.parseExpression(exprParsingConfig{
			disallowParsingSeveralPatternUnionCases: true,
		})
	}

	if isTuplePattern {
		return &ast.TuplePatternLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  parsingErr,
			},
			Elements:       elements,
			GeneralElement: generalElement,
		}
	}

	return &ast.ListPatternLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Elements:       elements,
		GeneralElement: generalElement,
	}
}

func (p *parser) parseDictionaryPatternLiteral() *ast.DictionaryPatternLiteral {
	p.panicIfContextDone()

	{
		inPattern := p.inPattern
		p.inPattern = true
		defer func() {
			p.inPattern = inPattern
		}()
	}

	openingIndex := p.i
	p.i += 2

	var parsingErr *sourcecode.ParsingError
	var entries []*ast.DictionaryPatternEntry
	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_DICTIONARY_BRACKET, Span: NodeSpan{p.i - 2, p.i}})

	p.eatSpaceNewlineCommaComment()

dictionary_pattern_literal_top_loop:
	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) { //one iteration == one entry (that can be invalid)

		if p.s[p.i] == '}' {
			break dictionary_pattern_literal_top_loop
		}

		entry := &ast.DictionaryPatternEntry{
			NodeBase: ast.NodeBase{
				NodeSpan{p.i, p.i + 1},
				nil,
				false,
			},
		}
		entries = append(entries, entry)

		key, isMissingExpr := p.parseExpression()
		entry.Key = key

		if isMissingExpr {
			if p.i < p.len && p.s[p.i] != '}' {
				p.i++
				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: key.Base().Span, Raw: string(p.s[p.i-1])})

				entry.Key = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						key.Base().Span,
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionaryPattern(p.s[p.i-1])},
						false,
					},
				}
				p.eatSpaceNewlineCommaComment()
				continue
			}

			p.i++
			entry.Span.End = key.Base().Span.End
			p.eatSpaceNewlineCommaComment()
			continue
		}

		colonInLiteral := false

		if key.Base().Err == nil || ast.NodeIs(key, (*ast.InvalidURL)(nil)) {
			var literalVal string
			switch k := key.(type) {
			case *ast.InvalidURL:
				literalVal = k.Value
			default:
				valueLit, ok := key.(ast.SimpleValueLiteral)
				if ok {
					literalVal = valueLit.ValueString()
				}
			}

			if literalVal != "" {
				if lastColonIndex := strings.LastIndex(literalVal, ":"); lastColonIndex > 0 && strings.Index(literalVal, "://") < lastColonIndex {
					colonInLiteral = true
				}
			}
		}

		p.eatSpace()

		if p.i >= p.len || p.s[p.i] == '}' {
			if entry.Err == nil {
				msg := INVALID_DICT_PATT_ENTRY_MISSING_COLON_AFTER_KEY
				if colonInLiteral {
					msg = INVALID_DICT_PATT_ENTRY_MISSING_SPACE_BETWEEN_KEY_AND_COLON
				}

				entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, msg}
				entry.Span.End = p.i
			}
			break
		}

		if p.s[p.i] != ':' {
			if p.s[p.i] != ',' {
				entry.Span.End = p.i
				entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionaryPattern(p.s[p.i])}
				p.i++
				p.eatSpaceNewlineCommaComment()
				continue
			}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.COLON, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}

		p.eatSpace()

		value, isMissingExpr := p.parseExpression()
		entry.Value = value
		entry.Span.End = value.Base().Span.End

		if isMissingExpr && p.i < p.len && p.s[p.i] != '}' && p.s[p.i] != ',' {
			char := p.s[p.i]
			if isClosingDelim(char) {
				break dictionary_pattern_literal_top_loop //No need to add the the entry since it is already added .
			}
			if entry.Err == nil {
				entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionaryPattern(char)}
			}
			p.i++
		}

		p.eatSpace()

		if p.i < p.len && !isValidEntryEnd(p.s, p.i) && entry.Err == nil {
			entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DICT_PATT_LIT_ENTRY_SEPARATION}
		}

		p.eatSpaceNewlineCommaComment()
	}

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	} else {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_DICT_PATT_MISSING_CLOSING_BRACE}
	}

	return &ast.DictionaryPatternLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{openingIndex, p.i},
			Err:  parsingErr,
		},
		Entries: entries,
	}
}

func (p *parser) parseReadonlyPatternExpression(readonlyIdent *ast.IdentifierLiteral) *ast.ReadonlyPatternExpression {
	p.panicIfContextDone()

	p.tokens = append(p.tokens, ast.Token{Type: ast.READONLY_KEYWORD, Span: readonlyIdent.Span})
	p.eatSpace()

	prev := p.inPattern
	p.inPattern = true
	defer func() {
		p.inPattern = prev
	}()

	pattern, _ := p.parseExpression()

	return &ast.ReadonlyPatternExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{readonlyIdent.Span.Start, pattern.Base().Span.End},
		},
		Pattern: pattern,
	}
}

// parsePatternUnion parses a pattern union until the next linefeed if $precededByOpeningParen is false, until the next
// unpaired or closing delimiter otherwise. Even if $precededByOpeningParen is true parsePatternUnion stops at the closing
// parenthesis, the parenthesis should be handled by the caller.
func (p *parser) parsePatternUnion(
	start int32,
	isPercentPrefixed bool,
	caseBeforeFirstPipe ast.Node, /*set if no leading pipe*/
	precededByOpeningParen bool,
) *ast.PatternUnion {
	p.panicIfContextDone()

	var (
		cases []ast.Node
	)

	if caseBeforeFirstPipe != nil {
		cases = append(cases, caseBeforeFirstPipe)
	}

	if isPercentPrefixed {
		p.tokens = append(p.tokens, ast.Token{
			Type: ast.PATTERN_UNION_OPENING_PIPE,
			Span: NodeSpan{Start: p.i - 1, End: p.i + 1},
		})
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.PIPE, SubType: ast.UNPREFIXED_PATTERN_UNION_PIPE, Span: NodeSpan{Start: p.i, End: p.i + 1}})
	}

	p.i++

	eatNonSignificant := func() {
		if precededByOpeningParen {
			p.eatSpaceNewlineCommaComment()
		} else {
			p.eatSpace()
		}
	}

	eatNonSignificant()

	case_, _ := p.parseExpression(exprParsingConfig{
		disallowParsingSeveralPatternUnionCases: true,
	})

	cases = append(cases, case_)

	eatNonSignificant()

	for p.i < p.len && (p.s[p.i] == '|' ||
		(precededByOpeningParen && p.s[p.i] == '\n') ||
		!isUnpairedOrIsClosingDelim(p.s[p.i])) {

		eatNonSignificant()

		if p.s[p.i] != '|' {
			return &ast.PatternUnion{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_PATT_UNION_ELEMENT_SEPARATOR_EXPLANATION},
				},
				Cases: cases,
			}
		}
		p.tokens = append(p.tokens, ast.Token{Type: ast.PIPE, SubType: ast.UNPREFIXED_PATTERN_UNION_PIPE, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		eatNonSignificant()

		case_, _ := p.parseExpression(exprParsingConfig{
			disallowParsingSeveralPatternUnionCases: true,
		})
		cases = append(cases, case_)

		eatNonSignificant()
	}

	return &ast.PatternUnion{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			nil,
			false,
		},
		Cases: cases,
	}
}

func (p *parser) parsePercentAlphaStartingExpr() ast.Node {
	p.panicIfContextDone()

	start := p.i
	p.i++

	for p.i < p.len && IsIdentChar(p.s[p.i]) {
		p.i++
	}

	ident := &ast.PatternIdentifierLiteral{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			nil,
			false,
		},
		Name: string(p.s[start+1 : p.i]),
	}

	var left ast.Node = ident

	if p.i < p.len && p.s[p.i] == '.' { //pattern namespace or pattern namespace member expression
		p.i++
		namespaceIdent := &ast.PatternNamespaceIdentifierLiteral{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Name: ident.Name,
		}

		if p.i >= p.len || IsDelim(p.s[p.i]) || isSpaceNotLF(p.s[p.i]) {
			return namespaceIdent
		}

		memberNameStart := p.i

		if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
			return &ast.PatternNamespaceMemberExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtPatternNamespaceMemberShouldStartWithAletterNot(p.s[p.i])},
					false,
				},
				Namespace: namespaceIdent,
			}
		}

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		left = &ast.PatternNamespaceMemberExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Namespace: namespaceIdent,
			MemberName: &ast.IdentifierLiteral{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{memberNameStart, p.i},
				},
				Name: string(p.s[memberNameStart:p.i]),
			},
		}
	}

	if p.i < p.len {

		if left == ident && ident.Name == "fn" {
			return p.parseFunctionPattern(ident.Span.Start, true)
		}

		switch {
		case p.s[p.i] == '(' || p.s[p.i] == '{':
			if left == ident && ident.Name == "str" && p.s[p.i] == '(' {
				p.i++
				return p.parseComplexStringPatternPiece(ident.Span.Start, rootSequencePatternPiece, ident)
			}
			return p.parsePatternCall(left)
		case p.s[p.i] == '?':
			p.i++
			return &ast.OptionalPatternExpression{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{left.Base().Span.Start, p.i},
				},
				Pattern: left,
			}
		case left == ident && p.s[p.i] == ':' && (slices.Contains(SCHEMES, ident.Name)):
			p.i++

			percentPrefixed := true
			return p.parseURLLikePattern(start, percentPrefixed)
		}
	}

	return left
}

func (p *parser) parsePatternDefinition(patternIdent *ast.IdentifierLiteral) *ast.PatternDefinition {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, ast.Token{Type: ast.PATTERN_KEYWORD, Span: patternIdent.Span})

	patternDef := &ast.PatternDefinition{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{patternIdent.Span.Start, p.i},
		},
	}

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] == '\n' {
		patternDef.Err = &sourcecode.ParsingError{UnterminatedPatternDefinition, UNTERMINATED_PATT_DEF_MISSING_NAME_AFTER_PATTERN_KEYWORD}
	} else {
		func() {
			prev := p.inPattern
			p.inPattern = true
			defer func() {
				p.inPattern = prev
			}()

			patternDef.Left, _ = p.parseExpression(exprParsingConfig{
				disallowUnparenthesizedBinForPipelineExprs: true,
				disallowParsingSeveralPatternUnionCases:    true,
			})
			patternDef.Span.End = p.i

			if _, ok := patternDef.Left.(*ast.PatternIdentifierLiteral); !ok && patternDef.Left.Base().Err == nil {
				patternDef.Left.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, A_PATTERN_NAME_WAS_EXPECTED}
			}
		}()

		p.eatSpace()

		if p.i >= p.len || p.s[p.i] != '=' {
			patternDef.Err = &sourcecode.ParsingError{UnterminatedPatternDefinition, UNTERMINATED_PATT_DEF_MISSING_EQUAL_SYMBOL_AFTER_PATTERN_NAME}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.ASSIGN_EQUAL, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			patternDef.Span.End = p.i

			p.eatSpace()

			if p.i < p.len && p.s[p.i] == '@' && p.i < p.len-1 && unicode.IsSpace(p.s[p.i+1]) {
				patternDef.IsLazy = true
				p.i++
				patternDef.Span.End = p.i
				p.eatSpace()
			}

			//parse RHS

			if p.i >= p.len || p.s[p.i] == '\n' {
				patternDef.Err = &sourcecode.ParsingError{UnterminatedPatternDefinition, UNTERMINATED_PATT_DEF_MISSING_RHS}
			} else {
				prev := p.inPattern
				p.inPattern = true
				defer func() {
					p.inPattern = prev
				}()

				patternDef.Right, _ = p.parseExpression()
				patternDef.Span.End = p.i
			}
		}
	}

	return patternDef
}

func (p *parser) parsePatternNamespaceDefinition(patternIdent *ast.IdentifierLiteral) *ast.PatternNamespaceDefinition {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, ast.Token{Type: ast.PNAMESPACE_KEYWORD, Span: patternIdent.Span})

	namespaceDef := &ast.PatternNamespaceDefinition{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{Start: patternIdent.Span.Start, End: p.i},
		},
	}

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] == '\n' {
		namespaceDef.Err = &sourcecode.ParsingError{UnterminatedPatternNamespaceDefinition, UNTERMINATED_PATT_NS_DEF_MISSING_NAME_AFTER_PATTERN_KEYWORD}
	} else {
		func() {
			prev := p.inPattern
			p.inPattern = true
			defer func() {
				p.inPattern = prev
			}()

			namespaceDef.Left, _ = p.parseExpression(exprParsingConfig{
				disallowUnparenthesizedBinForPipelineExprs: true,
				disallowParsingSeveralPatternUnionCases:    true,
			})
			namespaceDef.Span.End = p.i

			if _, ok := namespaceDef.Left.(*ast.PatternNamespaceIdentifierLiteral); !ok && namespaceDef.Left.Base().Err == nil {
				namespaceDef.Left.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, A_PATTERN_NAMESPACE_NAME_WAS_EXPECTED}
			}
		}()

		p.eatSpace()

		if p.i >= p.len || p.s[p.i] != '=' {
			namespaceDef.Err = &sourcecode.ParsingError{UnterminatedPatternNamespaceDefinition, UNTERMINATED_PATT_NS_DEF_MISSING_EQUAL_SYMBOL_AFTER_PATTERN_NAME}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.ASSIGN_EQUAL, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			namespaceDef.Span.End = p.i

			p.eatSpace()

			//parse RHS

			if p.i >= p.len || p.s[p.i] == '\n' {
				namespaceDef.Err = &sourcecode.ParsingError{UnterminatedPatternNamespaceDefinition, UNTERMINATED_PATT_NS_DEF_MISSING_RHS}
			} else {
				namespaceDef.Right, _ = p.parseExpression()
				namespaceDef.Span.End = p.i
			}
		}
	}

	return namespaceDef
}
