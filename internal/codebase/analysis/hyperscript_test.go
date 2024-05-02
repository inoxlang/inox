package analysis

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeHyperscript(t *testing.T) {

	setup := func() *core.Context {
		newMemFS := func() *fs_ns.MemFilesystem {
			return fs_ns.NewMemFilesystem(100_000)
		}

		return core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")}},
			Filesystem:  newMemFS(),
		}, nil)

	}

	t.Run("attribute shorthand", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte("manifest{}; return html<div {on click toggle .red}></div>"), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedUsedCommands := map[string]hsgen.Definition{
			"toggle": utils.MustGet(hsgen.GetBuiltinCommandDefinition("toggle")),
		}

		expectedUseFeatures := map[string]hsgen.Definition{
			"on": utils.MustGet(hsgen.GetBuiltinFeatureDefinition("on")),
		}

		assert.Equal(t, expectedUsedCommands, result.UsedHyperscriptCommands)
		assert.Equal(t, expectedUseFeatures, result.UsedHyperscriptFeatures)
	})

	t.Run("script", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte("manifest{}; return html<script h>on click toggle .red></script>"), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedUsedCommands := map[string]hsgen.Definition{
			"toggle": utils.MustGet(hsgen.GetBuiltinCommandDefinition("toggle")),
		}

		expectedUseFeatures := map[string]hsgen.Definition{
			"on": utils.MustGet(hsgen.GetBuiltinFeatureDefinition("on")),
		}

		assert.Equal(t, expectedUsedCommands, result.UsedHyperscriptCommands)
		assert.Equal(t, expectedUseFeatures, result.UsedHyperscriptFeatures)
	})

	t.Run("usage of a feature that is also a command", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte("manifest{}; return html<div {on click set a to 1}></div>"), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedUsedCommands := map[string]hsgen.Definition{
			"set": utils.MustGet(hsgen.GetBuiltinCommandDefinition("set")),
		}

		expectedUseFeatures := map[string]hsgen.Definition{
			"on":  utils.MustGet(hsgen.GetBuiltinFeatureDefinition("on")),
			"set": utils.MustGet(hsgen.GetBuiltinFeatureDefinition("set")),
		}

		assert.Equal(t, expectedUsedCommands, result.UsedHyperscriptCommands)
		assert.Equal(t, expectedUseFeatures, result.UsedHyperscriptFeatures)
	})

	t.Run("component", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			ctx := setup()
			defer ctx.CancelGracefully()

			util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}
			return html<div class="Counter" {init}></div>
		`), 0600)

			result, err := AnalyzeCodebase(ctx, Configuration{
				TopDirectories: []string{"/"},
			})

			if !assert.NoError(t, err) {
				return
			}

			mod := result.InoxModules["/routes/index.ix"]
			markupElem := parse.FindFirstNode(mod.Module.MainChunk.Node, (*parse.MarkupElement)(nil))

			if !assert.Len(t, result.HyperscriptComponents, 1) {
				return
			}

			components := result.HyperscriptComponents["Counter"]

			if !assert.Len(t, components, 1) {
				return
			}

			expectedComponent := &HyperscriptComponent{
				Name:               "Counter",
				Element:            markupElem,
				AttributeShorthand: markupElem.Opening.Attributes[1].(*parse.HyperscriptAttributeShorthand),
				ChunkSource:        mod.Module.MainChunk,
			}

			assert.Equal(t, expectedComponent, components[0])
		})

		t.Run("component with init feature and event handling", func(t *testing.T) {
			ctx := setup()
			defer ctx.CancelGracefully()

			util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
					manifest{}
					return html<div class="Counter" {
						init 
							set :count to 0
						on incr
							increment :count
						on decr
							decrement :count
					}>
					</div>
				`), 0600)

			result, err := AnalyzeCodebase(ctx, Configuration{
				TopDirectories: []string{"/"},
			})

			if !assert.NoError(t, err) {
				return
			}

			mod := result.InoxModules["/routes/index.ix"]
			markupElem := parse.FindFirstNode(mod.Module.MainChunk.Node, (*parse.MarkupElement)(nil))

			if !assert.Len(t, result.HyperscriptComponents, 1) {
				return
			}

			components := result.HyperscriptComponents["Counter"]

			if !assert.Len(t, components, 1) {
				return
			}

			expectedComponent := &HyperscriptComponent{
				Name:                        "Counter",
				Element:                     markupElem,
				AttributeShorthand:          markupElem.Opening.Attributes[1].(*parse.HyperscriptAttributeShorthand),
				ChunkSource:                 mod.Module.MainChunk,
				InitialElementScopeVarNames: []string{":count"},
				HandledEvents: []DOMEvent{
					{
						Type: "incr",
					},
					{
						Type: "decr",
					},
				},
			}

			assert.Equal(t, expectedComponent, components[0])
		})
	})

}
