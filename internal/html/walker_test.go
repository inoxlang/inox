package html_ns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

func TestWalkHTMLNode(t *testing.T) {

	node, err := ParseSingleNodeHTML(`<section>
			<div>
				<span>1</span>
				<a></a>
			</div>
			<ul>
				<li></li>
			</ul>
		</section>`)

	if !assert.NoError(t, err) {
		return
	}

	expectedTags := []string{"section", "div", "span", "a", "ul", "li"}
	i := 0

	walkHTMLNode(node.node, func(n *html.Node) error {
		if n.Type != html.ElementNode {
			return nil
		}
		if i >= len(expectedTags) {
			assert.FailNowf(t, "unexpected call", "tag is %s", n.Data)
		}

		assert.Equal(t, expectedTags[i], n.Data)
		i++

		return nil
	}, 0)
}
