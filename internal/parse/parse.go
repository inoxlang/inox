package parse

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_MODULE_BYTE_LEN     = 1 << 24
	MAX_OBJECT_KEY_BYTE_LEN = 64
	MAX_SCHEME_NAME_LEN     = 5

	LOOSE_URL_EXPR_PATTERN       = "^(@[a-zA-Z0-9_-]+|https?:\\/\\/([-\\w]+|(www\\.)?[-a-zA-Z0-9@:%._+~#=]{1,32}\\.[a-zA-Z0-9]{1,6}\\b|\\{[$]{0,2}[-\\w]+\\}))([{?#/][-a-zA-Z0-9@:%_+.~#?&//=${}]*)$"
	LOOSE_HOST_PATTERN_PATTERN   = "^([a-z0-9+]+)?:\\/\\/([-\\w]+|[*]+|(www\\.)?[-a-zA-Z0-9.*]{1,32}\\.[a-zA-Z0-9*]{1,6})(:[0-9]{1,5})?$"
	LOOSE_HOST_PATTERN           = "^([a-z0-9+]+)?:\\/\\/([-\\w]+|(www\\.)?[-a-zA-Z0-9.]{1,32}\\.[a-zA-Z0-9]{1,6})(:[0-9]{1,5})?$"
	URL_PATTERN                  = "^([a-z0-9+]+):\\/\\/([-\\w]+|(www\\.)?[-a-zA-Z0-9@:%._+~#=]{1,32}\\.[a-zA-Z0-9]{1,6})\\b([-a-zA-Z0-9@:%_+.~#?&//=]*)$"
	DATE_LITERAL_PATTERN         = "^(\\d+y)(-\\d{1,2}mt)?(-\\d{1,2}d)?(-\\d{1,2}h)?(-\\d{1,2}m)?(-\\d{1,2}s)?(-\\d{1,3}ms)?(-\\d{1,3}us)?(-[a-zA-Z_/]+[a-zA-Z_])$"
	STRICT_EMAIL_ADDRESS_PATTERN = "(?i)(^[A-Z0-9._%+-]+@[A-Z0-9.-]+\\.[A-Z]{2,24}$)"

	CONST_KEYWORD_STR = "const"
	VAR_KEYWORD_STR   = "var"
)

var (
	KEYWORDS             = tokenStrings[IF_KEYWORD : OR_KEYWORD+1]
	PREINIT_KEYWORD_STR  = tokenStrings[PREINIT_KEYWORD]
	MANIFEST_KEYWORD_STR = tokenStrings[MANIFEST_KEYWORD]
	SCHEMES              = []string{"http", "https", "ws", "wss", "ldb", "file", "mem", "s3"}

	//regexes
	URL_REGEX                  = regexp.MustCompile(URL_PATTERN)
	LOOSE_HOST_REGEX           = regexp.MustCompile(LOOSE_HOST_PATTERN)
	LOOSE_HOST_PATTERN_REGEX   = regexp.MustCompile(LOOSE_HOST_PATTERN_PATTERN)
	LOOSE_URL_EXPR_REGEX       = regexp.MustCompile(LOOSE_URL_EXPR_PATTERN)
	ContainsSpace              = regexp.MustCompile(`\s`).MatchString
	DATE_LITERAL_REGEX         = regexp.MustCompile(DATE_LITERAL_PATTERN)
	STRICT_EMAIL_ADDRESS_REGEX = regexp.MustCompile(STRICT_EMAIL_ADDRESS_PATTERN)
)

// parses a file module, resultErr is either a non-syntax error or an aggregation of syntax errors (*ParsingErrorAggregation).
// result and resultErr can be both non-nil at the same time because syntax errors are also stored in nodes.
func ParseChunk(str string, fpath string, opts ...parserOptions) (result *Chunk, resultErr error) {
	_, result, resultErr = ParseChunk2(str, fpath, opts...)
	return
}

func ParseChunk2(str string, fpath string, opts ...parserOptions) (runes []rune, result *Chunk, resultErr error) {

	if int32(len(str)) > MAX_MODULE_BYTE_LEN {
		return nil, nil, &ParsingError{UnspecifiedParsingError, fmt.Sprintf("module'p.s code is too long (%d bytes)", len(str))}
	}

	runes = []rune(str)
	p := newParser(runes, opts...)

	defer func() {
		v := recover()
		if err, ok := v.(error); ok {
			resultErr = err
		}

		if resultErr != nil {
			resultErr = fmt.Errorf("%s: %s", resultErr.Error(), debug.Stack())
		}

		if result != nil {
			//we walk the AST and adds each node'p.s error to resultErr

			var aggregation *ParsingErrorAggregation

			Walk(result, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
				if reflect.ValueOf(node).IsNil() {
					return Continue, nil
				}

				nodeBase := node.Base()

				parsingErr := nodeBase.Err
				if parsingErr == nil {
					return Continue, nil
				}

				if aggregation == nil {
					aggregation = &ParsingErrorAggregation{}
				}

				//add location in error message
				line := int32(1)
				col := int32(1)
				i := int32(0)

				for i < nodeBase.Span.Start {
					if p.s[i] == '\n' {
						line++
						col = 1
					} else {
						col++
					}

					i++
				}

				aggregation.Errors = append(aggregation.Errors, parsingErr)
				aggregation.ErrorPositions = append(aggregation.ErrorPositions, SourcePositionRange{
					SourceName:  fpath,
					StartLine:   line,
					StartColumn: col,
					Span:        nodeBase.Span,
				})

				aggregation.message = fmt.Sprintf("%s\n%s:%d:%d: %s", aggregation.message, fpath, line, col, parsingErr.message)
				resultErr = aggregation
				return Continue, nil
			}, nil)
		}

	}()

	result, resultErr = p.parseChunk()
	return
}

// a parser parses a single Inox module, it can recover from errors
type parser struct {
	s         []rune //module's code
	i         int32  //rune index
	len       int32
	inPattern bool

	noCheckFuel          int
	remainingNoCheckFuel int //refueled after each context check.

	context context.Context
}

type parserOptions struct {
	//the context is checked each time the no check fuel is empty.
	//this option is ignored if <= 0 or context is nil.
	noCheckFuel int

	//this option is ignored if noCheckFuel is <= 0/
	context context.Context
}

func newParser(s []rune, opts ...parserOptions) *parser {
	p := &parser{
		s:                    s,
		i:                    0,
		len:                  int32(len(s)),
		noCheckFuel:          -1,
		remainingNoCheckFuel: -1,
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.context != nil && opt.noCheckFuel > 0 {
			p.context = opt.context
			p.noCheckFuel = opt.noCheckFuel
			p.remainingNoCheckFuel = opt.noCheckFuel
		}
	}
	return p
}

func (p *parser) panicIfContextDone() {
	if p.noCheckFuel == -1 {
		return
	}

	p.remainingNoCheckFuel--

	if p.remainingNoCheckFuel == 0 {
		p.remainingNoCheckFuel = p.noCheckFuel
		if p.context != nil {
			select {
			case <-p.context.Done():
				panic(p.context.Err())
			default:
				break
			}
		}
	}
}

func (p *parser) eatComment(tokens *[]Token) bool {
	p.panicIfContextDone()

	start := p.i

	if p.i < p.len-1 && isSpaceNotLF(p.s[p.i+1]) {
		p.i += 2
		for p.i < p.len && p.s[p.i] != '\n' {
			p.i++
		}
		*tokens = append(*tokens, Token{Type: COMMENT, Span: NodeSpan{start, p.i}, Raw: string(p.s[start:p.i])})
		return true
	} else {
		return false
	}
}

func (p *parser) eatSpace() {
	p.panicIfContextDone()

	for p.i < p.len && isSpaceNotLF(p.s[p.i]) {
		p.i++
	}
}

func (p *parser) eatSpaceNewline(tokens *[]Token) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			*tokens = append(*tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceComments(tokens *[]Token) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '#':
			if !p.eatComment(tokens) {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceNewlineComment(tokens *[]Token) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			*tokens = append(*tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case '#':
			if !p.eatComment(tokens) {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceNewlineCommaComment(tokens *[]Token) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			*tokens = append(*tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case ',':
			*tokens = append(*tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
		case '#':
			if !p.eatComment(tokens) {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}
}

func (p *parser) eatSpaceNewlineSemicolonComment(tokens *[]Token) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			*tokens = append(*tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case ';':
			*tokens = append(*tokens, Token{Type: SEMICOLON, Span: NodeSpan{p.i, p.i + 1}})
		case '#':
			if !p.eatComment(tokens) {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceNewlineComma(tokens *[]Token) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			*tokens = append(*tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case ',':
			*tokens = append(*tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
		default:
			break loop
		}
		p.i++
	}
}

func (p *parser) eatSpaceComma(tokens *[]Token) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case ',':
			*tokens = append(*tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) isExpressionEnd() bool {
	return p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i])
}

func (p *parser) parseCssSelectorElement(ignoreNextSpace bool) (node Node, isSpace bool) {
	p.panicIfContextDone()

	start := p.i
	switch p.s[p.i] {
	case '>', '~', '+':
		name := string(p.s[p.i])
		p.i++
		return &CssCombinator{
			NodeBase{
				NodeSpan{p.i - 1, p.i},
				nil,
				nil,
			},
			name,
		}, false
	case '.':
		p.i++
		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			return &CssClassSelector{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_CSS_CLASS_SELECTOR_NAME_EXPECTED},
					nil,
				},
			}, false
		}

		p.i++
		for p.i < p.len && isIdentChar(p.s[p.i]) {
			p.i++
		}

		return &CssClassSelector{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Name: string(p.s[start+1 : p.i]),
		}, false
	case '#': // id
		p.i++
		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			return &CssIdSelector{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_CSS_ID_SELECTOR_NAME_EXPECTED},
					nil,
				},
			}, false
		}

		p.i++
		for p.i < p.len && isIdentChar(p.s[p.i]) {
			p.i++
		}

		return &CssIdSelector{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Name: string(p.s[start+1 : p.i]),
		}, false
	case '[': //atribute selector
		p.i++

		makeNode := func(err string) Node {
			return &CssAttributeSelector{
				NodeBase: NodeBase{
					NodeSpan{p.i - 1, p.i},
					&ParsingError{UnspecifiedParsingError, err},
					nil,
				},
			}
		}

		if p.i >= p.len {
			return makeNode(UNTERMINATED_CSS_ATTR_SELECTOR_NAME_EXPECTED), false
		}

		if !isAlpha(p.s[p.i]) {
			return makeNode(CSS_ATTRIBUTE_NAME_SHOULD_START_WITH_ALPHA_CHAR), false
		}

		name := p.parseIdentStartingExpression()

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

		value, _ := p.parseExpression()

		if p.i >= p.len || p.s[p.i] != ']' {
			return makeNode(UNTERMINATED_CSS_ATTRIBUTE_SELECTOR_MISSING_BRACKET), false
		}
		p.i++

		return &CssAttributeSelector{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			AttributeName: name.(*IdentifierLiteral),
			Pattern:       pattern,
			Value:         value,
		}, false

	case ':':
		p.i++
		makeErr := func(err string) *ParsingError {
			return &ParsingError{UnspecifiedParsingError, err}

		}
		if p.i >= p.len {
			return &InvalidCSSselectorNode{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					makeErr(INVALID_CSS_SELECTOR),
					nil,
				},
			}, false
		}

		if p.s[p.i] != ':' { //pseudo class
			nameStart := p.i
			p.i++

			if p.i >= p.len || !isAlpha(p.s[p.i]) {
				return &CssPseudoClassSelector{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						makeErr(INVALID_CSS_CLASS_SELECTOR_INVALID_NAME),
						nil,
					},
				}, false
			}

			p.i++
			for p.i < p.len && isIdentChar(p.s[p.i]) {
				p.i++
			}

			nameEnd := p.i

			return &CssPseudoClassSelector{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					nil,
					nil,
				},
				Name: string(p.s[nameStart:nameEnd]),
			}, false
		}

		p.i++

		//pseudo element
		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			return &CssPseudoElementSelector{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					makeErr(INVALID_PSEUDO_CSS_SELECTOR_INVALID_NAME),
					nil,
				},
			}, false
		}

		nameStart := p.i

		p.i++
		for p.i < p.len && isIdentChar(p.s[p.i]) {
			p.i++
		}

		nameEnd := p.i

		return &CssPseudoElementSelector{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Name: string(p.s[nameStart:nameEnd]),
		}, false
	case ' ':
		p.i++
		p.eatSpace()
		if p.i >= p.len || isNonSpaceCSSCombinator(p.s[p.i]) || ignoreNextSpace {
			return nil, true
		}

		return &CssCombinator{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Name: " ",
		}, false
	case '*':
		p.i++
		return &CssTypeSelector{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Name: "*",
		}, false
	}

	if p.i < p.len && isAlpha(p.s[p.i]) {
		p.i++
		for p.i < p.len && isIdentChar(p.s[p.i]) {
			p.i++
		}

		return &CssTypeSelector{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Name: string(p.s[start:p.i]),
		}, false
	}

	return &InvalidCSSselectorNode{
		NodeBase: NodeBase{
			NodeSpan{start - 1, p.i},
			&ParsingError{UnspecifiedParsingError, EMPTY_CSS_SELECTOR},
			nil,
		},
	}, false

}

func (p *parser) parseTopCssSelector(start int32) Node {
	p.panicIfContextDone()

	//p.s!
	tokens := []Token{
		{Type: CSS_SELECTOR_PREFIX, Span: NodeSpan{start, p.i}},
	}

	if p.i >= p.len {
		return &InvalidCSSselectorNode{
			NodeBase: NodeBase{
				NodeSpan{p.i - 1, p.i},
				&ParsingError{UnspecifiedParsingError, EMPTY_CSS_SELECTOR},
				tokens,
			},
		}
	}

	var elements []Node
	var ignoreNextSpace bool

	for p.i < p.len && p.s[p.i] != '\n' {
		if p.s[p.i] == '!' {
			p.i++
			break
		}
		e, isSpace := p.parseCssSelectorElement(ignoreNextSpace)

		if !isSpace {
			elements = append(elements, e)
			_, ignoreNextSpace = e.(*CssCombinator)

			if e.Base().Err != nil {
				p.i++
			}
		} else {
			ignoreNextSpace = false
		}
	}

	return &CssSelectorExpression{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			nil,
			nil,
		},
		Elements: elements,
	}
}

func (p *parser) parseBlock() *Block {
	p.panicIfContextDone()

	openingBraceIndex := p.i
	prevStmtEndIndex := int32(-1)
	var prevStmtErrKind ParsingErrorKind

	p.i++
	var (
		parsingErr      *ParsingError
		valuelessTokens = []Token{
			{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{openingBraceIndex, openingBraceIndex + 1}},
		}
		stmts []Node
	)

	for p.i < p.len && p.s[p.i] != '}' {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {
			stmts = append(stmts, &UnknownNode{
				NodeBase: NodeBase{
					Span:            NodeSpan{p.i, p.i + 1},
					Err:             &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i])},
					ValuelessTokens: []Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
			continue
		}

		var stmtErr *ParsingError
		p.eatSpaceNewlineSemicolonComment(&valuelessTokens)

		if p.i >= p.len || p.s[p.i] == '}' {
			break
		}

		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		stmt := p.parseStatement()

		prevStmtEndIndex = p.i
		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.kind
		}

		if stmtErr != nil && (stmt.Base().Err == nil || stmt.Base().Err.kind != InvalidNext) {
			stmt.BasePtr().Err = stmtErr
		}

		stmts = append(stmts, stmt)
		p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
	}

	closingBraceIndex := p.i

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE}

	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{closingBraceIndex, closingBraceIndex + 1}})
		p.i++
	}

	end := p.i

	return &Block{
		NodeBase: NodeBase{
			Span:            NodeSpan{openingBraceIndex, end},
			Err:             parsingErr,
			ValuelessTokens: valuelessTokens,
		},
		Statements: stmts,
	}
}

// parsePathExpressionSlices parses the slices in a path expression.
// example: /{$HOME}/.cache -> [ / , $HOME , /.cache ]
func (p *parser) parsePathExpressionSlices(start int32, exclEnd int32, tokens *[]Token) []Node {
	p.panicIfContextDone()

	slices := make([]Node, 0)
	index := start
	sliceStart := start
	inInterpolation := false

	for index < exclEnd {

		if inInterpolation {
			if p.s[index] == '}' || index == exclEnd-1 { //end if interpolation
				missingClosingBrace := false

				if index == exclEnd-1 && p.s[index] != '}' {
					index++
					missingClosingBrace = true
				} else {
					*tokens = append(*tokens, Token{
						Type: SINGLE_INTERP_CLOSING_BRACE,
						Span: NodeSpan{index, index + 1},
					})
				}

				interpolation := string(p.s[sliceStart:index])

				if interpolation != "" && interpolation[0] == ':' { //named segment
					i := int32(1)
					ok := true
					for i < int32(len(interpolation)) {
						if !isIdentChar(rune(interpolation[i])) {
							slices = append(slices, &UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{sliceStart, exclEnd},
									&ParsingError{UnspecifiedParsingError, INVALID_NAMED_SEGMENT_COLON_SHOULD_BE_FOLLOWED_BY_A_NAME},
									nil,
								},
							})
							ok = false
							break
						}
						i++
					}

					if ok {
						var err *ParsingError
						if len(interpolation) == 1 {
							err = &ParsingError{UnspecifiedParsingError, INVALID_NAMED_SEGMENT_COLON_SHOULD_BE_FOLLOWED_BY_A_NAME}
						}
						slices = append(slices, &NamedPathSegment{
							NodeBase: NodeBase{
								NodeSpan{sliceStart, index},
								err,
								nil,
							},
							Name: interpolation[1:],
						})
					}

				} else {

					expr, ok := ParseExpression(interpolation)

					if !ok {
						span := NodeSpan{sliceStart, index}
						err := &ParsingError{UnspecifiedParsingError, INVALID_PATH_INTERP}

						if len(interpolation) == 0 {
							err.message = EMPTY_PATH_INTERP
						}

						slices = append(slices, &UnknownNode{
							NodeBase: NodeBase{
								span,
								err,
								[]Token{{Type: INVALID_INTERP_SLICE, Span: span, Raw: string(p.s[sliceStart:index])}},
							},
						})

					} else {
						shiftNodeSpans(expr, sliceStart)
						slices = append(slices, expr)

						if missingClosingBrace {
							slices = append(slices, &PathSlice{
								NodeBase: NodeBase{
									NodeSpan{index, index},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP_MISSING_CLOSING_BRACE},
									nil,
								},
							})
						}
					}

				}
				inInterpolation = false
				sliceStart = index + 1
			} else if !isInterpolationAllowedChar(p.s[index]) {
				// we eat all the interpolation

				j := index
				for j < exclEnd && p.s[j] != '}' {
					j++
				}

				slices = append(slices, &UnknownNode{
					NodeBase: NodeBase{
						NodeSpan{sliceStart, j},
						&ParsingError{UnspecifiedParsingError, PATH_INTERP_EXPLANATION},
						[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{sliceStart, j}, Raw: string(p.s[sliceStart:j])}},
					},
				})

				if j < exclEnd { // '}'
					*tokens = append(*tokens, Token{
						Type: SINGLE_INTERP_CLOSING_BRACE,
						Span: NodeSpan{j, j + 1},
					})
					j++
				}

				inInterpolation = false
				sliceStart = j
				index = j
				continue
			}

		} else if p.s[index] == '{' { //start of a new interpolation
			slice := string(p.s[sliceStart:index]) //previous cannot be an interpolation

			*tokens = append(*tokens, Token{
				Type: SINGLE_INTERP_OPENING_BRACE,
				Span: NodeSpan{index, index + 1},
			})

			slices = append(slices, &PathSlice{
				NodeBase: NodeBase{
					NodeSpan{sliceStart, index},
					nil,
					nil,
				},
				Value: slice,
			})

			sliceStart = index + 1
			inInterpolation = true

			//if the interpolation is unterminated
			if index == p.len-1 {
				slices = append(slices, &PathSlice{
					NodeBase: NodeBase{
						NodeSpan{sliceStart, sliceStart},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
						nil,
					},
					Value: string(p.s[sliceStart:sliceStart]),
				})

				return slices
			}
		}
		index++
	}

	if inInterpolation {
		slices = append(slices, &PathSlice{
			NodeBase: NodeBase{
				NodeSpan{sliceStart, index},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
				nil,
			},
		})
	} else if sliceStart != index {
		slices = append(slices, &PathSlice{
			NodeBase: NodeBase{
				NodeSpan{sliceStart, index},
				nil,
				nil,
			},
			Value: string(p.s[sliceStart:index]),
		})
	}
	return slices
}

func (p *parser) parseQueryParameterValueSlices(start int32, exclEnd int32, tokens *[]Token) []Node {
	p.panicIfContextDone()

	slices := make([]Node, 0)
	index := start
	sliceStart := start
	inInterpolation := false

	for index < exclEnd {

		if inInterpolation {
			if p.s[index] == '}' || index == exclEnd-1 { //end of interpolation
				missingClosingBrace := false
				if index == exclEnd-1 && p.s[index] != '}' {
					index++
					missingClosingBrace = true
				} else {
					*tokens = append(*tokens, Token{
						Type: SINGLE_INTERP_CLOSING_BRACE,
						Span: NodeSpan{index, index + 1},
					})
				}

				interpolation := string(p.s[sliceStart:index])

				expr, ok := ParseExpression(interpolation)

				if !ok {
					span := NodeSpan{sliceStart, index}
					err := &ParsingError{UnspecifiedParsingError, INVALID_QUERY_PARAM_INTERP}

					if len(interpolation) == 0 {
						err.message = EMPTY_QUERY_PARAM_INTERP
					}

					slices = append(slices, &UnknownNode{
						NodeBase: NodeBase{
							span,
							err,
							[]Token{{Type: INVALID_INTERP_SLICE, Span: span, Raw: string(p.s[sliceStart:index])}},
						},
					})
				} else {
					shiftNodeSpans(expr, sliceStart)
					slices = append(slices, expr)

					if missingClosingBrace {
						slices = append(slices, &URLQueryParameterValueSlice{
							NodeBase: NodeBase{
								NodeSpan{index, index},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUERY_PARAM_INTERP_MISSING_CLOSING_BRACE},
								nil,
							},
						})
					}
				}

				inInterpolation = false
				sliceStart = index + 1
			} else if !isInterpolationAllowedChar(p.s[index]) {
				// we eat all the interpolation

				j := index
				for j < exclEnd && p.s[j] != '}' {
					j++
				}

				slices = append(slices, &URLQueryParameterValueSlice{
					NodeBase: NodeBase{
						NodeSpan{sliceStart, j},
						&ParsingError{UnspecifiedParsingError, QUERY_PARAM_INTERP_EXPLANATION},
						nil,
					},
					Value: string(p.s[sliceStart:j]),
				})

				if j < exclEnd { // '}'
					*tokens = append(*tokens, Token{
						Type: SINGLE_INTERP_CLOSING_BRACE,
						Span: NodeSpan{j, j + 1},
					})
					j++
				}

				inInterpolation = false
				sliceStart = j
				index = j
				continue
			}

		} else if p.s[index] == '{' { //start of interpolation
			*tokens = append(*tokens, Token{
				Type: SINGLE_INTERP_OPENING_BRACE,
				Span: NodeSpan{index, index + 1},
			})

			slice := string(p.s[sliceStart:index]) //previous cannot be an interpolation
			slices = append(slices, &URLQueryParameterValueSlice{
				NodeBase: NodeBase{
					NodeSpan{sliceStart, index},
					nil,
					nil,
				},
				Value: slice,
			})

			sliceStart = index + 1
			inInterpolation = true

			//if the interpolation is unterminated
			if index == p.len-1 {
				slices = append(slices, &URLQueryParameterValueSlice{
					NodeBase: NodeBase{
						NodeSpan{sliceStart, sliceStart},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUERY_PARAM_INTERP},
						nil,
					},
					Value: string(p.s[sliceStart:sliceStart]),
				})

				return slices
			}
		}
		index++
	}

	if sliceStart != index {
		slices = append(slices, &URLQueryParameterValueSlice{
			NodeBase: NodeBase{
				NodeSpan{sliceStart, index},
				nil,
				nil,
			},
			Value: string(p.s[sliceStart:index]),
		})
	}
	return slices
}

func (p *parser) parseDotStartingExpression() Node {
	p.panicIfContextDone()

	if p.i < p.len-1 {
		if p.s[p.i+1] == '/' || p.i < p.len-2 && p.s[p.i+1] == '.' && p.s[p.i+2] == '/' {
			return p.parsePathLikeExpression(false)
		}
		switch p.s[p.i+1] {
		case '{':
			return p.parseKeyList()
		case '.':
			start := p.i
			p.i += 2

			upperBound, _ := p.parseExpression()
			expr := &UpperBoundRangeExpression{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					nil,
					[]Token{{Type: TWO_DOTS, Span: NodeSpan{start, start + 2}}},
				},
				UpperBound: upperBound,
			}

			return expr
		default:
			r := p.s[p.i+1]
			if isIdentChar(r) && !isDecDigit(r) {
				start := p.i

				p.i++
				for p.i < p.len && isIdentChar(p.s[p.i]) {
					p.i++
				}

				return &PropertyNameLiteral{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						nil,
						nil,
					},
					Name: string(p.s[start+1 : p.i]),
				}
			}
		}
	}

	p.i++
	return &UnknownNode{
		NodeBase: NodeBase{
			Span:            NodeSpan{p.i - 1, p.i},
			Err:             &ParsingError{UnspecifiedParsingError, DOT_SHOULD_BE_FOLLOWED_BY},
			ValuelessTokens: []Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i - 1, p.i}, Raw: "."}},
		},
	}
}

// parseDashStartingExpression parses all expressions that start with a dash: numbers, numbers ranges, options and unquoted strings
func (p *parser) parseDashStartingExpression() Node {
	p.panicIfContextDone()

	__start := p.i

	p.i++
	if p.i >= p.len || unicode.IsSpace(p.s[p.i]) {
		return &UnquotedStringLiteral{
			NodeBase: NodeBase{
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

	singleDash := true

	if p.s[p.i] == '-' {
		singleDash = false
		p.i++
	}

	if p.i >= p.len || unicode.IsSpace(p.s[p.i]) {
		return &UnquotedStringLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Raw:   "--",
			Value: string(p.s[__start:p.i]),
		}
	}

	nameStart := p.i

	if p.i >= p.len || IsDelim(p.s[p.i]) {
		return &UnquotedStringLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Raw:   string(p.s[__start:p.i]),
			Value: string(p.s[__start:p.i]),
		}
	}

	if !isAlpha(p.s[p.i]) && !isDecDigit(p.s[p.i]) {
		if unicode.IsSpace(p.s[p.i]) || isUnquotedStringChar(p.s[p.i]) {
			return p.parseUnquotedStringLiteralAndEmailAddress(__start)
		}
		return &FlagLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, OPTION_NAME_CAN_ONLY_CONTAIN_ALPHANUM_CHARS},
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

		return &FlagLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, p.i},
			},
			Name:       name,
			SingleDash: singleDash,
			Raw:        string(p.s[__start:p.i]),
		}
	}

	tokens := []Token{{Type: EQUAL, Span: NodeSpan{p.i, p.i + 1}}}
	p.i++

	if p.i >= p.len {
		return &OptionExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{__start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_OPION_EXPR_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR},
				ValuelessTokens: tokens,
			},
			Name:       name,
			SingleDash: singleDash,
		}
	}

	value, _ := p.parseExpression()

	return &OptionExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{__start, p.i},
			ValuelessTokens: tokens,
		},
		Name:       name,
		Value:      value,
		SingleDash: singleDash,
	}
}

func (p *parser) parseLazyAndHostAliasStuff() Node {
	p.panicIfContextDone()

	start := p.i
	p.i++
	if p.i >= p.len {
		return &UnknownNode{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
				ValuelessTokens: []Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"}},
			},
		}
	}

	if p.s[p.i] == '(' { //lazy expression
		//no increment on purpose

		e, _ := p.parseExpression()
		return &LazyExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: []Token{{Type: AT_SIGN, Span: NodeSpan{start, start + 1}}},
			},
			Expression: e,
		}
	} else if p.s[p.i] >= 'a' && p.s[p.i] <= 'z' { //host alias definition | url expression starting with an alias
		j := p.i
		p.i--

		for j < p.len && isIdentChar(p.s[j]) {
			j++
		}

		aliasEndIndex := j

		for j < p.len && isSpaceNotLF(p.s[j]) {
			j++
		}

		if j >= p.len || (p.s[j] != '=' && isUnpairedOrIsClosingDelim(p.s[j])) {
			p.i = j
			return &InvalidAliasRelatedNode{
				NodeBase: NodeBase{
					NodeSpan{start, j},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_ALIAS_RELATED_LITERAL},
					nil,
				},
				Raw: string(p.s[start:j]),
			}
		}

		//@alias = <host>
		if p.s[j] == '=' {
			equalPos := j

			left := &AtHostLiteral{
				NodeBase: NodeBase{
					NodeSpan{start, aliasEndIndex},
					nil,
					nil,
				},
				Value: string(p.s[start:aliasEndIndex]),
			}

			p.i = j + 1

			p.eatSpace()
			var parsingErr *ParsingError
			var right Node
			var end int32

			if p.i >= p.len {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_ALIAS_DEF_MISSING_VALUE_AFTER_EQL_SIGN}
				end = p.len
			} else {
				right, _ = p.parseExpression()
				end = right.Base().Span.End
			}

			return &HostAliasDefinition{
				NodeBase: NodeBase{
					NodeSpan{start, end},
					parsingErr,
					[]Token{{Type: EQUAL, Span: NodeSpan{equalPos, equalPos + 1}}},
				},
				Left:  left,
				Right: right,
			}
		}

		return p.parseURLLike(start)
	}

	return &UnknownNode{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			Err:             &ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
			ValuelessTokens: []Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{start, p.i}, Raw: "@"}},
		},
	}
}

