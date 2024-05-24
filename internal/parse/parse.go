package parse

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/exp/slices"
)

const (
	MAX_MODULE_BYTE_LEN     = 1 << 24
	MAX_OBJECT_KEY_BYTE_LEN = 64

	DEFAULT_TIMEOUT       = 20 * time.Millisecond
	DEFAULT_NO_CHECK_FUEL = 10

	//date-like

	NO_LOCATION_DATELIKE_LITERAL_PATTERN  = "^(\\d+y)(?:|(-\\d{1,2}mt)(-\\d{1,2}d)(-\\d{1,2}h)?(-\\d{1,2}m)?(-\\d{1,2}s)?(-\\d{1,3}ms)?(-\\d{1,3}us)?)"
	_NO_LOCATION_DATELIKE_LITERAL_PATTERN = NO_LOCATION_DATELIKE_LITERAL_PATTERN + "$"
	DATELIKE_LITERAL_PATTERN              = NO_LOCATION_DATELIKE_LITERAL_PATTERN + "(-[a-zA-Z_/]+[a-zA-Z_])$"
)

var (
	ErrUnreachable = errors.New("unreachable")

	KEYWORDS                     = ast.TOKEN_STRINGS[ast.IF_KEYWORD : ast.OR_KEYWORD+1]
	MANIFEST_KEYWORD_STR         = ast.TOKEN_STRINGS[ast.MANIFEST_KEYWORD]
	INCLUDABLE_CHUNK_KEYWORD_STR = ast.TOKEN_STRINGS[ast.INCLUDABLE_FILE_KEYWORD]
	CONST_KEYWORD_STR            = ast.TOKEN_STRINGS[ast.CONST_KEYWORD]
	READONLY_KEYWORD_STR         = ast.TOKEN_STRINGS[ast.READONLY_KEYWORD]

	//date regexes

	NO_LOCATION_DATELIKE_LITERAL_REGEX = regexp.MustCompile(_NO_LOCATION_DATELIKE_LITERAL_PATTERN)
	DATELIKE_LITERAL_REGEX             = regexp.MustCompile(DATELIKE_LITERAL_PATTERN)

	//other regexes

	ContainsSpace = regexp.MustCompile(`\s`).MatchString
)

func (p *parser) isExpressionEnd() bool {
	return p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i])
}

func (p *parser) parseCssSelectorElement(ignoreNextSpace bool) (node ast.Node, isSpace bool) {
	p.panicIfContextDone()

	start := p.i
	switch p.s[p.i] {
	case '>', '~', '+':
		name := string(p.s[p.i])
		p.i++
		return &ast.CssCombinator{
			NodeBase: ast.NodeBase{
				NodeSpan{p.i - 1, p.i},
				nil,
				false,
			},
			Name: name,
		}, false
	case '.':
		p.i++
		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			return &ast.CssClassSelector{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_CSS_CLASS_SELECTOR_NAME_EXPECTED},
					false,
				},
			}, false
		}

		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		return &ast.CssClassSelector{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Name: string(p.s[start+1 : p.i]),
		}, false
	case '#': // id
		p.i++
		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			return &ast.CssIdSelector{
				NodeBase: ast.NodeBase{
					Span:            NodeSpan{start, p.i},
					Err:             &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_CSS_ID_SELECTOR_NAME_EXPECTED},
					IsParenthesized: false,
				},
			}, false
		}

		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		return &ast.CssIdSelector{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Name: string(p.s[start+1 : p.i]),
		}, false
	case '[': //atribute selector
		p.i++

		makeNode := func(err string) ast.Node {
			return &ast.CssAttributeSelector{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i - 1, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, err},
					false,
				},
			}
		}

		if p.i >= p.len {
			return makeNode(UNTERMINATED_CSS_ATTR_SELECTOR_NAME_EXPECTED), false
		}

		if !isAlpha(p.s[p.i]) {
			return makeNode(CSS_ATTRIBUTE_NAME_SHOULD_START_WITH_ALPHA_CHAR), false
		}

		name := p.parseIdentStartingExpression(false)

		if p.i >= p.len {
			return makeNode(UNTERMINATED_CSS_ATTR_SELECTOR_PATTERN_EXPECTED_AFTER_NAME), false
		}

		var pattern string

		switch p.s[p.i] {
		case '~', '*', '^', '|', '$':
			p.i++
			if p.i >= p.len {
				return makeNode(UNTERMINATED_CSS_ATTR_SELECTOR_INVALID_PATTERN), false
			}
			if p.s[p.i] != '=' {
				return makeNode(UNTERMINATED_CSS_ATTR_SELECTOR_INVALID_PATTERN), false
			}
			p.i++
			pattern = string(p.s[p.i-2 : p.i])

		case '=':
			pattern = string(p.s[p.i])
			p.i++
		default:
			return makeNode(UNTERMINATED_CSS_ATTR_SELECTOR_INVALID_PATTERN), false
		}

		value, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		if p.i >= p.len || p.s[p.i] != ']' {
			return makeNode(UNTERMINATED_CSS_ATTRIBUTE_SELECTOR_MISSING_BRACKET), false
		}
		p.i++

		return &ast.CssAttributeSelector{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			AttributeName: name.(*ast.IdentifierLiteral),
			Pattern:       pattern,
			Value:         value,
		}, false

	case ':':
		p.i++
		makeErr := func(err string) *sourcecode.ParsingError {
			return &sourcecode.ParsingError{UnspecifiedParsingError, err}

		}
		if p.i >= p.len {
			return &ast.InvalidCSSselectorNode{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					makeErr(INVALID_CSS_SELECTOR),
					false,
				},
			}, false
		}

		if p.s[p.i] != ':' { //pseudo class
			nameStart := p.i
			p.i++

			if p.i >= p.len || !isAlpha(p.s[p.i]) {
				return &ast.CssPseudoClassSelector{
					NodeBase: ast.NodeBase{
						NodeSpan{start, p.i},
						makeErr(INVALID_CSS_CLASS_SELECTOR_INVALID_NAME),
						false,
					},
				}, false
			}

			p.i++
			for p.i < p.len && IsIdentChar(p.s[p.i]) {
				p.i++
			}

			nameEnd := p.i

			return &ast.CssPseudoClassSelector{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					nil,
					false,
				},
				Name: string(p.s[nameStart:nameEnd]),
			}, false
		}

		p.i++

		//pseudo element
		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			return &ast.CssPseudoElementSelector{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					makeErr(INVALID_PSEUDO_CSS_SELECTOR_INVALID_NAME),
					false,
				},
			}, false
		}

		nameStart := p.i

		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		nameEnd := p.i

		return &ast.CssPseudoElementSelector{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Name: string(p.s[nameStart:nameEnd]),
		}, false
	case ' ':
		p.i++
		p.eatSpace()
		if p.i >= p.len || isNonSpaceCSSCombinator(p.s[p.i]) || ignoreNextSpace {
			return nil, true
		}

		return &ast.CssCombinator{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Name: " ",
		}, false
	case '*':
		p.i++
		return &ast.CssTypeSelector{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Name: "*",
		}, false
	}

	if p.i < p.len && isAlpha(p.s[p.i]) {
		p.i++
		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		return &ast.CssTypeSelector{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				nil,
				false,
			},
			Name: string(p.s[start:p.i]),
		}, false
	}

	return &ast.InvalidCSSselectorNode{
		NodeBase: ast.NodeBase{
			NodeSpan{start - 1, p.i},
			&sourcecode.ParsingError{UnspecifiedParsingError, EMPTY_CSS_SELECTOR},
			false,
		},
	}, false

}

func (p *parser) parseTopCssSelector(start int32) ast.Node {
	p.panicIfContextDone()

	//p.s!
	p.tokens = append(p.tokens, ast.Token{Type: ast.CSS_SELECTOR_PREFIX, Span: NodeSpan{start, p.i}})

	if p.i >= p.len {
		return &ast.InvalidCSSselectorNode{
			NodeBase: ast.NodeBase{
				NodeSpan{p.i - 1, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, EMPTY_CSS_SELECTOR},
				false,
			},
		}
	}

	var elements []ast.Node
	var ignoreNextSpace bool

	for p.i < p.len && p.s[p.i] != '\n' {
		if p.s[p.i] == '!' {
			p.i++
			break
		}
		e, isSpace := p.parseCssSelectorElement(ignoreNextSpace)

		if !isSpace {
			elements = append(elements, e)
			_, ignoreNextSpace = e.(*ast.CssCombinator)

			if e.Base().Err != nil {
				p.i++
			}
		} else {
			ignoreNextSpace = false
		}
	}

	return &ast.CssSelectorExpression{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			nil,
			false,
		},
		Elements: elements,
	}
}

func (p *parser) parseBlock() *ast.Block {
	p.panicIfContextDone()

	openingBraceIndex := p.i
	prevStmtEndIndex := int32(-1)
	var prevStmtErrKind string

	p.i++

	p.tokens = append(p.tokens, ast.Token{
		Type:    ast.OPENING_CURLY_BRACKET,
		SubType: ast.BLOCK_OPENING_BRACE,
		Span:    NodeSpan{openingBraceIndex, openingBraceIndex + 1},
	})

	var (
		parsingErr    *sourcecode.ParsingError
		stmts         []ast.Node
		regionHeaders []*ast.AnnotatedRegionHeader
	)

	p.eatSpaceNewlineSemicolonComment()

	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {

			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})

			stmts = append(stmts, &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, p.i + 1},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i])},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment()
			continue
		}

		var stmtErr *sourcecode.ParsingError

		if p.i >= p.len || p.s[p.i] == '}' {
			break
		}

		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &sourcecode.ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		annotations, moveForward := p.parseMetadaAnnotationsBeforeStatement(&stmts, &regionHeaders)
		if !moveForward {
			break
		}

		stmt := p.parseStatement()

		prevStmtEndIndex = p.i

		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.Kind
		}

		if stmtErr != nil && (stmt.Base().Err == nil || stmt.Base().Err.Kind != InvalidNext) {
			stmt.BasePtr().Err = stmtErr
		}

		if missingStmt := p.addAnnotationsToNodeIfPossible(annotations, stmt); missingStmt != nil {
			stmts = append(stmts, missingStmt)
		}

		stmts = append(stmts, stmt)

		p.eatSpaceNewlineSemicolonComment()
	}

	closingBraceIndex := p.i

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{
			Type:    ast.CLOSING_CURLY_BRACKET,
			SubType: ast.BLOCK_CLOSING_BRACE,
			Span:    NodeSpan{closingBraceIndex, closingBraceIndex + 1},
		})
		p.i++
	} else {
		parsingErr = &sourcecode.ParsingError{UnterminatedBlock, UNTERMINATED_BLOCK_MISSING_BRACE}
	}

	end := p.i

	return &ast.Block{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{openingBraceIndex, end},
			Err:  parsingErr,
		},
		RegionHeaders: regionHeaders,
		Statements:    stmts,
	}
}

// parsePathExpressionSlices parses the slices in a path expression.
// example: /{$HOME}/.cache -> [ / , $HOME , /.cache ]
func (p *parser) parsePathExpressionSlices(start int32, exclEnd int32) []ast.Node {
	p.panicIfContextDone()

	slices := make([]ast.Node, 0)
	index := start
	sliceStart := start
	inInterpolation := false

	for index < exclEnd {
		switch {
		//start of a new interpolation:
		case !inInterpolation && p.s[index] == '{':
			slice := string(p.s[sliceStart:index]) //previous cannot be an interpolation

			p.tokens = append(p.tokens, ast.Token{
				Type:    ast.OPENING_CURLY_BRACKET,
				SubType: ast.PATH_INTERP_OPENING_BRACE,
				Span:    NodeSpan{index, index + 1},
			})

			slices = append(slices, &ast.PathSlice{
				NodeBase: ast.NodeBase{
					NodeSpan{sliceStart, index},
					nil,
					false,
				},
				Value: slice,
			})

			sliceStart = index + 1
			inInterpolation = true

			//if the interpolation is unterminated
			if index == p.len-1 {
				slices = append(slices, &ast.PathSlice{
					NodeBase: ast.NodeBase{
						NodeSpan{sliceStart, sliceStart},
						&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
						false,
					},
					Value: string(p.s[sliceStart:sliceStart]),
				})

				return slices
			}
			index++
		//end of interpolation
		case inInterpolation && (p.s[index] == '}' || index == exclEnd-1):
			missingClosingBrace := false
			inInterpolation = false

			if index == exclEnd-1 && p.s[index] != '}' {
				index++
				missingClosingBrace = true
			} else {
				p.tokens = append(p.tokens, ast.Token{
					Type:    ast.CLOSING_CURLY_BRACKET,
					SubType: ast.PATH_INTERP_CLOSING_BRACE,
					Span:    NodeSpan{index, index + 1},
				})
			}

			interpolation := string(p.s[sliceStart:index])

			if interpolation != "" && interpolation[0] == ':' { //named segment
				slices = append(slices, p.parseNamedPatternSegment(interpolation, sliceStart, index))
				sliceStart = index + 1
				index++
				continue
			}

			//Regular interpolation

			expr, ok := ParseExpression(interpolation)

			if !ok {
				span := NodeSpan{sliceStart, index}
				err := &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_PATH_INTERP}

				if len(interpolation) == 0 {
					err.Message = EMPTY_PATH_INTERP
				}

				p.tokens = append(p.tokens, ast.Token{Type: ast.INVALID_INTERP_SLICE, Span: span, Raw: string(p.s[sliceStart:index])})
				slices = append(slices, &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						span,
						err,
						false,
					},
				})

			} else {
				ast.ShiftNodeSpans(expr, sliceStart)
				slices = append(slices, expr)

				if missingClosingBrace {
					slices = append(slices, &ast.PathSlice{
						NodeBase: ast.NodeBase{
							NodeSpan{index, index},
							&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP_MISSING_CLOSING_BRACE},
							false,
						},
					})
				}
			}

			sliceStart = index + 1
			index++
		//forbidden character
		case inInterpolation && !isInterpolationAllowedChar(p.s[index]):
			// we eat all the interpolation

			j := index
			for j < exclEnd && p.s[j] != '}' {
				j++
			}

			p.tokens = append(p.tokens, ast.Token{Type: ast.INVALID_INTERP_SLICE, Span: NodeSpan{sliceStart, j}, Raw: string(p.s[sliceStart:j])})

			slices = append(slices, &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					NodeSpan{sliceStart, j},
					&sourcecode.ParsingError{UnspecifiedParsingError, PATH_INTERP_EXPLANATION},
					false,
				},
			})

			if j < exclEnd { // '}'
				p.tokens = append(p.tokens, ast.Token{
					Type:    ast.CLOSING_CURLY_BRACKET,
					SubType: ast.PATH_INTERP_CLOSING_BRACE,
					Span:    NodeSpan{j, j + 1},
				})
				j++
			}

			inInterpolation = false
			sliceStart = j
			index = j
			continue
		default:
			index++
		}
	}

	if inInterpolation {
		slices = append(slices, &ast.PathSlice{
			NodeBase: ast.NodeBase{
				NodeSpan{sliceStart, index},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
				false,
			},
		})
	} else if sliceStart != index {
		slices = append(slices, &ast.PathSlice{
			NodeBase: ast.NodeBase{
				NodeSpan{sliceStart, index},
				nil,
				false,
			},
			Value: string(p.s[sliceStart:index]),
		})
	}
	return slices
}

func (p *parser) parseDotStartingExpression() ast.Node {
	p.panicIfContextDone()

	if p.i < p.len-1 {
		if p.s[p.i+1] == '/' || p.i < p.len-2 && p.s[p.i+1] == '.' && p.s[p.i+2] == '/' {
			return p.parsePathLikeExpression(false)
		}
		switch p.s[p.i+1] {
		case '{':
			return p.parseKeyList()
		case '.': //upper bound range expression.
			start := p.i
			p.i += 2

			p.tokens = append(p.tokens, ast.Token{Type: ast.TWO_DOTS, Span: NodeSpan{start, start + 2}})

			var err *sourcecode.ParsingError
			if p.i < p.len && p.s[p.i] == '.' {
				err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_UPPER_BOUND_RANGE_EXPR}
			}

			upperBound, _ := p.parseExpression()
			expr := &ast.UpperBoundRangeExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					err,
					false,
				},
				UpperBound: upperBound,
			}

			return expr
		default:
			r := p.s[p.i+1]
			if IsIdentChar(r) && !isDecDigit(r) {
				return p.parseValuePathLiteral()
			}
		}
	}

	p.i++
	p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i - 1, p.i}, Raw: "."})
	return &ast.UnknownNode{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{p.i - 1, p.i},
			Err:  &sourcecode.ParsingError{UnspecifiedParsingError, DOT_SHOULD_BE_FOLLOWED_BY},
		},
	}
}

// parseDashStartingExpression parses all expressions that start with a dash: numbers, numbers ranges, options, unquoted strings
// and number negations (unary expressions).
func (p *parser) parseDashStartingExpression(precededByOpeningParen bool) ast.Node {
	p.panicIfContextDone()

	__start := p.i

	p.i++
	if p.i >= p.len || isEndOfLine(p.s, p.i) {
		return &ast.UnquotedStringLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Raw:   "-",
			Value: "-",
		}
	}

	if isDecDigit(p.s[p.i]) {
		p.i--
		return p.parseNumberAndRangeAndRateLiterals()
	}

	if p.s[p.i] != '-' && (isSpaceNotLF(p.s[p.i]) || precededByOpeningParen || p.s[p.i] == '$' || p.s[p.i] == '(') {
		p.eatSpace()

		if precededByOpeningParen || p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
			return &ast.UnquotedStringLiteral{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{__start, __start + 1},
				},
				Raw:   "-",
				Value: "-",
			}
		}

		operand, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		p.tokens = append(p.tokens, ast.Token{Type: ast.MINUS, Span: NodeSpan{__start, __start + 1}})
		return &ast.UnaryExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Operator: ast.NumberNegate,
			Operand:  operand,
		}
	}

	singleDash := true

	if p.s[p.i] == '-' {
		singleDash = false
		p.i++
	}

	if p.i >= p.len || unicode.IsSpace(p.s[p.i]) {
		return &ast.UnquotedStringLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Raw:   "--",
			Value: string(p.s[__start:p.i]),
		}
	}

	nameStart := p.i

	if p.i >= p.len || IsDelim(p.s[p.i]) {
		return &ast.UnquotedStringLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Raw:   string(p.s[__start:p.i]),
			Value: string(p.s[__start:p.i]),
		}
	}

	if !isAlpha(p.s[p.i]) && !isDecDigit(p.s[p.i]) {
		if unicode.IsSpace(p.s[p.i]) || isValidUnquotedStringChar(p.s, p.i) {
			return p.parseUnquotedStringLiteral(__start)
		}
		return &ast.FlagLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{__start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, OPTION_NAME_CAN_ONLY_CONTAIN_ALPHANUM_CHARS},
			},
			SingleDash: singleDash,
			Raw:        string(p.s[__start:p.i]),
		}
	}

	for p.i < p.len && (isAlpha(p.s[p.i]) || isDecDigit(p.s[p.i]) || p.s[p.i] == '-') {
		p.i++
	}

	name := string(p.s[nameStart:p.i])

	if p.i >= p.len || p.s[p.i] != '=' {

		return &ast.FlagLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Name:       name,
			SingleDash: singleDash,
			Raw:        string(p.s[__start:p.i]),
		}
	}

	if p.inPattern {
		return p.parseOptionPatternLiteral(__start, name, singleDash)
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.FLAG_EQUAL, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	if p.i >= p.len {
		return &ast.OptionExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{__start, p.i},
				Err:  &sourcecode.ParsingError{UnterminatedOptionExpr, UNTERMINATED_OPTION_EXPR_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR},
			},
			Name:       name,
			SingleDash: singleDash,
		}
	}

	value, _ := p.parseExpression(exprParsingConfig{
		disallowUnparenthesizedBinForPipelineExprs: true,
	})

	return &ast.OptionExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{__start, p.i},
		},
		Name:       name,
		Value:      value,
		SingleDash: singleDash,
	}
}

