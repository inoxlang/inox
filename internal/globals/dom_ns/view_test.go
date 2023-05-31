package dom_ns

import (
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestViewWatcher(t *testing.T) {

	t.Run("mutation of auto node", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("Foo")}, ctx)
		dynVal, _ := core.NewDynamicMemberValue(ctx, obj, "name")

		node := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			"0": dynVal,
		}, ctx))
		node.initHTMLNode(ctx)

		view := NewView(ctx, "/", obj, node)

		watcher := view.Watcher(ctx, core.WatcherConfiguration{Filter: core.MUTATION_PATTERN})
		go func() {
			// trigger a rerender of the node
			obj.SetProp(ctx, "name", core.Str("Bar"))
		}()

		v, err := watcher.WaitNext(ctx, nil, time.Second)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.NewUnspecifiedMutation(0, ""), v)
	})

	t.Run("mutation of child node", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("Foo")}, ctx)
		dynVal, _ := core.NewDynamicMemberValue(ctx, obj, "name")

		node := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			"0": NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": dynVal}, ctx)),
		}, ctx))
		node.initHTMLNode(ctx)

		view := NewView(ctx, "/", obj, node)

		watcher := view.Watcher(ctx, core.WatcherConfiguration{Filter: core.MUTATION_PATTERN})
		go func() {
			// trigger a rerender of the child node
			obj.SetProp(ctx, "name", core.Str("Bar"))
		}()

		v, err := watcher.WaitNext(ctx, nil, time.Second)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.NewUnspecifiedMutation(0, ""), v)
	})

}
