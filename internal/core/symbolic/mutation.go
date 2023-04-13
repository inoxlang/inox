package internal

var (
	ANY_MUTATION = &Mutation{}
)

// An Mutation represents a symbolic Mutation.
type Mutation struct {
	_ int
}

func (r *Mutation) Test(v SymbolicValue) bool {
	_, ok := v.(Iterable)

	return ok
}

func (r *Mutation) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Mutation) IsWidenable() bool {
	return false
}

func (r *Mutation) String() string {
	return "%mutation"
}

func (r *Mutation) WidestOfType() SymbolicValue {
	return ANY_MUTATION
}
