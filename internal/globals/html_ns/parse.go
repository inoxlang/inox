package html_ns

import (
	"errors"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func init() {
	core.RegisterParser(core.HTML_CTYPE, &htmlDocParser{})
}

var (
	ErrSingleNode = errors.New("a single HTML node was expected")
)

func ParseSingleNodeHTML(nodeHTML string) (*HTMLNode, error) {
	nodes, err := parseHTML(nodeHTML)
	if err != nil {
		return nil, err
	}

	if len(nodes) > 1 {
		return nil, ErrSingleNode
	}
	return NewHTMLNode(nodes[0]), nil
}

func parseHTML(htm string) ([]*html.Node, error) {
	context := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Div,
		Data:     "div",
	}

	r := strings.NewReader(htm)

	nodes, err := html.ParseFragment(r, context)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

type htmlDocParser struct {
}

func (p *htmlDocParser) Validate(ctx *core.Context, s string) bool {
	_, parsingErr := html.Parse(strings.NewReader(s))
	return parsingErr == nil
}

func (p *htmlDocParser) Parse(ctx *core.Context, s string) (core.Serializable, error) {
	_res, parsingErr := html.Parse(strings.NewReader(s))
	if parsingErr != nil {
		return nil, parsingErr
	}

	return NewHTMLNode(_res), nil
}
