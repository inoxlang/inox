package containers

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	TREE_PROPNAMES      = []string{"root"}
	TREE_NODE_PROPNAMES = []string{"data", "children", "add_child"}
	_                   = []core.PotentiallySharable{(*Tree)(nil)}
)

func init() {
	core.RegisterDefaultPattern("tree", &core.TypePattern{
		Type:          reflect.TypeOf(&Tree{}),
		Name:          "tree",
		SymbolicValue: &coll_symbolic.Tree{},
	})

	core.RegisterDefaultPatternNamespace("tree", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"node": &core.TypePattern{
				Type:          reflect.TypeOf(TreeNode{}),
				Name:          "tree.node",
				SymbolicValue: &coll_symbolic.TreeNode{},
				CallImpl: func(typePattern *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
					var valuePattern core.Pattern

					for _, val := range values {
						switch v := val.(type) {
						case core.Pattern:
							if valuePattern != nil {
								return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("value pattern")
							}

							valuePattern = v
						default:
							if valuePattern == nil {
								valuePattern = core.NewExactValuePattern(v)
								continue
							}
							return nil, core.FmtErrInvalidArgument(v)
						}
					}

					return &TreeNodePattern{
						valuePattern: valuePattern,
						CallBasedPatternReprMixin: core.CallBasedPatternReprMixin{
							Callee: typePattern,
							Params: []core.Serializable{valuePattern},
						},
					}, nil
				},
				SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.Value) (symbolic.Pattern, error) {
					return coll_symbolic.NewTreeNodePattern(symbolic.ANY_PATTERN)
				},
			},
		},
	})
}

type Tree struct {
	root *TreeNode

	lock core.SmartLock
	jobs *core.ValueLifetimeJobs
}

func NewTree(ctx *core.Context, udata *core.UData, args ...core.Value) *Tree {

	//read arguments

	var jobs []*core.LifetimeJob

	for _, arg := range args {
		if job, ok := arg.(*core.LifetimeJob); ok {
			jobs = append(jobs, job)
		} else {
			panic(core.FmtErrInvalidArgument(arg))
		}
	}

	hasLifetimeJobs := len(jobs) != 0

	// construct the tree

	tree := &Tree{
		root: &TreeNode{
			data: udata.Root,
		},
	}

	tree.root.tree = tree
	tree.root.children = make([]*TreeNode, len(udata.HiearchyEntries))
	stack := []*TreeNode{tree.root}
	ancestorChainLen := 0

	udata.WalkEntriesDF(func(e core.UDataHiearchyEntry, index int, ancestorChain *[]core.UDataHiearchyEntry) error {
		if len(*ancestorChain) < ancestorChainLen {
			ancestorChainLen = len(*ancestorChain)
			stack = stack[:ancestorChainLen+1]
		} else {
			ancestorChainLen = len(*ancestorChain)
		}

		parentNode := stack[len(stack)-1]

		node := &TreeNode{
			data: e.Value,
			tree: tree,
		}
		parentNode.children[index] = node

		if len(e.Children) != 0 { // since we do a depth first traversal the next node will be the first child of the current node
			node.children = make([]*TreeNode, len(e.Children))
			stack = append(stack, node)
		}

		return nil
	})

	// instantiate lifetime jobs

	state := ctx.GetClosestState()

	if hasLifetimeJobs {
		if ok, expl := tree.IsSharable(state); !ok {
			panic(errors.New(expl))
		}
		tree.Share(state)
		jobs := core.NewValueLifetimeJobs(ctx, tree, jobs)
		if err := jobs.InstantiateJobs(ctx); err != nil {
			panic(err)
		}
		tree.jobs = jobs
	}

	return tree
}

func (t *Tree) walk(ctx *core.Context, fn func(n *TreeNode) (continue_ bool)) {
	it := t.Iterator(ctx, core.IteratorConfiguration{KeysNeverRead: true})

	for it.Next(ctx) {
		if !fn(it.Value(ctx).(*TreeNode)) {
			break
		}
	}
}

func (t *Tree) IsSharable(originState *core.GlobalState) (bool, string) {
	if t.lock.IsValueShared() {
		return true, ""
	}

	ok := true

	t.walk(originState.Ctx, func(n *TreeNode) bool {
		if sharable, _ := core.IsSharable(n.data, originState); !sharable {
			ok = false
			return false
		}
		return true
	})

	if ok {
		return true, ""
	}
	return false, fmt.Sprintf("tree is not sharable because of it's element is not sharable")
}

