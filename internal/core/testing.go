package core

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	TEST__MAX_FS_STORAGE_HINT = ByteCount(10_000_000)
	TEST_FULL_NAME_PART_SEP   = "::"
)

var (
	_ = Value((*CurrentTest)(nil))
)

// A TestItem is a TestSuite or a TestCase.
type TestItem interface {
	ItemName() (string, bool)
	ParentChunk() *parse.ParsedChunk
	ParentModule() *Module
	Statement() parse.Node
	FilesystemSnapshot() (FilesystemSnapshot, bool)
}

// A TestSuite represents a test suite, TestSuite implements Value.
type TestSuite struct {
	meta                             Value
	nameFrom                         string             //can be empty
	filesystemSnapshot               FilesystemSnapshot //can be nil
	passLiveFilesystemCopyToSubTests bool
	testedProgramPath                Path //can be empty
	isTestedProgramFromParent        bool
	programProject                   Project        //set if .testedProgramPath is set
	mainDatabaseSchema               *ObjectPattern //can be nil
	mainDatabaseMigrations           *Object        //can be nil

	node         *parse.TestSuiteExpression
	module       *Module // module executed when running the test suite
	parentModule *Module
	parentChunk  *parse.ParsedChunk
}

type TestSuiteCreationInput struct {
	Meta             Value
	Node             *parse.TestSuiteExpression
	EmbeddedModChunk *parse.Chunk
	ParentChunk      *parse.ParsedChunk
	ParentState      *GlobalState
}

