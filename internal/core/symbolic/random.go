package internal

// A RandomnessSource represents a symbolic RandomnessSource.
type RandomnessSource struct {
	UnassignablePropsMixin
	_ int
}

func (r *RandomnessSource) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *RandomnessSource:
		return true
	default:
		return false
	}
}

func (r *RandomnessSource) Start(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Commit(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Rollback(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Prop(name string) SymbolicValue {
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (r *RandomnessSource) PropertyNames() []string {
	return nil
}

func (r *RandomnessSource) GetGoMethod(name string) (*GoFunction, bool) {
	return &GoFunction{}, false
}

func (r *RandomnessSource) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *RandomnessSource) IsWidenable() bool {
	return false
}

func (r *RandomnessSource) String() string {
	return "random-source"
}

func (r *RandomnessSource) WidestOfType() SymbolicValue {
	return &RandomnessSource{}
}