func (p *parser) parseQuotedStringLiteral() *QuotedStringLiteral {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *ParsingError
	var value string
	var raw string

	p.i++

	for p.i < p.len && p.s[p.i] != '\n' && (p.s[p.i] != '"' || countPrevBackslashes(p.s, p.i)%2 == 1) {
		p.i++
	}

	if p.i >= p.len || (p.i < p.len && p.s[p.i] != '"') {
		raw = string(p.s[start:p.i])
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STRING_LIT}
	} else {
		p.i++

		raw = string(p.s[start:p.i])
		err := json.Unmarshal([]byte(raw), &value)

		if err != nil {
			parsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidStringLitJSON(err.Error())}
		}
	}

	return &QuotedStringLiteral{
		NodeBase: NodeBase{
			Span: NodeSpan{start, p.i},
			Err:  parsingErr,
		},
		Raw:   raw,
		Value: value,
	}
}

func (p *parser) parseUnquotedStringLiteralAndEmailAddress(start int32) Node {
	p.panicIfContextDone()

	p.i++

	var parsingErr *ParsingError
	for p.i < p.len &&
		(isUnquotedStringChar(p.s[p.i]) || (p.s[p.i] == '\\' && p.i < p.len-1 && p.s[p.i+1] == ':')) {
		if p.s[p.i] == '\\' {
			p.i++
		}
		p.i++
	}

	raw := string(p.s[start:p.i])
	value := strings.ReplaceAll(raw, "\\", "")

	base := NodeBase{
		Span: NodeSpan{start, p.i},
		Err:  parsingErr,
	}

	if STRICT_EMAIL_ADDRESS_REGEX.MatchString(raw) {
		return &EmailAddressLiteral{
			NodeBase: base,
			Value:    raw,
		}
	}

	return &UnquotedStringLiteral{
		NodeBase: base,
		Raw:      raw,
		Value:    value,
	}
}

// parsePathLikeExpression parses paths, path expressions, simple path patterns and named segment path patterns
func (p *parser) parsePathLikeExpression(isPattern bool) Node {
	p.panicIfContextDone()

	start := p.i
	if isPattern {
		p.i++
	}

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
		for p.i < p.len && p.s[p.i] != '`' {
			//no escape
			p.i++
		}
		if p.i < p.len && p.s[p.i] == '`' {
			p.i++
		}
	} else {
		// limit to ascii ? limit to ascii alphanum & some chars ?
		for p.i < p.len && p.s[p.i] != '\n' && !unicode.IsSpace(p.s[p.i]) && (!IsDelim(p.s[p.i]) || p.s[p.i] == '{') {

			//TODO: fix
			if p.s[p.i] == '{' {
				p.i++
				for p.i < p.len && p.s[p.i] != '\n' && p.s[p.i] != '}' { //ok since '}' is not allowed in interpolations for now
					p.i++
				}
				if p.i < p.len && p.s[p.i] == '}' {
					p.i++
				}
			} else {
				p.i++
			}
		}
	}

	runes := p.s[start:p.i]
	raw := string(runes)

	_path := p.s[pathStart:p.i]

	var clean []rune
	for _, r := range _path {
		if r == '`' {
			continue
		}
		clean = append(clean, r)
	}
	value := string(clean)

	base := NodeBase{
		Span: NodeSpan{start, p.i},
	}

	slices := p.parsePathExpressionSlices(pathStart, p.i, &base.ValuelessTokens)
	hasInterpolationsOrNamedSegments := len32(slices) > 1
	hasGlobbing := false

search_for_globbing:
	for _, slice := range slices {
		if pathSlice, ok := slice.(*PathSlice); ok {

			for i, e := range pathSlice.Value {
				if (e == '[' || e == '*' || e == '?') && countPrevBackslashes(p.s, start+int32(i))%2 == 0 {
					hasGlobbing = true
					break search_for_globbing
				}
			}
		}
	}

	isPrefixPattern := isPattern && strings.Contains(value, "/...")

	if isPrefixPattern && (!strings.HasSuffix(value, "/...") || strings.Contains(strings.TrimSuffix(value, "/..."), "/...")) {
		base.Err = &ParsingError{UnspecifiedParsingError, fmtSlashDotDotDotCanOnlyBePresentAtEndOfPathPattern(value)}
	}

	if !isPattern && isPrefixPattern && hasGlobbing {
		base.Err = &ParsingError{UnspecifiedParsingError, fmtPrefixPattCannotContainGlobbingPattern(value)}
		return &InvalidPathPattern{
			NodeBase: base,
			Value:    value,
		}
	}

	if isPattern {

		if !hasInterpolationsOrNamedSegments {
			if isAbsolute {
				return &AbsolutePathPatternLiteral{
					NodeBase: base,
					Raw:      raw,
					Value:    value,
				}
			}
			return &RelativePathPatternLiteral{
				NodeBase: base,
				Raw:      raw,
				Value:    value,
			}
		}

		base.ValuelessTokens = append([]Token{{Type: PERCENT_SYMBOL, Span: NodeSpan{start, start + 1}}}, base.ValuelessTokens...)

		//named segment path pattern literal & path pattern expressions
		containNamedSegments := false
		containInterpolations := false

		//search for named segments & interpolations + turn path slices into path pattern slices
		for i, e := range slices {

			switch E := e.(type) {
			case *NamedPathSegment:
				containNamedSegments = true
			case *PathSlice:
				slices[i] = &PathPatternSlice{
					NodeBase: E.NodeBase,
					Value:    E.Value,
				}
			default:
				containInterpolations = true
			}

			if containNamedSegments && containInterpolations {
				base.Err = &ParsingError{UnspecifiedParsingError, CANNOT_MIX_PATH_INTER_PATH_NAMED_SEGMENT}
				return &NamedSegmentPathPatternLiteral{
					NodeBase: base,
					Raw:      raw,
					Slices:   slices,
				}
			}
		}

		if containNamedSegments { //named segment path pattern

			for j := 0; j < len(slices); j++ {
				_, isNamedSegment := slices[j].(*NamedPathSegment)

				if isNamedSegment {

					prev := slices[j-1].(*PathPatternSlice).Value
					if prev[int32(len(prev))-1] != '/' {

						base.Err = &ParsingError{UnspecifiedParsingError, INVALID_PATH_PATT_NAMED_SEGMENTS}

						return &NamedSegmentPathPatternLiteral{
							NodeBase: base,
							Slices:   slices,
						}
					}
					if j < len(slices)-1 {
						next := slices[j+1].(*PathPatternSlice).Value
						if next[0] != '/' {
							base.Err = &ParsingError{UnspecifiedParsingError, INVALID_PATH_PATT_NAMED_SEGMENTS}

							return &NamedSegmentPathPatternLiteral{
								NodeBase: base,
								Slices:   slices,
							}
						}
					}
				}
			}

			return &NamedSegmentPathPatternLiteral{
				NodeBase:    base,
				Slices:      slices,
				Raw:         raw,
				StringValue: "%" + value,
			}
		} else {

			return &PathPatternExpression{
				NodeBase: base,
				Slices:   slices,
			}
		}

	}

	for _, e := range slices {

		switch e.(type) {
		case *NamedPathSegment:
			if base.Err == nil {
				base.Err = &ParsingError{UnspecifiedParsingError, ONLY_PATH_PATTERNS_CAN_CONTAIN_NAMED_SEGMENTS}
			}
		}
	}

	if hasInterpolationsOrNamedSegments {

		if isAbsolute {
			return &AbsolutePathExpression{
				NodeBase: base,
				Slices:   slices,
			}
		}
		return &RelativePathExpression{
			NodeBase: base,
			Slices:   slices,
		}
	}

	if isAbsolute {
		return &AbsolutePathLiteral{
			NodeBase: base,
			Raw:      raw,
			Value:    value,
		}
	}
	return &RelativePathLiteral{
		NodeBase: base,
		Raw:      raw,
		Value:    value,
	}

}

func CheckHost(u string) *ParsingError {
	hasScheme := u[0] != ':'

	_, hostPart, _ := strings.Cut(u, "://")

	var testedUrl = u
	if !hasScheme {
		testedUrl = "https" + u
	}

	if parsed, err := url.Parse(testedUrl); err != nil ||
		parsed.Host != hostPart || /* too strict ? */
		parsed.User.String() != "" ||
		parsed.RawPath != "" ||
		parsed.RawQuery != "" ||
		parsed.RawFragment != "" {
		return &ParsingError{UnspecifiedParsingError, INVALID_HOST_LIT}
	}

	return nil
}

func CheckHostPattern(u string) (parsingErr *ParsingError) {
	hasScheme := u[0] != ':'
	pattern := u[strings.Index(u, "://")+3:]
	pattern = strings.Split(pattern, ":")[0]
	parts := strings.Split(pattern, ".")

	if len32(parts) == 1 {
		if parts[0] != "**" {
			if parts[0] == "*" {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT_SUGGEST_DOUBLE_STAR}
			} else {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT}
			}
		}
	} else if strings.Count(u, "**") > 1 {
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT_AT_MOST_ONE_DOUBLE_STAR}
	} else if strings.Count(u, "***") != 0 {
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT_ONLY_SINGLE_OR_DOUBLE_STAR}
	} else {
		areAllStars := true

		for _, part := range parts {
			if part != "*" && part != "**" {
				areAllStars = false
				break
			}
		}

		if areAllStars {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT}
		} else {

			var testedUrl = u
			if !hasScheme {
				testedUrl = "https" + u
			}

			replaced := strings.ReplaceAll(testedUrl, "*", "com")
			if _, err := url.Parse(replaced); err != nil {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT}
			}
		}
	}

	return
}

func CheckURLPattern(u string) *ParsingError {
	isPrefixPattern := strings.HasSuffix(u, "/...")

	if strings.Contains(u, "...") && (!isPrefixPattern || strings.Count(u, "...") != 1) {
		lastSlashI := strings.LastIndex(u, "/")

		c := int32(0)
		for _, r := range u[lastSlashI+1:] {
			if r == '.' {
				if c >= 3 {
					return &ParsingError{UnspecifiedParsingError, URL_PATTERNS_CANNOT_END_WITH_SLASH_MORE_THAN_4_DOTS}
				}
				c++
			}
		}

		return &ParsingError{UnspecifiedParsingError, URL_PATTERN_SUBSEQUENT_DOT_EXPLANATION}
	}

	return nil
}

// parseURLLike parses URLs, URL expressions and Hosts
func (p *parser) parseURLLike(start int32) Node {
	p.panicIfContextDone()

	startsWithAtHost := p.s[start] == '@'

	if !startsWithAtHost {
		p.i += 3 // ://
	}
	afterSchemeIndex := p.i

	//we eat until we encounter a space or a delimiter different from ':' and '{'
	for p.i < p.len && p.s[p.i] != '\n' && !unicode.IsSpace(p.s[p.i]) && (!IsDelim(p.s[p.i]) || p.s[p.i] == ':' || p.s[p.i] == '{') {
		if p.s[p.i] == '{' {
			p.i++
			for p.i < p.len && p.s[p.i] != '\n' && p.s[p.i] != '}' { //ok since '}' is not allowed in interpolations for now
				p.i++
			}
			if p.i < p.len && p.s[p.i] == '}' {
				p.i++
			}
		} else {
			p.i++
		}
	}

	u := string(p.s[start:p.i])
	span := NodeSpan{start, p.i}

	//scheme literal
	if !startsWithAtHost && p.i == afterSchemeIndex {
		scheme := u[:int32(len(u))-3]
		var parsingErr *ParsingError
		if scheme == "" {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_SCHEME_LIT_MISSING_SCHEME}
		}

		return &SchemeLiteral{
			NodeBase: NodeBase{
				Span: span,
				Err:  parsingErr,
			},
			Name: scheme,
		}
	}

	switch {
	case LOOSE_HOST_REGEX.MatchString(u):

		parsingErr := CheckHost(u)

		return &HostLiteral{
			NodeBase: NodeBase{
				Span: span,
				Err:  parsingErr,
			},
			Value: u,
		}

	case LOOSE_URL_EXPR_REGEX.MatchString(u) && (startsWithAtHost || strings.Count(u, "{") >= 1): //url expressions
		var parsingErr *ParsingError
		pathStart := afterSchemeIndex
		pathExclEnd := afterSchemeIndex
		hasQuery := strings.Contains(u, "?")
		hostInterpolationStart := int32(-1)
		var valuelessTokens []Token

		if hasQuery {
			for p.s[pathExclEnd] != '?' {
				pathExclEnd++
			}
		} else {
			pathExclEnd = p.i
		}

		if !startsWithAtHost && p.s[afterSchemeIndex] == '{' { //host interpolation
			valuelessTokens = append(valuelessTokens, Token{
				Type: SINGLE_INTERP_OPENING_BRACE,
				Span: NodeSpan{afterSchemeIndex, afterSchemeIndex + 1},
			})

			hostInterpolationStart = pathStart
			pathStart++
			for pathStart < pathExclEnd && p.s[pathStart] != '}' {
				pathStart++
			}

			//there is necessarily a '}' because it's in the regex

			valuelessTokens = append(valuelessTokens, Token{
				Type: SINGLE_INTERP_CLOSING_BRACE,
				Span: NodeSpan{pathStart, pathStart + 1},
			})
			pathStart++

		} else {
			//we increment pathStart while we are in the host part
			for pathStart < pathExclEnd && p.s[pathStart] != '/' && p.s[pathStart] != '{' {
				pathStart++
			}
		}

		if pathStart == afterSchemeIndex {
			pathStart = pathExclEnd
		}

		slices := p.parsePathExpressionSlices(pathStart, pathExclEnd, &valuelessTokens)

		queryParams := make([]Node, 0)
		if hasQuery { //parse query

			_, err := url.ParseQuery(string(p.s[pathExclEnd+1 : start+int32(len(u))]))
			if err != nil {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_QUERY}
			}

			j := pathExclEnd + 1
			queryEnd := start + int32(len(u))

			for j < queryEnd {
				keyStart := j
				for j < queryEnd && p.s[j] != '=' {
					j++
				}
				if j >= queryEnd {
					parsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidQueryMissingEqualSignAfterKey(string(p.s[keyStart:j]))}
				}

				keyRunes := p.s[keyStart:j]
				key := string(keyRunes)
				j++

				//check key

				if containsNotEscapedBracket(keyRunes) || containsNotEscapedDollar(keyRunes) {
					parsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidQueryKeysCannotContaintDollar(string(p.s[keyStart:j]))}
				}

				//value

				valueStart := j
				slices := make([]Node, 0)

				if j < queryEnd && p.s[j] != '&' {

					for j < queryEnd && p.s[j] != '&' {
						j++
					}
					slices = p.parseQueryParameterValueSlices(valueStart, j, &valuelessTokens)
				}

				queryParams = append(queryParams, &URLQueryParameter{
					NodeBase: NodeBase{
						NodeSpan{keyStart, j},
						nil,
						nil,
					},
					Name:  key,
					Value: slices,
				})

				for j < queryEnd && p.s[j] == '&' {
					j++
				}
			}

		}

		var hostPart Node
		hostPartString := string(p.s[span.Start:pathStart])
		hostPartBase := NodeBase{
			NodeSpan{span.Start, pathStart},
			nil,
			nil,
		}

		if hostInterpolationStart > 0 {
			e, ok := ParseExpression(string(p.s[hostInterpolationStart+1 : pathStart-1]))
			hostPart = &HostExpression{
				NodeBase: hostPartBase,
				Scheme: &SchemeLiteral{
					NodeBase: NodeBase{NodeSpan{span.Start, afterSchemeIndex}, nil, nil},
					Name:     string(p.s[span.Start : afterSchemeIndex-3]),
				},
				Host: e,
				Raw:  hostPartString,
			}
			shiftNodeSpans(e, hostInterpolationStart+1)

			if !ok && parsingErr == nil {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_INTERPOLATION}
			}
		} else if strings.Contains(hostPartString, "://") {
			hostPart = &HostLiteral{
				NodeBase: hostPartBase,
				Value:    hostPartString,
			}
		} else {
			hostPart = &AtHostLiteral{
				NodeBase: hostPartBase,
				Value:    hostPartString,
			}
		}

		return &URLExpression{
			NodeBase:    NodeBase{span, parsingErr, valuelessTokens},
			Raw:         u,
			HostPart:    hostPart,
			Path:        slices,
			QueryParams: queryParams,
		}
	case URL_REGEX.MatchString(u): //urls & url patterns
		parsed, err := url.Parse(u)
		if err != nil {
			return &InvalidURL{
				NodeBase: NodeBase{
					Span: span,
					Err:  &ParsingError{UnspecifiedParsingError, INVALID_URL},
				},
				Value: u,
			}
		}

		if strings.Contains(parsed.Path, "/") {
			return &URLLiteral{
				NodeBase: NodeBase{
					Span: span,
				},
				Value: u,
			}
		}

	}

	return &InvalidURL{
		NodeBase: NodeBase{
			Span: span,
			Err:  &ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST},
		},
		Value: u,
	}
}

// parseURLLike parses URLs pattenrs and host patterns
func (p *parser) parseURLLikePattern(start int32) Node {
	p.panicIfContextDone()

	c := int32(0)
	for p.i < p.len && p.s[p.i] == '/' {
		p.i++
		c++
	}

	if c != 2 {

		return &InvalidURLPattern{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST_PATT_SCHEME_SHOULD_BE_FOLLOWED_BY_COLON_SLASH_SLASH},
			},
		}
	}

	//we eat until we encounter a space or a delimiter different from ':' and '{'
	for p.i < p.len && !unicode.IsSpace(p.s[p.i]) && (!IsDelim(p.s[p.i]) || p.s[p.i] == ':' || p.s[p.i] == '{') {
		if p.s[p.i] == '{' {
			p.i++
			for p.i < p.len && p.s[p.i] != '}' { //ok since '}' is not allowed in interpolations for now
				p.i++
			}
			if p.i < p.len {
				p.i++
			}
		} else {
			p.i++
		}
	}

	raw := string(p.s[start:p.i])
	u := raw[1:]
	span := NodeSpan{start, p.i}

	if LOOSE_HOST_PATTERN_REGEX.MatchString(u) {

		parsingErr := CheckHostPattern(u)

		return &HostPatternLiteral{
			NodeBase: NodeBase{
				Span: span,
				Err:  parsingErr,
			},
			Value: u,
			Raw:   raw,
		}
	}

	var parsingErr *ParsingError

	if !URL_REGEX.MatchString(u) {
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_URL_PATT}
	} else {
		parsingErr = CheckURLPattern(u)
		if parsingErr == nil && strings.Contains(u, "?") {
			parsingErr = &ParsingError{UnspecifiedParsingError, URL_PATT_LITS_WITH_QUERY_PART_NOT_SUPPORTED_YET}
		}
	}

	return &URLPatternLiteral{
		NodeBase: NodeBase{
			Span: span,
			Err:  parsingErr,
		},
		Value: u,
		Raw:   raw,
	}

}

// parseIdentStartingExpression parses identifiers, identifier member expressions, true, false, nil and URL-like expressions
func (p *parser) parseIdentStartingExpression() Node {
	p.panicIfContextDone()

	start := p.i
	p.i++
	for p.i < p.len && isIdentChar(p.s[p.i]) {
		p.i++
	}

	name := string(p.s[start:p.i])
	ident := &IdentifierLiteral{
		NodeBase: NodeBase{
			Span: NodeSpan{start, p.i},
		},
		Name: name,
	}

	switch name {
	case "self":
		return &SelfExpression{
			NodeBase: NodeBase{
				Span: ident.Span,
			},
		}
	case "supersys":
		return &SupersysExpression{
			NodeBase: NodeBase{
				Span: ident.Span,
			},
		}
	}

	isDynamic := false
	lastDotIndex := int32(-1)

	//identifier member expression
	if p.i < p.len && p.s[p.i] == '.' {
		lastDotIndex = p.i
		p.i++

		var memberExpr Node = &IdentifierMemberExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{Start: ident.Span.Start},
			},
			Left:          ident,
			PropertyNames: nil,
		}

		for {
			nameStart := p.i
			isOptional := false
			isComputed := false
			var propNameNode Node

			if p.i < p.len && p.s[p.i] == '?' {
				isOptional = true
				p.i++
				nameStart = p.i
			}

			if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
				base := memberExpr.BasePtr()
				base.Span.End = p.i

				base.Err = &ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR}
				base.ValuelessTokens = append(base.ValuelessTokens, Token{Type: DOT, Span: NodeSpan{p.i - 1, p.i}})
				return memberExpr
			}

			if p.s[p.i] == '<' {
				isDynamic = true
				p.i++
				nameStart = p.i
			} else if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' && p.s[p.i] != '(' {
				return p.parseUnquotedStringLiteralAndEmailAddress(start)
				//memberExpr.NodeBase.Err = &ParsingError{UnspecifiedParsingError, makePropNameShouldStartWithAletterNot(p.s[p.i])}
				//return memberExpr
			} else if p.s[p.i] == '(' {
				isComputed = true
			} else {
				isDynamic = false
			}

			if isComputed {
				p.i++
				propNameNode = p.parseUnaryBinaryAndParenthesizedExpression(p.i-1, -1)
			} else {
				for p.i < p.len && isIdentChar(p.s[p.i]) {
					p.i++
				}

				propName := string(p.s[nameStart:p.i])
				propNameNode = &IdentifierLiteral{
					NodeBase: NodeBase{
						Span: NodeSpan{nameStart, p.i},
					},
					Name: propName,
				}
			}

			if isDynamic {
				identMemberExpr, ok := memberExpr.(*IdentifierMemberExpression)

				if ok && len32(identMemberExpr.PropertyNames) == 0 {
					memberExpr = &DynamicMemberExpression{
						NodeBase:     NodeBase{Span: NodeSpan{ident.Span.Start, p.i}},
						Left:         ident,
						PropertyName: propNameNode.(*IdentifierLiteral),
						Optional:     isOptional,
					}
				} else {
					left := memberExpr
					if ok && len(identMemberExpr.PropertyNames) == 0 {
						left = ident
					}

					memberExpr = &DynamicMemberExpression{
						NodeBase:     NodeBase{Span: NodeSpan{ident.Span.Start, p.i}},
						Left:         left,
						PropertyName: propNameNode.(*IdentifierLiteral),
						Optional:     isOptional,
					}
				}
			} else {
				identMemberExpr, ok := memberExpr.(*IdentifierMemberExpression)
				if ok && !isOptional && !isComputed {
					identMemberExpr.PropertyNames = append(identMemberExpr.PropertyNames, propNameNode.(*IdentifierLiteral))
				} else {
					if ok {
						identMemberExpr.BasePtr().Span.End = lastDotIndex
					}

					left := memberExpr
					if ok && len(identMemberExpr.PropertyNames) == 0 {
						left = ident
					}

					if !isComputed {
						memberExpr = &MemberExpression{
							NodeBase:     NodeBase{Span: NodeSpan{ident.Span.Start, p.i}},
							Left:         left,
							PropertyName: propNameNode.(*IdentifierLiteral),
							Optional:     isOptional,
						}
					} else {
						memberExpr = &ComputedMemberExpression{
							NodeBase:     NodeBase{Span: NodeSpan{ident.Span.Start, p.i}},
							Left:         left,
							PropertyName: propNameNode,
							Optional:     isOptional,
						}
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

		if p.i < p.len && (p.s[p.i] == '\\' || (isUnquotedStringChar(p.s[p.i]) && p.s[p.i] != ':' && p.s[p.i] != '<')) {
			return p.parseUnquotedStringLiteralAndEmailAddress(start)
		}
		return memberExpr
	}

	isProtocol := p.i < p.len-2 && string(p.s[p.i:p.i+3]) == "://"

	if !isProtocol && p.i < p.len && (p.s[p.i] == '\\' || isUnquotedStringChar(p.s[p.i]) && p.s[p.i] != ':') {
		return p.parseUnquotedStringLiteralAndEmailAddress(start)
	}

	switch name {
	case "true", "false":
		return &BooleanLiteral{
			NodeBase: NodeBase{
				Span: ident.Span,
			},
			Value: name[0] == 't',
		}
	case "nil":
		return &NilLiteral{
			NodeBase: NodeBase{
				Span: ident.Span,
			},
		}
	}

	if isProtocol {
		if utils.SliceContains(SCHEMES, name) {
			return p.parseURLLike(start)
		}
		base := ident.NodeBase
		base.Err = &ParsingError{UnspecifiedParsingError, fmtInvalidURIUnsupportedProtocol(name)}

		return &InvalidURL{
			NodeBase: base,
			Value:    name,
		}
	}

	return ident
}

func (p *parser) parseKeyList() *KeyListExpression {
	p.panicIfContextDone()

	start := p.i
	p.i += 2

	var (
		idents          []Node
		valuelessTokens = []Token{{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{p.i - 2, p.i}}}
		parsingErr      *ParsingError
	)
	for p.i < p.len && p.s[p.i] != '}' {
		p.eatSpaceComma(&valuelessTokens)

		if p.i >= p.len {
			//this case is handled next
			break
		}

		e, missingExpr := p.parseExpression()
		if missingExpr {
			r := p.s[p.i]
			span := NodeSpan{p.i, p.i + 1}

			p.i++
			e = &UnknownNode{
				NodeBase: NodeBase{
					span,
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInKeyList(r)},
					[]Token{{Type: UNEXPECTED_CHAR, Span: span, Raw: string(r)}},
				},
			}
			idents = append(idents, e)
			continue
		}

		idents = append(idents, e)

		if _, ok := e.(IIdentifierLiteral); !ok {
			parsingErr = &ParsingError{UnspecifiedParsingError, KEY_LIST_CAN_ONLY_CONTAIN_IDENTS}
		}

		p.eatSpaceComma(&valuelessTokens)
	}

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_KEY_LIST_MISSING_BRACE}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &KeyListExpression{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			valuelessTokens,
		},
		Keys: idents,
	}
}

func (p *parser) parsePercentAlphaStartingExpr() Node {
	p.panicIfContextDone()

	start := p.i
	p.i++

	for p.i < p.len && isIdentChar(p.s[p.i]) {
		p.i++
	}

	ident := &PatternIdentifierLiteral{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			nil,
			nil,
		},
		Name: string(p.s[start+1 : p.i]),
	}

	var left Node = ident

	if p.i < p.len && p.s[p.i] == '.' { //pattern namespace or pattern namespace member expression
		p.i++
		namespaceIdent := &PatternNamespaceIdentifierLiteral{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Name: ident.Name,
		}

		if p.i >= p.len || IsDelim(p.s[p.i]) || isSpaceNotLF(p.s[p.i]) {
			return namespaceIdent
		}

		memberNameStart := p.i

		if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
			return &PatternNamespaceMemberExpression{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, fmtPatternNamespaceMemberShouldStartWithAletterNot(p.s[p.i])},
					nil,
				},
				Namespace: namespaceIdent,
			}
		}

		for p.i < p.len && isIdentChar(p.s[p.i]) {
			p.i++
		}

		left = &PatternNamespaceMemberExpression{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				nil,
				nil,
			},
			Namespace: namespaceIdent,
			MemberName: &IdentifierLiteral{
				NodeBase: NodeBase{
					Span: NodeSpan{memberNameStart, p.i},
				},
				Name: string(p.s[memberNameStart:p.i]),
			},
		}
	}

	if p.i < p.len {

		if left == ident && ident.Name == "fn" {
			return p.parseFunctionPattern(ident.Span.Start)
		}

		switch {
		case p.s[p.i] == '(':
			if left == ident && ident.Name == "str" {
				p.i++
				return p.parseComplexStringPatternPiece(ident.Span.Start, ident)
			}
			return p.parsePatternCall(left)
		case p.s[p.i] == '?':
			p.i++
			return &OptionalPatternExpression{
				NodeBase: NodeBase{
					Span: NodeSpan{left.Base().Span.Start, p.i},
				},
				Pattern: left,
			}
		case left == ident && p.s[p.i] == ':' && (utils.SliceContains(SCHEMES, ident.Name)):
			p.i++
			return p.parseURLLikePattern(start)
		}
	}

	return left
}

func (p *parser) parsePatternUnion(start int32, isPercentPrefixed bool) *PatternUnion {
	p.panicIfContextDone()

	var (
		cases  []Node
		tokens []Token
	)

	if isPercentPrefixed {
		tokens = []Token{{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{p.i - 1, p.i + 1}}}
	} else {
		tokens = []Token{{Type: PATTERN_UNION_PIPE, Span: NodeSpan{p.i, p.i + 1}}}
	}

	p.i++
	p.eatSpace()

	case_, _ := p.parseExpression()
	cases = append(cases, case_)

	p.eatSpace()

	for p.i < p.len && (p.s[p.i] == '|' || !isUnpairedOrIsClosingDelim(p.s[p.i])) {
		p.eatSpace()
		if p.s[p.i] != '|' {
			return &PatternUnion{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, INVALID_PATT_UNION_ELEMENT_SEPARATOR_EXPLANATION},
					nil,
				},
				Cases: cases,
			}
		}
		tokens = append(tokens, Token{Type: PATTERN_UNION_PIPE, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		case_, _ := p.parseExpression()
		cases = append(cases, case_)

		p.eatSpace()
	}

	return &PatternUnion{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			nil,
			tokens,
		},
		Cases: cases,
	}
}

func (p *parser) parseComplexStringPatternUnion(start int32) *PatternUnion {
	p.panicIfContextDone()

	var cases []Node

	pieceValuelessTokens := []Token{
		{Type: OPENING_PARENTHESIS, Span: NodeSpan{start, start + 1}},
	}

	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpace()

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
					nil,
				},
				Cases: cases,
			}
		}
		pieceValuelessTokens = append(pieceValuelessTokens, Token{Type: PATTERN_UNION_PIPE, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		case_ := p.parseComplexStringPatternElement()
		cases = append(cases, case_)
	}

	var parsingErr *ParsingError

	if p.i >= p.len || p.s[p.i] != ')' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_UNION_MISSING_CLOSING_PAREN}
	} else {
		pieceValuelessTokens = append(pieceValuelessTokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &PatternUnion{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			pieceValuelessTokens,
		},
		Cases: cases,
	}
}

