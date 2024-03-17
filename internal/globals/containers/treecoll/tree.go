package treecoll

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

var (
	_ = []core.PotentiallySharable{(*Tree)(nil)}

	TREE_PATTERN = &core.TypePattern{
		Type:          reflect.TypeOf(&Tree{}),
		Name:          "tree",
		SymbolicValue: &coll_symbolic.Tree{},
	}
	TREE_NODE_PATTERN = &core.TypePattern{
		Type:          reflect.TypeOf(TreeNode{}),
		Name:          "tree.node",
		SymbolicValue: &coll_symbolic.TreeNode{},
		CallImpl: func(ctx *core.Context, typePattern *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
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
			}, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.Value) (symbolic.Pattern, error) {
			return coll_symbolic.NewTreeNodePattern(symbolic.ANY_PATTERN)
		},
	}
	TREE_NODE_PATTERN_PATTERN = &core.TypePattern{
		Type:          reflect.TypeOf(&TreeNodePattern{}),
		Name:          "tree.node-pattern",
		SymbolicValue: coll_symbolic.ANY_TREE_NODE_PATTERN,
	}
)

func init() {
	core.RegisterDefaultPattern("tree", TREE_PATTERN)

	core.RegisterDefaultPatternNamespace("tree", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"node":         TREE_NODE_PATTERN,
			"node-pattern": TREE_NODE_PATTERN_PATTERN,
		},
	})

	core.RegisterPatternDeserializer(TREE_NODE_PATTERN_PATTERN, DeserializeTreeNodePattern)
}

type Tree struct {
	root *TreeNode

	lock core.SmartLock
	jobs *core.ValueLifetimeJobs
}

func NewTree(ctx *core.Context, treedata *core.Treedata, args ...core.Value) *Tree {

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
			data: treedata.Root,
		},
	}

	tree.root.tree = tree
	tree.root.children = make([]*TreeNode, len(treedata.HiearchyEntries))
	stack := []*TreeNode{tree.root}
	ancestorChainLen := 0

	treedata.WalkEntriesDF(func(e core.TreedataHiearchyEntry, index int, ancestorChain *[]core.TreedataHiearchyEntry) error {
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

	state := ctx.MustGetClosestState()

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

func (t *Tree) _lock(state *core.GlobalState) {
	t.lock.Lock(state, t)
}

func (t *Tree) _unlock(state *core.GlobalState) {
	t.lock.Unlock(state, t)
}

func (t *Tree) SmartLock(state *core.GlobalState) {
	t.lock.Lock(state, t, true)
}

func (t *Tree) SmartUnlock(state *core.GlobalState) {
	t.lock.Unlock(state, t, true)
}

func (t *Tree) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (t *Tree) Prop(ctx *core.Context, name string) core.Value {
	state := ctx.MustGetClosestState()
	t._lock(state)
	defer t._unlock(state)

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
	return coll_symbolic.TREE_PROPNAMES
}

func (t *Tree) IsMutable() bool {
	return true
}

func (it *TreeIterator) IsMutable() bool {
	return true
}

func (t *Tree) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherTree, ok := other.(*Tree)
	return ok && t == otherTree
}
