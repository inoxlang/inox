package symbolic

import (
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
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
	TEST_ITEM__EXPECTED_META_VALUE = NewMultivalue(ANY_STR_LIKE, NewExactObject(map[string]Serializable{
		TEST_ITEM_META__NAME_PROPNAME: ANY_STR_LIKE,

		//filesystem
		TEST_ITEM_META__FS_PROPNAME:       ANY_FS_SNAPSHOT_IL,
		TEST_ITEM_META__PASS_LIVE_FS_COPY: ANY_BOOL,

		//program testing
		TEST_ITEM_META__PROGRAM_PROPNAME: ANY_ABS_NON_DIR_PATH,
		TEST_ITEM_META__MAIN_DB_SCHEMA:   ANY_OBJECT_PATTERN,
		TEST_ITEM_META__MAIN_DB_MIGRATIONS: NewInexactObject(
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
			nil,
		),
	}, nil, nil))

	ANY_TEST_SUITE = &TestSuite{}
	ANY_TEST_CASE  = &TestCase{}

	ANY_TESTED_PROGRAM_OR_NIL = NewMultivalue(ANY_TESTED_PROGRAM, Nil)
	ANY_TESTED_PROGRAM        = &TestedProgram{databases: ANY_MUTABLE_ENTRIES_NAMESPACE}

	ANY_CURRENT_TEST              = &CurrentTest{testedProgram: ANY_TESTED_PROGRAM_OR_NIL}
	ANY_CURRENT_TEST_WITH_PROGRAM = &CurrentTest{testedProgram: ANY_TESTED_PROGRAM}

	CURRENT_TEST_PROPNAMES   = []string{"program"}
	TESTED_PROGRAM_PROPNAMES = []string{"is_done", "cancel", "dbs"}
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

func (s *TestSuite) Run(ctx *Context, options ...*Option) (*LThread, *Error) {
	return ANY_LTHREAD, nil
}

func (s *TestSuite) WidestOfType() Value {
	return ANY_TEST_SUITE
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

func (s *TestSuite) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
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
	return ANY_TEST_CASE
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

func (s *TestCase) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("test-case")
}

// checkTestItemMeta evaluates & checks the meta value of a test item, it returns a *CurrentTest for test cases.
func checkTestItemMeta(node parse.Node, state *State, isTestCase bool) (currentTest *CurrentTest, testedProgram *TestedProgram, _ error) {
	meta, err := _symbolicEval(node, state, evalOptions{
		expectedValue: TEST_ITEM__EXPECTED_META_VALUE,
	})
	if err != nil {
		return nil, nil, err
	}

	if isTestCase {
		currentTest = ANY_CURRENT_TEST
	}

	parentTestedProgram := state.testedProgram //can be nil

	switch m := meta.(type) {
	case *Object:
		hasMainDatabaseSchema := m.hasProperty(TEST_ITEM_META__MAIN_DB_SCHEMA)
		hasMainDatabaseMigrations := m.hasProperty(TEST_ITEM_META__MAIN_DB_MIGRATIONS)
		hasProgram := m.hasProperty(TEST_ITEM_META__PROGRAM_PROPNAME)

		if parentTestedProgram != nil && !hasProgram && !hasMainDatabaseSchema && !hasMainDatabaseMigrations {
			//inherit tested program
			testedProgram = parentTestedProgram
			currentTest = &CurrentTest{testedProgram: testedProgram}
			return
		}

		if !hasProgram {
			if hasMainDatabaseSchema {
				state.addError(makeSymbolicEvalError(node, state, MAIN_DB_SCHEMA_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM))
			}
			if hasMainDatabaseMigrations {
				state.addError(makeSymbolicEvalError(node, state, MAIN_DB_MIGRATIONS_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM))
			}
			return
		}
		//else if the test item tests a program

		if state.projectFilesystem == nil {
			state.addError(makeSymbolicEvalError(node, state, PROGRAM_TESTING_ONLY_SUPPORTED_IN_PROJECTS))
			return
		}

		testedProgram = &TestedProgram{
			databases: ANY_NAMESPACE,
		}
		currentTest = &CurrentTest{testedProgram: testedProgram}

		if hasMainDatabaseSchema {
			if !hasMainDatabaseMigrations {
				state.addError(makeSymbolicEvalError(node, state, MISSING_MAIN_DB_MIGRATIONS_PROPERTY))
			}

			schema, ok := m.Prop(TEST_ITEM_META__MAIN_DB_SCHEMA).(*ObjectPattern)

			if ok {
				testedProgram.databases = NewMutableEntriesNamespace(map[string]Value{
					"main": NewDatabaseIL(schema, false),
				})
			}
		}

		program, ok := m.Prop(TEST_ITEM_META__PROGRAM_PROPNAME).(*Path)
		if !ok || program.pattern == nil || program.pattern.absoluteness != AbsolutePath || program.pattern.dirConstraint != DirPath {
			return
		}

		if program.hasValue {
			info, err := state.projectFilesystem.Stat(program.value)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get info of file %s: %w", program.value, err)
			}
			if !info.Mode().IsRegular() {
				state.addError(makeSymbolicEvalError(node, state, fmtNotRegularFile(program.value)))
			}
		}
	case StringLike:

		//inherit tested program
		if parentTestedProgram != nil {
			testedProgram = parentTestedProgram
			currentTest = &CurrentTest{testedProgram: testedProgram}
			return
		}

	default:
		msg := META_VAL_OF_TEST_SUITE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD
		if isTestCase {
			msg = META_VAL_OF_TEST_CASE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD
		}
		state.addError(makeSymbolicEvalError(node, state, msg))
	}

	return
}

// A CurrentTest represents a symbolic CurrentTest.
type CurrentTest struct {
	UnassignablePropsMixin
	testedProgram Value
}

func (t *CurrentTest) hasMainDatabase() bool {
	program, ok := t.testedProgram.(*TestedProgram)
	if ok {
		return program.hasMainDatabase()
	}
	return false
}

func (t *CurrentTest) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherTest, ok := v.(*CurrentTest)

	if !ok {
		return false
	}

	return t.testedProgram.Test(otherTest.testedProgram, state)
}

func (t *CurrentTest) WidestOfType() Value {
	return ANY_CURRENT_TEST
}

func (t *CurrentTest) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (t *CurrentTest) Prop(name string) Value {
	switch name {
	case "program":
		return t.testedProgram
	}
	method, ok := t.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, t))
	}
	return method
}

func (*CurrentTest) PropertyNames() []string {
	return CURRENT_TEST_PROPNAMES
}

func (t *CurrentTest) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("current-test")
}

// A TestedProgram represents a symbolic TestedProgram.
type TestedProgram struct {
	UnassignablePropsMixin
	databases *Namespace
}

func (t *TestedProgram) hasMainDatabase() bool {
	return HasRequiredProperty(t.databases, "main")
}

func (t *TestedProgram) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *TestedProgram:
		return true
	default:
		return false
	}
}

func (t *TestedProgram) WidestOfType() Value {
	return ANY_TESTED_PROGRAM
}

func (t *TestedProgram) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "cancel":
		return WrapGoMethod(t.Cancel), true
	}
	return nil, false
}

func (t *TestedProgram) Prop(name string) Value {
	switch name {
	case "is_done":
		return ANY_BOOL
	case "dbs":
		return t.databases
	}
	method, ok := t.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, t))
	}
	return method
}

func (*TestedProgram) PropertyNames() []string {
	return TESTED_PROGRAM_PROPNAMES
}

func (t *TestedProgram) Cancel(*Context) {

}

func (t *TestedProgram) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("tested-program")
}
