package parse

import (
	"unicode"
	"unicode/utf8"
)

func (p *parser) parseUnaryBinaryAndParenthesizedExpression(openingParenIndex int32, previousOperatorEnd int32) Node {
	p.panicIfContextDone()

	//firstParenTokenIndex := -1
	var startIndex = openingParenIndex
	hasPreviousOperator := previousOperatorEnd > 0

	if hasPreviousOperator {
		startIndex = previousOperatorEnd
	} else {
		//firstParenTokenIndex = len(p.tokens)
		p.tokens = append(p.tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{openingParenIndex, openingParenIndex + 1}})
	}

	p.eatSpaceNewlineComment()

	var (
		left          Node
		isMissingExpr bool
	)

	if !hasPreviousOperator && p.i < p.len && p.s[p.i] == '<' {
		//XML expression without namespace.
		left = p.parseXMLExpression(nil, p.i)
	} else {
		left, isMissingExpr = p.parseExpression(exprParsingConfig{precededByOpeningParen: true, disallowUnparenthesizedBinExpr: true})
	}

	if ident, ok := left.(*IdentifierLiteral); ok && !hasPreviousOperator {
		switch ident.Name {
		case "if":
			return p.parseIfExpression(openingParenIndex, ident.Span.Start)
		case "for":
			return p.parseForExpression(openingParenIndex, ident.Span.Start)
		}
	}

	p.eatSpaceNewlineComment()

	if isMissingExpr {
		if p.i >= p.len {
			if hasPreviousOperator {
				return &MissingExpression{
					NodeBase: NodeBase{
						Span: NodeSpan{p.i - 1, p.i},
						Err:  &ParsingError{UnspecifiedParsingError, fmtExprExpectedHere(p.s, p.i, false)},
					},
				}
			}
			return &UnknownNode{
				NodeBase: NodeBase{
					NodeSpan{startIndex, p.i},
					left.Base().Err,
					false,
				},
			}
		}

		if p.s[p.i] == ')' {
			if !hasPreviousOperator {
				p.tokens = append(p.tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++

				return &UnknownNode{
					NodeBase: NodeBase{
						NodeSpan{startIndex, p.i},
						left.Base().Err,
						true,
					},
				}
			} else {
				return &MissingExpression{
					NodeBase: NodeBase{
						Span:            NodeSpan{p.i - 1, p.i},
						Err:             &ParsingError{UnspecifiedParsingError, fmtExprExpectedHere(p.s, p.i, false)},
						IsParenthesized: false,
					},
				}
			}
		}

		p.i++
		rune := p.s[p.i-1]
		p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(rune), Span: NodeSpan{p.i - 1, p.i}})

		return &UnknownNode{
			NodeBase: NodeBase{
				NodeSpan{startIndex, p.i},
				&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParenthesizedExpression(rune)},
				false,
			},
		}
	}

	if stringLiteral, ok := left.(*UnquotedStringLiteral); ok && stringLiteral.Value == "-" {
		operand, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

		p.tokens = append(p.tokens, Token{Type: MINUS, Span: left.Base().Span})

		unaryExpr := &UnaryExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{stringLiteral.Span.Start, p.i},
			},
			Operator: NumberNegate,
			Operand:  operand,
		}

		p.eatSpace()

		if !hasPreviousOperator && p.s[p.i] == ')' {
			p.tokens = append(p.tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			unaryExpr.Span = NodeSpan{startIndex, p.i + 1}
			unaryExpr.IsParenthesized = true
			p.i++
			return unaryExpr
		}

		left = unaryExpr
	}

	if p.i < p.len && p.s[p.i] == ')' { //parenthesized
		if !hasPreviousOperator {
			p.i++

			p.tokens = append(p.tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i - 1, p.i}})
			left.BasePtr().IsParenthesized = true
		}
		return left
	}

	if p.i >= p.len {
		left.BasePtr().IsParenthesized = !hasPreviousOperator

		if !hasPreviousOperator {
			if left.Base().Err == nil {
				left.BasePtr().Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PARENTHESIZED_EXPR_MISSING_CLOSING_PAREN}
			}
		}
		return left
	}

	endOnLinefeed := false
	operator, operatorToken, parsingErr, earlyReturnedBinExpr := p.getBinaryOperator(left, startIndex, endOnLinefeed)

	if earlyReturnedBinExpr != nil {
		return earlyReturnedBinExpr
	}

	p.tokens = append(p.tokens, operatorToken)

	p.eatSpace()

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
	}

	inPatternSave := p.inPattern

	switch operator {
	case Match, NotMatch:
		p.inPattern = true
	}

	right, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

	p.inPattern = inPatternSave

	p.eatSpace()
	if isMissingExpr {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
	} else if p.i >= p.len {
		if !hasPreviousOperator {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN}
		}
	}

	var continueParsing bool
	var andOrToken Token
	var moveRightOperand bool

	chainElementEnd := p.i

	if p.i < p.len {
		switch p.s[p.i] {
		case 'a':
			if p.len-p.i >= AND_LEN &&
				string(p.s[p.i:p.i+AND_LEN]) == "and" &&
				(p.len-p.i == AND_LEN || !IsIdentChar(p.s[p.i+AND_LEN])) {
				continueParsing = true
				andOrToken = Token{Type: AND_KEYWORD, Span: NodeSpan{p.i, p.i + AND_LEN}}
				p.i += AND_LEN
			}
		case 'o':
			if p.len-p.i >= OR_LEN &&
				string(p.s[p.i:p.i+OR_LEN]) == "or" &&
				(p.len-p.i == OR_LEN || !IsIdentChar(p.s[p.i+OR_LEN])) {
				andOrToken = Token{Type: OR_KEYWORD, Span: NodeSpan{p.i, p.i + OR_LEN}}
				p.i += OR_LEN
				continueParsing = true
			}
		case ')':
			if !hasPreviousOperator {
				p.tokens = append(p.tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				chainElementEnd = p.i
			}
		default:
			if operator == Or || operator == And || isAlphaOrUndescore(p.s[p.i]) {
				continueParsing = true
				moveRightOperand = true
				andOrToken = operatorToken
			} else if isNonIdentBinaryOperatorChar(p.s[p.i]) {
				if hasPreviousOperator {
					continueParsing = true
					moveRightOperand = true
					andOrToken = operatorToken
				} else {
					parsingErr = &ParsingError{UnspecifiedParsingError, COMPLEX_OPERANDS_OF_BINARY_EXPRS_MUST_BE_PARENTHESIZED}
				}
			} else if !hasPreviousOperator {
				parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN}
			}
		}
	}

	if continueParsing { //or|and chain
		var newLeft Node

		if moveRightOperand {
			newLeft = left
			p.i = right.Base().Span.Start
		} else {
			newLeft = &BinaryExpression{
				NodeBase: NodeBase{
					Span: NodeSpan{startIndex, chainElementEnd},
					Err:  parsingErr,
				},
				Operator: operator,
				Left:     left,
				Right:    right,
			}
		}

		//var openingParenToken Token
		if !hasPreviousOperator {
			//openingParenToken = p.tokens[firstParenTokenIndex]

			if !moveRightOperand {
				newLeft.BasePtr().Span.End = right.Base().Span.End
			}
		}

		var newOperator BinaryOperator = And
		var newComplementOperator = Or

		if andOrToken.Type == OR_KEYWORD {
			newOperator = Or
			newComplementOperator = And
		}

		newBinExpr := &BinaryExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{startIndex, p.i},
				IsParenthesized: !hasPreviousOperator,
			},
			Operator: newOperator,
			Left:     newLeft,
		}

		p.tokens = append(p.tokens, andOrToken)
		// if !hasPreviousOperator {
		// 	newBinExpr.Tokens = []Token{openingParenToken, andOrToken}
		// } else {
		// 	newBinExpr.Tokens = []Token{andOrToken}
		// }

		p.eatSpace()

		newRight := p.parseUnaryBinaryAndParenthesizedExpression(-1, p.i)
		newBinExpr.Right = newRight

		p.eatSpace()

		if !hasPreviousOperator {
			if p.i >= p.len || p.s[p.i] != ')' {
				if _, ok := newRight.(*MissingExpression); !ok {
					newBinExpr.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN}
				}
				newBinExpr.Span.End = newRight.Base().Span.End
			} else {
				p.tokens = append(p.tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				newBinExpr.Span.End = p.i
			}

			if rightBinExpr, ok := newRight.(*BinaryExpression); ok &&
				!rightBinExpr.IsParenthesized && newBinExpr.Err == nil {

				subLeft, isSubLeftBinExpr := rightBinExpr.Left.(*BinaryExpression)
				subRight, isSubRightBinExpr := rightBinExpr.Right.(*BinaryExpression)

				err := &ParsingError{UnspecifiedParsingError, BIN_EXPR_CHAIN_OPERATORS_SHOULD_BE_THE_SAME}

				if isSubLeftBinExpr {
					if (!subLeft.IsParenthesized && (subLeft.Operator == newComplementOperator)) ||
						(rightBinExpr.Operator == newComplementOperator) {
						newBinExpr.Err = err
					}
				}

				if isSubRightBinExpr {
					if (!subRight.IsParenthesized && subRight.Operator == newComplementOperator) ||
						(rightBinExpr.Operator == newComplementOperator) {
						newBinExpr.Err = err
					}
				}
			}
		} else {
			newBinExpr.Span.End = newRight.Base().Span.End
		}

		return newBinExpr
	}

	return &BinaryExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{startIndex, chainElementEnd},
			Err:             parsingErr,
			IsParenthesized: !hasPreviousOperator,
		},
		Operator: operator,
		Left:     left,
		Right:    right,
	}
}

