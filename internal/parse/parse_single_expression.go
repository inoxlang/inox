package parse

import (
	"errors"
	"strings"
)

// See ParseExpression.
func MustParseExpression(str string, opts ...ParserOptions) Node {
	n, ok := ParseExpression(str)
	if !ok {
		panic(errors.New("invalid expression"))
	}
	return n
}

// ParseExpression parses $s as single expression, leading space is not allowed.
// It returns (non-nil Node, false) if there is a parsing error  of if the
// expression is followed by space of additional code. (nil, false) is returned
// in the case of a internal error, although this is rare.
func ParseExpression(s string) (n Node, ok bool) {
	if len(s) > MAX_MODULE_BYTE_LEN {
		return nil, false
	}

	return parseExpression([]rune(s), false)
}

// ParseExpression parses the first expression in $s, leading space is not allowed.
// It returns (non-nil Node, false) if there is a parsing error. (nil, false) is
// returned in the case of an internal error, although this is rare.
func ParseFirstExpression(u string) (n Node, ok bool) {
	if len(u) > MAX_MODULE_BYTE_LEN {
		return nil, false
	}

	return parseExpression([]rune(u), true)
}

func parseExpression(runes []rune, firstOnly bool) (n Node, ok bool) {
	p := newParser(runes)
	defer p.cancel()

	expr, isMissingExpr := p.parseExpression()

	noError := true
	Walk(expr, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if node.Base().Err != nil {
			noError = false
			return StopTraversal, nil
		}
		return ContinueTraversal, nil
	}, nil)

	return expr, noError && !isMissingExpr && (firstOnly || p.i >= p.len)
}

// ParsePath parses $pth as a path, leading and trailing space is not allowed.
func ParsePath(pth string) (path string, ok bool) {
	if len(pth) > MAX_MODULE_BYTE_LEN || len(pth) == 0 {
		return "", false
	}

	p := newParser([]rune(pth))
	defer p.cancel()

	switch path := p.parsePathLikeExpression(false).(type) {
	case *AbsolutePathLiteral:
		return path.Value, p.i >= p.len
	case *RelativePathLiteral:
		return path.Value, p.i >= p.len
	default:
		return "", false
	}
}

// ParsePath parses $pth as a path pattern, leading and trailing space is not allowed.
func ParsePathPattern(pth string) (ok bool) {
	if len(pth) > MAX_MODULE_BYTE_LEN {
		return false
	}

	p := newParser([]rune(pth))
	defer p.cancel()

	switch p.parsePathLikeExpression(false).(type) {
	case *AbsolutePathPatternLiteral, *RelativePathPatternLiteral:
		return p.i >= p.len
	default:
		return false
	}
}

// ParsePath parses $pth as a URL literal, leading and trailing space is not allowed.
func ParseURL(u string) (urlString string, ok bool) {
	if len(u) > MAX_MODULE_BYTE_LEN {
		return "", false
	}

	p := newParser([]rune(u))
	defer p.cancel()

	index := int32(strings.Index(u, "://"))
	if index < 0 || index == 0 {
		return "", false
	}
	p.i = index

	url, ok := p.parseURLLike(0, nil).(*URLLiteral)

	if ok && p.i >= p.len {
		return url.Value, true
	}
	return "", false
}
