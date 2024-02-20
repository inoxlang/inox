package projectserver

import (
	"io"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSignatureHelpAt(t *testing.T) {

	if core.NewDefaultContext == nil {
		core.SetNewDefaultContext(func(config core.DefaultContextConfig) (*core.Context, error) {
			ctx := core.NewContext(core.ContextConfig{
				Permissions:   config.Permissions,
				ParentContext: config.ParentContext,
				Filesystem:    config.Filesystem,
			})

			for name, pattern := range core.DEFAULT_NAMED_PATTERNS {
				ctx.AddNamedPattern(name, pattern)
			}
			return ctx, nil
		})
		core.SetNewDefaultGlobalStateFn(func(ctx *core.Context, conf core.DefaultGlobalStateConfig) (*core.GlobalState, error) {
			state := core.NewGlobalState(ctx)
			state.Out = io.Discard
			state.Logger = zerolog.Nop()
			state.OutputFieldsInitialized.Store(true)
			return state, nil
		})
		defer core.UnsetNewDefaultContext()
		defer core.UnsetNewDefaultGlobalStateFn()
	}

	setup := func(code string) (*core.GlobalState, bool) {
		fls := fs_ns.NewMemFilesystem(1_000)
		util.WriteFile(fls, "/main.ix", []byte(code), 0600)

		parsingCtx := core.NewContextWithEmptyState(core.ContextConfig{
			Filesystem:  fls,
			Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
		}, nil)
		defer parsingCtx.CancelGracefully()

		state, _, _, _ := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     "/main.ix",
			ParsingCompilationContext: parsingCtx,
			PreinitFilesystem:         fls,
			ScriptContextFileSystem:   fls,
			Out:                       io.Discard,
			LogOut:                    io.Discard,
			DataExtractionMode:        true,
		})
		defer state.Ctx.CancelGracefully()

		if !assert.NotNil(t, state) {
			return nil, false
		}

		defer state.Ctx.CancelGracefully()
		return state, true
	}

	t.Run("call of a single-param function: no arguments", func(t *testing.T) {
		state, ok := setup("manifest {}\nfn f(arg int){}\nf()")
		if !ok {
			return
		}

		help, ok := getSignatureHelpAt(3, 2, state.Module.MainChunk, state)
		if !assert.True(t, ok) {
			return
		}
		if !assert.NotEmpty(t, help.Signatures) {
			return
		}
		signature := help.Signatures[0]

		assert.EqualValues(t, utils.New(uint(0)), help.ActiveParameter)
		assert.EqualValues(t, "arg int", (*signature.Parameters)[0].Label)
	})

	t.Run("call of a two-param function; no arguments", func(t *testing.T) {
		state, ok := setup("manifest {}\nfn f(a int, b int){}\nf()")
		if !ok {
			return
		}

		help, ok := getSignatureHelpAt(3, 2, state.Module.MainChunk, state)
		if !assert.True(t, ok) {
			return
		}
		if !assert.NotEmpty(t, help.Signatures) {
			return
		}
		signature := help.Signatures[0]

		assert.EqualValues(t, utils.New(uint(0)), help.ActiveParameter)
		assert.EqualValues(t, "a int", (*signature.Parameters)[0].Label)
		assert.EqualValues(t, "b int", (*signature.Parameters)[1].Label)
	})

	t.Run("call of a two-param function; single argument, no comma", func(t *testing.T) {
		state, ok := setup("manifest {}\nfn f(a int, b int){}\nf(1)")
		if !ok {
			return
		}

		help, ok := getSignatureHelpAt(3, 3, state.Module.MainChunk, state)
		if !assert.True(t, ok) {
			return
		}
		if !assert.NotEmpty(t, help.Signatures) {
			return
		}
		signature := help.Signatures[0]

		assert.EqualValues(t, utils.New(uint(0)), help.ActiveParameter)
		assert.EqualValues(t, "a int", (*signature.Parameters)[0].Label)
		assert.EqualValues(t, "b int", (*signature.Parameters)[1].Label)
	})

	t.Run("call of a two-param function; single argument + comma", func(t *testing.T) {
		state, ok := setup("manifest {}\nfn f(a int, b int){}\nf(1,)")
		if !ok {
			return
		}

		help, ok := getSignatureHelpAt(3, 4, state.Module.MainChunk, state)
		if !assert.True(t, ok) {
			return
		}
		if !assert.NotEmpty(t, help.Signatures) {
			return
		}
		signature := help.Signatures[0]

		assert.EqualValues(t, utils.New(uint(1)), help.ActiveParameter)
		assert.EqualValues(t, "a int", (*signature.Parameters)[0].Label)
		assert.EqualValues(t, "b int", (*signature.Parameters)[1].Label)
	})
}