// parsePathLikeExpression parses paths, path expressions, simple path patterns and named segment path patterns
func (p *parser) parsePathLikeExpression(percentPrefixed bool) ast.Node {
	p.panicIfContextDone()

	start := p.i
	if percentPrefixed {
		p.i++
	}

	isPattern := percentPrefixed || p.inPattern
	isUnprefixedPattern := p.inPattern && !percentPrefixed

	pathStart := p.i
	isAbsolute := p.s[p.i] == '/'
	p.i++

	if !isAbsolute {
		for p.i < p.len && p.s[p.i] == '.' {
			p.i++
		}
		for p.i < p.len && p.s[p.i] == '/' {
			p.i++
		}
	}

	isQuoted := p.i < p.len && p.s[p.i] == '`'

	if isQuoted {
		p.i++
		for p.i < p.len && p.s[p.i] != '\n' && p.s[p.i] != '`' {
			//no escape
			p.i++
		}
		if p.i < p.len && p.s[p.i] == '`' {
			p.i++
		}
	} else {
		// limit to ascii ? limit to ascii alphanum & some chars ?
	loop:
		for p.i < p.len && p.s[p.i] != '\n' && !unicode.IsSpace(p.s[p.i]) && (!IsDelim(p.s[p.i]) || p.s[p.i] == '{' || p.s[p.i] == ':') {

			//TODO: fix

			switch p.s[p.i] {
			case '{':
				p.i++
				for p.i < p.len && p.s[p.i] != '\n' && p.s[p.i] != '}' { //ok since '}' is not allowed in interpolations for now
					p.i++
				}
				if p.i < p.len && p.s[p.i] == '}' {
					p.i++
				}
			case ':':
				//Break the loop if ':' is trailing.
				if p.i == p.len-1 {
					break loop
				}
				nexChar := p.s[p.i+1]
				if unicode.IsSpace(nexChar) || (IsDelim(nexChar) && nexChar != '{') {
					break loop
				}
				p.i++
			default:
				p.i++
			}
		}
	}

	runes := p.s[start:p.i]
	raw := string(runes)

	_path := p.s[pathStart:p.i]
	missingClosingBacktick := isQuoted && len(_path) != 0 && _path[len(_path)-1] != '`'

	var clean []rune
	for _, r := range _path {
		if r == '`' {
			continue
		}
		clean = append(clean, r)
	}
	value := string(clean)

	base := ast.NodeBase{
		Span: NodeSpan{start, p.i},
	}

	slices := p.parsePathExpressionSlices(pathStart, p.i)
	hasInterpolationsOrNamedSegments := len32(slices) > 1
	hasGlobWildcard := false

search_for_glob_wildcard:
	for _, slice := range slices {
		if pathSlice, ok := slice.(*ast.PathSlice); ok {

			for i, e := range pathSlice.Value {
				if (e == '[' || e == '*' || e == '?') && utils.CountPrevBackslashes(p.s, start+int32(i))%2 == 0 {
					hasGlobWildcard = true
					break search_for_glob_wildcard
				}
			}
		}
	}

	isPrefixPattern := isPattern && strings.Contains(value, "/...")

	if isPrefixPattern && (!strings.HasSuffix(value, "/...") || strings.Contains(strings.TrimSuffix(value, "/..."), "/...")) {
		base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtSlashDotDotDotCanOnlyBePresentAtEndOfPathPattern(value)}
	}

	if !isPattern && isPrefixPattern && hasGlobWildcard {
		base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtPrefixPattCannotContainGlobbingPattern(value)}
		return &ast.InvalidPathPattern{
			NodeBase: base,
			Value:    value,
		}
	}

	if isPattern {

		if !hasInterpolationsOrNamedSegments {
			if missingClosingBacktick && base.Err == nil {
				base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_PATH_PATTERN_LIT_MISSING_CLOSING_BACTICK}
			}

			if isAbsolute {
				return &ast.AbsolutePathPatternLiteral{
					NodeBase:   base,
					Raw:        raw,
					Value:      value,
					Unprefixed: isUnprefixedPattern,
				}
			}
			return &ast.RelativePathPatternLiteral{
				NodeBase:   base,
				Raw:        raw,
				Value:      value,
				Unprefixed: isUnprefixedPattern,
			}
		}

		p.tokens = append(p.tokens, ast.Token{Type: ast.PERCENT_SYMBOL, Span: NodeSpan{start, start + 1}})

		//named segment path pattern literal & path pattern expressions
		containNamedSegments := false
		containInterpolations := false

		//search for named segments & interpolations + turn path slices into path pattern slices
		for i, e := range slices {

			switch E := e.(type) {
			case *ast.NamedPathSegment:
				containNamedSegments = true
			case *ast.PathSlice:
				slices[i] = &ast.PathPatternSlice{
					NodeBase: E.NodeBase,
					Value:    E.Value,
				}
			default:
				containInterpolations = true
			}

			if containNamedSegments && containInterpolations {
				base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, CANNOT_MIX_PATH_INTER_PATH_NAMED_SEGMENT}
				return &ast.NamedSegmentPathPatternLiteral{
					NodeBase: base,
					Raw:      raw,
					Slices:   slices,
				}
			}
		}

		if containNamedSegments { //named segment path pattern
			return p.newNamedSegmentPathPatternLiteral(base, isQuoted, slices, raw, value)
		} else {

			if isQuoted {
				base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, QUOTED_PATH_PATTERN_EXPRS_ARE_NOT_SUPPORTED_YET}
			}

			return &ast.PathPatternExpression{
				NodeBase: base,
				Slices:   slices,
			}
		}

	}

	for _, e := range slices {
		switch e.(type) {
		case *ast.NamedPathSegment:
			if base.Err == nil {
				base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, ONLY_PATH_PATTERNS_CAN_CONTAIN_NAMED_SEGMENTS}
			}
		}
	}

	if hasInterpolationsOrNamedSegments {
		if missingClosingBacktick && base.Err == nil {
			base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_PATH_EXPR_MISSING_CLOSING_BACTICK}
		}

		if isAbsolute {
			return &ast.AbsolutePathExpression{
				NodeBase: base,
				Slices:   slices,
			}
		}
		return &ast.RelativePathExpression{
			NodeBase: base,
			Slices:   slices,
		}
	}

	if missingClosingBacktick && base.Err == nil {
		base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_PATH_LIT_MISSING_CLOSING_BACTICK}
	}

	if isAbsolute {
		return &ast.AbsolutePathLiteral{
			NodeBase: base,
			Raw:      raw,
			Value:    value,
		}
	}
	return &ast.RelativePathLiteral{
		NodeBase: base,
		Raw:      raw,
		Value:    value,
	}

}

func (p *parser) newNamedSegmentPathPatternLiteral(base ast.NodeBase, isQuoted bool, slices []ast.Node, raw, value string) *ast.NamedSegmentPathPatternLiteral {
	for j := 0; j < len(slices); j++ {
		_, isNamedSegment := slices[j].(*ast.NamedPathSegment)

		if isNamedSegment {

			prev := slices[j-1].(*ast.PathPatternSlice).Value
			if prev[int32(len(prev))-1] != '/' {

				base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_PATH_PATT_NAMED_SEGMENTS}

				return &ast.NamedSegmentPathPatternLiteral{
					NodeBase: base,
					Slices:   slices,
				}
			}
			if j < len(slices)-1 {
				next := slices[j+1].(*ast.PathPatternSlice).Value

				if next[0] != '/' {
					if isQuoted {
						base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, QUOTED_NAMED_SEGMENT_PATH_PATTERNS_ARE_NOT_SUPPORTED_YET}
					} else {
						base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_PATH_PATT_NAMED_SEGMENTS}
					}

					return &ast.NamedSegmentPathPatternLiteral{
						NodeBase: base,
						Slices:   slices,
					}
				}
			}
		}
	}

	if isQuoted {
		base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, QUOTED_NAMED_SEGMENT_PATH_PATTERNS_ARE_NOT_SUPPORTED_YET}
	}

	return &ast.NamedSegmentPathPatternLiteral{
		NodeBase:    base,
		Slices:      slices,
		Raw:         raw,
		StringValue: "%" + value,
	}
}

// parseIdentStartingExpression parses identifiers, identifier member expressions, true, false, nil and URL-like expressions
func (p *parser) parseIdentStartingExpression(allowUnprefixedPatternNamespaceIdent bool) ast.Node {
	p.panicIfContextDone()

	start := p.i
	p.i++
	for p.i < p.len && IsIdentChar(p.s[p.i]) {
		p.i++
	}

	name := string(p.s[start:p.i])
	firstIdent := &ast.IdentifierLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
		},
		Name: name,
	}

	switch name {
	case "self":
		return &ast.SelfExpression{
			NodeBase: ast.NodeBase{
				Span: firstIdent.Span,
			},
		}
	}

	if firstIdent.Name[len(firstIdent.Name)-1] == '-' {
		firstIdent.Err = &sourcecode.ParsingError{UnspecifiedParsingError, IDENTIFIER_LITERAL_MUST_NO_END_WITH_A_HYPHEN}
	}

	lastDotIndex := int32(-1)

	//identifier member expression
	if p.i < p.len && p.s[p.i] == '.' {
		lastDotIndex = p.i
		p.i++

		if allowUnprefixedPatternNamespaceIdent && (p.i >= p.len || isSpaceNotLF(p.s[p.i]) || isUnpairedOrIsClosingDelim(p.s[p.i])) {
			return &ast.PatternNamespaceIdentifierLiteral{
				NodeBase:   ast.NodeBase{Span: NodeSpan{start, p.i}},
				Name:       name,
				Unprefixed: true,
			}
		}

		var memberExpr ast.Node = &ast.IdentifierMemberExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{Start: firstIdent.Span.Start},
			},
			Left:          firstIdent,
			PropertyNames: nil,
		}

		for {
			nameStart := p.i
			isOptional := false
			isComputed := false
			var propNameNode ast.Node

			if p.i < p.len && p.s[p.i] == '?' {
				isOptional = true
				p.i++
				nameStart = p.i
			}

			if p.i >= p.len {
				base := memberExpr.BasePtr()
				base.Span.End = p.i

				base.Err = &sourcecode.ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR}
				p.tokens = append(p.tokens, ast.Token{Type: ast.DOT, Span: NodeSpan{p.i - 1, p.i}})
				return memberExpr
			}

			switch {
			case p.s[p.i] == '(':
				isComputed = true
			case p.s[p.i] == '{':
				object := memberExpr
				identMemberExpr, ok := memberExpr.(*ast.IdentifierMemberExpression)
				//ast.IdentifierMemberExpression is the only possible type of memberExpr that can be incomplete
				if ok {
					object.BasePtr().Span.End = p.i - 1
					if len(identMemberExpr.PropertyNames) == 0 {
						object = identMemberExpr.Left
					}
				}

				p.i--
				keyList := p.parseKeyList()

				return &ast.ExtractionExpression{
					NodeBase: ast.NodeBase{Span: NodeSpan{firstIdent.Span.Start, keyList.Span.End}},
					Object:   object,
					Keys:     keyList,
				}
			case isAlpha(p.s[p.i]) || p.s[p.i] == '_':
				//
			case isValidUnquotedStringChar(p.s, p.i):
				return p.parseUnquotedStringLiteral(start)
				//memberExpr.NodeBase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, makePropNameShouldStartWithAletterNot(p.s[p.i])}
				//return memberExpr
			default:
				base := memberExpr.BasePtr()
				base.Span.End = p.i

				base.Err = &sourcecode.ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR}
				p.tokens = append(p.tokens, ast.Token{Type: ast.DOT, Span: NodeSpan{p.i - 1, p.i}})
				return memberExpr
			}

			if isComputed {

				p.i++
				propNameNode = p.parseUnaryBinaryAndParenthesizedExpression(p.i-1, -1)
			} else {
				for p.i < p.len && IsIdentChar(p.s[p.i]) {
					p.i++
				}

				propName := string(p.s[nameStart:p.i])
				propNameNode = &ast.IdentifierLiteral{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{nameStart, p.i},
					},
					Name: propName,
				}
			}

			identMemberExpr, ok := memberExpr.(*ast.IdentifierMemberExpression)
			if ok && !isOptional && !isComputed {
				identMemberExpr.PropertyNames = append(identMemberExpr.PropertyNames, propNameNode.(*ast.IdentifierLiteral))
			} else {
				if ok {
					identMemberExpr.BasePtr().Span.End = lastDotIndex
				}

				left := memberExpr
				if ok && len(identMemberExpr.PropertyNames) == 0 {
					left = firstIdent
				}

				if !isComputed {
					memberExpr = &ast.MemberExpression{
						NodeBase:     ast.NodeBase{Span: NodeSpan{firstIdent.Span.Start, p.i}},
						Left:         left,
						PropertyName: propNameNode.(*ast.IdentifierLiteral),
						Optional:     isOptional,
					}
				} else {
					memberExpr = &ast.ComputedMemberExpression{
						NodeBase:     ast.NodeBase{Span: NodeSpan{firstIdent.Span.Start, p.i}},
						Left:         left,
						PropertyName: propNameNode,
						Optional:     isOptional,
					}
				}
			}

			if p.i >= p.len || p.s[p.i] != '.' {
				break
			}
			lastDotIndex = p.i
			p.i++
		}

		memberExpr.BasePtr().Span.End = p.i

		if p.i < p.len && (p.s[p.i] == '\\' || (isValidUnquotedStringChar(p.s, p.i) && p.s[p.i] != ':')) {
			return p.parseUnquotedStringLiteral(start)
		}
		return memberExpr
	}

	isProtocol := p.i < p.len-1 && string(p.s[p.i:p.i+2]) == ":/"

	if !isProtocol && p.i < p.len && (p.s[p.i] == '\\' || (isValidUnquotedStringChar(p.s, p.i) && p.s[p.i] != ':')) {
		return p.parseUnquotedStringLiteral(start)
	}

	switch name {
	case "true", "false":
		return &ast.BooleanLiteral{
			NodeBase: ast.NodeBase{
				Span: firstIdent.Span,
			},
			Value: name[0] == 't',
		}
	case "nil":
		return &ast.NilLiteral{
			NodeBase: ast.NodeBase{
				Span: firstIdent.Span,
			},
		}
	}

	if isProtocol {
		if slices.Contains(SCHEMES, name) {
			if p.inPattern {
				p.i++ //eat ':'
				percentPrefixed := false
				return p.parseURLLikePattern(start, percentPrefixed)
			}

			return p.parseURLLike(start, nil)
		}
		base := firstIdent.NodeBase
		base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidURIUnsupportedProtocol(name)}

		return &ast.InvalidURL{
			NodeBase: base,
			Value:    name,
		}
	}

	return firstIdent
}

func (p *parser) parseKeyList() *ast.KeyListExpression {
	p.panicIfContextDone()

	start := p.i
	p.i += 2

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_KEYLIST_BRACKET, Span: NodeSpan{p.i - 2, p.i}})

	var (
		idents     []ast.Node
		parsingErr *sourcecode.ParsingError
	)
	for p.i < p.len && p.s[p.i] != '}' {
		p.eatSpaceComma()

		if p.i >= p.len {
			//this case is handled next
			break
		}

		e, missingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		if missingExpr {
			r := p.s[p.i]
			span := NodeSpan{p.i, p.i + 1}

			p.i++
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: span, Raw: string(r)})

			e = &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					span,
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInKeyList(r)},
					false,
				},
			}
			idents = append(idents, e)
			continue
		}

		if p.inPattern {
			if patternIdent, ok := e.(*ast.PatternIdentifierLiteral); ok {
				e = &ast.IdentifierLiteral{
					NodeBase: e.Base(),
					Name:     patternIdent.Name,
				}
			}
		}

		idents = append(idents, e)

		if _, ok := e.(*ast.IdentifierLiteral); !ok {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEY_LIST_CAN_ONLY_CONTAIN_IDENTS}
		}

		p.eatSpaceComma()
	}

	if p.i >= p.len {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_KEY_LIST_MISSING_BRACE}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &ast.KeyListExpression{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Keys: idents,
	}
}

func (p *parser) parseListOrTupleLiteral(isTuple bool) ast.Node {
	p.panicIfContextDone()

	var (
		openingBracketIndex = p.i
		elements            []ast.Node
		type_               ast.Node
		parsingErr          *sourcecode.ParsingError
	)

	if isTuple {
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_TUPLE_BRACKET, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	//parse type annotation if present
	if p.i < p.len-1 && p.s[p.i] == ']' && p.s[p.i+1] == '%' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		type_ = p.parsePercentPrefixedPattern(false)
		if p.i >= p.len || p.s[p.i] != '[' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_OPENING_BRACKET_AFTER_TYPE}
			if isTuple {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_TUPLE_LIT_MISSING_OPENING_BRACKET_AFTER_TYPE}
			}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	}

	if parsingErr == nil {

		//parse elements
		for p.i < p.len && p.s[p.i] != ']' {
			p.eatSpaceNewlineCommaComment()

			if p.i >= p.len || p.s[p.i] == ']' {
				break
			}

			spreadStart := p.i
			isSpread := p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.'
			if isSpread {
				p.i += 3
			}

			e, isMissingExpr := p.parseExpression()

			if isSpread {
				p.tokens = append(p.tokens, ast.Token{Type: ast.THREE_DOTS, Span: NodeSpan{spreadStart, spreadStart + 3}})
				e = &ast.ElementSpreadElement{
					NodeBase: ast.NodeBase{
						NodeSpan{spreadStart, e.Base().Span.End},
						nil,
						false,
					},
					Expr: e,
				}
			}

			if isMissingExpr && p.s[p.i] != ',' {
				break
			}

			elements = append(elements, e)
			if p.i >= p.len {
				break
			}
			p.eatSpaceNewlineCommaComment()
		}

		if p.i >= p.len || p.s[p.i] != ']' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_CLOSING_BRACKET}
			if isTuple {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_TUPLE_LIT_MISSING_CLOSING_BRACKET}
			}
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	}

	if isTuple {
		return &ast.TupleLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{openingBracketIndex, p.i},
				Err:  parsingErr,
			},
			TypeAnnotation: type_,
			Elements:       elements,
		}
	}

	return &ast.ListLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{openingBracketIndex, p.i},
			Err:  parsingErr,
		},
		TypeAnnotation: type_,
		Elements:       elements,
	}
}

func (p *parser) parseDictionaryLiteral() *ast.DictionaryLiteral {
	p.panicIfContextDone()

	openingIndex := p.i
	p.i += 2

	var parsingErr *sourcecode.ParsingError
	var entries []*ast.DictionaryEntry
	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_DICTIONARY_BRACKET, Span: NodeSpan{p.i - 2, p.i}})

	p.eatSpaceNewlineCommaComment()

dictionary_literal_top_loop:
	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) { //one iteration == one entry (that can be invalid)

		if p.s[p.i] == '}' {
			break dictionary_literal_top_loop
		}

		entry := &ast.DictionaryEntry{
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
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionary(p.s[p.i-1])},
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
				} else if !utils.Implements[*ast.IdentifierLiteral](k) {
					key.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DICT_KEY_ONLY_SIMPLE_VALUE_LITS}
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
				msg := INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY
				if colonInLiteral {
					msg = INVALID_DICT_ENTRY_MISSING_SPACE_BETWEEN_KEY_AND_COLON
				}

				entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, msg}
				entry.Span.End = p.i
			}
			break
		}

		if p.s[p.i] != ':' {
			if p.s[p.i] != ',' {
				entry.Span.End = p.i
				entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionary(p.s[p.i])}
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
				break dictionary_literal_top_loop //No need to add the the entry since it is already added .
			}
			if entry.Err == nil {
				entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionary(char)}
			}
			p.i++
		}

		p.eatSpace()

		if p.i < p.len && !isValidEntryEnd(p.s, p.i) && entry.Err == nil {
			entry.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DICT_LIT_ENTRY_SEPARATION}
		}

		p.eatSpaceNewlineCommaComment()
	}

	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	} else {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_DICT_MISSING_CLOSING_BRACE}
	}

	return &ast.DictionaryLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{openingIndex, p.i},
			Err:  parsingErr,
		},
		Entries: entries,
	}
}

func (p *parser) parseRuneRuneRange() ast.Node {
	p.panicIfContextDone()

	start := p.i

	parseRuneLiteral := func() *ast.RuneLiteral {
		start := p.i
		p.i++

		if p.i >= p.len {
			return &ast.RuneLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_RUNE_LIT},
					false,
				},
				Value: 0,
			}
		}

		value := p.s[p.i]

		if value == '\'' {
			return &ast.RuneLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RUNE_LIT_NO_CHAR},
					false,
				},
				Value: 0,
			}
		}

		if value == '\\' {
			p.i++
			switch p.s[p.i] {
			//same single character escapes as Golang
			case 'a':
				value = '\a'
			case 'b':
				value = '\b'
			case 'f':
				value = '\f'
			case 'n':
				value = '\n'
			case 'r':
				value = '\r'
			case 't':
				value = '\t'
			case 'v':
				value = '\v'
			case '\\':
				value = '\\'
			case '\'':
				value = '\''
			default:
				return &ast.RuneLiteral{
					NodeBase: ast.NodeBase{
						NodeSpan{start, p.i},
						&sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RUNE_LIT_INVALID_SINGLE_CHAR_ESCAPE},
						false,
					},
					Value: 0,
				}
			}
		}

		p.i++

		var parsingErr *sourcecode.ParsingError
		if p.i >= p.len || p.s[p.i] != '\'' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_RUNE_LIT_MISSING_QUOTE}
		} else {
			p.i++
		}

		return &ast.RuneLiteral{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
			Value: value,
		}

	}

	lower := parseRuneLiteral()

	if p.i >= p.len || p.s[p.i] != '.' {
		return lower
	}

	p.i++
	if p.i >= p.len || p.s[p.i] != '.' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.DOT, Span: NodeSpan{p.i - 1, p.i}})

		return &ast.RuneRangeExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RUNE_RANGE_EXPR},
				false,
			},
			Lower: lower,
			Upper: nil,
		}
	}
	p.i++
	p.tokens = append(p.tokens, ast.Token{Type: ast.TWO_DOTS, Span: NodeSpan{p.i - 2, p.i}})

	if p.i >= p.len || p.s[p.i] != '\'' {
		return &ast.RuneRangeExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RUNE_RANGE_EXPR},
				false,
			},
			Lower: lower,
			Upper: nil,
		}
	}

	upper := parseRuneLiteral()

	return &ast.RuneRangeExpression{
		NodeBase: ast.NodeBase{
			NodeSpan{start, upper.Base().Span.End},
			nil,
			false,
		},
		Lower: lower,
		Upper: upper,
	}
}

func (p *parser) parseIfExpression(openingParenIndex int32 /* -1 if unparenthesized (in markup interpolation) */, ifKeywordStart int32) *ast.IfExpression {
	p.panicIfContextDone()

	var alternate ast.Node
	var end int32
	var parsingErr *sourcecode.ParsingError
	shouldHaveClosingParen := openingParenIndex >= 0

	ifExprStart := openingParenIndex
	if openingParenIndex < 0 {
		ifExprStart = ifKeywordStart
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.IF_KEYWORD, Span: NodeSpan{ifKeywordStart, ifKeywordStart + 2}})

	p.eatSpaceNewlineComment()

	test, _ := p.parseExpression()
	p.eatSpaceNewlineComment()

	consequent, isMissingExpr := p.parseExpression()
	p.eatSpaceNewlineComment()

	if isMissingExpr {
		if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {

			if p.i < p.len && p.s[p.i] == ')' {
				p.tokens = append(p.tokens, ast.Token{
					Type: ast.CLOSING_PARENTHESIS,
					Span: NodeSpan{p.i, p.i + 1},
				})
				p.i++
			}

			end = p.i

			return &ast.IfExpression{
				NodeBase: ast.NodeBase{
					Span:            NodeSpan{ifExprStart, end},
					Err:             parsingErr,
					IsParenthesized: shouldHaveClosingParen,
				},
				Test:       test,
				Consequent: consequent,
				Alternate:  alternate,
			}
		}
	}

	hasElse := false

	if ident, ok := consequent.(*ast.IdentifierLiteral); ok && ident.Name == "else" {
		hasElse = true
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_IF_EXPR_MISSING_COND_BETWEEN_IF_ELSE}
	} else if p.i < p.len-3 && p.s[p.i] == 'e' && p.s[p.i+1] == 'l' && p.s[p.i+2] == 's' && p.s[p.i+3] == 'e' {
		if p.i == p.len-4 /*else<EOF>*/ || !IsIdentChar(p.s[p.i+4]) {
			p.tokens = append(p.tokens, ast.Token{
				Type: ast.ELSE_KEYWORD,
				Span: NodeSpan{p.i, p.i + 4},
			})
			p.i += 4
			hasElse = true
		}
	}

	p.eatSpaceNewlineComment()

	if hasElse {
		alternate, _ = p.parseExpression()
		p.eatSpaceNewlineComment()
	}

	p.eatSpaceNewlineComment()

	if shouldHaveClosingParen {
		if p.i >= p.len {
			end = p.i
			if consequent != nil {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_IF_EXPR_IF_CLAUSE_SHOULD_BE_FOLLOWED_BY_CLOSING_PAREN_OR_ELSE_CLAUSE}
			}
		} else if p.s[p.i] == ')' {
			p.tokens = append(p.tokens, ast.Token{
				Type: ast.CLOSING_PARENTHESIS,
				Span: NodeSpan{p.i, p.i + 1},
			})
			p.i++
			end = p.i
		} else {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_IF_EXPR_IF_CLAUSE_SHOULD_BE_FOLLOWED_BY_CLOSING_PAREN_OR_ELSE_CLAUSE}

			if !isUnpairedOrIsClosingDelim(p.s[p.i]) {
				alternate, _ = p.parseExpression()

				p.eatSpaceNewlineComment()

				if p.i < p.len && p.s[p.i] == ')' {
					p.tokens = append(p.tokens, ast.Token{
						Type: ast.CLOSING_PARENTHESIS,
						Span: NodeSpan{p.i, p.i + 1},
					})
					p.i++
				}
			}

			end = p.i
		}
	} else { //in markup interpolation
		//No need to report an error if there is something after the expression
		//because this is handled by the caller.
		end = p.i
	}

	return &ast.IfExpression{
		NodeBase: ast.NodeBase{
			Span:            NodeSpan{ifExprStart, end},
			Err:             parsingErr,
			IsParenthesized: shouldHaveClosingParen,
		},
		Test:       test,
		Consequent: consequent,
		Alternate:  alternate,
	}
}