func (p *parser) tryParseUnparenthesizedBinaryExpr(left Node) (Node, bool) {
	p.panicIfContextDone()

	startIndex := left.Base().Span.Start

	spacePresentAfterLeft := false
	tempIndex := p.i
	{
		//We can only parse regular whitespace.
		for tempIndex < p.len && isSpaceNotLF(p.s[tempIndex]) {
			tempIndex++
		}

		spacePresentAfterLeft = tempIndex > p.i
	}

	if tempIndex >= p.len {
		return nil, false
	}

	switch p.s[tempIndex] {
	case '.':
		if !spacePresentAfterLeft {
			//member expression
			return nil, false
		}
		if tempIndex < p.len-1 && p.s[tempIndex+1] == '/' {
			// path ./
			return nil, false
		}
		if tempIndex < p.len-2 && p.s[tempIndex+1] == '.' && p.s[tempIndex+2] == '/' {
			// path ../
			return nil, false
		}
		if tempIndex < p.len-3 && p.s[tempIndex+1] == '.' && p.s[tempIndex+2] == '.' {
			// spread ...
			return nil, false
		}
	case ':':
		return nil, false
	case '*', '<', '>', '!':
	case '=':
		if tempIndex < p.len-1 && p.s[tempIndex+1] != '=' {
			//=>
			//=}
			return nil, false
		}
	case '+', '/', '-':
		//Check we are not at the start of an unquoted string literal or path.
		if tempIndex < p.len-1 && !unicode.IsSpace(p.s[tempIndex+1]) {
			return nil, false
		}
	case '?':
		if !spacePresentAfterLeft {
			//Unexpected '?' char. It may be an attempt at typing a boolean conversion expression
			//on an unsupported operand.
			return nil, false
		}
	default:
		if !isAlpha(p.s[tempIndex]) {
			return nil, false
		}

		//Check that the name is a valid binary operator. If the check is not performed some pieces of code
		//can be mistakenly parsed as binary expressions. For example the highlighted region in the following
		//piece of code {a : >>1 b<<: 2}

		nameStart := tempIndex

		for tempIndex < p.len && IsIdentChar(p.s[tempIndex]) {
			tempIndex++
		}

		nameRunes := p.s[nameStart:tempIndex]
		isOperator := false

	operator_name_search:
		for _, operator := range BINARY_OPERATOR_STRINGS {
			firstOperatorChar, _ := utf8.DecodeRuneInString(operator)

			if firstOperatorChar != nameRunes[0] || utf8.RuneCountInString(operator) != len(nameRunes) {
				continue
			}

			realOperatorRunes := []rune(operator)

			for i, r := range nameRunes {
				if realOperatorRunes[i] != r {
					continue operator_name_search
				}
			}
			isOperator = true
			break
		}

		if !isOperator {
			return nil, false
		}
	}

	p.eatSpace()

	endOnLinefeed := true
	operator, _, parsingErr, earlyReturnedBinExpr := p.getBinaryOperator(left, startIndex, endOnLinefeed)

	if earlyReturnedBinExpr != nil {
		return earlyReturnedBinExpr, true
	}

	p.eatSpace()

	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
	}

	inPatternSave := p.inPattern

	switch operator {
	case Match, NotMatch:
		p.inPattern = true
	}

	right, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

	p.inPattern = inPatternSave

	if isMissingExpr {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
	} else {
		index := p.i
		for index < p.len && isSpaceNotLF(p.s[index]) {
			index++
		}
		if index < p.len && !isUnpairedOrIsClosingDelim(p.s[index]) {
			unterminatedOperation := false
			switch p.s[index] {
			case '+', '-', '*', '/', '?' /*'>'*/, '<', '!', '=':
				unterminatedOperation = true
			default:
				unterminatedOperation = isAlphaOrUndescore(p.s[index])
			}

			if unterminatedOperation {
				parsingErr = &ParsingError{UnspecifiedParsingError, COMPLEX_OPERANDS_OF_BINARY_EXPRS_MUST_BE_PARENTHESIZED}
			}
		}
	}

	return &BinaryExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{startIndex, p.i},
			Err:             parsingErr,
			IsParenthesized: false,
		},
		Operator: operator,
		Left:     left,
		Right:    right,
	}, true
}

