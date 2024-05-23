package parse

import (
	"unicode/utf8"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

const (
	OR_LEN  = ast.OR_LEN
	AND_LEN = ast.AND_LEN
	AS_LEN  = ast.AS_LEN
)

func (p *parser) parseUnaryBinaryAndParenthesizedExpression(
	openingParenIndex int32, /*-1 for subsequent binary expressions in or/and chain*/
	previousOperatorEnd int32,
) ast.Node {
	p.panicIfContextDone()

	//firstParenTokenIndex := -1
	var startIndex = openingParenIndex
	hasPreviousOperator := previousOperatorEnd > 0

	if hasPreviousOperator {
		startIndex = previousOperatorEnd
	} else {
		//firstParenTokenIndex = len(p.tokens)
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{openingParenIndex, openingParenIndex + 1}})
	}

	p.eatSpaceNewlineComment()

	var (
		left          ast.Node
		isMissingExpr bool
	)

	if !hasPreviousOperator && p.i < p.len && p.s[p.i] == '<' && (p.i == p.len-1 || p.s[p.i+1] != '{') { //markup
		if p.inPattern {
			prefixed := false
			left = p.parseMarkupPatternExpression(prefixed)
		} else {
			// Markup expression without namespace.
			left = p.parseMarkupExpression(nil, p.i)
		}

		if p.areNextSpacesNewlinesCommentsFollowedBy('<') {
			//Potentially malformed markup expressions.
			return left
		}
	} else {
		left, isMissingExpr = p.parseExpression(exprParsingConfig{
			precedingOpeningParenIndexPlusOne:          openingParenIndex + 1,
			disallowUnparenthesizedBinForPipelineExprs: true,
			forceAllowForWalkExpr:                      !hasPreviousOperator,
		})
	}

	if ident, ok := left.(*ast.IdentifierLiteral); ok && !hasPreviousOperator {
		switch ident.Name {
		case ast.IF_KEYWORD_STRING:
			return p.parseIfExpression(openingParenIndex, ident.Span.Start)
		}
	}

	switch left.(type) {
	case *ast.ForExpression, *ast.WalkExpression:
		return left
	}

	p.eatSpaceNewlineComment()

	if isMissingExpr {
		if p.i >= p.len {
			if hasPreviousOperator {
				return &ast.MissingExpression{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{p.i - 1, p.i},
						Err:  &sourcecode.ParsingError{MissingExpr, fmtExprExpectedHere(p.s, p.i, false)},
					},
				}
			}
			return &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					NodeSpan{startIndex, p.i},
					left.Base().Err,
					false,
				},
			}
		}

		if p.s[p.i] == ')' {
			if !hasPreviousOperator {
				p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++

				return &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						NodeSpan{startIndex, p.i},
						left.Base().Err,
						true,
					},
				}
			} else {
				return &ast.MissingExpression{
					NodeBase: ast.NodeBase{
						Span:            NodeSpan{p.i - 1, p.i},
						Err:             &sourcecode.ParsingError{MissingExpr, fmtExprExpectedHere(p.s, p.i, false)},
						IsParenthesized: false,
					},
				}
			}
		}

		p.i++
		rune := p.s[p.i-1]
		p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Raw: string(rune), Span: NodeSpan{p.i - 1, p.i}})

		return &ast.UnknownNode{
			NodeBase: ast.NodeBase{
				NodeSpan{startIndex, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParenthesizedExpression(rune)},
				false,
			},
		}
	}

	if stringLiteral, ok := left.(*ast.UnquotedStringLiteral); ok && stringLiteral.Value == "-" { //unary negation
		operand, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		p.tokens = append(p.tokens, ast.Token{Type: ast.MINUS, Span: left.Base().Span})

		unaryExpr := &ast.UnaryExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{stringLiteral.Span.Start, p.i},
			},
			Operator: ast.NumberNegate,
			Operand:  operand,
		}

		p.eatSpace()

		if !hasPreviousOperator && p.s[p.i] == ')' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			unaryExpr.Span = NodeSpan{startIndex, p.i + 1}
			unaryExpr.IsParenthesized = true
			p.i++
			return unaryExpr
		}

		left = unaryExpr
	}

	if p.i >= p.len {
		left.BasePtr().IsParenthesized = !hasPreviousOperator

		if !hasPreviousOperator {
			if left.Base().Err == nil {
				left.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PARENTHESIZED_EXPR_MISSING_CLOSING_PAREN}
			}
		}
		return left
	}

	if p.s[p.i] == ')' { //parenthesized expression or pattern union with a leading pipe
		if !hasPreviousOperator {
			p.i++

			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i - 1, p.i}})
			left.BasePtr().IsParenthesized = true
		}
		return left
	}

	if p.s[p.i] == '|' { //pattern union without a leading pipe
		precededByOpeningParen := true

		if p.inPattern {
			patternUnion, ok := p.tryParsePatternUnionWithoutLeadingPipe(left, precededByOpeningParen)
			if ok {
				patternUnion.Span.Start = openingParenIndex
				patternUnion.IsParenthesized = true
				p.eatSpaceNewlineComment()

				if p.i >= p.len || p.s[p.i] != ')' {
					patternUnion.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT_UNION_MISSING_PAREN}
				} else {
					p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
					patternUnion.Span = NodeSpan{startIndex, p.i + 1}
					p.i++
				}
				return patternUnion
			}
		} else {
			pipelineExpr, ok := p.tryParseSecondaryStagesOfPipelineExpression(left)
			if ok {
				pipelineExpr.Span.Start = openingParenIndex
				pipelineExpr.IsParenthesized = true
				p.eatSpaceNewlineComment()

				if p.i >= p.len || p.s[p.i] != ')' {
					pipelineExpr.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PARENTHESIZED_PIPE_EXPR_MISSING_CLOSING_PAREN}
				} else {
					p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
					pipelineExpr.Span = NodeSpan{startIndex, p.i + 1}
					p.i++
				}
				return pipelineExpr
			}

		}
	}

	//Parse the operator.

	endOnLinefeed := false
	operator, operatorToken, parsingErr, earlyReturnedBinExpr := p.getBinaryOperator(left, startIndex, endOnLinefeed)

	if earlyReturnedBinExpr != nil {
		return earlyReturnedBinExpr
	}

	p.tokens = append(p.tokens, operatorToken)

	p.eatSpace()

	if p.i >= p.len {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
	}

	inPatternSave := p.inPattern

	switch operator {
	case ast.As, ast.Match, ast.NotMatch:
		p.inPattern = true
	}

	right, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	p.inPattern = inPatternSave

	p.eatSpace()
	if isMissingExpr {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
	} else if p.i >= p.len {
		if !hasPreviousOperator {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN}
		}
	}

	var continueParsing bool
	var andOrToken ast.Token
	var moveRightOperand bool

	chainElementEnd := p.i

	if p.i < p.len {
		switch p.s[p.i] {
		case 'a':
			if p.len-p.i >= AND_LEN &&
				string(p.s[p.i:p.i+AND_LEN]) == ast.AND_KEYWORD_STRING &&
				(p.len-p.i == AND_LEN || !IsIdentChar(p.s[p.i+AND_LEN])) {
				continueParsing = true
				andOrToken = ast.Token{Type: ast.AND_KEYWORD, Span: NodeSpan{p.i, p.i + AND_LEN}}
				p.i += AND_LEN
			}
		case 'o':
			if p.len-p.i >= OR_LEN &&
				string(p.s[p.i:p.i+OR_LEN]) == ast.OR_KEYWORD_STRING &&
				(p.len-p.i == OR_LEN || !IsIdentChar(p.s[p.i+OR_LEN])) {
				andOrToken = ast.Token{Type: ast.OR_KEYWORD, Span: NodeSpan{p.i, p.i + OR_LEN}}
				p.i += OR_LEN
				continueParsing = true
			}
		case ')':
			if !hasPreviousOperator {
				p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				chainElementEnd = p.i
			}
		default:
			if operator == ast.Or || operator == ast.And || isAlphaOrUndescore(p.s[p.i]) {
				continueParsing = true
				moveRightOperand = true
				andOrToken = operatorToken
			} else if isNonIdentBinaryOperatorChar(p.s[p.i]) {
				if hasPreviousOperator {
					continueParsing = true
					moveRightOperand = true
					andOrToken = operatorToken
				} else {
					parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, COMPLEX_OPERANDS_OF_BINARY_EXPRS_MUST_BE_PARENTHESIZED}
				}
			} else if !hasPreviousOperator {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN}
			}
		}
	}

	if continueParsing { //or|and chain
		var newLeft ast.Node

		if moveRightOperand {
			newLeft = left
			p.i = right.Base().Span.Start
		} else {
			newLeft = &ast.BinaryExpression{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{startIndex, chainElementEnd},
					Err:  parsingErr,
				},
				Operator: operator,
				Left:     left,
				Right:    right,
			}
		}

		//var openingParenToken ast.Token
		if !hasPreviousOperator {
			//openingParenToken = p.tokens[firstParenTokenIndex]

			if !moveRightOperand {
				newLeft.BasePtr().Span.End = right.Base().Span.End
			}
		}

		var newOperator ast.BinaryOperator = ast.And
		var newComplementOperator = ast.Or

		if andOrToken.Type == ast.OR_KEYWORD {
			newOperator = ast.Or
			newComplementOperator = ast.And
		}

		newBinExpr := &ast.BinaryExpression{
			NodeBase: ast.NodeBase{
				Span:            NodeSpan{startIndex, p.i},
				IsParenthesized: !hasPreviousOperator,
			},
			Operator: newOperator,
			Left:     newLeft,
		}

		p.tokens = append(p.tokens, andOrToken)
		// if !hasPreviousOperator {
		// 	newBinExpr.Tokens = []ast.Token{openingParenToken, andOrToken}
		// } else {
		// 	newBinExpr.Tokens = []ast.Token{andOrToken}
		// }

		p.eatSpace()

		newRight := p.parseUnaryBinaryAndParenthesizedExpression(-1, p.i)
		newBinExpr.Right = newRight

		p.eatSpace()

		if !hasPreviousOperator {
			if p.i >= p.len || p.s[p.i] != ')' {
				if _, ok := newRight.(*ast.MissingExpression); !ok {
					newBinExpr.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN}
				}
				newBinExpr.Span.End = newRight.Base().Span.End
			} else {
				p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				newBinExpr.Span.End = p.i
			}

			if rightBinExpr, ok := newRight.(*ast.BinaryExpression); ok &&
				!rightBinExpr.IsParenthesized && newBinExpr.Err == nil {

				subLeft, isSubLeftBinExpr := rightBinExpr.Left.(*ast.BinaryExpression)
				subRight, isSubRightBinExpr := rightBinExpr.Right.(*ast.BinaryExpression)

				err := &sourcecode.ParsingError{UnspecifiedParsingError, BIN_EXPR_CHAIN_OPERATORS_SHOULD_BE_THE_SAME}

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

	return &ast.BinaryExpression{
		NodeBase: ast.NodeBase{
			Span:            NodeSpan{startIndex, chainElementEnd},
			Err:             parsingErr,
			IsParenthesized: !hasPreviousOperator,
		},
		Operator: operator,
		Left:     left,
		Right:    right,
	}
}

