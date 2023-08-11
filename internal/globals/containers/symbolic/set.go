package containers

import (
	"bufio"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []symbolic.Iterable{(*Set)(nil)}
	_ = []symbolic.Serializable{(*Set)(nil)}
	_ = []symbolic.PotentiallyConcretizable{(*SetPattern)(nil)}
	_ = []symbolic.MigrationInitialValueCapablePattern{(*SetPattern)(nil)}

	SET_PROPNAMES                       = []string{"has", "add", "remove", "get"}
	SET_CONFIG_ELEMENT_PATTERN_PROP_KEY = "element"
	SET_CONFIG_UNIQUE_PROP_KEY          = "unique"

	SET_ADD_METHOD_PARAM_NAMES = []string{"element"}
	SET_GET_METHOD_PARAM_NAMES = []string{"key"}

	ANY_SET         = NewSetWithPattern(symbolic.ANY_PATTERN, nil)
	ANY_SET_PATTERN = NewSetWithPattern(symbolic.ANY_PATTERN, nil)
)

type Set struct {
	elementPattern symbolic.Pattern
	uniqueness     *containers_common.UniquenessConstraint

	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
}

func NewSet(ctx *symbolic.Context, elements symbolic.Iterable, config ...*symbolic.Object) *Set {
	var patt symbolic.Pattern = symbolic.ANY_PATTERN
	var uniqueness *containers_common.UniquenessConstraint

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
				u, ok := containers_common.UniquenessConstraintFromSymbolicValue(propVal)
				if !ok {
					err := commonfmt.FmtInvalidValueForPropXOfArgY(SET_CONFIG_UNIQUE_PROP_KEY, "configuration", "#url, #repr or a property name is expected")
					ctx.AddSymbolicGoFunctionError(err.Error())
				} else {
					uniqueness = &u
				}
			}

			return nil
		})
	}

	if uniqueness != nil && uniqueness.Type == containers_common.UniquePropertyValue {
		if iprops, ok := patt.(symbolic.IProps); !ok || !symbolic.HasRequiredOrOptionalProperty(iprops, uniqueness.PropertyName.UnderlyingString()) {
			err := commonfmt.FmtInvalidValueForPropXOfArgY(SET_CONFIG_UNIQUE_PROP_KEY, "configuration", "uniqueness is based on the value of a given property but elements do not have such property")
			ctx.AddSymbolicGoFunctionError(err.Error())
		}
	}

	return NewSetWithPattern(patt, uniqueness)
}

func NewSetWithPattern(elementPattern symbolic.Pattern, uniqueness *containers_common.UniquenessConstraint) *Set {
	return &Set{elementPattern: elementPattern, uniqueness: uniqueness}
}

func (s *Set) Test(v symbolic.SymbolicValue) bool {
	otherSet, ok := v.(*Set)
	if !ok || !s.elementPattern.Test(otherSet.elementPattern) {
		return false
	}

	return s.uniqueness == nil || s.uniqueness == otherSet.uniqueness
}

func (s *Set) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "has":
		return symbolic.WrapGoMethod(s.Has), true
	case "add":
		return symbolic.WrapGoMethod(s.Add), true
	case "remove":
		return symbolic.WrapGoMethod(s.Remove), true
	case "get":
		return symbolic.WrapGoMethod(s.Get), true
	}
	return nil, false
}

func (s *Set) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*Set) PropertyNames() []string {
	return SET_PROPNAMES
}

func (s *Set) Has(ctx *symbolic.Context, v symbolic.Serializable) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.elementPattern.SymbolicValue(),
	}, SET_ADD_METHOD_PARAM_NAMES)
}

func (s *Set) Add(ctx *symbolic.Context, v symbolic.Serializable) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.elementPattern.SymbolicValue(),
	}, SET_ADD_METHOD_PARAM_NAMES)
}

func (s *Set) Remove(ctx *symbolic.Context, v symbolic.Serializable) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.elementPattern.SymbolicValue(),
	}, SET_ADD_METHOD_PARAM_NAMES)
}

func (s *Set) Get(ctx *symbolic.Context, k symbolic.StringLike) (symbolic.SymbolicValue, *symbolic.Bool) {
	return s.elementPattern.SymbolicValue(), symbolic.ANY_BOOL
}

func (s *Set) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%Set(")))
	s.elementPattern.SymbolicValue().PrettyPrint(w, config, depth, parentIndentCount)
	if s.uniqueness != nil {
		utils.PanicIfErr(w.WriteByte(','))
		s.uniqueness.ToSymbolicValue().PrettyPrint(w, config, depth, 0)
	}
	utils.Must(w.Write(utils.StringAsBytes(")")))
}

func (*Set) IteratorElementKey() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (s *Set) IteratorElementValue() symbolic.SymbolicValue {
	return s.elementPattern.SymbolicValue()
}

func (*Set) WidestOfType() symbolic.SymbolicValue {
	return ANY_SET
}

type SetPattern struct {
	symbolic.UnassignablePropsMixin
	elementPattern symbolic.Pattern
	uniqueness     *containers_common.UniquenessConstraint

	symbolic.NotCallablePatternMixin
	symbolic.SerializableMixin
}

func NewSetPatternWithElementPattern(elementPattern symbolic.Pattern) *SetPattern {
	return &SetPattern{elementPattern: elementPattern}
}

func NewSetPatternWithElementPatternAndUniqueness(elementPattern symbolic.Pattern, uniqueness *containers_common.UniquenessConstraint) *SetPattern {
	return &SetPattern{elementPattern: elementPattern, uniqueness: uniqueness}
}

func (p *SetPattern) MigrationInitialValue() (symbolic.Serializable, bool) {
	return symbolic.EMPTY_LIST, true
}

func (p *SetPattern) Test(v symbolic.SymbolicValue) bool {
	otherPattern, ok := v.(*SetPattern)
	if !ok || !p.elementPattern.Test(otherPattern.elementPattern) {
		return false
	}

	return p.uniqueness == nil || p.uniqueness == otherPattern.uniqueness
}

func (p *SetPattern) IsConcretizable() bool {
	if p.uniqueness == nil {
		return false
	}
	potentiallyConcretizable, ok := p.elementPattern.(symbolic.PotentiallyConcretizable)
	return ok && potentiallyConcretizable.IsConcretizable()
}

func (p *SetPattern) Concretize() any {
	if !p.IsConcretizable() {
		panic(symbolic.ErrNotConcretizable)
	}

	concreteElementPattern := utils.Must(symbolic.Concretize(p.elementPattern))
	return externalData.CreateConcreteSetPattern(*p.uniqueness, concreteElementPattern)
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

func (p *SetPattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (p *SetPattern) SymbolicValue() symbolic.SymbolicValue {
	return NewSetWithPattern(p.elementPattern, p.uniqueness)
}

func (p *SetPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%set-pattern(")))
	p.elementPattern.SymbolicValue().PrettyPrint(w, config, depth, parentIndentCount)
	if p.uniqueness != nil {
		utils.PanicIfErr(w.WriteByte(','))
		p.uniqueness.ToSymbolicValue().PrettyPrint(w, config, depth, 0)
	}
	utils.Must(w.Write(utils.StringAsBytes(")")))
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
