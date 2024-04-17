package parse

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/inoxlang/inox/internal/inoxconsts"
)

const (
	//URL & host

	URL_CREDENTIALS_PATTERN = "([-a-zA-Z0-9@:%._+~#=]*@)?"

	LOOSE_URL_EXPR_PATTERN = "^" +
		//host and credentials
		"(" +
		/* host is a variable */ "[$][a-zA-Z0-9_-]+|" +
		/* regular host, scheme:  */ "([a-z][a-z0-9+]*:\\/{2}" +
		/*     					  */ URL_CREDENTIALS_PATTERN +
		/*     hostname: */ "([-\\w]+|[-a-zA-Z0-9.]{1,64}\\.[a-zA-Z0-9]{1,6}\\b|\\{[$]{0,1}[-\\w]+\\}))" +
		/*     port:     */ "(:[0-9]+)?" +
		")" +
		//path, query and fragment and interpolations
		"([{?#/][-a-zA-Z0-9@:%_+.~#?&//=${}]*)$"

	LOOSE_HOST_PATTERN_PATTERN = "^([a-z][a-z0-9+]*)?:\\/\\/" + //scheme
		URL_CREDENTIALS_PATTERN +
		"([-\\w]+|[*]+|[-a-zA-Z0-9.*]{1,64}\\.[a-zA-Z0-9*]{1,6})" + //hostname
		"(:[0-9]+)?$" //port

	LOOSE_HOST_PATTERN = "^([a-z][a-z0-9+]*)?:\\/\\/" + //scheme
		URL_CREDENTIALS_PATTERN +
		"([-\\w]+|[-a-zA-Z0-9.]{1,64}\\.[a-zA-Z0-9]{1,6})" + //hostname
		"(:[0-9]+)?$" //port

	LOOSE_URL_PATTERN = "^([a-z][a-z0-9+]*):\\/\\/" + //scheme
		"([-\\w]+|[-a-zA-Z0-9@:%._+~#=]{1,64}\\.[a-zA-Z0-9]{1,6})\\b" + //hostname
		"(:[0-9]+)?" + //port
		"([?#/][-a-zA-Z0-9@:%_*+.~#?&//=]*)$" //path, query and fragment

)

var (
	SCHEMES = []string{"http", "https", "ws", "wss", inoxconsts.LDB_SCHEME_NAME, inoxconsts.ODB_SCHEME_NAME, "file", "mem", "s3"}

	//URL & host regexes

	LOOSE_URL_REGEX          = regexp.MustCompile(LOOSE_URL_PATTERN)
	LOOSE_HOST_REGEX         = regexp.MustCompile(LOOSE_HOST_PATTERN)
	LOOSE_HOST_PATTERN_REGEX = regexp.MustCompile(LOOSE_HOST_PATTERN_PATTERN)
	LOOSE_URL_EXPR_REGEX     = regexp.MustCompile(LOOSE_URL_EXPR_PATTERN)
)