// parseComplexStringPatternPiece parses a piece (of string pattern) that can have one ore more elements
func (p *parser) parseComplexStringPatternPiece(start int32, ident *PatternIdentifierLiteral) *ComplexStringPatternPiece {
	p.panicIfContextDone()

	var pieceValuelessTokens []Token
	if ident != nil {
		pieceValuelessTokens = []Token{
			{Type: PERCENT_STR, Span: ident.Span},
			{Type: OPENING_PARENTHESIS, Span: NodeSpan{ident.Span.End, ident.Span.End + 1}},
		}
	} else {
		pieceValuelessTokens = []Token{
			{Type: OPENING_PARENTHESIS, Span: NodeSpan{start, start + 1}},
		}
	}
	var elemValuelessTokens []Token
	var parsingErr *ParsingError
	var elements []*PatternPieceElement

	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewline(&pieceValuelessTokens)
		if p.i >= p.len || p.s[p.i] == ')' {
			break
		}

		elementStart := p.i
		ocurrenceModifier := ExactlyOneOcurrence
		count := 0
		elementEnd := int32(-1)
		var groupName *PatternGroupName
		var elemParsingErr *ParsingError
		var element Node

		if isAlpha(p.s[p.i]) { //group name
			for p.i < p.len && (isAlpha(p.s[p.i]) || p.s[p.i] == '_') {
				p.i++
			}
			groupName = &PatternGroupName{
				NodeBase: NodeBase{
					Span: NodeSpan{elementStart, p.i},
				},
				Name: string(p.s[elementStart:p.i]),
			}

			if p.i >= p.len || p.s[p.i] != ':' {
				elemParsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_COMPLEX_STRING_PATT_ELEM_MISSING_COLON_AFTER_GROUP_NAME}
				elementEnd = p.i
				goto after_ocurrence
			}
			elemValuelessTokens = append(elemValuelessTokens, Token{Type: COLON, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}

		element = p.parseComplexStringPatternElement()
		elementEnd = p.i

		if p.i < p.len && (p.s[p.i] == '+' || p.s[p.i] == '*' || p.s[p.i] == '?' || p.s[p.i] == '=') {
			switch p.s[p.i] {
			case '+':
				ocurrenceModifier = AtLeastOneOcurrence
				elementEnd++
				p.i++
			case '*':
				ocurrenceModifier = ZeroOrMoreOcurrence
				elementEnd++
				p.i++
			case '?':
				ocurrenceModifier = OptionalOcurrence
				elementEnd++
				p.i++
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
				ocurrenceModifier = ExactOcurrence
				elementEnd = p.i
			}
		}

	after_ocurrence:

		elements = append(elements, &PatternPieceElement{
			NodeBase: NodeBase{
				NodeSpan{elementStart, elementEnd},
				elemParsingErr,
				elemValuelessTokens,
			},
			Ocurrence:           ocurrenceModifier,
			ExactOcurrenceCount: int(count),
			Expr:                element,
			GroupName:           groupName,
		})

	}
	if p.i >= p.len || p.s[p.i] != ')' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_COMPLEX_STRING_PATT_MISSING_CLOSING_BRACKET}
	} else {
		pieceValuelessTokens = append(pieceValuelessTokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &ComplexStringPatternPiece{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			pieceValuelessTokens,
		},
		Elements: elements,
	}
}

func (p *parser) parsePatternCall(callee Node) *PatternCallExpression {
	p.panicIfContextDone()

	valuelessTokens := []Token{{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}}}
	p.i++
	p.eatSpaceComma(&valuelessTokens)

	var args []Node

	for p.i < p.len && p.s[p.i] != ')' {
		arg, isMissingExpr := p.parseExpression()

		if isMissingExpr {
			span := NodeSpan{p.i, p.i + 1}
			arg = &UnknownNode{
				NodeBase: NodeBase{
					span,
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInPatternCallArguments(p.s[p.i])},
					[]Token{{Type: UNEXPECTED_CHAR, Span: span, Raw: string(p.s[p.i])}},
				},
			}
			p.i++
		}

		args = append(args, arg)
		p.eatSpaceComma(&valuelessTokens)
	}

	var parsingErr *ParsingError

	if p.i >= p.len || p.s[p.i] != ')' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PATTERN_CALL_MISSING_CLOSING_PAREN}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &PatternCallExpression{
		Callee: callee,
		NodeBase: NodeBase{
			Span:            NodeSpan{callee.Base().Span.Start, p.i},
			ValuelessTokens: valuelessTokens,
			Err:             parsingErr,
		},
		Arguments: args,
	}
}

func (p *parser) parseObjectPatternLiteral(percentPrefixed bool) *ObjectPatternLiteral {
	p.panicIfContextDone()

	var (
		unamedPropCount    = 0
		properties         []*ObjectPatternProperty
		spreadElements     []*PatternPropertySpreadElement
		parsingErr         *ParsingError
		tokens             []Token
		exact              = false
		objectPatternStart int32
	)

	if percentPrefixed {
		tokens = []Token{{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{p.i - 1, p.i + 1}}}
		objectPatternStart = p.i - 1
	} else {
		tokens = []Token{{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}}}
		objectPatternStart = p.i
	}

	p.i++

	//entry
	var (
		key            Node
		keyName        string
		keyOrVal       Node
		isOptional     bool
		implicitKey    bool
		type_          Node
		v              Node
		isMissingExpr  bool
		propSpanStart  int32
		propSpanEnd    int32
		propParsingErr *ParsingError
		entryTokens    []Token
	)

object_pattern_top_loop:
	for p.i < p.len && p.s[p.i] != '}' { //one iteration == one entry or spread element (that can be invalid)
		p.eatSpaceNewlineCommaComment(&tokens)

		propParsingErr = nil
		key = nil
		isMissingExpr = false
		propSpanStart = 0
		propSpanEnd = 0
		keyName = ""
		v = nil
		entryTokens = nil
		propParsingErr = nil
		implicitKey = false

		if p.i < p.len && p.s[p.i] == '}' {
			break object_pattern_top_loop
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' { //spread element
			spreadStart := p.i
			dotStart := p.i

			p.i += 3

			p.eatSpace()

			// //inexact pattern
			// if p.i < p.len && (p.s[p.i] == '}' || p.s[p.i] == ',' || p.s[p.i] == '\n') {
			// 	tokens = append(tokens, Token{Type: THREE_DOTS, Span: NodeSpan{dotStart, dotStart + 3}})

			// 	exact = false

			// 	p.eatSpaceNewlineCommaComment(&tokens)
			// 	continue object_pattern_top_loop
			// }
			// p.eatSpace()

			expr, _ := p.parseExpression()

			spreadElements = append(spreadElements, &PatternPropertySpreadElement{
				NodeBase: NodeBase{
					NodeSpan{spreadStart, expr.Base().Span.End},
					nil,
					[]Token{
						{Type: THREE_DOTS, Span: NodeSpan{dotStart, dotStart + 3}},
					},
				},
				Expr: expr,
			})

		} else {
			prev := p.inPattern
			p.inPattern = false

			key, isMissingExpr = p.parseExpression()
			keyOrVal = key

			p.inPattern = prev

			//if missing expression we report an error and we continue the main loop
			if isMissingExpr {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectPattern(p.s[p.i])}
				entryTokens = append(entryTokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})

				p.i++
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, p.i - 1},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Key:   nil,
					Value: nil,
				})
				continue object_pattern_top_loop
			}

			if boolConvExpr, ok := key.(*BooleanConversionExpression); ok {
				key = boolConvExpr.Expr
				keyOrVal = key
				isOptional = true
				entryTokens = append(entryTokens, Token{Type: QUESTION_MARK, Span: NodeSpan{p.i - 1, p.i}})
			}

			propSpanStart = key.Base().Span.Start

			if len32(key.Base().ValuelessTokens) > 0 && key.Base().ValuelessTokens[0].Type == OPENING_PARENTHESIS {
				implicitKey = true
				keyName = strconv.Itoa(unamedPropCount)
				v = key
				propSpanEnd = v.Base().Span.End
				key = nil
			} else {
				switch k := key.(type) {
				case *IdentifierLiteral:
					keyName = k.Name
				case *QuotedStringLiteral:
					keyName = k.Value
				default:
					implicitKey = true
					propParsingErr = &ParsingError{UnspecifiedParsingError, IMPLICIT_KEY_PROPS_ARE_NOT_ALLOWED_IN_OBJECT_PATTERNS}
					keyName = strconv.Itoa(unamedPropCount)
					v = key
					propSpanEnd = v.Base().Span.End
					key = nil
				}
			}

			p.eatSpace()
			if p.i >= p.len {
				break object_pattern_top_loop
			}

			switch {
			case isValidEntryEnd(p.s, p.i):
				implicitKey = true
				if propParsingErr == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, IMPLICIT_KEY_PROPS_ARE_NOT_ALLOWED_IN_OBJECT_PATTERNS}
				}
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, p.i},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Value: keyOrVal,
				})
				goto step_end
			case p.s[p.i] == ':':
				goto at_colon
			case p.s[p.i] == '%': // type annotation
				switch {
				case implicitKey: // implicit key properties cannot be annotated
					if propParsingErr == nil {
						propParsingErr = &ParsingError{UnspecifiedParsingError, ONLY_EXPLICIT_KEY_CAN_HAVE_A_TYPE_ANNOT}
					}
					implicitKey = true
					type_ = p.parsePercentPrefixedPattern()
					propSpanEnd = type_.Base().Span.End

					properties = append(properties, &ObjectPatternProperty{
						NodeBase: NodeBase{
							Span:            NodeSpan{propSpanStart, propSpanEnd},
							Err:             propParsingErr,
							ValuelessTokens: entryTokens,
						},
						Value: keyOrVal,
						Type:  type_,
					})

					goto step_end
				default: //explicit key property
				}

				type_ = p.parsePercentPrefixedPattern()
				propSpanEnd = type_.Base().Span.End

				p.eatSpace()
				if p.i >= p.len {
					break object_pattern_top_loop
				}

				goto explicit_key
			default:

			}

			// if meta property we add an error
			if IsMetadataKey(keyName) && propParsingErr == nil {
				propParsingErr = &ParsingError{UnspecifiedParsingError, METAPROPS_ARE_NOT_ALLOWED_IN_OBJECT_PATTERNS}
			}

			if implicitKey { // implicit key property not followed by a valid entry end
				if propParsingErr == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION}
				}
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, p.i},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Value: keyOrVal,
				})
				goto step_end
			}

		explicit_key:
			if p.s[p.i] != ':' { //we add the property and we keep the current character for the next iteration step
				if type_ == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjPatternKeyMissingColonAfterKey(keyName)}
				} else {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjKeyMissingColonAfterTypeAnnotation(keyName)}
				}
				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, p.i},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Key:  key,
					Type: type_,
				})
				goto step_end
			}

		at_colon:
			{
				if implicitKey {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtOnlyIdentsAndStringsValidObjPatternKeysNot(key)}
					implicitKey = false
				}

				entryTokens = append(entryTokens, Token{Type: COLON, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				p.eatSpace()

				if p.i < p.len-1 && p.s[p.i] == '#' && IsCommentFirstSpace(p.s[p.i+1]) {
					p.eatSpaceNewlineComment(&entryTokens)
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjPatternKeyCommentBeforeValueOfKey(keyName)}
				}

				p.eatSpace()

				if p.i >= p.len || p.s[p.i] == '}' {
					break object_pattern_top_loop
				}

				if p.s[p.i] == '\n' {
					propParsingErr = &ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON}
					p.eatSpaceNewline(&entryTokens)
				}

				v, isMissingExpr = p.parseExpression()
				propSpanEnd = p.i

				if isMissingExpr {
					if p.i < p.len {
						propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectPattern(p.s[p.i])}
						entryTokens = append(entryTokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
						p.i++
					} else {
						v = nil
					}
				}

				p.eatSpace()

				if !isMissingExpr && p.i < p.len && !isValidEntryEnd(p.s, p.i) {
					propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION}
				}

				properties = append(properties, &ObjectPatternProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, propSpanEnd},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Key:      key,
					Type:     type_,
					Value:    v,
					Optional: isOptional,
				})
			}

		step_end:
			keyName = ""
			key = nil
			keyOrVal = nil
			isOptional = false
			v = nil
			implicitKey = false
			type_ = nil
			p.eatSpaceNewlineCommaComment(&tokens)
		}
	}

	if !implicitKey && keyName != "" || (keyName == "" && key != nil) {

		properties = append(properties, &ObjectPatternProperty{
			NodeBase: NodeBase{
				Span:            NodeSpan{propSpanStart, propSpanEnd},
				Err:             propParsingErr,
				ValuelessTokens: entryTokens,
			},
			Key:   key,
			Value: v,
		})
	}

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE}
	} else {
		tokens = append(tokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	base := NodeBase{
		Span:            NodeSpan{objectPatternStart, p.i},
		Err:             parsingErr,
		ValuelessTokens: tokens,
	}

	return &ObjectPatternLiteral{
		NodeBase:       base,
		Properties:     properties,
		SpreadElements: spreadElements,
		Exact:          exact,
	}
}

func (p *parser) parseListPatternLiteral(percentPrefixed bool) Node {
	p.panicIfContextDone()

	openingBracketIndex := p.i
	p.i++

	var (
		elements        []Node
		valuelessTokens []Token
		start           int32
	)

	if percentPrefixed {
		valuelessTokens = []Token{{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{openingBracketIndex - 1, openingBracketIndex + 1}}}
		start = openingBracketIndex - 1
	} else {
		valuelessTokens = []Token{{Type: OPENING_BRACKET, Span: NodeSpan{openingBracketIndex, openingBracketIndex + 1}}}
		start = openingBracketIndex
	}

	for p.i < p.len && p.s[p.i] != ']' {
		p.eatSpaceNewlineCommaComment(&valuelessTokens)

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

		p.eatSpaceNewlineCommaComment(&valuelessTokens)
	}

	var parsingErr *ParsingError

	if p.i >= p.len || p.s[p.i] != ']' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_PATT_LIT_MISSING_BRACE}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	var generalElement Node
	if p.i < p.len && (p.s[p.i] == '%' || isFirstIdentChar(p.s[p.i]) || isOpeningDelim(p.s[p.i])) {
		if len32(elements) > 0 {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_LIST_PATT_GENERAL_ELEMENT_IF_ELEMENTS}
		} else {
			elements = nil
		}
		generalElement, _ = p.parseExpression()
	}

	return &ListPatternLiteral{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			Err:             parsingErr,
			ValuelessTokens: valuelessTokens,
		},
		Elements:       elements,
		GeneralElement: generalElement,
	}
}

func (p *parser) parseObjectOrRecordLiteral(isRecord bool) Node {
	p.panicIfContextDone()

	var (
		unamedPropCount = 0
		properties      []*ObjectProperty
		metaProperties  []*ObjectMetaProperty
		spreadElements  []*PropertySpreadElement
		parsingErr      *ParsingError
		tokens          []Token
	)

	openingBraceIndex := p.i

	if isRecord {
		tokens = []Token{{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{p.i, p.i + 2}}}
		p.i += 2
	} else {
		tokens = []Token{{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}}}
		p.i++
	}

	//entry
	var (
		key            Node
		keyName        string
		keyOrVal       Node
		implicitKey    bool
		type_          Node
		v              Node
		isMissingExpr  bool
		propSpanStart  int32
		propSpanEnd    int32
		propParsingErr *ParsingError
		entryTokens    []Token
	)

object_literal_top_loop:
	for p.i < p.len && p.s[p.i] != '}' { //one iteration == one entry or spread element (that can be invalid)
		p.eatSpaceNewlineCommaComment(&tokens)

		propParsingErr = nil
		key = nil
		keyOrVal = nil
		isMissingExpr = false
		propSpanStart = 0
		propSpanEnd = 0
		keyName = ""
		type_ = nil
		v = nil
		entryTokens = nil
		propParsingErr = nil
		implicitKey = false

		if p.i >= p.len || p.s[p.i] == '}' {
			break object_literal_top_loop
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' { //spread element
			spreadStart := p.i
			entryTokens = []Token{{Type: THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}}}

			p.i += 3
			p.eatSpace()

			expr, _ := p.parseExpression()

			_, ok := expr.(*ExtractionExpression)
			if !ok {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidSpreadElemExprShouldBeExtrExprNot(expr)}
			}

			p.eatSpace()

			if p.i < p.len && !isValidEntryEnd(p.s, p.i) {
				propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION}
			}

			spreadElements = append(spreadElements, &PropertySpreadElement{
				NodeBase: NodeBase{
					NodeSpan{spreadStart, expr.Base().Span.End},
					propParsingErr,
					entryTokens,
				},
				Expr: expr,
			})

			goto step_end
		}

		key, isMissingExpr = p.parseExpression()
		keyOrVal = key

		//if missing expression we report an error and we continue the main loop
		if isMissingExpr {
			propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectRecord(p.s[p.i])}
			entryTokens = append(entryTokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})

			p.i++
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span:            NodeSpan{propSpanStart, p.i - 1},
					Err:             propParsingErr,
					ValuelessTokens: entryTokens,
				},
				Key:   nil,
				Value: nil,
			})
			continue object_literal_top_loop
		}

		propSpanStart = key.Base().Span.Start

		if len32(key.Base().ValuelessTokens) > 0 && key.Base().ValuelessTokens[0].Type == OPENING_PARENTHESIS {
			implicitKey = true
			keyName = strconv.Itoa(unamedPropCount)
			v = key
			propSpanEnd = v.Base().Span.End
			key = nil
		} else {
			switch k := key.(type) {
			case *IdentifierLiteral:
				keyName = k.Name
			case *QuotedStringLiteral:
				keyName = k.Value
			default:
				implicitKey = true
				keyName = strconv.Itoa(unamedPropCount)
				v = key
				propSpanEnd = v.Base().Span.End
				key = nil
			}
		}

		p.eatSpace()
		if p.i >= p.len {
			break object_literal_top_loop
		}

		switch {
		case isValidEntryEnd(p.s, p.i):
			implicitKey = true
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span:            NodeSpan{propSpanStart, p.i},
					Err:             propParsingErr,
					ValuelessTokens: entryTokens,
				},
				Value: keyOrVal,
			})
			goto step_end
		case p.s[p.i] == ':':
			goto at_colon
		case p.s[p.i] == '%': // type annotation
			switch {
			case implicitKey: // implicit key properties cannot be annotated
				if propParsingErr == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, ONLY_EXPLICIT_KEY_CAN_HAVE_A_TYPE_ANNOT}
				}
				implicitKey = true
				type_ = p.parsePercentPrefixedPattern()
				propSpanEnd = type_.Base().Span.End

				properties = append(properties, &ObjectProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, propSpanEnd},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Value: keyOrVal,
					Type:  type_,
				})

				goto step_end
			case isRecord: //explicit key properties of record cannot be annotated
				properties = append(properties, &ObjectProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, p.i},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Key: keyOrVal,
				})
				goto step_end //the pattern is kept for the next iteration step
			case IsMetadataKey(keyName): //meta properties cannot be annotated
				if propParsingErr == nil {
					propParsingErr = &ParsingError{UnspecifiedParsingError, ONLY_EXPLICIT_KEY_CAN_HAVE_A_TYPE_ANNOT}
				}
				metaProperties = append(metaProperties, &ObjectMetaProperty{
					NodeBase: NodeBase{
						Span:            NodeSpan{propSpanStart, p.i},
						Err:             propParsingErr,
						ValuelessTokens: entryTokens,
					},
					Key: keyOrVal,
				})
				goto step_end //the pattern is kept for the next iteration step
			default: //explicit key property
			}

			type_ = p.parsePercentPrefixedPattern()
			propSpanEnd = type_.Base().Span.End

			p.eatSpace()
			if p.i >= p.len {
				break object_literal_top_loop
			}

			goto explicit_key
		default:

		}

		// if meta property we parse it and continue to next iteration step
		if !implicitKey && IsMetadataKey(keyName) && !isRecord && p.i < p.len && p.s[p.i] != ':' {
			block := p.parseBlock()
			propSpanEnd = block.Span.End

			metaProperties = append(metaProperties, &ObjectMetaProperty{
				NodeBase: NodeBase{
					Span:            NodeSpan{propSpanStart, propSpanEnd},
					Err:             propParsingErr,
					ValuelessTokens: entryTokens,
				},
				Key: key,
				Initialization: &InitializationBlock{
					NodeBase:   block.NodeBase,
					Statements: block.Statements,
				},
			})

			p.eatSpace()

			goto step_end
		}

		if implicitKey { // implicit key property not followed by a valid entry end
			if propParsingErr == nil {
				propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION}
			}
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span:            NodeSpan{propSpanStart, p.i},
					Err:             propParsingErr,
					ValuelessTokens: entryTokens,
				},
				Value: keyOrVal,
			})
			goto step_end
		}

	explicit_key:

		if p.s[p.i] != ':' { //we add the property and we keep the current character for the next iteration step
			if type_ == nil {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyMissingColonAfterKey(keyName)}
			} else {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjKeyMissingColonAfterTypeAnnotation(keyName)}
			}
			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span:            NodeSpan{propSpanStart, p.i},
					Err:             propParsingErr,
					ValuelessTokens: entryTokens,
				},
				Key:  key,
				Type: type_,
			})
			goto step_end
		}

	at_colon:
		{
			if implicitKey {
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtOnlyIdentsAndStringsValidObjRecordKeysNot(key)}
				implicitKey = false
			}

			entryTokens = append(entryTokens, Token{Type: COLON, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
			p.eatSpace()

			if p.i < p.len-1 && p.s[p.i] == '#' && IsCommentFirstSpace(p.s[p.i+1]) {
				p.eatSpaceNewlineComment(&entryTokens)
				propParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyCommentBeforeValueOfKey(keyName)}
			}

			p.eatSpace()

			if p.i >= p.len || p.s[p.i] == '}' {
				break object_literal_top_loop
			}

			if p.s[p.i] == '\n' {
				propParsingErr = &ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON}
				p.eatSpaceNewline(&entryTokens)
			}

			v, isMissingExpr = p.parseExpression()
			propSpanEnd = p.i

			if isMissingExpr {
				if p.i < p.len {
					propParsingErr = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectRecord(p.s[p.i])}
					entryTokens = append(entryTokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
					p.i++
				} else {
					v = nil
				}
			}

			p.eatSpace()

			if !isMissingExpr && p.i < p.len && !isValidEntryEnd(p.s, p.i) {
				propParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION}
			}

			properties = append(properties, &ObjectProperty{
				NodeBase: NodeBase{
					Span:            NodeSpan{propSpanStart, propSpanEnd},
					Err:             propParsingErr,
					ValuelessTokens: entryTokens,
				},
				Key:   key,
				Type:  type_,
				Value: v,
			})
		}

	step_end:
		keyName = ""
		key = nil
		keyOrVal = nil
		v = nil
		implicitKey = false
		type_ = nil
		p.eatSpaceNewlineCommaComment(&tokens)
	}

	if !implicitKey && keyName != "" || v != nil {
		properties = append(properties, &ObjectProperty{
			NodeBase: NodeBase{
				Span:            NodeSpan{propSpanStart, propSpanEnd},
				Err:             propParsingErr,
				ValuelessTokens: entryTokens,
			},
			Key:   key,
			Type:  type_,
			Value: v,
		})
	}

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE}
	} else {
		tokens = append(tokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	base := NodeBase{
		Span:            NodeSpan{openingBraceIndex, p.i},
		Err:             parsingErr,
		ValuelessTokens: tokens,
	}

	if isRecord {
		return &RecordLiteral{
			NodeBase:       base,
			Properties:     properties,
			SpreadElements: spreadElements,
		}
	}

	return &ObjectLiteral{
		NodeBase:       base,
		Properties:     properties,
		MetaProperties: metaProperties,
		SpreadElements: spreadElements,
	}
}

func (p *parser) parseListOrTupleLiteral(isTuple bool) Node {
	p.panicIfContextDone()

	var (
		openingBracketIndex = p.i
		valuelessTokens     []Token
		elements            []Node
		type_               Node
		parsingErr          *ParsingError
	)

	if isTuple {
		valuelessTokens = []Token{{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{p.i, p.i + 2}}}
		p.i += 2
	} else {
		valuelessTokens = []Token{{Type: OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}}}
		p.i++
	}

	//parse type annotation if present
	if p.i < p.len-1 && p.s[p.i] == ']' && p.s[p.i+1] == '%' {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		type_ = p.parsePercentPrefixedPattern()
		if p.i >= p.len || p.s[p.i] != '[' {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_OPENING_BRACKET_AFTER_TYPE}
		} else {
			valuelessTokens = append(valuelessTokens, Token{Type: OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	}

	if parsingErr == nil {

		//parse elements
		for p.i < p.len && p.s[p.i] != ']' {
			p.eatSpaceNewlineCommaComment(&valuelessTokens)

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
				e = &ElementSpreadElement{
					NodeBase: NodeBase{
						NodeSpan{spreadStart, e.Base().Span.End},
						nil,
						[]Token{{Type: THREE_DOTS, Span: NodeSpan{spreadStart, spreadStart + 3}}},
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
			p.eatSpaceNewlineCommaComment(&valuelessTokens)
		}

		if p.i >= p.len || p.s[p.i] != ']' {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_CLOSING_BRACKET}
		} else {
			valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
	}

	if isTuple {
		return &TupleLiteral{
			NodeBase: NodeBase{
				Span:            NodeSpan{openingBracketIndex, p.i},
				Err:             parsingErr,
				ValuelessTokens: valuelessTokens,
			},
			TypeAnnotation: type_,
			Elements:       elements,
		}
	}

	return &ListLiteral{
		NodeBase: NodeBase{
			Span:            NodeSpan{openingBracketIndex, p.i},
			Err:             parsingErr,
			ValuelessTokens: valuelessTokens,
		},
		TypeAnnotation: type_,
		Elements:       elements,
	}
}

func (p *parser) parseDictionaryLiteral() *DictionaryLiteral {
	p.panicIfContextDone()

	openingIndex := p.i
	p.i += 2

	var parsingErr *ParsingError
	var entries []*DictionaryEntry
	var tokens = []Token{{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{p.i - 2, p.i}}}

dictionary_literal_top_loop:
	for p.i < p.len && p.s[p.i] != '}' { //one iteration == one entry (that can be invalid)
		p.eatSpaceNewlineCommaComment(&tokens)

		if p.i < p.len && p.s[p.i] == '}' {
			break dictionary_literal_top_loop
		}

		entry := &DictionaryEntry{
			NodeBase: NodeBase{
				NodeSpan{p.i, p.i + 1},
				nil,
				nil,
			},
		}

		key, isMissingExpr := p.parseExpression()
		entry.Key = key

		if isMissingExpr {
			p.i++
			entry.Span.End = key.Base().Span.End
			entries = append(entries, entry)
			p.eatSpaceNewlineCommaComment(&tokens)
			continue
		}

		if key.Base().Err == nil && !NodeIsSimpleValueLiteral(key) && !NodeIs(key, &IdentifierLiteral{}) {
			key.BasePtr().Err = &ParsingError{UnspecifiedParsingError, INVALID_DICT_KEY_ONLY_SIMPLE_VALUE_LITS}
		}

		p.eatSpace()

		if p.i >= p.len {
			break
		}

		if p.s[p.i] != ':' {
			if p.s[p.i] != ',' && p.s[p.i] != '}' {
				entry.Span.End = p.i
				entry.Err = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionary(p.s[p.i])}
				entries = append(entries, entry)
				p.i++
				p.eatSpaceNewlineCommaComment(&tokens)
				continue
			}
		} else {
			p.i++
		}

		if p.i >= p.len || p.s[p.i] == '}' {
			entry.Span.End = p.i
			entries = append(entries, entry)
			break
		}

		p.eatSpace()

		value, isMissingExpr := p.parseExpression()
		entry.Value = value
		entry.Span.End = value.Base().Span.End
		entries = append(entries, entry)

		for isMissingExpr && p.i < p.len && p.s[p.i] != '}' && p.s[p.i] != ',' {
			if entry.Err == nil {
				entry.Err = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDictionary(p.s[p.i])}
			}
			p.i++
		}

		p.eatSpace()

		if p.i < p.len && !isValidEntryEnd(p.s, p.i) && entry.Err == nil {
			entry.Err = &ParsingError{UnspecifiedParsingError, INVALID_DICT_LIT_ENTRY_SEPARATION}
		}

		p.eatSpaceNewlineCommaComment(&tokens)
	}

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_DICT_MISSING_CLOSING_BRACE}
	} else {
		tokens = append(tokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &DictionaryLiteral{
		NodeBase: NodeBase{
			Span:            NodeSpan{openingIndex, p.i},
			Err:             parsingErr,
			ValuelessTokens: tokens,
		},
		Entries: entries,
	}
}

func (p *parser) parseRuneRuneRange() Node {
	p.panicIfContextDone()

	start := p.i

	parseRuneLiteral := func() *RuneLiteral {
		start := p.i
		p.i++

		if p.i >= p.len {
			return &RuneLiteral{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_RUNE_LIT},
					nil,
				},
				Value: 0,
			}
		}

		value := p.s[p.i]

		if value == '\'' {
			return &RuneLiteral{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, INVALID_RUNE_LIT_NO_CHAR},
					nil,
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
				return &RuneLiteral{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						&ParsingError{UnspecifiedParsingError, INVALID_RUNE_LIT_INVALID_SINGLE_CHAR_ESCAPE},
						nil,
					},
					Value: 0,
				}
			}
		}

		p.i++

		var parsingErr *ParsingError
		if p.i >= p.len || p.s[p.i] != '\'' {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_RUNE_LIT_MISSING_QUOTE}
		} else {
			p.i++
		}

		return &RuneLiteral{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				nil,
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

		return &RuneRangeExpression{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, INVALID_RUNE_RANGE_EXPR},
				[]Token{{Type: DOT, Span: NodeSpan{p.i - 1, p.i}}},
			},
			Lower: lower,
			Upper: nil,
		}
	}
	p.i++
	tokens := []Token{{Type: TWO_DOTS, Span: NodeSpan{p.i - 2, p.i}}}

	if p.i >= p.len || p.s[p.i] != '\'' {
		return &RuneRangeExpression{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, INVALID_RUNE_RANGE_EXPR},
				tokens,
			},
			Lower: lower,
			Upper: nil,
		}
	}

	upper := parseRuneLiteral()

	return &RuneRangeExpression{
		NodeBase: NodeBase{
			NodeSpan{start, upper.Base().Span.End},
			nil,
			tokens,
		},
		Lower: lower,
		Upper: upper,
	}
}