func (p *parser) tryParseUnparenthesizedBinaryExpr(left ast.Node) (ast.Node, bool) {
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
		if tempIndex < p.len-1 && !isSpace(p.s[tempIndex+1]) && p.s[tempIndex+1] != '.' {
			//Prevent parsing property name literals.
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
	case '*', '>', '!':
	case '<':
		if utils.Implements[*ast.MarkupExpression](left) {
			//$left is potentially malformed
			return nil, false
		}
	case '=':
		if tempIndex < p.len-1 && p.s[tempIndex+1] != '=' {
			//=>
			//=}
			return nil, false
		}
	case '+', '/', '-':
		//We don't check that we are not at the start of an unquoted string literal or path because since `(a +b)` is parsed as a binary expression,
		//`a +b` should be parsed the same. Also tryParseUnparenthesizedBinaryExpr is never called in command-like calls.
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
		for _, operator := range ast.BINARY_OPERATOR_STRINGS {
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
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
	}

	inPatternSave := p.inPattern

	switch operator {
	case ast.As, ast.Match, ast.NotMatch:
		p.inPattern = true
	}

	right, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	p.inPattern = inPatternSave

	if isMissingExpr {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_RIGHT_OPERAND}
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
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, COMPLEX_OPERANDS_OF_BINARY_EXPRS_MUST_BE_PARENTHESIZED}
			}
		}
	}

	return &ast.BinaryExpression{
		NodeBase: ast.NodeBase{
			Span:            NodeSpan{startIndex, p.i},
			Err:             parsingErr,
			IsParenthesized: false,
		},
		Operator: operator,
		Left:     left,
		Right:    right,
	}, true
}

