package html_ns

import (
	"bytes"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/stretchr/testify/assert"
)

func TestHTMLRender(t *testing.T) {
	nodeHtml := "<div><span>a</span></div>"
	node, _ := ParseSingleNodeHTML(nodeHtml)
	ctx := core.NewContext(core.ContextConfig{})
	buf := bytes.NewBuffer(nil)
	n, err := node.Render(ctx, buf, core.RenderingInput{Mime: mimeconsts.HTML_CTYPE})
	assert.NoError(t, err)
	assert.Equal(t, len(nodeHtml), n)

	assert.Equal(t, nodeHtml, buf.String())
}
