package containers

import (
	"errors"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	MAP_PROPNAMES                = []string{"insert", "set", "remove", "get"}
	MAP_CONFIG_KEY_PATTERN_KEY   = "key"
	MAP_CONFIG_VALUE_PATTERN_KEY = "value"

	MAP_INSERT_METHOD_PARAM_NAMES = []string{"key", "value"}
	MAP_SET_METHOD_PARAM_NAMES    = []string{"key", "value"}
	MAP_REMOVE_METHOD_PARAM_NAMES = []string{"key"}
	MAP_GET_METHOD_PARAM_NAMES    = []string{"key"}

	ANY_MAP         = NewMapWithPatterns(symbolic.ANY_SERIALIZABLE_PATTERN, symbolic.ANY_SERIALIZABLE_PATTERN)
	ANY_MAP_PATTERN = NewMapPattern(symbolic.ANY_SERIALIZABLE_PATTERN, symbolic.ANY_SERIALIZABLE_PATTERN)

	ErrMapEntryListShouldHaveEvenLength = errors.New(`flat map entry list should have an even length: ["k1", 1,  "k2", 2]`)

	_ = []symbolic.Iterable{(*Map)(nil)}
	_ = []symbolic.Collection{(*Map)(nil)}
	_ = []symbolic.UrlHolder{(*Map)(nil)}

	_ = []symbolic.PotentiallyConcretizable{(*MapPattern)(nil)}
	_ = []symbolic.MigrationInitialValueCapablePattern{(*MapPattern)(nil)}
)

type Map struct {
	keyPattern symbolic.Pattern
	key        symbolic.Serializable //cache

	valuePattern symbolic.Pattern
	value        symbolic.Serializable //cache

	shared bool
	url    *symbolic.URL //can be nil

	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
	symbolic.CollectionMixin
}

func NewMap(ctx *symbolic.Context, flatEntryList symbolic.Indexable, config *symbolic.OptionalParam[*symbolic.Object]) *Map {
	var keyPattern symbolic.Pattern = symbolic.ANY_SERIALIZABLE_PATTERN
	var key symbolic.Serializable = symbolic.ANY_SERIALIZABLE
	var valuePattern symbolic.Pattern = symbolic.ANY_SERIALIZABLE_PATTERN
	var value symbolic.Serializable = symbolic.ANY_SERIALIZABLE

	if flatEntryList != nil && (!flatEntryList.HasKnownLen() || flatEntryList.KnownLen()%2 != 0) {
		panic(ErrMapEntryListShouldHaveEvenLength)
	}

	if config.Value != nil {
		configObject := *config.Value

		v, _, hasKeyPattern := configObject.GetProperty(MAP_CONFIG_KEY_PATTERN_KEY)

		if hasKeyPattern {
			pattern, ok := v.(symbolic.Pattern)
			if !ok {
				err := commonfmt.FmtInvalidValueForPropXOfArgY(MAP_CONFIG_KEY_PATTERN_KEY, "configuration", "a pattern is expected")
				ctx.AddSymbolicGoFunctionError(err.Error())
			} else {
				serializable, ok := symbolic.AsSerializable(pattern.SymbolicValue()).(symbolic.Serializable)
				if ok {
					keyPattern = pattern
					key = serializable
				} else {
					err := commonfmt.FmtInvalidValueForPropXOfArgY(MAP_CONFIG_KEY_PATTERN_KEY, "configuration", "a pattern matching serializable values is expected")
					ctx.AddSymbolicGoFunctionError(err.Error())
				}
			}
		}

		v, _, hasValuePattern := configObject.GetProperty(MAP_CONFIG_VALUE_PATTERN_KEY)

		if hasValuePattern {
			pattern, ok := v.(symbolic.Pattern)
			if !ok {
				err := commonfmt.FmtInvalidValueForPropXOfArgY(MAP_CONFIG_VALUE_PATTERN_KEY, "configuration", "a pattern is expected")
				ctx.AddSymbolicGoFunctionError(err.Error())
			} else {
				serializable, ok := symbolic.AsSerializable(pattern.SymbolicValue()).(symbolic.Serializable)
				if ok {
					valuePattern = pattern
					value = serializable
				} else {
					err := commonfmt.FmtInvalidValueForPropXOfArgY(MAP_CONFIG_VALUE_PATTERN_KEY, "configuration", "a pattern matching serializable values is expected")
					ctx.AddSymbolicGoFunctionError(err.Error())
				}
			}
		}

	}

	return &Map{
		keyPattern:   keyPattern,
		key:          key,
		valuePattern: valuePattern,
		value:        value,
	}
}

func NewMapWithPatterns(keyPattern, valuePattern symbolic.Pattern) *Map {
	set := &Map{
		keyPattern:   keyPattern,
		valuePattern: valuePattern,
	}
	set.key = symbolic.AsSerializableChecked(keyPattern.SymbolicValue())
	set.value = symbolic.AsSerializableChecked(valuePattern.SymbolicValue())
	return set
}

