package dom_ns

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	pseudorand "math/rand"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	_dom_symbolic "github.com/inoxlang/inox/internal/globals/dom_ns/symbolic"

	"github.com/inoxlang/inox/internal/globals/html_ns"

	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

const (
	CHANGE_WATCH_TIMEOUT = 10 * time.Millisecond
	NODE_WATCHER_PERIOD  = time.Millisecond
	RENDERING_GOROUTINE_TICK_INTERVAL

	FORWARDED_EVENTS_KEY      = "forwarded-events"
	LISTENED_EVENTS_ATTR_KEY  = "data-listened-events"
	JOBS_KEY                  = "jobs"
	MODEL_KEY                 = "model"
	USER_RENDERING_CONFIG_KEY = "config"
	ON_CLICK_TOGGLE_KEY       = "on-click-toggle"

	FORWARDER_ID_DATA_ATTR     = "data-forwarder-id"
	FORWARDER_ID_BITSIZE       = 32
	FORWARDER_ID_ENCODING_BASE = 16
	ID_BITSIZE                 = 32
	ID_ENCODING_BASE           = 16

	S_NODE_ALREADY_HAS_A_PARENT                    = "node that already has a parent"
	S_NODE_ALREADY_HAS_SIBLINGS                    = "node that already has siblings"
	S_CHILDREN_ALREADY_PROVIDED_WITH_CHILDREN_PROP = "children already provided with .children"
)

var (
	ErrRenderFailureModelNotRenderable       = errors.New("failed to render node: model is not renderable")
	ErrRenderFailureModelNotRenderableToHTML = errors.New("failed to render node: model is not renderable to HTML")
	ErrSeveralModelsProvided                 = errors.New("several models have been provided, a single implicit argument is expected OR a .model property")

	NODE_PROPNAMES = []string{"first-child", "data"}

	renderingGoroutineStarted atomic.Bool
	renderingTaskQueue        = make(chan renderingTask, 100)
	taskCancellation          = make(chan struct{})

	_ = []core.GoValue{(*Node)(nil)}
	_ = []core.PotentiallySharable{(*Node)(nil)}
	_ = []core.Watchable{(*Node)(nil)}
	_ = []core.SystemGraphNodeValue{(*Node)(nil)}
)

func init() {
	startRenderingGoroutine()

	// register the renderer for ValueHistory

	core.RegisterRenderer(
		reflect.TypeOf((*core.ValueHistory)(nil)),
		func(ctx *core.Context, w io.Writer, renderable core.Renderable, config core.RenderingInput) (int, error) {
			history := renderable.(*core.ValueHistory)
			fn := history.RenderCurrentToHTMLFn()
			lastValue := history.LastValue(ctx)
			result, err := fn.Call(ctx.GetClosestState(), nil, []core.Value{lastValue}, nil)
			if err != nil {
				return 0, fmt.Errorf("failed to render value history: rendering of last value in history: %w", err)
			}

			switch r := result.(type) {
			case *html_ns.HTMLNode:
				return r.Render(ctx, w, core.RenderingInput{Mime: config.Mime})
			default:
				return 0, fmt.Errorf("failed to render value history: rendering of last value in history returned a value of type %T", r)
			}
		},
	)

	// register symbolic version of Go functions

	symbolicElement := func(ctx *symbolic.Context, tag *symbolic.String, desc *symbolic.Object) *_dom_symbolic.Node {
		var model symbolic.SymbolicValue = symbolic.Nil
		desc.ForEachEntry(func(k string, v symbolic.SymbolicValue) error {
			switch k {
			case MODEL_KEY:
				model = v
			}
			return nil
		})
		return _dom_symbolic.NewDomNode(model)
	}

	core.RegisterSymbolicGoFunctions([]any{
		NewNode, symbolicElement,
		NewAutoNode, func(ctx *symbolic.Context, desc *symbolic.Object) *_dom_symbolic.Node {
			var model symbolic.SymbolicValue
			var err bool
			desc.ForEachEntry(func(k string, v symbolic.SymbolicValue) error {
				switch {
				case core.IsIndexKey(k) || k == MODEL_KEY:
					if model != nil {
						err = true
						return nil
					}
					model = v
				case k == USER_RENDERING_CONFIG_KEY:
				case k == html_ns.CLASS_KEY:
					if _, ok := v.(symbolic.StringLike); !ok {
						ctx.AddSymbolicGoFunctionError(commonfmt.FmtInvalidValueForPropXOfArgY(k, "description", "a string was expected").Error())
					}
				default:
					ctx.AddSymbolicGoFunctionError(commonfmt.FmtUnexpectedPropInArgX(k, "description").Error())
				}
				return nil
			})

			if err {
				ctx.AddSymbolicGoFunctionError(ErrSeveralModelsProvided.Error())
			}
			if model == nil {
				ctx.AddSymbolicGoFunctionError("missing model argument")
				model = symbolic.ANY
			}

			return _dom_symbolic.NewDomNode(model)
		},
	})

	specifcTagFactory := func(ctx *symbolic.Context, desc *symbolic.Object) *_dom_symbolic.Node {
		return symbolicElement(ctx, &symbolic.String{}, desc)
	}

	for _, fn := range []any{_a, _div, _ul, _ol, _li, _span, _svg, _h1, _h2, _h3, _h4} {
		core.RegisterSymbolicGoFunction(fn, specifcTagFactory)
	}
}

