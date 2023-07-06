package containers

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	jsoniter "github.com/json-iterator/go"
	"github.com/oklog/ulid/v2"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

var (
	ErrSetCanOnlyContainRepresentableValues = errors.New("a Set can only contain representable values")
	ErrValueDoesMatchElementPattern         = errors.New("provided value does not match the element pattern")
	ErrValueWithSameKeyAlreadyPresent       = errors.New("provided value has the same key as an already present element")
)

func init() {
	core.RegisterLoadInstanceFn(reflect.TypeOf((*SetPattern)(nil)), loadSet)
}

type Set struct {
	elements map[string]core.Value
	config   SetConfig

	//persistence
	storage core.SerializedValueStorage //nillable
	url     core.URL                    //set if .storage set
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

func loadSet(ctx *core.Context, path core.Path, storage core.SerializedValueStorage, pattern core.Pattern) (core.UrlHolder, error) {
	setPattern := pattern.(*SetPattern)
	rootData, ok := storage.GetSerialized(ctx, path)
	if !ok {
		return nil, fmt.Errorf("%w: %s", core.ErrFailedToLoadNonExistingValue, path)
	}

	set := NewSetWithConfig(ctx, nil, setPattern.config)
	set.storage = storage
	set.url = storage.BaseURL().AppendAbsolutePath(path)

	var finalErr error

	//TODO: lazy load
	it := jsoniter.ParseString(jsoniter.ConfigCompatibleWithStandardLibrary, rootData)
	it.ReadArrayCB(func(i *jsoniter.Iterator) (cont bool) {
		s := it.ReadAny().ToString()
		val, err := core.ParseRepr(ctx, []byte(s))
		if err != nil {
			finalErr = err
			return false
		}

		defer func() {
			e := recover()

			if err, ok := e.(error); ok {
				finalErr = err
			} else if e != nil {
				cont = false
				finalErr = fmt.Errorf("%#v", e)
			}
		}()
		set.Add(ctx, val)
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	return set, nil
}

func persistSet(ctx *core.Context, set *Set, path core.Path, storage core.SerializedValueStorage, pattern core.Pattern) error {
	buff := bytes.NewBufferString("[")

	first := true
	for _, e := range set.elements {
		if !first {
			buff.WriteByte(',')
		}
		first = false

		if err := core.WriteRepresentation(buff, e, &core.ReprConfig{AllVisible: true}, ctx); err != nil {
			return err
		}
	}

	buff.WriteByte(']')

	storage.SetSerialized(ctx, path, buff.String())
	return nil
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

	if elements != nil {
		it := elements.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			set.Add(ctx, e)
		}
	}

	return set
}

func (set *Set) URL() (core.URL, bool) {
	if set.storage != nil {
		return set.url, true
	}
	return "", false
}

func (set *Set) SetURLOnce(ctx *core.Context, url core.URL) error {
	return core.ErrValueDoesNotAcceptURL
}

func (set *Set) Has(ctx *core.Context, elem core.Value) core.Bool {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	key := getUniqueKey(ctx, elem, set.config.Uniqueness)
	_, ok := set.elements[key]
	return core.Bool(ok)
}

func (set *Set) Add(ctx *core.Context, elem core.Value) {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	if set.config.Uniqueness.Type == UniqueURL {
		holder, ok := elem.(core.UrlHolder)
		if !ok {
			panic(errors.New("elements should be URL holders"))
		}

		_, ok = holder.URL()
		if !ok {
			if set.storage == nil {
				panic(ErrFailedGetUniqueKeyNoURL)
			}

			//if the Set is persisted & the elements are unique by URL
			//we set the url of the new element to set.url + '/' + random ID

			url := set.url.ToDirURL().AppendAbsolutePath(core.Path("/" + ulid.Make().String()))
			utils.PanicIfErr(holder.SetURLOnce(ctx, url))
		}
	}

	key := getUniqueKey(ctx, elem, set.config.Uniqueness)

	curr, ok := set.elements[key]
	if ok && elem != curr {
		panic(ErrValueWithSameKeyAlreadyPresent)
	}
	set.elements[key] = elem
}

func (set *Set) Remove(ctx *core.Context, elem core.Value) {
	key := getUniqueKey(ctx, elem, set.config.Uniqueness)
	delete(set.elements, key)
}

func (f *Set) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "has":
		return core.WrapGoMethod(f.Has), true
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

	core.CallBasedPatternReprMixin

	core.NotCallablePatternMixin
	core.NotClonableMixin
}

func NewSetPattern(config SetConfig, callData core.CallBasedPatternReprMixin) *SetPattern {
	return &SetPattern{
		config:                    config,
		CallBasedPatternReprMixin: callData,
	}
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