func (t *Tree) Share(originState *core.GlobalState) {
	t.lock.Share(originState, func() {
		t.walk(originState.Ctx, func(n *TreeNode) bool {
			if psharable, ok := n.data.(core.PotentiallySharable); ok {
				psharable.Share(originState)
			}
			return true
		})
	})
}

func (t *Tree) IsShared() bool {
	return t.lock.IsValueShared()
}

func (t *Tree) Lock(state *core.GlobalState) {
	t.lock.Lock(state, t)
}

func (obj *Tree) Unlock(state *core.GlobalState) {
	obj.lock.Unlock(state, obj)
}

func (obj *Tree) ForceLock() {
	obj.lock.ForceLock()
}

func (obj *Tree) ForceUnlock() {
	obj.lock.ForceUnlock()
}

func (t *Tree) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (t *Tree) Prop(ctx *core.Context, name string) core.Value {
	state := ctx.GetClosestState()
	t.Lock(state)
	defer t.Unlock(state)

	switch name {
	case "root":
		return t.root
	}
	return core.GetGoMethodOrPanic(name, t)
}

func (*Tree) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Tree) PropertyNames(ctx *core.Context) []string {
	return TREE_PROPNAMES
}

// TODO: store tree nodes in a pool
type TreeNode struct {
	data     core.Value
	children []*TreeNode // TODO: use pool + make copy on write if tree is shared (see .Prop & tree node + tree iterator)
	tree     *Tree
}

func (n *TreeNode) AddChild(ctx *core.Context, childData core.Value) {
	state := ctx.GetClosestState()

	n.tree.Lock(state)
	defer n.tree.Unlock(state)

	if !utils.Ret0(core.IsSharable(childData, state)) {
		panic(core.ErrCannotAddNonSharableToSharedContainer)
	}

	child := &TreeNode{
		data:     childData,
		children: nil,
		tree:     n.tree,
	}
	n.children = append(n.children, child)
}

func (n *TreeNode) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "add_child":
		return core.WrapGoMethod(n.AddChild), true
	}
	return nil, false
}

func (n *TreeNode) Prop(ctx *core.Context, name string) core.Value {
	state := ctx.GetClosestState()
	n.tree.Lock(state)
	defer n.tree.Unlock(state)

	switch name {
	case "data":
		return n.data
	case "children":
		i := -1

		children := n.children

		if n.tree.IsShared() {
			children = slices.Clone(n.children)
		}

		return &CollectionIterator{
			hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
				return i < len(children)-1
			},
			next: func(ci *CollectionIterator, ctx *core.Context) bool {
				i++
				return true
			},
			key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
				return core.Int(i)
			},
			value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
				return children[i]
			},
		}
	}
	return core.GetGoMethodOrPanic(name, n)
}

func (*TreeNode) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*TreeNode) PropertyNames(ctx *core.Context) []string {
	return TREE_NODE_PROPNAMES
}

func (n *TreeNode) IsSharable(originState *core.GlobalState) (bool, string) {
	return n.tree.IsShared(), ""
}

func (n *TreeNode) Share(originState *core.GlobalState) {
	if n.tree.IsShared() {
		return
	}
	panic(errors.New("tree node cannot pass in shared mode by itself, this should be done on the tree"))
}

func (n *TreeNode) IsShared() bool {
	return n.tree.IsShared()
}

func (n *TreeNode) Lock(state *core.GlobalState) {
	n.tree.lock.Lock(state, n.tree)
}

func (n *TreeNode) Unlock(state *core.GlobalState) {
	n.tree.lock.Unlock(state, n.tree)
}

func (n *TreeNode) ForceLock() {
	n.tree.lock.ForceLock()
}

func (n *TreeNode) ForceUnlock() {
	n.tree.lock.ForceUnlock()
}

type TreeNodePattern struct {
	valuePattern core.Pattern
	core.CallBasedPatternReprMixin

	core.NotCallablePatternMixin
}

func (patt *TreeNodePattern) Test(ctx *core.Context, v core.Value) bool {
	node, ok := v.(*TreeNode)
	if !ok {
		return false
	}

	return patt.valuePattern.Test(ctx, node.data)
}

func (patt *TreeNodePattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(errors.New("cannot created random tree node"))
}

func (patt *TreeNodePattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (patt *TreeNodePattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}