func (p *parser) getBinaryOperator(left Node, startIndex int32, endOnLinefeed bool) (
	operator BinaryOperator,
	validOperatorToken Token, //if not zero it is added to the tokens before the function returns.
	parsingErr *ParsingError,
	earlyReturnedBinExpr *BinaryExpression,
) {
	p.panicIfContextDone()

	defer func() {
		if validOperatorToken != (Token{}) {
			p.tokens = append(p.tokens, validOperatorToken)
		}
	}()

	operator = -1

	makeInvalidOperatorError := func() *ParsingError {
		return &ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR}
	}

	makeInvalidOperatorMissingRightOperand := func(operator BinaryOperator) *BinaryExpression {
		return &BinaryExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{startIndex, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERAND_OR_INVALID_OPERATOR},
			},
			Operator: operator,
			Left:     left,
		}
	}

	eatInvalidOperatorToken := func(operatorStart int32) {
		j := operatorStart

		if isNonIdentBinaryOperatorChar(p.s[j]) {
			for j < p.i && isNonIdentBinaryOperatorChar(p.s[j]) {
				j++
			}

			for p.i < p.len && isNonIdentBinaryOperatorChar(p.s[p.i]) {
				p.i++
			}

		} else if isAlpha(p.s[j]) || p.s[j] == '_' {
			for j < p.i && IsIdentChar(p.s[j]) {
				j++
			}
			for p.i < p.len && IsIdentChar(p.s[p.i]) {
				p.i++
			}
		}
		p.tokens = append(p.tokens, Token{
			Type: INVALID_OPERATOR,
			Span: NodeSpan{Start: operatorStart, End: p.i},
			Raw:  string(p.s[operatorStart:p.i]),
		})
	}

	var (
		nameStart    = p.i
		operatorType TokenType
	)

