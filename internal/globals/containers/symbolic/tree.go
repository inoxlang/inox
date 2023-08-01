package containers

import (
	"bufio"
	"errors"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []symbolic.Iterable{(*Tree)(nil), (*TreeNode)(nil)}
	_ = []symbolic.PotentiallySharable{(*Tree)(nil), (*TreeNode)(nil)}

	ANY_TREE      = NewTree(false)
	ANY_TREE_NODE = NewTreeNode(ANY_TREE)
)

type Tree struct {
	symbolic.UnassignablePropsMixin
	shared   bool
	treeNode *TreeNode
}

func NewTree(shared bool) *Tree {
	t := &Tree{shared: shared}
	t.treeNode = NewTreeNode(t)
	return t
}

func (t *Tree) Test(v symbolic.SymbolicValue) bool {
	otherTree, ok := v.(*Tree)
	return ok && t.shared == otherTree.shared
}

func (t *Tree) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (t *Tree) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "root":
		return NewTreeNode(t)
	}
	return symbolic.GetGoMethodOrPanic(name, t)
}

func (*Tree) PropertyNames() []string {
	return []string{"root"}
}

func (t *Tree) InsertNode(ctx *symbolic.Context, v symbolic.SymbolicValue) *TreeNode {
	return t.treeNode
}

func (t *Tree) RemoveNode(ctx *symbolic.Context, k *TreeNode) {

}

func (t *Tree) Connect(ctx *symbolic.Context, n1, n2 *TreeNode) {

}

func (t *Tree) Get(ctx *symbolic.Context, k symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (t *Tree) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%tree")))
	return
}

func (t *Tree) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Tree) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (t *Tree) WalkerElement() symbolic.SymbolicValue {
	return t.treeNode
}

func (t *Tree) WalkerNodeMeta() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (r *Tree) WidestOfType() symbolic.SymbolicValue {
	return ANY_TREE
}

func (t *Tree) IsSharable() (bool, string) {
	if t.shared {
		return true, ""
	}
	// ?
	return true, ""
}

func (t *Tree) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	if t.shared {
		return t
	}
	shared := &Tree{
		shared: true,
	}

	return shared
}

func (t *Tree) IsShared() bool {
	return t.shared
}

type TreeNode struct {
	symbolic.UnassignablePropsMixin
	tree *Tree
	_    int
}

func NewTreeNode(t *Tree) *TreeNode {
	return &TreeNode{tree: t}
}

func (r *TreeNode) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*TreeNode)
	return ok
}

func (f *TreeNode) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (t *TreeNode) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "data":
		return &symbolic.Any{}
	case "children":
		return &symbolic.Iterator{ElementValue: t}
	case "add_child":
		return symbolic.WrapGoMethod(t.AddChild)
	}
	return symbolic.GetGoMethodOrPanic(name, t)
}

func (*TreeNode) PropertyNames() []string {
	return []string{"data", "children", "add_child"}
}

func (n *TreeNode) AddChild(ctx *symbolic.Context, data symbolic.SymbolicValue) {
	if n.tree.shared {
		if ok, _ := symbolic.IsSharable(data); !ok {
			ctx.AddSymbolicGoFunctionError(symbolic.ErrCannotAddNonSharableToSharedContainer.Error())
		}
	}
}

func (n *TreeNode) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (n *TreeNode) IteratorElementValue() symbolic.SymbolicValue {
	return n
}

func (r *TreeNode) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%tree-node")))
}

func (r *TreeNode) WidestOfType() symbolic.SymbolicValue {
	return ANY_TREE_NODE
}

type TreeNodePattern struct {
	valuePattern symbolic.Pattern

	symbolic.NotCallablePatternMixin
	symbolic.Serializable
}

func NewTreeNodePattern(valuePattern symbolic.Pattern) (*TreeNodePattern, error) {
	return &TreeNodePattern{
		valuePattern: valuePattern,
	}, nil
}

func (patt *TreeNodePattern) Test(v symbolic.SymbolicValue) bool {
	otherPatt, ok := v.(*TreeNodePattern)
	if !ok {
		return false
	}
	return patt.valuePattern.Test(otherPatt.valuePattern)
}

func (p *TreeNodePattern) TestValue(v symbolic.SymbolicValue) bool {
	if _, ok := v.(*TreeNode); ok {
		return true
	}
	return false
	//TODO: test nodes's value
}

func (p *TreeNodePattern) HasUnderylingPattern() bool {
	return true
}

func (p *TreeNodePattern) SymbolicValue() symbolic.SymbolicValue {
	return ANY_TREE_NODE
}

func (p *TreeNodePattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (p *TreeNodePattern) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Int{}
}

func (p *TreeNodePattern) IteratorElementValue() symbolic.SymbolicValue {
	return ANY_TREE_NODE
}

func (p *TreeNodePattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%tree-node-pattern")))
}

func (p *TreeNodePattern) WidestOfType() symbolic.SymbolicValue {
	return &TreeNodePattern{
		valuePattern: symbolic.ANY_PATTERN,
	}
}

func (n *TreeNode) IsSharable() (bool, string) {
	if n.tree.shared {
		return true, ""
	}
	// ?
	return true, ""
}

func (t *TreeNode) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	if t.tree.shared {
		return t
	}

	panic(errors.New("tree node cannot pass in shared mode by itself, this should be done on the tree"))
}

func (t *TreeNode) IsShared() bool {
	return t.tree.shared
}