// parseParenthesizedCallArgs parses the arguments of a parenthesized call up until the closing parenthesis (included)
func (p *parser) parseParenthesizedCallArgs(call *ast.CallExpression) *ast.CallExpression {
	p.panicIfContextDone()

	var (
		lastSpreadArg *ast.SpreadArgument = nil
		argErr        *sourcecode.ParsingError
	)

	//parse arguments
	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewlineComma()

		if p.i >= p.len || p.s[p.i] == ')' {
			break
		}

		if lastSpreadArg != nil {
			argErr = &sourcecode.ParsingError{UnspecifiedParsingError, SPREAD_ARGUMENT_CANNOT_BE_FOLLOWED_BY_ADDITIONAL_ARGS}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}})
			lastSpreadArg = &ast.SpreadArgument{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, 0},
					Err:  argErr,
				},
			}
			p.i += 3
		}

		arg, isMissingExpr := p.parseExpression()

		if isMissingExpr {
			if p.i >= p.len || p.s[p.i] == ')' {
				break
			}
			p.i++
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: arg.Base().Span, Raw: string(p.s[p.i-1])})

			arg = &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					arg.Base().Span,
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCallArguments(p.s[p.i-1])},
					false,
				},
			}
		}

		if lastSpreadArg != nil {
			lastSpreadArg.Expr = arg
			lastSpreadArg.Span.End = arg.Base().Span.End
			arg = lastSpreadArg
			if arg.Base().Err == nil {
				arg.BasePtr().Err = argErr
			}
		}

		call.Arguments = append(call.Arguments, arg)
		p.eatSpaceNewlineComma()
	}

	var parsingErr *sourcecode.ParsingError

	if p.i >= p.len || p.s[p.i] != ')' {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_CALL_MISSING_CLOSING_PAREN}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	call.NodeBase.Span.End = p.i
	call.Err = parsingErr
	return call
}

// parseCallArgsNoParenthesis parses the arguments of a call without parenthesis up until the end of the line or the next non-opening delimiter
func (p *parser) parseCallArgsNoParenthesis(call *ast.CallExpression) {
	p.panicIfContextDone()

	var lastSpreadArg *ast.SpreadArgument = nil

	for p.i < p.len && (!isUnpairedOrIsClosingDelim(p.s[p.i]) || p.s[p.i] == ':') {
		p.eatSpaceComments()

		if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && p.s[p.i] != ':') {
			break
		}

		var argErr *sourcecode.ParsingError

		if lastSpreadArg != nil {
			argErr = &sourcecode.ParsingError{UnspecifiedParsingError, SPREAD_ARGUMENT_CANNOT_BE_FOLLOWED_BY_ADDITIONAL_ARGS}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}})

			lastSpreadArg = &ast.SpreadArgument{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, 0},
					Err:  argErr,
				},
			}
			p.i += 3
		}

		arg, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		if lastSpreadArg != nil {
			lastSpreadArg.Expr = arg
			lastSpreadArg.Span.End = arg.Base().Span.End
			arg = lastSpreadArg
			if arg.Base().Err == nil {
				arg.BasePtr().Err = argErr
			}
		}

		if isMissingExpr {
			if p.i >= p.len {
				call.Arguments = append(call.Arguments, arg)
				break
			} else {
				p.i++

				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i - 1, p.i}, Raw: string(p.s[p.i-1])})

				arg = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						NodeSpan{p.i - 1, p.i},
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCallArguments(p.s[p.i-1])},
						false,
					},
				}
			}
		}

		call.Arguments = append(call.Arguments, arg)

		p.eatSpaceComments()
	}
}

type DateLikeLiteralKind int

const (
	YearLit DateLikeLiteralKind = iota + 1
	DateLit
	DateTimeLit
)

// ParseDateLikeLiteral parses a date-like literal (year, date or datetime). If there is a parsing error,
// the kind result is set to the best guess.
func ParseDateLikeLiteral(braw []byte) (date time.Time, kind DateLikeLiteralKind, parsingErr *sourcecode.ParsingError) {
	if len(braw) > 70 {
		return time.Time{}, DateTimeLit, &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DATE_LIKE_LITERAL}
	}

	if !DATELIKE_LITERAL_REGEX.Match(braw) {
		dashCount := bytes.Count(braw, []byte{'-'})

		//almost ok but location is missing
		if NO_LOCATION_DATELIKE_LITERAL_REGEX.Match(braw) {
			var estimatedKind DateLikeLiteralKind = DateTimeLit
			switch dashCount {
			case 0:
				estimatedKind = YearLit
			case 2:
				estimatedKind = DateLit
			default:
				estimatedKind = DateTimeLit
			}

			return time.Time{}, estimatedKind, &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DATELIKE_LITERAL_MISSING_LOCATION_PART_AT_THE_END}
		}

		hasDays := false
		hasMonths := false
		hasHours := false
		hasMinutes := false
		hasSeconds := false
		hasMicroseconds := false

		for i, c := range braw {
			// detect the sequence <digit> <alpha> ('-' | <alpha> | EOF)
			if isDecDigit(rune(c)) &&
				i < len(braw)-1 &&
				isAlpha(rune(braw[i+1])) &&
				(i == len(braw)-2 || braw[i+2] == '-' || isAlpha(rune(braw[i+2]))) {
				nextChar := braw[i+1]
				switch nextChar {
				case 'm':
					if i < len(braw)-2 && braw[i+2] == '-' {
						hasMinutes = true
					} else {
						hasMonths = true
					}
				case 'd':
					hasDays = true
				case 's':
					hasSeconds = true
				case 'h':
					hasHours = true
				case 'u':
					hasMicroseconds = true
				}
			}
		}

		estimatedKind := DateTimeLit
		errorMessage := INVALID_DATE_LIKE_LITERAL

		switch {
		case hasHours || hasMinutes || hasSeconds || hasMicroseconds:
			estimatedKind = DateTimeLit

			if !hasMonths && !hasDays {
				errorMessage = INVALID_DATETIME_LITERAL_BOTH_MONTH_AND_DAY_COUNT_PROBABLY_MISSING
			} else if !hasDays {
				errorMessage = INVALID_DATETIME_LITERAL_DAY_COUNT_PROBABLY_MISSING
			} else if !hasMonths {
				errorMessage = INVALID_DATETIME_LITERAL_MONTH_COUNT_PROBABLY_MISSING
			}

		case !hasDays && !hasMonths:
			estimatedKind = YearLit
			errorMessage = INVALID_YEAR_LITERAL

		case hasDays && !hasMonths:
			estimatedKind = DateLit
			errorMessage = INVALID_DATE_LITERAL_MONTH_COUNT_PROBABLY_MISSING

		case hasMonths && !hasDays:
			estimatedKind = DateLit
			errorMessage = INVALID_DATE_LITERAL_DAY_COUNT_PROBABLY_MISSING
		}

		return time.Time{}, estimatedKind, &sourcecode.ParsingError{UnspecifiedParsingError, errorMessage}
	}

	parts := bytes.Split(braw, []byte{'-'})
	year := string(parts[0][:len32(parts[0])-1])
	month := "1"
	day := "1"
	hour := "0"
	minute := "0"
	second := "0"
	ms := "0"
	us := "0"

	if len(parts) > 2 {
		if len(parts) > 4 {
			kind = DateTimeLit
		} else {
			//<year>-<month>-<day>-<location>
			kind = DateLit
		}
	} else {
		//<year>-<location>
		kind = YearLit
	}

	for _, part := range parts[1 : len32(parts)-1] {
		switch part[len32(part)-1] {
		case 't':
			month = string(part[:len32(part)-2])

			if len(month) == 0 {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, MISSING_MONTH_VALUE}
				return
			}

			if month[0] == '0' {
				if len(month) == 1 || !isDecDigit(rune(month[1])) {
					parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_MONTH_VALUE}
					return
				}
				month = month[1:]
			}
		case 'd':
			day = string(part[:len32(part)-1])

			if len(day) == 0 {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, MISSING_DAY_VALUE}
				return
			}

			if day[0] == '0' {
				if len(day) == 1 || !isDecDigit(rune(day[1])) {
					parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DAY_VALUE}
					return
				}
				day = day[1:]
			}
		case 'h':
			hour = string(part[:len32(part)-1])
		case 'm':
			minute = string(part[:len32(part)-1])
		case 's':
			if part[len32(part)-2] == 'm' {
				ms = string(part[:len32(part)-2])
			} else if part[len32(part)-2] == 'u' {
				us = string(part[:len32(part)-2])
			} else {
				second = string(part[:len32(part)-1])
			}
		}
	}

	locationPart := string(parts[len32(parts)-1])

	mustAtoi := func(s string) int {
		i, _ := strconv.Atoi(s)
		return i
	}

	loc, err := time.LoadLocation(locationPart)
	if err != nil {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmt.Sprintf("invalid time location in literal: %s", err)}
		return
	}

	nanoseconds := 1_000*mustAtoi(us) + 1_000_000*mustAtoi(ms)

	return time.Date(
		mustAtoi(year), time.Month(mustAtoi(month)), mustAtoi(day),
		mustAtoi(hour), mustAtoi(minute), mustAtoi(second), nanoseconds, loc), kind, nil
}

func (p *parser) parseDateLikeLiterals(start int32) ast.Node {
	p.panicIfContextDone()

	base := ast.NodeBase{
		NodeSpan{start, p.i},
		nil,
		false,
	}

	p.i++
	base.Span.End = p.i

	if p.i >= p.len {
		base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DATELIKE_LITERAL_MISSING_LOCATION_PART_AT_THE_END}
		return &ast.YearLiteral{
			NodeBase: base,
			Raw:      string(p.s[start:p.i]),
		}
	}

	r := p.s[p.i]

	if r == '-' {
		p.i++
		base.Span.End = p.i

		if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
			base.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DATELIKE_LITERAL_MISSING_LOCATION_PART_AT_THE_END}
			return &ast.YearLiteral{
				NodeBase: base,
				Raw:      string(p.s[start:p.i]),
			}
		}
	}

	r = p.s[p.i]
	for isAlpha(r) || isDecDigit(r) || r == '-' || r == '/' || r == '_' {
		p.i++
		if p.i >= p.len {
			break
		}
		r = p.s[p.i]
	}

	base.Span.End = p.i
	raw := string(p.s[start:p.i])
	braw := []byte(raw)

	var value time.Time

	date, kind, err := ParseDateLikeLiteral(braw)

	if err != nil {
		base.Err = err
	} else {
		value = date
	}

	switch kind {
	case YearLit:
		return &ast.YearLiteral{
			NodeBase: base,
			Raw:      raw,
			Value:    value,
		}
	case DateLit:
		return &ast.DateLiteral{
			NodeBase: base,
			Raw:      raw,
			Value:    value,
		}
	case DateTimeLit:
		return &ast.DateTimeLiteral{
			NodeBase: base,
			Raw:      raw,
			Value:    value,
		}
	default:
		panic(ErrUnreachable)
	}
}

func (p *parser) parsePortLiteral() *ast.PortLiteral {
	p.panicIfContextDone()

	start := p.i // ':'
	p.i++

	portNumber := int(p.s[p.i] - '0')
	p.i++

	for p.i < p.len && isDecDigit(p.s[p.i]) {
		portNumber *= 10
		portNumber += int(p.s[p.i] - '0')
		p.i++
	}

	var parsingErr *sourcecode.ParsingError
	if portNumber > math.MaxUint16 {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_PORT_LITERAL_INVALID_PORT_NUMBER}
	}

	schemeName := ""

	if p.i < p.len && p.s[p.i] == '/' { //scheme
		p.i++
		schemeNameStart := p.i

		for p.i < p.len && (isAlpha(p.s[p.i]) || p.s[p.i] == '-') {
			p.i++
		}
		schemeName = string(p.s[schemeNameStart:p.i])
		if len(schemeName) == 0 && parsingErr == nil {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PORT_LITERAL_MISSING_SCHEME_NAME_AFTER_SLASH}
		}
	}

	return &ast.PortLiteral{
		Raw: string(p.s[start:p.i]),
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		PortNumber: uint16(portNumber),
		SchemeName: schemeName,
	}
}

func (p *parser) parseNumberAndNumberRange() ast.Node {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *sourcecode.ParsingError
	base := 10

	parseIntegerLiteral := func(raw string, start, end int32, base int) (*ast.IntLiteral, int64) {
		s := raw
		switch base {
		case 8:
			s = strings.TrimPrefix(s, "0o")
		case 16:
			s = strings.TrimPrefix(s, "0x")
		}

		integer, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), base, 64)
		if err != nil {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_INT_LIT}
		}

		return &ast.IntLiteral{
			NodeBase: ast.NodeBase{
				NodeSpan{start, end},
				parsingErr,
				false,
			},
			Raw:   raw,
			Value: integer,
		}, integer
	}

	if p.s[p.i] == '-' {
		p.i++
	}

	for p.i < p.len && (isDecDigit(p.s[p.i]) || p.s[p.i] == '_') {
		p.i++
	}

	if p.i < p.len && p.s[p.i] == '.' {
		p.i++

		if p.i < p.len && p.s[p.i] == '.' { //int range literal
			lower := string(p.s[start : p.i-1])
			lowerIntLiteral, _ := parseIntegerLiteral(lower, start, p.i-1, 10)
			p.tokens = append(p.tokens, ast.Token{Type: ast.TWO_DOTS, Span: NodeSpan{p.i - 1, p.i + 1}})

			p.i++

			upperBound, isMissingExpr := p.parseExpression()

			if isMissingExpr {
				return &ast.IntegerRangeLiteral{
					NodeBase: ast.NodeBase{
						NodeSpan{start, p.i},
						nil,
						false,
					},
					LowerBound: lowerIntLiteral,
					UpperBound: nil,
				}
			}

			var parsingError *sourcecode.ParsingError
			if _, ok := upperBound.(*ast.IntLiteral); !ok {
				parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_INT_LIT}
			}

			return &ast.IntegerRangeLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{lowerIntLiteral.Base().Span.Start, upperBound.Base().Span.End},
					parsingError,
					false,
				},
				LowerBound: lowerIntLiteral,
				UpperBound: upperBound,
			}
		}

		//else float
		for p.i < p.len && (isDecDigit(p.s[p.i]) || p.s[p.i] == '-' || p.s[p.i] == '_') {
			p.i++
		}
	} else if p.i < p.len-1 && p.s[p.i] == 'x' && isHexDigit(p.s[p.i+1]) { //hexa decimal
		base = 16
		p.i++
		for p.i < p.len && (isHexDigit(p.s[p.i]) || p.s[p.i] == '_') {
			p.i++
		}
	} else if p.i < p.len-1 && p.s[p.i] == 'o' && isOctalDigit(p.s[p.i+1]) { //octal
		base = 8
		p.i++
		for p.i < p.len && (isOctalDigit(p.s[p.i]) || p.s[p.i] == '_') {
			p.i++
		}
	}

	raw := string(p.s[start:p.i])

	var literal ast.Node

	if strings.ContainsRune(raw, '.') { //float

		if p.i < p.len && p.s[p.i] == 'e' {
			p.i++

			if p.i < p.len && p.s[p.i] == '-' {
				p.i++
			}

			for p.i < p.len && isDecDigit(p.s[p.i]) {
				p.i++
			}
			raw = string(p.s[start:p.i])
		}

		float, err := strconv.ParseFloat(strings.ReplaceAll(raw, "_", ""), 64)
		if err != nil {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FLOAT_LIT}
		}

		literal = &ast.FloatLiteral{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
			Raw:   raw,
			Value: float,
		}

		if p.i < p.len-1 && p.s[p.i] == '.' && p.s[p.i+1] == '.' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.TWO_DOTS, Span: NodeSpan{p.i, p.i + 2}})
			p.i += 2

			lowerFloatLiteral := literal.(*ast.FloatLiteral)

			upperBound, isMissingExpr := p.parseExpression()

			if isMissingExpr {
				return &ast.FloatRangeLiteral{
					NodeBase: ast.NodeBase{
						NodeSpan{start, p.i},
						nil,
						false,
					},
					LowerBound: lowerFloatLiteral,
					UpperBound: nil,
				}
			}

			var parsingError *sourcecode.ParsingError
			if _, ok := upperBound.(*ast.FloatLiteral); !ok {
				parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_FLOAT_LIT}
			}

			return &ast.FloatRangeLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{lowerFloatLiteral.Base().Span.Start, upperBound.Base().Span.End},
					parsingError,
					false,
				},
				LowerBound: lowerFloatLiteral,
				UpperBound: upperBound,
			}
		}
	} else {
		literal, _ = parseIntegerLiteral(raw, start, p.i, base)
	}

	return literal
}

func (p *parser) parseByteSlices() ast.Node {
	p.panicIfContextDone()

	start := p.i //index of '0'
	p.i++

	var (
		parsingError *sourcecode.ParsingError
		value        []byte
	)

base_switch:
	switch p.s[p.i] {
	case 'x':
		p.i++
		if p.i >= p.len || p.s[p.i] != '[' {
			return &ast.ByteSliceLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_HEX_BYTE_SICE_LIT_MISSING_BRACKETS},
					false,
				},
			}
		}
		p.i++

		p.eatSpace()

		buff := make([]byte, 0)

		for p.i < p.len && p.s[p.i] != ']' {
			r := p.s[p.i]
			switch {
			case r >= '0' && r <= '9' || r >= 'a' && r <= 'z':
				buff = append(buff, byte(r))
			case isClosingDelim(r):
				break base_switch
			default:
				if parsingError == nil {
					parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInHexadecimalByteSliceLiteral(r)}
				} else {
					parsingError.Message += "\n" + fmtUnexpectedCharInHexadecimalByteSliceLiteral(r)
				}
			}
			p.i++
			p.eatSpace()
		}

		if parsingError == nil {
			if len32(buff)%2 != 0 {
				parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_HEX_BYTE_SICE_LIT_LENGTH_SHOULD_BE_EVEN}
			} else {
				value = make([]byte, hex.DecodedLen(len(buff)))
				_, err := hex.Decode(value, buff)
				if err != nil {
					parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_HEX_BYTE_SICE_LIT_FAILED_TO_DECODE}
				}
			}
		}

	case 'b':
		p.i++
		if p.i >= p.len || p.s[p.i] != '[' {
			return &ast.ByteSliceLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_BYTE_SICE_LIT_MISSING_BRACKETS},
					false,
				},
			}
		}
		p.i++

		p.eatSpace()

		value = make([]byte, 0)
		byte := byte(0)
		byteIndex := int32(0)

		for p.i < p.len && p.s[p.i] != ']' {
			r := p.s[p.i]
			switch r {
			case '0':
				byte = (byte << 1) + 0
				if byteIndex == 7 {
					value = append(value, byte)
					byteIndex = 0
				} else {
					byteIndex++
				}
			case '1':
				byte = (byte << 1) + 1
				if byteIndex == 7 {
					value = append(value, byte)
					byteIndex = 0
				} else {
					byteIndex++
				}
			case ' ', '\n', '\r':
			default:
				if isClosingDelim(r) {
					break base_switch
				}

				if parsingError == nil {
					parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBinByteSliceLiteral(r)}
				} else {
					parsingError.Message += "\n" + fmtUnexpectedCharInBinByteSliceLiteral(r)
				}
			}
			p.i++
			p.eatSpace()
		}
		if byteIndex != 0 {
			value = append(value, byte)
		}

	case 'd':
		p.i++
		if p.i >= p.len || p.s[p.i] != '[' {
			return &ast.ByteSliceLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_DECIMAL_BYTE_SICE_LIT_MISSING_BRACKETS},
					false,
				},
			}
		}
		p.i++

		p.eatSpace()

		buff := make([]byte, 0)

		for p.i < p.len && p.s[p.i] != ']' {
			r := p.s[p.i]
			switch r {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				buff = append(buff, byte(r))
			case ' ', '\t', '\r':
				if len32(buff) > 0 && buff[len32(buff)-1] != ' ' {
					buff = append(buff, ' ')
				}
			default:
				if isClosingDelim(r) {
					break base_switch
				}

				if parsingError == nil {
					parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDecimalByteSliceLiteral(r)}
				} else {
					parsingError.Message += "\n" + fmtUnexpectedCharInDecimalByteSliceLiteral(r)
				}
			}
			p.i++
		}

		//actual parsing
		if parsingError == nil {
			value = make([]byte, 0)

			if len32(buff) > 0 && buff[len32(buff)-1] != ' ' {
				buff = append(buff, ' ')
			}

			_byte := uint(0)
			byteStart := int32(0)

			for i, c := range buff {
				if c != ' ' { //digit
					_byte = _byte*10 + uint(c-'0')
				} else { //space
					if int32(i)-byteStart > 3 || _byte > 255 { //if the byte is invalid we add an error and exit the loop
						message := fmtInvalidByteInDecimalByteSliceLiteral(buff[byteStart:i])

						if parsingError == nil {
							parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, message}
						} else {
							parsingError.Message += "\n" + message
						}

						value = nil
						break
					}

					value = append(value, byte(_byte))
					_byte = 0
					byteStart = int32(i) + 1
				}
			}

		}
	default:
		return &ast.ByteSliceLiteral{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNKNOWN_BYTE_SLICE_BASE},
				false,
			},
		}
	}

	if p.i < p.len && p.s[p.i] == ']' {
		p.i++
	} else {
		if parsingError == nil {
			parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET}
		} else {
			parsingError.Message += "\n" + UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET
		}
	}

	return &ast.ByteSliceLiteral{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingError,
			false,
		},
		Raw:   string(p.s[start:p.i]),
		Value: value,
	}
}

