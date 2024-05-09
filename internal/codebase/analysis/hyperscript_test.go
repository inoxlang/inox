package analysis_test

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis"
	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis/text"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	. "github.com/inoxlang/inox/internal/codebase/analysis"
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

			mod := result.LocalModules["/routes/index.ix"]
			markupExpr := parse.FindFirstNode(mod.Module.MainChunk.Node, (*parse.MarkupExpression)(nil))

			if !assert.Len(t, result.HyperscriptComponents, 1) {
				return
			}

			components := result.HyperscriptComponents["Counter"]

			if !assert.Len(t, components, 1) {
				return
			}

			expectedComponent := &hsanalysis.Component{
				Name:               "Counter",
				Element:            markupExpr.Element,
				ClosestMarkupExpr:  markupExpr,
				AttributeShorthand: markupExpr.Element.Opening.Attributes[1].(*parse.HyperscriptAttributeShorthand),
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

			mod := result.LocalModules["/routes/index.ix"]
			markupExpr := parse.FindFirstNode(mod.Module.MainChunk.Node, (*parse.MarkupExpression)(nil))

			if !assert.Len(t, result.HyperscriptComponents, 1) {
				return
			}

			components := result.HyperscriptComponents["Counter"]

			if !assert.Len(t, components, 1) {
				return
			}

			expectedComponent := &hsanalysis.Component{
				Name:                        "Counter",
				Element:                     markupExpr.Element,
				ClosestMarkupExpr:           markupExpr,
				AttributeShorthand:          markupExpr.Element.Opening.Attributes[1].(*parse.HyperscriptAttributeShorthand),
				ChunkSource:                 mod.Module.MainChunk,
				InitialElementScopeVarNames: []string{":count"},
				HandledEvents: []hsanalysis.DOMEvent{
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

		t.Run("component with some data-* attributes that are initialized and others that depend on interpolations", func(t *testing.T) {
			ctx := setup()
			defer ctx.CancelGracefully()

			util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
					manifest{}
					return html<div class="Counter" data-x="a" data-y="((@data-x))" data-z="b" {}>
					</div>
				`), 0600)

			result, err := AnalyzeCodebase(ctx, Configuration{
				TopDirectories: []string{"/"},
			})

			if !assert.NoError(t, err) {
				return
			}

			mod := result.LocalModules["/routes/index.ix"]
			markupExpr := parse.FindFirstNode(mod.Module.MainChunk.Node, (*parse.MarkupExpression)(nil))

			if !assert.Len(t, result.HyperscriptComponents, 1) {
				return
			}

			components := result.HyperscriptComponents["Counter"]

			if !assert.Len(t, components, 1) {
				return
			}

			expectedComponent := &hsanalysis.Component{
				Name:                          "Counter",
				Element:                       markupExpr.Element,
				ClosestMarkupExpr:             markupExpr,
				AttributeShorthand:            markupExpr.Element.Opening.Attributes[4].(*parse.HyperscriptAttributeShorthand),
				ChunkSource:                   mod.Module.MainChunk,
				InitializedDataAttributeNames: []string{"data-x", "data-z"},
			}

			assert.Equal(t, expectedComponent, components[0])
		})

		t.Run("with x-for attribute and without hyperscript attribute shorthand", func(t *testing.T) {
			ctx := setup()
			defer ctx.CancelGracefully()

			util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
				manifest{}
				return html<div class="Counter" x-for=":e in list"></div>
			`), 0600)

			result, err := AnalyzeCodebase(ctx, Configuration{
				TopDirectories: []string{"/"},
			})

			if !assert.NoError(t, err) {
				return
			}

			mod := result.LocalModules["/routes/index.ix"]
			markupExpr := parse.FindFirstNode(mod.Module.MainChunk.Node, (*parse.MarkupExpression)(nil))

			if !assert.Len(t, result.HyperscriptComponents, 1) {
				return
			}

			components := result.HyperscriptComponents["Counter"]

			if !assert.Len(t, components, 1) {
				return
			}

			expectedComponent := &hsanalysis.Component{
				Name:                        "Counter",
				Element:                     markupExpr.Element,
				ClosestMarkupExpr:           markupExpr,
				ChunkSource:                 mod.Module.MainChunk,
				InitialElementScopeVarNames: []string{":e"},
			}

			assert.Equal(t, expectedComponent, components[0])
		})
	})

}

