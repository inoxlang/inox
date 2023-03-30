package internal

// An Walkable represents a symbolic Walkable.
type Walkable interface {
	SymbolicValue
	WalkerElement() SymbolicValue
	WalkerNodeMeta() SymbolicValue
}

// An AnyWalkable represents a symbolic Walkable we do not know the concrete type.
type AnyWalkable struct {
	_ int
}

func (r *AnyWalkable) Test(v SymbolicValue) bool {
	_, ok := v.(*AnyWalkable)

	return ok
}

func (r *AnyWalkable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyWalkable) IsWidenable() bool {
	return false
}

func (r *AnyWalkable) String() string {
	return "walkable"
}

func (r *AnyWalkable) WidestOfType() SymbolicValue {
	return &AnyWalkable{}
}

func (r *AnyWalkable) WalkerElement() SymbolicValue {
	return ANY
}

// A Walker represents a symbolic Walker.
type Walker struct {
	_ int
}

func (r *Walker) Test(v SymbolicValue) bool {
	_, ok := v.(*Walker)

	return ok
}

func (r *Walker) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Walker) IsWidenable() bool {
	return false
}

func (r *Walker) String() string {
	return "walker"
}

func (r *Walker) WidestOfType() SymbolicValue {
	return &Walker{}
}