func (p *parser) parseNumberAndRangeAndRateLiterals() ast.Node {
	p.panicIfContextDone()

	start := p.i //index of first digit or '-'
	e := p.parseNumberAndNumberRange()

	var fValue float64
	var isFloat = false
	isHexInt := false
	isOctalInt := false

	switch n := e.(type) {
	case *ast.IntLiteral:
		fValue = float64(n.Value)
		isHexInt = n.IsHex()
		isOctalInt = n.IsOctal()
	case *ast.FloatLiteral:
		fValue = float64(n.Value)
		isFloat = true
	default:
		return n
	}

	if p.i < p.len && (isAlpha(p.s[p.i]) || p.s[p.i] == '%') { //quantity literal or rate literal
		qtyOrRateLiteral := p.parseQuantityOrRateLiteral(start, fValue, isFloat)
		if isHexInt {
			qtyOrRateLiteral.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, QUANTITY_LIT_NOT_ALLOWED_WITH_HEXADECIMAL_NUM}
		} else if isOctalInt {
			qtyOrRateLiteral.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, QUANTITY_LIT_NOT_ALLOWED_WITH_OCTAL_NUM}
		}

		qtyLiteral, ok := qtyOrRateLiteral.(*ast.QuantityLiteral)
		//quantity range literal
		if ok && p.i < p.len-1 && p.s[p.i] == '.' && p.s[p.i+1] == '.' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.TWO_DOTS, Span: NodeSpan{p.i, p.i + 2}})
			p.i += 2

			upperBound, isMissingExpr := p.parseExpression()

			if isMissingExpr {
				return &ast.QuantityRangeLiteral{
					NodeBase: ast.NodeBase{
						NodeSpan{start, p.i},
						nil,
						false,
					},
					LowerBound: qtyLiteral,
					UpperBound: nil,
				}
			}

			var parsingError *sourcecode.ParsingError

			if _, ok := upperBound.(*ast.QuantityLiteral); !ok {
				parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_BE_QTY_LIT}
			}

			return &ast.QuantityRangeLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{qtyLiteral.Span.Start, upperBound.Base().Span.End},
					parsingError,
					false,
				},
				LowerBound: qtyLiteral,
				UpperBound: upperBound,
			}
		}
		return qtyOrRateLiteral
	}

	return e
}

func (p *parser) parseQuantityOrRateLiteral(start int32, fValue float64, float bool) ast.Node {
	p.panicIfContextDone()

	unitStart := p.i
	var parsingErr *sourcecode.ParsingError

	//date literal
	if !float && p.s[unitStart] == 'y' && (p.i < p.len-1 && p.s[p.i+1] == '-') {
		return p.parseDateLikeLiterals(start)
	}

	p.i++

	for p.i < p.len && isAlpha(p.s[p.i]) {
		p.i++
	}

	var values = []float64{fValue}
	var units = []string{string(p.s[unitStart:p.i])}

loop:
	for p.i < p.len && isDecDigit(p.s[p.i]) {
		e := p.parseNumberAndNumberRange()

		var fValue float64

		switch n := e.(type) {
		case *ast.IntLiteral:
			fValue = float64(n.Value)
		case *ast.FloatLiteral:
			fValue = float64(n.Value)
		default:
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_QUANTITY_LIT}
			break loop
		}

		values = append(values, fValue)

		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_QUANTITY_LIT}
			break
		} else {
			unitStart = p.i
			for p.i < p.len && isAlpha(p.s[p.i]) {
				p.i++
			}
			units = append(units, string(p.s[unitStart:p.i]))
		}
	}

	raw := string(p.s[start:p.i])

	literal := &ast.QuantityLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
		},
		Raw:    raw,
		Values: values,
		Units:  units,
	}

	if p.i < p.len {
		switch p.s[p.i] {
		case '/':
			p.i++

			var rateUnitStart = p.i
			var rateUnit string

			if p.i >= p.len {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT_DIV_SYMBOL_SHOULD_BE_FOLLOWED_BY_UNIT}
			} else {
				if !isAlpha(p.s[p.i]) {
					parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT}
				} else {
					for p.i < p.len && isAlpha(p.s[p.i]) {
						p.i++
					}
					rateUnit = string(p.s[rateUnitStart:p.i])

					if p.i < p.len && IsIdentChar(p.s[p.i]) {
						parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT}
					}
				}
			}

			return &ast.RateLiteral{
				NodeBase: ast.NodeBase{
					NodeSpan{literal.Base().Span.Start, p.i},
					parsingErr,
					false,
				},
				Values:  literal.Values,
				Units:   literal.Units,
				DivUnit: rateUnit,
				Raw:     literal.Raw + "/" + rateUnit,
			}
		}
	}

	return literal
}

// can return nil
func (p *parser) parsePreInitIfPresent() *ast.PreinitStatement {
	p.panicIfContextDone()

	var preinit *ast.PreinitStatement
	if p.i < p.len && strings.HasPrefix(string(p.s[p.i:]), ast.PREINIT_KEYWORD_STRING) {
		start := p.i

		p.tokens = append(p.tokens, ast.Token{Type: ast.PREINIT_KEYWORD, Span: NodeSpan{p.i, p.i + int32(len(ast.PREINIT_KEYWORD_STRING))}})
		p.i += int32(len(ast.PREINIT_KEYWORD_STRING))

		var end = p.i

		p.eatSpace()

		var (
			parsingErr   *sourcecode.ParsingError
			preinitBlock *ast.Block
		)
		if p.i >= p.len || p.s[p.i] != '{' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, PREINIT_KEYWORD_SHOULD_BE_FOLLOWED_BY_A_BLOCK}
		} else {
			preinitBlock = p.parseBlock()
			end = preinitBlock.Span.End
		}

		preinit = &ast.PreinitStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, end},
				Err:  parsingErr,
			},
			Block: preinitBlock,
		}
	}
	return preinit
}

// can return nil
func (p *parser) parseIncludaleChunkDescIfPresent() *ast.IncludableChunkDescription {
	p.panicIfContextDone()

	var includableChunk *ast.IncludableChunkDescription
	if p.i < p.len && strings.HasPrefix(string(p.s[p.i:]), INCLUDABLE_CHUNK_KEYWORD_STR) {
		start := p.i

		token := ast.Token{Type: ast.INCLUDABLE_FILE_KEYWORD, Span: NodeSpan{p.i, p.i + int32(len(INCLUDABLE_CHUNK_KEYWORD_STR))}}
		p.tokens = append(p.tokens, token)
		p.i += int32(len(INCLUDABLE_CHUNK_KEYWORD_STR))

		p.eatSpace()

		includableChunk = &ast.IncludableChunkDescription{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, token.Span.End},
			},
		}
	}
	return includableChunk
}

// can return nil
func (p *parser) parseManifestIfPresent() *ast.Manifest {
	p.panicIfContextDone()

	var manifest *ast.Manifest
	if p.i < p.len && strings.HasPrefix(string(p.s[p.i:]), MANIFEST_KEYWORD_STR) {
		start := p.i

		p.tokens = append(p.tokens, ast.Token{Type: ast.MANIFEST_KEYWORD, Span: NodeSpan{p.i, p.i + int32(len(MANIFEST_KEYWORD_STR))}})
		p.i += int32(len(MANIFEST_KEYWORD_STR))

		p.eatSpace()
		manifestObject, isMissingExpr := p.parseExpression()

		var err *sourcecode.ParsingError
		if _, ok := manifestObject.(*ast.ObjectLiteral); !ok && !isMissingExpr {
			err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_MANIFEST_DESC_VALUE}
		}

		manifest = &ast.Manifest{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, manifestObject.Base().Span.End},
				Err:  err,
			},
			Object: manifestObject,
		}

	}
	return manifest
}

func (p *parser) parseSingleGlobalConstDeclaration(declarations *[]*ast.GlobalConstantDeclaration) {
	p.panicIfContextDone()

	var declParsingErr *sourcecode.ParsingError

	lhs, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
	globvar, ok := lhs.(*ast.IdentifierLiteral)
	if !ok {
		declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_LHS_MUST_BE_AN_IDENT}
	} else if isKeyword(globvar.Name) {
		declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
	}

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '=' {
		if globvar != nil {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidConstDeclMissingEqualsSign(globvar.Name)}
		} else {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_MISSING_EQL_SIGN}
		}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &ast.GlobalConstantDeclaration{
			NodeBase: ast.NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				false,
			},
			Left: lhs,
		})
		return
	}

	equalSignIndex := p.i

	p.i++
	p.eatSpace()

	rhs, _ := p.parseExpression()
	p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.ASSIGN_EQUAL, Span: NodeSpan{equalSignIndex, equalSignIndex + 1}})

	*declarations = append(*declarations, &ast.GlobalConstantDeclaration{
		NodeBase: ast.NodeBase{
			NodeSpan{lhs.Base().Span.Start, rhs.Base().Span.End},
			declParsingErr,
			false,
		},
		Left:  lhs,
		Right: rhs,
	})
}

func (p *parser) parseGlobalConstantDeclarations() *ast.GlobalConstantDeclarations {
	p.panicIfContextDone()

	//nil is returned if there are no global constant declarations (no const (...) section)

	var (
		start            = p.i
		constKeywordSpan = NodeSpan{p.i, p.i + int32(len(CONST_KEYWORD_STR))}
	)

	if p.i < p.len && strings.HasPrefix(string(p.s[p.i:]), CONST_KEYWORD_STR) {
		p.i += int32(len(CONST_KEYWORD_STR))
		p.tokens = append(p.tokens, ast.Token{Type: ast.CONST_KEYWORD, Span: constKeywordSpan})

		p.eatSpace()
		var (
			declarations []*ast.GlobalConstantDeclaration
			parsingErr   *sourcecode.ParsingError
		)

		if p.i >= p.len || p.s[p.i] == '\n' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CONST_KEYWORD, Span: constKeywordSpan})

			return &ast.GlobalConstantDeclarations{
				NodeBase: ast.NodeBase{
					NodeSpan{start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
					false,
				},
			}
		}

		if p.s[p.i] != '(' { //single declaration, no parenthesis
			p.parseSingleGlobalConstDeclaration(&declarations)
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++

			for p.i < p.len && p.s[p.i] != ')' {
				p.eatSpaceNewlineComment()

				if p.i < p.len && p.s[p.i] == ')' {
					break
				}

				if p.i >= p.len {
					parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS_MISSING_CLOSING_PAREN}
					break
				}

				p.parseSingleGlobalConstDeclaration(&declarations)

				p.eatSpaceNewlineComment()
			}

			if p.i < p.len && p.s[p.i] == ')' {
				p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
			}
		}

		decls := &ast.GlobalConstantDeclarations{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				false,
			},
			Declarations: declarations,
		}

		return decls
	}

	return nil
}

func (p *parser) parseSingleLocalVarDeclarator(declarations *[]*ast.LocalVariableDeclarator) {
	p.panicIfContextDone()

	var declParsingErr *sourcecode.ParsingError

	//LHS
	var lhs ast.Node
	var objectDestructuration *ast.ObjectDestructuration
	var ident *ast.IdentifierLiteral

	if p.s[p.i] == '{' {
		objectDestructuration = p.parseObjectDestructuration()
		lhs = objectDestructuration
	} else {
		lhs, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		var ok bool
		ident, ok = lhs.(*ast.IdentifierLiteral)
		if !ok {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT}
		} else if isKeyword(ident.Name) {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
		}
	}

	p.eatSpace()

	//Unterminated

	if p.i >= p.len || (p.s[p.i] != '=' && !isAcceptedFirstVariableTypeAnnotationChar(p.s[p.i])) {
		if ident != nil {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidLocalVarDeclMissingEqualsSign(ident.Name)}
		} else if objectDestructuration != nil {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJECT_DESTRUCTURATION_MISSING_EQUAL_SIGN}
		}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &ast.LocalVariableDeclarator{
			NodeBase: ast.NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				false,
			},
			Left: lhs,
		})
		return
	}

	//Type annotation

	var type_ ast.Node

	if isAcceptedFirstVariableTypeAnnotationChar(p.s[p.i]) {
		prev := p.inPattern
		p.inPattern = true

		type_, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		p.inPattern = prev
		if objectDestructuration != nil && type_.Base().Err == nil {
			type_.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, TYPE_ANNOTATIONS_NOT_ALLOWED_WHEN_DESTRUCTURING_AN_OBJECT}
		}
	}

	p.eatSpace()

	//ast.Equal sign

	//temporary
	if p.i >= p.len || p.s[p.i] != '=' {
		declParsingErr = &sourcecode.ParsingError{MissingEqualsSignInDeclaration, EQUAL_SIGN_MISSING_AFTER_TYPE_ANNOTATION}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &ast.LocalVariableDeclarator{
			NodeBase: ast.NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				false,
			},
			Left: lhs,
			Type: type_,
		})
		return
	}

	equalSignIndex := p.i
	p.i++
	p.eatSpace()

	//RHS

	rhs, _ := p.parseExpression()
	p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.ASSIGN_EQUAL, Span: NodeSpan{equalSignIndex, equalSignIndex + 1}})

	*declarations = append(*declarations, &ast.LocalVariableDeclarator{
		NodeBase: ast.NodeBase{
			NodeSpan{lhs.Base().Span.Start, rhs.Base().Span.End},
			declParsingErr,
			false,
		},
		Left:  lhs,
		Type:  type_,
		Right: rhs,
	})
}

func (p *parser) parseLocalVariableDeclarations(varKeywordBase ast.NodeBase) *ast.LocalVariableDeclarations {
	p.panicIfContextDone()

	p.tokens = append(p.tokens, ast.Token{Type: ast.VAR_KEYWORD, Span: varKeywordBase.Span})

	var (
		start = varKeywordBase.Span.Start
	)

	p.eatSpace()
	var (
		declarations []*ast.LocalVariableDeclarator
		parsingErr   *sourcecode.ParsingError
	)

	if p.i >= p.len || p.s[p.i] == '\n' {
		return &ast.LocalVariableDeclarations{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
				false,
			},
		}
	}

	if isAlpha(p.s[p.i]) || p.s[p.i] == '_' || p.s[p.i] == '{' {
		p.parseSingleLocalVarDeclarator(&declarations)
	} else { //multi declarations
		hasOpeninParenthesis := false
		if p.s[p.i] != '(' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_OPENING_PAREN_EXPECTED}
		} else {
			hasOpeninParenthesis = true
			p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}

		p.eatSpaceNewlineComment()

		for p.i < p.len && p.s[p.i] != ')' {

			if p.i < p.len && p.s[p.i] == ')' {
				break
			}

			p.parseSingleLocalVarDeclarator(&declarations)

			if !hasOpeninParenthesis {
				break
			}

			p.eatSpaceNewlineCommaComment()
		}

		if p.i < p.len && p.s[p.i] == ')' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		} else if hasOpeninParenthesis && (p.i >= p.len || p.s[p.i] != ')') {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS_MISSING_CLOSING_PAREN}
		}
	}

	decls := &ast.LocalVariableDeclarations{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Declarations: declarations,
	}

	return decls
}

func (p *parser) parseSingleGlobalVarDeclarator(declarations *[]*ast.GlobalVariableDeclarator) {
	p.panicIfContextDone()

	var declParsingErr *sourcecode.ParsingError

	//LHS
	var lhs ast.Node
	var objectDestructuration *ast.ObjectDestructuration
	var ident *ast.IdentifierLiteral

	if p.s[p.i] == '{' {
		objectDestructuration = p.parseObjectDestructuration()
		lhs = objectDestructuration
	} else {
		lhs, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		var ok bool
		ident, ok = lhs.(*ast.IdentifierLiteral)
		if !ok {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECL_LHS_MUST_BE_AN_IDENT}
		} else if isKeyword(ident.Name) {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
		}
	}

	p.eatSpace()

	//Unterminated

	if p.i >= p.len || (p.s[p.i] != '=' && !isAcceptedFirstVariableTypeAnnotationChar(p.s[p.i])) {
		if ident != nil {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidGlobalVarDeclMissingEqualsSign(ident.Name)}
		} else if objectDestructuration != nil {
			declParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJECT_DESTRUCTURATION_MISSING_EQUAL_SIGN}
		}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &ast.GlobalVariableDeclarator{
			NodeBase: ast.NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				false,
			},
			Left: lhs,
		})
		return
	}

	//Type annotation

	var type_ ast.Node

	if isAcceptedFirstVariableTypeAnnotationChar(p.s[p.i]) {
		prev := p.inPattern
		p.inPattern = true

		type_, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		p.inPattern = prev
		if objectDestructuration != nil && type_.Base().Err == nil {
			type_.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, TYPE_ANNOTATIONS_NOT_ALLOWED_WHEN_DESTRUCTURING_AN_OBJECT}
		}
	}

	p.eatSpace()

	//ast.Equal sign

	//temporary
	if p.i >= p.len || p.s[p.i] != '=' {
		declParsingErr = &sourcecode.ParsingError{MissingEqualsSignInDeclaration, EQUAL_SIGN_MISSING_AFTER_TYPE_ANNOTATION}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &ast.GlobalVariableDeclarator{
			NodeBase: ast.NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				false,
			},
			Left: lhs,
			Type: type_,
		})
		return
	}

	equalSignIndex := p.i
	p.i++
	p.eatSpace()

	//RHS

	rhs, _ := p.parseExpression()
	p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.ASSIGN_EQUAL, Span: NodeSpan{equalSignIndex, equalSignIndex + 1}})

	*declarations = append(*declarations, &ast.GlobalVariableDeclarator{
		NodeBase: ast.NodeBase{
			NodeSpan{lhs.Base().Span.Start, rhs.Base().Span.End},
			declParsingErr,
			false,
		},
		Left:  lhs,
		Type:  type_,
		Right: rhs,
	})
}

func (p *parser) parseGlobalVariableDeclarations(globalVarKeywordBase ast.NodeBase) *ast.GlobalVariableDeclarations {
	p.panicIfContextDone()

	p.tokens = append(p.tokens, ast.Token{Type: ast.GLOBALVAR_KEYWORD, Span: globalVarKeywordBase.Span})

	var (
		start = globalVarKeywordBase.Span.Start
	)

	p.eatSpace()
	var (
		declarations []*ast.GlobalVariableDeclarator
		parsingErr   *sourcecode.ParsingError
	)

	if p.i >= p.len || p.s[p.i] == '\n' {
		return &ast.GlobalVariableDeclarations{
			NodeBase: ast.NodeBase{
				NodeSpan{start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_VAR_DECLS},
				false,
			},
		}
	}

	if isAlpha(p.s[p.i]) || p.s[p.i] == '_' || p.s[p.i] == '{' {
		p.parseSingleGlobalVarDeclarator(&declarations)
	} else {
		//multi declarations
		hasOpeninParenthesis := false
		if p.s[p.i] != '(' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECLS_OPENING_PAREN_EXPECTED}
		} else {
			hasOpeninParenthesis = true
			p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}

		p.eatSpaceNewlineComment()

		for p.i < p.len && p.s[p.i] != ')' {
			if p.i < p.len && p.s[p.i] == ')' {
				break
			}

			p.parseSingleGlobalVarDeclarator(&declarations)

			if !hasOpeninParenthesis {
				break
			}

			p.eatSpaceNewlineCommaComment()
		}

		if p.i < p.len && p.s[p.i] == ')' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		} else if hasOpeninParenthesis && (p.i >= p.len || p.s[p.i] != ')') {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_VAR_DECLS_MISSING_CLOSING_PAREN}
		}
	}

	decls := &ast.GlobalVariableDeclarations{
		NodeBase: ast.NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			false,
		},
		Declarations: declarations,
	}

	return decls
}

func (p *parser) parseObjectDestructuration() *ast.ObjectDestructuration {
	objectDestructuration := &ast.ObjectDestructuration{
		NodeBase: ast.NodeBase{Span: NodeSpan{p.i, p.i + 1}},
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	p.eatSpaceNewlineCommaComment()

	for p.i < p.len && (p.s[p.i] != '}' && p.s[p.i] != '=') {
		if !IsFirstIdentChar(p.s[p.i]) {
			p.tokens = append(p.tokens, ast.Token{
				Type: ast.UNEXPECTED_CHAR,
				Raw:  string(p.s[p.i]),
				Span: NodeSpan{p.i, p.i + 1},
			})

			objectDestructuration.Properties = append(objectDestructuration.Properties, &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, p.i + 1},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectDestructuration(p.s[p.i])},
				},
			})
			p.i++
		} else {
			identStart := p.i
			p.i++
			for p.i < p.len && IsIdentChar(p.s[p.i]) {
				p.i++
			}

			ident := &ast.IdentifierLiteral{
				NodeBase: ast.NodeBase{Span: NodeSpan{identStart, p.i}},
				Name:     string(p.s[identStart:p.i]),
			}

			prop := &ast.ObjectDestructurationProperty{
				NodeBase:     ident.NodeBase,
				PropertyName: ident,
			}
			objectDestructuration.Properties = append(objectDestructuration.Properties, prop)

			spaceCount := p.eatSpace()

			if p.i < p.len && p.s[p.i] == '?' {
				if spaceCount == 0 {
					prop.Nillable = true
				} else {
					p.tokens = append(p.tokens, ast.Token{
						Type: ast.UNEXPECTED_CHAR,
						Raw:  string(p.s[p.i]),
						Span: NodeSpan{p.i, p.i + 1},
					})
					prop.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNEXPECTED_SPACE_BETWEEN_PROPERTY_NAME_AND_QUESTION_MARK}
				}
				p.i++
				prop.Span.End = p.i
			}

			if p.i < p.len-2 && p.s[p.i] == 'a' && p.s[p.i+1] == 's' && (p.i == p.len-1 || !IsIdentChar(p.s[p.i+2])) {
				p.tokens = append(p.tokens, ast.Token{
					Type: ast.AS_KEYWORD,
					Span: NodeSpan{p.i, p.i + 2},
				})
				p.i += 2

				p.eatSpace()
				prop.Span.End = p.i

				if p.i < p.len && IsFirstIdentChar(p.s[p.i]) {
					identStart := p.i
					p.i++
					for p.i < p.len && IsIdentChar(p.s[p.i]) {
						p.i++
					}

					prop.NewName = &ast.IdentifierLiteral{
						NodeBase: ast.NodeBase{Span: NodeSpan{identStart, p.i}},
						Name:     string(p.s[identStart:p.i]),
					}
					prop.Span.End = p.i
				} else {
					prop.Err = &sourcecode.ParsingError{UnspecifiedParsingError, MISSING_NEW_NAME_AFTER_AS_KEYWORD}
				}
			}
			p.eatSpaceNewlineCommaComment()
		}
	}

	if p.i >= p.len || p.s[p.i] != '}' {
		objectDestructuration.Err = &sourcecode.ParsingError{UnterminatedObjectDestructuration, UNTERMINATED_OBJECT_DESTRUCTURATION_MISSING_CLOSING_BRACE}
	} else {
		p.tokens = append(p.tokens, ast.Token{
			Type: ast.CLOSING_CURLY_BRACKET,
			Span: NodeSpan{p.i, p.i + 1},
		})
		p.i++
	}

	objectDestructuration.Span.End = p.i
	return objectDestructuration
}

