package parse

import (
	"encoding/json"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

func (p *parser) parseQuotedStringLiteral() *DoubleQuotedStringLiteral {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *ParsingError
	var value string
	var raw string

	p.i++

	for p.i < p.len && p.s[p.i] != '\n' && (p.s[p.i] != '"' || utils.CountPrevBackslashes(p.s, p.i)%2 == 1) {
		p.i++
	}

	isUnterminated := false

	if p.i >= p.len || (p.i < p.len && p.s[p.i] != '"') {
		raw = string(p.s[start:p.i])
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STRING_LIT}
		isUnterminated = true
	} else {
		p.i++

		raw = string(p.s[start:p.i])
		decoded, ok := DecodeJsonStringLiteral(utils.StringAsBytes(raw))
		if ok {
			value = decoded
		} else { //use json.Unmarshal to get the error
			err := json.Unmarshal(utils.StringAsBytes(raw), &decoded)
			parsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidStringLitJSON(err.Error())}
		}
	}

	return &DoubleQuotedStringLiteral{
		NodeBase: NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Raw:            raw,
		Value:          value,
		IsUnterminated: isUnterminated,
	}
}

func (p *parser) parseUnquotedStringLiteral(start int32) Node {
	p.panicIfContextDone()

	p.i++

	var parsingErr *ParsingError
	for p.i < p.len &&
		(isUnquotedStringChar(p.s[p.i]) || (p.s[p.i] == '\\' && p.i < p.len-1 && p.s[p.i+1] == ':')) {
		if p.s[p.i] == '\\' {
			p.i++
		} else if p.s[p.i] == '/' && p.i < p.len-1 && p.s[p.i+1] == '>' {
			break
		}
		p.i++
	}

	raw := string(p.s[start:p.i])
	value := strings.ReplaceAll(raw, "\\", "")

	base := NodeBase{
		Span: NodeSpan{start, p.i},
		Err:  parsingErr,
	}

	return &UnquotedStringLiteral{
		NodeBase: base,
		Raw:      raw,
		Value:    value,
	}
}

func (p *parser) getValueOfMultilineStringSliceOrLiteral(raw []byte, literal bool) (string, *ParsingError) {
	p.panicIfContextDone()

	if literal {
		raw[0] = '"'
		raw[len32(raw)-1] = '"'
	} else {
		raw = append([]byte{'"'}, raw...)
		raw = append(raw, '"')
	}

	marshalingInput := make([]byte, 0, len32(raw))
	for i, _byte := range raw {
		switch _byte {
		case '\n':
			marshalingInput = append(marshalingInput, '\\', 'n')
		case '\r':
			marshalingInput = append(marshalingInput, '\\', 'r')
		case '\t':
			marshalingInput = append(marshalingInput, '\\', 't')
		case '\\':
			if i < len(raw)-1 && raw[i+1] == '`' { //escaped backquote
				continue
			}
			marshalingInput = append(marshalingInput, '\\')
		case '"':
			if i != 0 && i < len(raw)-1 {
				marshalingInput = append(marshalingInput, '\\', '"')
			} else {
				marshalingInput = append(marshalingInput, '"')
			}
		default:
			marshalingInput = append(marshalingInput, _byte)
		}
	}
	decoded, ok := DecodeJsonStringLiteral(marshalingInput)
	if ok {
		return decoded, nil
	}

	//use json.Unmarshal to get the error
	err := json.Unmarshal(marshalingInput, &decoded)
	return "", &ParsingError{UnspecifiedParsingError, fmtInvalidStringLitJSON(err.Error())}

}

