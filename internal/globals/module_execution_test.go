package internal

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mod"

	//_ "github.com/inoxlang/inox/internal/obsdb"

	"github.com/inoxlang/inox/internal/globals/fs_ns"

	"github.com/stretchr/testify/assert"
)

func TestRunLocalModule(t *testing.T) {

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

		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		ctx := createEvaluationCtx(dir)
		defer ctx.CancelGracefully()

		state, _, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContextRequired:     true,
			ParentContext:             ctx,
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
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

		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		ctx := createEvaluationCtx(dir)
		defer ctx.CancelGracefully()

		state, _, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContextRequired:     true,
			ParentContext:             ctx,
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		assert.Error(t, err)
		assert.Nil(t, state)
	})

	t.Run("too many warnings", func(t *testing.T) {
		t.SkipNow()

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		manySpawnExprs := strings.Repeat("go do idt(1)\n", mod.DEFAULT_MAX_ALLOWED_WARNINGS+1)

		os.WriteFile(file, []byte("manifest {}\n"+manySpawnExprs), 0o600)

		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		ctx := createEvaluationCtx(dir)
		defer ctx.CancelGracefully()

		state, _, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContextRequired:     true,
			ParentContext:             ctx,
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.ErrorIs(t, err, mod.ErrExecutionAbortedTooManyWarnings) {
			return
		}

		assert.Nil(t, state)
	})

	t.Run("too wide preinit permissions", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		ctx := createEvaluationCtx(dir)
		defer ctx.CancelGracefully()

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

		state, _, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContextRequired:     true,
			ParentContext:             ctx,
			Out:                       io.Discard,
			IgnoreHighRiskScore:       false, //<---
			PreinitFilesystem:         preinitFs,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.ErrorIs(t, err, mod.ErrNoProvidedConfirmExecPrompt) {
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

func randS3Host() core.Host {
	return core.Host("s3://bucket-" + strconv.Itoa(int(rand.Int31())))
}