func NewTestSuite(input TestSuiteCreationInput) (*TestSuite, error) {
	//implementation should not perform resource intensive operations or IO operations.

	meta := input.Meta
	embeddedModChunk := input.EmbeddedModChunk
	parentChunk := input.ParentChunk
	parentState := input.ParentState

	parsedChunk := &parse.ParsedChunk{
		Node:   embeddedModChunk,
		Source: parentChunk.Source,
	}

	parsedChunk.GetFormattedNodeLocation(embeddedModChunk)

	// manifest, err := evaluateTestingManifest(parsedChunk, parentState)
	// if err != nil {
	// 	return nil, err
	// }

	routineMod := &Module{
		MainChunk:        parsedChunk,
		ManifestTemplate: parsedChunk.Node.Manifest,
		ModuleKind:       TestSuiteModule,
		//TODO: bytecode ?
	}

	suite := &TestSuite{
		meta: meta,
		node: input.Node,

		module:       routineMod,
		parentModule: parentState.Module,
		parentChunk:  parentChunk,
	}

	switch m := meta.(type) {
	case StringLike:
		suite.nameFrom = m.GetOrBuildString()
	case *Object:
		err := m.ForEachEntry(func(k string, v Serializable) error {
			switch k {
			case symbolic.TEST_ITEM_META__NAME_PROPNAME:
				strLike, ok := v.(StringLike)
				if ok {
					suite.nameFrom = strLike.GetOrBuildString()
				}
			case symbolic.TEST_ITEM_META__FS_PROPNAME:
				snapshot, ok := v.(*FilesystemSnapshotIL)
				if ok {
					suite.filesystemSnapshot = snapshot.underlying
				}
			case symbolic.TEST_ITEM_META__PASS_LIVE_FS_COPY:
				suite.passLiveFilesystemCopyToSubTests = bool(v.(Bool))
			case symbolic.TEST_ITEM_META__PROGRAM_PROPNAME:
				if parentState.Project == nil {
					return errors.New("program testing is only supported in projects")
				}
				baseImg, err := parentState.Project.BaseImage()
				if err != nil {
					return err
				}
				suite.filesystemSnapshot = baseImg.FilesystemSnapshot()
				suite.testedProgramPath = v.(Path)
				suite.programProject = parentState.Project
			case symbolic.TEST_ITEM_META__MAIN_DB_SCHEMA:
				suite.mainDatabaseSchema = v.(*ObjectPattern)
			case symbolic.TEST_ITEM_META__MAIN_DB_MIGRATIONS:
				suite.mainDatabaseMigrations = v.(*Object)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	case NilT:
	default:
		panic(ErrUnreachable)
	}

	if suite.testedProgramPath == "" {
		//inherit tested program from parent test suite
		if parentTestSuite, ok := input.ParentState.TestItem.(*TestSuite); ok && parentTestSuite.testedProgramPath != "" {
			suite.testedProgramPath = parentTestSuite.testedProgramPath
			suite.isTestedProgramFromParent = true

			suite.programProject = parentTestSuite.programProject
			suite.mainDatabaseSchema = parentTestSuite.mainDatabaseSchema
			suite.mainDatabaseMigrations = parentTestSuite.mainDatabaseMigrations
		}
	}

	return suite, nil
}

// Module returns the module that contains the test.
func (s *TestSuite) ParentModule() *Module {
	return s.module
}

// Module returns the chunk that contains the test.
func (s *TestSuite) ParentChunk() *parse.ParsedChunk {
	return s.parentChunk
}

func (s *TestSuite) ItemName() (string, bool) {
	if s.nameFrom == "" {
		return "", false
	}
	return s.nameFrom, true
}

func (s *TestSuite) Statement() parse.Node {
	return s.node
}

func (s *TestSuite) FilesystemSnapshot() (FilesystemSnapshot, bool) {
	if s.filesystemSnapshot != nil {
		return s.filesystemSnapshot, true
	}
	return nil, false
}

func (s *TestSuite) Run(ctx *Context, options ...Option) (*LThread, error) {
	if !s.node.IsStatement {
		//TODO: if the TestSuiteExpression node is not a statement,
		//the global variables, patterns and host aliases should be captured in the TestSuite.
		return nil, errors.New("running free test suites is not supported yet")
	}

	var timeout time.Duration

	for _, opt := range options {
		switch opt.Name {
		case "timeout":
			if timeout != 0 {
				return nil, commonfmt.FmtErrOptionProvidedAtLeastTwice("timeout")
			}
			timeout = time.Duration(opt.Value.(Duration))
		default:
			return nil, commonfmt.FmtErrInvalidOptionName(opt.Name)
		}
	}

	spawnerState := ctx.GetClosestState()
	var parentTestSuite *TestSuite
	if spawnerState.TestItem != nil {
		parentTestSuite = spawnerState.TestItem.(*TestSuite)
	}

	fsProvider, err := getTestItemFilesystemProvider(s, parentTestSuite, spawnerState)
	if err != nil {
		return nil, err
	}

	createLthreadPerm := LThreadPermission{Kind_: permkind.Create}

	if err := spawnerState.Ctx.CheckHasPermission(createLthreadPerm); err != nil {
		return nil, fmt.Errorf("testing: following permission is required for running tests: %w", err)
	}

	return runTestItem(ctx, spawnerState, s, s.module, fsProvider, timeout, parentTestSuite, "", nil, nil, nil, nil, nil)
}

func (s *TestSuite) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "run":
		return WrapGoMethod(s.Run), true
	}
	return nil, false
}

func (s *TestSuite) Prop(ctx *Context, name string) Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*TestSuite) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*TestSuite) PropertyNames(ctx *Context) []string {
	return []string{"run"}
}

// A TestCase represents a test case, TestCase implements Value.
type TestCase struct {
	meta                             Value
	name                             string             //can be empty
	filesystemSnapshot               FilesystemSnapshot //can be nil
	passLiveFilesystemCopyToSubTests bool
	testedProgramPath                Path           //can be empty
	programProject                   Project        //set if .testedProgramPath is set
	mainDatabaseSchema               *ObjectPattern //can be nil
	mainDatabaseMigrations           *Object        //can be nil

	node *parse.TestCaseExpression

	module       *Module // module executed when running the test case
	parentModule *Module
	parentChunk  *parse.ParsedChunk

	positionStack     parse.SourcePositionStack //can be nil
	formattedPosition string                    //can be empty
}

type TestCaseCreationInput struct {
	Meta Value
	Node *parse.TestCaseExpression

	ModChunk    *parse.Chunk
	ParentState *GlobalState
	ParentChunk *parse.ParsedChunk

	//optional
	PositionStack     parse.SourcePositionStack
	FormattedLocation string
}

