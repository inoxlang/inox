package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/stretchr/testify/assert"
)

func TestCreateHTMLNodeFromXMLElement(t *testing.T) {
	testconfig.AllowParallelization(t)

	t.Run("script tag", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		element := CreateHTMLNodeFromXMLElement(ctx, core.NewRawTextXmlElement("script", nil, "<a>"))

		bytes := Render(ctx, element)
		s := string(bytes.UnderlyingBytes())

		assert.Equal(t, "<script><a></script>", s)
	})

	t.Run("pseudo htmx attributes", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("hx-lazy-load", func(t *testing.T) {
			attributes := []core.XMLAttribute{core.NewXMLAttribute("hx-lazy-load", core.String("/data"))}
			element := CreateHTMLNodeFromXMLElement(ctx, core.NewXmlElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-trigger="load" hx-get="/data"></div>`, s)
		})

		t.Run("hx-post-json", func(t *testing.T) {
			attributes := []core.XMLAttribute{core.NewXMLAttribute("hx-post-json", core.String("/data"))}
			element := CreateHTMLNodeFromXMLElement(ctx, core.NewXmlElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-post="/data" hx-ext="json-form"></div>`, s)
		})

		t.Run("hx-patch-json", func(t *testing.T) {
			attributes := []core.XMLAttribute{core.NewXMLAttribute("hx-patch-json", core.String("/data"))}
			element := CreateHTMLNodeFromXMLElement(ctx, core.NewXmlElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-patch="/data" hx-ext="json-form"></div>`, s)
		})

		t.Run("hx-put-json", func(t *testing.T) {
			attributes := []core.XMLAttribute{core.NewXMLAttribute("hx-put-json", core.String("/data"))}
			element := CreateHTMLNodeFromXMLElement(ctx, core.NewXmlElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-put="/data" hx-ext="json-form"></div>`, s)
		})
	})
}