func (p *parser) parsePercentPrefixedPattern() Node {
	p.panicIfContextDone()

	start := p.i
	p.i++

	percentSymbol := Token{Type: PERCENT_SYMBOL, Span: NodeSpan{start, p.i}}

	if p.i >= p.len {
		return &UnknownNode{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
				[]Token{percentSymbol},
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

		union := p.parsePatternUnion(start, true)
		p.eatSpace()

		return union
	case '.', '/':
		p.i--
		return p.parsePathLikeExpression(true)
	case ':':
		p.i++
		return p.parseURLLikePattern(start)
	case '{':
		prev := p.inPattern
		defer func() {
			p.inPattern = prev
		}()
		p.inPattern = true

		return p.parseObjectPatternLiteral(true)
	case '[':
		prev := p.inPattern
		defer func() {
			p.inPattern = prev
		}()
		p.inPattern = true

		return p.parseListPatternLiteral(true)
	case '(': //pattern conversion expresison
		prev := p.inPattern
		p.inPattern = false
		e, _ := p.parseExpression()

		p.inPattern = prev
		return &PatternConversionExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, e.Base().Span.End},
				ValuelessTokens: []Token{percentSymbol},
			},
			Value: e,
		}
	case '`':
		p.i++
		for p.i < p.len && (p.s[p.i] != '`' || countPrevBackslashes(p.s, p.i)%2 == 1) {
			p.i++
		}

		raw := ""
		str := ""

		var parsingErr *ParsingError
		if p.i >= p.len {
			raw = string(p.s[start:p.i])
			str = raw[2:]

			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_REGEX_LIT}
		} else {
			raw = string(p.s[start : p.i+1])
			str = raw[2 : len(raw)-1]
			p.i++

			_, err := regexp.Compile(str)
			if err != nil && parsingErr == nil {
				parsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidRegexLiteral(err.Error())}
			}
		}

		return &RegularExpressionLiteral{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				nil,
			},
			Value: str,
			Raw:   raw,
		}
	case '-':
		prev := p.inPattern
		defer func() {
			p.inPattern = prev
		}()
		p.inPattern = true

		p.i++
		if p.i >= p.len {
			return &OptionPatternLiteral{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, DASH_SHOULD_BE_FOLLOWED_BY_OPTION_NAME},
				},
				SingleDash: true,
			}
		}

		singleDash := true

		if p.s[p.i] == '-' {
			singleDash = false
			p.i++
		}

		nameStart := p.i

		if p.i >= p.len {
			return &OptionPatternLiteral{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, DOUBLE_DASH_SHOULD_BE_FOLLOWED_BY_OPTION_NAME},
				},
				SingleDash: singleDash,
			}
		}

		if !isAlpha(p.s[p.i]) && !isDecDigit(p.s[p.i]) {
			return &OptionPatternLiteral{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, OPTION_NAME_CAN_ONLY_CONTAIN_ALPHANUM_CHARS},
				},
				SingleDash: singleDash,
			}
		}

		for p.i < p.len && (isAlpha(p.s[p.i]) || isDecDigit(p.s[p.i]) || p.s[p.i] == '-') {
			p.i++
		}

		name := string(p.s[nameStart:p.i])

		if p.i >= p.len || p.s[p.i] != '=' {

			return &OptionPatternLiteral{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, UNTERMINATED_OPION_PATTERN_A_VALUE_IS_EXPECTED},
				},
				Name:       name,
				SingleDash: singleDash,
			}
		}

		p.i++

		if p.i >= p.len {
			return &OptionPatternLiteral{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, UNTERMINATED_OPION_PATT_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR},
				},
				Name:       name,
				SingleDash: singleDash,
			}
		}

		value, _ := p.parseExpression()

		return &OptionPatternLiteral{
			NodeBase:   NodeBase{Span: NodeSpan{start, p.i}},
			Name:       name,
			Value:      value,
			SingleDash: singleDash,
		}
	default:
		if isAlpha(p.s[p.i]) {
			p.i--
			return p.parsePercentAlphaStartingExpr()
		}

		//TODO: fix, error based on next char ?

		return &UnknownNode{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
				[]Token{percentSymbol},
			},
		}
	}
}

func (p *parser) getValueOfMultilineStringSliceOrLiteral(raw []byte, literal bool) (string, *ParsingError) {
	p.panicIfContextDone()

	var value string

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
	err := json.Unmarshal(marshalingInput, &value)

	if err != nil {
		return "", &ParsingError{UnspecifiedParsingError, fmtInvalidStringLitJSON(err.Error())}
	}
	return value, nil
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
	valuelessTokens := []Token{{Type: BACKQUOTE, Span: NodeSpan{p.i - 1, p.i}}}
	slices := make([]Node, 0)
	sliceStart := p.i

	var parsingErr *ParsingError
	isMultilineStringLiteral := false

	for p.i < p.len && (p.s[p.i] != '`' || countPrevBackslashes(p.s, p.i)%2 == 1) {

		//interpolation
		if p.s[p.i] == '{' && p.s[p.i-1] == '{' {
			valuelessTokens = append(valuelessTokens, Token{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{p.i - 1, p.i + 1}})

			// add previous slice
			raw := string(p.s[sliceStart : p.i-1])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			slices = append(slices, &StringTemplateSlice{
				NodeBase: NodeBase{
					NodeSpan{sliceStart, p.i - 1},
					sliceErr,
					nil,
				},
				Raw:   raw,
				Value: value,
			})

			inInterpolation = true
			p.i++
			interpolationStart = p.i
		} else if inInterpolation && p.s[p.i] == '}' && p.s[p.i-1] == '}' { //end of interpolation
			valuelessTokens = append(valuelessTokens, Token{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{p.i - 1, p.i + 1}})
			interpolationExclEnd := p.i - 1
			inInterpolation = false
			p.i++
			sliceStart = p.i

			var interpParsingErr *ParsingError
			var typ string //typename or typename.method followed by ':'
			var expr Node

			interpolation := p.s[interpolationStart:interpolationExclEnd]

			for j := int32(0); j < len32(interpolation); j++ {
				if !isInterpolationAllowedChar(interpolation[j]) {
					interpParsingErr = &ParsingError{UnspecifiedParsingError, STR_INTERP_LIMITED_CHARSET}
					break
				}
			}

			if interpParsingErr == nil {
				if strings.TrimSpace(string(interpolation)) == "" {
					interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_STRING_INTERPOLATION_SHOULD_NOT_BE_EMPTY}
				} else if pattern != nil && !isIdentChar(interpolation[0]) {
					interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_STRING_INTERPOLATION_SHOULD_START_WITH_A_NAME}
				} else {

					if pattern != nil { //typed interpolation
						i := int32(1)
						for ; i < len32(interpolation) && (interpolation[i] == '.' || isIdentChar(interpolation[i])); i++ {
						}

						typ = string(interpolation[:i+1])

						if i >= len32(interpolation) || interpolation[i] != ':' || i >= len32(interpolation)-1 {
							interpParsingErr = &ParsingError{UnspecifiedParsingError, NAME_IN_STR_INTERP_SHOULD_BE_FOLLOWED_BY_COLON_AND_EXPR}
						} else {
							i++

							exprStart := i + interpolationStart

							e, ok := ParseExpression(string(interpolation[i:]))
							if !ok {
								interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_STR_INTERP}
							} else {
								expr = e
								shiftNodeSpans(expr, exprStart)
							}
						}
					} else { //untyped interpolation
						e, ok := ParseExpression(string(interpolation))
						if !ok {
							interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_STR_INTERP}
						} else {
							expr = e
							shiftNodeSpans(expr, interpolationStart)
						}
					}
				}
			}

			typeWithoutColon := ""
			var interpTokens []Token
			if pattern != nil && len(typ) > 0 {
				typeWithoutColon = typ[:len(typ)-1]
				interpTokens = []Token{{
					Type: STR_TEMPLATE_INTERP_TYPE,
					Span: NodeSpan{interpolationStart,
						interpolationStart + int32(len(typ)),
					},
					Raw: typ,
				}}
			}

			interpolationNode := &StringTemplateInterpolation{
				NodeBase: NodeBase{
					NodeSpan{interpolationStart, interpolationExclEnd},
					interpParsingErr,
					interpTokens,
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
				nil,
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
				nil,
			},
			Raw:   raw,
			Value: value,
		})
	}

end:
	if isMultilineStringLiteral {
		var value string
		var raw string

		if p.i >= p.len && p.s[p.i-1] != '`' {
			raw = string(p.s[openingBackquoteIndex:])
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_MULTILINE_STRING_LIT}
		} else {
			p.i++

			raw = string(p.s[openingBackquoteIndex:p.i])
			value, parsingErr = p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), true)
		}

		return &MultilineStringLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{openingBackquoteIndex, p.i},
				Err:  parsingErr,
			},
			Raw:   raw,
			Value: value,
		}
	}

	if p.i >= p.len {
		if !inInterpolation {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_TEMPL_LIT}
		}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: BACKQUOTE, Span: NodeSpan{p.i, p.i + 1}})
		p.i++ // eat `
	}

	return &StringTemplateLiteral{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			valuelessTokens,
		},
		Pattern: pattern,
		Slices:  slices,
	}
}

func (p *parser) parseIfExpression(openingParenIndex int32, ifIdent *IdentifierLiteral) *IfExpression {
	p.panicIfContextDone()

	var alternate Node
	var end int32
	var parsingErr *ParsingError

	tokens := []Token{
		{Type: IF_KEYWORD, Span: ifIdent.Span},
	}

	p.eatSpace()
	test, _ := p.parseExpression()
	p.eatSpace()

	consequent, isMissingExpr := p.parseExpression()
	end = consequent.Base().Span.End
	p.eatSpace()

	if isMissingExpr {
		if p.i < p.len && p.s[p.i] == ')' {
			//missing expression
			p.i++
		}
	}

	if p.i < p.len-3 && p.s[p.i] == 'e' && p.s[p.i+1] == 'l' && p.s[p.i+2] == 's' && p.s[p.i+3] == 'e' {
		tokens = append(tokens, Token{
			Type: ELSE_KEYWORD,
			Span: NodeSpan{p.i, p.i + 4},
		})
		p.i += 4
		p.eatSpace()

		alternate, _ = p.parseExpression()
		end = alternate.Base().Span.End
		p.eatSpace()
	}

	if p.i >= p.len {
		end = p.i
		if !isMissingExpr && parsingErr == nil {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN}
		}
	} else if p.s[p.i] == ')' {
		p.i++
		end = p.i
	} else if !isMissingExpr && parsingErr == nil {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN}
	}

	return &IfExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{openingParenIndex, end},
			Err:             parsingErr,
			ValuelessTokens: tokens,
		},
		Test:       test,
		Consequent: consequent,
		Alternate:  alternate,
	}
}

func (p *parser) parseUnaryBinaryAndParenthesizedExpression(openingParenIndex int32, previousOperatorEnd int32) Node {
	p.panicIfContextDone()

	var tokens []Token
	var startIndex = openingParenIndex
	hasPreviousOperator := previousOperatorEnd > 0

	if hasPreviousOperator {
		startIndex = previousOperatorEnd
	} else {
		tokens = []Token{{Type: OPENING_PARENTHESIS, Span: NodeSpan{openingParenIndex, openingParenIndex + 1}}}
	}
	p.eatSpaceNewlineCommaComment(&tokens)

	left, isMissingExpr := p.parseExpression(true)

	if ident, ok := left.(*IdentifierLiteral); ok && ident.Name == "if" && !hasPreviousOperator {
		return p.parseIfExpression(openingParenIndex, ident)
	}

	p.eatSpaceNewlineCommaComment(&tokens)

	if isMissingExpr {
		left.BasePtr().ValuelessTokens = append(tokens, left.BasePtr().ValuelessTokens...)

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
					tokens,
				},
			}
		}

		if p.s[p.i] == ')' {
			if !hasPreviousOperator {
				tokens = append(tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++

				return &UnknownNode{
					NodeBase: NodeBase{
						NodeSpan{startIndex, p.i},
						left.Base().Err,
						tokens,
					},
				}
			} else {
				return &MissingExpression{
					NodeBase: NodeBase{
						Span: NodeSpan{p.i - 1, p.i},
						Err:  &ParsingError{UnspecifiedParsingError, fmtExprExpectedHere(p.s, p.i, false)},
					},
				}
			}
		}

		p.i++
		rune := p.s[p.i-1]
		tokens = append(tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(rune), Span: NodeSpan{p.i - 1, p.i}})

		return &UnknownNode{
			NodeBase: NodeBase{
				NodeSpan{startIndex, p.i},
				&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParenthesizedExpression(rune)},
				tokens,
			},
		}
	}

	if stringLiteral, ok := left.(*UnquotedStringLiteral); ok && stringLiteral.Value == "-" && p.i > left.Base().Span.End {

		operand, _ := p.parseExpression()
		p.eatSpace()

		var parsingErr *ParsingError
		if p.i >= p.len || p.s[p.i] != ')' {
			if !hasPreviousOperator {
				parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_UNARY_EXPR_MISSING_OPERAND}
			}
		} else if !hasPreviousOperator {
			tokens = append(tokens, Token{Type: MINUS, Span: left.Base().Span}, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}
		return &UnaryExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{startIndex, p.i},
				Err:             parsingErr,
				ValuelessTokens: tokens,
			},
			Operator: NumberNegate,
			Operand:  operand,
		}
	}

	if p.i < p.len && p.s[p.i] == ')' { //parenthesized
		if !hasPreviousOperator {
			base := left.BasePtr()
			p.i++
			tokens := left.Base().ValuelessTokens
			base.ValuelessTokens = append([]Token{
				{Type: OPENING_PARENTHESIS, Span: NodeSpan{startIndex, startIndex + 1}},
			}, tokens...)
			base.ValuelessTokens = append(base.ValuelessTokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i - 1, p.i}})
		}
		return left
	}

	if p.i >= p.len {
		if !hasPreviousOperator {
			if left.Base().Err == nil {
				left.BasePtr().Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PARENTHESIZED_EXPR_MISSING_CLOSING_PAREN}
			}
			tokens := left.Base().ValuelessTokens
			base := left.BasePtr()
			base.ValuelessTokens = append([]Token{
				{Type: OPENING_PARENTHESIS, Span: NodeSpan{startIndex, startIndex + 1}},
			}, tokens...)
		}
		return left
	}

	makeInvalidOperatorMissingRightOperand := func(operator BinaryOperator) Node {
		return &BinaryExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{startIndex, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERAND_OR_INVALID_OPERATOR},
				ValuelessTokens: tokens,
			},
			Operator: operator,
			Left:     left,
		}
	}

	makeInvalidOperatorError := func() *ParsingError {
		return &ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR}
	}

	eatInvalidOperatorToken := func(operatorStart int32, tokens *[]Token) {
		j := operatorStart

		if isNonIdentBinaryOperatorChar(p.s[j]) {
			for j < p.i && isNonIdentBinaryOperatorChar(p.s[j]) {
				j++
			}

			for p.i < p.len && isNonIdentBinaryOperatorChar(p.s[p.i]) {
				p.i++
			}

		} else if isAlpha(p.s[j]) || p.s[j] == '_' {
			for j < p.i && isIdentChar(p.s[j]) {
				j++
			}
			for p.i < p.len && isIdentChar(p.s[p.i]) {
				p.i++
			}
		}
		*tokens = append(*tokens, Token{
			Type: INVALID_OPERATOR,
			Span: NodeSpan{Start: operatorStart, End: p.i},
			Raw:  string(p.s[operatorStart:p.i]),
		})
	}

	const (
		AND_LEN = int32(len("and"))
		OR_LEN  = int32(len("or"))
	)

	var (
		parsingErr    *ParsingError
		operator      BinaryOperator = -1
		operatorStart                = p.i
		operatorType  TokenType
		operatorToken Token
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
		if p.i >= p.len {
			return makeInvalidOperatorMissingRightOperand(-1)
		}
		if p.s[p.i] == '?' {
			operator = NilCoalescing
			operatorType = DOUBLE_QUESTION_MARK
			p.i++
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case '!':
		p.i++
		if p.i >= p.len {
			return makeInvalidOperatorMissingRightOperand(-1)
		}
		if p.s[p.i] == '=' {
			operator = NotEqual
			operatorType = EXCLAMATION_MARK_EQUAL
			p.i++
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case '=':
		p.i++
		if p.i >= p.len {
			return makeInvalidOperatorMissingRightOperand(-1)
		}
		if p.s[p.i] == '=' {
			operator = Equal
			operatorType = EQUAL_EQUAL
			p.i++
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case 'a':
		if p.len-p.i >= AND_LEN &&
			string(p.s[p.i:p.i+AND_LEN]) == "and" &&
			(p.len-p.i == AND_LEN || !isIdentChar(p.s[p.i+AND_LEN])) {
			operator = And
			p.i += AND_LEN
			operatorType = AND_KEYWORD
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case 'i':
		operatorStart := p.i

		if p.i+1 >= p.len {
			return makeInvalidOperatorMissingRightOperand(-1)
		}

		for p.i+1 < p.len && (isAlpha(p.s[p.i+1]) || p.s[p.i+1] == '-') {
			p.i++
		}

		if p.i+1 >= p.len || !isIdentChar(p.s[p.i+1]) {
			switch string(p.s[operatorStart : p.i+1]) {
			case "in":
				operator = In
				operatorType = IN
				p.i++
				break _switch
			case "is":
				operator = Is
				operatorType = IS
				p.i++
				break _switch
			case "is-not":
				operator = IsNot
				operatorType = IS_NOT
				p.i++
				break _switch
			}
		}

		//TODO: eat some chars
		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case 'k':
		KEYOF_LEN := int32(len("keyof"))
		if p.len-p.i >= KEYOF_LEN &&
			string(p.s[p.i:p.i+KEYOF_LEN]) == "keyof" &&
			(p.len-p.i == KEYOF_LEN || !isIdentChar(p.s[p.i+KEYOF_LEN])) {
			operator = Keyof
			operatorType = KEYOF
			p.i += KEYOF_LEN
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case 'n':
		NOTIN_LEN := int32(len("not-in"))
		if p.len-p.i >= NOTIN_LEN &&
			string(p.s[p.i:p.i+NOTIN_LEN]) == "not-in" &&
			(p.len-p.i == NOTIN_LEN || !isIdentChar(p.s[p.i+NOTIN_LEN])) {
			operator = NotIn
			operatorType = NOT_IN
			p.i += NOTIN_LEN
			break
		}

		NOTMATCH_LEN := int32(len("not-match"))
		if p.len-p.i >= NOTMATCH_LEN &&
			string(p.s[p.i:p.i+NOTMATCH_LEN]) == "not-match" &&
			(p.len-p.i == NOTMATCH_LEN || !isIdentChar(p.s[p.i+NOTMATCH_LEN])) {
			operator = NotMatch
			operatorType = NOT_MATCH
			p.i += NOTMATCH_LEN
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case 'm':
		MATCH_LEN := int32(len("match"))
		if p.len-p.i >= MATCH_LEN &&
			string(p.s[p.i:p.i+MATCH_LEN]) == "match" &&
			(p.len-p.i == MATCH_LEN || !isIdentChar(p.s[p.i+MATCH_LEN])) {
			operator = Match
			p.i += MATCH_LEN
			operatorType = MATCH_KEYWORD
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case 'o':
		if p.len-p.i >= OR_LEN &&
			string(p.s[p.i:p.i+OR_LEN]) == "or" &&
			(p.len-p.i == OR_LEN || !isIdentChar(p.s[p.i+OR_LEN])) {
			operator = Or
			operatorType = OR_KEYWORD
			p.i += OR_LEN
			break
		}

		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case 's':
		SUBSTROF_LEN := int32(len("substrof"))
		if p.len-p.i >= SUBSTROF_LEN &&
			string(p.s[p.i:p.i+SUBSTROF_LEN]) == "substrof" &&
			(p.len-p.i == SUBSTROF_LEN || !isIdentChar(p.s[p.i+SUBSTROF_LEN])) {
			operator = Substrof
			operatorType = SUBSTROF
			p.i += SUBSTROF_LEN
			break
		}
		eatInvalidOperatorToken(operatorStart, &tokens)
		parsingErr = makeInvalidOperatorError()
	case '.':
		operator = Dot
		operatorType = DOT
		p.i++
	case '$', '"', '\'', '`', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': //start of right operand
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERATOR}
	default:
		tokens = append(tokens, Token{Type: UNEXPECTED_CHAR, Raw: string(p.s[p.i]), Span: NodeSpan{p.i, p.i + 1}})
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

		operatorToken = Token{Type: operatorType, Span: NodeSpan{operatorStart, p.i}}
		tokens = append(tokens, operatorToken)
	}

	p.eatSpace()

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERAND}
	}

	right, isMissingExpr := p.parseExpression()

	p.eatSpace()
	if isMissingExpr {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERAND}
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
				(p.len-p.i == AND_LEN || !isIdentChar(p.s[p.i+AND_LEN])) {
				continueParsing = true
				andOrToken = Token{Type: AND_KEYWORD, Span: NodeSpan{p.i, p.i + AND_LEN}}
				p.i += AND_LEN
			}
		case 'o':
			if p.len-p.i >= OR_LEN &&
				string(p.s[p.i:p.i+OR_LEN]) == "or" &&
				(p.len-p.i == OR_LEN || !isIdentChar(p.s[p.i+OR_LEN])) {
				andOrToken = Token{Type: OR_KEYWORD, Span: NodeSpan{p.i, p.i + OR_LEN}}
				p.i += OR_LEN
				continueParsing = true
			}
		case ')':
			if !hasPreviousOperator {
				tokens = append(tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				chainElementEnd = p.i
			}
		default:
			if (operator == Or || operator == And) || (isNonIdentBinaryOperatorChar(p.s[p.i]) || isAlphaOrUndescore(p.s[p.i])) {
				continueParsing = true
				moveRightOperand = true
				andOrToken = operatorToken
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

		var openingParenToken Token
		if !hasPreviousOperator {
			openingParenToken = tokens[0]
			tokens = tokens[1:] //remove '('

			if !moveRightOperand {
				newLeft.BasePtr().Span.End = right.Base().Span.End
			}
		}

		if !moveRightOperand {
			newLeft.BasePtr().ValuelessTokens = tokens
		}

		var newOperator BinaryOperator = And
		var newComplementOperator = Or

		if andOrToken.Type == OR_KEYWORD {
			newOperator = Or
			newComplementOperator = And
		}

		newBinExpr := &BinaryExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{startIndex, p.i},
			},
			Operator: newOperator,
			Left:     newLeft,
		}

		if !hasPreviousOperator {
			newBinExpr.ValuelessTokens = []Token{openingParenToken, andOrToken}
		} else {
			newBinExpr.ValuelessTokens = []Token{andOrToken}
		}

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
				newBinExpr.ValuelessTokens = append(newBinExpr.ValuelessTokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				newBinExpr.Span.End = p.i
			}

			if rightBinExpr, ok := newRight.(*BinaryExpression); ok &&
				!rightBinExpr.IsParenthesized() && newBinExpr.Err == nil {

				subLeft, isSubLeftBinExpr := rightBinExpr.Left.(*BinaryExpression)
				subRight, isSubRightBinExpr := rightBinExpr.Right.(*BinaryExpression)

				err := &ParsingError{UnspecifiedParsingError, ALL_BIN_EXPR_CHAIN_SHOULD_HAVE_THE_SAME_OPERATOR}

				if isSubLeftBinExpr {
					if (!subLeft.IsParenthesized() && (subLeft.Operator == newComplementOperator)) ||
						(rightBinExpr.Operator == newComplementOperator) {
						newBinExpr.Err = err
					}
				}

				if isSubRightBinExpr {
					if (!subRight.IsParenthesized() && subRight.Operator == newComplementOperator) ||
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
			ValuelessTokens: tokens,
		},
		Operator: operator,
		Left:     left,
		Right:    right,
	}
}

func (p *parser) parseComplexStringPatternElement() Node {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *ParsingError

	if p.i >= p.len {
		return &InvalidComplexStringPatternElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, fmtAPatternWasExpected(p.s, p.i)},
				nil,
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
					nil,
				},
			}
		}

		if p.s[p.i] == '|' { //parenthesized union
			element := p.parseComplexStringPatternUnion(elemStart)

			return element
		}

		return p.parseComplexStringPatternPiece(elemStart, nil)
	case p.s[p.i] == '"' || p.s[p.i] == '\'':
		e, _ := p.parseExpression()
		return e
	case p.s[p.i] == '-' || isDecDigit(p.s[p.i]):
		e, _ := p.parseExpression()
		switch e.(type) {
		case *IntegerRangeLiteral:
		default:
			return &InvalidComplexStringPatternElement{
				NodeBase: NodeBase{
					e.Base().Span,
					&ParsingError{UnspecifiedParsingError, INVALID_COMPLEX_PATTERN_ELEMENT},
					nil,
				},
			}
		}
		return e
	case p.s[p.i] == '%':
		return p.parsePercentPrefixedPattern()
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
			nil,
		},
	}
}

// parseParenthesizedCallArgs parses the arguments of a parenthesized call up until the closing parenthesis (included)
func (p *parser) parseParenthesizedCallArgs(call *CallExpression) *CallExpression {
	p.panicIfContextDone()

	var (
		lastSpreadArg *SpreadArgument = nil
		argErr        *ParsingError
	)

	//parse arguments
	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewlineComma(&call.ValuelessTokens)

		if p.i >= p.len || p.s[p.i] == ')' {
			break
		}

		if lastSpreadArg != nil {
			argErr = &ParsingError{UnspecifiedParsingError, SPREAD_ARGUMENT_CANNOT_BE_FOLLOWED_BY_ADDITIONAL_ARGS}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			lastSpreadArg = &SpreadArgument{
				NodeBase: NodeBase{
					Span:            NodeSpan{p.i, 0},
					ValuelessTokens: []Token{{Type: THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}}},
					Err:             argErr,
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

			arg = &UnknownNode{
				NodeBase: NodeBase{
					arg.Base().Span,
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCallArguments(p.s[p.i-1])},
					[]Token{{Type: UNEXPECTED_CHAR, Span: arg.Base().Span, Raw: string(p.s[p.i-1])}},
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
		p.eatSpaceNewlineComma(&call.ValuelessTokens)
	}

	var parsingErr *ParsingError

	if p.i >= p.len || p.s[p.i] != ')' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_CALL_MISSING_CLOSING_PAREN}
	} else {
		call.ValuelessTokens = append(call.ValuelessTokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	call.NodeBase.Span.End = p.i
	call.Err = parsingErr
	return call
}

// parseCallArgsNoParenthesis parses the arguments of a call without parenthesis up until the end of the line or the next non-opening delimiter
func (p *parser) parseCallArgsNoParenthesis(call *CallExpression) {
	p.panicIfContextDone()

	var lastSpreadArg *SpreadArgument = nil

	for p.i < p.len && (!isUnpairedOrIsClosingDelim(p.s[p.i]) || p.s[p.i] == ':') {
		p.eatSpaceComments(&call.ValuelessTokens)

		if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && p.s[p.i] != ':') {
			break
		}

		var argErr *ParsingError

		if lastSpreadArg != nil {
			argErr = &ParsingError{UnspecifiedParsingError, SPREAD_ARGUMENT_CANNOT_BE_FOLLOWED_BY_ADDITIONAL_ARGS}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {

			lastSpreadArg = &SpreadArgument{
				NodeBase: NodeBase{
					Span:            NodeSpan{p.i, 0},
					ValuelessTokens: []Token{{Type: THREE_DOTS, Span: NodeSpan{p.i, p.i + 3}}},
					Err:             argErr,
				},
			}
			p.i += 3
		}

		arg, isMissingExpr := p.parseExpression()

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

				arg = &UnknownNode{
					NodeBase: NodeBase{
						NodeSpan{p.i - 1, p.i},
						&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCallArguments(p.s[p.i-1])},
						[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i - 1, p.i}, Raw: string(p.s[p.i-1])}},
					},
				}
			}
		}

		call.Arguments = append(call.Arguments, arg)

		p.eatSpaceComments(&call.ValuelessTokens)
	}
}

func ParseDateLiteral(braw []byte) (date time.Time, parsingErr *ParsingError) {
	if len(braw) > 40 || !DATE_LITERAL_REGEX.Match(braw) {
		return time.Time{}, &ParsingError{UnspecifiedParsingError, INVALID_DATE_LITERAL}
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

	for _, part := range parts[1 : len32(parts)-1] {
		switch part[len32(part)-1] {
		case 't':
			month = string(part[:len32(part)-2])
		case 'd':
			day = string(part[:len32(part)-1])
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
		return time.Time{}, &ParsingError{UnspecifiedParsingError, fmt.Sprintf("invalid time location in literal: %s", err)}
	}

	nanoseconds := 1_000*mustAtoi(us) + 1_000_000*mustAtoi(ms)

	return time.Date(
		mustAtoi(year), time.Month(mustAtoi(month)), mustAtoi(day),
		mustAtoi(hour), mustAtoi(minute), mustAtoi(second), nanoseconds, loc), nil
}

func (p *parser) parseDateLiterals(start int32) *DateLiteral {
	p.panicIfContextDone()

	literal := &DateLiteral{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			nil,
			nil,
		},
	}
	p.i++

	if p.i >= p.len {
		literal.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_DATE_LITERAL}
		return literal
	}

	r := p.s[p.i]

	if r == '-' {
		p.i++
		if p.i >= p.len {
			literal.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_DATE_LITERAL}
			return literal
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

	literal.Span.End = p.i
	literal.Raw = string(p.s[start:p.i])
	braw := []byte(literal.Raw)

	date, err := ParseDateLiteral(braw)

	if err != nil {
		literal.Err = err
	} else {
		literal.Value = date
	}

	return literal

}

func (p *parser) parsePortLiteral() *PortLiteral {
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

	var parsingErr *ParsingError
	if portNumber > math.MaxUint16 {
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_PORT_LITERAL_INVALID_PORT_NUMBER}
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
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PORT_LITERAL_MISSING_SCHEME_NAME_AFTER_SLASH}
		}
	}

	return &PortLiteral{
		Raw: string(p.s[start:p.i]),
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			nil,
		},
		PortNumber: uint16(portNumber),
		SchemeName: schemeName,
	}
}

