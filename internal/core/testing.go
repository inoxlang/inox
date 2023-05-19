package internal

import (
	"fmt"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

// A TestSuite represents a test suite, TestSuite implements Value.
type TestSuite struct {
	NoReprMixin
	NotClonableMixin

	meta         Value
	module       *Module // module executed when running the test suite
	parentModule *Module
}

func NewTestSuite(meta Value, embeddedModChunk *parse.Chunk, parentState *GlobalState) (*TestSuite, error) {

	parsedChunk := &parse.ParsedChunk{
		Node:   embeddedModChunk,
		Source: parentState.Module.MainChunk.Source,
	}

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

	return &TestSuite{
		meta:         meta,
		module:       routineMod,
		parentModule: parentState.Module,
	}, nil
}

func (s *TestSuite) Run(ctx *Context, options ...Option) (*Routine, error) {
	var timeout time.Duration

	for _, opt := range options {
		switch opt.Name {
		case "timeout":
			if timeout != 0 {
				return nil, FmtErrOptionProvidedAtLeastTwice("timeout")
			}
			timeout = time.Duration(opt.Value.(Duration))
		default:
			return nil, FmtErrInvalidOptionName(opt.Name)
		}
	}

	spawnerState := ctx.GetClosestState()

	createRoutinePerm := RoutinePermission{Kind_: permkind.Create}

	if err := spawnerState.Ctx.CheckHasPermission(createRoutinePerm); err != nil {
		return nil, fmt.Errorf("testing: following permission is required for running tests: %w", err)
	}

	manifest, err := s.module.EvalManifest(ManifestEvaluationConfig{
		RunningState: NewTreeWalkStateWithGlobal(spawnerState),
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
	permissions = append(permissions, createRoutinePerm)

	routineCtx := NewContext(ContextConfig{
		Kind:            TestingContext,
		ParentContext:   ctx,
		Permissions:     permissions,
		Limitations:     manifest.Limitations,
		HostResolutions: manifest.HostResolutions,
	})

	routine, err := SpawnRoutine(RoutineSpawnArgs{
		SpawnerState: spawnerState,
		RoutineCtx:   routineCtx,
		Globals:      spawnerState.Globals,
		Module:       s.module,
		Timeout:      timeout,
	})

	if err != nil {
		return nil, fmt.Errorf("testing: %w", err)
	}
	return routine, nil
}

func (s *TestSuite) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "run":
		return &GoFunction{fn: s.Run}, true
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
	NoReprMixin
	NotClonableMixin

	meta         Value
	module       *Module // module executed when running the test case
	parentModule *Module
}

func NewTestCase(meta Value, modChunk *parse.Chunk, parentState *GlobalState) (*TestCase, error) {
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

	return &TestCase{
		meta:         meta,
		module:       routineMod,
		parentModule: parentState.Module,
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

func (s *TestCase) Run(ctx *Context, options ...Option) (*Routine, error) {
	var timeout time.Duration

	for _, opt := range options {
		switch opt.Name {
		case "timeout":
			if timeout != 0 {
				return nil, FmtErrOptionProvidedAtLeastTwice("timeout")
			}
			timeout = time.Duration(opt.Value.(Duration))
		default:
			return nil, FmtErrInvalidOptionName(opt.Name)
		}
	}

	spawnerState := ctx.GetClosestState()

	manifest, err := s.module.EvalManifest(ManifestEvaluationConfig{
		RunningState: NewTreeWalkStateWithGlobal(spawnerState),
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
		Limitations:     manifest.Limitations,
		HostResolutions: manifest.HostResolutions,
	})

	routine, err := SpawnRoutine(RoutineSpawnArgs{
		SpawnerState: spawnerState,
		RoutineCtx:   routineCtx,
		Globals:      spawnerState.Globals,
		Module:       s.module,
		Timeout:      timeout,
	})

	if err != nil {
		return nil, fmt.Errorf("testing: %w", err)
	}
	return routine, nil
}

func (s *TestCase) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "run":
		return &GoFunction{fn: s.Run}, true
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
