package core

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	TEST__MAX_FS_STORAGE_HINT = ByteCount(10_000_000)
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
	nameFromMeta                     string             //can be empty
	filesystemSnapshotFromMeta       FilesystemSnapshot //can be nil
	passLiveFilesystemCopyToSubTests bool

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

	//get name of test suite
	nameFromMeta := ""
	passLiveFsCopyToSubTests := false
	var snapshotFromMeta FilesystemSnapshot

	switch m := meta.(type) {
	case StringLike:
		nameFromMeta = m.GetOrBuildString()
	case *Record:
		m.ForEachEntry(func(k string, v Value) error {
			switch k {
			case symbolic.TEST_ITEM_META__NAME_PROPNAME:
				strLike, ok := v.(StringLike)
				if ok {
					nameFromMeta = strLike.GetOrBuildString()
				}
			case symbolic.TEST_ITEM_META__FS_PROPNAME:
				snapshot, ok := v.(*FilesystemSnapshotIL)
				if ok {
					snapshotFromMeta = snapshot.underlying
				}
			case symbolic.TEST_ITEM_META__PASS_LIVE_FS_COPY:
				passLiveFsCopyToSubTests = bool(v.(Bool))
			}
			return nil
		})
	case NilT:
	default:
		panic(ErrUnreachable)
	}

	return &TestSuite{
		meta:         meta,
		node:         input.Node,
		module:       routineMod,
		parentModule: parentState.Module,
		parentChunk:  parentChunk,

		nameFromMeta:                     nameFromMeta,
		filesystemSnapshotFromMeta:       snapshotFromMeta,
		passLiveFilesystemCopyToSubTests: passLiveFsCopyToSubTests,
	}, nil
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
	if s.nameFromMeta == "" {
		return "", false
	}
	return s.nameFromMeta, true
}

func (s *TestSuite) Statement() parse.Node {
	return s.node
}

func (s *TestSuite) FilesystemSnapshot() (FilesystemSnapshot, bool) {
	if s.filesystemSnapshotFromMeta != nil {
		return s.filesystemSnapshotFromMeta, true
	}
	return nil, false
}

