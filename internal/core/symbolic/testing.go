package symbolic

import (
	"fmt"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

const (
	TEST_ITEM_META__NAME_PROPNAME      = "name"
	TEST_ITEM_META__FS_PROPNAME        = "fs"
	TEST_ITEM_META__PROGRAM_PROPNAME   = "program"
	TEST_ITEM_META__PASS_LIVE_FS_COPY  = "pass-live-fs-copy-to-subtests"
	TEST_ITEM_META__MAIN_DB_SCHEMA     = "main-db-schema"
	TEST_ITEM_META__MAIN_DB_MIGRATIONS = "main-db-migrations"
)

var (
	TEST_ITEM__EXPECTED_META_VALUE = NewMultivalue(ANY_STR_LIKE, NewInexactRecord(map[string]Serializable{
		TEST_ITEM_META__NAME_PROPNAME: ANY_STR_LIKE,

		//filesystem
		TEST_ITEM_META__FS_PROPNAME:       ANY_FS_SNAPSHOT_IL,
		TEST_ITEM_META__PASS_LIVE_FS_COPY: ANY_BOOL,

		//program testing
		TEST_ITEM_META__PROGRAM_PROPNAME: ANY_ABS_NON_DIR_PATH,
		TEST_ITEM_META__MAIN_DB_SCHEMA:   ANY_OBJECT_PATTERN,
		TEST_ITEM_META__MAIN_DB_MIGRATIONS: NewInexactRecord(
			map[string]Serializable{
				DB_MIGRATION__DELETIONS_PROP_NAME:       ANY_DICT,
				DB_MIGRATION__INCLUSIONS_PROP_NAME:      ANY_DICT,
				DB_MIGRATION__REPLACEMENTS_PROP_NAME:    ANY_DICT,
				DB_MIGRATION__INITIALIZATIONS_PROP_NAME: ANY_DICT,
			},
			//optional entries
			map[string]struct{}{
				DB_MIGRATION__DELETIONS_PROP_NAME:       {},
				DB_MIGRATION__INCLUSIONS_PROP_NAME:      {},
				DB_MIGRATION__REPLACEMENTS_PROP_NAME:    {},
				DB_MIGRATION__INITIALIZATIONS_PROP_NAME: {},
			},
		),
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
}

func checkTestItemMeta(node parse.Node, state *State, isTestCase bool) error {
	meta, err := _symbolicEval(node, state, evalOptions{
		expectedValue: TEST_ITEM__EXPECTED_META_VALUE,
	})
	if err != nil {
		return err
	}
	switch m := meta.(type) {
	case *Record:
		if !m.hasProperty(TEST_ITEM_META__PROGRAM_PROPNAME) {
			if m.hasProperty(TEST_ITEM_META__MAIN_DB_SCHEMA) {
				state.addError(makeSymbolicEvalError(node, state, MAIN_DB_SCHEMA_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM))
			}
			if m.hasProperty(TEST_ITEM_META__MAIN_DB_MIGRATIONS) {
				state.addError(makeSymbolicEvalError(node, state, MAIN_DB_MIGRATIONS_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM))
			}
			return nil
		}
		if state.projectFilesystem == nil {
			state.addError(makeSymbolicEvalError(node, state, PROGRAM_TESTING_ONLY_SUPPORTED_IN_PROJECTS))
			return nil
		}

		program, ok := m.Prop(TEST_ITEM_META__PROGRAM_PROPNAME).(*Path)
		if !ok || program.pattern == nil || program.pattern.absoluteness != AbsolutePath || program.pattern.dirConstraint != DirPath {
			return nil
		}

		if program.hasValue {
			info, err := state.projectFilesystem.Stat(program.value)
			if err != nil {
				return fmt.Errorf("failed to get info of file %s: %w", program.value, err)
			}
			if !info.Mode().IsRegular() {
				state.addError(makeSymbolicEvalError(node, state, fmtNotRegularFile(program.value)))
			}
		}
	case StringLike:
	default:
		msg := META_VAL_OF_TEST_SUITE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD
		if isTestCase {
			msg = META_VAL_OF_TEST_CASE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD
		}
		state.addError(makeSymbolicEvalError(node, state, msg))
	}

	return nil
}
