package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

const (
	TEST_ITEM_META__NAME_PROPNAME     = "name"
	TEST_ITEM_META__FS_PROPNAME       = "fs"
	TEST_ITEM_META__PASS_LIVE_FS_COPY = "pass-live-fs-copy-to-subtests"
)

var (
	TEST_ITEM__EXPECTED_META_VALUE = NewMultivalue(ANY_STR_LIKE, NewInexactRecord(map[string]Serializable{
		TEST_ITEM_META__NAME_PROPNAME:     ANY_STR_LIKE,
		TEST_ITEM_META__FS_PROPNAME:       ANY_FS_SNAPSHOT_IL,
		TEST_ITEM_META__PASS_LIVE_FS_COPY: ANY_BOOL,
	}, nil))
)

// A TestSuite represents a symbolic TestSuite.
type TestSuite struct {
	UnassignablePropsMixin
	_ int
}

func (s *TestSuite) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *TestSuite:
		return true
	default:
		return false
	}
}

func (s *TestSuite) Run(ctx *Context, options ...Option) (*LThread, *Error) {
	return &LThread{}, nil
}

func (s *TestSuite) WidestOfType() Value {
	return &TestSuite{}
}

func (s *TestSuite) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "run":
		return &GoFunction{fn: s.Run}, true
	}
	return nil, false
}

func (s *TestSuite) Prop(name string) Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*TestSuite) PropertyNames() []string {
	return []string{"run"}
}

func (s *TestSuite) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("test-suite")
	return
}

// A TestCase represents a symbolic TestCase.
type TestCase struct {
	UnassignablePropsMixin
	_ int
}

func (s *TestCase) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *TestCase:
		return true
	default:
		return false
	}
}

func (s *TestCase) WidestOfType() Value {
	return &TestCase{}
}

func (s *TestCase) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (s *TestCase) Prop(name string) Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*TestCase) PropertyNames() []string {
	return nil
}

func (s *TestCase) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("test-case")
	return
}
