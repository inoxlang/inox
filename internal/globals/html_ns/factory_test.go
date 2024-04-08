package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/stretchr/testify/assert"
)

func TestCreateHTMLNodeFromMarkupElement(t *testing.T) {
	testconfig.AllowParallelization(t)

	t.Run("interpolation with a Go string value", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		element := CreateHTMLNodeFromMarkupElement(ctx, core.NewMarkupElement("div", nil, []core.Value{
			core.Host("https://localhost"),
		}))

		bytes := Render(ctx, element)
		s := string(bytes.UnderlyingBytes())

		assert.Equal(t, "<div>https://localhost</div>", s)
	})

	t.Run("attribute with a Go string value", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		attrs := []core.MarkupAttribute{core.NewMarkupAttribute("a", core.Host("https://localhost"))}
		element := CreateHTMLNodeFromMarkupElement(ctx, core.NewMarkupElement("div", attrs, nil))

		bytes := Render(ctx, element)
		s := string(bytes.UnderlyingBytes())

		assert.Equal(t, `<div a="https://localhost"></div>`, s)
	})

	t.Run("script tag", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		element := CreateHTMLNodeFromMarkupElement(ctx, core.NewRawTextMarkupElement("script", nil, "<a>"))

		bytes := Render(ctx, element)
		s := string(bytes.UnderlyingBytes())

		assert.Equal(t, "<script><a></script>", s)
	})

	t.Run("script tag with hyperscript marker", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		attributes := []core.MarkupAttribute{core.NewMarkupAttribute("h", core.String(""))}
		element := CreateHTMLNodeFromMarkupElement(ctx, core.NewRawTextMarkupElement("script", attributes, "<a>"))

		bytes := Render(ctx, element)
		s := string(bytes.UnderlyingBytes())

		assert.Equal(t, "<script type=\"text/hyperscript\"><a></script>", s)
	})

	t.Run("pseudo htmx attributes", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		t.Run("hx-lazy-load", func(t *testing.T) {
			attributes := []core.MarkupAttribute{core.NewMarkupAttribute("hx-lazy-load", core.String("/data"))}
			element := CreateHTMLNodeFromMarkupElement(ctx, core.NewMarkupElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-trigger="load" hx-get="/data"></div>`, s)
		})

		t.Run("hx-post-json", func(t *testing.T) {
			attributes := []core.MarkupAttribute{core.NewMarkupAttribute("hx-post-json", core.String("/data"))}
			element := CreateHTMLNodeFromMarkupElement(ctx, core.NewMarkupElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-post="/data" hx-ext="json-form"></div>`, s)
		})

		t.Run("hx-patch-json", func(t *testing.T) {
			attributes := []core.MarkupAttribute{core.NewMarkupAttribute("hx-patch-json", core.String("/data"))}
			element := CreateHTMLNodeFromMarkupElement(ctx, core.NewMarkupElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-patch="/data" hx-ext="json-form"></div>`, s)
		})

		t.Run("hx-put-json", func(t *testing.T) {
			attributes := []core.MarkupAttribute{core.NewMarkupAttribute("hx-put-json", core.String("/data"))}
			element := CreateHTMLNodeFromMarkupElement(ctx, core.NewMarkupElement("div", attributes, nil))

			bytes := Render(ctx, element)
			s := string(bytes.UnderlyingBytes())

			assert.Equal(t, `<div hx-put="/data" hx-ext="json-form"></div>`, s)
		})
	})
}