func (p *parser) parseEmbeddedModule() *ast.EmbeddedModule {
	p.panicIfContextDone()

	start := p.i
	p.i++

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{start, start + 1}})

	firstInnerTokenIndex := len(p.tokens)

	var (
		emod             = &ast.EmbeddedModule{}
		prevStmtEndIndex = int32(-1)
		prevStmtErrKind  string
		stmts            []ast.Node
		regionHeaders    []*ast.AnnotatedRegionHeader
	)

	p.eatSpaceNewlineCommaComment()
	manifest := p.parseManifestIfPresent()

	p.eatSpaceNewlineSemicolonComment()

	for p.i < p.len && p.s[p.i] != '}' {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			stmts = append(stmts, &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, p.i + 1},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i])},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment()
			continue
		}

		var stmtErr *sourcecode.ParsingError
		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &sourcecode.ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		annotations, moveForward := p.parseMetadaAnnotationsBeforeStatement(&stmts, &regionHeaders)

		if !moveForward {
			break
		}

		stmt := p.parseStatement()
		prevStmtEndIndex = p.i

		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.Kind
		}

		if missingStmt := p.addAnnotationsToNodeIfPossible(annotations, stmt); missingStmt != nil {
			stmts = append(stmts, missingStmt)
		}

		if _, isMissingExpr := stmt.(*ast.MissingExpression); isMissingExpr {
			if isMissingExpr {
				p.i++

				if p.i >= p.len {
					stmts = append(stmts, stmt)
					break
				}
			}
		}

		if stmtErr != nil {
			stmt.BasePtr().Err = stmtErr
		}

		stmts = append(stmts, stmt)
		p.eatSpaceNewlineSemicolonComment()
	}

	var embeddedModuleErr *sourcecode.ParsingError
	hasClosingBracket := false

	if p.i >= p.len || p.s[p.i] != '}' {
		embeddedModuleErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_EMBEDDED_MODULE}
	} else {
		hasClosingBracket = true
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	emod.Manifest = manifest
	emod.RegionHeaders = regionHeaders
	emod.Statements = stmts
	emod.NodeBase = ast.NodeBase{
		NodeSpan{start, p.i},
		embeddedModuleErr,
		false,
	}
	//add tokens
	if firstInnerTokenIndex < len(p.tokens) {
		end := len(p.tokens)
		if hasClosingBracket {
			end--
		}
		emod.Tokens = p.tokens[firstInnerTokenIndex:end]
	}

	return emod
}

func (p *parser) parseSpawnExpression(goIdent ast.Node) ast.Node {
	p.panicIfContextDone()

	spawnExprStart := goIdent.Base().Span.Start
	p.tokens = append(p.tokens, ast.Token{Type: ast.GO_KEYWORD, Span: goIdent.Base().Span})

	p.eatSpace()
	if p.i >= p.len {
		return &ast.SpawnExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{spawnExprStart, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_GO_KEYWORD},
				false,
			},
		}
	}

	meta, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
	var e ast.Node
	p.eatSpace()

	if ident, ok := meta.(*ast.IdentifierLiteral); ok && ident.Name == "do" {
		p.tokens = append(p.tokens, ast.Token{Type: ast.DO_KEYWORD, Span: ident.Span})
		meta = nil
		goto parse_embedded_module
	}

	e, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
	p.eatSpace()

	if ident, ok := e.(*ast.IdentifierLiteral); ok && ident.Name == "do" {
		p.tokens = append(p.tokens, ast.Token{Type: ast.DO_KEYWORD, Span: ident.Span})
	} else {
		return &ast.SpawnExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{spawnExprStart, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_DO_KEYWORD_AFTER_META},
				false,
			},
			Meta: meta,
		}
	}

parse_embedded_module:
	p.eatSpace()

	var emod *ast.EmbeddedModule

	if p.i >= p.len {
		return &ast.SpawnExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{spawnExprStart, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_DO_KEYWORD},
				false,
			},
			Meta: meta,
		}
	}

	if p.s[p.i] == '{' {
		emod = p.parseEmbeddedModule()
	} else {
		expr, _ := p.parseExpression()

		var embeddedModuleErr *sourcecode.ParsingError

		if call, ok := expr.(*ast.CallExpression); !ok {
			embeddedModuleErr = &sourcecode.ParsingError{UnspecifiedParsingError, SPAWN_EXPR_ONLY_SIMPLE_CALLS_ARE_SUPPORTED}
		} else {
			switch call.Callee.(type) {
			case *ast.IdentifierLiteral, *ast.IdentifierMemberExpression:
			default:
				embeddedModuleErr = &sourcecode.ParsingError{UnspecifiedParsingError, SPAWN_EXPR_ONLY_SIMPLE_CALLS_ARE_SUPPORTED}
			}
		}

		emod = &ast.EmbeddedModule{}
		emod.NodeBase.Span = expr.Base().Span
		emod.Err = embeddedModuleErr
		emod.Statements = []ast.Node{expr}
		emod.SingleCallExpr = true
	}

	return &ast.SpawnExpression{
		NodeBase: ast.NodeBase{Span: NodeSpan{spawnExprStart, p.i}},
		Meta:     meta,
		Module:   emod,
	}
}

func (p *parser) parseMappingExpression(mappingIdent ast.Node) *ast.MappingExpression {
	p.panicIfContextDone()

	start := mappingIdent.Base().Span.Start
	p.eatSpace()
	p.tokens = append(p.tokens, ast.Token{Type: ast.MAPPING_KEYWORD, Span: mappingIdent.Base().Span})

	if p.i >= p.len || p.s[p.i] != '{' {
		return &ast.MappingExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_BODY},
			},
		}
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	p.eatSpaceNewlineComment()
	var entries []ast.Node

	for p.i < p.len && p.s[p.i] != '}' && !isClosingDelim(p.s[p.i]) {
		key, isMissingExpr := p.parseExpression()

		if p.i < p.len && isMissingExpr {
			char := p.s[p.i]
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(char)})
			key = &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i, p.i + 1},
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInMappingExpression(char)},
					false,
				},
			}
			p.i++
		}

		dynamicEntryVar, isDynamicEntry := key.(*ast.IdentifierLiteral)
		var entryParsingErr *sourcecode.ParsingError
		if isDynamicEntry && isKeyword(dynamicEntryVar.Name) {
			entryParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
		}

		p.eatSpace()

		if p.i >= p.len || isClosingDelim(p.s[p.i]) {
			if entryParsingErr == nil {
				entryParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_ENTRY}
			}

			if isDynamicEntry {
				entries = append(entries, &ast.DynamicMappingEntry{
					NodeBase: ast.NodeBase{
						Span: dynamicEntryVar.Base().Span,
						Err:  entryParsingErr,
					},
					KeyVar: dynamicEntryVar,
				})
			} else {
				entries = append(entries, &ast.StaticMappingEntry{
					NodeBase: ast.NodeBase{
						Span: key.Base().Span,
						Err:  entryParsingErr,
					},
					Key: key,
				})
			}
			break
		}

		var (
			value                 ast.Node
			groupMatchingVariable ast.Node
		)

		if isDynamicEntry {
			key, isMissingExpr = p.parseExpression()

			if p.i < p.len && isMissingExpr {
				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
				key = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						NodeSpan{p.i, p.i + 1},
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInMappingExpression(p.s[p.i])},
						false,
					},
				}
				p.i++
			}

			p.eatSpace()

			if p.i < p.len && (isAlpha(p.s[p.i]) || p.s[p.i] == '_') {
				groupMatchingVariable = p.parseIdentStartingExpression(false)
				ident, ok := groupMatchingVariable.(*ast.IdentifierLiteral)

				if !ok && groupMatchingVariable.Base().Err == nil {
					groupMatchingVariable.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_DYNAMIC_MAPPING_ENTRY_GROUP_MATCHING_VAR_EXPECTED}
				}

				if ok && isKeyword(ident.Name) {
					entryParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
				}
			}
		}

		end := p.i
		p.eatSpace()

		if p.i < p.len-1 && p.s[p.i] == '=' && p.s[p.i+1] == '>' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.ARROW, Span: NodeSpan{p.i, p.i + 2}})
			p.i += 2
			p.eatSpace()

			value, _ = p.parseExpression()
		}

		if value != nil {
			end = value.Base().Span.End
		} else {
			entryParsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_ENTRY_MISSING_ARROW_VALUE}
		}

		if !isDynamicEntry {
			entries = append(entries, &ast.StaticMappingEntry{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{key.Base().Span.Start, end},
					Err:  entryParsingErr,
				},
				Key:   key,
				Value: value,
			})
		} else {
			entries = append(entries, &ast.DynamicMappingEntry{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{dynamicEntryVar.Base().Span.Start, end},
					Err:  entryParsingErr,
				},
				Key:                   key,
				KeyVar:                dynamicEntryVar,
				GroupMatchingVariable: groupMatchingVariable,
				ValueComputation:      value,
			})
		}

		p.eatSpaceNewlineComment()
	}

	var parsingErr *sourcecode.ParsingError
	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	} else {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE}
	}

	return &ast.MappingExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Entries: entries,
	}
}

func (p *parser) parseComputeExpression(compIdent ast.Node) *ast.ComputeExpression {
	p.panicIfContextDone()

	start := compIdent.Base().Span.Start
	p.eatSpace()

	arg, _ := p.parseExpression()
	p.tokens = append(p.tokens, ast.Token{Type: ast.COMP_KEYWORD, Span: compIdent.Base().Span})

	return &ast.ComputeExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
		},
		Arg: arg,
	}
}

func (p *parser) parseTreedataLiteral(treedataIdent ast.Node) *ast.TreedataLiteral {
	start := treedataIdent.Base().Span.Start
	p.tokens = append(p.tokens, ast.Token{Type: ast.TREEDATA_KEYWORD, Span: treedataIdent.Base().Span})

	p.eatSpace()

	root, _ := p.parseExpression()
	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '{' {
		return &ast.TreedataLiteral{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Root: root,
		}
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

	p.i++
	p.eatSpaceNewlineCommaComment()
	var children []*ast.TreedataEntry

	for p.i < p.len && p.s[p.i] != '}' { //
		entry, cont := p.parseTreeStructureEntry()
		if entry != nil {
			children = append(children, entry)
		}

		if !cont {
			break
		}

		p.eatSpaceNewlineCommaComment()
	}

	var parsingErr *sourcecode.ParsingError
	if p.i < p.len && p.s[p.i] == '}' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	} else {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_TREEDATA_LIT_MISSING_CLOSING_BRACE}
	}

	return &ast.TreedataLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Root:     root,
		Children: children,
	}
}

func (p *parser) parseTreeStructureEntry() (entry *ast.TreedataEntry, cont bool) {
	p.panicIfContextDone()

	start := p.i

	node, isMissingExpr := p.parseExpression()
	p.eatSpace()

	if p.i < p.len && isMissingExpr {
		char := p.s[p.i]
		if isClosingDelim(char) {
			cont = false
			return
		}
		p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(char)})
		node = &ast.UnknownNode{
			NodeBase: ast.NodeBase{
				node.Base().Span,
				&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInTreedataLiteral(char)},
				false,
			},
		}
		p.i++
		return &ast.TreedataEntry{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Value: node,
		}, true
	}

	if p.i >= p.len {
		return &ast.TreedataEntry{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_TREEDATA_ENTRY},
			},
			Value: node,
		}, false
	}

	if p.s[p.i] != '{' { //leaf
		if p.s[p.i] == ':' { //pair
			p.tokens = append(p.tokens, ast.Token{Type: ast.COLON, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			p.eatSpace()

			key := node
			value, _ := p.parseExpression()
			end := p.i
			p.eatSpace()

			base := ast.NodeBase{Span: NodeSpan{start, end}}

			return &ast.TreedataEntry{
				NodeBase: base,
				Value: &ast.TreedataPair{
					NodeBase: base,
					Key:      key,
					Value:    value,
				},
			}, true
		}

		return &ast.TreedataEntry{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Value: node,
		}, true
	}

	p.i++
	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i - 1, p.i}})
	var children []*ast.TreedataEntry

	p.eatSpaceNewlineComment()

	for p.i < p.len && p.s[p.i] != '}' { //
		entry, cont := p.parseTreeStructureEntry()
		children = append(children, entry)

		if !cont {
			return &ast.TreedataEntry{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{start, p.i},
				},
				Value:    node,
				Children: children,
			}, false
		}

		p.eatSpaceNewlineCommaComment()
	}

	var parsingErr *sourcecode.ParsingError
	if p.i >= p.len || p.s[p.i] != '}' {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_TREEDATA_ENTRY_MISSING_CLOSING_BRACE}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}
	return &ast.TreedataEntry{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Value:    node,
		Children: children,
	}, true
}

func (p *parser) parseConcatenationExpression(concatIdent ast.Node, precededByOpeningParen bool) *ast.ConcatenationExpression {
	p.panicIfContextDone()

	start := concatIdent.Base().Span.Start
	p.tokens = append(p.tokens, ast.Token{Type: ast.CONCAT_KEYWORD, Span: concatIdent.Base().Span})
	var elements []ast.Node

	if precededByOpeningParen {
		p.eatSpaceNewlineComment()
	} else {
		p.eatSpace()
	}

	for p.i < p.len && !isUnpairedOrIsClosingDelim(p.s[p.i]) {

		var elem ast.Node

		//spread element
		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			spreadStart := p.i
			threeDotsSpan := NodeSpan{p.i, p.i + 3}
			p.i += 3

			e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
			p.tokens = append(p.tokens, ast.Token{Type: ast.THREE_DOTS, Span: threeDotsSpan})

			elem = &ast.ElementSpreadElement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{spreadStart, e.Base().Span.End},
				},
				Expr: e,
			}

		} else {
			e, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

			if isMissingExpr {
				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
				elem = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						e.Base().Span,
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInConcatenationExpression(p.s[p.i])},
						false,
					},
				}
				p.i++
			} else {
				elem = e
			}
		}

		elements = append(elements, elem)
		if precededByOpeningParen {
			p.eatSpaceNewlineComment()
		} else {
			p.eatSpace()
		}
	}

	var parsingErr *sourcecode.ParsingError
	if len32(elements) == 0 {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_CONCAT_EXPR_ELEMS_EXPECTED}
	}

	return &ast.ConcatenationExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Elements: elements,
	}
}

func (p *parser) parseTestSuiteExpression(ident *ast.IdentifierLiteral) *ast.TestSuiteExpression {
	p.panicIfContextDone()

	start := ident.Base().Span.Start
	p.tokens = append(p.tokens, ast.Token{Type: ast.TESTSUITE_KEYWORD, Span: ident.Base().Span})

	p.eatSpace()
	if p.i >= p.len {
		return &ast.TestSuiteExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
			},
		}
	}

	var meta ast.Node

	if p.s[p.i] != '{' {
		meta, _ = p.parseExpression()
		p.eatSpace()
	}

	if p.i >= p.len || p.s[p.i] != '{' {
		return &ast.TestSuiteExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
			},
			Meta: meta,
		}
	}

	emod := p.parseEmbeddedModule()

	return &ast.TestSuiteExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
		},
		Meta:   meta,
		Module: emod,
	}

}

func (p *parser) parseTestCaseExpression(ident *ast.IdentifierLiteral) *ast.TestCaseExpression {
	p.panicIfContextDone()

	start := ident.Base().Span.Start
	p.tokens = append(p.tokens, ast.Token{Type: ast.TESTCASE_KEYWORD, Span: ident.Base().Span})

	p.eatSpace()
	if p.i >= p.len {
		return &ast.TestCaseExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{MissingBlock, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
			},
		}
	}

	var meta ast.Node

	if p.s[p.i] != '{' {
		meta, _ = p.parseExpression()
		p.eatSpace()
	}

	if p.i >= p.len || p.s[p.i] != '{' {
		return &ast.TestCaseExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
			},
			Meta: meta,
		}
	}

	emod := p.parseEmbeddedModule()

	return &ast.TestCaseExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
		},
		Meta:   meta,
		Module: emod,
	}
}

func (p *parser) parseReceptionHandlerExpression(onIdent ast.Node) ast.Node {
	p.panicIfContextDone()

	exprStart := onIdent.Base().Span.Start
	p.tokens = append(p.tokens, ast.Token{Type: ast.ON_KEYWORD, Span: onIdent.Base().Span})

	p.eatSpace()
	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		return &ast.ReceptionHandlerExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{exprStart, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_RECEIVED_KEYWORD},
				false,
			},
		}
	}

	e, _ := p.parseExpression()
	p.eatSpace()

	var missingReceivedKeywordError *sourcecode.ParsingError

	if ident, ok := e.(*ast.IdentifierLiteral); ok && ident.Name == "received" {
		p.tokens = append(p.tokens, ast.Token{Type: ast.RECEIVED_KEYWORD, Span: ident.Span})
		e = nil
	} else {
		missingReceivedKeywordError = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_RECEP_HANDLER_MISSING_RECEIVED_KEYWORD}
	}

	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		return &ast.ReceptionHandlerExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{exprStart, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_PATTERN},
				false,
			},
		}
	}

	pattern, _ := p.parseExpression()
	p.eatSpace()

	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		return &ast.ReceptionHandlerExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{exprStart, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_HANDLER_OR_PATTERN},
				false,
			},
			Pattern: pattern,
		}
	}

	handler, _ := p.parseExpression()
	p.eatSpace()

	return &ast.ReceptionHandlerExpression{
		NodeBase: ast.NodeBase{Span: NodeSpan{exprStart, p.i}, Err: missingReceivedKeywordError},
		Pattern:  pattern,
		Handler:  handler,
	}
}

func (p *parser) parseSendValueExpression(ident *ast.IdentifierLiteral) *ast.SendValueExpression {
	p.panicIfContextDone()

	start := ident.Base().Span.Start
	p.tokens = append(p.tokens, ast.Token{Type: ast.SENDVAL_KEYWORD, Span: ident.Base().Span})

	p.eatSpace()
	if p.isExpressionEnd() {
		return &ast.SendValueExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_SENDVALUE_EXPRESSION_MISSING_VALUE},
			},
		}
	}

	value, _ := p.parseExpression()
	p.eatSpace()

	e, _ := p.parseExpression()
	p.eatSpace()

	var receiver ast.Node
	var parsingErr *sourcecode.ParsingError

	if ident, ok := e.(*ast.IdentifierLiteral); !ok || ident.Name != "to" {
		receiver = e
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_SENDVALUE_EXPRESSION_MISSING_TO_KEYWORD_BEFORE_RECEIVER}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.TO_KEYWORD, Span: ident.Span})

		receiver, _ = p.parseExpression()
		p.eatSpace()
	}

	return &ast.SendValueExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Value:    value,
		Receiver: receiver,
	}
}

// tryParseCall tries to parse a call or return nil (calls with parsing errors are returned)
func (p *parser) tryParseCall(callee ast.Node, firstNameIfIdentOnTheLeft string) *ast.CallExpression {
	p.panicIfContextDone()

	switch {
	case p.s[p.i] == '"': //func_name"string"
		call := &ast.CallExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{callee.Base().Span.Start, 0},
			},
			Callee:    callee,
			Arguments: nil,
			Must:      true,
		}

		str, _ := p.parseExpression()
		call.Arguments = append(call.Arguments, str)
		call.NodeBase.Span.End = str.Base().Span.End
		return call
	case p.s[p.i] == '{': //func_name{key: "value"}
		call := &ast.CallExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{callee.Base().Span.Start, 0},
			},
			Callee:    callee,
			Arguments: nil,
			Must:      true,
		}

		str, _ := p.parseExpression()
		call.Arguments = append(call.Arguments, str)
		call.NodeBase.Span.End = str.Base().Span.End
		return call
	case !isKeyword(firstNameIfIdentOnTheLeft) && (p.s[p.i] == '(' || (p.s[p.i] == '!' && p.i < p.len-1 && p.s[p.i+1] == '(')): //func_name(...

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
		p.eatSpace()

		call := &ast.CallExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{callee.Base().Span.Start, 0},
				nil,
				false,
			},
			Callee:    callee,
			Arguments: nil,
			Must:      must,
		}

		return p.parseParenthesizedCallArgs(call)
	}

	return nil
}