_switch:
	switch p.s[p.i] {
	case '+':
		operator = Add
		operatorType = PLUS
		p.i++
	case '-':
		operator = Sub
		operatorType = MINUS
		p.i++
	case '*':
		operator = Mul
		operatorType = ASTERISK
		p.i++
	case '/':
		operator = Div
		operatorType = SLASH
		p.i++
	case '\\':
		operator = SetDifference
		operatorType = ANTI_SLASH
		p.i++
	case '<':
		if p.i < p.len-1 && p.s[p.i+1] == '=' {
			operator = LessOrEqual
			operatorType = LESS_OR_EQUAL
			p.i += 2
			break
		}
		operator = LessThan
		operatorType = LESS_THAN
		p.i++
	case '>':
		if p.i < p.len-1 && p.s[p.i+1] == '=' {
			operator = GreaterOrEqual
			operatorType = GREATER_OR_EQUAL
			p.i += 2
			break
		}
		operator = GreaterThan
		operatorType = GREATER_THAN
		p.i++
	case '?':
		p.i++
		if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (endOnLinefeed || p.s[p.i] != '\n')) {
			eatInvalidOperatorToken(nameStart)
			earlyReturnedBinExpr = makeInvalidOperatorMissingRightOperand(-1)
			return
		}
		if p.s[p.i] == '?' {
			operator = NilCoalescing
			operatorType = DOUBLE_QUESTION_MARK
			p.i++
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case '!':
		p.i++

		eof := p.i >= p.len
		atEndDelim := !eof && isUnpairedOrIsClosingDelim(p.s[p.i]) && p.s[p.i] != '=' && (endOnLinefeed || p.s[p.i] != '\n')

		if eof || atEndDelim {
			eatInvalidOperatorToken(nameStart)
			earlyReturnedBinExpr = makeInvalidOperatorMissingRightOperand(-1)
			return
		}

		if p.s[p.i] == '=' {
			operator = NotEqual
			operatorType = EXCLAMATION_MARK_EQUAL
			p.i++
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case '=':
		p.i++

		eof := p.i >= p.len
		atEndDelim := !eof && isUnpairedOrIsClosingDelim(p.s[p.i]) && p.s[p.i] != '=' && (endOnLinefeed || p.s[p.i] != '\n')

		if eof || atEndDelim {
			eatInvalidOperatorToken(nameStart)
			earlyReturnedBinExpr = makeInvalidOperatorMissingRightOperand(-1)
			return
		}

		if p.s[p.i] == '=' {
			operator = Equal
			operatorType = EQUAL_EQUAL
			p.i++
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'a':
		if p.len-p.i >= AND_LEN &&
			string(p.s[p.i:p.i+AND_LEN]) == "and" &&
			(p.len-p.i == AND_LEN || !IsIdentChar(p.s[p.i+AND_LEN])) {
			operator = And
			p.i += AND_LEN
			operatorType = AND_KEYWORD
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'i':
		operatorStart := p.i

		if p.i+1 >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i+1]) && (endOnLinefeed || p.s[p.i+1] != '\n')) {
			eatInvalidOperatorToken(nameStart)
			earlyReturnedBinExpr = makeInvalidOperatorMissingRightOperand(-1)
			return
		}

		for p.i+1 < p.len && (isAlpha(p.s[p.i+1]) || p.s[p.i+1] == '-') {
			p.i++
		}

		if p.i+1 >= p.len || !IsIdentChar(p.s[p.i+1]) {
			switch string(p.s[operatorStart : p.i+1]) {
			case "in":
				operator = In
				operatorType = IN_KEYWORD
				p.i++
				break _switch
			case "is":
				operator = Is
				operatorType = IS_KEYWORD
				p.i++
				break _switch
			case "is-not":
				operator = IsNot
				operatorType = IS_NOT_KEYWORD
				p.i++
				break _switch
			}
		}

		//TODO: eat some chars
		eatInvalidOperatorToken(operatorStart)
		parsingErr = makeInvalidOperatorError()
	case 'k':
		KEYOF_LEN := int32(len("keyof"))
		if p.len-p.i >= KEYOF_LEN &&
			string(p.s[p.i:p.i+KEYOF_LEN]) == "keyof" &&
			(p.len-p.i == KEYOF_LEN || !IsIdentChar(p.s[p.i+KEYOF_LEN])) {
			operator = Keyof
			operatorType = KEYOF_KEYWORD
			p.i += KEYOF_LEN
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'n':
		NOTIN_LEN := int32(len("not-in"))
		if p.len-p.i >= NOTIN_LEN &&
			string(p.s[p.i:p.i+NOTIN_LEN]) == "not-in" &&
			(p.len-p.i == NOTIN_LEN || !IsIdentChar(p.s[p.i+NOTIN_LEN])) {
			operator = NotIn
			operatorType = NOT_IN_KEYWORD
			p.i += NOTIN_LEN
			break
		}

		NOTMATCH_LEN := int32(len("not-match"))
		if p.len-p.i >= NOTMATCH_LEN &&
			string(p.s[p.i:p.i+NOTMATCH_LEN]) == "not-match" &&
			(p.len-p.i == NOTMATCH_LEN || !IsIdentChar(p.s[p.i+NOTMATCH_LEN])) {
			operator = NotMatch
			operatorType = NOT_MATCH_KEYWORD
			p.i += NOTMATCH_LEN
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'm':
		MATCH_LEN := int32(len("match"))
		if p.len-p.i >= MATCH_LEN &&
			string(p.s[p.i:p.i+MATCH_LEN]) == "match" &&
			(p.len-p.i == MATCH_LEN || !IsIdentChar(p.s[p.i+MATCH_LEN])) {
			operator = Match
			p.i += MATCH_LEN
			operatorType = MATCH_KEYWORD
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'o':
		if p.len-p.i >= OR_LEN &&
			string(p.s[p.i:p.i+OR_LEN]) == "or" &&
			(p.len-p.i == OR_LEN || !IsIdentChar(p.s[p.i+OR_LEN])) {
			operator = Or
			operatorType = OR_KEYWORD
			p.i += OR_LEN
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 's':
		SUBSTROF_LEN := int32(len("substrof"))
		if p.len-p.i >= SUBSTROF_LEN &&
			string(p.s[p.i:p.i+SUBSTROF_LEN]) == "substrof" &&
			(p.len-p.i == SUBSTROF_LEN || !IsIdentChar(p.s[p.i+SUBSTROF_LEN])) {
			operator = Substrof
			operatorType = SUBSTROF_KEYWORD
			p.i += SUBSTROF_LEN
			break
		}
		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'u':
		operatorName := BINARY_OPERATOR_STRINGS[Urlof]

		URLOF_LEN := int32(len(operatorName))
		if p.len-p.i >= URLOF_LEN &&
			string(p.s[p.i:p.i+URLOF_LEN]) == operatorName &&
			(p.len-p.i == URLOF_LEN || !IsIdentChar(p.s[p.i+URLOF_LEN])) {
			operator = Urlof
			operatorType = URLOF_KEYWORD
			p.i += URLOF_LEN
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case '.':
		operator = Dot
		operatorType = DOT
		p.i++
	case ',':
		operator = PairComma
		operatorType = COMMA
		p.i++
	case '$', '"', '\'', '`', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': //start of right operand
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERATOR}
	default:
		p.tokens = append(p.tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		parsingErr = makeInvalidOperatorError()
	}

	if operator >= 0 {

		if p.i < p.len-1 && p.s[p.i] == '.' {
			switch operator {
			case Add, Sub, Mul, Div, GreaterThan, GreaterOrEqual, LessThan, LessOrEqual, Dot:
				p.i++
				operator++
				operatorType++
			default:
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR}
			}
		}

		if operator == Range && p.i < p.len && p.s[p.i] == '<' {
			operator = ExclEndRange
			operatorType = DOT_DOT_LESS_THAN
			p.i++
		}

		validOperatorToken = Token{Type: operatorType, Span: NodeSpan{nameStart, p.i}}
		p.tokens = append(p.tokens, validOperatorToken)
	}

	return
}