func NewTestCase(input TestCaseCreationInput) (*TestCase, error) {
	//implementation should not perform resource intensive operations or IO operations.

	meta := input.Meta
	modChunk := input.ModChunk
	parentState := input.ParentState
	parentChunk := input.ParentChunk

	//optional
	positionStack := input.PositionStack
	formattedPosition := input.FormattedLocation

	parsedChunk := &parse.ParsedChunk{
		Node:   modChunk,
		Source: parentChunk.Source,
	}

	// manifest, err := evaluateTestingManifest(parsedChunk, parentState)
	// if err != nil {
	// 	return nil, err
	// }

	routineMod := &Module{
		MainChunk:        parsedChunk,
		ManifestTemplate: parsedChunk.Node.Manifest,
		ModuleKind:       TestCaseModule,
		//TODO: bytecode ?
	}

	testCase := &TestCase{
		meta: meta,
		node: input.Node,

		module:       routineMod,
		parentModule: parentState.Module,
		parentChunk:  parentChunk,

		positionStack:     positionStack,
		formattedPosition: formattedPosition,
	}

	//get name of test case

	switch m := meta.(type) {
	case StringLike:
		testCase.name = m.GetOrBuildString()
	case *Object:
		m.ForEachEntry(func(k string, v Serializable) error {
			switch k {
			case symbolic.TEST_ITEM_META__NAME_PROPNAME:
				strLike, ok := v.(StringLike)
				if ok {
					testCase.name = strLike.GetOrBuildString()
				}
			case symbolic.TEST_ITEM_META__FS_PROPNAME:
				snapshot, ok := v.(*FilesystemSnapshotIL)
				if ok {
					testCase.filesystemSnapshot = snapshot.underlying
				}
			case symbolic.TEST_ITEM_META__PASS_LIVE_FS_COPY:
				testCase.passLiveFilesystemCopyToSubTests = bool(v.(Bool))
			case symbolic.TEST_ITEM_META__PROGRAM_PROPNAME:
				if parentState.Project == nil {
					return errors.New("program testing is only supported in projects")
				}
				baseImg, err := parentState.Project.BaseImage()
				if err != nil {
					return err
				}
				testCase.filesystemSnapshot = baseImg.FilesystemSnapshot()
				testCase.testedProgramPath = v.(Path)
				testCase.programProject = parentState.Project
			case symbolic.TEST_ITEM_META__MAIN_DB_SCHEMA:
				testCase.mainDatabaseSchema = v.(*ObjectPattern)
			case symbolic.TEST_ITEM_META__MAIN_DB_MIGRATIONS:
				testCase.mainDatabaseMigrations = v.(*Object)
			}
			return nil
		})
	case NilT:
	default:
		panic(ErrUnreachable)
	}

	//clean formattedPosition
	formattedPosition = strings.TrimSpace(formattedPosition)
	formattedPosition = strings.TrimSuffix(formattedPosition, ":")
	testCase.formattedPosition = formattedPosition

	return testCase, nil
}

// Module returns the module that contains the test.
func (c *TestCase) ParentModule() *Module {
	return c.parentModule
}

// Module returns the chunk that contains the test.
func (c *TestCase) ParentChunk() *parse.ParsedChunk {
	return c.parentChunk
}

func (c *TestCase) ItemName() (string, bool) {
	if c.name == "" {
		return "", false
	}
	return c.name, true
}

func (c *TestCase) Statement() parse.Node {
	return c.node
}

func (c *TestCase) FilesystemSnapshot() (FilesystemSnapshot, bool) {
	if c.filesystemSnapshot != nil {
		return c.filesystemSnapshot, true
	}
	return nil, false
}