func (m *Map) WithURL(url *symbolic.URL) symbolic.UrlHolder {
	copy := *m
	copy.url = url
	mv, ok := copy.value.(symbolic.IMultivalue)

	elementURL := copy.url.WithAdditionalPathPatternSegment("*")

	if ok {
		transformed := mv.OriginalMultivalue().TransformsValues(func(v symbolic.Value) symbolic.Value {
			if urlHolder, ok := v.(symbolic.UrlHolder); ok {
				return urlHolder.WithURL(elementURL)
			}
			return v
		})
		copy.value = symbolic.AsSerializableChecked(transformed)
	} else if urlHolder, ok := copy.value.(symbolic.UrlHolder); ok {
		copy.value = urlHolder.WithURL(elementURL)
	}
	return &copy
}

func (m *Map) URL() (*symbolic.URL, bool) {
	if m.url != nil {
		return m.url, true
	}
	return nil, false
}

func (m *Map) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherMap, ok := v.(*Map)
	return ok && m.key.Test(otherMap.key, state) && m.value.Test(otherMap.value, state)
}

func (m *Map) IsSharable() (bool, string) {
	return true, ""
}

func (m *Map) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	shared := *m
	shared.shared = true
	if psharable, ok := shared.value.(symbolic.PotentiallySharable); ok {
		shared.value = psharable.Share(originState).(symbolic.Serializable)
	}
	return &shared
}

func (m *Map) IsShared() bool {
	return m.shared
}

func (m *Map) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "insert":
		return symbolic.WrapGoMethod(m.Insert), true
	case "set":
		return symbolic.WrapGoMethod(m.Set), true
	case "remove":
		return symbolic.WrapGoMethod(m.Remove), true
	case "get":
		return symbolic.WrapGoMethod(m.Get), true
	}
	return nil, false
}

func (m *Map) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, m)
}

func (*Map) PropertyNames() []string {
	return MAP_PROPNAMES
}

func (m *Map) Insert(ctx *symbolic.Context, k, v symbolic.Value) *symbolic.Error {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{
		m.key,
		m.value,
	}, MAP_INSERT_METHOD_PARAM_NAMES)
	return nil
}

func (m *Map) Set(ctx *symbolic.Context, k, v symbolic.Value) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{
		m.key,
		m.value,
	}, MAP_SET_METHOD_PARAM_NAMES)
}

func (m *Map) Remove(ctx *symbolic.Context, k symbolic.Value) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{m.key}, MAP_REMOVE_METHOD_PARAM_NAMES)
}

func (m *Map) Get(ctx *symbolic.Context, k symbolic.Value) (symbolic.Value, *symbolic.Bool) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{m.key}, MAP_GET_METHOD_PARAM_NAMES)
	return m.value, symbolic.ANY_BOOL
}

func (m *Map) Contains(value symbolic.Serializable) (yes bool, possible bool) {
	if !m.value.Test(value, symbolic.RecTestCallState{}) {
		return
	}
	possible = true
	return
}

func (*Map) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("map")
}

func (m *Map) IteratorElementKey() symbolic.Value {
	return m.key
}

func (m *Map) IteratorElementValue() symbolic.Value {
	return m.value
}

func (*Map) WidestOfType() symbolic.Value {
	return ANY_MAP
}

type MapPattern struct {
	symbolic.UnassignablePropsMixin
	keyPattern   symbolic.Pattern
	valuePattern symbolic.Pattern

	symbolic.NotCallablePatternMixin
	symbolic.SerializableMixin
}

func NewMapPattern(keyPattern, valuePattern symbolic.Pattern) *MapPattern {
	return &MapPattern{
		keyPattern:   keyPattern,
		valuePattern: valuePattern,
	}
}

func (p *MapPattern) MigrationInitialValue() (symbolic.Serializable, bool) {
	return symbolic.EMPTY_LIST, true
}

func (p *MapPattern) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*MapPattern)
	return ok && p.keyPattern.Test(otherPattern.keyPattern, state) && p.valuePattern.Test(otherPattern.valuePattern, state)
}

func (p *MapPattern) TestValue(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	set, ok := v.(*Map)
	if !ok {
		return false
	}
	return p.keyPattern.Test(set.keyPattern, state) && p.valuePattern.Test(set.valuePattern, state)
}

func (p *MapPattern) IsConcretizable() bool {
	keyPattern, ok1 := p.keyPattern.(symbolic.PotentiallyConcretizable)
	valuePattern, ok2 := p.valuePattern.(symbolic.PotentiallyConcretizable)

	return ok1 && ok2 && keyPattern.IsConcretizable() && valuePattern.IsConcretizable()
}

func (p *MapPattern) Concretize(ctx symbolic.ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(symbolic.ErrNotConcretizable)
	}

	concreteKeyPattern := utils.Must(symbolic.Concretize(p.keyPattern, ctx))
	concreteValuePattern := utils.Must(symbolic.Concretize(p.valuePattern, ctx))

	return externalData.CreateConcreteMapPattern(concreteKeyPattern, concreteValuePattern)
}

func (p *MapPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *MapPattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (p *MapPattern) SymbolicValue() symbolic.Value {
	return NewMapWithPatterns(p.keyPattern, p.valuePattern)
}

func (p *MapPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("set-pattern(")
	p.keyPattern.SymbolicValue().PrettyPrint(w, config)
	w.WriteByte(',')

	p.valuePattern.SymbolicValue().PrettyPrint(w, config)
	w.WriteByte(')')
}

func (*MapPattern) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (*MapPattern) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (*MapPattern) WidestOfType() symbolic.Value {
	return ANY_MAP_PATTERN
}