type NodeKind int8

const (
	DefaultNodeKind NodeKind = iota
	AutoNode
	StaticNode
)

type Node struct {
	core.NoReprMixin
	core.NotClonableMixin

	lock sync.Mutex

	kind           NodeKind
	html           *html_ns.HTMLNode
	originalDesc   html_ns.NodeDescription //only set for node of default kind
	view           *View
	attachedToView atomic.Bool

	children []*Node
	parent   *Node

	// state
	forwardedEvents   []string
	listenedEvents    []string
	forwarderId       uint64
	jobs              []*core.LifetimeJob
	model             core.Value
	modelReaction     modelReaction
	modelReactionData core.Value
	watchers          *core.ValueWatchers
	mutationCallbacks *core.MutationCallbacks
	sysgraph          core.SystemGraphPointer

	// auto rendering
	lastDomNode               *Node
	lastDomNodeChangeCallback core.CallbackHandle
	userRenderingConfig       core.Value
	additionalClass           string
}

func NewNode(ctx *core.Context, tag core.Str, desc *core.Object) *Node {
	var jobs []*core.LifetimeJob
	var forwardedEvents []string
	var listenedEvents []string
	var model core.Value = core.Nil
	var children []*Node
	var htmlChildren []*html_ns.HTMLNode

	var modelReaction modelReaction
	var modelReactionData core.Value
	var class string

	addChild := func(v core.Value) {
		var child *Node
		var html *html_ns.HTMLNode

		switch val := v.(type) {
		case core.StringLike:
			html = html_ns.CreateTextNode(val)
			child = NewStaticNode(html)
		case *html_ns.HTMLNode:
			child = NewStaticNode(val)
		case *Node:
			if val.view != nil {
				panic(errors.New("failed to create DOM node: one of the children nodes already has an associated view"))
			}
			child = val
		default:
			child = NewAutoNode(ctx, core.NewObjectFromMap(core.ValMap{"0": val}, ctx))
		}
		html = child.html
		if html == nil {
			html = html_ns.CreateSpanElem(core.Str("[lazy]"))
		}
		htmlChildren = append(htmlChildren, html)
		children = append(children, child)
	}

	it := desc.Iterator(ctx, core.IteratorConfiguration{})

	//first iteration: non-index keys
	for it.Next(ctx) {
		k := string(it.Key(ctx).(core.Str))
		if core.IsIndexKey(k) {
			continue
		}

		v := it.Value(ctx)
		switch k {
		case html_ns.CHILDREN_KEY:
			iterable, ok := v.(core.Iterable)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(html_ns.CHILDREN_KEY, "description", "iterable", v))
			}
			it := iterable.Iterator(ctx, core.IteratorConfiguration{})
			for it.Next(ctx) {
				elem := it.Value(ctx)

				child, ok := elem.(*html_ns.HTMLNode)
				if ok {
					if child.HasParent() {
						panic(core.FmtUnexpectedElementInPropIterable("children", S_NODE_ALREADY_HAS_A_PARENT))
					}

					if child.HasPrevSibling() || child.HasNextSibling() {
						panic(core.FmtUnexpectedElementInPropIterable("children", S_NODE_ALREADY_HAS_SIBLINGS))
					}
				}

				addChild(elem)
			}
		case JOBS_KEY:
			iterable, ok := v.(core.Iterable)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(JOBS_KEY, "description", "iterable", v))
			}
			it := iterable.Iterator(ctx, core.IteratorConfiguration{})
			for it.Next(ctx) {
				elem := it.Value(ctx)
				job, ok := elem.(*core.LifetimeJob)
				if !ok {
					panic(core.FmtUnexpectedElementInPropIterableShowVal(elem, JOBS_KEY))
				}
				jobs = append(jobs, job)
			}
		case html_ns.CLASS_KEY:
			class = v.(core.StringLike).GetOrBuildString()
		case MODEL_KEY:
			model = v
		case FORWARDED_EVENTS_KEY:
			iterable, ok := v.(core.Iterable)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(FORWARDED_EVENTS_KEY, "description", "iterable", v))
			}
			it := iterable.Iterator(ctx, core.IteratorConfiguration{})
			for it.Next(ctx) {
				elem := it.Value(ctx)
				ident, ok := elem.(core.Identifier)
				if !ok {
					panic(core.FmtUnexpectedElementInPropIterableShowVal(elem, FORWARDED_EVENTS_KEY))
				}
				forwardedEvents = append(forwardedEvents, string(ident))
			}
		case ON_CLICK_TOGGLE_KEY:
			propName, ok := v.(core.PropertyName)
			if !ok {
				panic(core.FmtPropOfArgXShouldBeOfTypeY(ON_CLICK_TOGGLE_KEY, "description", "property-name", v))
			}
			modelReaction = toggleOnClick
			modelReactionData = propName

			listenedEvents = append(listenedEvents, "click")
		default:
			panic(commonfmt.FmtUnexpectedPropInArgX(k, "description"))
		}
	}

	listenedEvents = append(listenedEvents, forwardedEvents...)
	childrenAlreadyProvided := len(children) != 0
	length := desc.Len()

	//second iteration: get children at index keys
	if length > 0 {
		for i := 0; i < int(length); i++ {
			k := strconv.Itoa(i)
			v := desc.Prop(ctx, k)

			if childrenAlreadyProvided {
				panic(core.FmtUnexpectedElementAtIndeKeyXofArg(k, "description", S_CHILDREN_ALREADY_PROVIDED_WITH_CHILDREN_PROP))
			}

			childNode, ok := v.(*html_ns.HTMLNode)
			if ok {
				if childNode.HasParent() {
					panic(core.FmtUnexpectedElementAtIndeKeyXofArg(k, "description", S_NODE_ALREADY_HAS_A_PARENT))
				}

				if childNode.HasPrevSibling() || childNode.HasNextSibling() {
					panic(core.FmtUnexpectedElementAtIndeKeyXofArg(k, "description", S_NODE_ALREADY_HAS_SIBLINGS))
				}
			}

			addChild(v)
		}
	}

	htmlDesc := html_ns.NodeDescription{
		Tag:      string(tag),
		Children: htmlChildren,
		Class:    class,
	}

	node := &Node{
		kind:         DefaultNodeKind,
		originalDesc: htmlDesc,

		model:             model,
		modelReaction:     modelReaction,
		modelReactionData: modelReactionData,

		jobs:            jobs,
		children:        children,
		forwardedEvents: forwardedEvents,
		listenedEvents:  listenedEvents,
		forwarderId:     0,
	}

	return node
}

