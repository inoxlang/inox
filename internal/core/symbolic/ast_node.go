package symbolic

import (
	"github.com/inoxlang/inox/internal/ast"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

var (
	AST_NODE_PROPNAMES = []string{"position", "token-at-position"}
	TOKEN_PROPNAMES    = []string{"type", "rune-count"}

	ANY_AST_NODE     = &AstNode{}
	ANY_TOKEN        = &Token{}
	ANY_TOKEN_OR_NIL = NewMultivalue(ANY_TOKEN, Nil)
)

// An AstNode represents a symbolic AstNode.
type AstNode struct {
	Node ast.Node

	UnassignablePropsMixin
}

func (n *AstNode) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherNode, ok := v.(*AstNode)
	if !ok {
		return false
	}
	if n.Node == nil {
		return true
	} else {
		if otherNode.Node == nil {
			return false
		}
		return utils.SamePointer(n.Node, otherNode.Node)
	}
}

func (n *AstNode) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if n.Node == nil {
		w.WriteName("ast-node")
		return
	}

	w.WriteNameF("ast-node(%T)", n.Node)
}

func (n *AstNode) WidestOfType() Value {
	return ANY_AST_NODE
}

func (r *AstNode) Prop(name string) Value {
	switch name {
	case "position":
		return extData.DEFAULT_PATTERN_NAMESPACES["inox"].entries["source_position"].SymbolicValue()
	case "token-at-position":
		return WrapGoClosure(func(ctx *Context, pos *Int) Value {
			return ANY_TOKEN_OR_NIL
		})
	}

	panic(FormatErrPropertyDoesNotExist(name, r))
}

func (*AstNode) PropertyNames() []string {
	return AST_NODE_PROPNAMES
}

// An Token represents a symbolic Token.
type Token struct {
	_ int
	UnassignablePropsMixin
}

func (n *Token) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Token)
	if !ok {
		return false
	}
	return ok
}

func (t *Token) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("ast-token")
}

func (t *Token) WidestOfType() Value {
	return ANY_TOKEN
}

func (r *Token) Prop(name string) Value {
	switch name {
	case "type":
		return ANY_STR_LIKE
	case "rune-count":
		return ANY_INT
	}
	panic(FormatErrPropertyDoesNotExist(name, r))
}

func (*Token) PropertyNames() []string {
	return TOKEN_PROPNAMES
}
