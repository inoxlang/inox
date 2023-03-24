package internal

import (
	"fmt"

	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"
)

var (
	AST_NODE_PROPNAMES = []string{"position", "token_at_position"}

	ANY_AST_NODE     = &AstNode{}
	ANY_TOKEN        = &Token{}
	ANY_TOKEN_OR_NIL = NewMultivalue(ANY_TOKEN, Nil)
)

// An AstNode represents a symbolic AstNode.
type AstNode struct {
	Node parse.Node

	UnassignablePropsMixin
}

func (n *AstNode) Test(v SymbolicValue) bool {
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

func (n *AstNode) Widen() (SymbolicValue, bool) {
	if n.Node == nil {
		return nil, false
	}
	return &AstNode{}, true
}

func (n *AstNode) IsWidenable() bool {
	return n.Node != nil
}

func (n *AstNode) String() string {
	if n.Node == nil {
		return "ast-node"
	}
	return fmt.Sprintf("ast-node(%T)", n.Node)
}

func (n *AstNode) WidestOfType() SymbolicValue {
	return ANY_AST_NODE
}

func (r *AstNode) Prop(name string) SymbolicValue {
	switch name {
	case "position":
		return extData.DEFAULT_PATTERN_NAMESPACES["inox"].entries["source_position"].SymbolicValue()
	case "token_at_position":
		return WrapGoClosure(func(ctx *Context, pos *Int) SymbolicValue {
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

func (n *Token) Test(v SymbolicValue) bool {
	_, ok := v.(*Token)
	if !ok {
		return false
	}
	return ok
}

func (t *Token) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (t *Token) IsWidenable() bool {
	return false
}

func (t *Token) String() string {
	return "ast-token"
}

func (t *Token) WidestOfType() SymbolicValue {
	return ANY_TOKEN
}

func (r *Token) Prop(name string) SymbolicValue {
	switch name {
	case "type":
		return ANY_STR_LIKE
	case "rune_count":
		return ANY_INT
	}
	panic(FormatErrPropertyDoesNotExist(name, r))
}

func (*Token) PropertyNames() []string {
	return AST_NODE_PROPNAMES
}
