package parse

import (
	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
)

type exprParsingConfig struct {
	precedingOpeningParenIndexPlusOne          int32 //0 if no parenthesis
	statement                                  bool
	disallowUnparenthesizedBinForPipelineExprs bool
	disallowParsingSeveralPatternUnionCases    bool
	forceAllowForWalkExpr                      bool
}

// parseExpression parses any expression, if $expr is a *ast.MissingExpression $isMissingExpr will be true.
// The term 'expression' is quite broad here, it refers to any standalone node type that is not a statement
func (p *parser) parseExpression(config ...exprParsingConfig) (expr ast.Node, isMissingExpr bool) {
	p.panicIfContextDone()

	var precedingOpeningParenIndex int32 = -1
	precededByOpeningParen := false
	isStmt := false
	forceAllowForWalkExpr := true

	if len(config) > 0 {
		if config[0].precedingOpeningParenIndexPlusOne > 0 {
			precedingOpeningParenIndex = config[0].precedingOpeningParenIndexPlusOne - 1
			precededByOpeningParen = true
		}
		isStmt = config[0].statement
		forceAllowForWalkExpr = !isStmt && (!config[0].disallowUnparenthesizedBinForPipelineExprs || config[0].forceAllowForWalkExpr)
	}

	defer func() {
		allowUnparenthesizedBinForPipelineExprs := !p.inPattern && !isStmt && (len(config) == 0 || !config[0].disallowUnparenthesizedBinForPipelineExprs)

		if expr != nil && !isMissingExpr && allowUnparenthesizedBinForPipelineExprs {
			pipelineExpr, ok := p.tryParseSecondaryStagesOfPipelineExpression(expr)
			if ok {
				expr = pipelineExpr
				return
			}

			binExpr, ok := p.tryParseUnparenthesizedBinaryExpr(expr)
			if ok {
				expr = binExpr
				return
			}
		}

		//union pattern without leading pipe
		if p.inPattern && !isStmt && expr != nil && !isMissingExpr && (len(config) == 0 || !config[0].disallowParsingSeveralPatternUnionCases) {
			patternUnion, ok := p.tryParsePatternUnionWithoutLeadingPipe(expr, precededByOpeningParen)
			if ok {
				expr = patternUnion
				return
			}
		}
	}()

	exprStartIndex := p.i
	// these variables are only used for expressions that can be on the left side of a member/slice/index/call expression,
	// other expressions are directly returned.
	var (
		left  ast.Node
		first ast.Node
	)

	if p.i >= p.len {
		return &ast.MissingExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{p.i - 1, p.i},
				Err:  &sourcecode.ParsingError{MissingExpr, fmtExprExpectedHere(p.s, p.i, false)},
			},
		}, true
	}

	switch p.s[p.i] {
	case '$': //variables and URL expressions
		start := p.i
		p.i++

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		variable := &ast.Variable{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Name: string(p.s[start+1 : p.i]),
		}

		if p.i < p.len && p.s[p.i] == '/' {
			return p.parseURLLike(start, variable), false
		}

		left = variable
	case '!':
		p.i++
		operand, _ := p.parseExpression()
		p.tokens = append(p.tokens, ast.Token{Type: ast.EXCLAMATION_MARK, Span: NodeSpan{exprStartIndex, exprStartIndex + 1}})

		return &ast.UnaryExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{exprStartIndex, operand.Base().Span.End},
			},
			Operator: ast.BoolNegate,
			Operand:  operand,
		}, false
	case '~':
		p.i++
		expr, _ := p.parseExpression()
		p.tokens = append(p.tokens, ast.Token{Type: ast.TILDE, Span: NodeSpan{exprStartIndex, exprStartIndex + 1}})

		return &ast.RuntimeTypeCheckExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{exprStartIndex, expr.Base().Span.End},
			},
			Expr: expr,
		}, false
	case ':':
		if p.i >= p.len-1 {
			break
		}

		switch p.s[p.i+1] {
		case '/':
			if p.i >= p.len-2 || p.s[p.i+2] != '/' {
				break
			}
			//Scheme-less
			return p.parseURLLike(p.i, nil), false
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return p.parsePortLiteral(), false
		case '{':
			if p.inPattern {
				return p.parseDictionaryPatternLiteral(), false
			}
			return p.parseDictionaryLiteral(), false
		}

	//TODO: refactor ?
	case '_', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		e, returnNow := p.parseUnderscoreAlphaStartingExpression(precedingOpeningParenIndex, isStmt, forceAllowForWalkExpr)
		if returnNow {
			return e, false
		}
		left = e
	case '0':
		if p.i < p.len-2 && isByteSliceBase(p.s[p.i+1]) && p.s[p.i+2] == '[' {
			return p.parseByteSlices(), false
		}
		return p.parseNumberAndRangeAndRateLiterals(), false
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.parseNumberAndRangeAndRateLiterals(), false
	case '{':
		if p.inPattern {
			return p.parseObjectRecordPatternLiteral(false, false), false
		}
		return p.parseObjectOrRecordLiteral(false), false
	case '[':
		if p.inPattern {
			return p.parseListTuplePatternLiteral(false, false), false
		}
		return p.parseListOrTupleLiteral(false), false
	case '|':
		if p.inPattern {
			caseBeforeFirstPipe := ast.Node(nil)
			return p.parsePatternUnion(p.i, false, caseBeforeFirstPipe, precededByOpeningParen), false
		}
	case '\'':
		return p.parseRuneRuneRange(), false
	case '"':
		return p.parseQuotedStringLiteral(), false
	case '`':
		if p.inPattern {
			return p.parseRegularExpressionLiteral(false), false
		}
		return p.parseStringTemplateLiteralOrMultilineStringLiteral(nil), false
	case '+':
		if p.i < p.len-1 && isDecDigit(p.s[p.i+1]) {
			break
		}
		start := p.i
		return p.parseUnquotedStringLiteral(start), false

	case '/':
		return p.parsePathLikeExpression(false), false
	case '.':
		return p.parseDotStartingExpression(), false
	case '-':
		return p.parseDashStartingExpression(precededByOpeningParen), false
	case '#':
		if p.i < p.len-1 {
			switch p.s[p.i+1] {
			case '{':
				if p.inPattern {
					return p.parseObjectRecordPatternLiteral(false, true), false
				}
				return p.parseObjectOrRecordLiteral(true), false
			case '[':
				if p.inPattern {
					return p.parseListTuplePatternLiteral(false, true), false
				}
				return p.parseListOrTupleLiteral(true), false
			}
		}
		p.i++

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		var parsingErr *sourcecode.ParsingError

		if p.i == exprStartIndex+1 {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT}
		}

		return &ast.UnambiguousIdentifierLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{exprStartIndex, p.i},
				Err:  parsingErr,
			},
			Name: string(p.s[exprStartIndex+1 : p.i]),
		}, false
	case '@':
		e, returnNow := p.parseQuotedAndMetaStuff()
		if returnNow {
			return e, false
		}
		left = e

		if p.i < p.len {
			call := p.tryParseCall(left, "")
			if call != nil {
				left = call
			}
		}
	case '<':
		if p.i+1 < p.len {
			switch {
			case isAlpha(p.s[p.i+1]): //markup expression without namespace.
				if p.inPattern {
					prefixed := false
					return p.parseMarkupPatternExpression(prefixed), false
				}
				return p.parseMarkupExpression(nil, p.i), false
			case p.s[p.i+1] == '{':
				return p.parseUnquotedRegion(), false
			}
		}
	case '*':
		start := p.i
		p.tokens = append(p.tokens, ast.Token{Type: ast.ASTERISK, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		if p.inPattern {
			typ, _ := p.parseExpression()
			return &ast.PointerType{
				NodeBase:  ast.NodeBase{Span: NodeSpan{start, typ.Base().Span.End}},
				ValueType: typ,
			}, false
		} else {
			pointer, _ := p.parseExpression()
			return &ast.DereferenceExpression{
				NodeBase: ast.NodeBase{Span: NodeSpan{start, pointer.Base().Span.End}},
				Pointer:  pointer,
			}, false
		}
	case '%':
		patt := p.parsePercentPrefixedPattern(precededByOpeningParen)

		switch patt.(type) {
		case *ast.PatternIdentifierLiteral, *ast.PatternNamespaceMemberExpression:
			if p.i < p.len && p.s[p.i] == '`' {
				return p.parseStringTemplateLiteralOrMultilineStringLiteral(patt), false
			}
		}
		return patt, false
	case '(': //parenthesized expression, unary expression, binary expression, pattern union
		openingParenIndex := p.i
		p.i++

		left = p.parseUnaryBinaryAndParenthesizedExpression(openingParenIndex, -1)
		if p.i >= p.len {
			return left, false
		}
	}

	first = left

loop:
	for left != nil && p.i < p.len && (!isUnpairedOrIsClosingDelim(p.s[p.i]) || (p.i < p.len-1 && p.s[p.i] == ':' && p.s[p.i+1] == ':')) {
		isDoubleColon := p.i < p.len-1 && p.s[p.i] == ':' && p.s[p.i+1] == ':'

		switch {
		case p.s[p.i] == '[' || p.s[p.i] == '.' || isDoubleColon:
			//member expressions, index/slice expressions, extraction expression & double-colon expressions.
			membLike, continueLoop := p.parseMemberLike(exprStartIndex, first, left, isDoubleColon)
			if continueLoop {
				left = membLike
				continue loop
			}
			return membLike, false
		case ((p.i < p.len && p.s[p.i] == '(') ||
			(p.i < p.len-1 && p.s[p.i] == '!' && p.s[p.i+1] == '(')): //call: <left> '(' ...

			must := false
			if p.s[p.i] == '!' {
				must = true
				p.i++
				p.tokens = append(p.tokens,
					ast.Token{Type: ast.EXCLAMATION_MARK, Span: NodeSpan{p.i - 1, p.i}},
					ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}},
				)
			} else {
				p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			}

			p.i++
			spanStart := left.Base().Span.Start

			if left == first {
				spanStart = exprStartIndex
			}

			call := &ast.CallExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{spanStart, 0},
					nil,
					false,
				},
				Callee:    left,
				Arguments: nil,
				Must:      must,
			}

			left = p.parseParenthesizedCallArgs(call)
		case p.s[p.i] == '?':
			p.i++
			left = &ast.BooleanConversionExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{exprStartIndex, p.i},
					nil,
					false,
				},
				Expr: left,
			}
		default:
			break loop
		}
	}

	if left != nil {
		return left, false
	}

	return &ast.MissingExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{p.i, p.i + 1},
			Err:  &sourcecode.ParsingError{MissingExpr, fmtExprExpectedHere(p.s, p.i, true)},
		},
	}, true
}