func NewStaticNode(n *html_ns.HTMLNode) *Node {
	return &Node{
		html:  n,
		model: core.Nil,
		kind:  StaticNode,
	}
}

func NewAutoNode(ctx *core.Context, desc *core.Object) *Node {
	node := &Node{kind: AutoNode}

	desc.ForEachEntry(func(k string, v core.Value) error {
		switch {
		case core.IsIndexKey(k) || k == MODEL_KEY:
			if node.model != nil {
				panic(ErrSeveralModelsProvided)
			}
			node.model = v
		case k == USER_RENDERING_CONFIG_KEY:
			node.userRenderingConfig = v
			if watchable, ok := node.userRenderingConfig.(core.Watchable); ok {
				watchable.OnMutation(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
					registerAgain = true
					node.addModelRenderTask()
					return
				}, core.MutationWatchingConfiguration{Depth: core.IntermediateDepthWatching})
			}
		case k == html_ns.CLASS_KEY:
			node.additionalClass = v.(core.StringLike).GetOrBuildString()
		default:
			panic(commonfmt.FmtUnexpectedPropInArgX(k, "description"))
		}

		return nil
	})

	if node.model == nil {
		panic(errors.New("missing model argument"))
	}

	if watchable, ok := node.model.(core.Watchable); ok {
		watchable.OnMutation(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
			registerAgain = true
			node.addModelRenderTask()
			return
		}, core.MutationWatchingConfiguration{Depth: core.ShallowWatching})
	}

	return node
}

