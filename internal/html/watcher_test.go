package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

func TestNodeWatcher(t *testing.T) {
	t.Run("SetAttribute", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

		node := utils.Must(ParseSingleNodeHTML("<div></div>"))
		called := false

		_, err := node.OnMutation(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
			registerAgain = true
			called = true

			assert.Equal(t, core.Path("/attributes/id"), mutation.Path)
			return

		}, core.MutationWatchingConfiguration{Depth: core.ShallowWatching})

		if !assert.NoError(t, err) {
			return
		}

		node.SetAttribute(ctx, html.Attribute{Key: "id", Val: "x"})

		if !assert.True(t, called) {
			return
		}
	})

	t.Run("RemoveAttribute", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

		node := utils.Must(ParseSingleNodeHTML("<div id=\"x\"></div>"))
		called := false

		_, err := node.OnMutation(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
			registerAgain = true
			called = true

			assert.Equal(t, core.Path("/attributes/id"), mutation.Path)
			return

		}, core.MutationWatchingConfiguration{Depth: core.ShallowWatching})

		if !assert.NoError(t, err) {
			return
		}

		node.RemoveAttribute(ctx, "id")

		if !assert.True(t, called) {
			return
		}
	})

	t.Run("ReplaceChildHTML", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

		node := utils.Must(ParseSingleNodeHTML("<div id=\"x\"><span></span></div>"))
		called := false

		_, err := node.OnMutation(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
			registerAgain = true
			called = true

			assert.Equal(t, core.Path("/children/0"), mutation.Path)
			return

		}, core.MutationWatchingConfiguration{Depth: core.ShallowWatching})

		if !assert.NoError(t, err) {
			return
		}
		childNode := NewHTMLNode(node.node.FirstChild)
		newChildNode := utils.Must(ParseSingleNodeHTML("<span>x</span>"))

		node.ReplaceChildHTML(ctx, childNode, newChildNode)

		if !assert.True(t, called) {
			return
		}
	})
}
