package internal

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/inoxlang/inox/internal/core"
	_fs "github.com/inoxlang/inox/internal/globals/fs"

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
			Filesystem:  _fs.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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
			Filesystem:  _fs.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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
			Filesystem: _fs.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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
			Filesystem: _fs.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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
			Filesystem: _fs.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{}, //missing file argument
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{"true"}, //too many arguments
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{"-x"}, //unknown argument
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath:                     file,
			Args:                      core.NewObjectFromMap(core.ValMap{}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		res, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0": core.Path("./a.txt"),
			}, state),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             state,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0": core.True,
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0": core.True,
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"0":      core.Path("./a.txt"),
				"output": core.True,
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"outpu": core.True, //unknown argument
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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

		state, mod, _, err := PrepareLocalScript(ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewObjectFromMap(core.ValMap{
				"x": core.True, //unknown argument
			}, ctx),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			UseContextAsParent:        true,
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
			Filesystem:  _fs.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)
		return ctx
	}

	//TODO: improve tests

	t.Run("a script with static check errors should not be runned", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		os.WriteFile(file, []byte("fn(){self}; return 1"), 0o600)

		state, _, _, err := RunLocalScript(RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			UseContextAsParent:        true,
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

		manySpawnExprs := strings.Repeat("go do idt(1)\n", DEFAULT_MAX_ALLOWED_WARNINGS+1)

		os.WriteFile(file, []byte("manifest {}\n"+manySpawnExprs), 0o600)

		state, _, _, err := RunLocalScript(RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			UseContextAsParent:        true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
		})

		if !assert.ErrorIs(t, err, ErrExecutionAbortedTooManyWarnings) {
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
		Filesystem: _fs.GetOsFilesystem(),
	})
	core.NewGlobalState(compilationCtx)
	return compilationCtx
}