func (c *TestCase) Run(ctx *Context, options ...Option) (*LThread, error) {
	var timeout time.Duration

	if !c.node.IsStatement {
		//TODO: if the TestCaseExpression node is not a statement,
		//the global variables, patterns and host aliases should be captured in the TestCase.
		return nil, errors.New("running free test cases is not supported yet")
	}

	for _, opt := range options {
		switch opt.Name {
		case "timeout":
			if timeout != 0 {
				return nil, commonfmt.FmtErrOptionProvidedAtLeastTwice("timeout")
			}
			timeout = time.Duration(opt.Value.(Duration))
		default:
			return nil, commonfmt.FmtErrInvalidOptionName(opt.Name)
		}
	}

	spawnerState := ctx.GetClosestState()
	if spawnerState.TestItem == nil {
		panic(ErrUnreachable)
	}
	parentTestSuite := spawnerState.TestItem.(*TestSuite)

	fls, err := getTestItemFilesystemProvider(c, parentTestSuite, spawnerState)
	if err != nil {
		return nil, err
	}

	programToTest := c.testedProgramPath
	programProject := c.programProject
	mainDatabaseSchema := c.mainDatabaseSchema
	mainDatabaseMigrations := c.mainDatabaseMigrations

	var programModuleCache *Module
	var programDatabasePermissions []Permission

	//if the current case has not specified a tested program, it inherits the tested program.
	if programToTest == "" && parentTestSuite != nil && parentTestSuite.testedProgramPath != "" {
		programToTest = parentTestSuite.testedProgramPath
		programProject = parentTestSuite.programProject
		mainDatabaseSchema = parentTestSuite.mainDatabaseSchema
		mainDatabaseMigrations = parentTestSuite.mainDatabaseMigrations

		if spawnerState.TestedProgram == nil {
			panic(ErrUnreachable)
		}

		programModuleCache = spawnerState.TestedProgram
		resourceName, ok := programModuleCache.AbsoluteSource()
		if !ok || resourceName != programToTest {
			panic(ErrUnreachable)
		}
		programDatabasePermissions = utils.FilterSlice(spawnerState.Ctx.GetGrantedPermissions(), func(e Permission) bool {
			return utils.Implements[DatabasePermission](e)
		})
	}

	return runTestItem(
		ctx,
		spawnerState,
		c,
		c.module,
		fls,
		timeout,
		parentTestSuite,

		//program testing
		programToTest,
		programModuleCache,
		programProject,
		programDatabasePermissions,
		mainDatabaseSchema,
		mainDatabaseMigrations,
	)
}

func (s *TestCase) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "run":
		return WrapGoMethod(s.Run), true
	}
	return nil, false
}

func (s *TestCase) Prop(ctx *Context, name string) Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*TestCase) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*TestCase) PropertyNames(ctx *Context) []string {
	return []string{"run"}
}

