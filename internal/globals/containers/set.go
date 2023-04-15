package internal

import (
	"errors"

	core "github.com/inoxlang/inox/internal/core"
)

var (
	ErrSetCanOnlyContainRepresentableValues = errors.New("a Set can only contain representable values")
)

type Set struct {
	elements   map[string]core.Value
	reprConfig *core.ReprConfig
}

func NewSet(ctx *core.Context, elements core.Iterable) *Set {
	set := &Set{
		elements:   make(map[string]core.Value),
		reprConfig: &core.ReprConfig{},
	}

	it := elements.Iterator(ctx, core.IteratorConfiguration{})
	for it.Next(ctx) {
		e := it.Value(ctx)
		if !e.HasRepresentation(map[uintptr]int{}, set.reprConfig) {
			panic(ErrSetCanOnlyContainRepresentableValues)
		}

		repr := string(core.MustGetRepresentationWithConfig(e, set.reprConfig, ctx)) // representation is context-dependent -> possible issues
		set.elements[repr] = e
	}

	return set
}

func (set *Set) Add(ctx *core.Context, elem core.Value) {
	repr := string(core.MustGetRepresentationWithConfig(elem, set.reprConfig, ctx))
	set.elements[repr] = elem
}

func (set *Set) Remove(ctx *core.Context, elem core.Value) {
	repr := string(core.MustGetRepresentationWithConfig(elem, set.reprConfig, ctx))
	delete(set.elements, repr)
}

func (f *Set) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "add":
		return core.WrapGoMethod(f.Add), true
	case "remove":
		return core.WrapGoMethod(f.Remove), true
	}
	return nil, false
}

func (s *Set) Prop(ctx *core.Context, name string) core.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*Set) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Set) PropertyNames(ctx *core.Context) []string {
	return []string{"add", "remove"}
}
