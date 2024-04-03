package parse

func (p *parser) parseMetadaAnnotationsBeforeStatement(statements *[]Node) (annotations *MetadataAnnotations, moveForward bool) {

	moveForward = true

	if p.i >= p.len-1 || p.s[p.i] != '@' || !isAlphaOrUndescore(p.s[p.i+1]) {
		return
	}

	var annotationList []Node

	start := p.i

	for p.i < p.len && p.s[p.i] == '@' {
		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

		isAnnotation := isAnnotationExpression(e)

		if !isAnnotation {
			if e.Base().Err == nil {
				e.BasePtr().Err = &ParsingError{UnspecifiedParsingError, INVALID_METADATA_ANNOTATION}
			}
		}

		annotationList = append(annotationList, e)

		p.eatSpaceComments()
		linefeedCount := p.eatSpaceNewline()
		if linefeedCount > 1 || p.i >= p.len || p.eatComment() || p.s[p.i] == '}' {

			errMsg := MISSING_STMT_AFTER_ANNOTATIONS

			if len(annotationList) == 1 {
				errMsg = MISSING_STMT_AFTER_ANNOTATIONS_EXPR_EXPLANATION
			}

			missingStmt := &MissingStatement{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, errMsg},
				},
				Annotations: &MetadataAnnotations{
					NodeBase:    NodeBase{Span: NodeSpan{start, p.i}},
					Expressions: annotationList,
				},
			}
			*statements = append(*statements, missingStmt)

			p.eatSpaceNewlineComment()

			if p.i >= p.len || p.s[p.i] == '}' {
				moveForward = false
				return
			}
		}
	}

	annotations = &MetadataAnnotations{
		NodeBase:    NodeBase{Span: NodeSpan{start, p.i}},
		Expressions: annotationList,
	}

	return
}

func (p *parser) tryParseMetadaAnnotationsAfterProperty() *MetadataAnnotations {

	if p.i >= p.len-1 || p.s[p.i] != '@' || !isAlphaOrUndescore(p.s[p.i+1]) {
		return nil
	}

	var annotationList []Node

	start := p.i

	for p.i < p.len && p.s[p.i] == '@' {
		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinExpr: true})

		isAnnotation := isAnnotationExpression(e)

		if !isAnnotation {
			if e.Base().Err == nil {
				e.BasePtr().Err = &ParsingError{UnspecifiedParsingError, INVALID_METADATA_ANNOTATION}
			}
		}

		annotationList = append(annotationList, e)

		p.eatSpaceComments()

		linefeedCount := p.eatSpaceNewline()
		if linefeedCount > 1 || p.i >= p.len || p.eatComment() || p.s[p.i] == '}' || p.s[p.i] == ',' {
			break
		}
	}

	return &MetadataAnnotations{
		NodeBase:    NodeBase{Span: NodeSpan{start, p.i}},
		Expressions: annotationList,
	}
}

func isAnnotationExpression(e Node) bool {
	switch e := e.(type) {
	case *MetaIdentifier:
		return true
	case *CallExpression:
		return e.IsMetaCallee()
	default:
		return false
	}
}

// addAnnotationsToNodeIfPossible adds $annotations to $node if it supports them, a non-nil *MissingStatement is returned otherwise.
// If $annotations is nil addAnnotationsToNodeIfPossible dos nothing and returns nil.
func (p *parser) addAnnotationsToNodeIfPossible(annotations *MetadataAnnotations, stmt Node) *MissingStatement {

	if annotations == nil {
		return nil
	}

	switch stmt := stmt.(type) {
	case *FunctionDeclaration:
		stmt.Annotations = annotations
		stmt.Span.Start = annotations.Span.Start
	default:
		return &MissingStatement{
			NodeBase: NodeBase{
				Span: annotations.Span,
				Err:  &ParsingError{UnspecifiedParsingError, METADATA_ANNOTATIONS_SHOULD_BE_FOLLOWED_BY_STMT_SUPPORTING_THEM},
			},
			Annotations: annotations,
		}
	}

	return nil
}
