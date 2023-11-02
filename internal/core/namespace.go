package core

import (
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
)

type Namespace struct {
	name           string
	entries        map[string]Value
	names          []string
	mutableEntries bool
}

func NewNamespace(name string, entries map[string]Value) *Namespace {
	for entryName, value := range entries {
		if value.IsMutable() {
			panic(fmt.Errorf("failed to create namespace %q: value of entry %q is mutable", name, entryName))
		}
	}

	ns := &Namespace{
		name:    name,
		entries: maps.Clone(entries),
		names:   maps.Keys(entries),
	}

	sort.Strings(ns.names)
	return ns
}

// NewMutableEntriesNamespace creates a namespace that allows the entry values to be modified.
// Adding, removing or assigning an entry is not allowed.
func NewMutableEntriesNamespace(name string, entries map[string]Value) *Namespace {
	ns := &Namespace{
		name:           name,
		entries:        maps.Clone(entries),
		names:          maps.Keys(entries),
		mutableEntries: true,
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