// parseFunction parses function declarations and function expressions
func (p *parser) parseFunction(start int32) ast.Node {
	p.panicIfContextDone()

	p.tokens = append(p.tokens, ast.Token{Type: ast.FN_KEYWORD, Span: NodeSpan{p.i - 2, p.i}})
	p.eatSpace()

	var (
		funcName       ast.Node
		funcNameIdent  *ast.IdentifierLiteral
		parsingErr     *sourcecode.ParsingError
		capturedLocals []ast.Node
		hasCaptureList = false
	)

	createNodeWithError := func() ast.Node {
		fn := ast.FunctionExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{start, p.i},
			},
			CaptureList: capturedLocals,
		}

		if funcName != nil {
			if parsingErr == nil && funcNameIdent != nil && isKeyword(funcNameIdent.Name) {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_FN_NAMES}
			}
			return &ast.FunctionDeclaration{
				NodeBase: ast.NodeBase{
					Span: fn.Span,
					Err:  parsingErr,
				},
				Function: &fn,
				Name:     funcName,
			}
		}
		fn.Err = parsingErr
		return &fn
	}

	//parse capture list
	if p.i < p.len && p.s[p.i] == '[' {
		hasCaptureList = true
		p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		for p.i < p.len && p.s[p.i] != ']' {
			e, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

			if isMissingExpr && p.i >= p.len {
				break
			}

			if isMissingExpr {
				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
				e = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						e.Base().Span,
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCaptureList(p.s[p.i])},
						false,
					},
				}
				p.i++
			} else {
				if _, ok := e.(*ast.IdentifierLiteral); !ok && e.Base().Err == nil {
					e.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, CAPTURE_LIST_SHOULD_ONLY_CONTAIN_IDENTIFIERS}
				}
			}

			capturedLocals = append(capturedLocals, e)
			p.eatSpaceComma()
		}

		if p.i >= p.len {
			parsingErr = &sourcecode.ParsingError{InvalidNext, UNTERMINATED_CAPTURE_LIST_MISSING_CLOSING_BRACKET}
			return createNodeWithError()
		} else {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}

		p.eatSpace()
	}

	//Detect and parse the function's name.

	if p.i < p.len {

		if isAlpha(p.s[p.i]) {
			identLike := p.parseIdentStartingExpression(false)
			var ok bool
			funcNameIdent, ok = identLike.(*ast.IdentifierLiteral)

			if !ok {
				return &ast.FunctionDeclaration{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{start, p.i},
						Err:  &sourcecode.ParsingError{UnspecifiedParsingError, fmtFuncNameShouldBeAnIdentNot(identLike)},
					},
					Function: nil,
					Name:     nil,
				}
			}
			funcName = funcNameIdent
		} else if p.s[p.i] == '<' && p.i < p.len-1 && p.s[p.i+1] == '{' {
			funcName = p.parseUnquotedRegion()
		}
	}

	if p.i >= p.len || p.s[p.i] != '(' {
		if hasCaptureList && funcName == nil {
			parsingErr = &sourcecode.ParsingError{InvalidNext, CAPTURE_LIST_SHOULD_BE_FOLLOWED_BY_PARAMS}
		} else {
			parsingErr = &sourcecode.ParsingError{InvalidNext, FN_KEYWORD_OR_FUNC_NAME_SHOULD_BE_FOLLOWED_BY_PARAMS}
		}

		return createNodeWithError()
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	var parameters []*ast.FunctionParameter
	isVariadic := false

	//we parse the parameters
	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewlineComma()
		var paramErr *sourcecode.ParsingError

		if p.i >= p.len || p.s[p.i] == ')' {
			break
		}

		if isVariadic {
			paramErr = &sourcecode.ParsingError{UnspecifiedParsingError, VARIADIC_PARAM_IS_UNIQUE_AND_SHOULD_BE_LAST_PARAM}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			isVariadic = true
			p.i += 3
		}

		varNode, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
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
			p.eatSpace()

			{
				prev := p.inPattern
				p.inPattern = true

				typ, isMissingExpr = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

				p.inPattern = prev
			}

			if isMissingExpr {
				typ = nil
			}

			span := varNode.Base().Span

			if typ != nil {
				span.End = typ.Base().Span.End
			}

			if ident, ok := varNode.(*ast.IdentifierLiteral); ok {
				if paramErr == nil && isKeyword(ident.Name) {
					paramErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES}
				}
			} else if _, ok := varNode.(*ast.UnquotedRegion); !ok {
				varNode.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, PARAM_LIST_OF_FUNC_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS}

				if typ != nil {
					typ.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, PARAM_LIST_OF_FUNC_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS}
				}
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
		}

		p.eatSpaceNewlineComma()
	}

	var (
		returnType       ast.Node
		body             ast.Node
		isBodyExpression bool
		end              int32
	)

	if p.i >= p.len {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN}
		end = p.i
	} else if p.s[p.i] != ')' {
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FUNC_SYNTAX}
		end = p.i
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		if p.i < p.len && isAcceptedReturnTypeStart(p.s, p.i) {
			prev := p.inPattern
			p.inPattern = true

			returnType, _ = p.parseExpression(exprParsingConfig{
				disallowUnparenthesizedBinForPipelineExprs: true,
				disallowParsingSeveralPatternUnionCases:    true,
			})

			p.inPattern = prev
		}

		p.eatSpace()

		var error = &sourcecode.ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW_OR_TYPE}
		if returnType != nil {
			error = &sourcecode.ParsingError{UnspecifiedParsingError, RETURN_TYPE_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW}
		}

		if p.i >= p.len || p.s[p.i] == '\n' {
			error.Kind = MissingFnBody
			parsingErr = error
			end = p.i
		} else {
			switch p.s[p.i] {
			case '{':
				body = p.parseBlock()
				end = body.Base().Span.End
			case '=':
				if p.i >= p.len-1 || p.s[p.i+1] != '>' {
					error.Kind = MissingFnBody
					parsingErr = error
					end = p.i
				} else {
					p.tokens = append(p.tokens, ast.Token{Type: ast.ARROW, Span: NodeSpan{p.i, p.i + 2}})
					p.i += 2
					p.eatSpace()
					body, _ = p.parseExpression()
					end = body.Base().Span.End
					isBodyExpression = true
				}
			default:
				parsingErr = error
				end = p.i
			}
		}

	}

	fn := ast.FunctionExpression{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, end},
			Err:  parsingErr,
		},
		CaptureList:      capturedLocals,
		Parameters:       parameters,
		ReturnType:       returnType,
		IsVariadic:       isVariadic,
		Body:             body,
		IsBodyExpression: isBodyExpression,
	}

	if funcName != nil {
		fn.Err = nil

		if parsingErr == nil && funcNameIdent != nil && isKeyword(funcNameIdent.Name) {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_FN_NAMES}
		}

		return &ast.FunctionDeclaration{
			NodeBase: ast.NodeBase{
				Span: fn.Span,
				Err:  parsingErr,
			},
			Function: &fn,
			Name:     funcName,
		}
	}

	return &fn
}

func (p *parser) parseIfStatement(ifIdent *ast.IdentifierLiteral) *ast.IfStatement {
	p.panicIfContextDone()

	var alternate ast.Node
	var blk *ast.Block
	var end int32
	var parsingErr *sourcecode.ParsingError

	p.tokens = append(p.tokens, ast.Token{Type: ast.IF_KEYWORD, Span: ifIdent.Span})

	p.eatSpace()
	test, _ := p.parseExpression()
	p.eatSpace()

	if p.i >= p.len {
		end = p.i
		parsingErr = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_IF_STMT_MISSING_BLOCK}
	} else if p.s[p.i] != '{' {
		end = p.i
		parsingErr = &sourcecode.ParsingError{MissingBlock, fmtUnterminatedIfStmtShouldBeFollowedByBlock(p.s[p.i])}
	} else {
		blk = p.parseBlock()
		end = blk.Span.End
		p.eatSpace()

		if p.i < p.len-3 && p.s[p.i] == 'e' && p.s[p.i+1] == 'l' && p.s[p.i+2] == 's' && p.s[p.i+3] == 'e' {
			p.tokens = append(p.tokens, ast.Token{
				Type: ast.ELSE_KEYWORD,
				Span: NodeSpan{p.i, p.i + 4},
			})
			p.i += 4
			p.eatSpace()

			switch {
			case p.i >= p.len:
				parsingErr = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_IF_STMT_MISSING_BLOCK_AFTER_ELSE}
			case p.s[p.i] == '{':
				alternate = p.parseBlock()
				end = alternate.(*ast.Block).Span.End
			case p.i < p.len-1 && p.s[p.i] == 'i' && p.s[p.i+1] == 'f' && (p.i >= p.len-2 || !IsIdentChar(p.s[p.i+2])):
				ident, _ := p.parseExpression()
				alternate = p.parseIfStatement(ident.(*ast.IdentifierLiteral))
				end = alternate.(*ast.IfStatement).Span.End
			default:
				parsingErr = &sourcecode.ParsingError{MissingBlock, fmtUnterminatedIfStmtElseShouldBeFollowedByBlock(p.s[p.i])}
			}
		}
	}

	return &ast.IfStatement{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{ifIdent.Span.Start, end},
			Err:  parsingErr,
		},
		Test:       test,
		Consequent: blk,
		Alternate:  alternate,
	}
}

func (p *parser) parseForStatement(forIdent *ast.IdentifierLiteral) *ast.ForStatement {
	p.panicIfContextDone()

	var parsingErr *sourcecode.ParsingError
	var valuePattern ast.Node
	var valueElemIdent *ast.IdentifierLiteral
	var keyPattern ast.Node
	var keyIndexIdent *ast.IdentifierLiteral
	p.eatSpace()

	var firstPattern ast.Node
	var first ast.Node
	chunked := false
	p.tokens = append(p.tokens, ast.Token{Type: ast.FOR_KEYWORD, Span: forIdent.Span})

	if p.i < p.len && p.s[p.i] == '%' {
		firstPattern = p.parsePercentPrefixedPattern(false)
		p.eatSpace()

		if p.i < p.len && p.s[p.i] == '{' {
			blk := p.parseBlock()
			end := p.i

			return &ast.ForStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{forIdent.Span.Start, end},
					Err:  parsingErr,
				},
				KeyIndexIdent:  nil,
				ValueElemIdent: nil,
				Body:           blk,
				IteratedValue:  firstPattern,
			}
		}
		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		first = e
	} else {
		first, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		if ident, ok := first.(*ast.IdentifierLiteral); ok && !ident.IsParenthesized && ident.Name == "chunked" {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CHUNKED_KEYWORD, Span: ident.Span})
			chunked = true
			p.eatSpace()
			first, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		}
	}

	switch v := first.(type) {
	case *ast.IdentifierLiteral: //for ... in ...
		p.eatSpace()

		var badValueElemIdent ast.Node

		if p.i >= p.len {
			return &ast.ForStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{forIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT},
				},
				Chunked:       chunked,
				KeyPattern:    firstPattern,
				KeyIndexIdent: v,
			}
		}

		//if not directly followed by "in"
		if p.i >= p.len-1 || p.s[p.i] != 'i' || p.s[p.i+1] != 'n' {
			keyIndexIdent = v
			keyPattern = firstPattern

			if p.s[p.i] != ',' {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtForStmtKeyIndexShouldBeFollowedByCommaNot(p.s[p.i])}
			}

			p.tokens = append(p.tokens, ast.Token{Type: ast.COMMA, Span: NodeSpan{p.i, p.i + 1}})

			p.i++
			p.eatSpace()

			if p.i >= p.len {
				return &ast.ForStatement{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{forIdent.Span.Start, p.i},
						Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_FOR_STMT},
					},
					Chunked:           chunked,
					KeyPattern:        firstPattern,
					KeyIndexIdent:     v,
					BadValueElemIdent: badValueElemIdent,
				}
			}

			if p.s[p.i] == '%' {
				valuePattern = p.parsePercentPrefixedPattern(false)
				p.eatSpace()
			}

			e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

			if ident, isVar := e.(*ast.IdentifierLiteral); !isVar {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidForStmtKeyIndexVarShouldBeFollowedByVarNot(keyIndexIdent)}
				badValueElemIdent = e
			} else {
				valueElemIdent = ident
			}

			p.eatSpace()

			if p.i >= p.len {
				return &ast.ForStatement{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{forIdent.Span.Start, p.i},
						Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_FOR_STMT},
					},
					KeyPattern:        firstPattern,
					KeyIndexIdent:     v,
					ValuePattern:      valuePattern,
					BadValueElemIdent: badValueElemIdent,
					Chunked:           chunked,
				}
			}

			if p.s[p.i] != 'i' || p.i > p.len-2 || p.s[p.i+1] != 'n' {
				return &ast.ForStatement{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{forIdent.Span.Start, p.i},
						Err:  &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT_MISSING_IN_KEYWORD},
					},
					KeyPattern:        keyPattern,
					KeyIndexIdent:     keyIndexIdent,
					ValuePattern:      valuePattern,
					ValueElemIdent:    valueElemIdent,
					BadValueElemIdent: badValueElemIdent,
					Chunked:           chunked,
				}
			}

		} else { //if directly followed by "in"
			valueElemIdent = v
			valuePattern = firstPattern
		}

		p.tokens = append(p.tokens, ast.Token{Type: ast.IN_KEYWORD, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2

		if p.i < p.len && p.s[p.i] != ' ' {

			return &ast.ForStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{forIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT_IN_KEYWORD_SHOULD_BE_FOLLOWED_BY_SPACE},
				},
				KeyPattern:        keyPattern,
				KeyIndexIdent:     keyIndexIdent,
				ValuePattern:      valuePattern,
				ValueElemIdent:    valueElemIdent,
				BadValueElemIdent: badValueElemIdent,
				Chunked:           chunked,
			}
		}
		p.eatSpace()

		if p.i >= p.len {
			return &ast.ForStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{forIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT_MISSING_VALUE_AFTER_IN},
				},
				KeyPattern:        keyPattern,
				KeyIndexIdent:     keyIndexIdent,
				ValuePattern:      valuePattern,
				ValueElemIdent:    valueElemIdent,
				BadValueElemIdent: badValueElemIdent,
				Chunked:           chunked,
			}
		}

		iteratedValue, _ := p.parseExpression()
		p.eatSpace()

		var blk *ast.Block
		var end = p.i

		if p.i >= p.len || p.s[p.i] != '{' {
			parsingErr = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_FOR_STMT_MISSING_BLOCK}
		} else {
			blk = p.parseBlock()
			end = blk.Span.End
		}

		return &ast.ForStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{forIdent.Span.Start, end},
				Err:  parsingErr,
			},
			KeyPattern:        keyPattern,
			KeyIndexIdent:     keyIndexIdent,
			ValueElemIdent:    valueElemIdent,
			ValuePattern:      valuePattern,
			BadValueElemIdent: badValueElemIdent,
			Body:              blk,
			Chunked:           chunked,
			IteratedValue:     iteratedValue,
		}
	default:
		if firstPattern != nil {
			return &ast.ForStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{forIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT},
				},
				Chunked:           chunked,
				ValuePattern:      firstPattern,
				BadValueElemIdent: first,
			}
		}

		p.eatSpace()

		var blk *ast.Block
		end := int32(0)

		if p.i >= p.len || p.s[p.i] != '{' {
			parsingErr = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_FOR_STMT_MISSING_BLOCK}
			end = p.i
		} else {
			blk = p.parseBlock()
			end = p.i
		}

		return &ast.ForStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{forIdent.Span.Start, end},
				Err:  parsingErr,
			},
			KeyIndexIdent:  nil,
			ValueElemIdent: nil,
			Body:           blk,
			IteratedValue:  first,
		}
	}
}

func (p *parser) parseForExpression(openingParenIndex int32 /*-1 if no unparenthesized*/, forKeywordStart int32) *ast.ForExpression {
	p.panicIfContextDone()

	forExprStart := openingParenIndex
	if forExprStart < 0 {
		forExprStart = forKeywordStart
	}
	shouldHaveClosingParen := openingParenIndex >= 0

	var parsingErr *sourcecode.ParsingError
	var valuePattern ast.Node
	var valueElemIdent *ast.IdentifierLiteral
	var keyPattern ast.Node
	var keyIndexIdent *ast.IdentifierLiteral
	p.eatSpace()

	var firstPattern ast.Node
	var first ast.Node
	chunked := false
	p.tokens = append(p.tokens, ast.Token{Type: ast.FOR_KEYWORD, Span: NodeSpan{forKeywordStart, forKeywordStart + int32(len(ast.FOR_KEYWORD_STRING))}})

	if p.i < p.len && p.s[p.i] == '%' {
		firstPattern = p.parsePercentPrefixedPattern(false)
		p.eatSpace()

		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		first = e
	} else {
		first, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		if ident, ok := first.(*ast.IdentifierLiteral); ok && !ident.IsParenthesized && ident.Name == "chunked" {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CHUNKED_KEYWORD, Span: ident.Span})
			chunked = true
			p.eatSpace()
			first, _ = p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
		}
	}

	switch v := first.(type) {
	case *ast.IdentifierLiteral: //for ... in ...
		p.eatSpace()

		var badValueElemIdent ast.Node

		if p.i >= p.len {
			return &ast.ForExpression{
				NodeBase: ast.NodeBase{
					Span:            NodeSpan{forExprStart, p.i},
					Err:             &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_EXPR},
					IsParenthesized: shouldHaveClosingParen,
				},
				Chunked:       chunked,
				KeyPattern:    firstPattern,
				KeyIndexIdent: v,
			}
		}

		//if not directly followed by "in"
		if p.i >= p.len-1 || p.s[p.i] != 'i' || p.s[p.i+1] != 'n' {
			keyIndexIdent = v
			keyPattern = firstPattern

			if p.s[p.i] != ',' {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtForExprKeyIndexShouldBeFollowedByCommaNot(p.s[p.i])}
			}

			p.tokens = append(p.tokens, ast.Token{Type: ast.COMMA, Span: NodeSpan{p.i, p.i + 1}})

			p.i++
			p.eatSpace()

			if p.i >= p.len {
				return &ast.ForExpression{
					NodeBase: ast.NodeBase{
						Span:            NodeSpan{forExprStart, p.i},
						Err:             &sourcecode.ParsingError{UnterminatedForExpr, UNTERMINATED_FOR_EXPR},
						IsParenthesized: shouldHaveClosingParen,
					},
					Chunked:           chunked,
					KeyPattern:        firstPattern,
					KeyIndexIdent:     v,
					BadValueElemIdent: badValueElemIdent,
				}
			}

			if p.s[p.i] == '%' {
				valuePattern = p.parsePercentPrefixedPattern(false)
				p.eatSpace()
			}

			e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

			if ident, isVar := e.(*ast.IdentifierLiteral); !isVar {
				parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidForExprKeyIndexVarShouldBeFollowedByVarNot(keyIndexIdent)}
				badValueElemIdent = e
			} else {
				valueElemIdent = ident
			}

			p.eatSpace()

			if p.i >= p.len {
				return &ast.ForExpression{
					NodeBase: ast.NodeBase{
						Span:            NodeSpan{forExprStart, p.i},
						Err:             &sourcecode.ParsingError{UnterminatedForExpr, UNTERMINATED_FOR_EXPR},
						IsParenthesized: shouldHaveClosingParen,
					},
					KeyPattern:        firstPattern,
					KeyIndexIdent:     v,
					ValuePattern:      valuePattern,
					BadValueElemIdent: badValueElemIdent,
					Chunked:           chunked,
				}
			}

			if p.s[p.i] != 'i' || p.i > p.len-2 || p.s[p.i+1] != 'n' {
				return &ast.ForExpression{
					NodeBase: ast.NodeBase{
						Span:            NodeSpan{forExprStart, p.i},
						Err:             &sourcecode.ParsingError{UnterminatedForExpr, UNTERMINATED_FOR_EXPR_MISSING_IN_KEYWORD},
						IsParenthesized: shouldHaveClosingParen,
					},
					KeyPattern:        keyPattern,
					KeyIndexIdent:     keyIndexIdent,
					ValuePattern:      valuePattern,
					ValueElemIdent:    valueElemIdent,
					BadValueElemIdent: badValueElemIdent,
					Chunked:           chunked,
				}
			}

		} else { //if directly followed by "in"
			valueElemIdent = v
			valuePattern = firstPattern
		}

		p.tokens = append(p.tokens, ast.Token{Type: ast.IN_KEYWORD, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2

		if p.i < p.len && p.s[p.i] != ' ' {

			return &ast.ForExpression{
				NodeBase: ast.NodeBase{
					Span:            NodeSpan{forExprStart, p.i},
					Err:             &sourcecode.ParsingError{UnterminatedForExpr, INVALID_FOR_EXPR_IN_KEYWORD_SHOULD_BE_FOLLOWED_BY_SPACE},
					IsParenthesized: shouldHaveClosingParen,
				},
				KeyPattern:        keyPattern,
				KeyIndexIdent:     keyIndexIdent,
				ValuePattern:      valuePattern,
				ValueElemIdent:    valueElemIdent,
				BadValueElemIdent: badValueElemIdent,
				Chunked:           chunked,
			}
		}
		p.eatSpace()

		if p.i >= p.len {
			return &ast.ForExpression{
				NodeBase: ast.NodeBase{
					Span:            NodeSpan{forExprStart, p.i},
					Err:             &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT_MISSING_VALUE_AFTER_IN},
					IsParenthesized: shouldHaveClosingParen,
				},
				KeyPattern:        firstPattern,
				KeyIndexIdent:     keyIndexIdent,
				ValuePattern:      valuePattern,
				ValueElemIdent:    valueElemIdent,
				BadValueElemIdent: badValueElemIdent,
				Chunked:           chunked,
			}
		}

		iteratedValue, _ := p.parseExpression()
		p.eatSpace()

		var body ast.Node
		var end = p.i

		switch {
		case p.i < p.len-1 && p.s[p.i] == '=' && p.s[p.i+1] == '>':
			p.tokens = append(p.tokens, ast.Token{Type: ast.ARROW, Span: NodeSpan{p.i, p.i + 2}})
			p.i += 2

			p.eatSpaceNewlineComment()

			body, _ = p.parseExpression()
			end = body.Base().Span.End
		case p.i == p.len-1 && p.s[p.i] == '=':
			p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			end = p.i

			parsingErr = &sourcecode.ParsingError{UnterminatedForExpr, UNTERMINATED_FOR_EXPR_MISSING_ARROW_ITEM_OR_BODY}
		case p.i < p.len && p.s[p.i] == '{':
			body = p.parseBlock()
			end = body.Base().Span.End
		default:
			parsingErr = &sourcecode.ParsingError{UnterminatedForExpr, UNTERMINATED_FOR_EXPR_MISSING_ARROW_ITEM_OR_BODY}
		}

		if shouldHaveClosingParen {
			p.eatSpaceNewlineComment()

			if p.i < p.len && p.s[p.i] == ')' {
				p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				end = p.i
			} else {
				parsingErr = &sourcecode.ParsingError{UnterminatedForExpr, UNTERMINATED_FOR_EXPR_MISSING_CLOSIN_PAREN}
			}
		} else {
			end = p.i
		}

		return &ast.ForExpression{
			NodeBase: ast.NodeBase{
				Span:            NodeSpan{forExprStart, end},
				Err:             parsingErr,
				IsParenthesized: shouldHaveClosingParen,
			},
			KeyPattern:        keyPattern,
			KeyIndexIdent:     keyIndexIdent,
			ValueElemIdent:    valueElemIdent,
			ValuePattern:      valuePattern,
			BadValueElemIdent: badValueElemIdent,
			Body:              body,
			Chunked:           chunked,
			IteratedValue:     iteratedValue,
		}
	default:
		return &ast.ForExpression{
			NodeBase: ast.NodeBase{
				Span:            NodeSpan{forExprStart, p.i},
				Err:             &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_FOR_EXPR},
				IsParenthesized: shouldHaveClosingParen,
			},
			Chunked:           chunked,
			ValuePattern:      firstPattern,
			BadValueElemIdent: first,
		}
	}
}

