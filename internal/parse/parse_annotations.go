package parse

import (
	"encoding/json"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

func (p *parser) parseMetadaAnnotationsBeforeStatement(
	statements *[]ast.Node,
	regionHeaders *[]*ast.AnnotatedRegionHeader,
) (annotations *ast.MetadataAnnotations, moveForward bool) {

	//Parse region headers.

	for p.i < p.len-1 && p.s[p.i] == '@' && p.s[p.i+1] == '\'' {
		text := p.parseAnnotatedRegionHeaderText()

		header := &ast.AnnotatedRegionHeader{
			NodeBase: ast.NodeBase{Span: text.Span},
			Text:     text,
		}

		p.eatSpace()

		var headerAnnotations []ast.Node
		start := p.i

		for p.i < p.len && p.s[p.i] == '@' {
			e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

			isAnnotation := isAnnotationExpression(e)

			if !isAnnotation {
				if e.Base().Err == nil {
					e.BasePtr().Err = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: INVALID_METADATA_ANNOTATION}
				}
			}

			headerAnnotations = append(headerAnnotations, e)

			p.eatSpace()
		}

		p.eatSpace()

		if headerAnnotations != nil {
			header.Annotations = &ast.MetadataAnnotations{
				NodeBase:    ast.NodeBase{Span: NodeSpan{Start: start, End: p.i}},
				Expressions: headerAnnotations,
			}
			header.Span.End = header.Annotations.Span.End
		}

		*regionHeaders = append(*regionHeaders, header)

		i := p.i
		p.eatSpaceNewlineSemicolonComment()
		if i < p.len && i == p.i { //Missing delimiter (no `\n`, `;`, nor comment)
			header.Err = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: MISSING_DELIMITER_AFTER_ANNOTATED_REGION_HEADER}
		}
	}

	if p.i >= p.len || p.s[p.i] == '}' {
		return
	}

	//Parse annotations.

	moveForward = true

	if p.i >= p.len-1 || p.s[p.i] != '@' || !isAlphaOrUndescore(p.s[p.i+1]) {
		return
	}

	var annotationList []ast.Node

	start := p.i

	for p.i < p.len && p.s[p.i] == '@' {
		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		isAnnotation := isAnnotationExpression(e)

		if !isAnnotation {
			if e.Base().Err == nil {
				e.BasePtr().Err = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: INVALID_METADATA_ANNOTATION}
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

			missingStmt := &ast.MissingStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, errMsg},
				},
				Annotations: &ast.MetadataAnnotations{
					NodeBase:    ast.NodeBase{Span: NodeSpan{Start: start, End: p.i}},
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

	annotations = &ast.MetadataAnnotations{
		NodeBase:    ast.NodeBase{Span: NodeSpan{start, p.i}},
		Expressions: annotationList,
	}

	return
}

func (p *parser) parseAnnotatedRegionHeaderText() *ast.AnnotatedRegionHeaderText {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *sourcecode.ParsingError
	var value string
	var raw string

	p.i += 2 //eat `@'`

	for p.i < p.len && p.s[p.i] != '\n' && (p.s[p.i] != '\'' || utils.CountPrevBackslashes(p.s, p.i)%2 == 1) {
		p.i++
	}

	if p.i >= p.len || (p.i < p.len && p.s[p.i] != '\'') {
		raw = string(p.s[start:p.i])
		parsingErr = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: UNTERMINATED_REGION_HEADER_TEXT}
	} else {
		p.i++

		raw = string(p.s[start:p.i])
		rawUnquotedText := raw[2 : len(raw)-1]

		decoded, ok := DecodeJsonStringBytesNoQuotes(utils.StringAsBytes(rawUnquotedText))
		if ok {
			value = string(decoded)
		} else { //use json.Unmarshal to get the error
			err := json.Unmarshal(utils.StringAsBytes(rawUnquotedText), &decoded)
			parsingErr = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: fmtInvalidStringLitJSON(err.Error())}
		}
	}

	return &ast.AnnotatedRegionHeaderText{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{Start: start, End: p.i},
			Err:  parsingErr,
		},
		Raw:   raw,
		Value: value,
	}
}