func runTestItem(
	parentCtx *Context,
	spawnerState *GlobalState,
	testItem TestItem,
	testItemModule *Module,
	testItemFSProvider *fsProvider,
	timeout time.Duration,
	parentTestSuite *TestSuite,

	programToExecute Path, //can be empty
	programModuleCache *Module, //can be nil
	programProject Project, //only set if programToTest is not empty
	programDatabasePermissions []Permission,
	mainDatabaseSchema *ObjectPattern, //can be nil
	mainDatabaseMigrations *Object, //can be nil
) (*LThread, error) {

	suite, isTestSuite := testItem.(*TestSuite)
	_, isTestCase := testItem.(*TestCase)

	if !isTestCase && !isTestSuite {
		panic(ErrUnreachable)
	}

	//get the manifest of the test item's module
	manifest, _, _, err := testItemModule.PreInit(PreinitArgs{
		RunningState:          NewTreeWalkStateWithGlobal(spawnerState),
		ParentState:           spawnerState,
		AddDefaultPermissions: true,

		//TODO: should Project be set ?
	})

	if err != nil {
		return nil, err
	}

	var testedProgramModule *Module

	var implicitlyAddedPermissions []Permission
	implicitlyAddedPermissions = append(implicitlyAddedPermissions, programDatabasePermissions...)

	fsPerms := []Permission{
		FilesystemPermission{Kind_: permkind.Read, Entity: PathPattern("/...")},
		FilesystemPermission{Kind_: permkind.Write, Entity: PathPattern("/...")},
		FilesystemPermission{Kind_: permkind.Delete, Entity: PathPattern("/...")},
	}
	for _, fsPerm := range fsPerms {
		if parentCtx.HasPermission(fsPerm) {
			implicitlyAddedPermissions = append(implicitlyAddedPermissions, fsPerm)
		}
	}

	var fls afs.Filesystem

	// if the test item is a test suite with a program to test we parse it for later use by sub suites & test cases.
	if programToExecute == "" && isTestSuite && suite.testedProgramPath != "" {

		//if the parent test suite has already parsed the module, use the cache.
		if suite.isTestedProgramFromParent && parentTestSuite != nil && parentTestSuite.testedProgramPath == suite.testedProgramPath {
			if spawnerState.TestedProgram.sourceName != parentTestSuite.testedProgramPath {
				panic(ErrUnreachable)
			}
			testedProgramModule = spawnerState.TestedProgram
		} else {
			ctxWithFilesystem, isTempContext := testItemFSProvider.getMakeContextOnlyOnce()
			if isTempContext {
				defer ctxWithFilesystem.CancelGracefully()
			}

			mod, err := ParseLocalModule(string(suite.testedProgramPath), ModuleParsingConfig{
				Context: ctxWithFilesystem,
			})
			if err != nil {
				return nil, fmt.Errorf("testing: failed to parse the program to test for caching (%q): %w", suite.testedProgramPath, err)
			}
			testedProgramModule = mod
			fls = ctxWithFilesystem.GetFileSystem()
		}

		//read the manifest to determine the database resource names

		fmtImpossibleToDetermineDatabaseResources := func(fmtString string, args ...any) error {
			return fmt.Errorf("testing: impossible to determine the scheme of the database resources: "+fmtString, args...)
		}

		if testedProgramModule.ManifestTemplate == nil {
			return nil, fmtImpossibleToDetermineDatabaseResources("missing manifest in tested program")
		}

		if testedProgramModule == nil {
			return nil, fmtImpossibleToDetermineDatabaseResources("module cache is not present")
		}

		manifestObj, ok := testedProgramModule.ManifestTemplate.Object.(*parse.ObjectLiteral)
		if !ok {
			return nil, fmtImpossibleToDetermineDatabaseResources("tested program's manifest has no object literal")
		}

		val, _ := manifestObj.PropValue(MANIFEST_DATABASES_SECTION_NAME)
		databasesObj, ok := val.(*parse.ObjectLiteral)

		//add database permissions
		if ok {
			checkDatabasesObject(databasesObj, nil, func(name string, scheme Scheme, resource ResourceName) {
				implicitlyAddedPermissions = append(implicitlyAddedPermissions,
					DatabasePermission{Kind_: permkind.Read, Entity: resource},
					DatabasePermission{Kind_: permkind.Write, Entity: resource},
					DatabasePermission{Kind_: permkind.Delete, Entity: resource},
				)
			}, spawnerState.Project)
		}
	}

	//create the lthread context

	for _, perm := range manifest.RequiredPermissions {
		if err := spawnerState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("testing: cannot allow permission: %w", err)
		}
	}

	createLthreadPerm := LThreadPermission{Kind_: permkind.Create}

	permissions := slices.Clone(manifest.RequiredPermissions)
	permissions = append(permissions, createLthreadPerm)
	permissions = append(permissions, implicitlyAddedPermissions...)

	if fls == nil {
		fls = testItemFSProvider.getFilesystemOnlyOnce()
	}

	lthreadCtx := NewContext(ContextConfig{
		Kind:            TestingContext,
		ParentContext:   parentCtx,
		Permissions:     permissions,
		Limits:          manifest.Limits,
		HostResolutions: manifest.HostResolutions,

		Filesystem: fls,
	})

	//inherit patterns and host aliases.
	spawnerState.Ctx.ForEachNamedPattern(func(name string, pattern Pattern) error {
		lthreadCtx.AddNamedPattern(name, pattern)
		return nil
	})

	spawnerState.Ctx.ForEachPatternNamespace(func(name string, namespace *PatternNamespace) error {
		lthreadCtx.AddPatternNamespace(name, namespace)
		return nil
	})

	spawnerState.Ctx.ForEachHostAlias(func(name string, value Host) error {
		lthreadCtx.AddHostAlias(name, value)
		return nil
	})

	//prepare & start the program to test
	var testedProgramThread *LThread
	var testedProgramDatabases *Namespace

	if programToExecute != "" {
		programState, _, _, err := PrepareLocalScript(ScriptPreparationArgs{
			FullAccessToDatabases:   true,
			ForceExpectSchemaUpdate: true,

			Fpath:        string(programToExecute),
			Project:      programProject,
			CachedModule: programModuleCache,

			ParsingCompilationContext: parentCtx,
			ParentContext:             lthreadCtx, //TODO: gracefully stops the program
			DefaultLimits:             GetDefaultScriptLimits(),

			ScriptContextFileSystem: fls,
			PreinitFilesystem:       fls,

			Out:    spawnerState.Out,
			Logger: spawnerState.Logger,
		})

		if err != nil {
			if programState != nil && programState.Ctx != nil {
				programState.Ctx.CancelGracefully()
			}
			return nil, fmt.Errorf("testing: failed to prepare the program to test (%q): %w", programToExecute, err)
		}

		if mainDatabaseSchema != nil {
			db, ok := programState.Databases["main"]
			if !ok {
				return nil, fmt.Errorf("testing: the program to test (%q) has not a main database", programToExecute)
			}
			db.UpdateSchema(programState.Ctx, mainDatabaseSchema, mainDatabaseMigrations)
		}

		dbs := map[string]Value{}
		for k, v := range programState.Databases {
			dbs[k] = v
		}
		testedProgramDatabases = NewMutableEntriesNamespace("dbs", dbs)

		testedProgramThread, err = SpawnLthreadWithState(LthreadWithStateSpawnArgs{
			SpawnerState: spawnerState,
			State:        programState,
		})

		if err != nil {
			return nil, fmt.Errorf("testing: failed to spawn a lthread for the program to test (%q): %w", programToExecute, err)
		}
	}

	//spawn the lthread

	globals := spawnerState.Globals.Entries()

	for _, name := range globalnames.TEST_ITEM_NON_INHERITED_GLOBALS {
		delete(globals, name)
	}

	//Note: the globals are going to be shared/cloned by SpawnLThread.
	//This is okay because only testsuite & testcase statements are supported for now, not expressions.
	//Therefore the globals cannot be modified by another goroutine.

	var currentTest *CurrentTest
	if !isTestSuite {
		currentTest = &CurrentTest{
			&TestedProgram{
				lthread:   testedProgramThread,
				databases: testedProgramDatabases,
			},
		}
		globals[globalnames.CURRENT_TEST] = currentTest
	}

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: spawnerState,
		LthreadCtx:   lthreadCtx,
		Globals:      GlobalVariablesFromMap(globals, nil),
		NewGlobals:   []string{globalnames.CURRENT_TEST},
		Module:       testItemModule,
		Manifest:     manifest,
		Timeout:      timeout,

		IsTestingEnabled: spawnerState.IsTestingEnabled,
		TestFilters:      spawnerState.TestFilters,
		TestItem:         testItem,
		TestedProgram:    testedProgramModule,
	})

	if err != nil {
		return nil, err
	}

	err = lthread.ResumeAsync()
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("testing: %w", err)
	}
	return lthread, nil
}

