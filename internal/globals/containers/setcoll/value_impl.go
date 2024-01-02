package setcoll

import (
	"github.com/inoxlang/inox/internal/core"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

// GoValue and PotentiallySharable impls for Set

func (f *Set) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "has":
		return core.WrapGoMethod(f.Has), true
	case "add":
		return core.WrapGoMethod(f.Add), true
	case "remove":
		return core.WrapGoMethod(f.Remove), true
	case "get":
		return core.WrapGoMethod(f.Get), true
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

func (s *Set) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSet, ok := other.(*Set)
	return ok && s == otherSet
}

func (s *Set) IsMutable() bool {
	return true
}

func (s *Set) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (s *Set) Share(originState *core.GlobalState) {
	s.lock.Share(originState, func() {})
}

func (s *Set) IsShared() bool {
	return s.lock.IsValueShared()
}

func (s *Set) Lock(state *core.GlobalState) {
	s.lock.Lock(state, s)
}

func (s *Set) Unlock(state *core.GlobalState) {
	s.lock.Unlock(state, s)
}

func (s *Set) ForceLock() {
	s.lock.ForceLock()
}

func (s *Set) ForceUnlock() {
	s.lock.ForceUnlock()
}