func (p *parser) parseStringTemplateLiteralOrMultilineStringLiteral(pattern Node) Node {
	p.panicIfContextDone()

	start := p.i
	if pattern != nil {
		start = pattern.Base().Span.Start
	}
	openingBackquoteIndex := p.i
	p.i++ // eat `

	inInterpolation := false
	interpolationStart := int32(-1)
	p.tokens = append(p.tokens, Token{Type: BACKQUOTE, Span: NodeSpan{p.i - 1, p.i}})
	slices := make([]Node, 0)
	sliceStart := p.i

	var parsingErr *ParsingError
	isMultilineStringLiteral := false

	for p.i < p.len && (p.s[p.i] != '`' || utils.CountPrevBackslashes(p.s, p.i)%2 == 1) {

		//interpolation
		if p.s[p.i] == '{' && p.s[p.i-1] == '$' {
			p.tokens = append(p.tokens, Token{Type: STR_INTERP_OPENING, Span: NodeSpan{p.i - 1, p.i + 1}})

			// add previous slice
			raw := string(p.s[sliceStart : p.i-1])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			slices = append(slices, &StringTemplateSlice{
				NodeBase: NodeBase{
					NodeSpan{sliceStart, p.i - 1},
					sliceErr,
					false,
				},
				Raw:   raw,
				Value: value,
			})

			inInterpolation = true
			p.i++
			interpolationStart = p.i
		} else if inInterpolation && p.s[p.i] == '}' { //end of interpolation
			p.tokens = append(p.tokens, Token{Type: STR_INTERP_CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			interpolationExclEnd := p.i
			inInterpolation = false
			p.i++
			sliceStart = p.i

			var interpParsingErr *ParsingError
			var typ string //typename or typename.method followed by ':'
			var expr Node  //expression inside the interpolation

			interpolation := p.s[interpolationStart:interpolationExclEnd]

			for j := int32(0); j < len32(interpolation); j++ {
				if !isInterpolationAllowedChar(interpolation[j]) {
					interpParsingErr = &ParsingError{UnspecifiedParsingError, STR_INTERP_LIMITED_CHARSET}
					break
				}
			}

			if interpParsingErr == nil {
				switch {
				case strings.TrimSpace(string(interpolation)) == "": //empty
					interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_STRING_INTERPOLATION_SHOULD_NOT_BE_EMPTY}
				case pattern != nil && !IsIdentChar(interpolation[0]): //not starting with a type name
					interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_STRING_INTERPOLATION_SHOULD_START_WITH_A_NAME}
				default:
					typ, expr, interpParsingErr = p.getStrTemplateInterTypeAndExpr(interpolation, interpolationStart, pattern)
				}
			}

			typeWithoutColon := ""
			if pattern != nil && len(typ) > 0 {
				typeWithoutColon = typ[:len(typ)-1]
				p.tokens = append(p.tokens, Token{
					Type: STR_TEMPLATE_INTERP_TYPE,
					Span: NodeSpan{interpolationStart,
						interpolationStart + int32(len(typ)),
					},
					Raw: typ,
				})
			}

			interpolationNode := &StringTemplateInterpolation{
				NodeBase: NodeBase{
					NodeSpan{interpolationStart, interpolationExclEnd},
					interpParsingErr,
					false,
				},
				Type: typeWithoutColon,
				Expr: expr,
			}
			slices = append(slices, interpolationNode)
		} else {
			p.i++
		}
	}

	if inInterpolation {
		raw := string(p.s[interpolationStart:p.i])
		value, _ := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		slices = append(slices, &StringTemplateSlice{
			NodeBase: NodeBase{
				NodeSpan{interpolationStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_INTERP},
				false,
			},
			Raw:   raw,
			Value: value,
		})
	} else {
		if len(slices) == 0 && pattern == nil { // multiline string literal
			isMultilineStringLiteral = true
			goto end
		}

		raw := string(p.s[sliceStart:p.i])
		value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		slices = append(slices, &StringTemplateSlice{
			NodeBase: NodeBase{
				NodeSpan{sliceStart, p.i},
				sliceErr,
				false,
			},
			Raw:   raw,
			Value: value,
		})
	}

end:
	if isMultilineStringLiteral {
		var value string
		var raw string
		isUnterminated := false

		if p.i >= p.len && (p.i == openingBackquoteIndex+1 || p.s[p.i-1] != '`') {
			raw = string(p.s[openingBackquoteIndex:])
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_MULTILINE_STRING_LIT}
			isUnterminated = true
		} else {
			p.tokens = append(p.tokens, Token{Type: BACKQUOTE, Span: NodeSpan{p.i, p.i + 1}})

			p.i++

			raw = string(p.s[openingBackquoteIndex:p.i])
			value, parsingErr = p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), true)
		}

		return &MultilineStringLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{openingBackquoteIndex, p.i},
				Err:  parsingErr,
			},
			Raw:            raw,
			Value:          value,
			IsUnterminated: isUnterminated,
		}
	}

	if p.i >= p.len {
		if !inInterpolation {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_TEMPL_LIT}
		}
	} else {
		p.tokens = append(p.tokens, Token{Type: BACKQUOTE, Span: NodeSpan{p.i, p.i + 1}})
		p.i++ // eat `
	}

	return &StringTemplateLiteral{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Pattern: pattern,
		Slices:  slices,
	}
}

func (p *parser) getStrTemplateInterTypeAndExpr(interpolation []rune, interpolationStart int32, pattern Node) (typename string, expr Node, err *ParsingError) {
	if pattern != nil { //typed interpolation
		i := int32(1)
		for ; i < len32(interpolation) && (interpolation[i] == '.' || IsIdentChar(interpolation[i])); i++ {
		}

		typename = string(interpolation[:i+1])

		if i >= len32(interpolation) || interpolation[i] != ':' || i >= len32(interpolation)-1 {
			err = &ParsingError{UnspecifiedParsingError, NAME_IN_STR_INTERP_SHOULD_BE_FOLLOWED_BY_COLON_AND_EXPR}
			return
		} else {
			i++
			exprStart := i + interpolationStart

			e, ok := ParseExpression(string(interpolation[i:]))
			if !ok {
				err = &ParsingError{UnspecifiedParsingError, INVALID_STR_INTERP}
				return
			} else {
				shiftNodeSpans(e, exprStart)
				expr = e
				return
			}
		}
	} else { //untyped interpolation
		e, ok := ParseExpression(string(interpolation))
		if !ok {
			err = &ParsingError{UnspecifiedParsingError, INVALID_STR_INTERP}
			return
		} else {
			shiftNodeSpans(e, interpolationStart)
			expr = e
			return
		}
	}
}
