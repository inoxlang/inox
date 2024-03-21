package core

import (
	"embed"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"
)

const (
	SINGLE_MOD_TRANSPILATION_TIMEOUT = 2 * time.Second
)

var (
	InoxCodebaseFS embed.FS
)

type AppTranspilationParams struct {
	MainModule       ResourceName
	PreparedModules  map[ResourceName]*PreparationCacheEntry
	Config           AppTranspilationConfig
	ParentContext    *Context
	ThreadSafeLogger zerolog.Logger
}

type AppTranspilationConfig struct {
}

type Transpiler struct {
	//input
	mainModule      ResourceName
	preparedModules map[ResourceName]*PreparationCacheEntry
	config          AppTranspilationConfig
	ctx             *Context
	logger          zerolog.Logger

	//state
	nonMainModuleTranspilations map[ResourceName]*moduleTranspilationState

	//trace
	trace  io.Writer
	indent int
}

func TranspileApp(args AppTranspilationParams) (*TranspiledApp, error) {
	transpiler := &Transpiler{
		//input
		mainModule:      args.MainModule,
		preparedModules: args.PreparedModules,
		ctx:             args.ParentContext.BoundChildWithOptions(BoundChildContextOptions{}),
		logger:          args.ThreadSafeLogger,

		config: args.Config,

		//state
		nonMainModuleTranspilations: make(map[ResourceName]*moduleTranspilationState),
	}

	//	pkg: gen.NewPkg("main"),

	//transpiler.pkg.AddFile("main.go", gen.NewMainFile())

	defer transpiler.ctx.CancelGracefully()
	return transpiler.transpileApp()
}

func (t *Transpiler) transpileApp() (*TranspiledApp, error) {

	var mainModuleTranspilation *moduleTranspilationState
	packageIDs := map[string]ResourceName{}

	for resourceName, prepared := range t.preparedModules {

		state, err := t.newModuleTranspilationState(resourceName, prepared)
		if err != nil {
			return nil, err
		}

		if resourceName == t.mainModule {
			mainModuleTranspilation = state
		} else {
			t.nonMainModuleTranspilations[resourceName] = state
		}

		if otherModName, ok := packageIDs[state.pkgID]; ok {
			return nil, fmt.Errorf("unexpected: modules %s and %s have the same Go package ID (%s)", resourceName, otherModName, state.pkgID)
		}
		packageIDs[state.pkgID] = resourceName
	}

	if mainModuleTranspilation == nil {
		return nil, fmt.Errorf("the main module %s was not found in prepared modules", t.mainModule)
	}

	//Check that no modules share the same package ID.

	//Transpile all modules apart from the main one.

	for _, state := range t.nonMainModuleTranspilations {
		if t.mainModule == state.moduleName {
			continue
		}
		go t.transpileModule(state)
	}

	transpiledModules := map[ResourceName]*TranspiledModule{}

	for resourceName, state := range t.nonMainModuleTranspilations {
		select {
		case err := <-state.endChan:
			if err != nil {
				return nil, err
			}
		case <-time.After(SINGLE_MOD_TRANSPILATION_TIMEOUT):
			return nil, fmt.Errorf("transpiling %s takes too much time: abort application transpilation", resourceName)
		}
		transpiledModules[resourceName] = state.transpiledModule
	}

	//Transpile the main module.

	go t.transpileModule(mainModuleTranspilation)

	select {
	case err := <-mainModuleTranspilation.endChan:
		if err != nil {
			return nil, err
		}
	case <-time.After(SINGLE_MOD_TRANSPILATION_TIMEOUT):
		return nil, fmt.Errorf("transpiling %s takes too much time: abort application transpilation", t.mainModule)
	}

	transpiledModules[t.mainModule] = mainModuleTranspilation.transpiledModule

	app := &TranspiledApp{
		mainModuleName:      t.mainModule,
		mainPkg:             mainModuleTranspilation.pkg,
		inoxModules:         transpiledModules,
		transpilationConfig: t.config,
	}

	return app, nil
}
