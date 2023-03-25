package internal

import (
	"log"
	"os"
	"testing"
	"time"

	core "github.com/inox-project/inox/internal/core"
	_html "github.com/inox-project/inox/internal/globals/html"
	parse "github.com/inox-project/inox/internal/parse"

	"github.com/inox-project/inox/internal/utils"

	"github.com/stretchr/testify/assert"
)

func TestNewNode(t *testing.T) {

	t.Run("empty description", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		result := NewNode(ctx, core.Str("div"), core.NewObject())

		//remove ids
		result.html.Walk(func(n _html.HTMLNode) error {
			n.RemoveAttribute(ctx, "id")
			return nil
		})

		assert.Equal(t, &Node{
			html:  _html.NewNode(ctx, "div", core.NewObject()),
			model: core.Nil,
		}, result)
	})

	t.Run("description with .jobs", func(t *testing.T) {
		state := core.NewGlobalState(core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{core.RoutinePermission{Kind_: core.CreatePerm}},
		}))
		chunk := parse.MustParseChunk("")
		state.Module = &core.Module{MainChunk: &parse.ParsedChunk{Node: chunk}}

		jobChunk := &parse.ParsedChunk{
			Node:   chunk,
			Source: parse.InMemorySource{NameString: "test"},
		}

		jobMod := &core.Module{
			MainChunk:  jobChunk,
			ModuleKind: core.LifetimeJobModule,
		}

		job := utils.Must(core.NewLifetimeJob(core.Str("job"), NODE_PATTERN, jobMod, state))

		result := NewNode(state.Ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			JOBS_KEY: core.NewWrappedValueList(job),
		}, state.Ctx))
		assert.NotNil(t, result)
		assert.Equal(t, []*core.LifetimeJob{job}, result.jobs)
	})

	t.Run("description with string-like value", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		result := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			"0": core.Str("hello"),
		}, ctx))

		assert.Len(t, result.children, 1)
		child := result.children[0]
		assert.Equal(t, core.Nil, child.model)
	})

	t.Run("description with dynamic string-like value", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("Foo")}, ctx)
		dynVal, _ := core.NewDynamicMemberValue(ctx, obj, "name")

		result := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			"0": dynVal,
		}, ctx))

		assert.Len(t, result.children, 1)
		child := result.children[0]
		assert.Same(t, dynVal, child.model)
	})

	t.Run("description with model", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		model := core.NewObject()
		result := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			MODEL_KEY: model,
		}, ctx))

		assert.Same(t, model, result.model)
	})

	t.Run("description with forwarded events", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		result := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			FORWARDED_EVENTS_KEY: core.NewWrappedValueList(core.Identifier("keypress")),
		}, ctx))

		assert.Equal(t, []string{"keypress"}, result.forwardedEvents)
		assert.Equal(t, "keypress", result.html.AttrOrEmpty(LISTENED_EVENTS_ATTR_KEY))
	})
}

func TestNewAutoNode(t *testing.T) {

	t.Run("boolean", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		result := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": core.True}, ctx))

		assert.Equal(t, &Node{
			html:  _html.CreateTextNode(core.Str("true")),
			model: core.True,
		}, result)
	})

	t.Run("value with user provided rendering config", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		config := core.NewDateFormat(time.RFC822)
		date, _ := time.Parse(time.RFC822, time.RFC822)
		result := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": core.Date(date), "config": config}, ctx))

		dateString := time.RFC822

		result.modelChangeWatcher = nil
		result.renderingConfigChangeWatcher = nil
		result.userRenderingConfig = nil

		assert.Equal(t, &Node{
			html:  _html.CreateTimeElem(core.Str(dateString)),
			model: core.Date(date),
		}, result)
	})

	t.Run("empty list", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		result := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": core.NewWrappedValueList()}, ctx))

		assert.Equal(t, "<ul></ul>", string(_html.RenderToString(ctx, result)))
	})

	t.Run("dynamic if with only simple values", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		obj := core.NewObjectFromMap(core.ValMap{"cond": core.True}, ctx)
		cond, _ := core.NewDynamicMemberValue(ctx, obj, "cond")
		model := core.NewDynamicIf(ctx, cond, core.True, core.False)

		result := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": model}, ctx))

		assert.Equal(t, "true", string(_html.RenderToString(ctx, result)))
		obj.SetProp(ctx, "cond", core.False)
		time.Sleep(time.Millisecond)
		assert.Equal(t, "false", string(_html.RenderToString(ctx, result)))
	})

	t.Run("dynamic if", func(t *testing.T) {

		t.Run("simple value | nil", func(t *testing.T) {
			ctx := core.NewContext(core.ContextConfig{})
			core.NewGlobalState(ctx)

			obj := core.NewObjectFromMap(core.ValMap{"cond": core.True}, ctx)
			cond, _ := core.NewDynamicMemberValue(ctx, obj, "cond")
			model := core.NewDynamicIf(ctx, cond, core.True, core.Nil)

			result := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": model}, ctx))

			assert.Equal(t, "true", string(_html.RenderToString(ctx, result)))
			obj.SetProp(ctx, "cond", core.False)
			time.Sleep(time.Millisecond)
			assert.Equal(t, "<span></span>", string(_html.RenderToString(ctx, result)))
		})

		t.Run("nil | static DOM node", func(t *testing.T) {
			ctx := core.NewContext(core.ContextConfig{})
			core.NewGlobalState(ctx)

			obj := core.NewObjectFromMap(core.ValMap{"cond": core.True}, ctx)
			cond, _ := core.NewDynamicMemberValue(ctx, obj, "cond")
			childNode := NewStaticNode(_html.CreateSpanElem(core.Str("a")))
			model := core.NewDynamicIf(ctx, cond, core.Nil, childNode)

			result := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": model}, ctx))

			assert.Equal(t, "<span></span>", string(_html.RenderToString(ctx, result)))
			obj.SetProp(ctx, "cond", core.False)
			time.Sleep(time.Millisecond)
			assert.Equal(t, "<span>a</span>", string(_html.RenderToString(ctx, result)))
		})

		t.Run("nil | auto DOM node", func(t *testing.T) {
			ctx := core.NewContext(core.ContextConfig{})
			core.NewGlobalState(ctx)

			childNodeModel := core.NewWrappedValueList()
			childNode := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": childNodeModel}, ctx))

			obj := core.NewObjectFromMap(core.ValMap{"cond": core.True}, ctx)
			cond, _ := core.NewDynamicMemberValue(ctx, obj, "cond")
			model := core.NewDynamicIf(ctx, cond, core.Nil, childNode)

			result := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": model}, ctx))

			assert.Equal(t, "<span></span>", string(_html.RenderToString(ctx, result)))
			obj.SetProp(ctx, "cond", core.False)
			time.Sleep(time.Millisecond)
			assert.Equal(t, "<ul></ul>", string(_html.RenderToString(ctx, result)))
		})

	})

}

