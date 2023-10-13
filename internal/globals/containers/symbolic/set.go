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
	_ = []symbolic.PotentiallySharable{(*Set)(nil)}
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
	element        symbolic.SymbolicValue //cache

	uniqueness *containers_common.UniquenessConstraint
	shared     bool

	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
}

func NewSet(ctx *symbolic.Context, elements symbolic.Iterable, config ...*symbolic.Object) *Set {
	var patt symbolic.Pattern = symbolic.ANY_PATTERN
	var uniqueness *containers_common.UniquenessConstraint = &containers_common.UniquenessConstraint{
		Type: containers_common.UniqueRepr,
	}

	if len(config) > 0 {
		configObject := config[0]

		val, _, hasElemPattern := configObject.GetProperty(SET_CONFIG_ELEMENT_PATTERN_PROP_KEY)

		if hasElemPattern {
			pattern, ok := val.(symbolic.Pattern)
			if !ok {
				err := commonfmt.FmtInvalidValueForPropXOfArgY(SET_CONFIG_ELEMENT_PATTERN_PROP_KEY, "configuration", "a pattern is expected")
				ctx.AddSymbolicGoFunctionError(err.Error())
			} else {
				patt = pattern
			}
		}

		val, _, hasUniquenessConstraint := configObject.GetProperty(SET_CONFIG_UNIQUE_PROP_KEY)
		if hasUniquenessConstraint {
			u, err := containers_common.UniquenessConstraintFromSymbolicValue(val, patt)
			if err != nil {
				err := commonfmt.FmtInvalidValueForPropXOfArgY(SET_CONFIG_UNIQUE_PROP_KEY, "configuration", err.Error())
				ctx.AddSymbolicGoFunctionError(err.Error())
			} else {
				uniqueness = &u
			}
		}
	}

	return NewSetWithPattern(patt, uniqueness)
}

func NewSetWithPattern(elementPattern symbolic.Pattern, uniqueness *containers_common.UniquenessConstraint) *Set {
	set := &Set{elementPattern: elementPattern, uniqueness: uniqueness}
	set.element = elementPattern.SymbolicValue()
	return set
}

func (s *Set) Test(v symbolic.SymbolicValue, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherSet, ok := v.(*Set)
	if !ok || !s.elementPattern.Test(otherSet.elementPattern, state) {
		return false
	}

	return s.uniqueness == nil || s.uniqueness == otherSet.uniqueness
}

func (s *Set) IsSharable() (bool, string) {
	return true, ""
}

func (s *Set) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	shared := *s
	shared.shared = true
	if psharablbe, ok := shared.element.(symbolic.PotentiallySharable); ok {
		shared.element = psharablbe.Share(originState)
	}
	return &shared
}

func (s *Set) IsShared() bool {
	return s.shared
}

// it should NOT modify the value and should instead return a copy of the value but shared.

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

func (s *Set) Has(ctx *symbolic.Context, v symbolic.Serializable) *symbolic.Bool {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.element,
	}, SET_ADD_METHOD_PARAM_NAMES)
	return symbolic.ANY_BOOL
}

func (s *Set) Add(ctx *symbolic.Context, v symbolic.Serializable) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.element,
	}, SET_ADD_METHOD_PARAM_NAMES)
}

func (s *Set) Remove(ctx *symbolic.Context, v symbolic.Serializable) {
	ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
		s.element,
	}, SET_ADD_METHOD_PARAM_NAMES)
}

func (s *Set) Get(ctx *symbolic.Context, k symbolic.StringLike) (symbolic.SymbolicValue, *symbolic.Bool) {
	return s.element, symbolic.ANY_BOOL
}

func (s *Set) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%Set(")))
	s.element.PrettyPrint(w, config, depth, parentIndentCount)
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
	return s.element
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

func NewSetPatternWithElementPatternAndUniqueness(elementPattern symbolic.Pattern, uniqueness *containers_common.UniquenessConstraint) *SetPattern {
	return &SetPattern{elementPattern: elementPattern, uniqueness: uniqueness}
}

func (p *SetPattern) MigrationInitialValue() (symbolic.Serializable, bool) {
	return symbolic.EMPTY_LIST, true
}

func (p *SetPattern) Test(v symbolic.SymbolicValue, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*SetPattern)
	if !ok || !p.elementPattern.Test(otherPattern.elementPattern, state) {
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

func (p *SetPattern) Concretize(ctx symbolic.ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(symbolic.ErrNotConcretizable)
	}

	concreteElementPattern := utils.Must(symbolic.Concretize(p.elementPattern, ctx))
	return externalData.CreateConcreteSetPattern(*p.uniqueness, concreteElementPattern)
}

func (p *SetPattern) TestValue(v symbolic.SymbolicValue, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	if otherPatt, ok := v.(*SetPattern); ok {
		return p.elementPattern.TestValue(otherPatt.elementPattern, state)
	}
	return false
	//TODO: test nodes's value
}

func (p *SetPattern) HasUnderlyingPattern() bool {
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