func (p *parser) parseNumberAndNumberRange() Node {
	p.panicIfContextDone()

	start := p.i
	var parsingErr *ParsingError
	base := 10

	parseIntegerLiteral := func(raw string, start, end int32, base int) (*IntLiteral, int64) {
		s := raw
		if base == 16 {
			s = strings.TrimPrefix(s, "0x")
		}

		integer, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), base, 64)
		if err != nil {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_INT_LIT}
		}

		return &IntLiteral{
			NodeBase: NodeBase{
				NodeSpan{start, end},
				parsingErr,
				nil,
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
			tokens := []Token{{Type: TWO_DOTS, Span: NodeSpan{p.i - 1, p.i + 1}}}

			p.i++
			if p.i >= p.len || !isDecDigit(p.s[p.i]) {
				return &IntegerRangeLiteral{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_INT_RANGE_LIT},
						tokens,
					},
					LowerBound: lowerIntLiteral,
					UpperBound: nil,
				}
			}

			upperStart := p.i

			for p.i < p.len && (isDecDigit(p.s[p.i]) || p.s[p.i] == '-' || p.s[p.i] == '_') {
				p.i++
			}

			upper := string(p.s[upperStart:p.i])

			upperIntLiteral, _ := parseIntegerLiteral(upper, upperStart, p.i, 10)
			return &IntegerRangeLiteral{
				NodeBase: NodeBase{
					NodeSpan{lowerIntLiteral.Base().Span.Start, upperIntLiteral.Base().Span.End},
					nil,
					tokens,
				},
				LowerBound: lowerIntLiteral,
				UpperBound: upperIntLiteral,
			}
		}

		//else float
		for p.i < p.len && (isDecDigit(p.s[p.i]) || p.s[p.i] == '-') {
			p.i++
		}
	} else if p.i < p.len-1 && p.s[p.i] == 'x' && isHexDigit(p.s[p.i+1]) { //hexa decimal
		base = 16
		p.i++
		for p.i < p.len && (isHexDigit(p.s[p.i]) || p.s[p.i] == '_') {
			p.i++
		}
	}

	raw := string(p.s[start:p.i])

	var literal Node

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

		float, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_FLOAT_LIT}
		}

		literal = &FloatLiteral{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				nil,
			},
			Raw:   raw,
			Value: float,
		}

	} else {
		literal, _ = parseIntegerLiteral(raw, start, p.i, base)
	}

	return literal
}

func (p *parser) parseByteSlices() Node {
	p.panicIfContextDone()

	start := p.i //index of '0'
	p.i++

	var (
		parsingError *ParsingError
		value        []byte
	)

	switch p.s[p.i] {
	case 'x':
		p.i++
		if p.i >= p.len || p.s[p.i] != '[' {
			return &ByteSliceLiteral{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_HEX_BYTE_SICE_LIT_MISSING_BRACKETS},
					nil,
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
			default:
				if parsingError == nil {
					parsingError = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInHexadecimalByteSliceLiteral(r)}
				} else {
					parsingError.message += "\n" + fmtUnexpectedCharInHexadecimalByteSliceLiteral(r)
				}
			}
			p.i++
			p.eatSpace()
		}

		if parsingError == nil {
			if len32(buff)%2 != 0 {
				parsingError = &ParsingError{UnspecifiedParsingError, INVALID_HEX_BYTE_SICE_LIT_LENGTH_SHOULD_BE_EVEN}
			} else {
				value = make([]byte, hex.DecodedLen(len(buff)))
				_, err := hex.Decode(value, buff)
				if err != nil {
					parsingError = &ParsingError{UnspecifiedParsingError, INVALID_HEX_BYTE_SICE_LIT_FAILED_TO_DECODE}
				}
			}
		}

	case 'b':
		p.i++
		if p.i >= p.len || p.s[p.i] != '[' {
			return &ByteSliceLiteral{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_BYTE_SICE_LIT_MISSING_BRACKETS},
					nil,
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
				if parsingError == nil {
					parsingError = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBinByteSliceLiteral(r)}
				} else {
					parsingError.message += "\n" + fmtUnexpectedCharInBinByteSliceLiteral(r)
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
			return &ByteSliceLiteral{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_DECIMAL_BYTE_SICE_LIT_MISSING_BRACKETS},
					nil,
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
				if parsingError == nil {
					parsingError = &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInDecimalByteSliceLiteral(r)}
				} else {
					parsingError.message += "\n" + fmtUnexpectedCharInDecimalByteSliceLiteral(r)
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
							parsingError = &ParsingError{UnspecifiedParsingError, message}
						} else {
							parsingError.message += "\n" + message
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
		return &ByteSliceLiteral{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNKNOWN_BYTE_SLICE_BASE},
				nil,
			},
		}
	}

	if p.i >= p.len {
		if parsingError == nil {
			parsingError = &ParsingError{UnspecifiedParsingError, UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET}
		} else {
			parsingError.message += "\n" + UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET
		}
	} else {
		p.i++
	}

	return &ByteSliceLiteral{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingError,
			nil,
		},
		Raw:   string(p.s[start:p.i]),
		Value: value,
	}
}

func (p *parser) parseNumberAndRangeAndRateLiterals() Node {
	p.panicIfContextDone()

	start := p.i //index of first digit or '-'
	e := p.parseNumberAndNumberRange()

	var fValue float64
	var isFloat = false

	switch n := e.(type) {
	case *IntLiteral:
		fValue = float64(n.Value)
	case *FloatLiteral:
		fValue = float64(n.Value)
		isFloat = true
	default:
		return n
	}

	if p.i < p.len && (isAlpha(p.s[p.i]) || p.s[p.i] == '%') { //quantity literal or rate literal
		return p.parseQuantityOrRateLiteral(start, fValue, isFloat)
	}

	return e
}

func (p *parser) parseQuantityOrRateLiteral(start int32, fValue float64, float bool) Node {
	p.panicIfContextDone()

	unitStart := p.i
	var parsingErr *ParsingError

	//date literal
	if !float && p.s[unitStart] == 'y' && (p.i < p.len-1 && p.s[p.i+1] == '-') {
		return p.parseDateLiterals(start)
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
		case *IntLiteral:
			fValue = float64(n.Value)
		case *FloatLiteral:
			fValue = float64(n.Value)
		default:
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_QUANTITY_LIT}
			break loop
		}

		values = append(values, fValue)

		if p.i >= p.len || !isAlpha(p.s[p.i]) {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_QUANTITY_LIT}
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

	literal := &QuantityLiteral{
		NodeBase: NodeBase{
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
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT_DIV_SYMBOL_SHOULD_BE_FOLLOWED_BY_UNIT}
			} else {
				if !isAlpha(p.s[p.i]) {
					parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT}
				} else {
					for p.i < p.len && isAlpha(p.s[p.i]) {
						p.i++
					}
					rateUnit = string(p.s[rateUnitStart:p.i])

					if p.i < p.len && isIdentChar(p.s[p.i]) {
						parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT}
					}
				}
			}

			return &RateLiteral{
				NodeBase: NodeBase{
					NodeSpan{literal.Base().Span.Start, p.i},
					parsingErr,
					nil,
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

// parseExpression parses any expression, if expr is a *MissingExpression isMissingExpr will be true.
func (p *parser) parseExpression(precededByOpeningParen ...bool) (expr Node, isMissingExpr bool) {
	p.panicIfContextDone()

	if p.remainingNoCheckFuel <= 0 {
		p.remainingNoCheckFuel = 0
		if p.context != nil {
			select {
			case <-p.context.Done():
				panic(context.Canceled)
			default:
				p.remainingNoCheckFuel = 100
			}
		}
	} else {
		p.remainingNoCheckFuel--
	}
	__start := p.i
	// these variables are only used for expressions that can be on the left side of a member/slice/index/call expression,
	// other expressions are directly returned.
	var (
		lhs   Node
		first Node
	)

	if p.i >= p.len {
		return &MissingExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{p.i - 1, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, fmtExprExpectedHere(p.s, p.i, false)},
			},
		}, true
	}

	switch p.s[p.i] {
	case '$': //normal & global variables
		start := p.i
		isGlobal := false
		p.i++

		if p.i < p.len && p.s[p.i] == '$' {
			isGlobal = true
			p.i++
		}

		for p.i < p.len && isIdentChar(p.s[p.i]) {
			p.i++
		}

		if isGlobal {
			lhs = &GlobalVariable{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
				},
				Name: string(p.s[start+2 : p.i]),
			}
		} else {
			lhs = &Variable{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
				},
				Name: string(p.s[start+1 : p.i]),
			}
		}

	case '!':
		p.i++
		operand, _ := p.parseExpression()

		return &UnaryExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{__start, operand.Base().Span.End},
				ValuelessTokens: []Token{{Type: EXCLAMATION_MARK, Span: NodeSpan{__start, __start + 1}}},
			},
			Operator: BoolNegate,
			Operand:  operand,
		}, false
	case '~':
		p.i++
		expr, _ := p.parseExpression()

		return &RuntimeTypeCheckExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{__start, expr.Base().Span.End},
				ValuelessTokens: []Token{{Type: TILDE, Span: NodeSpan{__start, __start + 1}}},
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
			return p.parseURLLike(p.i), false
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return p.parsePortLiteral(), false
		case '{':
			return p.parseDictionaryLiteral(), false
		}

	//TODO: refactor ?
	case '_', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		identStartingExpr := p.parseIdentStartingExpression()
		var name string

		switch v := identStartingExpr.(type) {
		case *IdentifierLiteral:
			name = v.Name
			switch name {
			case "go":
				return p.parseSpawnExpression(identStartingExpr), false
			case "fn":
				return p.parseFunction(identStartingExpr.Base().Span.Start), false
			case "s":
				if p.i < p.len && p.s[p.i] == '!' {
					p.i++
					return p.parseTopCssSelector(p.i - 2), false
				}
			case "Mapping":
				return p.parseMappingExpression(v), false
			case "comp":
				return p.parseComputeExpression(v), false
			case "udata":
				return p.parseUdataLiteral(v), false
			case "concat":
				return p.parseConcatenationExpression(v, len(precededByOpeningParen) > 0 && precededByOpeningParen[0]), false
			case "testsuite":
				return p.parseTestSuiteExpression(v), false
			case "testcase":
				return p.parseTestCaseExpression(v), false
			case "lifetimejob":
				return p.parseLifetimeJobExpression(v), false
			case "on":
				return p.parseReceptionHandlerExpression(v), false
			case "sendval":
				return p.parseSendValueExpression(v), false
			}
			if isKeyword(name) {
				return v, false
			}
			if p.inPattern {
				result := &PatternIdentifierLiteral{
					NodeBase:   v.NodeBase,
					Unprefixed: true,
					Name:       v.Name,
				}
				if p.i < p.len && p.s[p.i] == '(' {
					return p.parsePatternCall(result), false
				} else {
					return result, false
				}
			}
		case *IdentifierMemberExpression:
			if p.inPattern && len(v.PropertyNames) == 1 {
				base := v.Left.NodeBase
				base.Span.End += 1 //add one for the dot

				result := &PatternNamespaceMemberExpression{
					NodeBase: v.NodeBase,
					Namespace: &PatternNamespaceIdentifierLiteral{
						NodeBase:   base,
						Unprefixed: true,
						Name:       v.Left.Name,
					},
					MemberName: v.PropertyNames[0],
				}
				if p.i < p.len && p.s[p.i] == '(' {
					return p.parsePatternCall(result), false
				} else {
					return result, false
				}
			}

			name = v.Left.Name
		case *SelfExpression, *MemberExpression:
			lhs = identStartingExpr
		default:
			return v, false
		}

		if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
			return identStartingExpr, false
		}

		if p.s[p.i] == '<' && NodeIs(identStartingExpr, (*IdentifierLiteral)(nil)) {
			return p.parseXMLExpression(identStartingExpr.(*IdentifierLiteral)), false
		}

		call := p.tryParseCall(identStartingExpr, name)
		if call != nil {
			identStartingExpr = call
		}

		lhs = identStartingExpr
	case '0':
		if p.i < p.len-2 && isByteSliceBase(p.s[p.i+1]) && p.s[p.i+2] == '[' {
			return p.parseByteSlices(), false
		}
		return p.parseNumberAndRangeAndRateLiterals(), false
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.parseNumberAndRangeAndRateLiterals(), false
	case '{':
		if p.inPattern {
			return p.parseObjectPatternLiteral(false), false
		}
		return p.parseObjectOrRecordLiteral(false), false
	case '[':
		if p.inPattern {
			return p.parseListPatternLiteral(false), false
		}
		return p.parseListOrTupleLiteral(false), false
	case '|':
		if p.inPattern {
			return p.parsePatternUnion(p.i, false), false
		}
		break
	case '\'':
		return p.parseRuneRuneRange(), false
	case '"':
		return p.parseQuotedStringLiteral(), false
	case '`':
		return p.parseStringTemplateLiteralOrMultilineStringLiteral(nil), false
	case '+':
		if p.i < p.len-1 && isDecDigit(p.s[p.i+1]) {
			break
		}
		start := p.i
		return p.parseUnquotedStringLiteralAndEmailAddress(start), false

	case '/':
		return p.parsePathLikeExpression(false), false
	case '.':
		return p.parseDotStartingExpression(), false
	case '-':
		return p.parseDashStartingExpression(), false
	case '#':
		if p.i < p.len-1 {
			switch p.s[p.i+1] {
			case '{':
				return p.parseObjectOrRecordLiteral(true), false
			case '[':
				return p.parseListOrTupleLiteral(true), false
			}
		}
		p.i++

		for p.i < p.len && isIdentChar(p.s[p.i]) {
			p.i++
		}

		var parsingErr *ParsingError

		if p.i == __start+1 {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT}
		}

		return &UnambiguousIdentifierLiteral{
			NodeBase: NodeBase{
				Span: NodeSpan{__start, p.i},
				Err:  parsingErr,
			},
			Name: string(p.s[__start+1 : p.i]),
		}, false
	case '@':
		return p.parseLazyAndHostAliasStuff(), false
	case '%':
		patt := p.parsePercentPrefixedPattern()

		switch patt.(type) {
		case *PatternIdentifierLiteral, *PatternNamespaceMemberExpression:
			if p.i < p.len && p.s[p.i] == '`' {
				return p.parseStringTemplateLiteralOrMultilineStringLiteral(patt), false
			}
		}
		return patt, false
	case '(': //parenthesized expression, unary expression, binary expression, pattern union
		openingParenIndex := p.i
		p.i++

		lhs = p.parseUnaryBinaryAndParenthesizedExpression(openingParenIndex, -1)
		if p.i >= p.len {
			return lhs, false
		}
	}

	first = lhs

loop:
	for lhs != nil && p.i < p.len && !isUnpairedOrIsClosingDelim(p.s[p.i]) {

		switch {
		//member expressions, index/slice expressions, extraction expression
		case p.s[p.i] == '[' || p.s[p.i] == '.':
			dot := p.s[p.i] == '.'
			p.i++
			start := p.i
			isOptional := false

			isDot := p.s[p.i-1] == '.'
			isBracket := !isDot

			if isDot && p.i < p.len && p.s[p.i] == '?' {
				isOptional = true
				p.i++
				start = p.i
			}

			if p.i >= p.len || (isUnpairedOrIsClosingDelim(p.s[p.i]) && (dot || (p.s[p.i] != ':' && p.s[p.i] != ']'))) {
				//unterminated member expression
				if isDot {
					return &MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left:     lhs,
						Optional: isOptional,
					}, false
				}
				return &InvalidMemberLike{
					NodeBase: NodeBase{
						NodeSpan{first.Base().Span.Start, p.i},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_MEMB_OR_INDEX_EXPR},
						nil,
					},
					Left: lhs,
				}, false
			}

			switch {
			case isBracket: //index/slice expression
				p.eatSpace()

				if p.i >= p.len {
					return &InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
							nil,
						},
						Left: lhs,
					}, false
				}

				var startIndex Node
				var endIndex Node
				isSliceExpr := p.s[p.i] == ':'

				if isSliceExpr {
					p.i++
				} else {
					startIndex, _ = p.parseExpression()
				}

				p.eatSpace()

				if p.i >= p.len {
					return &InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR},
							nil,
						},
						Left: lhs,
					}, false
				}

				if p.s[p.i] == ':' {
					if isSliceExpr {
						return &SliceExpression{
							NodeBase: NodeBase{
								NodeSpan{first.Base().Span.Start, p.i},
								&ParsingError{UnspecifiedParsingError, INVALID_SLICE_EXPR_SINGLE_COLON},
								nil,
							},
							Indexed:    lhs,
							StartIndex: startIndex,
							EndIndex:   endIndex,
						}, false
					}
					isSliceExpr = true
					p.i++
				}

				p.eatSpace()

				if isSliceExpr && startIndex == nil && (p.i >= p.len || p.s[p.i] == ']') {
					return &SliceExpression{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_SLICE_EXPR_MISSING_END_INDEX},
							nil,
						},
						Indexed:    lhs,
						StartIndex: startIndex,
						EndIndex:   endIndex,
					}, false
				}

				if p.i < p.len && p.s[p.i] != ']' && isSliceExpr {
					endIndex, _ = p.parseExpression()
				}

				p.eatSpace()

				if p.i >= p.len || p.s[p.i] != ']' {
					return &InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{first.Base().Span.Start, p.i},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_INDEX_OR_SLICE_EXPR_MISSING_CLOSING_BRACKET},
							nil,
						},
						Left: lhs,
					}, false
				}

				p.i++

				spanStart := lhs.Base().Span.Start
				if lhs == first {
					spanStart = __start
				}

				if isSliceExpr {
					lhs = &SliceExpression{
						NodeBase: NodeBase{
							NodeSpan{spanStart, p.i},
							nil,
							nil,
						},
						Indexed:    lhs,
						StartIndex: startIndex,
						EndIndex:   endIndex,
					}
					continue loop
				}

				lhs = &IndexExpression{
					NodeBase: NodeBase{
						NodeSpan{spanStart, p.i},
						nil,
						nil,
					},
					Indexed: lhs,
					Index:   startIndex,
				}
			case p.s[p.i] == '{': //extraction expression (result is returned, the loop is not continued)
				p.i--
				keyList := p.parseKeyList()

				lhs = &ExtractionExpression{
					NodeBase: NodeBase{
						NodeSpan{lhs.Base().Span.Start, keyList.Span.End},
						nil,
						nil,
					},
					Object: lhs,
					Keys:   keyList,
				}
				continue loop
			default:
				isDynamic := false
				isComputed := false
				spanStart := lhs.Base().Span.Start
				var computedPropertyNode Node
				var propertyNameIdent *IdentifierLiteral
				propNameStart := start

				if !isOptional && p.i < p.len {
					switch p.s[p.i] {
					case '<':
						isDynamic = true
						p.i++
						propNameStart++
					case '(':
						isComputed = true
						p.i++
						computedPropertyNode = p.parseUnaryBinaryAndParenthesizedExpression(p.i-1, -1)
					}
				}

				newMemberExpression := func(err *ParsingError) Node {
					if isDynamic {
						return &DynamicMemberExpression{
							NodeBase: NodeBase{
								NodeSpan{spanStart, p.i},
								err,
								nil,
							},
							Left:         lhs,
							PropertyName: propertyNameIdent,
							Optional:     isOptional,
						}
					}
					if isComputed {
						return &ComputedMemberExpression{
							NodeBase: NodeBase{
								NodeSpan{spanStart, p.i},
								err,
								nil,
							},
							Left:         lhs,
							PropertyName: computedPropertyNode,
							Optional:     isOptional,
						}
					}
					return &MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{spanStart, p.i},
							err,
							nil,
						},
						Left:         lhs,
						PropertyName: propertyNameIdent,
						Optional:     isOptional,
					}
				}

				if !isComputed {
					if isDynamic && p.i >= p.len {
						return newMemberExpression(&ParsingError{UnspecifiedParsingError, UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR}), false
					}

					//member expression with invalid property name
					if !isAlpha(p.s[p.i]) && p.s[p.i] != '_' {
						return newMemberExpression(&ParsingError{UnspecifiedParsingError, fmtPropNameShouldStartWithAletterNot(p.s[p.i])}), false
					}

					for p.i < p.len && isIdentChar(p.s[p.i]) {
						p.i++
					}

					propName := string(p.s[propNameStart:p.i])
					if lhs == first {
						spanStart = __start
					}

					propertyNameIdent = &IdentifierLiteral{
						NodeBase: NodeBase{
							NodeSpan{propNameStart, p.i},
							nil,
							nil,
						},
						Name: propName,
					}
				}

				lhs = newMemberExpression(nil)
			}
		case ((p.i < p.len && p.s[p.i] == '(') ||
			(p.i < p.len-1 && p.s[p.i] == '!' && p.s[p.i+1] == '(')): //call: <lhs> '(' ...
			var tokens []Token

			must := false
			if p.s[p.i] == '!' {
				must = true
				p.i++
				tokens = append(tokens,
					Token{Type: EXCLAMATION_MARK, Span: NodeSpan{p.i - 1, p.i}},
					Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}},
				)
			} else {
				tokens = append(tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			}

			p.i++
			spanStart := lhs.Base().Span.Start

			if lhs == first {
				spanStart = __start
			}

			call := &CallExpression{
				NodeBase: NodeBase{
					NodeSpan{spanStart, 0},
					nil,
					tokens,
				},
				Callee:    lhs,
				Arguments: nil,
				Must:      must,
			}

			lhs = p.parseParenthesizedCallArgs(call)
		case p.s[p.i] == '?':
			p.i++
			lhs = &BooleanConversionExpression{
				NodeBase: NodeBase{
					NodeSpan{__start, p.i},
					nil,
					nil,
				},
				Expr: lhs,
			}
		default:
			break loop
		}
	}

	if lhs != nil {
		return lhs, false
	}

	return &MissingExpression{
		NodeBase: NodeBase{
			Span: NodeSpan{p.i, p.i + 1},
			Err:  &ParsingError{UnspecifiedParsingError, fmtExprExpectedHere(p.s, p.i, true)},
		},
	}, true
}

// can return nil
func (p *parser) parsePreInitIfPresent() *PreinitStatement {
	p.panicIfContextDone()

	var preinit *PreinitStatement
	if p.i < p.len && strings.HasPrefix(string(p.s[p.i:]), PREINIT_KEYWORD_STR) {
		start := p.i

		tokens := []Token{{Type: PREINIT_KEYWORD, Span: NodeSpan{p.i, p.i + int32(len(PREINIT_KEYWORD_STR))}}}
		p.i += int32(len(PREINIT_KEYWORD_STR))

		p.eatSpace()

		var parsingErr *ParsingError
		var preinitBlock *Block
		if p.i >= p.len || p.s[p.i] != '{' {
			parsingErr = &ParsingError{UnspecifiedParsingError, PREINIT_KEYWORD_SHOULD_BE_FOLLOWED_BY_A_BLOCK}
		} else {
			preinitBlock = p.parseBlock()
		}

		preinit = &PreinitStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, preinitBlock.Base().Span.End},
				Err:             parsingErr,
				ValuelessTokens: tokens,
			},
			Block: preinitBlock,
		}
	}
	return preinit
}

// can return nil
func (p *parser) parseManifestIfPresent() *Manifest {
	p.panicIfContextDone()

	var manifest *Manifest
	if p.i < p.len && strings.HasPrefix(string(p.s[p.i:]), MANIFEST_KEYWORD_STR) {
		start := p.i

		tokens := []Token{{Type: MANIFEST_KEYWORD, Span: NodeSpan{p.i, p.i + int32(len(MANIFEST_KEYWORD_STR))}}}
		p.i += int32(len(MANIFEST_KEYWORD_STR))

		p.eatSpace()
		manifestObject, isMissingExpr := p.parseExpression()

		var err *ParsingError
		if _, ok := manifestObject.(*ObjectLiteral); !ok && !isMissingExpr {
			err = &ParsingError{UnspecifiedParsingError, INVALID_MANIFEST_DESC_VALUE}
		}

		manifest = &Manifest{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, manifestObject.Base().Span.End},
				Err:             err,
				ValuelessTokens: tokens,
			},
			Object: manifestObject,
		}

	}
	return manifest
}

func (p *parser) parseSingleGlobalConstDeclaration(declarations *[]*GlobalConstantDeclaration) {
	p.panicIfContextDone()

	var declParsingErr *ParsingError

	lhs, _ := p.parseExpression()
	globvar, ok := lhs.(*IdentifierLiteral)
	if !ok {
		declParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_LHS_MUST_BE_AN_IDENT}
	} else if isKeyword(globvar.Name) {
		declParsingErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
	}

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '=' {
		if globvar != nil {
			declParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidConstDeclMissingEqualSign(globvar.Name)}
		} else {
			declParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_MISSING_EQL_SIGN}
		}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &GlobalConstantDeclaration{
			NodeBase: NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				nil,
			},
			Left: lhs,
		})
		return
	}

	equalSignIndex := p.i

	p.i++
	p.eatSpace()

	rhs, _ := p.parseExpression()

	*declarations = append(*declarations, &GlobalConstantDeclaration{
		NodeBase: NodeBase{
			NodeSpan{lhs.Base().Span.Start, rhs.Base().Span.End},
			declParsingErr,
			[]Token{{Type: EQUAL, Span: NodeSpan{equalSignIndex, equalSignIndex + 1}}},
		},
		Left:  lhs,
		Right: rhs,
	})
}

func (p *parser) parseGlobalConstantDeclarations() *GlobalConstantDeclarations {
	p.panicIfContextDone()

	//nil is returned if there are no global constant declarations (no const (...) section)

	var (
		start            = p.i
		constKeywordSpan = NodeSpan{p.i, p.i + int32(len(CONST_KEYWORD_STR))}
		valuelessTokens  []Token
	)

	if p.i < p.len && strings.HasPrefix(string(p.s[p.i:]), CONST_KEYWORD_STR) {
		p.i += int32(len(CONST_KEYWORD_STR))
		valuelessTokens = append(valuelessTokens, Token{Type: CONST_KEYWORD, Span: constKeywordSpan})

		p.eatSpace()
		var (
			declarations []*GlobalConstantDeclaration
			parsingErr   *ParsingError
		)

		if p.i >= p.len || p.s[p.i] == '\n' {
			return &GlobalConstantDeclarations{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
					[]Token{{Type: CONST_KEYWORD, Span: constKeywordSpan}},
				},
			}
		}

		if p.s[p.i] != '(' { //single declaration, no parenthesis
			p.parseSingleGlobalConstDeclaration(&declarations)
		} else {
			valuelessTokens = append(valuelessTokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
			p.i++

			for p.i < p.len && p.s[p.i] != ')' {
				p.eatSpaceNewlineComment(&valuelessTokens)

				if p.i < p.len && p.s[p.i] == ')' {
					break
				}

				if p.i >= p.len {
					parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_MISSING_CLOSING_PAREN}
					break
				}

				p.parseSingleGlobalConstDeclaration(&declarations)

				p.eatSpaceNewlineComment(&valuelessTokens)
			}

			if p.i < p.len && p.s[p.i] == ')' {
				valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
			}
		}

		decls := &GlobalConstantDeclarations{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				valuelessTokens,
			},
			Declarations: declarations,
		}

		return decls
	}

	return nil
}

func (p *parser) parseSingleLocalVarDeclaration(declarations *[]*LocalVariableDeclaration) {
	p.panicIfContextDone()

	var declParsingErr *ParsingError

	lhs, _ := p.parseExpression()
	ident, ok := lhs.(*IdentifierLiteral)
	if !ok {
		declParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT}
	} else if isKeyword(ident.Name) {
		declParsingErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
	}

	p.eatSpace()

	isAcceptedFirstTypeChar := func(r rune) bool {
		return r == '%' || isFirstIdentChar(r) || isOpeningDelim(r)
	}

	if p.i >= p.len || (p.s[p.i] != '=' && !isAcceptedFirstTypeChar(p.s[p.i])) {
		if ident != nil {
			declParsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidLocalVarDeclMissingEqualSign(ident.Name)}
		}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &LocalVariableDeclaration{
			NodeBase: NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				nil,
			},
			Left: lhs,
		})
		return
	}

	var type_ Node

	if isAcceptedFirstTypeChar(p.s[p.i]) {
		prev := p.inPattern
		p.inPattern = true

		type_, _ = p.parseExpression()
		p.inPattern = prev
	}

	p.eatSpace()

	//temporary
	if p.i >= p.len || p.s[p.i] != '=' {
		declParsingErr = &ParsingError{UnspecifiedParsingError, "invalid local variable declaration, missing '=' after type annotation"}
		if p.i < p.len {
			p.i++
		}
		*declarations = append(*declarations, &LocalVariableDeclaration{
			NodeBase: NodeBase{
				NodeSpan{lhs.Base().Span.Start, p.i},
				declParsingErr,
				nil,
			},
			Left: lhs.(*IdentifierLiteral),
			Type: type_,
		})
		return
	}

	equalSignIndex := p.i
	p.i++
	p.eatSpace()

	rhs, _ := p.parseExpression()

	*declarations = append(*declarations, &LocalVariableDeclaration{
		NodeBase: NodeBase{
			NodeSpan{lhs.Base().Span.Start, rhs.Base().Span.End},
			declParsingErr,
			[]Token{{Type: EQUAL, Span: NodeSpan{equalSignIndex, equalSignIndex + 1}}},
		},
		Left:  lhs,
		Type:  type_,
		Right: rhs,
	})
}

