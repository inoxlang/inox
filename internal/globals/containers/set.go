package internal

import (
	"errors"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

var (
	ErrSetCanOnlyContainRepresentableValues = errors.New("a Set can only contain representable values")
	ErrValueDoesMatchElementPattern         = errors.New("provided value does not match the element pattern")
)

type Set struct {
	elements map[string]core.Value
	config   SetConfig
}

func NewSet(ctx *core.Context, elements core.Iterable, configObject ...*core.Object) *Set {
	config := SetConfig{
		Uniqueness: UniquenessConstraint{
			Type: UniqueRepr,
		},
	}

	if len(configObject) > 0 {
		obj := configObject[0]
		obj.ForEachEntry(func(k string, v core.Value) error {
			switch k {
			case coll_symbolic.SET_CONFIG_ELEMENT_PATTERN_PROP_KEY:
				pattern, ok := v.(core.Pattern)
				if !ok {
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "a pattern is expected"))
				}
				config.Element = pattern
			case coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY:
				switch val := v.(type) {
				case core.Identifier:
					if val == "url" {
						config.Uniqueness.Type = UniqueURL
					} else {
						panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "?"))
					}
				case core.PropertyName:
					config.Uniqueness.Type = UniquePropertyValue
					config.Uniqueness.PropertyName = val
				default:
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "?"))
				}
			default:
				panic(commonfmt.FmtUnexpectedPropInArgX(k, "configuration"))
			}
			return nil
		})
	}

	return NewSetWithConfig(ctx, elements, config)
}

type SetConfig struct {
	Element    core.Pattern //optional
	Uniqueness UniquenessConstraint
}

func (c SetConfig) Equal(ctx *core.Context, otherConfig SetConfig, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if !c.Uniqueness.Equal(otherConfig.Uniqueness) {
		return false
	}

	//TODO: check Repr config
	if (c.Element == nil) != (otherConfig.Element == nil) {
		return false
	}

	return c.Element == nil || c.Element.Equal(ctx, otherConfig.Element, alreadyCompared, depth+1)
}
func NewSetWithConfig(ctx *core.Context, elements core.Iterable, config SetConfig) *Set {
	set := &Set{
		elements: make(map[string]core.Value),
		config:   config,
	}

	it := elements.Iterator(ctx, core.IteratorConfiguration{})
	for it.Next(ctx) {
		e := it.Value(ctx)
		set.Add(ctx, e)
	}

	return set
}

func (set *Set) Add(ctx *core.Context, elem core.Value) {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	key := getUniqueKey(ctx, elem, set.config.Uniqueness)
	set.elements[key] = elem
}

func (set *Set) Remove(ctx *core.Context, elem core.Value) {
	key := getUniqueKey(ctx, elem, set.config.Uniqueness)
	delete(set.elements, key)
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
	return coll_symbolic.SET_PROPNAMES
}

type SetPattern struct {
	config SetConfig

	core.NotCallablePatternMixin
	core.NotClonableMixin
	core.NoReprMixin
}

func NewSetPattern(config SetConfig) *SetPattern {
	return &SetPattern{config: config}
}

func (patt *SetPattern) Test(ctx *core.Context, v core.Value) bool {
	set, ok := v.(*Set)
	if !ok {
		return false
	}

	return patt.config.Equal(ctx, set.config, map[uintptr]uintptr{}, 0)
}
func (p *SetPattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (p *SetPattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplementedYet)
}

func (p *SetPattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}