// getTestItemFilesystemProvider retrieves or create the filesystem for a test item and returns a testItemFSProvider providing it.
//   - if the test item has a filesystem snapshot then a filesystem is created from it.
//   - else if the parent test suite is nil then the parent state's filesystem is returned.
//   - else if the parent test suite is configured to pass a shapshot of its live filesystem
//     then its filesystem is snapshoted and a filesystem is created from it.
//   - else if the parent test suite has a filesystem snapshot then a filesystem is created from it.
//   - else the parent state's filesystem is returned.
func getTestItemFilesystemProvider(test TestItem, parentTestSuite *TestSuite, spawnerState *GlobalState) (*fsProvider, error) {
	if snapshot, ok := test.FilesystemSnapshot(); ok {
		fls, err := snapshot.NewAdaptedFilesystem(TEST__MAX_FS_STORAGE_HINT)
		if err != nil {
			return nil, err
		}

		return &fsProvider{
			filesystem: fls,
			makeContext: func() *Context {
				return spawnerState.Ctx.BoundChildWithOptions(BoundChildContextOptions{
					Filesystem: fls,
				})
			},
		}, nil
	} else if parentTestSuite == nil {
		return &fsProvider{
			filesystem: spawnerState.Ctx.GetFileSystem(),
			getContext: func() *Context {
				return spawnerState.Ctx
			},
		}, nil
	} else if parentTestSuite.passLiveFilesystemCopyToSubTests {
		parentFls := spawnerState.Ctx.GetFileSystem()

		if snapshotable, ok := parentFls.(SnapshotableFilesystem); ok {
			snapshotConfig := FilesystemSnapshotConfig{
				GetContent: func(ChecksumSHA256 [32]byte) AddressableContent {
					return nil
				},
				InclusionFilters: []PathPattern{"/..."},
			}

			snapshot, err := snapshotable.TakeFilesystemSnapshot(snapshotConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to take snapshot of the live filesystem of the parent test suite: %w", err)
			}
			filesystem, err := snapshot.NewAdaptedFilesystem(TEST__MAX_FS_STORAGE_HINT)
			if err != nil {
				return nil, fmt.Errorf("failed to create filesystem from the live filesystem of the parent test suite")
			}

			return &fsProvider{
				filesystem: filesystem,
				makeContext: func() *Context {
					return spawnerState.Ctx.BoundChildWithOptions(BoundChildContextOptions{
						Filesystem: filesystem,
					})
				},
			}, nil
		} else {
			return nil, fmt.Errorf("failed to create filesystem: the filesystem of the parent test suite is not snapshotable")
		}
	} else {
		snapshot, ok := spawnerState.TestItem.FilesystemSnapshot()
		if ok {
			filesystem, err := snapshot.NewAdaptedFilesystem(TEST__MAX_FS_STORAGE_HINT)
			if err != nil {
				return nil, fmt.Errorf("failed to create filesystem from the filesystem snapshot of the parent test suite")
			}
			return &fsProvider{
				filesystem: filesystem,
				makeContext: func() *Context {
					return spawnerState.Ctx.BoundChildWithOptions(BoundChildContextOptions{
						Filesystem: filesystem,
					})
				},
			}, nil
		} else {
			return &fsProvider{
				filesystem: spawnerState.Ctx.GetFileSystem(),
				getContext: func() *Context {
					return spawnerState.Ctx
				},
			}, nil
		}
	}
}

