package core

import (
	"sort"

	"github.com/inoxlang/inox/internal/utils"
)

type Namespace struct {
	name    string
	entries map[string]Value
	names   []string
}

func NewNamespace(name string, entries map[string]Value) *Namespace {
	ns := &Namespace{
		name:    name,
		entries: utils.CopyMap(entries),
		names:   utils.GetMapKeys(entries),
	}

	sort.Strings(ns.names)
	return ns
}

func (ns *Namespace) Prop(ctx *Context, name string) Value {
	for key, value := range ns.entries {
		if key == name {
			return value
		}
	}
	panic(FormatErrPropertyDoesNotExist(name, ns))
}

func (*Namespace) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (ns *Namespace) PropertyNames(ctx *Context) []string {
	return ns.names
}
