package html_ns

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

func TestParseHTML(t *testing.T) {

	render := func(n *html.Node) string {
		buf := bytes.NewBuffer(nil)
		html.Render(buf, n)
		return buf.String()
	}

	testCases := []string{
		"<span>1</span>", "<div><span>1</span><span>2</span></div>",
	}

	for _, str := range testCases {
		n, err := ParseSingleNodeHTML(str)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, str, render(n.node))
	}

}