func (p *parser) parseLocalVariableDeclarations(varKeywordBase NodeBase) *LocalVariableDeclarations {
	p.panicIfContextDone()

	var (
		start           = varKeywordBase.Span.Start
		valuelessTokens = []Token{{Type: VAR_KEYWORD, Span: varKeywordBase.Span}}
	)

	p.eatSpace()
	var (
		declarations []*LocalVariableDeclaration
		parsingErr   *ParsingError
	)

	if p.i >= p.len || p.s[p.i] == '\n' {
		return &LocalVariableDeclarations{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
				valuelessTokens,
			},
		}
	}

	if isAlpha(p.s[p.i]) || p.s[p.i] == '_' {
		p.parseSingleLocalVarDeclaration(&declarations)
	} else {
		if p.s[p.i] != '(' {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_OPENING_PAREN_EXPECTED}
		}

		p.i++

		for p.i < p.len && p.s[p.i] != ')' {
			p.eatSpaceNewlineComment(&valuelessTokens)

			if p.i < p.len && p.s[p.i] == ')' {
				break
			}

			if p.i >= p.len {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_MISSING_CLOSING_PAREN}
				break
			}

			p.parseSingleLocalVarDeclaration(&declarations)

			p.eatSpaceNewlineComment(&valuelessTokens)
		}

		if p.i < p.len && p.s[p.i] == ')' {
			p.i++
		}
	}

	decls := &LocalVariableDeclarations{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			valuelessTokens,
		},
		Declarations: declarations,
	}

	return decls
}

func (p *parser) parseEmbeddedModule() *EmbeddedModule {
	p.panicIfContextDone()

	start := p.i
	p.i++

	var (
		emod             = &EmbeddedModule{}
		prevStmtEndIndex = int32(-1)
		prevStmtErrKind  ParsingErrorKind
		stmts            []Node
		valuelessTokens  = []Token{{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{start, start + 1}}}
	)

	p.eatSpaceNewlineCommaComment(&valuelessTokens)
	manifest := p.parseManifestIfPresent()

	p.eatSpaceNewlineSemicolonComment(&valuelessTokens)

	for p.i < p.len && p.s[p.i] != '}' {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {
			stmts = append(stmts, &UnknownNode{
				NodeBase: NodeBase{
					Span:            NodeSpan{p.i, p.i + 1},
					Err:             &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i])},
					ValuelessTokens: []Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
			continue
		}

		var stmtErr *ParsingError
		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		stmt := p.parseStatement()
		prevStmtEndIndex = p.i
		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.kind
		}

		if _, isMissingExpr := stmt.(*MissingExpression); isMissingExpr {
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
		p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
	}

	var embeddedModuleErr *ParsingError

	if p.i >= p.len || p.s[p.i] != '}' {
		embeddedModuleErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_EMBEDDED_MODULE}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	emod.Manifest = manifest
	emod.Statements = stmts
	emod.NodeBase = NodeBase{
		NodeSpan{start, p.i},
		embeddedModuleErr,
		valuelessTokens,
	}

	return emod
}

func (p *parser) parseSpawnExpression(goIdent Node) Node {
	p.panicIfContextDone()

	spawnExprStart := goIdent.Base().Span.Start
	tokens := []Token{{Type: GO_KEYWORD, Span: goIdent.Base().Span}}

	p.eatSpace()
	if p.i >= p.len {
		return &SpawnExpression{
			NodeBase: NodeBase{
				NodeSpan{spawnExprStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_GO_KEYWORD},
				tokens,
			},
		}
	}

	meta, _ := p.parseExpression()
	var e Node
	p.eatSpace()

	if ident, ok := meta.(*IdentifierLiteral); ok && ident.Name == "do" {
		tokens = append(tokens, Token{Type: DO_KEYWORD, Span: ident.Span})
		meta = nil
		goto parse_embedded_module
	}

	e, _ = p.parseExpression()
	p.eatSpace()

	if ident, ok := e.(*IdentifierLiteral); ok && ident.Name == "do" {
		tokens = append(tokens, Token{Type: DO_KEYWORD, Span: ident.Span})
	} else {
		return &SpawnExpression{
			NodeBase: NodeBase{
				NodeSpan{spawnExprStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_DO_KEYWORD_AFTER_META},
				tokens,
			},
			Meta: meta,
		}
	}

parse_embedded_module:
	p.eatSpace()

	var emod *EmbeddedModule

	if p.i >= p.len {
		return &SpawnExpression{
			NodeBase: NodeBase{
				NodeSpan{spawnExprStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_DO_KEYWORD},
				tokens,
			},
			Meta: meta,
		}
	}

	if p.s[p.i] == '{' {
		emod = p.parseEmbeddedModule()
	} else {
		expr, _ := p.parseExpression()

		var embeddedModuleErr *ParsingError

		if call, ok := expr.(*CallExpression); !ok {
			embeddedModuleErr = &ParsingError{UnspecifiedParsingError, SPAWN_EXPR_ONLY_SIMPLE_CALLS_ARE_SUPPORTED}
		} else if _, ok := call.Callee.(*IdentifierLiteral); !ok {
			embeddedModuleErr = &ParsingError{UnspecifiedParsingError, SPAWN_EXPR_ONLY_SIMPLE_CALLS_ARE_SUPPORTED}
		}

		emod = &EmbeddedModule{}
		emod.NodeBase.Span = expr.Base().Span
		emod.Err = embeddedModuleErr
		emod.Statements = []Node{expr}
		emod.SingleCallExpr = true
	}

	return &SpawnExpression{
		NodeBase: NodeBase{Span: NodeSpan{spawnExprStart, p.i}, ValuelessTokens: tokens},
		Meta:     meta,
		Module:   emod,
	}
}

func (p *parser) parseMappingExpression(mappingIdent Node) *MappingExpression {
	p.panicIfContextDone()

	start := mappingIdent.Base().Span.Start
	p.eatSpace()

	var valuelessTokens = []Token{{Type: MAPPING_KEYWORD, Span: mappingIdent.Base().Span}}

	if p.i >= p.len || p.s[p.i] != '{' {
		return &MappingExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_BODY},
			},
		}
	}

	valuelessTokens = append(valuelessTokens, Token{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	p.eatSpaceNewlineComment(&valuelessTokens)
	var entries []Node

	for p.i < p.len && p.s[p.i] != '}' {
		key, isMissingExpr := p.parseExpression()
		p.eatSpace()

		if p.i < p.len && isMissingExpr {
			key = &UnknownNode{
				NodeBase: NodeBase{
					NodeSpan{p.i, p.i + 1},
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInMappingExpression(p.s[p.i])},
					[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
				},
			}
			p.i++
		}

		dynamicEntryVar, isDynamicEntry := key.(*IdentifierLiteral)
		var entryParsingErr *ParsingError
		if isDynamicEntry && isKeyword(dynamicEntryVar.Name) {
			entryParsingErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
		}

		if p.i >= p.len {
			if isDynamicEntry {
				entries = append(entries, &DynamicMappingEntry{
					NodeBase: NodeBase{
						Span: dynamicEntryVar.Base().Span,
						Err:  entryParsingErr,
					},
					KeyVar: dynamicEntryVar,
				})
			} else {
				entries = append(entries, &StaticMappingEntry{
					NodeBase: NodeBase{
						Span: key.Base().Span,
						Err:  entryParsingErr,
					},
					Key: key,
				})
			}

			return &MappingExpression{
				NodeBase: NodeBase{
					Span:            NodeSpan{start, p.i},
					ValuelessTokens: valuelessTokens,
					Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_ENTRY},
				},
				Entries: entries,
			}
		}

		var (
			value                 Node
			groupMatchingVariable Node
			entryTokens           []Token
		)

		if isDynamicEntry {
			key, isMissingExpr = p.parseExpression()

			if p.i < p.len && isMissingExpr {
				key = &UnknownNode{
					NodeBase: NodeBase{
						NodeSpan{p.i, p.i + 1},
						&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInMappingExpression(p.s[p.i])},
						[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
					},
				}
				p.i++
			}

			p.eatSpace()

			if p.i < p.len && (isAlpha(p.s[p.i]) || p.s[p.i] == '_') {
				groupMatchingVariable = p.parseIdentStartingExpression()
				ident, ok := groupMatchingVariable.(*IdentifierLiteral)

				if !ok && groupMatchingVariable.Base().Err == nil {
					groupMatchingVariable.BasePtr().Err = &ParsingError{UnspecifiedParsingError, INVALID_DYNAMIC_MAPPING_ENTRY_GROUP_MATCHING_VAR_EXPECTED}
				}

				if ok && isKeyword(ident.Name) {
					entryParsingErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
				}
			}
		}

		end := p.i
		p.eatSpace()

		if p.i < p.len-1 && p.s[p.i] == '=' && p.s[p.i+1] == '>' {
			entryTokens = append(entryTokens, Token{Type: ARROW, Span: NodeSpan{p.i, p.i + 2}})
			p.i += 2
			p.eatSpace()

			value, _ = p.parseExpression()
		}

		if value != nil {
			end = value.Base().Span.End
		} else {
			entryParsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_ENTRY_MISSING_ARROW_VALUE}
		}

		if !isDynamicEntry {
			entries = append(entries, &StaticMappingEntry{
				NodeBase: NodeBase{
					Span:            NodeSpan{key.Base().Span.Start, end},
					ValuelessTokens: entryTokens,
					Err:             entryParsingErr,
				},
				Key:   key,
				Value: value,
			})
		} else {
			entries = append(entries, &DynamicMappingEntry{
				NodeBase: NodeBase{
					Span:            NodeSpan{dynamicEntryVar.Base().Span.Start, end},
					ValuelessTokens: entryTokens,
					Err:             entryParsingErr,
				},
				Key:                   key,
				KeyVar:                dynamicEntryVar,
				GroupMatchingVariable: groupMatchingVariable,
				ValueComputation:      value,
			})
		}

		p.eatSpaceNewlineComment(&valuelessTokens)
	}

	var parsingErr *ParsingError
	if p.i >= p.len || p.s[p.i] != '}' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &MappingExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			ValuelessTokens: valuelessTokens,
			Err:             parsingErr,
		},
		Entries: entries,
	}
}

func (p *parser) parseComputeExpression(compIdent Node) *ComputeExpression {
	p.panicIfContextDone()

	start := compIdent.Base().Span.Start
	p.eatSpace()

	arg, _ := p.parseExpression()

	return &ComputeExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			ValuelessTokens: []Token{{Type: COMP_KEYWORD, Span: compIdent.Base().Span}},
		},
		Arg: arg,
	}
}

func (p *parser) parseUdataLiteral(udataIdent Node) *UDataLiteral {
	start := udataIdent.Base().Span.Start
	var valuelessTokens = []Token{{Type: UDATA_KEYWORD, Span: udataIdent.Base().Span}}

	p.eatSpace()

	root, _ := p.parseExpression()
	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '{' {
		return &UDataLiteral{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
			},
			Root: root,
		}
	}

	valuelessTokens = append(valuelessTokens, Token{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

	p.i++
	p.eatSpaceNewlineCommaComment(&valuelessTokens)
	var children []*UDataEntry

	for p.i < p.len && p.s[p.i] != '}' { //
		entry, cont := p.parseTreeStructureEntry()
		children = append(children, entry)

		if !cont {
			return &UDataLiteral{
				NodeBase: NodeBase{
					Span:            NodeSpan{start, p.i},
					ValuelessTokens: valuelessTokens,
				},
				Root:     root,
				Children: children,
			}
		}

		p.eatSpaceNewlineCommaComment(&valuelessTokens)
	}

	var parsingErr *ParsingError
	if p.i >= p.len || p.s[p.i] != '}' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_UDATA_LIT_MISSING_CLOSING_BRACE}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	return &UDataLiteral{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			Err:             parsingErr,
			ValuelessTokens: valuelessTokens,
		},
		Root:     root,
		Children: children,
	}
}

func (p *parser) parseTreeStructureEntry() (entry *UDataEntry, cont bool) {
	p.panicIfContextDone()

	start := p.i

	node, isMissingExpr := p.parseExpression()
	p.eatSpace()

	if p.i < p.len && isMissingExpr {
		node = &UnknownNode{
			NodeBase: NodeBase{
				node.Base().Span,
				&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInUdataLiteral(p.s[p.i])},
				[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
			},
		}
		p.i++
		return &UDataEntry{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Value: node,
		}, true
	}

	if p.i >= p.len {
		return &UDataEntry{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, UNTERMINATED_UDATA_ENTRY},
			},
			Value: node,
		}, false
	}

	if p.s[p.i] != '{' { //leaf
		return &UDataEntry{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
			},
			Value: node,
		}, true
	}

	p.i++
	var valuelessTokens []Token = []Token{{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i - 1, p.i}}}
	var children []*UDataEntry

	p.eatSpaceNewlineComment(&valuelessTokens)

	for p.i < p.len && p.s[p.i] != '}' { //
		entry, cont := p.parseTreeStructureEntry()
		children = append(children, entry)

		if !cont {
			return &UDataEntry{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
				},
				Value:    node,
				Children: children,
			}, false
		}

		p.eatSpaceNewlineCommaComment(&valuelessTokens)
	}

	var parsingErr *ParsingError
	if p.i >= p.len || p.s[p.i] != '}' {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_UDATA_ENTRY_MISSING_CLOSING_BRACE}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}
	return &UDataEntry{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			Err:             parsingErr,
			ValuelessTokens: valuelessTokens,
		},
		Value:    node,
		Children: children,
	}, true
}

func (p *parser) parseConcatenationExpression(concatIdent Node, precededByOpeningParen bool) *ConcatenationExpression {
	p.panicIfContextDone()

	start := concatIdent.Base().Span.Start
	var valuelessTokens = []Token{{Type: CONCAT_KEYWORD, Span: concatIdent.Base().Span}}
	var elements []Node

	p.eatSpace()

	for p.i < p.len && !isUnpairedOrIsClosingDelim(p.s[p.i]) {

		var elem Node

		//spread element
		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			spreadStart := p.i
			threeDotsSpan := NodeSpan{p.i, p.i + 3}
			p.i += 3

			e, _ := p.parseExpression()

			elem = &ElementSpreadElement{
				NodeBase: NodeBase{
					Span:            NodeSpan{spreadStart, e.Base().Span.End},
					ValuelessTokens: []Token{{Type: THREE_DOTS, Span: threeDotsSpan}},
				},
				Expr: e,
			}

		} else {
			elem, _ = p.parseExpression()
		}

		elements = append(elements, elem)
		if precededByOpeningParen {
			p.eatSpaceNewlineComment(&valuelessTokens)
		} else {
			p.eatSpace()
		}
	}

	var parsingErr *ParsingError
	if len32(elements) == 0 {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_CONCAT_EXPR_ELEMS_EXPECTED}
	}

	return &ConcatenationExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			Err:             parsingErr,
			ValuelessTokens: valuelessTokens,
		},
		Elements: elements,
	}
}

func (p *parser) parseTestSuiteExpression(ident *IdentifierLiteral) *TestSuiteExpression {
	p.panicIfContextDone()

	start := ident.Base().Span.Start
	var valuelessTokens = []Token{{Type: TESTSUITE_KEYWORD, Span: ident.Base().Span}}

	p.eatSpace()
	if p.i >= p.len {
		return &TestSuiteExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
			},
		}
	}

	var meta Node

	if p.s[p.i] != '{' {
		meta, _ = p.parseExpression()
		p.eatSpace()
	}

	if p.i >= p.len || p.s[p.i] != '{' {
		return &TestSuiteExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
			},
			Meta: meta,
		}
	}

	emod := p.parseEmbeddedModule()

	return &TestSuiteExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			ValuelessTokens: valuelessTokens,
		},
		Meta:   meta,
		Module: emod,
	}

}

func (p *parser) parseTestCaseExpression(ident *IdentifierLiteral) *TestCaseExpression {
	p.panicIfContextDone()

	start := ident.Base().Span.Start
	var valuelessTokens = []Token{{Type: TESTCASE_KEYWORD, Span: ident.Base().Span}}

	p.eatSpace()
	if p.i >= p.len {
		return &TestCaseExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{MissingBlock, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
			},
		}
	}

	var meta Node

	if p.s[p.i] != '{' {
		meta, _ = p.parseExpression()
		p.eatSpace()
	}

	if p.i >= p.len || p.s[p.i] != '{' {
		return &TestCaseExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
			},
			Meta: meta,
		}
	}

	emod := p.parseEmbeddedModule()

	return &TestCaseExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			ValuelessTokens: valuelessTokens,
		},
		Meta:   meta,
		Module: emod,
	}
}

func (p *parser) parseLifetimeJobExpression(ident *IdentifierLiteral) *LifetimejobExpression {
	p.panicIfContextDone()

	start := ident.Base().Span.Start
	var valuelessTokens = []Token{{Type: LIFETIMEJOB_KEYWORD, Span: ident.Base().Span}}

	p.eatSpace()
	if p.i >= p.len {
		return &LifetimejobExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_META},
			},
		}
	}

	meta, _ := p.parseExpression()
	p.eatSpace()

	var subject Node

	if p.i < p.len && p.s[p.i] == 'f' { //TODO: rework
		e := p.parseIdentStartingExpression()
		if ident, ok := e.(*IdentifierLiteral); ok && ident.Name == "for" {
			valuelessTokens = append(valuelessTokens, Token{Type: FOR_KEYWORD, Span: ident.Span})

			p.eatSpace()
			subject, _ = p.parseExpression()
			p.eatSpace()
		}
	}

	if p.i >= p.len || p.s[p.i] != '{' {
		return &LifetimejobExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_EMBEDDED_MODULE},
			},
			Meta:    meta,
			Subject: subject,
		}
	}

	emod := p.parseEmbeddedModule()

	return &LifetimejobExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			ValuelessTokens: valuelessTokens,
		},
		Meta:    meta,
		Subject: subject,
		Module:  emod,
	}
}

func (p *parser) parseReceptionHandlerExpression(onIdent Node) Node {
	p.panicIfContextDone()

	exprStart := onIdent.Base().Span.Start
	tokens := []Token{{Type: ON_KEYWORD, Span: onIdent.Base().Span}}

	p.eatSpace()
	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		return &ReceptionHandlerExpression{
			NodeBase: NodeBase{
				NodeSpan{exprStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_RECEIVED_KEYWORD},
				tokens,
			},
		}
	}

	e, _ := p.parseExpression()
	p.eatSpace()

	var missingReceivedKeywordError *ParsingError

	if ident, ok := e.(*IdentifierLiteral); ok && ident.Name == "received" {
		tokens = append(tokens, Token{Type: RECEIVED_KEYWORD, Span: ident.Span})
		e = nil
	} else {
		missingReceivedKeywordError = &ParsingError{UnspecifiedParsingError, INVALID_RECEP_HANDLER_MISSING_RECEIVED_KEYWORD}
	}

	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		return &ReceptionHandlerExpression{
			NodeBase: NodeBase{
				NodeSpan{exprStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_PATTERN},
				tokens,
			},
		}
	}

	pattern, _ := p.parseExpression()
	p.eatSpace()

	if p.i >= p.len || isUnpairedOrIsClosingDelim(p.s[p.i]) {
		return &ReceptionHandlerExpression{
			NodeBase: NodeBase{
				NodeSpan{exprStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_HANDLER_OR_PATTERN},
				tokens,
			},
			Pattern: pattern,
		}
	}

	handler, _ := p.parseExpression()
	p.eatSpace()

	return &ReceptionHandlerExpression{
		NodeBase: NodeBase{Span: NodeSpan{exprStart, p.i}, ValuelessTokens: tokens, Err: missingReceivedKeywordError},
		Pattern:  pattern,
		Handler:  handler,
	}
}

func (p *parser) parseSendValueExpression(ident *IdentifierLiteral) *SendValueExpression {
	p.panicIfContextDone()

	start := ident.Base().Span.Start
	var valuelessTokens = []Token{{Type: SENDVAL_KEYWORD, Span: ident.Base().Span}}

	p.eatSpace()
	if p.isExpressionEnd() {
		return &SendValueExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: valuelessTokens,
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_SENDVALUE_EXPRESSION_MISSING_VALUE},
			},
		}
	}

	value, _ := p.parseExpression()
	p.eatSpace()

	e, _ := p.parseExpression()
	p.eatSpace()

	var receiver Node
	var parsingErr *ParsingError

	if ident, ok := e.(*IdentifierLiteral); !ok || ident.Name != "to" {
		receiver = e
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_SENDVALUE_EXPRESSION_MISSING_TO_KEYWORD_BEFORE_RECEIVER}
	} else {
		valuelessTokens = append(valuelessTokens, Token{Type: TO_KEYWORD, Span: ident.Span})

		receiver, _ = p.parseExpression()
		p.eatSpace()
	}

	return &SendValueExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			ValuelessTokens: valuelessTokens,
			Err:             parsingErr,
		},
		Value:    value,
		Receiver: receiver,
	}
}

func (p *parser) parseXMLExpression(namespaceIdent *IdentifierLiteral) *XMLExpression {
	p.panicIfContextDone()

	start := namespaceIdent.Span.Start

	//we do not increment because we keep the '<' for parsing the top element

	if p.i+1 >= p.len || !isAlpha(p.s[p.i+1]) {
		tokens := []Token{{Type: LESS_THAN, Span: NodeSpan{p.i, p.i + 1}}}

		return &XMLExpression{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_XML_EXPRESSION_MISSING_TOP_ELEM_NAME},
				tokens,
			},
		}
	}

	topElem := p.parseXMLElement(p.i)
	return &XMLExpression{
		NodeBase:  NodeBase{Span: NodeSpan{start, p.i}},
		Namespace: namespaceIdent,
		Element:   topElem,
	}
}

func (p *parser) parseXMLElement(start int32) *XMLElement {
	p.panicIfContextDone()

	var parsingErr *ParsingError
	openingElemValuelessTokens := []Token{{Type: LESS_THAN, Span: NodeSpan{start, start + 1}}}
	p.i++
	openingName := p.parseIdentStartingExpression()

	openingIdent, ok := openingName.(*IdentifierLiteral)
	if !ok {
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	}

	p.eatSpace()

	openingElement := &XMLOpeningElement{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, p.i},
			ValuelessTokens: openingElemValuelessTokens,
		},
		Name: openingIdent,
	}

	//attributes
	for p.i < p.len && p.s[p.i] != '>' {
		name, isMissingExpr := p.parseExpression()

		if isMissingExpr {
			openingElement.Attributes = append(openingElement.Attributes, &XMLAttribute{
				NodeBase: NodeBase{
					Span: name.Base().Span,
				},
				Name: name,
			})
			break
		}

		switch name.(type) {
		case *IdentifierLiteral:
		default:
			if name.Base().Err == nil {
				name.BasePtr().Err = &ParsingError{UnspecifiedParsingError, XML_ATTRIBUTE_NAME_SHOULD_BE_IDENT}
			}
		}

		if p.i < p.len && p.s[p.i] == '=' {
			token := Token{Type: EQUAL, Span: NodeSpan{p.i, p.i + 1}}
			p.i++

			value, isMissingExpr := p.parseExpression()

			openingElement.Attributes = append(openingElement.Attributes, &XMLAttribute{
				NodeBase: NodeBase{
					Span:            NodeSpan{name.Base().Span.Start, p.i},
					ValuelessTokens: []Token{token},
				},
				Name:  name,
				Value: value,
			})

			if isMissingExpr {
				break
			}
		} else {

			openingElement.Attributes = append(openingElement.Attributes, &XMLAttribute{
				NodeBase: NodeBase{
					Span: NodeSpan{name.Base().Span.Start, p.i},
				},
				Name: name,
			})
		}

		p.eatSpace()
	}

	if p.i >= p.len || p.s[p.i] != '>' {
		openingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING}

		return &XMLElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				openingElemValuelessTokens,
			},
			Opening: openingElement,
		}
	}

	openingElemValuelessTokens = append(openingElemValuelessTokens, Token{Type: GREATER_THAN, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	openingElement.Span.End = p.i
	openingElement.ValuelessTokens = openingElemValuelessTokens

	//children
	var valuelessTokens []Token

	children, err := p.parseXMLChildren(&valuelessTokens)
	parsingErr = err

	if p.i >= p.len || p.s[p.i] != '<' {
		return &XMLElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_XML_ELEMENT_MISSING_CLOSING_TAG},
				valuelessTokens,
			},
			Opening:  openingElement,
			Children: children,
		}
	}

	//closing element
	closingElemStart := p.i
	closingElemValuelessTokens := []Token{{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{p.i, p.i + 2}}}
	p.i += 2

	closingName, _ := p.parseExpression()

	closingElement := &XMLClosingElement{
		NodeBase: NodeBase{
			Span:            NodeSpan{closingElemStart, p.i},
			ValuelessTokens: closingElemValuelessTokens,
		},
		Name: closingName,
	}

	if closing, ok := closingName.(*IdentifierLiteral); !ok {
		closingElement.Err = &ParsingError{UnspecifiedParsingError, INVALID_TAG_NAME}
	} else if openingIdent != nil && closing.Name != openingIdent.Name {
		closingElement.Err = &ParsingError{UnspecifiedParsingError, fmtExpectedClosingTag(openingIdent.Name)}
	}

	if p.i >= p.len || p.s[p.i] != '>' {
		if closingElement.Err == nil {
			closingElement.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_CLOSING_XML_TAG_MISSING_CLOSING_DELIM}
		}

		return &XMLElement{
			NodeBase: NodeBase{
				NodeSpan{start, p.i},
				parsingErr,
				openingElemValuelessTokens,
			},
			Opening:  openingElement,
			Closing:  closingElement,
			Children: children,
		}
	}

	closingElemValuelessTokens = append(closingElemValuelessTokens, Token{Type: GREATER_THAN, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	closingElement.Span.End = p.i
	closingElement.ValuelessTokens = closingElemValuelessTokens

	return &XMLElement{
		NodeBase: NodeBase{
			NodeSpan{start, p.i},
			parsingErr,
			valuelessTokens,
		},
		Opening:  openingElement,
		Closing:  closingElement,
		Children: children,
	}
}

func (p *parser) parseXMLChildren(valuelessTokens *[]Token) ([]Node, *ParsingError) {
	p.panicIfContextDone()

	inInterpolation := false
	interpolationStart := int32(-1)
	children := make([]Node, 0)
	childStart := p.i

	var parsingErr *ParsingError

	for p.i < p.len && (inInterpolation || (p.s[p.i] != '<' || (p.i < p.len-1 && p.s[p.i+1] != '/'))) {

		//interpolation
		switch {
		case p.s[p.i] == '{':
			*valuelessTokens = append(*valuelessTokens, Token{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})

			// add previous slice
			raw := string(p.s[childStart:p.i])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			children = append(children, &XMLText{
				NodeBase: NodeBase{
					NodeSpan{childStart, p.i},
					sliceErr,
					nil,
				},
				Raw:   raw,
				Value: value,
			})

			inInterpolation = true
			p.i++
			interpolationStart = p.i
		case inInterpolation && p.s[p.i] == '}': //end of interpolation
			*valuelessTokens = append(*valuelessTokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			interpolationExclEnd := p.i
			inInterpolation = false
			p.i++
			childStart = p.i

			var interpParsingErr *ParsingError
			var expr Node

			interpolation := p.s[interpolationStart:interpolationExclEnd]

			if strings.TrimSpace(string(interpolation)) == "" {
				interpParsingErr = &ParsingError{UnspecifiedParsingError, EMPTY_XML_INTERP}
			} else {
				e, ok := ParseExpression(string(interpolation))
				if !ok {
					interpParsingErr = &ParsingError{UnspecifiedParsingError, INVALID_XML_INTERP}
				} else {
					expr = e
					shiftNodeSpans(expr, interpolationStart)
				}
			}

			var interpTokens []Token

			interpolationNode := &XMLInterpolation{
				NodeBase: NodeBase{
					NodeSpan{interpolationStart, interpolationExclEnd},
					interpParsingErr,
					interpTokens,
				},
				Expr: expr,
			}
			children = append(children, interpolationNode)
		case !inInterpolation && p.s[p.i] == '<': //child element

			// add previous slice
			raw := string(p.s[childStart:p.i])
			value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)
			children = append(children, &XMLText{
				NodeBase: NodeBase{
					NodeSpan{childStart, p.i},
					sliceErr,
					nil,
				},
				Raw:   raw,
				Value: value,
			})

			child := p.parseXMLElement(p.i)
			children = append(children, child)
			childStart = p.i
		default:
			p.i++
		}
	}

	if inInterpolation {
		raw := string(p.s[interpolationStart:p.i])
		value, _ := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		children = append(children, &XMLText{
			NodeBase: NodeBase{
				NodeSpan{interpolationStart, p.i},
				&ParsingError{UnspecifiedParsingError, UNTERMINATED_XML_INTERP},
				nil,
			},
			Raw:   raw,
			Value: value,
		})
	} else {
		raw := string(p.s[childStart:p.i])
		value, sliceErr := p.getValueOfMultilineStringSliceOrLiteral([]byte(raw), false)

		children = append(children, &XMLText{
			NodeBase: NodeBase{
				NodeSpan{childStart, p.i},
				sliceErr,
				nil,
			},
			Raw:   raw,
			Value: value,
		})
	}

	return children, parsingErr
}

// tryParseCall tries to parse a call or return nil (calls with parsing errors are returned)
func (p *parser) tryParseCall(callee Node, firstName string) *CallExpression {
	p.panicIfContextDone()

	switch {
	case p.s[p.i] == '"': //func_name"string"
		call := &CallExpression{
			NodeBase: NodeBase{
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
		call := &CallExpression{
			NodeBase: NodeBase{
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
	case !isKeyword(firstName) && (p.s[p.i] == '(' || (p.s[p.i] == '!' && p.i < p.len-1 && p.s[p.i+1] == '(')): //func_name(...

		var tokens []Token

		must := false
		if p.s[p.i] == '!' {
			must = true
			p.i++
			tokens = append(tokens,
				Token{Type: EXCLAMATION_MARK, Span: NodeSpan{p.i - 1, p.i}},
				Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}},
			)
		} else {
			tokens = append(tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		}

		p.i++
		p.eatSpace()

		call := &CallExpression{
			NodeBase: NodeBase{
				NodeSpan{callee.Base().Span.Start, 0},
				nil,
				tokens,
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
func (p *parser) parseFunction(start int32) Node {
	p.panicIfContextDone()

	tokens := []Token{{Type: FN_KEYWORD, Span: NodeSpan{p.i - 2, p.i}}}
	p.eatSpace()

	var (
		ident                  *IdentifierLiteral
		parsingErr             *ParsingError
		additionalInvalidNodes []Node
		capturedLocals         []Node
		hasCaptureList         = false
	)

	createNodeWithError := func() Node {
		fn := FunctionExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: tokens,
			},
			CaptureList: capturedLocals,
		}

		if ident != nil {
			if parsingErr == nil && isKeyword(ident.Name) {
				parsingErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_FN_NAMES}
			}
			return &FunctionDeclaration{
				NodeBase: NodeBase{
					Span:            fn.Span,
					Err:             parsingErr,
					ValuelessTokens: tokens,
				},
				Function: &fn,
				Name:     ident,
			}
		}
		fn.Err = parsingErr
		return &fn
	}

	//parse capture list
	if p.i < p.len && p.s[p.i] == '[' {
		hasCaptureList = true
		tokens = append(tokens, Token{Type: OPENING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		for p.i < p.len && p.s[p.i] != ']' {
			e, isMissingExpr := p.parseExpression()

			if isMissingExpr && p.i >= p.len {
				break
			}

			if isMissingExpr {
				e = &UnknownNode{
					NodeBase: NodeBase{
						e.Base().Span,
						&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCaptureList(p.s[p.i])},
						[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
					},
				}
				p.i++
			} else {
				if _, ok := e.(*IdentifierLiteral); !ok && e.Base().Err == nil {
					e.BasePtr().Err = &ParsingError{UnspecifiedParsingError, CAPTURE_LIST_SHOULD_ONLY_CONTAIN_IDENTIFIERS}
				}
			}

			capturedLocals = append(capturedLocals, e)
			p.eatSpaceComma(&tokens)
		}

		if p.i >= p.len {
			parsingErr = &ParsingError{InvalidNext, UNTERMINATED_CAPTURE_LIST_MISSING_CLOSING_BRACKET}
			return createNodeWithError()
		} else {
			tokens = append(tokens, Token{Type: CLOSING_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
			p.i++
		}

		p.eatSpace()
	}

	if p.i < p.len && isAlpha(p.s[p.i]) {
		identLike := p.parseIdentStartingExpression()
		var ok bool
		if ident, ok = identLike.(*IdentifierLiteral); !ok {
			return &FunctionDeclaration{
				NodeBase: NodeBase{
					Span:            NodeSpan{start, p.i},
					Err:             &ParsingError{UnspecifiedParsingError, fmtFuncNameShouldBeAnIndentNot(identLike)},
					ValuelessTokens: tokens,
				},
				Function: nil,
				Name:     nil,
			}
		}
	}

	if p.i >= p.len || p.s[p.i] != '(' {
		if hasCaptureList && ident == nil {
			parsingErr = &ParsingError{InvalidNext, CAPTURE_LIST_SHOULD_BE_FOLLOWED_BY_PARAMS}
		} else {
			parsingErr = &ParsingError{InvalidNext, FN_KEYWORD_OR_FUNC_NAME_SHOULD_BE_FOLLOWED_BY_PARAMS}
		}

		return createNodeWithError()
	}

	tokens = append(tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	var parameters []*FunctionParameter
	isVariadic := false

	//we parse the parameters
	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewlineComma(&tokens)
		var paramErr *ParsingError

		if p.i >= p.len || p.s[p.i] == ')' {
			break
		}

		if isVariadic {
			paramErr = &ParsingError{UnspecifiedParsingError, VARIADIC_PARAM_IS_UNIQUE_AND_SHOULD_BE_LAST_PARAM}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			isVariadic = true
			p.i += 3
		}

		varNode, isMissingExpr := p.parseExpression()
		var typ Node

		if isMissingExpr {
			r := p.s[p.i]
			p.i++

			additionalInvalidNodes = append(additionalInvalidNodes, &UnknownNode{
				NodeBase: NodeBase{
					NodeSpan{p.i - 1, p.i},
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(r)},
					[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i - 1, p.i}, Raw: string(r)}},
				},
			})

		} else {
			p.eatSpace()

			{
				prev := p.inPattern
				p.inPattern = true

				typ, isMissingExpr = p.parseExpression()

				p.inPattern = prev
			}

			if isMissingExpr {
				typ = nil
			}

			if ident, ok := varNode.(*IdentifierLiteral); ok {
				span := varNode.Base().Span
				if typ != nil {
					span.End = typ.Base().Span.End
				}

				if paramErr == nil && isKeyword(ident.Name) {
					paramErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES}
				}

				parameters = append(parameters, &FunctionParameter{
					NodeBase: NodeBase{
						span,
						paramErr,
						nil,
					},
					Var:        varNode.(*IdentifierLiteral),
					Type:       typ,
					IsVariadic: isVariadic,
				})
			} else {
				varNode.BasePtr().Err = &ParsingError{UnspecifiedParsingError, PARAM_LIST_OF_FUNC_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS}
				additionalInvalidNodes = append(additionalInvalidNodes, varNode)

				if typ != nil {
					typ.BasePtr().Err = &ParsingError{UnspecifiedParsingError, PARAM_LIST_OF_FUNC_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS}
					additionalInvalidNodes = append(additionalInvalidNodes, typ)
				}
			}
		}

		p.eatSpaceNewlineComma(&tokens)
	}

	var (
		returnType       Node
		body             Node
		isBodyExpression bool
		end              int32
	)

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN}
		end = p.i
	} else if p.s[p.i] != ')' {
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_FUNC_SYNTAX}
		end = p.i
	} else {
		tokens = append(tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		isAcceptedFirstTypeChar := func(r rune) bool {
			return r == '%' || isFirstIdentChar(r) || r == '(' || r == '['
		}

		if p.i < p.len && isAcceptedFirstTypeChar(p.s[p.i]) {
			prev := p.inPattern
			p.inPattern = true

			returnType, _ = p.parseExpression()

			p.inPattern = prev
		}

		p.eatSpace()
		if p.i >= p.len {
			parsingErr = &ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW}
			end = p.i
		} else {
			switch p.s[p.i] {
			case '{':
				body = p.parseBlock()
				end = body.Base().Span.End
			case '=':
				if p.i >= p.len+1 || p.s[p.i+1] != '>' {
					parsingErr = &ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW}
					end = p.i
				} else {
					tokens = append(tokens, Token{Type: ARROW, Span: NodeSpan{p.i, p.i + 2}})
					p.i += 2
					p.eatSpace()
					body, _ = p.parseExpression()
					end = body.Base().Span.End
					isBodyExpression = true
				}
			default:
				parsingErr = &ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW}
				end = p.i
			}
		}

	}

	fn := FunctionExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, end},
			Err:             parsingErr,
			ValuelessTokens: tokens,
		},
		CaptureList:            capturedLocals,
		Parameters:             parameters,
		AdditionalInvalidNodes: additionalInvalidNodes,
		ReturnType:             returnType,
		IsVariadic:             isVariadic,
		Body:                   body,
		IsBodyExpression:       isBodyExpression,
	}

	if ident != nil {
		fn.Err = nil
		fn.ValuelessTokens = nil

		if parsingErr == nil && isKeyword(ident.Name) {
			parsingErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_FN_NAMES}
		}

		return &FunctionDeclaration{
			NodeBase: NodeBase{
				Span:            fn.Span,
				Err:             parsingErr,
				ValuelessTokens: tokens,
			},
			Function: &fn,
			Name:     ident,
		}
	}

	return &fn
}

