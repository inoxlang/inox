package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"

	"github.com/stretchr/testify/assert"
)

func TestPrepareLocalScript(t *testing.T) {

	t.Run("recoverable parsing error", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
			a = ;
			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present because we can still make perform static check
		if !assert.NotNil(t, state) {
			return
		}
		if !assert.True(t, state.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("preinit block defines a pattern used in the manifest", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			preinit {
				%patt = %/...
			}
			manifest {
				permissions: {
					read: %patt
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.True(t, state.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("preinit block defines a host alias used in the manifest", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			preinit {
				@host = https://localhost
			}
			manifest {
				permissions: {
					read: @host/
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.True(t, state.Ctx.HasPermission(core.CreateHttpReadPerm(core.URL("https://localhost/")))) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("preinit-files", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				preinit-files: {
					FILE: {
						path: /file.txt
						pattern: %str
					}
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		preinitFs := fs_ns.NewMemFilesystem(100)
		util.WriteFile(preinitFs, "/file.txt", nil, 0o600)

		state, mod, manifest, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem: preinitFs,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		//the manifest should contain the preinit config.

		if !assert.Len(t, manifest.PreinitFiles, 1) {
			return
		}

		assert.Equal(t, &core.PreinitFile{
			Name:    "FILE",
			Path:    "/file.txt",
			Pattern: core.STR_PATTERN,
			Content: []byte{},
			Parsed:  core.Str(""),
			RequiredPermission: core.FilesystemPermission{
				Kind_:  permkind.Read,
				Entity: core.Path("/file.txt"),
			},
		}, manifest.PreinitFiles[0])

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("local database", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
					write: %/...
				}
				databases: {
					local: {
						resource: ldb://main
						resolution-data: /
					}
				}
			}
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(1000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
			),
			Filesystem: fs,
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		//the state should contain the database.

		if !assert.Contains(t, state.Databases, "local") {
			return
		}

		assert.Equal(t, core.Host("ldb://main"), state.Databases["local"].Resource())
	})

	t.Run("manifest & symbolic eval should be ignored when there is a preinit check error", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			preinit {
				go do {}
			}
			manifest {
				permissions: {
					read: https://localhost
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// manifest should be empty
		if !assert.False(t, state.Ctx.HasPermission(core.CreateHttpReadPerm(core.URL("https://localhost/")))) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("manifest & symbolic eval should be ignored when there is a manifest check error", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: https://localhost
				}
				env: 1
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// manifest should be empty
		if !assert.False(t, state.Ctx.HasPermission(core.CreateHttpReadPerm(core.URL("https://localhost/")))) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("invalid CLI arguments: missing positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path}
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{}, //missing file argument
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("CLI: too many positional arguments", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{"true"}, //too many arguments
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("CLI: unknown argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{"-x"}, //unknown argument
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("missing positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path}
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			Args:                      core.NewObjectFromMap(core.ValMap{}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("missing non positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path},
					output: %path
				}
			}
		
		`), 0o600)

		state := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(state)

		res, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0": core.Path("./a.txt"),
			}, state),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             state,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, res) {
			return
		}
	})

	t.Run("invalid value for positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path}
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0": core.True,
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("too many positional arguments", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0": core.True,
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("invalid value for non positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path},
					output: %path
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0":      core.Path("./a.txt"),
				"output": core.True,
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("unknown non positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					output: %path
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"outpu": core.True, //unknown argument
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("unknown argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"x": core.True, //unknown argument
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

}

func TestRunLocalScript(t *testing.T) {

	createEvaluationCtx := func(dir string) *core.Context {
		perms := core.GetDefaultGlobalVarPermissions()
		perms = append(perms, core.CreateFsReadPerm(core.PathPattern(dir+"/...")))

		ctx := core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)
		return ctx
	}

	//TODO: improve tests

	t.Run("a script with static check errors should not be runned", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		os.WriteFile(file, []byte("fn(){self}; return 1"), 0o600)

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
		})

		assert.Error(t, err)
		assert.Nil(t, state)
	})

	t.Run("a script with static check errors in the preinit-files section should not be runned", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		os.WriteFile(file, []byte(`
			manifest {
				preinit-files: {
					A: {path: /a, pattern: %str},
					A: {path: /b, pattern: %str},
				}
			}
		`), 0o600)

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
		})

		assert.Error(t, err)
		assert.Nil(t, state)
	})

	t.Run("too many warnings", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		manySpawnExprs := strings.Repeat("go do idt(1)\n", inox_ns.DEFAULT_MAX_ALLOWED_WARNINGS+1)

		os.WriteFile(file, []byte("manifest {}\n"+manySpawnExprs), 0o600)

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
		})

		if !assert.ErrorIs(t, err, inox_ns.ErrExecutionAbortedTooManyWarnings) {
			return
		}

		assert.Nil(t, state)
	})

	t.Run("too wide preinit permissions", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		os.WriteFile(file, []byte(`
			manifest {
				preinit-files: {
					FILE1: {
						path: /file1.txt
						pattern: %str
					}
					FILE2: {
						path: /file2.txt
						pattern: %str
					}
					FILE3: {
						path: /file3.txt
						pattern: %str
					}
					FILE4: {
						path: /file4.txt
						pattern: %str
					}
					FILE5: {
						path: /file5.txt
						pattern: %str
					}
					FILE6: {
						path: /file6.txt
						pattern: %str
					}
					FILE7: {
						path: /file7.txt
						pattern: %str
					}
					FILE8: {
						path: /file8.txt
						pattern: %str
					}
					FILE9: {
						path: /file9.txt
						pattern: %str
					}
				}
			}
		`), 0o600)

		preinitFs := fs_ns.NewMemFilesystem(100)
		for i := 1; i <= 9; i++ {
			util.WriteFile(preinitFs, fmt.Sprintf("/file%d.txt", i), nil, 0o600)
		}

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       false, //<---
			PreinitFilesystem:         preinitFs,
		})

		if !assert.ErrorIs(t, err, inox_ns.ErrNoProvidedConfirmExecPrompt) {
			return
		}

		assert.Nil(t, state)
	})
}

func createCompilationCtx(dir string) *core.Context {
	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.CreateFsReadPerm(core.PathPattern(dir + "/...")),
		},
		Filesystem: fs_ns.GetOsFilesystem(),
	})
	core.NewGlobalState(compilationCtx)
	return compilationCtx
}
