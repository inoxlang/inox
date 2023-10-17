package core

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

// A TestItem is a TestSuite or a TestCase.
type TestItem interface {
	ItemName() (string, bool)
	ParentChunk() *parse.ParsedChunk
	ParentModule() *Module
}

// A TestSuite represents a test suite, TestSuite implements Value.
type TestSuite struct {
	meta         Value
	nameFromMeta string

	module       *Module // module executed when running the test suite
	parentModule *Module
	parentChunk  *parse.ParsedChunk
}

func NewTestSuite(meta Value, embeddedModChunk *parse.Chunk, parentChunk *parse.ParsedChunk, parentState *GlobalState) (*TestSuite, error) {

	parsedChunk := &parse.ParsedChunk{
		Node:   embeddedModChunk,
		Source: parentState.Module.MainChunk.Source,
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

	switch m := meta.(type) {
	case StringLike:
		nameFromMeta = m.GetOrBuildString()
	case *Record:
		m.ForEachEntry(func(k string, v Value) error {
			switch k {
			case "name":
				strLike, ok := v.(StringLike)
				if ok {
					nameFromMeta = strLike.GetOrBuildString()
				}
			}
			return nil
		})
	case NilT:
	default:
		panic(ErrUnreachable)
	}

	return &TestSuite{
		meta:         meta,
		module:       routineMod,
		parentModule: parentState.Module,
		parentChunk:  parentChunk,
		nameFromMeta: nameFromMeta,
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

	createLthreadPerm := LThreadPermission{Kind_: permkind.Create}

	if err := spawnerState.Ctx.CheckHasPermission(createLthreadPerm); err != nil {
		return nil, fmt.Errorf("testing: following permission is required for running tests: %w", err)
	}

	manifest, _, _, err := s.module.PreInit(PreinitArgs{
		RunningState: NewTreeWalkStateWithGlobal(spawnerState),
		ParentState:  spawnerState,

		//TODO: should Project be set ?
	})

	if err != nil {
		return nil, err
	}

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
	})

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: spawnerState,
		LthreadCtx:   routineCtx,
		Globals:      spawnerState.Globals,
		Module:       s.module,
		Manifest:     manifest,
		Timeout:      timeout,
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
	meta         Value
	nameFromMeta string //can be empty

	module       *Module // module executed when running the test case
	parentModule *Module
	parentChunk  *parse.ParsedChunk

	positionStack     parse.SourcePositionStack //can be nil
	formattedPosition string                    //can be empty
}

func NewTestCase(
	meta Value,
	modChunk *parse.Chunk,
	parentState *GlobalState,
	parentChunk *parse.ParsedChunk,

	//optional
	positionStack parse.SourcePositionStack,
	formattedPosition string,
) (*TestCase, error) {
	parsedChunk := &parse.ParsedChunk{
		Node:   modChunk,
		Source: parentState.Module.MainChunk.Source,
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

	switch m := meta.(type) {
	case StringLike:
		nameFromMeta = m.GetOrBuildString()
	case *Record:
		m.ForEachEntry(func(k string, v Value) error {
			switch k {
			case "name":
				strLike, ok := v.(StringLike)
				if ok {
					nameFromMeta = strLike.GetOrBuildString()
				}
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
		meta:         meta,
		nameFromMeta: nameFromMeta,
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

func (s *TestCase) Run(ctx *Context, options ...Option) (*LThread, error) {
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

	manifest, _, _, err := s.module.PreInit(PreinitArgs{
		RunningState: NewTreeWalkStateWithGlobal(spawnerState),
		ParentState:  spawnerState,

		//TODO: should Project be set ?
	})

	if err != nil {
		return nil, err
	}

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
	})

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: spawnerState,
		LthreadCtx:   routineCtx,
		Globals:      spawnerState.Globals,
		Module:       s.module,
		Manifest:     manifest,
		Timeout:      timeout,
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

	if !strings.HasSuffix(srcName, "/") {
		return false, "the test is disabled because it is not located in a local file or the file's path is not absolute"
	}

	_ = itemName

	//absoluteFilePath := srcName

	// for _, negativeFilter := range filters.NegativeTestFilters {
	// 	if disabled, reason := negativeFilter.IsTestEnabled(absoluteFilePath, itemName, statementSpan); disabled {
	// 		return false, fmt.Sprintf(
	// 			"the test is disabled because its path (%q) does not match the filter's path pattern (%q)", absoluteFilePath, f.AbsolutePath)
	// 	}
	// }

	// for _, positiveFilter := range filters.PositiveTestFilters {
	// 	if positiveFilter.Test(absoluteFilePath, itemName, statementSpan) {

	// 		return true
	// 	}
	// }

	return false, ""
}

// A TestFilter filter tests by checking several values.
type TestFilter struct {
	//if path ends with '/...' all tests found in subdirectories are also enabled.
	AbsolutePath string

	//never nil
	NameRegex *regexp.Regexp

	//span of the test suite or test case statement, this field is ignored if it is equal to the zero value.
	NodeSpan parse.NodeSpan
}

func (f TestFilter) String() string {
	if reflect.ValueOf(f.NodeSpan).IsZero() {
		return fmt.Sprintf("[path(s):%s name:%s]", f.AbsolutePath, f.NameRegex.String())
	}
	return fmt.Sprintf("[path(s):%s span:%d,%d name:%s]", f.AbsolutePath, f.NodeSpan.Start, f.NodeSpan.End, f.NameRegex.String())
}

func (f TestFilter) IsTestEnabled(absoluteFilePath string, name string, statementSpan parse.NodeSpan) (enabled bool, reason string) {
	if strings.HasSuffix(f.AbsolutePath, "/...") {
		if !PathPattern(f.AbsolutePath).Test(nil, Str(absoluteFilePath)) {
			return false, fmt.Sprintf(
				"the test is disabled because its path (%q) does not match the filter's path pattern (%q)", absoluteFilePath, f.AbsolutePath)
		}
	} else if absoluteFilePath != f.AbsolutePath {
		return false, fmt.Sprintf(
			"the test is disabled because its path (%q) does not match the filter's path (%q)", absoluteFilePath, f.AbsolutePath)
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
