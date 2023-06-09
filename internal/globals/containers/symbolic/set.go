package containers

import (
	"bufio"

	"github.com/inoxlang/inox/internal/commonfmt"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []symbolic.Iterable{&Set{}}

	SET_PROPNAMES                       = []string{"add", "remove"}
	SET_CONFIG_ELEMENT_PATTERN_PROP_KEY = "element"
	SET_CONFIG_UNIQUE_PROP_KEY          = "unique"

	SET_ADD_METHOD_PARAM_NAMES = []string{"element"}

	ANY_SET         = NewSetWithPattern(symbolic.ANY_PATTERN)
	ANY_SET_PATTERN = NewSetWithPattern(symbolic.ANY_PATTERN)
)

type Set struct {
	symbolic.UnassignablePropsMixin
	elementPattern symbolic.Pattern
}

func NewSet(ctx *symbolic.Context, elements symbolic.Iterable, config ...*symbolic.Object) *Set {
	var patt symbolic.Pattern = symbolic.ANY_PATTERN

	if len(config) > 0 {
		configObject := config[0]

		configObject.ForEachEntry(func(k string, propVal symbolic.SymbolicValue) error {

			switch k {
			case SET_CONFIG_ELEMENT_PATTERN_PROP_KEY:
				pattern, ok := propVal.(symbolic.Pattern)
				if !ok {
					err := commonfmt.FmtInvalidValueForPropXOfArgY(SET_CONFIG_ELEMENT_PATTERN_PROP_KEY, "configuration", "a pattern is expected")
					ctx.AddSymbolicGoFunctionError(err.Error())
				} else {
					patt = pattern
				}
			case SET_CONFIG_UNIQUE_PROP_KEY:
				ok := false
				switch val := propVal.(type) {
				case *symbolic.PropertyName:
					ok = true
				case *symbolic.Identifier:
					ok = !val.HasConcreteName() || val.Name() == "url"
				}
				if !ok {
					err := commonfmt.FmtInvalidValueForPropXOfArgY(SET_CONFIG_UNIQUE_PROP_KEY, "configuration", "#url or a property name is expected")
					ctx.AddSymbolicGoFunctionError(err.Error())
				}
			}

			return nil
		})
	}
	return NewSetWithPattern(patt)
}

func NewSetWithPattern(elementPattern symbolic.Pattern) *Set {
	return &Set{elementPattern: elementPattern}
}

func (s *Set) Test(v symbolic.SymbolicValue) bool {
	otherSet, ok := v.(*Set)
	if !ok {
		return false
	}

	return s.elementPattern.Test(otherSet.elementPattern)
}

func (s *Set) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "add":
		return symbolic.WrapGoMethod(s.Add), true
	case "remove":
		return symbolic.WrapGoMethod(s.Remove), true
	}
	return nil, false
}

func (s *Set) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*Set) PropertyNames() []string {
	return SET_PROPNAMES
}

func (s *Set) Add(ctx *symbolic.Context, v symbolic.SymbolicValue) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.elementPattern.SymbolicValue(),
	}, SET_ADD_METHOD_PARAM_NAMES)
}

func (s *Set) Remove(ctx *symbolic.Context, v symbolic.SymbolicValue) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.elementPattern.SymbolicValue(),
	}, SET_ADD_METHOD_PARAM_NAMES)
}

func (*Set) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (*Set) IsWidenable() bool {
	return false
}

func (s *Set) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%Set(")))
	s.elementPattern.SymbolicValue().PrettyPrint(w, config, depth, parentIndentCount)
	utils.Must(w.Write(utils.StringAsBytes(")")))
}

func (*Set) IteratorElementKey() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (*Set) IteratorElementValue() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (*Set) WidestOfType() symbolic.SymbolicValue {
	return ANY_SET
}

type SetPattern struct {
	symbolic.UnassignablePropsMixin
	elementPattern symbolic.Pattern

	symbolic.NotCallablePatternMixin
}

func NewSetPatternWithElementPattern(elementPattern symbolic.Pattern) *SetPattern {
	return &SetPattern{elementPattern: elementPattern}
}

func (p *SetPattern) Test(v symbolic.SymbolicValue) bool {
	otherPattern, ok := v.(*SetPattern)
	if !ok {
		return false
	}

	return p.elementPattern.Test(otherPattern.elementPattern)
}

func (p *SetPattern) TestValue(v symbolic.SymbolicValue) bool {
	if otherPatt, ok := v.(*SetPattern); ok {
		return p.elementPattern.TestValue(otherPatt.elementPattern)
	}
	return false
	//TODO: test nodes's value
}

func (p *SetPattern) HasUnderylingPattern() bool {
	return true
}

func (p *SetPattern) StringPattern() (symbolic.StringPatternElement, bool) {
	return nil, false
}

func (p *SetPattern) SymbolicValue() symbolic.SymbolicValue {
	return NewSetWithPattern(p.elementPattern)
}

func (p *SetPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%set-pattern")))
}

func (*SetPattern) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (*SetPattern) IsWidenable() bool {
	return false
}

func (*SetPattern) IteratorElementKey() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (*SetPattern) IteratorElementValue() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (*SetPattern) WidestOfType() symbolic.SymbolicValue {
	return ANY_SET
}
