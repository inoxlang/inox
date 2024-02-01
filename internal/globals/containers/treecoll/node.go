package treecoll

import (
	"errors"
	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/utils"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

// TODO: store tree nodes in a pool
type TreeNode struct {
	data     core.Value
	children []*TreeNode // TODO: use pool + make copy on write if tree is shared (see .Prop & tree node + tree iterator)
	tree     *Tree
}

func (n *TreeNode) AddChild(ctx *core.Context, childData core.Value) {
	state := ctx.GetClosestState()

	n.tree._lock(state)
	defer n.tree._unlock(state)

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
	n.tree._lock(state)
	defer n.tree._unlock(state)

	switch name {
	case "data":
		return n.data
	case "children":
		i := -1

		children := n.children

		if n.tree.IsShared() {
			children = slices.Clone(n.children)
		}

		return &common.CollectionIterator{
			HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
				return i < len(children)-1
			},
			Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
				i++
				return true
			},
			Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
				return core.Int(i)
			},
			Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
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
	return coll_symbolic.TREE_NODE_PROPNAMES
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

func (n *TreeNode) SmartLock(state *core.GlobalState) {
	n.tree._lock(state)
}

func (n *TreeNode) SmartUnlock(state *core.GlobalState) {
	n.tree._unlock(state)
}

func (n TreeNode) IsMutable() bool {
	return true
}

func (n *TreeNode) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNode, ok := other.(*TreeNode)
	return ok && n == otherNode
}
