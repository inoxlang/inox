package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

func getScriptDir(fpath string) string {
	dir := filepath.Dir(fpath)
	dir, _ = filepath.Abs(dir)
	dir = core.AppendTrailingSlashIfNotPresent(dir)
	return dir
}

func runStartupScript(startupScriptPath string, processTempDirPerms []core.Permission, outW io.Writer) (*core.Object, *core.GlobalState) {
	initialWorkingDir := utils.Must(os.Getwd())

	//we read, parse and evaluate the startup script

	absPath, err := filepath.Abs(startupScriptPath)
	if err != nil {
		panic(err)
	}
	startupScriptPath = absPath

	parsingCtx := core.NewContext(core.ContextConfig{
		Permissions:             []core.Permission{core.CreateFsReadPerm(core.Path(startupScriptPath))},
		Filesystem:              fs_ns.GetOsFilesystem(),
		InitialWorkingDirectory: core.DirPathFrom(initialWorkingDir),
	})
	{
		state := core.NewGlobalState(parsingCtx)
		state.Out = outW
		state.Logger = zerolog.New(outW)
		state.OutputFieldsInitialized.Store(true)
	}
	defer parsingCtx.CancelGracefully()

	startupMod, err := core.ParseLocalModule(startupScriptPath, core.ModuleParsingConfig{
		Context: parsingCtx,
	})
	if err != nil {
		panic(fmt.Errorf("failed to parse startup script: %w", err))
	}

	startupManifest, _, _, err := startupMod.PreInit(core.PreinitArgs{
		GlobalConsts:          startupMod.MainChunk.Node.GlobalConstantDeclarations,
		AddDefaultPermissions: true,
	})

	if err != nil {
		panic(fmt.Errorf("failed to evalute startup script's manifest: %w", err))
	}

	ctx := utils.Must(core.NewDefaultContext(core.DefaultContextConfig{
		Permissions:             append(slices.Clone(startupManifest.RequiredPermissions), processTempDirPerms...),
		Limits:                  startupManifest.Limits,
		HostDefinitions:         startupManifest.HostDefinitions,
		InitialWorkingDirectory: core.DirPathFrom(initialWorkingDir),
		Filesystem:              fs_ns.GetOsFilesystem(),
	}))
	state, err := core.NewDefaultGlobalState(ctx, core.DefaultGlobalStateConfig{
		Out:    outW,
		LogOut: outW,
	})
	if err != nil {
		panic(fmt.Errorf("failed to startup script's global state: %w", err))
	}
	state.Manifest = startupManifest
	state.Module = startupMod
	state.MainState = state

	//

	staticCheckData, err := core.StaticCheck(core.StaticCheckInput{
		State:             state,
		Node:              startupMod.MainChunk.Node,
		Chunk:             startupMod.MainChunk,
		Patterns:          state.Ctx.GetNamedPatternNames(),
		PatternNamespaces: state.Ctx.GetPatternNamespaceNames(),
	})
	state.StaticCheckData = staticCheckData

	if err != nil {
		panic(fmt.Sprint("startup script: ", err.Error()))
	}

	//

	startupResult, err := core.TreeWalkEval(startupMod.MainChunk.Node, core.NewTreeWalkStateWithGlobal(state))
	if err != nil {
		panic(fmt.Sprint("startup script failed:", err))
	}

	if object, ok := startupResult.(*core.Object); !ok {
		panic(fmt.Sprintf("startup script should return an Object or nothing (nil), not a(n) %T", startupResult))
	} else {
		return object, state
	}
}
