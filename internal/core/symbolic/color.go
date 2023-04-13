package internal

// A Color represents a symbolic Color.
type Color struct {
	UnassignablePropsMixin
	_ int
}

func (r *Color) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *Color:
		return true
	default:
		return false
	}
}

func (r *Color) WidestOfType() SymbolicValue {
	return &Color{}
}

func (r *Color) Prop(name string) SymbolicValue {
	switch name {
	}
	return nil
}

func (*Color) PropertyNames() []string {
	return []string{}
}

func (r *Color) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *Color) IsWidenable() bool {
	return false
}

func (r *Color) String() string {
	return "%color"
}