func (n *Node) initHTMLNode(ctx *core.Context) {
	if n.html != nil {
		return //aready initialized
	}

	switch n.kind {
	case DefaultNodeKind:
		desc := n.originalDesc

		for i, child := range n.children {
			child.lock.Lock()
			child.initHTMLNode(ctx)
			child.parent = n
			desc.Children[i] = child.html
			child.lock.Unlock()
		}

		n.html = html_ns.NewNodeFromGoDescription(desc)

		// add attributes & id
		{
			if len(n.listenedEvents) > 0 {
				n.html.SetAttribute(ctx, html.Attribute{
					Key: LISTENED_EVENTS_ATTR_KEY,
					Val: strings.Join(n.listenedEvents, ","),
				})

				for n.forwarderId == 0 {
					n.forwarderId = pseudorand.Uint64() >> (64 - FORWARDER_ID_BITSIZE)
				}

				forwarderId := strconv.FormatInt(int64(n.forwarderId), ID_ENCODING_BASE)
				n.html.SetAttribute(ctx, html.Attribute{Key: FORWARDER_ID_DATA_ATTR, Val: forwarderId})
			}

			n.html.Walk(func(n html_ns.HTMLNode) error {
				if !n.HasId() {
					id := pseudorand.Uint64() >> (64 - ID_BITSIZE)
					idStr := strconv.FormatInt(int64(id), ID_ENCODING_BASE)
					n.SetId(ctx, core.Str(idStr))
				}
				return nil
			})
		}
	case AutoNode:
		n.modelRenderNoLock(ctx)
	}

}

type modelReaction int

const (
	toggleOnClick modelReaction = iota + 1
)

func (n *Node) attachToView(ctx *core.Context, v *View) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.view == v {
		return
	}
	if n.view != nil {
		panic(errors.New("node is already attached to another view"))
	}

	n.attachedToView.Store(true)
	n.view = v

	for _, child := range n.children {
		child.attachToView(ctx, v)
	}

}

func (n *Node) detachFromView() {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.view = nil
	n.attachedToView.Store(false)

	for _, child := range n.children {
		child.detachFromView()
	}

}

func (n *Node) hasView() bool {
	n.lock.Lock()
	defer n.lock.Unlock()

	return n.view != nil
}

func (n *Node) addModelRenderTask() {
	if n.view == nil {
		return
	}
	view := n.view
	addRenderingTask(renderingTask{
		node: n,
		fn: func() {
			n.modelRender(view.ctx)
		},
	})
}

func (n *Node) modelRender(ctx *core.Context) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.modelRenderNoLock(ctx)
}

func (n *Node) modelRenderNoLock(ctx *core.Context) {
	var parsed *html_ns.HTMLNode

	unwrapped := core.Unwrap(ctx, n.model)
	renderable, ok := unwrapped.(core.Renderable)

	if _, isNil := unwrapped.(core.NilT); isNil {
		parsed = html_ns.CreateSpanElem(core.Str(""))
		goto update
	}

	if !ok {
		panic(ErrRenderFailureModelNotRenderable)
	}

	// if the model is a node we attach it to the view and we listen for changes
	if domNode, ok := renderable.(*Node); ok {
		_, err := domNode.OnMutation(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
			if n.lastDomNode != domNode { // fix (data race)
				//domNode.detachFromView() //TODO: fix (this causes issues)
				return false
			}
			n.addModelRenderTask()
			registerAgain = true
			return
		}, core.MutationWatchingConfiguration{Depth: core.ShallowWatching})
		if err != nil {
			panic(err)
		}
		n.lastDomNode = domNode
		domNode.attachToView(ctx, n.view)
	}

	//render to HTML
	{
		var userRenderingConfig = n.userRenderingConfig
		if dyn, ok := n.userRenderingConfig.(*core.DynamicValue); ok {
			userRenderingConfig = dyn.Resolve(ctx)
		}

		renderingInput := core.RenderingInput{Mime: core.HTML_CTYPE, OptionalUserConfig: userRenderingConfig}

		if !renderable.IsRecursivelyRenderable(ctx, renderingInput) {
			panic(fmt.Errorf("%w: renderable type: %T", ErrRenderFailureModelNotRenderableToHTML, renderable))
		}

		buf := bytes.NewBuffer(nil)
		_, err := renderable.Render(ctx, buf, renderingInput)
		if err != nil {
			panic(err)
		}

		parsed, err = html_ns.ParseSingleNodeHTML(buf.String())
		if err != nil {
			parsed = html_ns.CreateSpanElem(core.Str("???"))
		}
	}

	if n.additionalClass != "" {
		parsed.AppendToAttribute(ctx, html.Attribute{Key: "class", Val: n.additionalClass})
	}