func makeTestFullName(item TestItem, parentState *GlobalState) string {
	name, ok := item.ItemName()
	if !ok {
		name = "??"
	}
	parentTestItem := parentState.TestItem

	if parentTestItem == nil {
		return name
	}

	return parentState.TestItemFullName + TEST_FULL_NAME_PART_SEP + name
}

type TestFilters struct {
	PositiveTestFilters []TestFilter
	NegativeTestFilters []TestFilter
}

func (filters TestFilters) IsTestEnabled(item TestItem, parentState *GlobalState) (enabled bool, reason string) {
	srcName := item.ParentChunk().Source.Name()

	if !strings.HasPrefix(srcName, "/") {
		return false, "the test is disabled because it is not located in a local file or the file's path is not absolute"
	}
	absoluteFilePath := srcName

	for _, negativeFilter := range filters.NegativeTestFilters {
		if disabled, _ := negativeFilter.IsTestEnabled(absoluteFilePath, item, parentState); disabled {
			return false, fmt.Sprintf(
				"the test is disabled because it matches the negative filter %q", negativeFilter.String())
		}
	}

	for _, positiveFilter := range filters.PositiveTestFilters {
		if enabled, _ := positiveFilter.IsTestEnabled(absoluteFilePath, item, parentState); enabled {
			return true, fmt.Sprintf(
				"the test is enabled because it matches the positive filter %q", positiveFilter.String())
		}
	}

	return false, ""
}

// A TestFilter filter tests by checking several values.
type TestFilter struct {
	//never nil
	NameRegex string

	//if path ends with '/...' all tests found in subdirectories are also enabled.
	//this field is ignored if it is empty.
	AbsolutePath string

	//span of the test suite or test case statement, this field is ignored if it is equal to the zero value
	//or AbsolutePath is empty.
	NodeSpan parse.NodeSpan
}

func (f TestFilter) String() string {
	absolutePath := f.AbsolutePath
	if absolutePath == "" {
		absolutePath = "<any>"
	}

	if reflect.ValueOf(f.NodeSpan).IsZero() {
		return fmt.Sprintf("[path(s):%s name:%s]", absolutePath, f.NameRegex)
	}
	return fmt.Sprintf("[path(s):%s span:%d,%d name:%s]", absolutePath, f.NodeSpan.Start, f.NodeSpan.End, f.NameRegex)
}