func (p *parser) parseWalkStatement(walkIdent *ast.IdentifierLiteral) *ast.WalkStatement {
	p.panicIfContextDone()

	p.tokens = append(p.tokens, ast.Token{Type: ast.WALK_KEYWORD, Span: walkIdent.Span})

	if p.i >= p.len {
		return &ast.WalkStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{walkIdent.Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnterminatedWalkStmt, UNTERMINATED_WALK_STMT_MISSING_WALKED_VALUE},
			},
		}
	}

	var parsingErr *sourcecode.ParsingError
	var metaIdent, entryIdent *ast.IdentifierLiteral
	p.eatSpace()

	walked, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	if isMissingExpr {
		return &ast.WalkStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{walkIdent.Span.Start, p.i},
			},
			Walked: walked,
		}
	}

	p.eatSpace()
	e, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	var ok bool
	if entryIdent, ok = e.(*ast.IdentifierLiteral); !ok {
		parsingErr = &sourcecode.ParsingError{UnterminatedWalkExpr, UNTERMINATED_WALK_EXPR_MISSING_ENTRY_VARIABLE_NAME}
		if isMissingExpr {
			e = nil
		}

		return &ast.WalkStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{walkIdent.Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnterminatedWalkStmt, UNTERMINATED_WALK_STMT_MISSING_ENTRY_VARIABLE_NAME},
			},
			Walked:        walked,
			BadEntryIdent: e,
		}
	}

	p.eatSpace()

	// if the parsed identifier is instead the meta variable identifier we try to parse the entry variable identifier
	if p.i < p.len && p.s[p.i] == ',' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.COMMA, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		metaIdent = entryIdent
		entryIdent = nil
		p.eatSpace()

		// missing entry identifier
		if p.i >= p.len || p.s[p.i] == '{' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_WALK_STMT_MISSING_ENTRY_IDENTIFIER}
		} else {
			e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
			if entryIdent, ok = e.(*ast.IdentifierLiteral); !ok {
				parsingErr = &sourcecode.ParsingError{UnterminatedWalkExpr, UNTERMINATED_WALK_EXPR_MISSING_ENTRY_VARIABLE_NAME}
				if isMissingExpr {
					e = nil
				}

				return &ast.WalkStatement{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{walkIdent.Span.Start, p.i},
						Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_WALK_STMT_MISSING_ENTRY_VARIABLE_NAME},
					},
					MetaIdent:     metaIdent,
					BadEntryIdent: e,
					Walked:        walked,
				}
			}
			p.eatSpace()
		}
	}

	var blk *ast.Block
	var end int32

	if p.i >= p.len || p.s[p.i] != '{' {
		end = p.i
		parsingErr = &sourcecode.ParsingError{UnterminatedWalkStmt, UNTERMINATED_WALK_STMT_MISSING_BODY}
	} else {
		blk = p.parseBlock()
		end = blk.Span.End
	}

	return &ast.WalkStatement{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{walkIdent.Span.Start, end},
			Err:  parsingErr,
		},
		Walked:     walked,
		MetaIdent:  metaIdent,
		EntryIdent: entryIdent,
		Body:       blk,
	}
}

func (p *parser) parseWalkExpression(openingParenIndex int32 /*-1 if no unparenthesized*/, walkKeywordStart int32) *ast.WalkExpression {
	p.panicIfContextDone()

	walkExprStart := openingParenIndex
	if walkExprStart < 0 {
		walkExprStart = walkKeywordStart
	}
	shouldHaveClosingParen := openingParenIndex >= 0

	p.tokens = append(p.tokens, ast.Token{
		Type: ast.WALK_KEYWORD,
		Span: NodeSpan{
			Start: walkKeywordStart,
			End:   walkKeywordStart + int32(len(ast.WALK_KEYWORD_STRING)),
		},
	})

	p.eatSpace()

	eatClosingParen := func() {
		if shouldHaveClosingParen && p.i < p.len && p.s[p.i] == ')' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	}

	if p.i >= p.len || isClosingDelim(p.s[p.i]) {
		eatClosingParen()

		return &ast.WalkExpression{
			NodeBase: ast.NodeBase{
				Span:            NodeSpan{walkExprStart, p.i},
				Err:             &sourcecode.ParsingError{UnterminatedWalkExpr, UNTERMINATED_WALK_EXPR_MISSING_WALKED_VALUE},
				IsParenthesized: shouldHaveClosingParen,
			},
		}
	}

	var parsingErr *sourcecode.ParsingError
	var metaIdent, entryIdent *ast.IdentifierLiteral

	p.eatSpace()

	walked, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	if isMissingExpr {
		eatClosingParen()

		return &ast.WalkExpression{
			NodeBase: ast.NodeBase{
				Span:            NodeSpan{walkExprStart, p.i},
				IsParenthesized: shouldHaveClosingParen,
			},
			Walked: walked,
		}
	}

	p.eatSpace()

	e, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	var ok bool
	if entryIdent, ok = e.(*ast.IdentifierLiteral); !ok {
		eatClosingParen()

		parsingErr = &sourcecode.ParsingError{UnterminatedWalkExpr, UNTERMINATED_WALK_EXPR_MISSING_ENTRY_VARIABLE_NAME}
		if isMissingExpr {
			e = nil
		}

		return &ast.WalkExpression{
			NodeBase: ast.NodeBase{
				Span:            NodeSpan{walkExprStart, p.i},
				Err:             parsingErr,
				IsParenthesized: shouldHaveClosingParen,
			},
			Walked:        walked,
			BadEntryIdent: e,
		}
	}

	p.eatSpace()

	// if the parsed identifier is instead the meta variable identifier we try to parse the entry variable identifier
	if p.i < p.len && p.s[p.i] == ',' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.COMMA, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		metaIdent = entryIdent
		entryIdent = nil
		p.eatSpace()

		// missing entry identifier
		if p.i >= p.len || p.s[p.i] == '{' {
			parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_WALK_EXPR_MISSING_ENTRY_VARIABLE_NAME}
		} else {
			e, isMissingExpr := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})
			if entryIdent, ok = e.(*ast.IdentifierLiteral); !ok {
				eatClosingParen()

				parsingErr = &sourcecode.ParsingError{UnterminatedWalkExpr, UNTERMINATED_WALK_EXPR_MISSING_ENTRY_VARIABLE_NAME}
				if isMissingExpr {
					e = nil
				}

				return &ast.WalkExpression{
					NodeBase: ast.NodeBase{
						Span:            NodeSpan{walkExprStart, p.i},
						Err:             parsingErr,
						IsParenthesized: shouldHaveClosingParen,
					},
					MetaIdent:     metaIdent,
					BadEntryIdent: e,
					Walked:        walked,
				}
			}
			p.eatSpace()
		}
	}

	var blk *ast.Block
	var end int32

	if p.i >= p.len || p.s[p.i] != '{' {
		end = p.i
		parsingErr = &sourcecode.ParsingError{UnterminatedWalkExpr, UNTERMINATED_WALK_EXPR_MISSING_BODY}
	} else {
		blk = p.parseBlock()
		end = blk.Span.End
	}

	if shouldHaveClosingParen {
		p.eatSpaceNewlineComment()

		if p.i < p.len && p.s[p.i] == ')' {
			p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			end = p.i
		} else {
			parsingErr = &sourcecode.ParsingError{UnterminatedForExpr, UNTERMINATED_WALK_EXPR_MISSING_CLOSIN_PAREN}
		}
	} else {
		end = p.i
	}

	return &ast.WalkExpression{
		NodeBase: ast.NodeBase{
			Span:            NodeSpan{walkExprStart, end},
			Err:             parsingErr,
			IsParenthesized: shouldHaveClosingParen,
		},
		Walked:     walked,
		MetaIdent:  metaIdent,
		EntryIdent: entryIdent,
		Body:       blk,
	}
}

func (p *parser) parseSwitchMatchStatement(keywordIdent *ast.IdentifierLiteral) ast.Node {
	p.panicIfContextDone()

	if keywordIdent.Name[0] == 's' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.SWITCH_KEYWORD, Span: keywordIdent.Base().Span})
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.MATCH_KEYWORD, Span: keywordIdent.Base().Span})
	}

	isMatchStmt := keywordIdent.Name == "match"

	p.eatSpace()

	if p.i >= p.len {
		if keywordIdent.Name == "switch" {
			return &ast.SwitchStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{keywordIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnterminatedSwitchStmt, UNTERMINATED_SWITCH_STMT_MISSING_VALUE},
				},
			}
		}

		return &ast.MatchStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{keywordIdent.Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnterminatedMatchStmt, UNTERMINATED_MATCH_STMT_MISSING_VALUE},
			},
		}
	}

	discriminant, _ := p.parseExpression()
	var switchCases []*ast.SwitchStatementCase
	var matchCases []*ast.MatchStatementCase
	var defaultCases []*ast.DefaultCaseWithBlock

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '{' {
		if !isMatchStmt {
			return &ast.SwitchStatement{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{keywordIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnterminatedSwitchStmt, UNTERMINATED_SWITCH_STMT_MISSING_BODY},
				},
				Discriminant: discriminant,
			}
		}

		return &ast.MatchStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{keywordIdent.Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnterminatedMatchStmt, UNTERMINATED_MATCH_STMT_MISSING_BODY},
			},
			Discriminant: discriminant,
		}
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

top_loop:
	for p.i < p.len && p.s[p.i] != '}' {
		p.eatSpaceNewlineSemicolonComment()

		if p.i < p.len && p.s[p.i] == '}' {
			break
		}

		if p.i < p.len && p.s[p.i] == '{' { //missing value before block
			missingExpr := &ast.MissingExpression{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i, p.i + 1},
					&sourcecode.ParsingError{MissingExpr, fmtCaseValueExpectedHere(p.s, p.i, true)},
					false,
				},
			}

			blk := p.parseBlock()
			base := ast.NodeBase{
				NodeSpan{missingExpr.Span.Start, blk.Span.End},
				nil,
				false,
			}

			if isMatchStmt {
				matchCases = append(matchCases, &ast.MatchStatementCase{
					NodeBase: base,
					Values:   []ast.Node{missingExpr},
					Block:    blk,
				})
			} else {
				switchCases = append(switchCases, &ast.SwitchStatementCase{
					NodeBase: base,
					Values:   []ast.Node{missingExpr},
					Block:    blk,
				})
			}
		} else { //parse values of case + block

			var switchCase *ast.SwitchStatementCase
			var matchCase *ast.MatchStatementCase
			var defaultCase *ast.DefaultCaseWithBlock

			if isMatchStmt {
				matchCase = &ast.MatchStatementCase{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{p.i, 0},
					},
				}
				matchCases = append(matchCases, matchCase)
			} else {
				switchCase = &ast.SwitchStatementCase{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{p.i, 0},
					},
				}
				switchCases = append(switchCases, switchCase)
			}

			//parse case's values
			for p.i < p.len && p.s[p.i] != '{' {
				valueNode, isMissingExpr := p.parseExpression()

				//if unexpected character, add case with error, increment p.i & parse next value
				if isMissingExpr && (p.i >= p.len || p.s[p.i] != '}') {
					p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
					valueNode = &ast.UnknownNode{
						NodeBase: ast.NodeBase{
							NodeSpan{p.i, p.i + 1},
							&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSwitchOrMatchStatement(p.s[p.i])},
							false,
						},
					}

					if isMatchStmt {
						matchCase.Values = append(matchCase.Values, valueNode)
					} else {
						switchCase.Values = append(switchCase.Values, valueNode)
					}

					p.i++
					continue top_loop
				}
				//if ok

				//default case
				if ident, ok := valueNode.(*ast.IdentifierLiteral); ok && ident.Name == ast.TOKEN_STRINGS[ast.DEFAULTCASE_KEYWORD] {

					//remove case
					if isMatchStmt {
						matchCases = matchCases[:len(matchCases)-1]
					} else {
						switchCases = switchCases[:len(switchCases)-1]
					}

					p.tokens = append(p.tokens, ast.Token{Type: ast.DEFAULTCASE_KEYWORD, Span: NodeSpan{ident.Span.Start, ident.Span.End}})
					defaultCase = &ast.DefaultCaseWithBlock{
						NodeBase: ast.NodeBase{
							Span: NodeSpan{ident.Span.Start, ident.Span.End},
						},
					}

					defaultCases = append(defaultCases, defaultCase)

					if len(defaultCases) > 1 {
						defaultCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, DEFAULT_CASE_MUST_BE_UNIQUE}
					}

					p.eatSpace()

					goto parse_block
				}

				if isMatchStmt && !isAllowedMatchCase(valueNode) {
					matchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_MATCH_CASE_VALUE_EXPLANATION}
				} else if !isMatchStmt && !ast.NodeIsSimpleValueLiteral(valueNode) {
					switchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_SWITCH_CASE_VALUE_EXPLANATION}
				}

				if isMatchStmt {
					matchCase.Values = append(matchCase.Values, valueNode)
				} else {
					switchCase.Values = append(switchCase.Values, valueNode)
				}

				p.eatSpace()

				if p.i >= p.len {
					goto parse_block
				}

				switch {
				case p.s[p.i] == ',': //comma before next value
					p.tokens = append(p.tokens, ast.Token{Type: ast.COMMA, Span: NodeSpan{p.i, p.i + 1}})
					p.i++

				case isAlpha(p.s[p.i]) && isMatchStmt: // group matching variable
					e, _ := p.parseExpression()

					ident, ok := e.(*ast.IdentifierLiteral)
					if ok && isKeyword(ident.Name) {
						matchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
					}
					matchCase.GroupMatchingVariable = e
					p.eatSpace()
					goto parse_block
				case p.s[p.i] != '{' && p.s[p.i] != '}': //unexpected character: we add an error and parse next case
					p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
					valueNode = &ast.UnknownNode{
						NodeBase: ast.NodeBase{
							NodeSpan{p.i, p.i + 1},
							&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSwitchOrMatchStatement(p.s[p.i])},
							false,
						},
					}
					p.i++

					if isMatchStmt {
						matchCase.Values = append(matchCase.Values, valueNode)
						matchCase.Span.End = p.i
					} else {
						switchCase.Values = append(switchCase.Values, valueNode)
						switchCase.Span.End = p.i
					}
					continue top_loop
				case p.s[p.i] == '}':
					break top_loop
				}

				p.eatSpace()
			}

		parse_block:
			var blk *ast.Block
			end := p.i

			if p.i >= p.len || p.s[p.i] != '{' { // missing block
				if defaultCase != nil {
					defaultCase.Err = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_DEFAULT_CASE_MISSING_BLOCK}
				} else if isMatchStmt {
					matchCase.Err = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_MATCH_CASE_MISSING_BLOCK}
				} else {
					switchCase.Err = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_SWITCH_CASE_MISSING_BLOCK}
				}
			} else {
				blk = p.parseBlock()
				end = blk.Span.End
			}

			if defaultCase != nil {
				defaultCase.Span.End = end
				defaultCase.Block = blk
			} else if isMatchStmt {
				matchCase.Span.End = end
				matchCase.Block = blk
			} else {
				switchCase.Span.End = end
				switchCase.Block = blk
			}
		}

		p.eatSpaceNewlineSemicolonComment()
	}

	var parsingErr *sourcecode.ParsingError

	if p.i >= p.len || p.s[p.i] != '}' {
		if keywordIdent.Name == "switch" {
			parsingErr = &sourcecode.ParsingError{UnterminatedSwitchStmt, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE}
		} else {
			parsingErr = &sourcecode.ParsingError{UnterminatedMatchStmt, UNTERMINATED_MATCH_STMT_MISSING_CLOSING_BRACE}
		}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	if isMatchStmt {
		return &ast.MatchStatement{
			NodeBase: ast.NodeBase{
				NodeSpan{keywordIdent.Span.Start, p.i},
				parsingErr,
				false,
			},
			Discriminant: discriminant,
			Cases:        matchCases,
			DefaultCases: defaultCases,
		}
	}

	return &ast.SwitchStatement{
		NodeBase: ast.NodeBase{
			NodeSpan{keywordIdent.Span.Start, p.i},
			parsingErr,
			false,
		},
		Discriminant: discriminant,
		Cases:        switchCases,
		DefaultCases: defaultCases,
	}
}

func (p *parser) parseSwitchMatchExpression(keywordIdent *ast.IdentifierLiteral) ast.Node {
	p.panicIfContextDone()

	if keywordIdent.Name[0] == 's' {
		p.tokens = append(p.tokens, ast.Token{Type: ast.SWITCH_KEYWORD, Span: keywordIdent.Base().Span})
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.MATCH_KEYWORD, Span: keywordIdent.Base().Span})
	}

	isMatchExpr := keywordIdent.Name == "match"

	p.eatSpace()

	if p.i >= p.len {

		if keywordIdent.Name == "switch" {
			return &ast.SwitchExpression{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{keywordIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnterminatedSwitchExpr, UNTERMINATED_SWITCH_EXPR_MISSING_VALUE},
				},
			}
		}

		return &ast.MatchExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{keywordIdent.Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnterminatedMatchExpr, UNTERMINATED_MATCH_EXPR_MISSING_VALUE},
			},
		}
	}

	discriminant, _ := p.parseExpression()
	var switchCases []*ast.SwitchExpressionCase
	var matchCases []*ast.MatchExpressionCase
	var defaultCases []*ast.DefaultCaseWithResult

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '{' {
		if !isMatchExpr {
			return &ast.SwitchExpression{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{keywordIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnterminatedSwitchExpr, UNTERMINATED_SWITCH_EXPR_MISSING_BODY},
				},
				Discriminant: discriminant,
			}
		}

		return &ast.MatchExpression{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{keywordIdent.Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnterminatedMatchExpr, UNTERMINATED_MATCH_EXPR_MISSING_BODY},
			},
			Discriminant: discriminant,
		}
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

top_loop:
	for p.i < p.len && (!isUnpairedOrIsClosingDelim(p.s[p.i]) || p.s[p.i] == '\n') {
		p.eatSpaceNewlineSemicolonComment()

		if p.i < p.len && p.s[p.i] != '\n' && isUnpairedOrIsClosingDelim(p.s[p.i]) {
			break
		}

		var switchCase *ast.SwitchExpressionCase
		var matchCase *ast.MatchExpressionCase
		var defaultCase *ast.DefaultCaseWithResult

		if isMatchExpr {
			matchCase = &ast.MatchExpressionCase{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, 0},
				},
			}
			matchCases = append(matchCases, matchCase)
		} else {
			switchCase = &ast.SwitchExpressionCase{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{p.i, 0},
				},
			}
			switchCases = append(switchCases, switchCase)
		}

		//parse case's values
		for p.i < p.len && p.s[p.i] != '=' {
			valueNode, isMissingExpr := p.parseExpression()

			//if unexpected character, add case with error, increment p.i & parse next value
			if isMissingExpr && (p.i >= p.len || p.s[p.i] != '}') {
				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
				valueNode = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						NodeSpan{p.i, p.i + 1},
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSwitchOrMatchExpression(p.s[p.i])},
						false,
					},
				}

				if isMatchExpr {
					matchCase.Values = append(matchCase.Values, valueNode)
				} else {
					switchCase.Values = append(switchCase.Values, valueNode)
				}

				p.i++
				continue top_loop
			}
			//if ok

			//default case
			if ident, ok := valueNode.(*ast.IdentifierLiteral); ok && ident.Name == ast.TOKEN_STRINGS[ast.DEFAULTCASE_KEYWORD] {

				//remove case
				if isMatchExpr {
					matchCases = matchCases[:len(matchCases)-1]
				} else {
					switchCases = switchCases[:len(switchCases)-1]
				}

				p.tokens = append(p.tokens, ast.Token{Type: ast.DEFAULTCASE_KEYWORD, Span: NodeSpan{ident.Span.Start, ident.Span.End}})
				defaultCase = &ast.DefaultCaseWithResult{
					NodeBase: ast.NodeBase{
						Span: NodeSpan{ident.Span.Start, ident.Span.End},
					},
				}

				defaultCases = append(defaultCases, defaultCase)

				if len(defaultCases) > 1 {
					defaultCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, DEFAULT_CASE_MUST_BE_UNIQUE}
				}

				p.eatSpace()

				goto parse_case_result
			}

			if isMatchExpr && !isAllowedMatchCase(valueNode) {
				matchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_MATCH_CASE_VALUE_EXPLANATION}
			} else if !isMatchExpr && !ast.NodeIsSimpleValueLiteral(valueNode) {
				switchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_SWITCH_CASE_VALUE_EXPLANATION}
			}

			if isMatchExpr {
				matchCase.Values = append(matchCase.Values, valueNode)
			} else {
				switchCase.Values = append(switchCase.Values, valueNode)
			}

			p.eatSpace()

			if p.i >= p.len {
				goto parse_case_result
			}

			switch {
			case p.s[p.i] == ',': //comma before next value
				p.tokens = append(p.tokens, ast.Token{Type: ast.COMMA, Span: NodeSpan{p.i, p.i + 1}})
				p.i++

			case isAlpha(p.s[p.i]) && isMatchExpr: // group matching variable
				e, _ := p.parseExpression()

				ident, ok := e.(*ast.IdentifierLiteral)
				if ok && isKeyword(ident.Name) {
					matchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
				}
				matchCase.GroupMatchingVariable = e
				p.eatSpace()
				goto parse_case_result
			case p.s[p.i] != '=' && p.s[p.i] != '}': //unexpected character: we add an error and parse next case
				p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
				valueNode = &ast.UnknownNode{
					NodeBase: ast.NodeBase{
						NodeSpan{p.i, p.i + 1},
						&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSwitchOrMatchExpression(p.s[p.i])},
						false,
					},
				}
				p.i++

				if isMatchExpr {
					matchCase.Values = append(matchCase.Values, valueNode)
					matchCase.Span.End = p.i
				} else {
					switchCase.Values = append(switchCase.Values, valueNode)
					switchCase.Span.End = p.i
				}
				continue top_loop
			case p.s[p.i] == '}':
				break top_loop
			}

			p.eatSpace()
		}

	parse_case_result:
		var caseResult ast.Node
		end := p.i

		p.eatSpace()

		if p.i >= p.len-1 || p.s[p.i] != '=' || p.s[p.i+1] != '>' { // missing or unterminated arrow '=>'

			unterminatedArrow := p.i < p.len && p.s[p.i] == '='

			if unterminatedArrow {
				p.tokens = append(p.tokens, ast.Token{
					Type: ast.EQUAL,
					Span: NodeSpan{p.i, p.i + 1},
				})

				p.i++
			}

			switch {
			case defaultCase != nil:
				if unterminatedArrow {
					defaultCase.Err = &sourcecode.ParsingError{UnterminatedArrow, UNTERMINATED_DEFAULT_CASE_UNTERMINATED_ARROW}
				} else {
					defaultCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_DEFAULT_CASE_MISSING_RESULT}
				}
			case isMatchExpr:
				if unterminatedArrow {
					matchCase.Err = &sourcecode.ParsingError{UnterminatedArrow, UNTERMINATED_MATCH_EXPR_CASE_UNTERMINATED_ARROW}
				} else {
					matchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MATCH_EXPR_CASE_MISSING_RESULT}
				}
			default:
				if unterminatedArrow {
					switchCase.Err = &sourcecode.ParsingError{UnterminatedArrow, UNTERMINATED_SWITCH_EXPR_CASE_UNTERMINATED_ARROW}
				} else {
					switchCase.Err = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_EXPR_CASE_MISSING_RESULT}
				}
			}
		} else {
			p.tokens = append(p.tokens, ast.Token{
				Type: ast.ARROW,
				Span: NodeSpan{p.i, p.i + 2},
			})

			p.i += 2

			p.eatSpace()

			caseResult, _ = p.parseExpression()
			end = caseResult.Base().Span.End
		}

		if defaultCase != nil {
			defaultCase.Span.End = end
			defaultCase.Result = caseResult
		} else if isMatchExpr {
			matchCase.Span.End = end
			matchCase.Result = caseResult
		} else {
			switchCase.Span.End = end
			switchCase.Result = caseResult
		}

		p.eatSpaceNewlineSemicolonComment()
	}

	var parsingErr *sourcecode.ParsingError

	if p.i >= p.len || p.s[p.i] != '}' {
		if keywordIdent.Name == "switch" {
			parsingErr = &sourcecode.ParsingError{UnterminatedSwitchExpr, UNTERMINATED_SWITCH_EXPR_MISSING_CLOSING_BRACE}
		} else {
			parsingErr = &sourcecode.ParsingError{UnterminatedMatchExpr, UNTERMINATED_MATCH_EXPR_MISSING_CLOSING_BRACE}
		}
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	if isMatchExpr {
		return &ast.MatchExpression{
			NodeBase: ast.NodeBase{
				NodeSpan{keywordIdent.Span.Start, p.i},
				parsingErr,
				false,
			},
			Discriminant: discriminant,
			Cases:        matchCases,
			DefaultCases: defaultCases,
		}
	}

	return &ast.SwitchExpression{
		NodeBase: ast.NodeBase{
			NodeSpan{keywordIdent.Span.Start, p.i},
			parsingErr,
			false,
		},
		Discriminant: discriminant,
		Cases:        switchCases,
		DefaultCases: defaultCases,
	}
}