update:
	// update with new HTML

	if n.html == nil { // first render
		n.html = parsed
	} else {
		prevHTML := n.html
		n.html = parsed

		//update parent
		if n.parent != nil {
			n.parent.replaceChildHTML(ctx, prevHTML, n)
		}

		//inform watchers & microtasks about the update

		mutation := core.NewUnspecifiedMutation(core.ShallowWatching, "")

		n.watchers.InformAboutAsync(ctx, mutation, core.ShallowWatching, true)
		n.mutationCallbacks.CallMicrotasks(ctx, mutation)
	}
}

func (n *Node) replaceChildHTML(ctx *core.Context, prevHTML *html_ns.HTMLNode, child *Node) {
	n.lock.Lock()
	defer func() {
		n.lock.Unlock()

		if n.parent != nil {
			//TODO: inform parent ?
		}
	}()

	n.html.ReplaceChildHTML(ctx, prevHTML, child.html)
	//inform watchers & microtasks about the update

	mutation := core.NewUnspecifiedMutation(core.ShallowWatching, "")

	n.watchers.InformAboutAsync(ctx, mutation, core.ShallowWatching, true)
	n.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (n *Node) Render(ctx *core.Context, w io.Writer, config core.RenderingInput) (int, error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.html == nil {
		n.initHTMLNode(ctx)
	}

	return n.html.Render(ctx, w, config)
}

func (n *Node) SendDOMEventToForwader(ctx *core.Context, forwarderId uint64, eventData *core.Record, t time.Time) {
	if forwarderId == 0 { // invalid id
		return
	}

	logger := ctx.Logger()

	if n.forwarderId == forwarderId {
		// forward event to model
		eventType, ok := eventData.Prop(ctx, "type").(core.Str)
		if ok {
			if utils.SliceContains(n.forwardedEvents, string(eventType)) {
				receiver, ok := n.model.(core.MessageReceiver)
				if ok {
					logger.Print("forward", eventType, "dom event to model")
					err := receiver.ReceiveMessage(ctx, core.NewMessage(core.NewEvent(eventData, core.Date(t)), nil))
					if err != nil {
						logger.Print(err)
					}
				}
			} else if utils.SliceContains(n.listenedEvents, string(eventType)) {
				switch n.modelReaction {
				case toggleOnClick:
					propName := string(n.modelReactionData.(core.PropertyName))
					iprops, ok := n.model.(core.IProps)
					if ok {
						val, ok := iprops.Prop(ctx, propName).(core.Bool)
						if ok {
							iprops.SetProp(ctx, propName, !val)
						}
					}
				}
			}
		}
	} else {
		for _, child := range n.children {
			child.SendDOMEventToForwader(ctx, forwarderId, eventData, t)
		}
	}
}

func (n *Node) ProposeSystemGraph(ctx *core.Context, g *core.SystemGraph, proposedName string, optionalParent core.SystemGraphNodeValue) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if !n.sysgraph.Set(g.Ptr()) {
		return
	}

	if optionalParent == nil {
		g.AddNode(ctx, n, proposedName)
	} else {
		g.AddChildNode(ctx, optionalParent, n, proposedName, core.EdgeWatched)
	}

	for i, child := range n.children {
		child.ProposeSystemGraph(ctx, g, strconv.Itoa(i), n)
	}
}

func (n *Node) SystemGraph() *core.SystemGraph {
	return n.sysgraph.Graph()
}

func (n *Node) AddSystemGraphEvent(ctx *core.Context, text string) {
	n.sysgraph.AddEvent(ctx, text, n)
}