func TestAnalyzeHyperscriptContainingErrors(t *testing.T) {

	setup := func() *core.Context {
		newMemFS := func() *fs_ns.MemFilesystem {
			return fs_ns.NewMemFilesystem(100_000)
		}

		return core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")}},
			Filesystem:  newMemFS(),
		}, nil)
	}

	t.Run("misplaced element-scoped variable in '_' attribute of root element's tag", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" {init tell closest .Counter set :count to 1}>
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Equal(t, text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD, hyperscriptError.Message)
	})

	t.Run("misplaced element-scoped variable in '_' attribute of non-root element's tag", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" {}>
				<div {init tell closest .Counter set :count to 1}></div>
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Equal(t, text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD, hyperscriptError.Message)
	})

	t.Run("misplaced element-scoped variable in '_' attribute of non-root element's tag that is conditionally included", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" {}>
				{
					if true
						<div {init tell closest .Counter set :count to 1}></div>
					else
						<div></div>
				}
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Equal(t, text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD, hyperscriptError.Message)
	})

	t.Run("error in client-side interpolation in attribute of root element of component", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" a="(())" {}> </div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Contains(t, "Missing expression", hyperscriptError.Message)
	})

	t.Run("parsing error in client-side interpolation in attribute of non-root element of component", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" {}> 
				<div a="(())"></div>
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Contains(t, "Missing expression", hyperscriptError.Message)
	})

	t.Run("error in client-side interpolation in attribute of non-root element of component that is injected through an interpolation", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			child = <div a="(())"></div>
			return html<div class="Counter" {}> 
				{child}
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Contains(t, "Missing expression", hyperscriptError.Message)
	})

	t.Run("error in client-side interpolation in attribute of non-root element of component", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" {}> 
				<div a="(( :non_existing ))"></div>
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Equal(t, text.FmtElementScopeVarMayNotBeDefined(":non_existing", true), hyperscriptError.Message)
	})

	t.Run("parsing error in client-side interpolation in text of root element of component", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" {}> (()) </div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Contains(t, "Missing expression", hyperscriptError.Message)
	})

	t.Run("parsing error in client-side interpolation in text of non-root element of component", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			return html<div class="Counter" {}>
				<div> (()) </div>
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Contains(t, "Missing expression", hyperscriptError.Message)
	})

	t.Run("parsing error in client-side interpolation in text of non-root element of component that is injected through an interpolation", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}; 
			child = <div>(())</div>
			return html<div class="Counter" {}> 
				{child}
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Contains(t, "Missing expression", hyperscriptError.Message)
	})

	t.Run("error in client-side interpolation in text of non-root element of component", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix",
			[]byte(`manifest{}; return html<div class="Counter" {}>  <div>(( :non_existing ))</div> </div>`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Equal(t, text.FmtElementScopeVarMayNotBeDefined(":non_existing", true), hyperscriptError.Message)
		assert.Equal(t, parse.NodeSpan{Start: 57, End: 70}, hyperscriptError.Location.Span)
	})

	t.Run("text interpolation error in component located in an includable file imported by a module", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/counter.ix",
			[]byte(`includable-file; fn Counter() => html<div class="Counter" {}> (( :non_existing )) </div>`), 0600)

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}
			import /counter.ix
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Equal(t, parse.NodeSpan{Start: 65, End: 78}, hyperscriptError.Location.Span)
	})

	t.Run("attribute interpolation error in component located in an includable file imported by a module", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/counter.ix",
			[]byte(`includable-file; fn Counter() => html<div class="Counter" x="(( :non_existing ))" {}></div>`), 0600)

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}
			import /counter.ix
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, result.HyperscriptErrors, 1) {
			return
		}

		hyperscriptError := result.HyperscriptErrors[0]
		assert.Equal(t, parse.NodeSpan{Start: 64, End: 77}, hyperscriptError.Location.Span)
	})
}