func (p *parser) parsePermissionDroppingStatement(dropPermIdent *ast.IdentifierLiteral) *ast.PermissionDroppingStatement {
	p.panicIfContextDone()

	p.eatSpace()

	e, _ := p.parseExpression()
	objLit, ok := e.(*ast.ObjectLiteral)

	var parsingErr *sourcecode.ParsingError
	var end int32

	if ok {
		end = objLit.Span.End
	} else {
		end = e.Base().Span.End
		parsingErr = &sourcecode.ParsingError{UnspecifiedParsingError, DROP_PERM_KEYWORD_SHOULD_BE_FOLLOWED_BY}
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.DROP_PERMS_KEYWORD, Span: dropPermIdent.Span})

	return &ast.PermissionDroppingStatement{
		NodeBase: ast.NodeBase{
			NodeSpan{dropPermIdent.Base().Span.Start, end},
			parsingErr,
			false,
		},
		Object: objLit,
	}

}

func (p *parser) parseImportStatement(importIdent *ast.IdentifierLiteral) ast.Node {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, ast.Token{Type: ast.IMPORT_KEYWORD, Span: importIdent.Span})

	p.eatSpace()

	e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

	var identifier *ast.IdentifierLiteral

	switch src := e.(type) {
	case *ast.RelativePathLiteral:
		p.checkImportSource(src)

		return &ast.InclusionImportStatement{
			NodeBase: ast.NodeBase{
				NodeSpan{importIdent.Span.Start, p.i},
				nil,
				false,
			},
			Source: src,
		}
	case *ast.AbsolutePathLiteral:
		p.checkImportSource(src)

		return &ast.InclusionImportStatement{
			NodeBase: ast.NodeBase{
				NodeSpan{importIdent.Span.Start, p.i},
				nil,
				false,
			},
			Source: src,
		}
	case *ast.IdentifierLiteral:
		identifier = src
		//we continue parsing the module import statement
	default:
		if ast.NodeIsSimpleValueLiteral(src) {
			return &ast.InclusionImportStatement{
				NodeBase: ast.NodeBase{
					NodeSpan{importIdent.Span.Start, p.i},
					&sourcecode.ParsingError{UnspecifiedParsingError, INCLUSION_IMPORT_STMT_SRC_SHOULD_BE_A_PATH_LIT},
					false,
				},
				Source: src,
			}
		}

		return &ast.ImportStatement{
			NodeBase: ast.NodeBase{
				NodeSpan{importIdent.Span.Start, p.i},
				&sourcecode.ParsingError{UnspecifiedParsingError, IMPORT_STMT_IMPORT_KEYWORD_SHOULD_BE_FOLLOWED_BY_IDENT},
				false,
			},
			Source: src,
		}
	}

	p.eatSpace()

	src, _ := p.parseExpression()

	var parsingError *sourcecode.ParsingError

	switch src := src.(type) {
	case *ast.URLLiteral:
		p.checkImportSource(src)
	case *ast.RelativePathLiteral:
		p.checkImportSource(src)
	case *ast.AbsolutePathLiteral:
		p.checkImportSource(src)
	default:
		parsingError = &sourcecode.ParsingError{UnspecifiedParsingError, IMPORT_STMT_SRC_SHOULD_BE_AN_URL_OR_PATH_LIT}
	}

	p.eatSpace()
	config, _ := p.parseExpression()

	if _, ok := config.(*ast.ObjectLiteral); !ok && config.Base().Err == nil {
		config.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, IMPORT_STMT_CONFIG_SHOULD_BE_AN_OBJ_LIT}
	}

	return &ast.ImportStatement{
		NodeBase: ast.NodeBase{
			NodeSpan{importIdent.Span.Start, p.i},
			parsingError,
			false,
		},
		Identifier:    identifier,
		Source:        src,
		Configuration: config,
	}
}

func (p *parser) checkImportSource(node ast.SimpleValueLiteral) {
	if node.Base().Err != nil {
		return
	}
	var path string
	urlLit, isUrl := node.(*ast.URLLiteral)

	if isUrl {
		u, err := url.Parse(urlLit.Value)
		if err != nil {
			return
		}
		path = u.Path
	} else {
		path = node.ValueString()
	}

	if !strings.HasSuffix(path, inoxconsts.INOXLANG_FILE_EXTENSION) {
		node.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, URL_LITS_AND_PATH_LITS_USED_AS_IMPORT_SRCS_SHOULD_END_WITH_IX}
		return
	}

	runes := []rune(path)

	absolute := path[0] == '/'
	dotSlash := strings.HasPrefix(path, "./")
	if !absolute && !dotSlash && !strings.HasPrefix(path, "../") {
		node.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, "unexpected path beginning"}
		return
	}

	i := 0

	if i >= len(runes) {
		return
	}

	for i < len(runes) {
		r := runes[i]
		switch r {
		case '/':
			if i != 0 && runes[i-1] == '/' {
				err := &sourcecode.ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH}

				if isUrl {
					err.Message = PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH
				}

				node.BasePtr().Err = err
				return
			}
		case '.':
			/* /../ */
			if (i == 0 || runes[i-1] == '/') && i < len(runes)-2 && runes[i+1] == '.' && runes[i+2] == '/' {
				err := &sourcecode.ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH}
				if isUrl {
					err.Message = PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH
				}

				node.BasePtr().Err = err
				return
			}
			/* /../ */
			if i > 0 && runes[i-1] == '/' && i < len(runes)-1 && runes[i+1] == '/' {
				err := &sourcecode.ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS}
				if isUrl {
					err.Message = PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS
				}

				node.BasePtr().Err = err
				return
			}
		default:
		}
		i++
	}
}

func (p *parser) parseReturnStatement(returnIdent *ast.IdentifierLiteral) *ast.ReturnStatement {
	p.panicIfContextDone()

	var end int32 = p.i
	var returnValue ast.Node

	p.eatSpace()

	if p.i < p.len && p.s[p.i] != ';' && p.s[p.i] != '}' && p.s[p.i] != '\n' {
		returnValue, _ = p.parseExpression()
		end = returnValue.Base().Span.End
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.RETURN_KEYWORD, Span: returnIdent.Span})

	return &ast.ReturnStatement{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{returnIdent.Span.Start, end},
		},
		Expr: returnValue,
	}
}

func (p *parser) parseCoyieldStatement(yieldIdent *ast.IdentifierLiteral) *ast.CoyieldStatement {
	p.panicIfContextDone()

	var end int32 = p.i
	var returnValue ast.Node

	p.eatSpace()

	if p.i < p.len && p.s[p.i] != ';' && p.s[p.i] != '}' && p.s[p.i] != '\n' {
		returnValue, _ = p.parseExpression()
		end = returnValue.Base().Span.End
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.COYIELD_KEYWORD, Span: yieldIdent.Span})

	return &ast.CoyieldStatement{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{yieldIdent.Span.Start, end},
		},
		Expr: returnValue,
	}
}

func (p *parser) parseYieldStatement(yieldIdent *ast.IdentifierLiteral) *ast.YieldStatement {
	p.panicIfContextDone()

	var end int32 = p.i
	var returnValue ast.Node

	p.eatSpace()

	if p.i < p.len && p.s[p.i] != ';' && p.s[p.i] != '}' && p.s[p.i] != '\n' {
		returnValue, _ = p.parseExpression()
		end = returnValue.Base().Span.End
	}

	p.tokens = append(p.tokens, ast.Token{Type: ast.YIELD_KEYWORD, Span: yieldIdent.Span})

	return &ast.YieldStatement{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{yieldIdent.Span.Start, end},
		},
		Expr: returnValue,
	}
}

func (p *parser) parseSynchronizedBlock(synchronizedIdent *ast.IdentifierLiteral) *ast.SynchronizedBlockStatement {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, ast.Token{Type: ast.SYNCHRONIZED_KEYWORD, Span: synchronizedIdent.Span})

	p.eatSpace()
	if p.i >= p.len {
		return &ast.SynchronizedBlockStatement{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{synchronizedIdent.Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, SYNCHRONIZED_KEYWORD_SHOULD_BE_FOLLOWED_BY_SYNC_VALUES},
			},
		}
	}

	var synchronizedValues []ast.Node

	for p.i < p.len && p.s[p.i] != '{' {
		valueNode, isMissingExpr := p.parseExpression()
		if isMissingExpr {
			p.tokens = append(p.tokens, ast.Token{Type: ast.UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])})
			valueNode = &ast.UnknownNode{
				NodeBase: ast.NodeBase{
					NodeSpan{p.i, p.i + 1},
					&sourcecode.ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSynchronizedValueList(p.s[p.i])},
					false,
				},
			}
			p.i++
		}
		synchronizedValues = append(synchronizedValues, valueNode)

		p.eatSpace()
	}

	var parsingErr *sourcecode.ParsingError
	var block *ast.Block

	if p.i >= p.len || p.s[p.i] != '{' {
		parsingErr = &sourcecode.ParsingError{MissingBlock, UNTERMINATED_SYNCHRONIZED_MISSING_BLOCK}
	} else {
		block = p.parseBlock()
	}

	return &ast.SynchronizedBlockStatement{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{synchronizedIdent.Span.Start, p.i},
			Err:  parsingErr,
		},
		SynchronizedValues: synchronizedValues,
		Block:              block,
	}
}

func (p *parser) parseMultiAssignmentStatement(assignIdent *ast.IdentifierLiteral) *ast.MultiAssignment {
	p.panicIfContextDone()

	p.tokens = append(p.tokens, ast.Token{Type: ast.ASSIGN_KEYWORD, Span: assignIdent.Span})
	var vars []ast.Node

	nillable := false

	if p.i < p.len && p.s[p.i] == '?' {
		nillable = true
		p.tokens = append(p.tokens, ast.Token{Type: ast.QUESTION_MARK, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	var keywordLHSError *sourcecode.ParsingError

	//Pass the names of assigned variables.

	for p.i < p.len && p.s[p.i] != '=' {
		p.eatSpace()
		e, _ := p.parseExpression(exprParsingConfig{disallowUnparenthesizedBinForPipelineExprs: true})

		switch nameNode := e.(type) {
		case *ast.IdentifierLiteral:
			if isKeyword(nameNode.Name) {
				keywordLHSError = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
			}
		case *ast.UnquotedRegion:
			//ok
		default:
			return &ast.MultiAssignment{
				NodeBase: ast.NodeBase{
					Span: NodeSpan{assignIdent.Span.Start, p.i},
					Err:  &sourcecode.ParsingError{UnspecifiedParsingError, ASSIGN_KEYWORD_SHOULD_BE_FOLLOWED_BY_IDENTS},
				},
				Variables: vars,
			}
		}

		vars = append(vars, e)
		p.eatSpace()
	}

	var (
		right ast.Node
		end   int32
	)
	if p.i >= p.len || p.s[p.i] != '=' {
		keywordLHSError = &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_MULTI_ASSIGN_MISSING_EQL_SIGN}
		end = p.i
	} else {
		p.tokens = append(p.tokens, ast.Token{Type: ast.EQUAL, SubType: ast.ASSIGN_EQUAL, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		p.eatSpace()
		right, _ = p.parseExpression()
		end = right.Base().Span.End
	}

	// terminator
	p.eatSpace()
	if p.i < p.len {
		switch p.s[p.i] {
		case ';', '\r', '\n', '}':
		case '#':
			if p.i < p.len-1 && IsCommentFirstSpace(p.s[p.i+1]) {
				break
			}
			fallthrough
		default:
			if keywordLHSError == nil {
				keywordLHSError = &sourcecode.ParsingError{Kind: InvalidNext, Message: UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR}
			}
		}
	}

	return &ast.MultiAssignment{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{assignIdent.Span.Start, end},
			Err:  keywordLHSError,
		},
		Variables: vars,
		Right:     right,
		Nillable:  nillable,
	}
}

func (p *parser) parseAssignment(left ast.Node) (result ast.Node) {
	p.panicIfContextDone()

	// terminator
	defer func() {
		if result == nil { //panic
			return
		}

		p.eatSpace()
		if p.i >= p.len {
			return
		}

		switch p.s[p.i] {
		case ';', '\r', '\n', '}':
		case '#':
			if p.i < p.len-1 && IsCommentFirstSpace(p.s[p.i+1]) {
				break
			}
			fallthrough
		default:
			base := result.BasePtr()
			if base.Err == nil {
				base.Err = &sourcecode.ParsingError{InvalidNext, UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR}
			}
		}
	}()

	var assignmentTokenType ast.TokenType
	var assignmentSubTokenType ast.TokenSubType
	var assignmentOperator ast.AssignmentOperator

	{
		switch p.s[p.i] {
		case '=':
			assignmentTokenType = ast.EQUAL
			assignmentSubTokenType = ast.ASSIGN_EQUAL
			assignmentOperator = ast.Assign
		case '+':
			assignmentTokenType = ast.PLUS_EQUAL
			assignmentOperator = ast.PlusAssign
			p.i++
		case '-':
			assignmentTokenType = ast.MINUS_EQUAL
			assignmentOperator = ast.MinusAssign
			p.i++
		case '*':
			assignmentTokenType = ast.MUL_EQUAL
			assignmentOperator = ast.MulAssign
			p.i++
		case '/':
			assignmentTokenType = ast.DIV_EQUAL
			assignmentOperator = ast.DivAssign
			p.i++
		}
		p.tokens = append(p.tokens, ast.Token{Type: assignmentTokenType, SubType: assignmentSubTokenType, Span: NodeSpan{p.i, p.i + 1}})
	}

	p.i++
	p.eatSpace()

	var keywordLHSError *sourcecode.ParsingError

	switch l := left.(type) {
	case *ast.Variable, *ast.MemberExpression, *ast.IndexExpression, *ast.SliceExpression, *ast.IdentifierMemberExpression,
		*ast.UnquotedRegion:
	case *ast.IdentifierLiteral:
		if isKeyword(l.Name) {
			keywordLHSError = &sourcecode.ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
		}
	default:
		return &ast.Assignment{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{left.Base().Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, fmtInvalidAssignmentInvalidLHS(left)},
			},
			Left:     left,
			Operator: assignmentOperator,
		}
	}

	if p.i >= p.len || p.s[p.i] == '\n' {
		return &ast.Assignment{
			NodeBase: ast.NodeBase{
				Span: NodeSpan{left.Base().Span.Start, p.i},
				Err:  &sourcecode.ParsingError{UnspecifiedParsingError, UNTERMINATED_ASSIGNMENT_MISSING_VALUE_AFTER_EQL_SIGN},
			},
			Left:     left,
			Operator: assignmentOperator,
		}
	}

	right, _ := p.parseExpression()

	return &ast.Assignment{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{left.Base().Span.Start, right.Base().Span.End},
			Err:  keywordLHSError,
		},
		Left:     left,
		Right:    right,
		Operator: assignmentOperator,
	}
}

func (p *parser) parseExtendStatement(extendIdent *ast.IdentifierLiteral) *ast.ExtendStatement {
	p.panicIfContextDone()
	p.tokens = append(p.tokens, ast.Token{Type: ast.EXTEND_KEYWORD, Span: extendIdent.Span})

	p.eatSpace()

	extendStmt := &ast.ExtendStatement{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{Start: extendIdent.Span.Start, End: p.i},
		},
	}

	if p.i >= p.len || p.s[p.i] == '\n' {
		extendStmt.Err = &sourcecode.ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_PATTERN_TO_EXTEND_AFTER_KEYWORD}
	} else {
		func() {
			prev := p.inPattern
			p.inPattern = true
			defer func() {
				p.inPattern = prev
			}()

			extendStmt.ExtendedPattern, _ = p.parseExpression(exprParsingConfig{
				disallowUnparenthesizedBinForPipelineExprs: true,
				disallowParsingSeveralPatternUnionCases:    true,
			})
			extendStmt.Span.End = p.i

			if _, ok := extendStmt.ExtendedPattern.(*ast.PatternIdentifierLiteral); !ok && extendStmt.ExtendedPattern.Base().Err == nil {
				extendStmt.ExtendedPattern.BasePtr().Err = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: A_PATTERN_NAME_WAS_EXPECTED}
			}
		}()

		p.eatSpace()

		if p.i >= p.len || p.s[p.i] == '\n' {
			extendStmt.Err = &sourcecode.ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_OBJECT_LITERAL_AFTER_EXTENDED_PATTERN}
		} else {
			extendStmt.Extension, _ = p.parseExpression()
			extendStmt.Span.End = p.i

			if _, ok := extendStmt.Extension.(*ast.ObjectLiteral); !ok && extendStmt.Extension.Base().Err == nil {
				extendStmt.Extension.BasePtr().Err = &sourcecode.ParsingError{UnspecifiedParsingError, INVALID_EXTENSION_VALUE_AN_OBJECT_LITERAL_WAS_EXPECTED}
			}
		}
	}

	return extendStmt
}

func (p *parser) parseValuePathLiteral() ast.Node {

	var firstSegment ast.SimpleValueLiteral
	var segments []ast.SimpleValueLiteral

	for p.i < p.len && p.s[p.i] == '.' {
		start := p.i
		p.i++

		for p.i < p.len && IsIdentChar(p.s[p.i]) {
			p.i++
		}

		node := &ast.PropertyNameLiteral{
			NodeBase: ast.NodeBase{Span: NodeSpan{start, p.i}},
			Name:     string(p.s[start+1 : p.i]),
		}

		if node.Name == "" {
			node.Err = &sourcecode.ParsingError{Kind: UnspecifiedParsingError, Message: UNTERMINATED_VALUE_PATH_LITERAL}
		}

		if firstSegment == nil {
			firstSegment = node
		} else {
			segments = append(segments, node)
		}
	}

	if len(segments) == 0 {
		return firstSegment
	}

	segments = append(segments, nil)
	copy(segments[1:], segments[:len(segments)-1])
	segments[0] = firstSegment

	start := firstSegment.Base().Span.Start
	end := segments[len(segments)-1].Base().Span.End
	return &ast.LongValuePathLiteral{
		NodeBase: ast.NodeBase{
			Span: NodeSpan{start, end},
		},
		Segments: segments,
	}
}

func isKeyword(str string) bool {
	return slices.Contains(KEYWORDS, str)
}

func IsMetadataKey(key string) bool {
	return len(key) >= 3 && key[0] == '_' && key[1] != '_' && key[len(key)-2] != '_' && key[len(key)-1] == '_'
}

func IsSupportedSchemeName(s string) bool {
	return slices.Contains(SCHEMES, s)
}

func GetNameIfVariable(node ast.Node) (string, bool) {
	switch n := node.(type) {
	case *ast.Variable:
		return n.Name, true
	case *ast.IdentifierLiteral:
		return n.Name, true
	default:
		return "", false
	}
}

func isAllowedMatchCase(node ast.Node) (result bool) {
	isAllowedMatchCaseNode := func(node ast.Node) bool {
		if ast.NodeIsPattern(node) {
			return true
		}

		switch node.(type) {
		case ast.SimpleValueLiteral, *ast.IntegerRangeLiteral, *ast.FloatRangeLiteral, *ast.NamedSegmentPathPatternLiteral:
			return true
		case *ast.ObjectLiteral, *ast.ObjectProperty, *ast.RecordLiteral, *ast.ListLiteral, *ast.TupleLiteral:
			return true
		case *ast.ObjectPatternProperty, *ast.PatternPieceElement:
			return true
		}
		return false
	}

	if !isAllowedMatchCaseNode(node) {
		return false
	}

	if ast.NodeIsPattern(node) {
		return true
	}

	switch node.(type) {
	case ast.SimpleValueLiteral, *ast.IntegerRangeLiteral, *ast.FloatRangeLiteral, *ast.NamedSegmentPathPatternLiteral:
		return true
	case *ast.ObjectLiteral, *ast.ObjectProperty, *ast.RecordLiteral, *ast.ListLiteral, *ast.TupleLiteral,
		*ast.ObjectPatternLiteral, *ast.RecordPatternLiteral, *ast.ListPatternLiteral, *ast.TuplePatternLiteral:
		result = true
		ast.Walk(node, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) (ast.TraversalAction, error) {
			if !isAllowedMatchCaseNode(node) {
				result = false
				return ast.StopTraversal, nil
			}
			return ast.ContinueTraversal, nil
		}, nil)
	}
	return
}

func len32[T any](arg []T) int32 {
	return int32(len(arg))
}

// CheckGetEffectivePort("https", "") -> 443
// CheckGetEffectivePort("https", 9000) -> 9000
// CheckGetEffectivePort("https", 9999) -> error !
// CheckGetEffectivePort("https", -1) -> error !
// CheckGetEffectivePort("s3", 9000) -> error ! pseudo protocol does not use network ports
// CheckGetEffectivePort("mem", 9000) -> error ! pseudo protocol does not use network ports
func CheckGetEffectivePort(scheme string, port string) (string, error) {
	switch scheme {
	case "https", "wss":
		if port == "" {
			port = "443"
		}
	case "http", "ws":
		if port == "" {
			port = "80"
		}
	default:
		if port == "" {
			return "", nil
		}
		return "", errors.New(fmtProtocolOrPseudoProtocolDoesNotUseNetworkPorts(scheme))
	}

	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65_535 {
		return "", errors.New(NET_PORT_INVALID_OR_OUT_OR_RANGE)
	}

	return port, nil
}
