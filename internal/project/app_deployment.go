package project

import (
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/permkind"
)

type ApplicationDeploymentPreparationParams struct {
	ModulePath       core.Path
	AppName          string
	UpdateRunningApp bool
}

func (p *Project) PrepareApplicationDeployment(args ApplicationDeploymentPreparationParams) (node.ApplicationDeployment, error) {
	modulePath := args.ModulePath
	if modulePath.IsDirPath() {
		return nil, fmt.Errorf("unexpected directory path: %s", modulePath)
	}

	appName, err := node.ApplicationNameFrom(args.AppName)
	if err != nil {
		return nil, err
	}

	baseImg, err := p.BaseImage()
	if err != nil {
		return nil, err
	}

	//TODO: Create a readonly memory filesystem that only includes necessary files for parsing (.ix files, ...).
	//TODO: This can be difficult because the parsing and analysis may require non-code files.
	fls := fs_ns.NewMemFilesystemFromSnapshot(baseImg.FilesystemSnapshot(), 10_000_000)

	parsingCtx := core.NewContexWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.ROOT_PREFIX_PATH_PATTERN},
		},
		DoNotSpawnDoneGoroutine:   true,
		DoNotSetFilesystemContext: true,
		Filesystem:                fls,
	}, nil)
	defer parsingCtx.CancelGracefully()

	mod, err := core.ParseLocalModule(modulePath.UnderlyingString(), core.ModuleParsingConfig{Context: parsingCtx})

	if err != nil {
		return nil, err
	}

	if mod.ModuleKind != core.ApplicationModule {
		if mod.ModuleKind == core.UnspecifiedModuleKind {
			return nil,
				fmt.Errorf("module %s is not of kind 'application': make sure to add a section `kind: \"application\"` in its manifest", modulePath)
		}
		return nil, fmt.Errorf("module %s is of kind '%s' not 'application'", modulePath, mod.ModuleKind)
	}

	agent := node.GetAgent()
	app, err := agent.GetOrCreateApplication(appName)
	if err != nil {
		return nil, err
	}

	return app.PrepareDeployment(node.ApplicationDeploymentParams{
		AppMod:           mod,
		BaseImg:          baseImg,
		UpdateRunningApp: args.UpdateRunningApp,
	})
}
