package internal

// A TestSuite represents a symbolic TestSuite.
type TestSuite struct {
	UnassignablePropsMixin
	_ int
}

func (s *TestSuite) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *TestSuite:
		return true
	default:
		return false
	}
}

func (s *TestSuite) Run(ctx *Context, options ...Option) (*Routine, *Error) {
	return &Routine{}, nil
}

func (s *TestSuite) WidestOfType() SymbolicValue {
	return &TestSuite{}
}

func (s *TestSuite) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "run":
		return &GoFunction{fn: s.Run}, true
	}
	return &GoFunction{}, false
}

func (s *TestSuite) Prop(name string) SymbolicValue {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*TestSuite) PropertyNames() []string {
	return []string{"run"}
}

func (s *TestSuite) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *TestSuite) IsWidenable() bool {
	return false
}

func (s *TestSuite) String() string {
	return "%test-suite"
}

// A TestCase represents a symbolic TestCase.
type TestCase struct {
	UnassignablePropsMixin
	_ int
}

func (s *TestCase) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *TestCase:
		return true
	default:
		return false
	}
}

func (s *TestCase) WidestOfType() SymbolicValue {
	return &TestCase{}
}

func (s *TestCase) GetGoMethod(name string) (*GoFunction, bool) {
	return &GoFunction{}, false
}

func (s *TestCase) Prop(name string) SymbolicValue {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*TestCase) PropertyNames() []string {
	return nil
}

func (s *TestCase) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *TestCase) IsWidenable() bool {
	return false
}

func (s *TestCase) String() string {
	return "%test-case"
}
