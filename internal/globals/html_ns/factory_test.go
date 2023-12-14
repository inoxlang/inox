package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestCreateHTMLNodeFromXMLElement(t *testing.T) {
	t.Parallel()

	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	element := CreateHTMLNodeFromXMLElement(ctx, core.NewRawTextXmlElement("script", nil, "<a>"))

	bytes := Render(ctx, element)
	s := string(bytes.UnderlyingBytes())

	assert.Equal(t, "<script><a></script>", s)
}