func (p *parser) parseUnderscoreAlphaStartingExpression(precedingOpeningParen int32, stmt, forceAllowForExpr bool) (node ast.Node, returnNow bool) {
	returnNow = true
	precededByOpeningParen := precedingOpeningParen >= 0
	identStartingExpr := p.parseIdentStartingExpression(p.inPattern)

	var name string

	switch v := identStartingExpr.(type) {
	case *ast.IdentifierLiteral:
		name = v.Name

		if p.inPattern {
			switch name {
			case ast.FN_KEYWORD_STRING:
				node = p.parseFunctionPattern(identStartingExpr.Base().Span.Start, false)
				return
			case ast.READONLY_KEYWORD_STRING:
				node = p.parseReadonlyPatternExpression(v)
				return
			}

			result := &ast.PatternIdentifierLiteral{
				NodeBase:   v.NodeBase,
				Unprefixed: true,
				Name:       v.Name,
			}
			if p.i < p.len {
				switch p.s[p.i] {
				case '(', '{':
					if v.Name == "str" && p.s[p.i] == '(' {
						p.i++
						node = p.parseComplexStringPatternPiece(result.Span.Start, rootSequencePatternPiece, result)
						return
					}

					node = p.parsePatternCall(result)
					return
				case '?':
					p.i++
					node = &ast.OptionalPatternExpression{
						NodeBase: ast.NodeBase{
							Span: NodeSpan{result.Base().Span.Start, p.i},
						},
						Pattern: result,
					}
					return
				}
			}
			node = result
			return
		}

		switch name {
		case ast.TOKEN_STRINGS[ast.GO_KEYWORD]:
			node = p.parseSpawnExpression(identStartingExpr)
			return
		case ast.FN_KEYWORD_STRING:
			node = p.parseFunction(identStartingExpr.Base().Span.Start)
			return
		case "s":
			if p.i < p.len && p.s[p.i] == '!' {
				p.i++
				node = p.parseTopCssSelector(p.i - 2)
				return
			}
		case ast.TOKEN_STRINGS[ast.MAPPING_KEYWORD]:
			node = p.parseMappingExpression(v)
			return
		case ast.TOKEN_STRINGS[ast.COMP_KEYWORD]:
			node = p.parseComputeExpression(v)
			return
		case ast.TOKEN_STRINGS[ast.TREEDATA_KEYWORD]:
			node = p.parseTreedataLiteral(v)
			return
		case ast.TOKEN_STRINGS[ast.CONCAT_KEYWORD]:
			node = p.parseConcatenationExpression(v, precededByOpeningParen)
			return
		case ast.TOKEN_STRINGS[ast.TESTSUITE_KEYWORD]:
			node = p.parseTestSuiteExpression(v)
			return
		case ast.TOKEN_STRINGS[ast.TESTCASE_KEYWORD]:
			node = p.parseTestCaseExpression(v)
			return
		case ast.NEW_KEYWORD_STRING:
			node = p.parseNewExpression(v)
			return
		case ast.FOR_KEYWORD_STRING:
			if !stmt && forceAllowForExpr {
				node = p.parseForExpression(int32(precedingOpeningParen), v.Span.Start)
				return
			}
		case ast.WALK_KEYWORD_STRING:
			if !stmt && forceAllowForExpr {
				node = p.parseWalkExpression(int32(precedingOpeningParen), v.Span.Start)
				return
			}
		default:
			if !stmt && (name == ast.SWITCH_KEYWORD_STRING || name == ast.MATCH_KEYWORD_STRING) {
				node = p.parseSwitchMatchExpression(v)
				return
			}
		}

		if isKeyword(name) {
			node = v
			return
		}
	case *ast.IdentifierMemberExpression:
		if p.inPattern && len(v.PropertyNames) == 1 {
			base := v.Left.NodeBase
			base.Span.End += 1 //add one for the dot

			result := &ast.PatternNamespaceMemberExpression{
				NodeBase: v.NodeBase,
				Namespace: &ast.PatternNamespaceIdentifierLiteral{
					NodeBase:   base,
					Unprefixed: true,
					Name:       v.Left.Name,
				},
				MemberName: v.PropertyNames[0],
			}
			if p.i < p.len {
				switch p.s[p.i] {
				case '(', '{':
					node = p.parsePatternCall(result)
					return
				case '?':
					p.i++
					node = &ast.OptionalPatternExpression{
						NodeBase: ast.NodeBase{
							Span: NodeSpan{result.Base().Span.Start, p.i},
						},
						Pattern: result,
					}
					return
				}
			}
			node = result
			return
		}

		name = v.Left.Name
	case *ast.SelfExpression, *ast.MemberExpression:
		node = identStartingExpr
		returnNow = false
	default:
		node = v
		return
	}

	if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (p.i >= p.len-1 || p.s[p.i] != ':' || p.s[p.i+1] != ':')) {
		node = identStartingExpr
		return
	}

	if p.s[p.i] == '<' && ast.NodeIs(identStartingExpr, (*ast.IdentifierLiteral)(nil)) {
		ident := identStartingExpr.(*ast.IdentifierLiteral)
		node = p.parseMarkupExpression(ident, ident.Span.Start)
		return
	}

	call := p.tryParseCall(identStartingExpr, name)
	if call != nil {
		identStartingExpr = call
	}

	node = identStartingExpr
	returnNow = false
	return
}