func (n *Node) Watcher(ctx *core.Context, config core.WatcherConfiguration) core.Watcher {
	if config.Depth >= core.IntermediateDepthWatching {
		panic(core.ErrIntermediateDepthWatchingNotSupported)
	}

	n.lock.Lock()
	defer n.lock.Unlock()

	watcher := core.NewPeriodicWatcher(config, NODE_WATCHER_PERIOD)

	if n.watchers == nil {
		n.watchers = core.NewValueWatchers()
	}

	n.watchers.Add(watcher)

	_, err := n.onMutationNoLock(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
		if watcher.IsStopped() {
			registerAgain = false
			return
		}

		registerAgain = true

		if !config.Filter.Test(ctx, mutation) {
			return
		}

		watcher.InformAboutAsync(ctx, core.NewUnspecifiedMutation(core.ShallowWatching, config.Path))
		return
	}, core.MutationWatchingConfiguration{Depth: core.ShallowWatching})

	if err != nil {
		panic(err)
	}

	return watcher
}

func (n *Node) onMutationNoLock(ctx *core.Context, microtask core.MutationCallbackMicrotask, config core.MutationWatchingConfiguration) (core.CallbackHandle, error) {
	if config.Depth >= core.IntermediateDepthWatching {
		panic(core.ErrIntermediateDepthWatchingNotSupported)
	}

	if n.mutationCallbacks == nil {
		n.mutationCallbacks = core.NewMutationCallbackMicrotasks()
	}

	return n.mutationCallbacks.AddMicrotask(microtask, config), nil
}

func (n *Node) OnMutation(ctx *core.Context, microtask core.MutationCallbackMicrotask, config core.MutationWatchingConfiguration) (core.CallbackHandle, error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	return n.onMutationNoLock(ctx, microtask, config)
}

func (node *Node) RemoveMutationCallbackMicrotasks(ctx *core.Context) {
	node.mutationCallbacks.RemoveMicrotasks()
}

func (node *Node) RemoveMutationCallback(ctx *core.Context, handle core.CallbackHandle) {
	node.mutationCallbacks.RemoveMicrotask(handle)
}

func (n *Node) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (n *Node) Share(originState *core.GlobalState) {

}

func (n *Node) IsShared() bool {
	return true
}

func (n *Node) ForceLock() {

}

func (n *Node) ForceUnlock() {

}

func (n *Node) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "first-child":
		if len(n.children) == 0 {
			return core.Nil
		}
		return n.children[0] //NewStaticNode(html_ns.NewHTMLNode(n.html.Node.FirstChild))
	case "data":
		return core.Str(n.html.Data())
	default:
		method, ok := n.GetGoMethod(name)
		if !ok {
			panic(core.FormatErrPropertyDoesNotExist(name, n))
		}
		return method
	}
}

func (*Node) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (n *Node) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (n *Node) PropertyNames(ctx *core.Context) []string {
	return NODE_PROPNAMES
}

func (n *Node) IsRecursivelyRenderable(ctx *core.Context, input core.RenderingInput) bool {
	return input.Mime == core.HTML_CTYPE
}

type renderingTask struct {
	fn           func()
	node         *Node
	creationDate time.Time
}

func addRenderingTask(t renderingTask) {
	t.creationDate = time.Now()
	renderingTaskQueue <- t
}

func startRenderingGoroutine() {
	if !renderingGoroutineStarted.CompareAndSwap(false, true) {
		return
	}

	// TODO: scale to N queues & N rendering routines

	go func() {
		lastRenderingTimes := map[*Node]time.Time{}

		for {
			select {
			case <-taskCancellation:
				for len(renderingTaskQueue) > 0 {
					<-renderingTaskQueue
				}
			case task := <-renderingTaskQueue:

				// if the node is no long attached to a view de don't need to render it
				if !task.node.attachedToView.Load() {
					delete(lastRenderingTimes, task.node)
					continue
				}

				// if a rendering was submited before the the last rendering time we ignore it
				if lastRenderingTime, ok := lastRenderingTimes[task.node]; ok && task.creationDate.Before(lastRenderingTime) {
					fmt.Println("ignore old rendering task")
					continue
				}

				lastRenderingTimes[task.node] = time.Now()

				func() {
					defer func() {
						err := recover()
						_ = err
					}()
					task.fn()
				}()
			}
		}

	}()
}