// parseFunction parses function declarations and function expressions
func (p *parser) parseFunctionPattern(start int32) Node {
	p.panicIfContextDone()

	tokens := []Token{{Type: PERCENT_FN, Span: NodeSpan{p.i - 3, p.i}}}
	p.eatSpace()

	var (
		parsingErr             *ParsingError
		additionalInvalidNodes []Node
		capturedLocals         []Node
	)

	createNodeWithError := func() Node {
		fn := FunctionExpression{
			NodeBase: NodeBase{
				Span:            NodeSpan{start, p.i},
				ValuelessTokens: tokens,
			},
			CaptureList: capturedLocals,
		}

		fn.Err = parsingErr
		return &fn
	}

	if p.i >= p.len || p.s[p.i] != '(' {
		parsingErr = &ParsingError{InvalidNext, PERCENT_FN_SHOULD_BE_FOLLOWED_BY_PARAMETERS}
		return createNodeWithError()
	}

	tokens = append(tokens, Token{Type: OPENING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

	var parameters []*FunctionParameter
	isVariadic := false

	//we parse the parameters
	for p.i < p.len && p.s[p.i] != ')' {
		p.eatSpaceNewlineComma(&tokens)
		var paramErr *ParsingError

		if p.i < p.len && p.s[p.i] == ')' {
			break
		}

		if isVariadic {
			paramErr = &ParsingError{UnspecifiedParsingError, VARIADIC_PARAM_IS_UNIQUE_AND_SHOULD_BE_LAST_PARAM}
		}

		if p.i < p.len-2 && p.s[p.i] == '.' && p.s[p.i+1] == '.' && p.s[p.i+2] == '.' {
			isVariadic = true
			p.i += 3
		}

		varNode, isMissingExpr := p.parseExpression()

		var typ Node
		if isMissingExpr {
			r := p.s[p.i]
			p.i++

			additionalInvalidNodes = append(additionalInvalidNodes, &UnknownNode{
				NodeBase: NodeBase{
					NodeSpan{p.i - 1, p.i},
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(r)},
					[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i - 1, p.i}, Raw: string(r)}},
				},
			})

		} else {

			switch varNode := varNode.(type) {
			case *IdentifierLiteral:
				p.eatSpace()

				if paramErr == nil && isKeyword(varNode.Name) {
					paramErr = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES}
				}

				typ, isMissingExpr = p.parseExpression()
				if isMissingExpr {
					typ = nil
				}

				span := varNode.Base().Span
				if typ != nil {
					span.End = typ.Base().Span.End
				}

				parameters = append(parameters, &FunctionParameter{
					NodeBase: NodeBase{
						span,
						paramErr,
						nil,
					},
					Var:        varNode,
					Type:       typ,
					IsVariadic: isVariadic,
				})
			case *PatternCallExpression, *PatternNamespaceMemberExpression, *PatternIdentifierLiteral,
				*ObjectPatternLiteral, *ListPatternLiteral, *ComplexStringPatternPiece, *RegularExpressionLiteral:

				typ = varNode

				parameters = append(parameters, &FunctionParameter{
					NodeBase: NodeBase{
						typ.Base().Span,
						paramErr,
						nil,
					},
					Type:       typ,
					IsVariadic: isVariadic,
				})

			default:
				varNode.BasePtr().Err = &ParsingError{UnspecifiedParsingError, PARAM_LIST_OF_FUNC_PATT_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS}
				additionalInvalidNodes = append(additionalInvalidNodes, varNode)

				if typ != nil {
					typ.BasePtr().Err = &ParsingError{UnspecifiedParsingError, PARAM_LIST_OF_FUNC_SHOULD_CONTAIN_PARAMETERS_SEP_BY_COMMAS}
					additionalInvalidNodes = append(additionalInvalidNodes, typ)
				}
			}

		}

		p.eatSpaceNewlineComma(&tokens)
	}

	var (
		returnType       Node
		body             Node
		isBodyExpression bool
		end              int32
	)

	if p.i >= p.len {
		parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN}
		end = p.i
	} else if p.s[p.i] != ')' {
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_FUNC_SYNTAX}
		end = p.i
	} else { //')'
		tokens = append(tokens, Token{Type: CLOSING_PARENTHESIS, Span: NodeSpan{p.i, p.i + 1}})
		p.i++

		p.eatSpace()

		if p.i < p.len && p.s[p.i] == '%' {
			returnType = p.parsePercentPrefixedPattern()
		}

		p.eatSpace()

		//optional body

		if p.i < p.len {
			switch p.s[p.i] {
			case '{':
				body = p.parseBlock()
				end = body.Base().Span.End
			case '=':
				if p.i >= p.len+1 || p.s[p.i+1] != '>' {
					parsingErr = &ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_PATT_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW}
					end = p.i
				} else {
					tokens = append(tokens, Token{Type: ARROW, Span: NodeSpan{p.i, p.i + 2}})
					p.i += 2
					p.eatSpace()
					body, _ = p.parseExpression()
					end = body.Base().Span.End
					isBodyExpression = true
				}
			default:
				if !isUnpairedOrIsClosingDelim(p.s[p.i]) {

					parsingErr = &ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_PATT_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW}
					end = p.i
				}
			}
		}
	}

	fn := FunctionPatternExpression{
		NodeBase: NodeBase{
			Span:            NodeSpan{start, end},
			Err:             parsingErr,
			ValuelessTokens: tokens,
		},
		Parameters:             parameters,
		AdditionalInvalidNodes: additionalInvalidNodes,
		ReturnType:             returnType,
		IsVariadic:             isVariadic,
		Body:                   body,
		IsBodyExpression:       isBodyExpression,
	}

	return &fn
}

func (p *parser) parseIfStatement(ifIdent *IdentifierLiteral) *IfStatement {
	p.panicIfContextDone()

	var alternate *Block
	var blk *Block
	var end int32
	var parsingErr *ParsingError

	tokens := []Token{
		{Type: IF_KEYWORD, Span: ifIdent.Span},
	}

	p.eatSpace()
	test, _ := p.parseExpression()
	p.eatSpace()

	if p.i >= p.len {
		end = p.i
		parsingErr = &ParsingError{MissingBlock, UNTERMINATED_IF_STMT_MISSING_BLOCK}
	} else if p.s[p.i] != '{' {
		end = p.i
		parsingErr = &ParsingError{MissingBlock, fmtUnterminatedIfStmtShouldBeFollowedByBlock(p.s[p.i])}
	} else {
		blk = p.parseBlock()
		end = blk.Span.End
		p.eatSpace()

		if p.i < p.len-3 && p.s[p.i] == 'e' && p.s[p.i+1] == 'l' && p.s[p.i+2] == 's' && p.s[p.i+3] == 'e' {
			tokens = append(tokens, Token{
				Type: ELSE_KEYWORD,
				Span: NodeSpan{p.i, p.i + 4},
			})
			p.i += 4
			p.eatSpace()

			if p.i >= p.len {
				parsingErr = &ParsingError{MissingBlock, UNTERMINATED_IF_STMT_MISSING_BLOCK_AFTER_ELSE}
			} else if p.s[p.i] != '{' {
				parsingErr = &ParsingError{MissingBlock, fmtUnterminatedIfStmtElseShouldBeFollowedByBlock(p.s[p.i])}
			} else {
				alternate = p.parseBlock()
				end = alternate.Span.End
			}
		}
	}

	return &IfStatement{
		NodeBase: NodeBase{
			Span:            NodeSpan{ifIdent.Span.Start, end},
			Err:             parsingErr,
			ValuelessTokens: tokens,
		},
		Test:       test,
		Consequent: blk,
		Alternate:  alternate,
	}
}

func (p *parser) parseForStatement(forIdent *IdentifierLiteral) *ForStatement {
	p.panicIfContextDone()

	var parsingErr *ParsingError
	var valuePattern Node
	var valueElemIdent *IdentifierLiteral
	var keyPattern Node
	var keyIndexIdent *IdentifierLiteral
	p.eatSpace()

	var firstPattern Node
	var first Node
	chunked := false
	tokens := []Token{{Type: FOR_KEYWORD, Span: forIdent.Span}}

	parseVariableLessForStatement := func(iteratedValue Node) *ForStatement {
		var blk *Block
		end := int32(0)

		if p.i >= p.len || p.s[p.i] != '{' {
			parsingErr = &ParsingError{MissingBlock, UNTERMINATED_FOR_STMT_MISSING_BLOCK}
			end = p.i
		} else {
			blk = p.parseBlock()
			end = p.i
		}

		return &ForStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{forIdent.Span.Start, end},
				Err:             parsingErr,
				ValuelessTokens: tokens,
			},
			KeyIndexIdent:  nil,
			ValueElemIdent: nil,
			Body:           blk,
			IteratedValue:  iteratedValue,
		}
	}

	if p.i < p.len && p.s[p.i] == '%' {
		firstPattern = p.parsePercentPrefixedPattern()
		p.eatSpace()

		if p.i < p.len && p.s[p.i] == '{' {
			return parseVariableLessForStatement(firstPattern)
		}
		e, _ := p.parseExpression()
		first = e
	} else {
		first, _ = p.parseExpression()

		if ident, ok := first.(*IdentifierLiteral); ok && !ident.IsParenthesized() && ident.Name == "chunked" {
			tokens = append(tokens, Token{Type: CHUNKED_KEYWORD, Span: ident.Span})
			chunked = true
			p.eatSpace()
			first, _ = p.parseExpression()
		}
	}

	switch v := first.(type) {
	case *IdentifierLiteral: //for ... in ...
		p.eatSpace()

		if p.i >= p.len {
			return &ForStatement{
				NodeBase: NodeBase{
					Span:            NodeSpan{forIdent.Span.Start, p.i},
					Err:             &ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT},
					ValuelessTokens: tokens,
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
				parsingErr = &ParsingError{UnspecifiedParsingError, fmtForStmtKeyIndexShouldBeFollowedByCommaNot(p.s[p.i])}
			}

			tokens = append(tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})

			p.i++
			p.eatSpace()

			if p.i >= p.len {
				return &ForStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{forIdent.Span.Start, p.i},
						Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_FOR_STMT},
						ValuelessTokens: tokens,
					},
					Chunked:       chunked,
					KeyPattern:    firstPattern,
					KeyIndexIdent: v,
				}
			}

			if p.s[p.i] == '%' {
				valuePattern = p.parsePercentPrefixedPattern()
				p.eatSpace()
			}

			e, _ := p.parseExpression()

			if ident, isVar := e.(*IdentifierLiteral); !isVar {
				parsingErr = &ParsingError{UnspecifiedParsingError, fmtInvalidForStmtKeyIndexVarShouldBeFollowedByVarNot(keyIndexIdent)}
			} else {
				valueElemIdent = ident
			}

			p.eatSpace()

			if p.i >= p.len {
				return &ForStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{forIdent.Span.Start, p.i},
						Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_FOR_STMT},
						ValuelessTokens: tokens,
					},
					KeyPattern:    firstPattern,
					KeyIndexIdent: v,
					ValuePattern:  valuePattern,
					Chunked:       chunked,
				}
			}

			if p.s[p.i] != 'i' || p.i > p.len-2 || p.s[p.i+1] != 'n' {
				return &ForStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{forIdent.Span.Start, p.i},
						Err:             &ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT_MISSING_IN_KEYWORD},
						ValuelessTokens: tokens,
					},
					KeyPattern:     keyPattern,
					KeyIndexIdent:  keyIndexIdent,
					ValuePattern:   valuePattern,
					ValueElemIdent: valueElemIdent,
					Chunked:        chunked,
				}
			}

		} else { //if directly followed by "in"
			valueElemIdent = v
			valuePattern = firstPattern
		}

		tokens = append(tokens, Token{Type: IN_KEYWORD, Span: NodeSpan{p.i, p.i + 2}})
		p.i += 2

		if p.i < p.len && p.s[p.i] != ' ' {

			return &ForStatement{
				NodeBase: NodeBase{
					Span:            NodeSpan{forIdent.Span.Start, p.i},
					Err:             &ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT_IN_KEYWORD_SHOULD_BE_FOLLOWED_BY_SPACE},
					ValuelessTokens: tokens,
				},
				KeyPattern:     keyPattern,
				KeyIndexIdent:  keyIndexIdent,
				ValuePattern:   valuePattern,
				ValueElemIdent: valueElemIdent,
				Chunked:        chunked,
			}
		}
		p.eatSpace()

		if p.i >= p.len {
			return &ForStatement{
				NodeBase: NodeBase{
					Span:            NodeSpan{forIdent.Span.Start, p.i},
					Err:             &ParsingError{UnspecifiedParsingError, INVALID_FOR_STMT_MISSING_VALUE_AFTER_IN},
					ValuelessTokens: tokens,
				},
				KeyPattern:     firstPattern,
				KeyIndexIdent:  keyIndexIdent,
				ValuePattern:   valuePattern,
				ValueElemIdent: valueElemIdent,
				Chunked:        chunked,
			}
		}

		iteratedValue, _ := p.parseExpression()
		p.eatSpace()

		var blk *Block
		var end = p.i

		if p.i >= p.len || p.s[p.i] != '{' {
			parsingErr = &ParsingError{MissingBlock, UNTERMINATED_FOR_STMT_MISSING_BLOCK}
		} else {
			blk = p.parseBlock()
			end = blk.Span.End
		}

		return &ForStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{forIdent.Span.Start, end},
				Err:             parsingErr,
				ValuelessTokens: tokens,
			},
			KeyPattern:     keyPattern,
			KeyIndexIdent:  keyIndexIdent,
			ValueElemIdent: valueElemIdent,
			ValuePattern:   valuePattern,
			Body:           blk,
			Chunked:        chunked,
			IteratedValue:  iteratedValue,
		}
	default:
		p.eatSpace()
		return parseVariableLessForStatement(first)
	}
}

func (p *parser) parseWalkStatement(walkIdent *IdentifierLiteral) *WalkStatement {
	p.panicIfContextDone()

	var parsingErr *ParsingError
	var metaIdent, entryIdent *IdentifierLiteral
	p.eatSpace()

	walked, isMissingExpr := p.parseExpression()
	tokens := []Token{{Type: WALK_KEYWORD, Span: walkIdent.Span}}

	if isMissingExpr {
		return &WalkStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{walkIdent.Span.Start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_WALK_STMT_MISSING_WALKED_VALUE},
				ValuelessTokens: tokens,
			},
			Walked: walked,
		}
	}

	p.eatSpace()
	e, _ := p.parseExpression()

	var ok bool
	if entryIdent, ok = e.(*IdentifierLiteral); !ok {
		return &WalkStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{walkIdent.Span.Start, e.Base().Span.End},
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_WALK_STMT_MISSING_ENTRY_VARIABLE_NAME},
				ValuelessTokens: tokens,
			},
			Walked: walked,
		}
	}

	p.eatSpace()

	// if the parsed identifier is instead the meta variable identifier we try to parse the entry variable identifier
	if p.i < p.len && p.s[p.i] == ',' {
		tokens = append(tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
		metaIdent = entryIdent
		entryIdent = nil
		p.eatSpace()

		// missing entry identifier
		if p.i >= p.len || p.s[p.i] == '{' {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_WALK_STMT_MISSING_ENTRY_IDENTIFIER}
		} else {
			e, _ := p.parseExpression()
			if entryIdent, ok = e.(*IdentifierLiteral); !ok {
				return &WalkStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{walkIdent.Span.Start, e.Base().Span.End},
						Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_WALK_STMT_MISSING_ENTRY_VARIABLE_NAME},
						ValuelessTokens: tokens,
					},
					MetaIdent: metaIdent,
					Walked:    walked,
				}
			}
			p.eatSpace()
		}
	}

	var blk *Block
	var end int32

	if p.i >= p.len || p.s[p.i] != '{' {
		end = p.i
		parsingErr = &ParsingError{MissingBlock, UNTERMINATED_WALK_STMT_MISSING_BLOCK}
	} else {
		blk = p.parseBlock()
		end = blk.Span.End
	}

	return &WalkStatement{
		NodeBase: NodeBase{
			Span:            NodeSpan{walkIdent.Span.Start, end},
			Err:             parsingErr,
			ValuelessTokens: tokens,
		},
		Walked:     walked,
		MetaIdent:  metaIdent,
		EntryIdent: entryIdent,
		Body:       blk,
	}
}

