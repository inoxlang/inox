package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestCreateHTMLNodeFromXMLElement(t *testing.T) {
	t.Parallel()

	t.Run("script tag", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		element := CreateHTMLNodeFromXMLElement(ctx, core.NewRawTextXmlElement("script", nil, "<a>"))

		bytes := Render(ctx, element)
		s := string(bytes.UnderlyingBytes())

		assert.Equal(t, "<script><a></script>", s)
	})

	t.Run("pseudo htmx attribute", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		attributes := []core.XMLAttribute{core.NewXMLAttribute("hx-lazy-load", core.Str("/data"))}
		element := CreateHTMLNodeFromXMLElement(ctx, core.NewXmlElement("div", attributes, nil))

		bytes := Render(ctx, element)
		s := string(bytes.UnderlyingBytes())

		assert.Equal(t, `<div hx-trigger="load" hx-get="/data"></div>`, s)
	})
}
