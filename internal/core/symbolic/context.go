package internal

import (
	"errors"
	"fmt"
)

type Context struct {
	forkingParent   *Context
	associatedState *State

	hostAliases       map[string]SymbolicValue
	namedPatterns     map[string]Pattern
	patternNamespaces map[string]*PatternNamespace
}

func NewSymbolicContext() *Context {
	return &Context{
		hostAliases:       make(map[string]SymbolicValue, 0),
		namedPatterns:     make(map[string]Pattern, 0),
		patternNamespaces: make(map[string]*PatternNamespace, 0),
	}
}

func (ctx *Context) AddHostAlias(name string, val *Host) {
	_, ok := ctx.hostAliases[name]
	if ok {
		panic(errors.New("cannot register a host alias more than once"))
	}
	ctx.hostAliases[name] = val
}

func (ctx *Context) ResolveHostAlias(alias string) interface{} {
	host, ok := ctx.hostAliases[alias]
	if !ok {
		if ctx.forkingParent != nil {
			host, ok := ctx.forkingParent.hostAliases[alias]
			if ok {
				return host
			}
		}
		return nil
	}
	return host
}

func (ctx *Context) ResolveNamedPattern(name string) Pattern {
	pattern, ok := ctx.namedPatterns[name]
	if !ok {
		if ctx.forkingParent != nil {
			return ctx.forkingParent.ResolveNamedPattern(name)
		}
		return nil
	}

	return pattern
}

func (ctx *Context) AddNamedPattern(name string, pattern Pattern) {
	ctx.namedPatterns[name] = pattern
}

func (ctx *Context) ForEachPattern(fn func(name string, pattern Pattern)) {
	if ctx.forkingParent != nil {
		ctx.forkingParent.ForEachPattern(fn)
	}
	for k, v := range ctx.namedPatterns {
		fn(k, v)
	}
}

func (ctx *Context) ResolvePatternNamespace(name string) *PatternNamespace {
	namespace, ok := ctx.patternNamespaces[name]
	if !ok {
		if ctx.forkingParent != nil {
			return ctx.forkingParent.ResolvePatternNamespace(name)
		}
		return nil
	}

	return namespace
}

func (ctx *Context) AddPatternNamespace(name string, namespace *PatternNamespace) {
	ctx.patternNamespaces[name] = namespace
}

func (ctx *Context) ForEachPatternNamespace(fn func(name string, namespace *PatternNamespace)) {
	if ctx.forkingParent != nil {
		ctx.forkingParent.ForEachPatternNamespace(fn)
	}
	for k, v := range ctx.patternNamespaces {
		fn(k, v)
	}
}

func (ctx *Context) AddSymbolicGoFunctionError(msg string) {
	ctx.associatedState.addSymbolicGoFunctionError(msg)
}

func (ctx *Context) AddFormattedSymbolicGoFunctionError(format string, args ...any) {
	ctx.associatedState.addSymbolicGoFunctionError(fmt.Sprintf(format, args...))
}

func (ctx *Context) SetSymbolicGoFunctionParameters(parameters *[]SymbolicValue, names []string) {
	ctx.associatedState.setSymbolicGoFunctionParameters(parameters, names)
}

func (ctx *Context) currentData() (data ContextData) {
	//TODO: share some pieces of data between ContextData values in order to save memor
	//forking makes that non trivial

	for name, pattern := range ctx.namedPatterns {
		data.Patterns = append(data.Patterns, NamedPatternData{name, pattern})
	}

	for name, namespace := range ctx.patternNamespaces {
		data.PatternNamespaces = append(data.PatternNamespaces, PatternNamespaceData{name, namespace})
	}

	return data
}

func (ctx *Context) fork() *Context {
	child := NewSymbolicContext()
	child.forkingParent = ctx
	return child
}
