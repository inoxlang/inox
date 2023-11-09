package symbolic

import (
	"context"
	"fmt"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

const INITIAL_NO_CHECK_FUEL = 10

type Context struct {
	forkingParent           *Context
	associatedState         *State
	isolatedConcreteContext ConcreteContext

	parent                  *Context
	noCheckFuel             int
	startingConcreteContext ConcreteContext

	hostAliases                         map[string]Value
	namedPatterns                       map[string]Pattern
	namedPatternPositionDefinitions     map[string]parse.SourcePositionRange
	patternNamespaces                   map[string]*PatternNamespace
	patternNamespacePositionDefinitions map[string]parse.SourcePositionRange
	typeExtensions                      []*TypeExtension
}

func NewSymbolicContext(startingConcreteContext, concreteContext ConcreteContext, parentContext *Context) *Context {
	if concreteContext == nil {
		concreteContext = startingConcreteContext
	}
	return &Context{
		startingConcreteContext: startingConcreteContext,
		isolatedConcreteContext: concreteContext,

		parent:      parentContext,
		noCheckFuel: INITIAL_NO_CHECK_FUEL,

		hostAliases:                         make(map[string]Value, 0),
		namedPatterns:                       make(map[string]Pattern, 0),
		namedPatternPositionDefinitions:     make(map[string]parse.SourcePositionRange, 0),
		patternNamespaces:                   make(map[string]*PatternNamespace, 0),
		patternNamespacePositionDefinitions: make(map[string]parse.SourcePositionRange, 0),
	}
}

func (ctx *Context) AddHostAlias(name string, val *Host, ignoreError bool) {
	_, ok := ctx.hostAliases[name]
	if ok && !ignoreError {
		panic(fmt.Errorf("cannot register a host alias more than once: %s", name))
	}
	ctx.hostAliases[name] = val
}

func (ctx *Context) ResolveHostAlias(alias string) Value {
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

func (ctx *Context) CopyHostAliasesIn(destCtx *Context) {
	if ctx.forkingParent != nil {
		ctx.forkingParent.CopyHostAliasesIn(destCtx)
	}

	for name, value := range ctx.hostAliases {
		if _, alreadyDefined := destCtx.hostAliases[name]; alreadyDefined {
			panic(fmt.Errorf("host alias %q already defined", name))
		}
		destCtx.hostAliases[name] = value
	}
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

func (ctx *Context) AllNamedPatternNames() []string {
	return maps.Keys(ctx.namedPatterns)
}

func (ctx *Context) AddNamedPattern(name string, pattern Pattern, ignoreError bool, optDefinitionPosition ...parse.SourcePositionRange) {
	_, ok := ctx.namedPatterns[name]
	if ok && !ignoreError {
		panic(fmt.Errorf("cannot register a pattern more than once: %s", name))
	}

	ctx.namedPatterns[name] = pattern

	if len(optDefinitionPosition) > 0 {
		ctx.namedPatternPositionDefinitions[name] = optDefinitionPosition[0]
	}
}

func (ctx *Context) ForEachPattern(fn func(name string, pattern Pattern, knowPosition bool, position parse.SourcePositionRange)) {
	if ctx.forkingParent != nil {
		ctx.forkingParent.ForEachPattern(fn)
	}
	for k, v := range ctx.namedPatterns {
		pos, knowPosition := ctx.namedPatternPositionDefinitions[k]
		fn(k, v, knowPosition, pos)
	}
}

func (ctx *Context) CopyNamedPatternsIn(destCtx *Context) {
	if ctx.forkingParent != nil {
		ctx.forkingParent.CopyNamedPatternsIn(destCtx)
	}

	ctx.ForEachPattern(func(name string, pattern Pattern, knowPosition bool, position parse.SourcePositionRange) {
		if knowPosition {
			destCtx.AddNamedPattern(name, pattern, false, position)
		} else {
			destCtx.AddNamedPattern(name, pattern, false)
		}
	})
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

func (ctx *Context) AddPatternNamespace(name string, namespace *PatternNamespace, ignoreError bool, optDefinitionPosition ...parse.SourcePositionRange) {
	_, ok := ctx.patternNamespaces[name]
	if ok && !ignoreError {
		panic(fmt.Errorf("cannot register a pattern namespace more than once: %s", name))
	}

	ctx.patternNamespaces[name] = namespace

	if len(optDefinitionPosition) > 0 {
		ctx.patternNamespacePositionDefinitions[name] = optDefinitionPosition[0]
	}
}

func (ctx *Context) ForEachPatternNamespace(fn func(name string, namespace *PatternNamespace, knowPosition bool, position parse.SourcePositionRange)) {
	if ctx.forkingParent != nil {
		ctx.forkingParent.ForEachPatternNamespace(fn)
	}
	for k, v := range ctx.patternNamespaces {
		pos, knowPosition := ctx.patternNamespacePositionDefinitions[k]
		fn(k, v, knowPosition, pos)
	}
}

func (ctx *Context) CopyPatternNamespacesIn(destCtx *Context) {
	if ctx.forkingParent != nil {
		ctx.forkingParent.CopyPatternNamespacesIn(destCtx)
	}
	ctx.ForEachPatternNamespace(func(name string, namespace *PatternNamespace, knowPosition bool, position parse.SourcePositionRange) {
		if knowPosition {
			destCtx.AddPatternNamespace(name, namespace, false, position)
		} else {
			destCtx.AddPatternNamespace(name, namespace, false)
		}
	})
}

func (ctx *Context) AddTypeExtension(extension *TypeExtension) {
	ctx.typeExtensions = append(ctx.typeExtensions, extension)
}

func (ctx *Context) GetExtensions(v Value) (extensions []*TypeExtension) {
	for _, extension := range ctx.typeExtensions {
		if extension.ExtendedPattern.TestValue(v, RecTestCallState{}) {
			extensions = append(extensions, extension)
		}
	}

	slices.SortFunc(extensions, func(a, b *TypeExtension) int {
		if a.ExtendedPattern.Test(b.ExtendedPattern, RecTestCallState{}) {
			return 0
		}
		return 0
	})
	//

	return
}

func (ctx *Context) CopyTypeExtensions(destCtx *Context) {
	for _, extension := range ctx.typeExtensions {
		destCtx.AddTypeExtension(extension)
	}
}

func (ctx *Context) AddSymbolicGoFunctionError(msg string) {
	ctx.associatedState.addSymbolicGoFunctionError(msg)
}

func (ctx *Context) AddSymbolicGoFunctionWarning(msg string) {
	ctx.associatedState.addSymbolicGoFunctionWarning(msg)
}

func (ctx *Context) AddFormattedSymbolicGoFunctionError(format string, args ...any) {
	ctx.associatedState.addSymbolicGoFunctionError(fmt.Sprintf(format, args...))
}

func (ctx *Context) SetSymbolicGoFunctionParameters(parameters *[]Value, names []string) {
	ctx.associatedState.setSymbolicGoFunctionParameters(parameters, names)
}

func (ctx *Context) SetUpdatedSelf(v Value) {
	ctx.associatedState.setUpdatedSelf(v)
}

func (ctx *Context) HasPermission(perm any) bool {
	if ctx.isolatedConcreteContext == nil {
		return false
	}
	return ctx.isolatedConcreteContext.HasPermissionUntyped(perm)
}

func (ctx *Context) HasAPermissionWithKindAndType(kind permkind.PermissionKind, name permkind.InternalPermissionTypename) bool {
	if ctx.isolatedConcreteContext == nil {
		return false
	}
	return ctx.isolatedConcreteContext.HasAPermissionWithKindAndType(kind, name)
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

	data.Extensions = slices.Clone(ctx.typeExtensions)

	return data
}

func (ctx *Context) fork() *Context {
	child := NewSymbolicContext(ctx.startingConcreteContext, ctx.isolatedConcreteContext, ctx.parent)
	child.forkingParent = ctx
	return child
}

type ConcreteContext interface {
	context.Context
	HasPermissionUntyped(perm any) bool
	HasAPermissionWithKindAndType(kind permkind.PermissionKind, typename permkind.InternalPermissionTypename) bool
}

type dummyConcreteContext struct {
	context.Context
}

func (ctx dummyConcreteContext) HasPermissionUntyped(perm any) bool {
	return false
}
func (ctx dummyConcreteContext) HasAPermissionWithKindAndType(kind permkind.PermissionKind, typename permkind.InternalPermissionTypename) bool {
	return false
}
