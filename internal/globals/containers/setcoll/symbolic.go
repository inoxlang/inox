package setcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

func (s *Set) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	p, err := s.config.Element.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	elementPattern := p.(symbolic.Pattern)
	uniqueness := s.config.Uniqueness
	return coll_symbolic.NewSetWithPattern(elementPattern, &uniqueness), nil
}

func (p *SetPattern) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	var patt symbolic.Pattern = symbolic.ANY_PATTERN

	if p.config.Element != nil {
		p, err := p.config.Element.ToSymbolicValue(ctx, encountered)
		if err != nil {
			return nil, err
		}
		patt = p.(symbolic.Pattern)
	}
	uniqueness := p.config.Uniqueness
	return coll_symbolic.NewSetPatternWithElementPatternAndUniqueness(patt, &uniqueness), nil
}