func TestNodeWatcher(t *testing.T) {

	t.Run("mutation of child node's model (auto node)", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("Foo")}, ctx)
		dynModel, _ := core.NewDynamicMemberValue(ctx, obj, "name")

		node := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			"0": dynModel,
		}, ctx))

		watcher := node.Watcher(ctx, core.WatcherConfiguration{Filter: core.MUTATION_PATTERN})
		go func() {
			// trigger a rerender of the node
			obj.SetProp(ctx, "name", core.Str("Bar"))
		}()

		v, err := watcher.WaitNext(ctx, nil, time.Second)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.NewUnspecifiedMutation(core.ShallowWatching, ""), v)
	})

	t.Run("mutation of child node", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("Foo")}, ctx)
		dynVal, _ := core.NewDynamicMemberValue(ctx, obj, "name")

		node := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			"0": NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": dynVal}, ctx)),
		}, ctx))

		watcher := node.Watcher(ctx, core.WatcherConfiguration{Filter: core.MUTATION_PATTERN})
		go func() {
			// trigger a rerender of the child node
			obj.SetProp(ctx, "name", core.Str("Bar"))
		}()

		v, err := watcher.WaitNext(ctx, nil, time.Second)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.NewUnspecifiedMutation(core.ShallowWatching, ""), v)
	})

	t.Run("mutation of auto node's rendering config", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)

		model := core.Date(utils.Must(time.Parse(time.RFC822, time.RFC822)))
		obj := core.NewObjectFromMap(core.ValMap{"config": core.NewDateFormat(time.RFC850)}, ctx)
		dynConfig, _ := core.NewDynamicMemberValue(ctx, obj, "config")

		node := NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"model": model, "config": dynConfig}, ctx))

		watcher := node.Watcher(ctx, core.WatcherConfiguration{Filter: core.MUTATION_PATTERN})
		go func() {
			// trigger a rerender of the node
			obj.SetProp(ctx, "config", core.NewDateFormat(time.RFC1123))
		}()

		v, err := watcher.WaitNext(ctx, nil, time.Second)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.NewUnspecifiedMutation(core.ShallowWatching, ""), v)
	})

}

//TODO: add OnMutation tests

func TestSendDOMEventToForwader(t *testing.T) {

	t.Run("send to forwarder", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		state := core.NewGlobalState(ctx)
		state.Out = os.Stdout
		state.Logger = log.Default()

		model := core.NewObjectFromMap(core.ValMap{"name": core.Str("Foo")}, ctx)

		node := NewNode(ctx, core.Str("div"), core.NewObjectFromMap(core.ValMap{
			MODEL_KEY:            model,
			FORWARDED_EVENTS_KEY: core.NewWrappedValueList(core.Identifier("click")),
		}, ctx))

		now := time.Now()
		eventData := core.NewRecordFromMap(core.ValMap{"type": core.Str("click")})

		// we create a watcher to see received values
		watcher := model.Watcher(ctx, core.WatcherConfiguration{
			Filter: core.ANYVAL_PATTERN,
		})

		go func() {
			node.SendDOMEventToForwader(ctx, node.forwarderId, eventData, now)
		}()

		msg, err := watcher.WaitNext(ctx, nil, 20*time.Second)
		if !assert.NoError(t, err) {
			return
		}

		assert.IsType(t, core.Message{}, msg)
		assert.Equal(t, core.NewEvent(eventData, core.Date(now)), msg.(core.Message).Data())
	})

}
