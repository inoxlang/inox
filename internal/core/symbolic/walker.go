package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_WALKABLE = &AnyWalkable{}
	ANY_WALKER   = &Walker{}

	_ = []Walkable{(*Path)(nil), (*UData)(nil)}
)

// An Walkable represents a symbolic Walkable.
type Walkable interface {
	Value
	WalkerElement() Value
	WalkerNodeMeta() Value
}

// An AnyWalkable represents a symbolic Walkable we do not know the concrete type.
type AnyWalkable struct {
	_ int
}

func (r *AnyWalkable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*AnyWalkable)

	return ok
}

func (r *AnyWalkable) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("walkable")
}

func (r *AnyWalkable) WidestOfType() Value {
	return ANY_WALKABLE
}

func (r *AnyWalkable) WalkerElement() Value {
	return ANY
}

// A Walker represents a symbolic Walker.
type Walker struct {
	//after any update make sure ANY_WALKER is still valid

	_ int
}

func (r *Walker) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Walker)

	return ok
}

func (r *Walker) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("walker")
}

func (r *Walker) WidestOfType() Value {
	return ANY_WALKER
}
