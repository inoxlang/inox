package internal

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	core "github.com/inox-project/inox/internal/core"

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
		})
		core.NewGlobalState(ctx)

		res, mod, err := PrepareLocalScript(ScriptPreparationArgs{
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
		if !assert.NotNil(t, res) {
			return
		}
		if !assert.True(t, res.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, res.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, res.SymbolicData.IsEmpty())
	})
}

func TestRunLocalScript(t *testing.T) {

	t.Run("a script with static check errors should not be runned", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		ctx := createCompilationCtx(dir)

		os.WriteFile(file, []byte("fn(){self}; return 1"), 0o600)

		res, _, _, err := RunLocalScript(RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: ctx,
			UseContextAsParent:        true,
			ParentContext:             ctx,
			Out:                       io.Discard,
		})

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func createCompilationCtx(dir string) *core.Context {
	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.CreateFsReadPerm(core.PathPattern(dir + "/...")),
		},
	})
	core.NewGlobalState(compilationCtx)
	return compilationCtx
}