// parseURLLike parses URLs, URL expressions and Hosts
func (p *parser) parseURLLike(start int32, hostVariable *Variable) Node {
	p.panicIfContextDone()

	missingSlashAfterScheme := false

	if hostVariable == nil {
		if p.i == p.len-2 || p.s[p.i+2] != '/' {
			p.i += 2 // :/
			missingSlashAfterScheme = true
		} else {
			p.i += 3 // ://
		}
	}
	afterSchemeIndex := p.i

	isIgnoredDelim := func(r rune) bool {
		return r == '=' || r == ':' || r == '{'
	}

	//we eat until we encounter a space or a delimiter different from ':' and '{'
loop:
	for p.i < p.len && p.s[p.i] != '\n' && !unicode.IsSpace(p.s[p.i]) && (!IsDelim(p.s[p.i]) || isIgnoredDelim(p.s[p.i])) {
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

	u := string(p.s[start:p.i])
	span := NodeSpan{start, p.i}

	if missingSlashAfterScheme {
		return &InvalidURL{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, INVALID_SCHEME_HOST_OR_URL_SLASH_EXPECTED},
			},
			Value: string(p.s[start:p.i]),
		}
	}

	//scheme literal
	if hostVariable == nil && p.i == afterSchemeIndex {
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

		err := CheckHost(u)
		var parsingErr *ParsingError
		if err != nil {
			parsingErr = &ParsingError{UnspecifiedParsingError, err.Error()}
		}

		return &HostLiteral{
			NodeBase: NodeBase{
				Span: span,
				Err:  parsingErr,
			},
			Value: u,
		}

	case LOOSE_URL_EXPR_REGEX.MatchString(u) && (hostVariable != nil || strings.Count(u, "{") >= 1): //url expressions
		var parsingErr *ParsingError
		pathStart := afterSchemeIndex
		pathExclEnd := afterSchemeIndex
		hasQuery := strings.Contains(u, "?")
		hostInterpolationStart := int32(-1)

		if hasQuery {
			for p.s[pathExclEnd] != '?' {
				pathExclEnd++
			}
		} else {
			pathExclEnd = p.i
		}

		if hostVariable == nil && p.s[afterSchemeIndex] == '{' { //host interpolation
			p.tokens = append(p.tokens, Token{
				Type:    OPENING_CURLY_BRACKET,
				SubType: HOST_INTERP_OPENING_BRACE,
				Span:    NodeSpan{afterSchemeIndex, afterSchemeIndex + 1},
			})

			hostInterpolationStart = pathStart
			pathStart++
			for pathStart < pathExclEnd && p.s[pathStart] != '}' {
				pathStart++
			}

			//there is necessarily a '}' because it's in the regex

			p.tokens = append(p.tokens, Token{
				Type:    CLOSING_CURLY_BRACKET,
				SubType: HOST_INTERP_CLOSING_BRACE,
				Span:    NodeSpan{pathStart, pathStart + 1},
			})
			pathStart++

		} else {
			//we increment pathStart while we are in the host part
			for pathStart < pathExclEnd && p.s[pathStart] != '/' && p.s[pathStart] != '{' {
				pathStart++
			}
		}

		if pathStart == afterSchemeIndex && hostVariable == nil {
			pathStart = pathExclEnd
		}

		slices := p.parsePathExpressionSlices(pathStart, pathExclEnd)

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
					slices = p.parseQueryParameterValueSlices(valueStart, j)
				}

				queryParams = append(queryParams, &URLQueryParameter{
					NodeBase: NodeBase{
						NodeSpan{keyStart, j},
						nil,
						false,
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
			false,
		}

		if hostInterpolationStart > 0 {
			e, ok := ParseExpression(string(p.s[hostInterpolationStart+1 : pathStart-1]))
			hostPart = &HostExpression{
				NodeBase: hostPartBase,
				Scheme: &SchemeLiteral{
					NodeBase: NodeBase{NodeSpan{span.Start, afterSchemeIndex}, nil, false},
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

			if hostPartBase.Err == nil {
				err := CheckHost(hostPartString)
				var parsingErr *ParsingError
				if err != nil {
					parsingErr = &ParsingError{UnspecifiedParsingError, err.Error()}
				}
				hostPartBase.Err = parsingErr
			}

			hostPart = &HostLiteral{
				NodeBase: hostPartBase,
				Value:    hostPartString,
			}
		} else {
			hostPart = hostVariable
		}

		return &URLExpression{
			NodeBase:    NodeBase{span, parsingErr, false},
			Raw:         u,
			HostPart:    hostPart,
			Path:        slices,
			QueryParams: queryParams,
		}
	case LOOSE_URL_REGEX.MatchString(u): //urls & url patterns
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

		var parsingErr *ParsingError

		_, err = CheckGetEffectivePort(parsed.Scheme, parsed.Port())
		if err != nil {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_URL + ": " + err.Error()}
		}

		if strings.Contains(parsed.Hostname(), "..") {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_URL}
		}

		if strings.Contains(parsed.Path, "/") {
			return &URLLiteral{
				NodeBase: NodeBase{
					Span: span,
					Err:  parsingErr,
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

func (p *parser) parseQueryParameterValueSlices(start int32, exclEnd int32) []Node {
	p.panicIfContextDone()

	slices := make([]Node, 0)
	index := start
	sliceStart := start
	inInterpolation := false

	for index < exclEnd {
		switch {
		//start of interpolation
		case !inInterpolation && p.s[index] == '{':
			p.tokens = append(p.tokens, Token{
				Type:    OPENING_CURLY_BRACKET,
				SubType: QUERY_PARAM_INTERP_OPENING_BRACE,
				Span:    NodeSpan{index, index + 1},
			})

			slice := string(p.s[sliceStart:index]) //previous cannot be an interpolation
			slices = append(slices, &URLQueryParameterValueSlice{
				NodeBase: NodeBase{
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
				slices = append(slices, &URLQueryParameterValueSlice{
					NodeBase: NodeBase{
						NodeSpan{sliceStart, sliceStart},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUERY_PARAM_INTERP},
						false,
					},
					Value: string(p.s[sliceStart:sliceStart]),
				})

				return slices
			}
		//end of interpolation
		case inInterpolation && (p.s[index] == '}' || index == exclEnd-1):
			missingClosingBrace := false
			if index == exclEnd-1 && p.s[index] != '}' {
				index++
				missingClosingBrace = true
			} else {
				p.tokens = append(p.tokens, Token{
					Type:    CLOSING_CURLY_BRACKET,
					SubType: QUERY_PARAM_INTERP_CLOSING_BRACE,
					Span:    NodeSpan{index, index + 1},
				})
			}

			interpolation := string(p.s[sliceStart:index])

			expr, ok := ParseExpression(interpolation)

			if !ok {
				span := NodeSpan{sliceStart, index}
				err := &ParsingError{UnspecifiedParsingError, INVALID_QUERY_PARAM_INTERP}

				if len(interpolation) == 0 {
					err.Message = EMPTY_QUERY_PARAM_INTERP
				}

				p.tokens = append(p.tokens, Token{Type: INVALID_INTERP_SLICE, Span: span, Raw: string(p.s[sliceStart:index])})
				slices = append(slices, &UnknownNode{
					NodeBase: NodeBase{
						span,
						err,
						false,
					},
				})
			} else {
				shiftNodeSpans(expr, sliceStart)
				slices = append(slices, expr)

				if missingClosingBrace {
					badSlice := &URLQueryParameterValueSlice{
						NodeBase: NodeBase{
							NodeSpan{index, index},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUERY_PARAM_INTERP_MISSING_CLOSING_BRACE},
							false,
						},
					}
					slices = append(slices, badSlice)
				}
			}

			inInterpolation = false
			sliceStart = index + 1
		//forbidden character
		case inInterpolation && !isInterpolationAllowedChar(p.s[index]):
			// we eat all the interpolation

			j := index
			for j < exclEnd && p.s[j] != '}' {
				j++
			}

			slices = append(slices, &URLQueryParameterValueSlice{
				NodeBase: NodeBase{
					NodeSpan{sliceStart, j},
					&ParsingError{UnspecifiedParsingError, QUERY_PARAM_INTERP_EXPLANATION},
					false,
				},
				Value: string(p.s[sliceStart:j]),
			})

			if j < exclEnd { // '}'
				p.tokens = append(p.tokens, Token{
					Type:    CLOSING_CURLY_BRACKET,
					SubType: QUERY_PARAM_INTERP_CLOSING_BRACE,
					Span:    NodeSpan{j, j + 1},
				})
				j++
			}

			inInterpolation = false
			sliceStart = j
			index = j
			continue
		}
		index++
	}

	if sliceStart != index {
		slices = append(slices, &URLQueryParameterValueSlice{
			NodeBase: NodeBase{
				NodeSpan{sliceStart, index},
				nil,
				false,
			},
			Value: string(p.s[sliceStart:index]),
		})
	}
	return slices
}

// parseURLLike parses URLs pattenrs and host patterns
func (p *parser) parseURLLikePattern(start int32, percentPrefixed bool) Node {
	p.panicIfContextDone()

	leadingSlashCount := int32(0)
	for p.i < p.len && p.s[p.i] == '/' {
		p.i++
		leadingSlashCount++
	}

	//we eat until we encounter a space or a delimiter different from ':' and '{'
loop:
	for p.i < p.len && !unicode.IsSpace(p.s[p.i]) && (!IsDelim(p.s[p.i]) || p.s[p.i] == ':' || p.s[p.i] == '{') {
		switch p.s[p.i] {
		case '{':
			p.i++
			for p.i < p.len && p.s[p.i] != '}' { //ok since '}' is not allowed in interpolations for now
				p.i++
			}
			if p.i < p.len {
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

	raw := string(p.s[start:p.i])
	u := raw
	if percentPrefixed {
		u = u[1:]
	}
	span := NodeSpan{start, p.i}

	if leadingSlashCount != 2 {
		if !percentPrefixed && strings.HasSuffix(u, ":/") && strings.Count(u, "/") == 1 {
			return &InvalidURL{
				NodeBase: NodeBase{
					Span: NodeSpan{start, p.i},
					Err:  &ParsingError{UnspecifiedParsingError, INVALID_SCHEME_LIT_SLASH_EXPECTED},
				},
				Value: u,
			}
		}
		return &InvalidURLPattern{
			NodeBase: NodeBase{
				Span: NodeSpan{start, p.i},
				Err:  &ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST_PATT_SCHEME_SHOULD_BE_FOLLOWED_BY_COLON_SLASH_SLASH},
			},
			Value: u,
		}
	}

	var parsingErr *ParsingError

	switch {
	case LOOSE_HOST_PATTERN_REGEX.MatchString(u):
		return &HostPatternLiteral{
			NodeBase: NodeBase{
				Span: span,
				Err:  CheckHostPattern(u),
			},
			Value:      u,
			Raw:        raw,
			Unprefixed: !percentPrefixed,
		}
	case percentPrefixed && strings.HasSuffix(raw, "://"):
		return &HostPatternLiteral{
			NodeBase: NodeBase{
				Span: span,
				Err:  &ParsingError{UnspecifiedParsingError, UNTERMINATED_HOST_PATT_MISSING_HOSTNAME},
			},
			Value:      u,
			Raw:        raw,
			Unprefixed: !percentPrefixed,
		}
	case !percentPrefixed && strings.HasSuffix(raw, "://"):
		return &SchemeLiteral{
			NodeBase: NodeBase{
				Span: span,
				Err:  parsingErr,
			},
			Name: raw[:int32(len(u))-3],
		}
	case !LOOSE_URL_REGEX.MatchString(u):
		parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_URL_PATT}
	default:
		parsingErr = CheckURLPattern(u)
	}

	return &URLPatternLiteral{
		NodeBase: NodeBase{
			Span: span,
			Err:  parsingErr,
		},
		Value:      u,
		Raw:        raw,
		Unprefixed: !percentPrefixed,
	}
}

func CheckHost(u string) error {
	hasScheme := u[0] != ':'

	scheme, hostPart, _ := strings.Cut(u, "://")

	var testedUrl = u

	if !hasScheme {
		scheme = inoxconsts.NO_SCHEME_SCHEME_NAME
		testedUrl = scheme + u
	}

	parsed, err := url.Parse(testedUrl)

	if parsed != nil && parsed.User.String() != "" {
		return errors.New(CREDENTIALS_NOT_ALLOWED_IN_HOST_LITERALS)
	}

	if err != nil ||
		parsed.Host != hostPart || /* too strict ? */
		parsed.User.String() != "" ||
		parsed.RawPath != "" ||
		parsed.RawQuery != "" ||
		parsed.RawFragment != "" {
		return &ParsingError{UnspecifiedParsingError, INVALID_HOST_LIT}
	}

	if strings.Contains(parsed.Hostname(), "..") {
		return &ParsingError{UnspecifiedParsingError, INVALID_HOST_LIT}
	}

	if u[len(u)-1] == ':' {
		return errors.New(INVALID_HOST_LIT_MISSING_NUMBER_AFTER_COLON)
	}

	if hasScheme {
		_, err = CheckGetEffectivePort(scheme, parsed.Port())
		if err != nil {
			return fmt.Errorf(INVALID_HOST_LIT+": %w", err)
		}
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
			} else if _, err := url.ParseRequestURI(u); err != nil {
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
		}
	}

	if parsingErr == nil {
		var testedUrl = u
		if !hasScheme {
			testedUrl = inoxconsts.NO_SCHEME_SCHEME_NAME + u
		}

		replaced := strings.ReplaceAll(testedUrl, "*", "com")
		parsed, err := url.ParseRequestURI(replaced)
		if err != nil {
			parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT}
		} else if hasScheme {
			_, err = CheckGetEffectivePort(parsed.Scheme, parsed.Port())
			if err != nil {
				parsingErr = &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT + ": " + err.Error()}
			}

			if parsed.User.String() != "" {
				return &ParsingError{UnspecifiedParsingError, CREDENTIALS_NOT_ALLOWED_IN_HOST_PATTERN_LITERALS}
			}
		}

		if parsingErr == nil && strings.Contains(parsed.Hostname(), "..") {
			return &ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT}
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

	replaced := strings.ReplaceAll(u, "*", "com")

	parsed, err := url.ParseRequestURI(replaced)
	if err != nil {
		return &ParsingError{UnspecifiedParsingError, INVALID_URL_PATT}
	} else {
		_, err = CheckGetEffectivePort(parsed.Scheme, parsed.Port())
		if err != nil {
			return &ParsingError{UnspecifiedParsingError, INVALID_URL_PATT + ": " + err.Error()}
		}
	}

	if strings.Contains(parsed.Hostname(), "..") {
		return &ParsingError{UnspecifiedParsingError, INVALID_URL_PATT}
	}

	return nil
}