func (f TestFilter) IsTestEnabled(absoluteFilePath string, item TestItem, parentState *GlobalState) (enabled bool, reason string) {
	stmtSpan := item.Statement().Base().Span
	itemFullName := makeTestFullName(item, parentState)

	if f.AbsolutePath != "" {
		if strings.HasSuffix(f.AbsolutePath, PREFIX_PATH_PATTERN_SUFFIX) {
			if !PathPattern(f.AbsolutePath).Test(nil, Str(absoluteFilePath)) {
				return false, fmt.Sprintf(
					"the test is disabled because its path (%q) does not match the filter's path pattern (%q)", absoluteFilePath, f.AbsolutePath)
			}
		} else if absoluteFilePath != f.AbsolutePath {
			return false, fmt.Sprintf(
				"the test is disabled because its path (%q) does not match the filter's path (%q)", absoluteFilePath, f.AbsolutePath)
		}

		//check the node span
		if !reflect.ValueOf(f.NodeSpan).IsZero() &&
			//if the statement's span is not equal to and not inside the filter's node span
			stmtSpan != f.NodeSpan &&
			(stmtSpan.Start < f.NodeSpan.Start || stmtSpan.End > f.NodeSpan.End) {

			if _, ok := item.(*TestCase); ok {
				return false, fmt.Sprintf(
					"the test case is disabled because its span (%d:%d) does not match the filter's span (%d:%d)",
					stmtSpan.Start, stmtSpan.End, f.NodeSpan.Start, f.NodeSpan.End)
				//else check that the span is inside the test suite's span
			} else if f.NodeSpan.Start < stmtSpan.Start || f.NodeSpan.End > stmtSpan.End {
				return false, fmt.Sprintf(
					"the test is disabled because its span (%d:%d) does not includes the filter's span (%d:%d)",
					stmtSpan.Start, stmtSpan.End, f.NodeSpan.Start, f.NodeSpan.End)
			}
		}
	}

	nameParts := strings.Split(itemFullName, TEST_FULL_NAME_PART_SEP)
	partRegexes := strings.Split(f.NameRegex, TEST_FULL_NAME_PART_SEP)
	nameMatches := true

	//check that part regexes match their corresponding name part.
	//it's not a problem if there are more name parts than part regexes ot the other way around.

	for i, regex := range partRegexes[:min(len(nameParts), len(partRegexes))] {
		namePart := nameParts[i]
		if ok, err := regexp.MatchString(regex, namePart); !ok || err != nil {
			nameMatches = false
			break
		}
	}

	if !nameMatches {
		return false, fmt.Sprintf(
			"the test is disabled because its full name (%q) does not match the filter's regex (%q)", itemFullName, f.NameRegex)
	}

	return true, "the test is enabled because it matches the filter " + f.String()
}

type CurrentTest struct {
	program *TestedProgram //can be nil
}

func (t *CurrentTest) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (t *CurrentTest) Prop(ctx *Context, name string) Value {
	switch name {
	case "program":
		if t.program == nil {
			return Nil
		}
		return t.program
	}
	method, ok := t.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, t))
	}
	return method
}

func (*CurrentTest) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*CurrentTest) PropertyNames(ctx *Context) []string {
	return symbolic.CURRENT_TEST_PROPNAMES
}

type TestedProgram struct {
	lthread   *LThread
	databases *Namespace
}

func (p *TestedProgram) Cancel(*Context) {
	p.lthread.state.Ctx.CancelGracefully()
}

func (p *TestedProgram) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "cancel":
		return WrapGoMethod(p.Cancel), true
	}
	return nil, false
}

func (p *TestedProgram) Prop(ctx *Context, name string) Value {
	switch name {
	case "is_done":
		return Bool(p.lthread.IsDone())
	case "dbs":
		return p.databases
	}
	method, ok := p.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, p))
	}
	return method
}

func (*TestedProgram) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*TestedProgram) PropertyNames(ctx *Context) []string {
	return symbolic.TESTED_PROGRAM_PROPNAMES
}

type fsProvider struct {
	makeContext func() *Context //creates a context containing .filesystem
	getContext  func() *Context //returns a context containing .filesystem
	filesystem  afs.Filesystem
	used        bool
}

func (p *fsProvider) getFilesystemOnlyOnce() afs.Filesystem {
	if p.used {
		panic(errors.New("already used"))
	}
	p.used = true
	return p.filesystem
}

func (p *fsProvider) getMakeContextOnlyOnce() (ctx *Context, isTempContext bool) {
	if p.used {
		panic(errors.New("already used"))
	}
	p.used = true
	if p.getContext != nil {
		return p.getContext(), false
	}
	return p.makeContext(), true
}