func (s *TestSuite) Run(ctx *Context, options ...Option) (*LThread, error) {
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

	fls, err := getTestItemFilesystem(s, parentTestSuite, spawnerState)
	if err != nil {
		return nil, err
	}

	createLthreadPerm := LThreadPermission{Kind_: permkind.Create}

	if err := spawnerState.Ctx.CheckHasPermission(createLthreadPerm); err != nil {
		return nil, fmt.Errorf("testing: following permission is required for running tests: %w", err)
	}

	manifest, _, _, err := s.module.PreInit(PreinitArgs{
		RunningState:          NewTreeWalkStateWithGlobal(spawnerState),
		ParentState:           spawnerState,
		AddDefaultPermissions: true,

		//TODO: should Project be set ?
	})

	if err != nil {
		return nil, err
	}

	//create the lthread context

	for _, perm := range manifest.RequiredPermissions {
		if err := spawnerState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("testing: cannot allow permission: %w", err)
		}
	}

	permissions := utils.CopySlice(manifest.RequiredPermissions)
	permissions = append(permissions, createLthreadPerm)

	routineCtx := NewContext(ContextConfig{
		Kind:            TestingContext,
		ParentContext:   ctx,
		Permissions:     permissions,
		Limits:          manifest.Limits,
		HostResolutions: manifest.HostResolutions,

		Filesystem: fls,
	})

	//spawn the lthread

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: spawnerState,
		LthreadCtx:   routineCtx,
		Globals:      spawnerState.Globals,
		Module:       s.module,
		Manifest:     manifest,
		Timeout:      timeout,

		IsTestingEnabled: spawnerState.IsTestingEnabled,
		TestFilters:      spawnerState.TestFilters,
		TestItem:         s,
	})

	if err != nil {
		return nil, fmt.Errorf("testing: %w", err)
	}
	return lthread, nil
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
	nameFromMeta                     string             //can be empty
	filesystemSnapshotFromMeta       FilesystemSnapshot //can be nil
	passLiveFilesystemCopyToSubTests bool
	node                             *parse.TestCaseExpression

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

	//get name of test case
	nameFromMeta := ""
	passLiveFsCopyToSubTests := false
	var snapshotFromMeta FilesystemSnapshot

	switch m := meta.(type) {
	case StringLike:
		nameFromMeta = m.GetOrBuildString()
	case *Record:
		m.ForEachEntry(func(k string, v Value) error {
			switch k {
			case symbolic.TEST_ITEM_META__NAME_PROPNAME:
				strLike, ok := v.(StringLike)
				if ok {
					nameFromMeta = strLike.GetOrBuildString()
				}
			case symbolic.TEST_ITEM_META__FS_PROPNAME:
				snapshot, ok := v.(*FilesystemSnapshotIL)
				if ok {
					snapshotFromMeta = snapshot.underlying
				}
			case symbolic.TEST_ITEM_META__PASS_LIVE_FS_COPY:
				passLiveFsCopyToSubTests = bool(v.(Bool))
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

	return &TestCase{
		meta:                             meta,
		nameFromMeta:                     nameFromMeta,
		filesystemSnapshotFromMeta:       snapshotFromMeta,
		passLiveFilesystemCopyToSubTests: passLiveFsCopyToSubTests,
		node:                             input.Node,

		module:       routineMod,
		parentModule: parentState.Module,
		parentChunk:  parentChunk,

		positionStack:     positionStack,
		formattedPosition: formattedPosition,
	}, nil
}

// func evaluateTestingManifest(chunk *parse.ParsedChunk, parentState *GlobalState) (*Manifest, error) {
// 	createRoutinePerm := RoutinePermission{Kind_: permkind.Create}

// 	if err := parentState.Ctx.CheckHasPermission(createRoutinePerm); err != nil {
// 		return nil, fmt.Errorf("testing: following permission is required for running tests: %w", err)
// 	}

// 	if chunk.Node.Manifest == nil {
// 		return &Manifest{
// 			RequiredPermissions: []Permission{createRoutinePerm},
// 		}, nil
// 	}

// 	objectLiteral := chunk.Node.Manifest.Object
// 	state := NewTreeWalkStateWithGlobal(parentState)

// 	manifestObj, err := evaluateManifestObjectNode(objectLiteral, ManifestEvaluationConfig{
// 		RunningState: state,
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	manifest, err := createManifest(manifestObj, manifestObjectConfig{
// 		addDefaultPermissions: true,
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	hasCreateRoutine := false

// 	for _, perm := range manifest.RequiredPermissions {
// 		if perm.Includes(createRoutinePerm) && createRoutinePerm.Includes(perm) {
// 			hasCreateRoutine = true
// 		}
// 		if err := parentState.Ctx.CheckHasPermission(perm); err != nil {
// 			return nil, fmt.Errorf("testing: cannot allow permission: %w", err)
// 		}
// 	}

// 	if !hasCreateRoutine {
// 		manifest.RequiredPermissions = append(manifest.RequiredPermissions, createRoutinePerm)
// 	}
// 	return manifest, err
// }

// Module returns the module that contains the test.
func (c *TestCase) ParentModule() *Module {
	return c.parentModule
}

// Module returns the chunk that contains the test.
func (c *TestCase) ParentChunk() *parse.ParsedChunk {
	return c.parentChunk
}

func (c *TestCase) ItemName() (string, bool) {
	if c.nameFromMeta == "" {
		return "", false
	}
	return c.nameFromMeta, true
}

func (c *TestCase) Statement() parse.Node {
	return c.node
}

func (c *TestCase) FilesystemSnapshot() (FilesystemSnapshot, bool) {
	if c.filesystemSnapshotFromMeta != nil {
		return c.filesystemSnapshotFromMeta, true
	}
	return nil, false
}

func (c *TestCase) Run(ctx *Context, options ...Option) (*LThread, error) {
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
	if spawnerState.TestItem == nil {
		panic(ErrUnreachable)
	}
	parentTestSuite := spawnerState.TestItem.(*TestSuite)

	fls, err := getTestItemFilesystem(c, parentTestSuite, spawnerState)
	if err != nil {
		return nil, err
	}

	manifest, _, _, err := c.module.PreInit(PreinitArgs{
		RunningState: NewTreeWalkStateWithGlobal(spawnerState),
		ParentState:  spawnerState,

		AddDefaultPermissions: true,

		//TODO: should Project be set ?
	})

	if err != nil {
		return nil, err
	}

	//create the lthread context

	for _, perm := range manifest.RequiredPermissions {
		if err := spawnerState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("testing: cannot allow permission: %w", err)
		}
	}

	routineCtx := NewContext(ContextConfig{
		Kind:            TestingContext,
		ParentContext:   ctx,
		Permissions:     manifest.RequiredPermissions,
		Limits:          manifest.Limits,
		HostResolutions: manifest.HostResolutions,

		Filesystem: fls,
	})

	//spawn the lthread

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: spawnerState,
		LthreadCtx:   routineCtx,
		Globals:      spawnerState.Globals,
		Module:       c.module,
		Manifest:     manifest,
		Timeout:      timeout,

		IsTestingEnabled: spawnerState.IsTestingEnabled,
		TestFilters:      spawnerState.TestFilters,
		TestItem:         c,
	})

	if err != nil {
		return nil, fmt.Errorf("testing: %w", err)
	}
	return lthread, nil
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

type TestFilters struct {
	PositiveTestFilters []TestFilter
	NegativeTestFilters []TestFilter
}

func (filters TestFilters) IsTestEnabled(item TestItem) (enabled bool, reason string) {
	itemName, _ := item.ItemName()
	srcName := item.ParentChunk().Source.Name()
	stmtSpan := item.Statement().Base().Span

	if !strings.HasPrefix(srcName, "/") {
		return false, "the test is disabled because it is not located in a local file or the file's path is not absolute"
	}
	absoluteFilePath := srcName

	for _, negativeFilter := range filters.NegativeTestFilters {
		if disabled, _ := negativeFilter.IsTestEnabled(absoluteFilePath, itemName, stmtSpan); disabled {
			return false, fmt.Sprintf(
				"the test is disabled because it matches the negative filter %q", negativeFilter.String())
		}
	}

	for _, positiveFilter := range filters.PositiveTestFilters {
		if enabled, _ := positiveFilter.IsTestEnabled(absoluteFilePath, itemName, stmtSpan); enabled {
			return true, fmt.Sprintf(
				"the test is enabled because it matches the positive filter %q", positiveFilter.String())
		}
	}

	return false, ""
}

// A TestFilter filter tests by checking several values.
type TestFilter struct {
	//if path ends with '/...' all tests found in subdirectories are also enabled.
	//this field is ignore if it is empty.
	AbsolutePath string

	//never nil
	NameRegex *regexp.Regexp

	//span of the test suite or test case statement, this field is ignored if it is equal to the zero value.
	NodeSpan parse.NodeSpan
}

func (f TestFilter) String() string {
	absolutePath := f.AbsolutePath
	if absolutePath == "" {
		absolutePath = "<any>"
	}

	if reflect.ValueOf(f.NodeSpan).IsZero() {
		return fmt.Sprintf("[path(s):%s name:%s]", absolutePath, f.NameRegex.String())
	}
	return fmt.Sprintf("[path(s):%s span:%d,%d name:%s]", absolutePath, f.NodeSpan.Start, f.NodeSpan.End, f.NameRegex.String())
}

func (f TestFilter) IsTestEnabled(absoluteFilePath string, name string, statementSpan parse.NodeSpan) (enabled bool, reason string) {
	if f.AbsolutePath != "" {

		if strings.HasSuffix(f.AbsolutePath, "/...") {
			if !PathPattern(f.AbsolutePath).Test(nil, Str(absoluteFilePath)) {
				return false, fmt.Sprintf(
					"the test is disabled because its path (%q) does not match the filter's path pattern (%q)", absoluteFilePath, f.AbsolutePath)
			}
		} else if absoluteFilePath != f.AbsolutePath {
			return false, fmt.Sprintf(
				"the test is disabled because its path (%q) does not match the filter's path (%q)", absoluteFilePath, f.AbsolutePath)
		}
	}

	if !f.NameRegex.MatchString(name) {
		return false, fmt.Sprintf(
			"the test is disabled because its name (%q) does not match the filter's name regex (%q)", name, f.NameRegex.String())
	}

	if !reflect.ValueOf(f.NodeSpan).IsZero() && statementSpan != f.NodeSpan {
		return false, fmt.Sprintf(
			"the test is disabled because its span (%q:%q) does not match the filter's span (%d:%d)",
			statementSpan.Start, statementSpan.End, f.NodeSpan.Start, f.NodeSpan.End)
	}

	return true, "the test is enabled because it matches the filter " + f.String()
}

// getTestItemFilesystem retrieves or create the filesystem for a test item.
//   - if the test item has a filesystem snapshot then a filesystem is created from it.
//   - else if the parent test suite is nil then the parent state's filesystem is returned.
//   - else if the parent test suite is configured to pass a shapshot of its live filesystem
//     then its filesystem is snapshoted and a filesystem is created from it.
//   - else if the parent test suite has a filesystem snapshot then a filesystem is created from it.
//   - else the parent state's filesystem is returned.
func getTestItemFilesystem(test TestItem, parentTestSuite *TestSuite, spawnerState *GlobalState) (afs.Filesystem, error) {
	if snapshot, ok := test.FilesystemSnapshot(); ok {
		return snapshot.NewAdaptedFilesystem(TEST__MAX_FS_STORAGE_HINT)
	} else if parentTestSuite == nil {
		return spawnerState.Ctx.GetFileSystem(), nil
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
			return filesystem, nil
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
			return filesystem, nil
		} else {
			return spawnerState.Ctx.GetFileSystem(), nil
		}
	}
}
