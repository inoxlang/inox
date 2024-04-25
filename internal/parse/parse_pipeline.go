package parse

func (p *parser) parseCommandLikeStatement(expr Node) Node {
	p.panicIfContextDone()

	call := &CallExpression{
		NodeBase: NodeBase{
			Span: NodeSpan{expr.Base().Span.Start, 0},
		},
		Callee:            expr,
		Arguments:         nil,
		Must:              true,
		CommandLikeSyntax: true,
	}

	p.parseCallArgsNoParenthesis(call)

	call.NodeBase.Span.End = p.i

	p.eatSpace()

	//normal call

	if p.i >= p.len || p.s[p.i] != '|' {
		return call
	}

	//pipe statement

	stmt := &PipelineStatement{
		NodeBase: NodeBase{
			NodeSpan{call.Span.Start, 0},
			nil,
			false,
		},
		Stages: []*PipelineStage{
			{
				Kind: NormalStage,
				Expr: call,
			},
		},
	}

	p.tokens = append(p.tokens, Token{Type: PIPE, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	p.eatSpace()

	if p.i >= p.len {
		stmt.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PIPE_STMT_LAST_STAGE_EMPTY}
		return stmt
	}

	for p.i < p.len && p.s[p.i] != '\n' && p.s[p.i] != ';' && !IsCommentStart(p.s, p.i) {
		p.eatSpace()
		if p.i >= p.len {
			stmt.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PIPE_STMT_LAST_STAGE_EMPTY}
			return stmt
		}

		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		switch e.(type) {
		case *IdentifierLiteral, *IdentifierMemberExpression:

			p.eatSpace()

			if p.i >= p.len || p.s[p.i] == '\n' || p.s[p.i] == '|' || p.s[p.i] == ';' { //if no arguments
				stmt.Stages = append(stmt.Stages, &PipelineStage{
					Kind: NormalStage,
					Expr: e,
				})
				stmt.Span.End = p.i
				break
			}

			currentCall := &CallExpression{
				NodeBase: NodeBase{
					Span: NodeSpan{e.Base().Span.Start, 0},
				},
				Callee:            e,
				Arguments:         nil,
				Must:              true,
				CommandLikeSyntax: true,
			}

			stmt.Stages = append(stmt.Stages, &PipelineStage{
				Kind: NormalStage,
				Expr: currentCall,
			})

			p.parseCallArgsNoParenthesis(currentCall)

			if len32(currentCall.Arguments) == 0 {
				currentCall.NodeBase.Span.End = e.Base().Span.End
			} else {
				currentCall.NodeBase.Span.End = currentCall.Arguments[len32(currentCall.Arguments)-1].Base().Span.End
			}

			stmt.Span.End = currentCall.Span.End
		case *CallExpression:
			stmt.Stages = append(stmt.Stages, &PipelineStage{
				Kind: NormalStage,
				Expr: e,
			})
			stmt.Span.End = p.i
		default:
			stmt.Stages = append(stmt.Stages, &PipelineStage{
				Kind: NormalStage,
				Expr: e,
			})
			stmt.Span.End = p.i

			base := e.BasePtr()
			if base.Err == nil {
				base.Err = &ParsingError{UnspecifiedParsingError, INVALID_PIPE_STMT_STAGE_ALL_STAGES_SHOULD_BE_CALLS}
			}
		}

		p.eatSpace()

		if p.i >= p.len {
			return stmt
		}

		switch p.s[p.i] {
		case '|':
			p.tokens = append(p.tokens, Token{Type: PIPE, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			continue //we parse the next stage
		case '\n':
			return stmt
		case ';':
			return stmt
		default:
			if !IsCommentStart(p.s, p.i) {
				stmt.Err = &ParsingError{UnspecifiedParsingError, fmtInvalidPipelineStageUnexpectedChar(p.s[p.i])}
			}
			return stmt
		}
	}

	return stmt
}

func (p *parser) tryParseSecondaryStagesOfPipelineExpression(left Node) (pipelineExpr *PipelineExpression, isPresent bool) {
	p.panicIfContextDone()

	startIndex := left.Base().Span.Start

	tempIndex := p.i
	{
		//We can only parse regular whitespace.
		for tempIndex < p.len && isSpaceNotLF(p.s[tempIndex]) {
			tempIndex++
		}
	}

	if tempIndex >= p.len || p.s[tempIndex] != '|' {
		return nil, false
	}
	isPresent = true

	p.i = tempIndex

	p.tokens = append(p.tokens, Token{Type: PIPE, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	p.eatSpace()

	pipelineExpr = &PipelineExpression{
		NodeBase: NodeBase{Span: NodeSpan{startIndex, p.i}},
		Stages: []*PipelineStage{
			{
				Kind: NormalStage,
				Expr: left,
			},
		},
	}

	defer func() {
		if len(pipelineExpr.Stages) == 1 {
			pipelineExpr.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PIPE_EXPR_LAST_STAGE_EMPTY}
		}
	}()

	for p.i < p.len && !IsCommentStart(p.s, p.i) && (p.s[p.i] == '|' || !isUnpairedOrIsClosingDelim(p.s[p.i])) {
		p.eatSpace()
		if p.i >= p.len {
			pipelineExpr.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PIPE_EXPR_LAST_STAGE_EMPTY}
			return
		}

		e, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		switch e.(type) {
		case *IdentifierLiteral, *IdentifierMemberExpression:
			pipelineExpr.Stages = append(pipelineExpr.Stages, &PipelineStage{
				Kind: NormalStage,
				Expr: e,
			})
			pipelineExpr.Span.End = e.Base().Span.End
		case *CallExpression:
			pipelineExpr.Stages = append(pipelineExpr.Stages, &PipelineStage{
				Kind: NormalStage,
				Expr: e,
			})
			pipelineExpr.Span.End = p.i
		default:
			pipelineExpr.Stages = append(pipelineExpr.Stages, &PipelineStage{
				Kind: NormalStage,
				Expr: e,
			})
			pipelineExpr.Span.End = p.i

			base := e.BasePtr()
			if isMissingExpr {
				return
			}
			if base.Err == nil {
				base.Err = &ParsingError{UnspecifiedParsingError, INVALID_PIPE_EXPR_STAGE_ALL_STAGES_SHOULD_BE_CALLS}
			}
		}

		p.eatSpace()

		if p.i >= p.len {
			return
		}

		switch p.s[p.i] {
		case '|':
			p.tokens = append(p.tokens, Token{Type: PIPE, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			continue //we parse the next stage
		default:
			if !IsCommentStart(p.s, p.i) && !isUnpairedOrIsClosingDelim(p.s[p.i]) {
				pipelineExpr.Err = &ParsingError{UnspecifiedParsingError, fmtInvalidPipelineStageUnexpectedChar(p.s[p.i])}
			}
			return
		}
	}

	return
}
