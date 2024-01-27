package core

import (
	"unicode/utf8"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

// An AstNode is an immutable Value wrapping an AST node.
type AstNode struct {
	Node  parse.Node
	chunk *parse.ParsedChunkSource
}

// Chunk returns the parsed chunk the node is part of.
func (n AstNode) Chunk() *parse.ParsedChunkSource {
	return n.chunk
}

func (AstNode) PropertyNames(ctx *Context) []string {
	return symbolic.AST_NODE_PROPNAMES
}

func (n AstNode) Prop(ctx *Context, name string) Value {
	switch name {
	case "position":
		pos := n.chunk.GetSourcePosition(n.Node.Base().Span)
		return createRecordFromSourcePosition(pos)
	case "token-at-position":
		return WrapGoClosure(func(ctx *Context, pos Int) Value {
			token, ok := parse.GetTokenAtPosition(int(pos), n.Node, n.chunk.Node)
			if !ok {
				return Nil
			}
			return Token{value: token}
		})
	default:
		return nil
	}
}

func (AstNode) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

// A Token is an immutable Value wrapping a token.
type Token struct {
	value parse.Token
}

func (Token) PropertyNames(ctx *Context) []string {
	return symbolic.TOKEN_PROPNAMES
}

func (t Token) Prop(ctx *Context, name string) Value {
	switch name {
	case "type":
		return String(t.value.Type.String())
	case "rune-count":
		return Int(utf8.RuneCountInString(t.value.Str()))
	default:
		return nil
	}
}

func (Token) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}