func (p *parser) getBinaryOperator(left ast.Node, startIndex int32, endOnLinefeed bool) (
	operator ast.BinaryOperator,
	validOperatorToken ast.Token, //if not zero it is added to the tokens before the function returns.
	parsingErr *sourcecode.ParsingError,
	earlyReturnedBinExpr *ast.BinaryExpression,
) {
	p.panicIfContextDone()

	defer func() {
		if validOperatorToken != (ast.Token{}) {
			p.tokens = append(p.tokens, validOperatorToken)
		}
	}()

	operator = -1

	makeInvalidOperatorError := func() *sourcecode.ParsingError {
		return &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR}
	}

	makeInvalidOperatorMissingRightOperand := func(operator ast.BinaryOperator) *ast.BinaryExpression {
		return &ast.BinaryExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{startIndex, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERAND_OR_INVALID_OPERATOR},
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
		p.tokens = append(p.tokens, ast.Token{
			Type: ast.INVALID_OPERATOR,
			Span: NodeSpan{Start: operatorStart, End: p.i},
			Raw:  string(p.s[operatorStart:p.i]),
		})
	}

	var (
		nameStart    = p.i
		operatorType ast.TokenType
	)

_switch:
	switch p.s[p.i] {
	case '+':
		operator = ast.Add
		operatorType = ast.PLUS
		p.i++
	case '-':
		operator = ast.Sub
		operatorType = ast.MINUS
		p.i++
	case '*':
		operator = ast.Mul
		operatorType = ast.ASTERISK
		p.i++
	case '/':
		operator = ast.Div
		operatorType = ast.SLASH
		p.i++
	case '\\':
		operator = ast.SetDifference
		operatorType = ast.ANTI_SLASH
		p.i++
	case '<':
		if p.i < p.len-1 && p.s[p.i+1] == '=' {
			operator = ast.LessOrEqual
			operatorType = ast.LESS_OR_EQUAL
			p.i += 2
			break
		}
		operator = ast.LessThan
		operatorType = ast.LESS_THAN
		p.i++
	case '>':
		if p.i < p.len-1 && p.s[p.i+1] == '=' {
			operator = ast.GreaterOrEqual
			operatorType = ast.GREATER_OR_EQUAL
			p.i += 2
			break
		}
		operator = ast.GreaterThan
		operatorType = ast.GREATER_THAN
		p.i++
	case '?':
		p.i++
		if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (endOnLinefeed || p.s[p.i] != '\n')) {
			eatInvalidOperatorToken(nameStart)
			earlyReturnedBinExpr = makeInvalidOperatorMissingRightOperand(-1)
			return
		}
		if p.s[p.i] == '?' {
			operator = ast.NilCoalescing
			operatorType = ast.DOUBLE_QUESTION_MARK
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
			operator = ast.NotEqual
			operatorType = ast.EXCLAMATION_MARK_EQUAL
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
			operator = ast.Equal
			operatorType = ast.EQUAL_EQUAL
			p.i++
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'a':
		if p.len-p.i >= AND_LEN &&
			string(p.s[p.i:p.i+AND_LEN]) == ast.AND_KEYWORD_STRING &&
			(p.len-p.i == AND_LEN || !IsIdentChar(p.s[p.i+AND_LEN])) {
			operator = ast.And
			p.i += AND_LEN
			operatorType = ast.AND_KEYWORD
			break
		}

		if p.len-p.i >= AS_LEN &&
			string(p.s[p.i:p.i+AS_LEN]) == ast.AS_KEYWORD_STRING &&
			(p.len-p.i == AS_LEN || !IsIdentChar(p.s[p.i+AS_LEN])) {
			operator = ast.As
			p.i += AS_LEN
			operatorType = ast.AS_KEYWORD
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
				operator = ast.In
				operatorType = ast.IN_KEYWORD
				p.i++
				break _switch
			case "is":
				operator = ast.Is
				operatorType = ast.IS_KEYWORD
				p.i++
				break _switch
			case "is-not":
				operator = ast.IsNot
				operatorType = ast.IS_NOT_KEYWORD
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
			operator = ast.Keyof
			operatorType = ast.KEYOF_KEYWORD
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
			operator = ast.NotIn
			operatorType = ast.NOT_IN_KEYWORD
			p.i += NOTIN_LEN
			break
		}

		NOTMATCH_LEN := int32(len("not-match"))
		if p.len-p.i >= NOTMATCH_LEN &&
			string(p.s[p.i:p.i+NOTMATCH_LEN]) == "not-match" &&
			(p.len-p.i == NOTMATCH_LEN || !IsIdentChar(p.s[p.i+NOTMATCH_LEN])) {
			operator = ast.NotMatch
			operatorType = ast.NOT_MATCH_KEYWORD
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
			operator = ast.Match
			p.i += MATCH_LEN
			operatorType = ast.MATCH_KEYWORD
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'o':
		if p.len-p.i >= OR_LEN &&
			string(p.s[p.i:p.i+OR_LEN]) == ast.OR_KEYWORD_STRING &&
			(p.len-p.i == OR_LEN || !IsIdentChar(p.s[p.i+OR_LEN])) {
			operator = ast.Or
			operatorType = ast.OR_KEYWORD
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
			operator = ast.Substrof
			operatorType = ast.SUBSTROF_KEYWORD
			p.i += SUBSTROF_LEN
			break
		}
		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case 'u':
		operatorName := ast.BINARY_OPERATOR_STRINGS[ast.Urlof]

		URLOF_LEN := int32(len(operatorName))
		if p.len-p.i >= URLOF_LEN &&
			string(p.s[p.i:p.i+URLOF_LEN]) == operatorName &&
			(p.len-p.i == URLOF_LEN || !IsIdentChar(p.s[p.i+URLOF_LEN])) {
			operator = ast.Urlof
			operatorType = ast.URLOF_KEYWORD
			p.i += URLOF_LEN
			break
		}

		eatInvalidOperatorToken(nameStart)
		parsingErr = makeInvalidOperatorError()
	case '.':
		operator = ast.Dot
		operatorType = ast.DOT
		p.i++
	case ',':
		operator = ast.PairComma
		operatorType = ast.COMMA
		p.i++
	case '$', '"', '\'', '`', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': //start of right operand
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERATOR}
	default:
		p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		parsingErr = makeInvalidOperatorError()
	}

	if operator >= 0 {

		if p.i < p.len-1 && p.s[p.i] == '.' {
			switch operator {
			case ast.Add, ast.Sub, ast.Mul, ast.Div, ast.GreaterThan, ast.GreaterOrEqual, ast.LessThan, ast.LessOrEqual, ast.Dot:
				p.i++
				operator++
				operatorType++
			default:
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR}
			}
		}

		if operator == ast.Range && p.i < p.len && p.s[p.i] == '<' {
			operator = ast.ExclEndRange
			operatorType = ast.DOT_DOT_LESS_THAN
			p.i++
		}

		validOperatorToken = ast.Token{Type: operatorType, Span: NodeSpan{nameStart, p.i}}
		p.tokens = append(p.tokens, validOperatorToken)
	}

	return
}
