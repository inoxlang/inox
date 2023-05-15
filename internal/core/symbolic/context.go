package internal

import (
	"errors"
	"fmt"

	parse "github.com/inoxlang/inox/internal/parse"
)

type Context struct {
	forkingParent   *Context
	associatedState *State

	hostAliases                         map[string]SymbolicValue
	namedPatterns                       map[string]Pattern
	namedPatternPositionDefinitions     map[string]parse.SourcePositionRange
	patternNamespaces                   map[string]*PatternNamespace
	patternNamespacePositionDefinitions map[string]parse.SourcePositionRange
}

func NewSymbolicContext() *Context {
	return &Context{
		hostAliases:                         make(map[string]SymbolicValue, 0),
		namedPatterns:                       make(map[string]Pattern, 0),
		namedPatternPositionDefinitions:     make(map[string]parse.SourcePositionRange, 0),
		patternNamespaces:                   make(map[string]*PatternNamespace, 0),
		patternNamespacePositionDefinitions: make(map[string]parse.SourcePositionRange, 0),
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

func (ctx *Context) AddNamedPattern(name string, pattern Pattern, optDefinitionPosition ...parse.SourcePositionRange) {
	ctx.namedPatterns[name] = pattern

	if len(optDefinitionPosition) > 0 {
		ctx.namedPatternPositionDefinitions[name] = optDefinitionPosition[0]
	}
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

func (ctx *Context) AddPatternNamespace(name string, namespace *PatternNamespace, optDefinitionPosition ...parse.SourcePositionRange) {
	ctx.patternNamespaces[name] = namespace

	if len(optDefinitionPosition) > 0 {
		ctx.patternNamespacePositionDefinitions[name] = optDefinitionPosition[0]
	}
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
		data.Patterns = append(data.Patterns, NamedPatternData{
			name,
			pattern,
			ctx.namedPatternPositionDefinitions[name], //ok if zero value
		})
	}

	for name, namespace := range ctx.patternNamespaces {
		data.PatternNamespaces = append(data.PatternNamespaces, PatternNamespaceData{
			name,
			namespace,
			ctx.patternNamespacePositionDefinitions[name], //ok if zero value
		})
	}

	return data
}

func (ctx *Context) fork() *Context {
	child := NewSymbolicContext()
	child.forkingParent = ctx
	return child
}