func (p *parser) parseSwitchMatchStatement(keywordIdent *IdentifierLiteral) Node {
	p.panicIfContextDone()

	var tokens []Token
	if keywordIdent.Name[0] == 's' {
		tokens = append(tokens, Token{Type: SWITCH_KEYWORD, Span: keywordIdent.Base().Span})
	} else {
		tokens = append(tokens, Token{Type: MATCH_KEYWORD, Span: keywordIdent.Base().Span})
	}

	isMatchStmt := keywordIdent.Name == "match"

	p.eatSpace()

	if p.i >= p.len {

		if keywordIdent.Name == "switch" {
			return &SwitchStatement{
				NodeBase: NodeBase{
					Span:            NodeSpan{keywordIdent.Span.Start, p.i},
					Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_VALUE},
					ValuelessTokens: tokens,
				},
			}
		}

		return &SwitchStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{keywordIdent.Span.Start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_MATCH_STMT_MISSING_VALUE},
				ValuelessTokens: tokens,
			},
		}
	}

	discriminant, _ := p.parseExpression()
	var switchCases []*SwitchCase
	var matchCases []*MatchCase

	p.eatSpace()

	if p.i >= p.len || p.s[p.i] != '{' {
		if !isMatchStmt {
			return &SwitchStatement{
				NodeBase: NodeBase{
					Span:            NodeSpan{keywordIdent.Span.Start, p.i},
					Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_BODY},
					ValuelessTokens: tokens,
				},
				Discriminant: discriminant,
			}
		}

		return &MatchStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{keywordIdent.Span.Start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_MATCH_STMT_MISSING_BODY},
				ValuelessTokens: tokens,
			},
			Discriminant: discriminant,
		}
	}

	tokens = append(tokens, Token{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
	p.i++

top_loop:
	for p.i < p.len && p.s[p.i] != '}' {
		p.eatSpaceNewlineSemicolonComment(&tokens)

		if p.i < p.len && p.s[p.i] == '}' {
			break
		}

		if p.i < p.len && p.s[p.i] == '{' { //missing value before block
			missingExpr := &MissingExpression{
				NodeBase: NodeBase{
					NodeSpan{p.i, p.i + 1},
					&ParsingError{UnspecifiedParsingError, fmtCaseValueExpectedHere(p.s, p.i, true)},
					nil,
				},
			}

			blk := p.parseBlock()
			base := NodeBase{
				NodeSpan{missingExpr.Span.Start, blk.Span.End},
				nil,
				nil,
			}

			if isMatchStmt {
				matchCases = append(matchCases, &MatchCase{
					NodeBase: base,
					Values:   []Node{missingExpr},
					Block:    blk,
				})
			} else {
				switchCases = append(switchCases, &SwitchCase{
					NodeBase: base,
					Values:   []Node{missingExpr},
					Block:    blk,
				})
			}
		} else { //parse values of case + block

			var switchCase *SwitchCase
			var matchCase *MatchCase

			if isMatchStmt {
				matchCase = &MatchCase{
					NodeBase: NodeBase{
						Span: NodeSpan{p.i, 0},
					},
				}
				matchCases = append(matchCases, matchCase)
			} else {
				switchCase = &SwitchCase{
					NodeBase: NodeBase{
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
					valueNode = &UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{p.i, p.i + 1},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSwitchOrMatchStatement(p.s[p.i])},
							[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
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

				if isMatchStmt && !hasStaticallyKnownValue(valueNode) {
					matchCase.Err = &ParsingError{UnspecifiedParsingError, INVALID_MATCH_CASE_VALUE_EXPLANATION}
				} else if !isMatchStmt && !NodeIsSimpleValueLiteral(valueNode) {
					switchCase.Err = &ParsingError{UnspecifiedParsingError, INVALID_SWITCH_CASE_VALUE_EXPLANATION}
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
					if isMatchStmt {
						matchCase.ValuelessTokens = append(matchCase.ValuelessTokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
					} else {
						switchCase.ValuelessTokens = append(switchCase.ValuelessTokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
					}
					p.i++

				case isAlpha(p.s[p.i]) && isMatchStmt: // group matching variable
					e, _ := p.parseExpression()

					ident, ok := e.(*IdentifierLiteral)
					if ok && isKeyword(ident.Name) {
						matchCase.Err = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
					}
					matchCase.GroupMatchingVariable = e
					p.eatSpace()
					goto parse_block
				case p.s[p.i] != '{' && p.s[p.i] != '}': //unexpected character: we add an error and parse next case
					valueNode = &UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{p.i, p.i + 1},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSwitchOrMatchStatement(p.s[p.i])},
							[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
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
			var blk *Block
			end := p.i

			if p.i >= p.len || p.s[p.i] != '{' { // missing block
				if isMatchStmt {
					matchCase.Err = &ParsingError{MissingBlock, UNTERMINATED_MATCH_CASE_MISSING_BLOCK}
				} else {
					switchCase.Err = &ParsingError{MissingBlock, UNTERMINATED_SWITCH_CASE_MISSING_BLOCK}
				}
			} else {
				blk = p.parseBlock()
				end = blk.Span.End
			}

			if isMatchStmt {
				matchCase.Span.End = end
				matchCase.Block = blk
			} else {
				switchCase.Span.End = end
				switchCase.Block = blk
			}
		}

		p.eatSpaceNewlineSemicolonComment(&tokens)
	}

	var parsingErr *ParsingError

	if p.i >= p.len || p.s[p.i] != '}' {
		if keywordIdent.Name == "switch" {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE}
		} else {
			parsingErr = &ParsingError{UnspecifiedParsingError, UNTERMINATED_MATCH_STMT_MISSING_CLOSING_BRACE}
		}
	} else {
		tokens = append(tokens, Token{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	if isMatchStmt {
		return &MatchStatement{
			NodeBase: NodeBase{
				NodeSpan{keywordIdent.Span.Start, p.i},
				parsingErr,
				tokens,
			},
			Discriminant: discriminant,
			Cases:        matchCases,
		}
	}

	return &SwitchStatement{
		NodeBase: NodeBase{
			NodeSpan{keywordIdent.Span.Start, p.i},
			parsingErr,
			tokens,
		},
		Discriminant: discriminant,
		Cases:        switchCases,
	}
}

func (p *parser) parsePermissionDroppingStatement(dropPermIdent *IdentifierLiteral) *PermissionDroppingStatement {
	p.panicIfContextDone()

	p.eatSpace()

	e, _ := p.parseExpression()
	objLit, ok := e.(*ObjectLiteral)

	var parsingErr *ParsingError
	var end int32

	if ok {
		end = objLit.Span.End
	} else {
		end = e.Base().Span.End
		parsingErr = &ParsingError{UnspecifiedParsingError, DROP_PERM_KEYWORD_SHOULD_BE_FOLLOWED_BY}
	}

	return &PermissionDroppingStatement{
		NodeBase: NodeBase{
			NodeSpan{dropPermIdent.Base().Span.Start, end},
			parsingErr,
			[]Token{{Type: DROP_PERMS_KEYWORD, Span: dropPermIdent.Span}},
		},
		Object: objLit,
	}

}

func (p *parser) parseImportStatement(importIdent *IdentifierLiteral) Node {
	p.panicIfContextDone()

	tokens := []Token{
		{Type: IMPORT_KEYWORD, Span: importIdent.Span},
	}

	p.eatSpace()

	e, _ := p.parseExpression()

	var identifier *IdentifierLiteral

	switch node := e.(type) {
	case *RelativePathLiteral:
		return &InclusionImportStatement{
			NodeBase: NodeBase{
				NodeSpan{importIdent.Span.Start, p.i},
				nil,
				tokens,
			},
			Source: node,
		}
	case *AbsolutePathLiteral:
		return &InclusionImportStatement{
			NodeBase: NodeBase{
				NodeSpan{importIdent.Span.Start, p.i},
				&ParsingError{UnspecifiedParsingError, INCLUSION_IMPORT_STMT_SRC_SHOULD_BE_A_RELATIVE_PATH_LIT},
				tokens,
			},
			Source: node,
		}
	case *IdentifierLiteral:
		identifier = node
	default:
		return &ImportStatement{
			NodeBase: NodeBase{
				NodeSpan{importIdent.Span.Start, p.i},
				&ParsingError{UnspecifiedParsingError, IMPORT_STMT_IMPORT_KEYWORD_SHOULD_BE_FOLLOWED_BY_IDENT},
				tokens,
			},
			Source: node,
		}
	}

	p.eatSpace()

	src, _ := p.parseExpression()

	switch src.(type) {
	case *URLLiteral, *RelativePathLiteral, *AbsolutePathLiteral:
	default:
		return &ImportStatement{
			NodeBase: NodeBase{
				NodeSpan{importIdent.Span.Start, p.i},
				&ParsingError{UnspecifiedParsingError, IMPORT_STMT_SRC_SHOULD_BE_AN_URL_OR_PATH_LIT},
				nil,
			},
		}
	}

	p.eatSpace()
	config, _ := p.parseExpression()

	if _, ok := config.(*ObjectLiteral); !ok && config.Base().Err == nil {
		config.BasePtr().Err = &ParsingError{UnspecifiedParsingError, IMPORT_STMT_CONFIG_SHOULD_BE_AN_OBJ_LIT}
	}

	return &ImportStatement{
		NodeBase: NodeBase{
			NodeSpan{importIdent.Span.Start, p.i},
			nil,
			tokens,
		},
		Identifier:    identifier,
		Source:        src,
		Configuration: config,
	}
}

func (p *parser) parseReturnStatement(returnIdent *IdentifierLiteral) *ReturnStatement {
	p.panicIfContextDone()

	var end int32 = p.i
	var returnValue Node

	p.eatSpace()

	if p.i < p.len && p.s[p.i] != ';' && p.s[p.i] != '}' && p.s[p.i] != '\n' {
		returnValue, _ = p.parseExpression()
		end = returnValue.Base().Span.End
	}

	return &ReturnStatement{
		NodeBase: NodeBase{
			Span:            NodeSpan{returnIdent.Span.Start, end},
			ValuelessTokens: []Token{{Type: RETURN_KEYWORD, Span: returnIdent.Span}},
		},
		Expr: returnValue,
	}
}

func (p *parser) parseYieldStatement(yieldIdent *IdentifierLiteral) *YieldStatement {
	p.panicIfContextDone()

	var end int32 = p.i
	var returnValue Node

	p.eatSpace()

	if p.i < p.len && p.s[p.i] != ';' && p.s[p.i] != '}' && p.s[p.i] != '\n' {
		returnValue, _ = p.parseExpression()
		end = returnValue.Base().Span.End
	}

	return &YieldStatement{
		NodeBase: NodeBase{
			Span:            NodeSpan{yieldIdent.Span.Start, end},
			ValuelessTokens: []Token{{Type: YIELD_KEYWORD, Span: yieldIdent.Span}},
		},
		Expr: returnValue,
	}
}

func (p *parser) parseSynchronizedBlock(synchronizedIdent *IdentifierLiteral) *SynchronizedBlockStatement {
	p.panicIfContextDone()

	var tokens = []Token{{Type: SYNCHRONIZED_KEYWORD, Span: synchronizedIdent.Span}}

	p.eatSpace()
	if p.i >= p.len {
		return &SynchronizedBlockStatement{
			NodeBase: NodeBase{
				Span:            NodeSpan{synchronizedIdent.Span.Start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, SYNCHRONIZED_KEYWORD_SHOULD_BE_FOLLOWED_BY_SYNC_VALUES},
				ValuelessTokens: tokens,
			},
		}
	}

	var synchronizedValues []Node

	for p.i < p.len && p.s[p.i] != '{' {
		valueNode, isMissingExpr := p.parseExpression()
		if isMissingExpr {
			valueNode = &UnknownNode{
				NodeBase: NodeBase{
					NodeSpan{p.i, p.i + 1},
					&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSynchronizedValueList(p.s[p.i])},
					[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
				},
			}
			p.i++
		}
		synchronizedValues = append(synchronizedValues, valueNode)

		p.eatSpace()
	}

	var parsingErr *ParsingError
	var block *Block

	if p.i >= p.len || p.s[p.i] != '{' {
		parsingErr = &ParsingError{MissingBlock, UNTERMINATED_SYNCHRONIZED_MISSING_BLOCK}
	} else {
		block = p.parseBlock()
	}

	return &SynchronizedBlockStatement{
		NodeBase: NodeBase{
			Span:            NodeSpan{synchronizedIdent.Span.Start, p.i},
			Err:             parsingErr,
			ValuelessTokens: tokens,
		},
		SynchronizedValues: synchronizedValues,
		Block:              block,
	}
}

func (p *parser) parseMultiAssignmentStatement(assignIdent *IdentifierLiteral) *MultiAssignment {
	p.panicIfContextDone()

	var vars []Node
	var tokens = []Token{
		{Type: ASSIGN_KEYWORD, Span: assignIdent.Span},
	}

	nillable := false

	if p.i < p.len && p.s[p.i] == '?' {
		nillable = true
		tokens = append(tokens, Token{Type: QUESTION_MARK, Span: NodeSpan{p.i, p.i + 1}})
		p.i++
	}

	var keywordLHSError *ParsingError

	for p.i < p.len && p.s[p.i] != '=' {
		p.eatSpace()
		e, _ := p.parseExpression()
		ident, ok := e.(*IdentifierLiteral)
		if !ok {
			return &MultiAssignment{
				NodeBase: NodeBase{
					Span: NodeSpan{assignIdent.Span.Start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, ASSIGN_KEYWORD_SHOULD_BE_FOLLOWED_BY_IDENTS},
				},
				Variables: vars,
			}
		}
		if isKeyword(ident.Name) {
			keywordLHSError = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
		}
		vars = append(vars, e)
		p.eatSpace()
	}

	var (
		right Node
		end   int32
	)
	if p.i >= p.len || p.s[p.i] != '=' {
		keywordLHSError = &ParsingError{UnspecifiedParsingError, UNTERMINATED_MULTI_ASSIGN_MISSING_EQL_SIGN}
		end = p.i
	} else {
		tokens = append(tokens, Token{Type: EQUAL, Span: NodeSpan{p.i, p.i + 1}})
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
				keywordLHSError = &ParsingError{InvalidNext, UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR}
			}
		}
	}

	return &MultiAssignment{
		NodeBase: NodeBase{
			Span:            NodeSpan{assignIdent.Span.Start, end},
			Err:             keywordLHSError,
			ValuelessTokens: tokens,
		},
		Variables: vars,
		Right:     right,
		Nillable:  nillable,
	}
}

func (p *parser) parseAssignmentAndPatternDefinition(left Node) (result Node) {
	p.panicIfContextDone()

	// terminator
	defer func() {
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
				base.Err = &ParsingError{InvalidNext, UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR}
			}
		}
	}()

	var tokens []Token
	var assignmentTokenType TokenType
	var assignmentOperator AssignmentOperator

	{
		switch p.s[p.i] {
		case '=':
			assignmentTokenType = EQUAL
			assignmentOperator = Assign
		case '+':
			assignmentTokenType = PLUS_EQUAL
			assignmentOperator = PlusAssign
			p.i++
		case '-':
			assignmentTokenType = MINUS_EQUAL
			assignmentOperator = MinusAssign
			p.i++
		case '*':
			assignmentTokenType = MUL_EQUAL
			assignmentOperator = MulAssign
			p.i++
		case '/':
			assignmentTokenType = DIV_EQUAL
			assignmentOperator = DivAssign
			p.i++
		}
		tokens = append(tokens, Token{Type: assignmentTokenType, Span: NodeSpan{p.i, p.i + 1}})
	}

	p.i++
	p.eatSpace()

	var keywordLHSError *ParsingError

	switch l := left.(type) {
	case *PatternIdentifierLiteral:
		{
			start := left.Base().Span.Start
			var right Node
			var parsingErr *ParsingError

			if p.i >= p.len {
				return &PatternDefinition{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT_DEF_MISSING_RHS},
						tokens,
					},
					Left: l,
				}
			} else if assignmentTokenType != EQUAL {
				return &PatternDefinition{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						&ParsingError{UnspecifiedParsingError, INVALID_PATT_DEF_MISSING_OPERATOR_SHOULD_BE_EQUAL},
						tokens,
					},
					Left: l,
				}
			}

			isLazy := false
			if p.s[p.i] == '@' && p.i < p.len-1 && unicode.IsSpace(p.s[p.i+1]) {
				isLazy = true
				p.i++
			}

			p.eatSpace()

			{
				prev := p.inPattern
				defer func() {
					p.inPattern = prev
				}()
			}

			p.inPattern = true
			right, _ = p.parseExpression()

			return &PatternDefinition{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					parsingErr,
					tokens,
				},
				Left:   left.(*PatternIdentifierLiteral),
				Right:  right,
				IsLazy: isLazy,
			}

		}
	case *PatternNamespaceIdentifierLiteral:
		{
			start := left.Base().Span.Start
			var right Node
			var parsingErr *ParsingError

			if p.i >= p.len {
				return &PatternNamespaceDefinition{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT_NS_DEF_MISSING_RHS},
						tokens,
					},
					Left: l,
				}
			} else if assignmentTokenType != EQUAL {
				return &PatternNamespaceDefinition{
					NodeBase: NodeBase{
						NodeSpan{start, p.i},
						&ParsingError{UnspecifiedParsingError, INVALID_PATT_NS_DEF_MISSING_OPERATOR_SHOULD_BE_EQUAL},
						tokens,
					},
					Left: l,
				}
			}

			p.eatSpace()
			right, _ = p.parseExpression()

			return &PatternNamespaceDefinition{
				NodeBase: NodeBase{
					NodeSpan{start, p.i},
					parsingErr,
					tokens,
				},
				Left:  l,
				Right: right,
			}
		}

	case *GlobalVariable, *Variable, *MemberExpression, *IndexExpression, *SliceExpression, *IdentifierMemberExpression:
	case *IdentifierLiteral:
		if isKeyword(l.Name) {
			keywordLHSError = &ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS}
		}
	default:
		return &Assignment{
			NodeBase: NodeBase{
				Span:            NodeSpan{left.Base().Span.Start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, fmtInvalidAssignmentInvalidLHS(left)},
				ValuelessTokens: tokens,
			},
			Left:     left,
			Operator: assignmentOperator,
		}
	}

	if p.i >= p.len {
		return &Assignment{
			NodeBase: NodeBase{
				Span:            NodeSpan{left.Base().Span.Start, p.i},
				Err:             &ParsingError{UnspecifiedParsingError, UNTERMINATED_ASSIGNMENT_MISSING_VALUE_AFTER_EQL_SIGN},
				ValuelessTokens: tokens,
			},
			Left:     left,
			Operator: assignmentOperator,
		}
	}

	var right Node

	if p.s[p.i] == '|' {
		tokens = append(tokens, Token{Type: PIPE, Span: NodeSpan{p.i, p.i + 1}})

		p.i++
		p.eatSpace()
		right = p.parseStatement()
		pipeline, ok := right.(*PipelineStatement)

		if !ok {
			return &Assignment{
				NodeBase: NodeBase{
					Span:            NodeSpan{left.Base().Span.Start, p.i},
					Err:             &ParsingError{UnspecifiedParsingError, INVALID_ASSIGN_A_PIPELINE_EXPR_WAS_EXPECTED_AFTER_PIPE},
					ValuelessTokens: tokens,
				},
				Left:     left,
				Right:    right,
				Operator: assignmentOperator,
			}
		}

		right = &PipelineExpression{
			NodeBase: pipeline.NodeBase,
			Stages:   pipeline.Stages,
		}
	} else {
		right, _ = p.parseExpression()
	}

	return &Assignment{
		NodeBase: NodeBase{
			Span:            NodeSpan{left.Base().Span.Start, right.Base().Span.End},
			ValuelessTokens: tokens,
			Err:             keywordLHSError,
		},
		Left:     left,
		Right:    right,
		Operator: assignmentOperator,
	}
}

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
			nil,
		},
		Stages: []*PipelineStage{
			{
				Kind: NormalStage,
				Expr: call,
			},
		},
	}

	stmt.ValuelessTokens = append(stmt.ValuelessTokens, Token{Type: PIPE, Span: NodeSpan{p.i, p.i + 1}})
	p.i++
	p.eatSpace()

	if p.i >= p.len {
		stmt.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PIPE_STMT_LAST_STAGE_EMPTY}
		return stmt
	}

	for p.i < p.len && p.s[p.i] != '\n' {
		p.eatSpace()
		if p.i >= p.len {
			stmt.Err = &ParsingError{UnspecifiedParsingError, UNTERMINATED_PIPE_STMT_LAST_STAGE_EMPTY}
			return stmt
		}

		callee, _ := p.parseExpression()

		currentCall := &CallExpression{
			NodeBase: NodeBase{
				Span: NodeSpan{callee.Base().Span.Start, 0},
			},
			Callee:            callee,
			Arguments:         nil,
			Must:              true,
			CommandLikeSyntax: true,
		}

		stmt.Stages = append(stmt.Stages, &PipelineStage{
			Kind: NormalStage,
			Expr: currentCall,
		})

		switch callee.(type) {
		case *IdentifierLiteral, *IdentifierMemberExpression:

			p.parseCallArgsNoParenthesis(currentCall)

			if len32(currentCall.Arguments) == 0 {
				currentCall.NodeBase.Span.End = callee.Base().Span.End
			} else {
				currentCall.NodeBase.Span.End = currentCall.Arguments[len32(currentCall.Arguments)-1].Base().Span.End
			}

			stmt.Span.End = currentCall.Span.End

			p.eatSpace()

			if p.i >= p.len {
				return stmt
			}

			switch p.s[p.i] {
			case '|':
				stmt.ValuelessTokens = append(stmt.ValuelessTokens, Token{Type: PIPE, Span: NodeSpan{p.i, p.i + 1}})
				p.i++
				continue //we parse the next stage
			case '\n':
				return stmt
			case ';':
				return stmt
			default:
				stmt.Err = &ParsingError{UnspecifiedParsingError, fmtInvalidPipelineStageUnexpectedChar(p.s[p.i])}
				return stmt
			}
		default:
			stmt.Err = &ParsingError{UnspecifiedParsingError, INVALID_PIPE_STATE_ALL_STAGES_SHOULD_BE_CALLS}
			return stmt
		}
	}

	return stmt
}

func (p *parser) parseStatement() Node {
	// no p.panicIfContextDone() call because there is one in the following statement.

	expr, _ := p.parseExpression()

	var b rune
	followedBySpace := false
	isAKeyword := false

	switch e := expr.(type) {
	case *IdentifierLiteral, *IdentifierMemberExpression: //funcname <no args>
		if expr.Base().IsParenthesized() {
			break
		}

		if idnt, isIdentLiteral := expr.(*IdentifierLiteral); isIdentLiteral && isKeyword(idnt.Name) {
			isAKeyword = true
			break
		}

		prevI := p.i
		p.eatSpace()

		//function call with command-line syntax and no arguments
		if p.i < p.len && p.s[p.i] == ';' {
			if p.i < p.len {
				p.i++
			}
			return &CallExpression{
				NodeBase: NodeBase{
					Span: NodeSpan{expr.Base().Span.Start, p.i},
				},
				Callee:            expr,
				Arguments:         nil,
				Must:              true,
				CommandLikeSyntax: true,
			}
		} else {
			p.i = prevI
		}
	case *MissingExpression:
		if p.i >= p.len {
			break
		}
		p.i++
		return &UnknownNode{
			NodeBase: NodeBase{
				NodeSpan{expr.Base().Span.Start, p.i},
				&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i-1])},
				append(expr.Base().ValuelessTokens, Token{
					Type: UNEXPECTED_CHAR,
					Raw:  string(p.s[p.i-1]),
					Span: NodeSpan{p.i - 1, p.i},
				}),
			},
		}
	case *TestSuiteExpression:
		if expr.Base().IsParenthesized() {
			break
		}

		e.IsStatement = true
	case *TestCaseExpression:
		if expr.Base().IsParenthesized() {
			break
		}

		e.IsStatement = true
	}

	if p.i >= p.len {
		if !isAKeyword {
			return expr
		}
	} else {
		b = p.s[p.i]
		followedBySpace = b == ' '
	}

	switch ev := expr.(type) {
	case *CallExpression:
		return ev
	case *IdentifierLiteral:
		switch ev.Name {
		case "assert":
			p.eatSpace()

			expr, _ := p.parseExpression()

			return &AssertionStatement{
				NodeBase: NodeBase{
					NodeSpan{ev.Span.Start, expr.Base().Span.End},
					nil,
					[]Token{{Type: ASSERT_KEYWORD, Span: ev.Span}},
				},
				Expr: expr,
			}
		case "if":
			return p.parseIfStatement(ev)
		case "for":
			return p.parseForStatement(ev)
		case "walk":
			return p.parseWalkStatement(ev)
		case "switch", "match":
			return p.parseSwitchMatchStatement(ev)
		case "fn":
			log.Panic("invalid state: function parsing should be hanlded by p.parseExpression")
			return nil
		case "drop-perms":
			return p.parsePermissionDroppingStatement(ev)
		case "import":
			return p.parseImportStatement(ev)
		case "return":
			return p.parseReturnStatement(ev)
		case "yield":
			return p.parseYieldStatement(ev)
		case "break":
			return &BreakStatement{
				NodeBase: NodeBase{
					Span:            ev.Span,
					ValuelessTokens: []Token{{Type: BREAK_KEYWORD, Span: ev.Span}},
				},
				Label: nil,
			}
		case "continue":
			return &ContinueStatement{
				NodeBase: NodeBase{
					Span:            ev.Span,
					ValuelessTokens: []Token{{Type: CONTINUE_KEYWORD, Span: ev.Span}},
				},
				Label: nil,
			}
		case "prune":
			return &PruneStatement{
				NodeBase: NodeBase{
					Span:            ev.Span,
					ValuelessTokens: []Token{{Type: PRUNE_KEYWORD, Span: ev.Span}},
				},
			}
		case "assign":
			return p.parseMultiAssignmentStatement(ev)
		case "var":
			return p.parseLocalVariableDeclarations(ev.Base())
		case "synchronized":
			return p.parseSynchronizedBlock(ev)
		}

	}

	p.eatSpace()

	if p.i >= p.len {
		return expr
	}

	switch p.s[p.i] {
	case '=': //assignment
		return p.parseAssignmentAndPatternDefinition(expr)
	case ';':
		return expr
	case '+', '-', '*', '/':
		if p.i < p.len-1 && p.s[p.i+1] == '=' {
			return p.parseAssignmentAndPatternDefinition(expr)
		}

		if followedBySpace && !expr.Base().IsParenthesized() {
			return p.parseCommandLikeStatement(expr)
		}
	default:
		if expr.Base().IsParenthesized() {
			break
		}

		switch expr.(type) {
		case *IdentifierLiteral, *IdentifierMemberExpression:
			if !followedBySpace ||
				(isUnpairedOrIsClosingDelim(p.s[p.i]) && p.s[p.i] != '(' && p.s[p.i] != '|' && p.s[p.i] != '\n' && p.s[p.i] != ':') {
				break
			}
			return p.parseCommandLikeStatement(expr)
		}
	}
	return expr
}

func (p *parser) parseChunk() (*Chunk, error) {
	p.panicIfContextDone()

	chunk := &Chunk{
		NodeBase: NodeBase{
			Span: NodeSpan{Start: 0, End: p.len},
		},
		Statements: nil,
	}

	var (
		stmts           []Node
		valuelessTokens []Token
	)

	//shebang
	if p.i < p.len-1 && p.s[0] == '#' && p.s[1] == '!' {
		for p.i < p.len && p.s[p.i] != '\n' {
			p.i++
		}
	}

	p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
	globalConstDecls := p.parseGlobalConstantDeclarations()

	p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
	preinit := p.parsePreInitIfPresent()

	p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
	manifest := p.parseManifestIfPresent()

	p.eatSpaceNewlineSemicolonComment(&valuelessTokens)

	//parse statements

	prevStmtEndIndex := int32(-1)
	var prevStmtErrKind ParsingErrorKind

	for p.i < p.len {
		if IsForbiddenSpaceCharacter(p.s[p.i]) {
			stmts = append(stmts, &UnknownNode{
				NodeBase: NodeBase{
					Span:            NodeSpan{p.i, p.i + 1},
					Err:             &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(p.s[p.i])},
					ValuelessTokens: []Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{p.i, p.i + 1}, Raw: string(p.s[p.i])}},
				},
			})
			p.i++
			p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
			continue
		}

		var stmtErr *ParsingError

		if p.i == prevStmtEndIndex && prevStmtErrKind != InvalidNext && !unicode.IsSpace(p.s[p.i-1]) {
			stmtErr = &ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY}
		}

		stmt := p.parseStatement()
		prevStmtEndIndex = p.i

		if _, isMissingExpr := stmt.(*MissingExpression); isMissingExpr {
			stmts = append(stmts, stmt)
			break
		}

		if stmt.Base().Err != nil {
			prevStmtErrKind = stmt.Base().Err.kind
		} else if stmtErr != nil {
			stmt.BasePtr().Err = stmtErr
		}
		stmts = append(stmts, stmt)

		p.eatSpaceNewlineSemicolonComment(&valuelessTokens)
	}

	chunk.Preinit = preinit
	chunk.Manifest = manifest
	chunk.Statements = stmts
	chunk.GlobalConstantDeclarations = globalConstDecls
	chunk.ValuelessTokens = valuelessTokens

	return chunk, nil
}

func ParseExpression(u string) (n Node, ok bool) {
	if len(u) > MAX_MODULE_BYTE_LEN {
		return nil, false
	}

	p := newParser([]rune(u))
	expr, isMissingExpr := p.parseExpression()

	noError := true
	Walk(expr, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if node.Base().Err != nil {
			noError = false
			return StopTraversal, nil
		}
		return Continue, nil
	}, nil)

	return expr, noError && !isMissingExpr && p.i >= p.len
}

func ParsePath(pth string) (path string, ok bool) {
	if len(pth) > MAX_MODULE_BYTE_LEN || len(pth) == 0 {
		return "", false
	}

	p := newParser([]rune(pth))

	switch path := p.parsePathLikeExpression(false).(type) {
	case *AbsolutePathLiteral:
		return path.Value, p.i >= p.len
	case *RelativePathLiteral:
		return path.Value, p.i >= p.len
	default:
		return "", false
	}
}

func ParsePathPattern(pth string) (ok bool) {
	if len(pth) > MAX_MODULE_BYTE_LEN {
		return false
	}

	p := newParser([]rune(pth))

	switch p.parsePathLikeExpression(false).(type) {
	case *AbsolutePathPatternLiteral, *RelativePathPatternLiteral:
		return p.i >= p.len
	default:
		return false
	}
}

func ParseURL(u string) (path string, ok bool) {
	if len(u) > MAX_MODULE_BYTE_LEN {
		return "", false
	}

	p := newParser([]rune(u))
	url, ok := p.parseURLLike(0).(*URLLiteral)

	return url.Value, ok && p.i >= p.len
}

func isKeyword(str string) bool {
	return utils.SliceContains(KEYWORDS, str)
}

func IsMetadataKey(key string) bool {
	return len(key) > 2 && key[0] == '_' && key[len(key)-1] == '_'
}

func isAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isAlphaOrUndescore(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}

func isByteSliceBase(r rune) bool {
	switch r {
	case 'x', 'd', 'b':
		return true
	}
	return false
}

func isDecDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isIdentChar(r rune) bool {
	return isAlpha(r) || isDecDigit(r) || r == '-' || r == '_'
}

func isFirstIdentChar(r rune) bool {
	return isAlpha(r) || r == '_'
}

func isInterpolationAllowedChar(r rune) bool {
	return isIdentChar(r) || isDecDigit(r) || r == '[' || r == ']' || r == '.' || r == '$' || r == ':'
}

func isUnquotedStringChar(r rune) bool {
	return isIdentChar(r) || r == '+' || r == '~' || r == '/' || r == '^' || r == '@' || r == '.' || r == '%'
}

func isSpaceNotLF(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r'
}

func IsCommentFirstSpace(r rune) bool {
	return isSpaceNotLF(r)
}

func IsDelim(r rune) bool {
	switch r {
	case '{', '}', '[', ']', '(', ')', '\n', ',', ';', ':', '|':
		return true
	default:
		return false
	}
}

func isOpeningDelim(r rune) bool {
	switch r {
	case '{', '[', '(':
		return true
	default:
		return false
	}
}

func isUnpairedDelim(r rune) bool {
	switch r {
	case '\n', ',', ';', ':', '|':
		return true
	default:
		return false
	}
}

func isPairedDelim(r rune) bool {
	switch r {
	case '{', '}', '[', ']', '(', ')':
		return true
	default:
		return false
	}
}

func isClosingDelim(r rune) bool {
	switch r {
	case '}', ')', ']':
		return true
	default:
		return false
	}
}

func isUnpairedOrIsClosingDelim(r rune) bool {
	switch r {
	case '\n', ',', ';', ':', '=', ')', ']', '}', '|':
		return true
	default:
		return false
	}
}

func isNonSpaceCSSCombinator(r rune) bool {
	switch r {
	case '>', '~', '+':
		return true
	default:
		return false
	}
}

func HasPathLikeStart(s string) bool {
	if len(s) == 0 {
		return false
	}

	return s[0] == '/' || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../")
}

func countPrevBackslashes(s []rune, i int32) int32 {
	index := i - 1
	count := int32(0)
	for ; index >= 0; index-- {
		if s[index] == '\\' {
			count += 1
		} else {
			break
		}
	}

	return count
}

func containsNotEscapedBracket(s []rune) bool {
	for i, e := range s {
		if e == '{' {
			if countPrevBackslashes(s, int32(i))%2 == 0 {
				return true
			}
		}
	}
	return false
}

func containsNotEscapedDollar(s []rune) bool {
	for i, e := range s {
		if e == '$' {
			if countPrevBackslashes(s, int32(i))%2 == 0 {
				return true
			}
		}
	}
	return false
}

func IsForbiddenSpaceCharacter(r rune) bool {
	return unicode.IsSpace(r) && r != '\n' && !isSpaceNotLF(r)
}

func isValidEntryEnd(s []rune, i int32) bool {
	switch s[i] {
	case '\n', ',', '}':
	case '#':
		if i < len32(s)-1 && IsCommentFirstSpace(s[i+1]) {
			break
		}
		fallthrough
	default:
		return false
	}
	return true
}

func isNonIdentBinaryOperatorChar(r rune) bool {
	switch r {
	case '+', '-', '*', '/', '\\', '>', '<', '?', '.', '!', '=':
		return true
	default:
		return false
	}
}

func IsAnyVariableIdentifier(node Node) bool {
	switch node.(type) {
	case *GlobalVariable, *Variable, *IdentifierLiteral:
		return true
	default:
		return false
	}
}

func IsSupportedSchemeName(s string) bool {
	return utils.SliceContains(SCHEMES, s)
}

func GetVariableName(node Node) string {
	switch n := node.(type) {
	case *GlobalVariable:
		return n.Name
	case *Variable:
		return n.Name
	case *IdentifierLiteral:
		return n.Name
	default:
		panic(fmt.Errorf("cannot get variable name from node of type %T", node))
	}
}

func GetNameIfVariable(node Node) (string, bool) {
	switch n := node.(type) {
	case *GlobalVariable:
		return n.Name, true
	case *Variable:
		return n.Name, true
	case *IdentifierLiteral:
		return n.Name, true
	default:
		return "", false
	}
}

func hasStaticallyKnownValue(node Node) (result bool) {

	result = true

	Walk(node, func(node, parent, scopeNode Node, ancestorChain []Node, _ bool) (TraversalAction, error) {
		switch node.(type) {
		case *NamedSegmentPathPatternLiteral:
			return Prune, nil
		case *GlobalVariable, *Variable, *AtHostLiteral, *CallExpression, *IndexExpression, *MemberExpression,
			*SliceExpression, *AbsolutePathExpression, *IfStatement, *ForStatement, *SwitchStatement, *MatchStatement, *Assignment,
			*MultiAssignment, *ImportStatement, *BreakStatement, *ContinueStatement, *ReturnStatement, *FunctionExpression:
			result = false
			return StopTraversal, nil
		}
		return Continue, nil
	}, nil)

	return
}
func len32[T any](arg []T) int32 {
	return int32(len(arg))
}

func MustParseChunk(str string, opts ...parserOptions) (result *Chunk) {
	n, err := ParseChunk(str, "<chunk>", opts...)
	if err != nil {
		panic(err)
	}
	return n
}

func MustParseExpression(str string, opts ...parserOptions) Node {
	n, ok := ParseExpression(str)
	if !ok {
		panic(errors.New("invalid expression"))
	}
	return n
}