func (p *parser) parseMemberLike(_start int32, first, left ast.Node, isDoubleColon bool) (result ast.Node, continueLoop bool) {
	dot := p.s[p.i] == '.'
	isBracket := p.s[p.i] == '['
	tokenStart := p.i

	if isDoubleColon {
		p.i++
	}

	p.i++
	start := p.i
	isOptional := false

	isDot := p.s[p.i-1] == '.'

	if isDot && p.i < p.len && p.s[p.i] == '?' {
		isOptional = true
		p.i++
		start = p.i
	}

	if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (dot || (p.s[p.i] != ':' && p.s[p.i] != ']'))) {
		//unterminated member expression
		if isDot {
			result = &ast.MemberExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&sourcecode.ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
					false,
				},
				Left:     left,
				Optional: isOptional,
			}
			return
		}
		if isDoubleColon {
			p.tokens = append(p.tokens, ast.Token{Type: ast.DOUBLE_COLON, Span: NodeSpan{tokenStart, tokenStart + 2}})

			result = &ast.DoubleColonExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&sourcecode.ParsingError{UnterminatedDoubleColonExpr, UNTERMINATED_DOUBLE_COLON_EXPR},
					false,
				},
				Left: left,
			}
			return
		}
		result = &ast.InvalidMemberLike{
			NodeBase: ast.NodeBase{
				NodeSpan{first.Base().Span.Start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MEMB_OR_INDEX_EXPR},
				false,
			},
			Left: left,
		}
		return
	}

	switch {
	case isBracket: //index/slice expression
		p.eatSpace()

		if p.i >= p.len {
			result = &ast.InvalidMemberLike{
				NodeBase: ast.NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
					false,
				},
				Left: left,
			}
			return
		}

		var startIndex ast.Node
		var endIndex ast.Node
		isSliceExpr := p.s[p.i] == ':'

		if isSliceExpr {
			p.i++
		} else {
			startIndex, _ = p.parseExpression()
		}

		p.eatSpace()

		if p.i >= p.len {
			result = &ast.InvalidMemberLike{
				NodeBase: ast.NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
					false,
				},
				Left: left,
			}
			return
		}

		if p.s[p.i] == ':' {
			if isSliceExpr {
				result = &ast.SliceExpression{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{first.Base().Span.Start, p.i},
						Err:  &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_SLICE_EXPR_SINGLE_COLON},
					},
					Indexed:    left,
					StartIndex: startIndex,
					EndIndex:   endIndex,
				}
				return
			}
			isSliceExpr = true
			p.i++
		}

		p.eatSpace()

		if isSliceExpr && startIndex == nil && (p.i >= p.len || p.s[p.i] == ']') {
			result = &ast.SliceExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_SLICE_EXPR_MISSING_END_INDEX},
					false,
				},
				Indexed:    left,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			}
			return
		}

		if p.i < p.len && p.s[p.i] != ']' && isSliceExpr {
			endIndex, _ = p.parseExpression()
		}

		p.eatSpace()

		if p.i >= p.len || p.s[p.i] != ']' {
			result = &ast.InvalidMemberLike{
				NodeBase: ast.NodeBase{
					NodeSpan{first.Base().Span.Start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR_MISSING_CLOSING_BRACKET},
					false,
				},
				Left: left,
			}
			return
		}

		p.i++

		spanStart := left.Base().Span.Start
		if left == first {
			spanStart = _start
		}

		if isSliceExpr {
			result = &ast.SliceExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{spanStart, p.i},
					nil,
					false,
				},
				Indexed:    left,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			}
			continueLoop = true
			return
		}

		result = &ast.IndexExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{spanStart, p.i},
				nil,
				false,
			},
			Indexed: left,
			Index:   startIndex,
		}
		continueLoop = true
		return
	case isDoubleColon: //double-colon expression
		p.tokens = append(p.tokens, ast.Token{Type: ast.DOUBLE_COLON, Span: NodeSpan{tokenStart, tokenStart + 2}})

		elementNameStart := p.i
		var parsingErr *sourcecode.ParsingError
		if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtDoubleColonExpressionelementShouldStartWithAletterNot(p.s[p.i])}
		}

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		spanStart := left.Base().Span.Start
		if left == first {
			spanStart = _start
		}

		elementName := string(p.s[elementNameStart:p.i])
		if left == first {
			spanStart = _start
		}

		element := &ast.IdentifierLiteral{
			NodeBase: ast.NodeBase{
				NodeSpan{elementNameStart, p.i},
				nil,
				false,
			},
			Name: elementName,
		}

		result = &ast.DoubleColonExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{spanStart, p.i},
				Err:  parsingErr,
			},
			Left:    left,
			Element: element,
		}
		continueLoop = true
		return
	case p.s[p.i] == '{': //extraction expression (result is returned, the loop is not continued)
		p.i--
		keyList := p.parseKeyList()

		result = &ast.ExtractionExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{left.Base().Span.Start, keyList.Span.End},
				nil,
				false,
			},
			Object: left,
			Keys:   keyList,
		}
		continueLoop = true
		return
	default:
		isComputed := false
		spanStart := left.Base().Span.Start
		var computedPropertyNode ast.Node
		var propertyNameIdent *ast.IdentifierLiteral
		propNameStart := start

		if !isOptional && p.i < p.len {
			switch p.s[p.i] {
			case '(':
				isComputed = true
				p.i++
				computedPropertyNode = p.parseUnaryBinaryAndParenthesizedExpression(p.i-1, -1)
			}
		}

		newMemberExpression := func(err *sourcecode.ParsingError) ast.Node {
			if isComputed {
				return &ast.ComputedMemberExpression{
					NodeBase: ast.NodeBase{
						NodeSpan{spanStart, p.i},
						err,
						false,
					},
					Left:         left,
					PropertyName: computedPropertyNode,
					Optional:     isOptional,
				}
			}
			return &ast.MemberExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{spanStart, p.i},
					err,
					false,
				},
				Left:         left,
				PropertyName: propertyNameIdent,
				Optional:     isOptional,
			}
		}

		if !isComputed {

			//member expression with invalid property name
			if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
				result = newMemberExpression(&sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: fmtPropNameShouldStartWithAletterNot(p.s[p.i])})
				return
			}

			for p.i < p.len && IsIdentChar(p.s[p.i]) {
				p.i++
			}

			propName := string(p.s[propNameStart:p.i])
			if left == first {
				spanStart = _start
			}

			propertyNameIdent = &ast.IdentifierLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{propNameStart, p.i},
					nil,
					false,
				},
				Name: propName,
			}
		}

		result = newMemberExpression(nil)
		continueLoop = true
		return
	}
}