func (p *parser) tryParseMetadaAnnotationsAfterProperty() *ast.MetadataAnnotations {

	if p.i >= p.len-1 || p.s[p.i] != '@' || !isAlphaOrUndescore(p.s[p.i+1]) {
		return nil
	}

	var annotationList []ast.Node

	start := p.i

	for p.i < p.len && p.s[p.i] == '@' {
		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		isAnnotation := isAnnotationExpression(e)

		if !isAnnotation {
			if e.Base().Err == nil {
				e.BasePtr().Err = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: INVALID_METADATA_ANNOTATION}
			}
		}

		annotationList = append(annotationList, e)

		p.eatSpaceComments()

		linefeedCount := p.eatSpaceNewline()
		if linefeedCount > 1 || p.i >= p.len || p.eatComment() || p.s[p.i] == '}' || p.s[p.i] == ',' {
			break
		}
	}

	return &ast.MetadataAnnotations{
		NodeBase:    ast.NodeBase{Span: NodeSpan{Start: start, End: p.i}},
		Expressions: annotationList,
	}
}

func (p *parser) parseAnnotatedRegionHeadersInMarkup(
	regionHeaders *[]*ast.AnnotatedRegionHeader,
) {

	var lastHeader *ast.AnnotatedRegionHeader

	for p.i < p.len-1 && p.s[p.i] == '@' && p.s[p.i+1] == '\'' {
		text := p.parseAnnotatedRegionHeaderText()

		header := &ast.AnnotatedRegionHeader{
			NodeBase: ast.NodeBase{Span: text.Span},
			Text:     text,
		}
		lastHeader = header

		p.eatSpace()

		var headerAnnotations []ast.Node
		start := p.i

		for p.i < p.len && p.s[p.i] == '@' {
			e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

			isAnnotation := isAnnotationExpression(e)

			if !isAnnotation {
				if e.Base().Err == nil {
					e.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_METADATA_ANNOTATION}
				}
			}

			headerAnnotations = append(headerAnnotations, e)

			p.eatSpace()
		}

		p.eatSpace()

		if headerAnnotations != nil {
			header.Annotations = &ast.MetadataAnnotations{
				NodeBase:    ast.NodeBase{Span: NodeSpan{start, p.i}},
				Expressions: headerAnnotations,
			}
			header.Span.End = header.Annotations.Span.End
		}

		*regionHeaders = append(*regionHeaders, header)
	}

	p.eatSpace()

	if p.i < p.len && p.s[p.i] != '\n' && lastHeader != nil {
		lastHeader.Err = &sourcecode.ParsingError{UnspecifiedParsingError, MISSING_LINEFEED_AFTER_ANNOTATED_REGION_HEADER}
	}

	return
}

func isAnnotationExpression(e ast.Node) bool {
	switch e := e.(type) {
	case *ast.MetaIdentifier:
		return true
	case *ast.CallExpression:
		return e.IsMetaCallee()
	default:
		return false
	}
}

// addAnnotationsToNodeIfPossible adds $annotations to $node if it supports them, a non-nil *ast.MissingStatement is returned otherwise.
// If $annotations is nil addAnnotationsToNodeIfPossible dos nothing and returns nil.
func (p *parser) addAnnotationsToNodeIfPossible(annotations *ast.MetadataAnnotations, stmt ast.Node) *ast.MissingStatement {

	if annotations == nil {
		return nil
	}

	switch stmt := stmt.(type) {
	case *ast.FunctionDeclaration:
		stmt.Annotations = annotations
		stmt.Span.Start = annotations.Span.Start
	default:
		return &ast.MissingStatement{
			NodeBase: ast.NodeBase{
				Span: annotations.Span,
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, METADATA_ANNOTATIONS_SHOULD_BE_FOLLOWED_BY_STMT_SUPPORTING_THEM},
			},
			Annotations: annotations,
		}
	}

	return nil
}
