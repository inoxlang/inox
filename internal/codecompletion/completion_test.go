package codecompletion

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/stretchr/testify/assert"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

func TestFindCompletions(t *testing.T) {

	wd, _ := os.Getwd()
	dir := t.TempDir()
	dir, _ = filepath.Abs(dir)

	f, _ := os.Create(filepath.Join(dir, "file1.txt"))
	f.Close()
	f, _ = os.Create(filepath.Join(dir, "file2.txt"))
	f.Close()

	for _, mode := range []Mode{LspCompletions, ShellCompletions} {
		t.Run(mode.String(), func(t *testing.T) {

			// tests
			runSingleModeTests(t, mode, wd, dir)
		})
	}

}

func runSingleModeTests(t *testing.T, mode Mode, wd, dir string) {
	perms := []core.Permission{
		core.CommandPermission{CommandName: core.String("cmd"), SubcommandNameChain: []string{"help", "build"}},
		core.CommandPermission{CommandName: core.String("cmd"), SubcommandNameChain: []string{"help", "run"}},
		core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern(dir)},
		core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern(dir + "/...")},
	}

	newState := func() *core.TreeWalkState {
		return core.NewTreeWalkState(core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  fs_ns.GetOsFilesystem(),
		}))
	}

	doSymbolicCheck := func(chunk *parse.ParsedChunkSource, state *core.GlobalState, additionalSymbolicGlobalConsts ...map[string]symbolic.Value) {
	}

	if mode == LspCompletions {
		doSymbolicCheck = func(
			chunk *parse.ParsedChunkSource,
			state *core.GlobalState,
			additionalSymbolicGlobalConsts ...map[string]symbolic.Value) {

			additional := map[string]symbolic.Value{}
			for _, additionalGlobals := range additionalSymbolicGlobalConsts {
				maps.Copy(additional, additionalGlobals)
			}

			globals := map[string]symbolic.ConcreteGlobalValue{}
			state.Globals.Foreach(func(name string, v core.Value, isConst bool) error {
				globals[name] = symbolic.ConcreteGlobalValue{
					Value:      v,
					IsConstant: isConst,
				}
				return nil
			})

			data, _ := symbolic.EvalCheck(symbolic.EvalCheckInput{
				Node:                           chunk.Node,
				Module:                         symbolic.NewModule(chunk, nil, nil),
				Globals:                        globals,
				AdditionalSymbolicGlobalConsts: additional,
				IsShellChunk:                   false,
				Context:                        utils.Must(state.Ctx.ToSymbolicValue(core.ContextSymbolicConversionParams{})),
			})
			if data != nil {
				state.SymbolicData.AddData(data)
			}
		}
	}

	parseChunkSource := func(s, name string) (*parse.ParsedChunkSource, error) {
		return parse.ParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: s,
		})
	}

	_findCompletions := func(state *core.TreeWalkState, chunk *parse.ParsedChunkSource, cursorIndex int, keepDoc bool, inputData *InputData) []Completion {
		args := SearchArgs{
			State:       state,
			Chunk:       chunk,
			CursorIndex: cursorIndex,
			Mode:        mode,
		}
		if inputData != nil {
			args.InputData = *inputData
		}
		completions := FindCompletions(args)
		//in order to simplify tests we remove/simplify some information like replaced ranges
		for i, codecompletion := range completions {
			completions[i].ReplacedRange = parse.SourcePositionRange{
				SourceName:  "",
				StartLine:   0,
				StartColumn: 0,
				Span:        codecompletion.ReplacedRange.Span,
			}
			completions[i].Kind = 0
			completions[i].LabelDetail = ""
			if !keepDoc {
				completions[i].MarkdownDocumentation = ""
			}
		}
		return completions
	}

	findCompletions := func(state *core.TreeWalkState, chunk *parse.ParsedChunkSource, cursorIndex int) []Completion {
		return _findCompletions(state, chunk, cursorIndex, false, nil)
	}

	t.Run("identifiers and variables", func(t *testing.T) {
		if mode != LspCompletions {
			t.Skip()
			return
		}

		t.Run("local variable in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("val = 1; v", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 10)
			assert.EqualValues(t, []Completion{
				{ShownString: "val", Value: "val", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
			}, completions)
		})

		t.Run("local variable in top level module: different letter casse", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("val = 1; V", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 10)
			assert.EqualValues(t, []Completion{
				{ShownString: "val", Value: "val", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
			}, completions)
		})

		t.Run("local variable within a function", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("fn(val){v}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 9)
			assert.EqualValues(t, []Completion{
				{ShownString: "val", Value: "val", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 8, End: 9}}},
			}, completions)
		})

		t.Run("local variable in a function call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("val = 1; print(v)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 16)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "val",
					Value:         "val",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 15, End: 16}}},
			}, completions)
		})

		t.Run("local variable in a command-liked function call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("val = 1; print v", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 16)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "$val",
					Value:         "$val",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 15, End: 16}}},
			}, completions)
		})

		t.Run("local variable in a method call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("o = {print: fn(arg){}}; val = 1; o.print(v)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 42)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "val",
					Value:         "val",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 41, End: 42}}},
			}, completions)
		})

		t.Run("global variable (identifier) in top level module", func(t *testing.T) {
			if mode != LspCompletions {
				t.Skip()
				return
			}

			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource(`
				manifest {
					permissions: {
						create: {threads: {}}
					}
				}
				
				import test ./test.ix {}
				te
			`, "")

			idents := parse.FindNodes(chunk.Node, (*parse.IdentifierLiteral)(nil), nil)
			ident := idents[len(idents)-1]
			span := ident.Span

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, int(ident.Span.End))
			assert.EqualValues(t, []Completion{
				{ShownString: "test", Value: "test", ReplacedRange: parse.SourcePositionRange{Span: span}},
			}, completions)
		})

		t.Run("global variable ($) in top level module", func(t *testing.T) {
			if mode != LspCompletions {
				t.Skip()
				return
			}

			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource(`
				manifest {
					permissions: {
						create: {threads: {}}
					}
				}
				
				import test ./test.ix {}
				$t
			`, "")

			globalVar := parse.FindNode(chunk.Node, (*parse.Variable)(nil), nil)
			span := globalVar.Span

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, int(globalVar.Span.End))
			assert.EqualValues(t, []Completion{
				{ShownString: "test", Value: "$test", ReplacedRange: parse.SourcePositionRange{Span: span}},
			}, completions)
		})

		t.Run("global variable in a command-liked function call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("globalvar val = 1; print v", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 26)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "$val",
					Value:         "$val",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 25, End: 26}}},
			}, completions)
		})

		t.Run("suggest global function", func(t *testing.T) {
			if mode != LspCompletions {
				t.Skip()
			}

			//TODO: fix (it's working in VSCode)
			t.Skip()

			state := newState()
			state.SetGlobal("sleep", core.WrapGoFunction(core.Sleep), core.GlobalConst)
			chunk, _ := parseChunkSource("sle", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 3, true /*keep documentation*/, nil)

			assert.EqualValues(t, []Completion{
				{
					ShownString:           "sleep",
					Value:                 "sleep",
					ReplacedRange:         parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 3}},
					MarkdownDocumentation: utils.MustGet(help.HelpFor("sleep", helpMessageConfig)),
				},
			}, completions)
		})

		t.Run("property name from prefix in object literal", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("pattern o = {prop: int}; var o o = {p} # error at p (not declared)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 37)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "prop: ",
					Value:         "prop: ",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 36, End: 37}}},
			}, completions)
		})

		t.Run("non-ident-like property name from prefix in object literal", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("pattern o = {\"c fé\": int}; var o o = {c} # error at p (not declared)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 39)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   `"c fé": `,
					Value:         `"c fé": `,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 38, End: 39}}},
			}, completions)
		})

		t.Run("property name from prefix in object literal: linefeed before prefix", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("pattern o = {prop: int}; var o o = {\np} # error at p (not declared)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 38)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "prop: ",
					Value:         "prop: ",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 37, End: 38}}},
			}, completions)
		})

		t.Run("property name + expected value from prefix in object literal", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("pattern o = {prop: 1}; var o o = {p} # error at p (not declared)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 35)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "prop: 1",
					Value:         "prop: 1",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 34, End: 35}}},
			}, completions)
		})

		t.Run("property name + expected value from prefix in record literal", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("pattern o = #{prop: 1}; var o o = #{p} # error at p (not declared)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 37)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "prop: 1",
					Value:         "prop: 1",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 36, End: 37}}},
			}, completions)
		})
	})

	t.Run("identifier member expression", func(t *testing.T) {
		t.Run("suggest object property: object has no property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			state.Global.Globals.Set("obj", core.NewObject())
			chunk, _ := parseChunkSource("obj.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 4)
			assert.Empty(t, completions)
		})

		t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 4)
			assert.EqualValues(t, []Completion{
				{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 3, End: 4}}},
			}, completions)
		})

		t.Run("suggest object property: empty property name: object has single property that is not a ident-like", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"c fé": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 4)
			assert.EqualValues(t, []Completion{
				{ShownString: `.("c fé")`, Value: `.("c fé")`, ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 3, End: 4}}},
			}, completions)
		})

		t.Run("suggest object property: start of property name: object has single property", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.n", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 3, End: 5}}},
			}, completions)
		})

		t.Run("suggest object property: start of property name: object has single property that is not ident-like", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"c fé": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.c", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: `.("c fé")`, Value: `.("c fé")`, ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 3, End: 5}}},
			}, completions)
		})

		t.Run("suggest object property (length 2): empty property name: object has single property", func(t *testing.T) {
			state := newState()
			inner := core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, state.Global.Ctx)
			obj := core.NewObjectFromMap(core.ValMap{"inner": inner}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.inner.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 10)
			assert.EqualValues(t, []Completion{
				{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
			}, completions)
		})

		t.Run("suggest object property (length 2): start of property name: object has single property", func(t *testing.T) {
			state := newState()
			inner := core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, state.Global.Ctx)
			obj := core.NewObjectFromMap(core.ValMap{"inner": inner}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.inner.n", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 11)
			assert.EqualValues(t, []Completion{
				{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 11}}},
			}, completions)
		})

		t.Run("suggest documented namespace method", func(t *testing.T) {
			if mode != LspCompletions {
				t.Skip()
			}

			state := newState()
			state.SetGlobal("dns", net_ns.NewDNSnamespace(), core.GlobalConst)
			chunk, _ := parseChunkSource("dns.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 4, true /*keep documentation*/, nil)

			assert.EqualValues(t, []Completion{
				{
					ShownString:           ".resolve",
					Value:                 ".resolve",
					ReplacedRange:         parse.SourcePositionRange{Span: parse.NodeSpan{Start: 3, End: 4}},
					MarkdownDocumentation: utils.MustGet(help.HelpFor("dns.resolve", helpMessageConfig)),
				},
			}, completions)
		})

	})

	t.Run("object literal interior", func(t *testing.T) {
		if mode != LspCompletions {
			return
		}

		t.Run("suggest properties: single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o {a: int} = {}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 18)

			assert.EqualValues(t, []Completion{
				{
					ShownString:   "a: ",
					Value:         "a: ",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 18}},
				},
			}, completions)
		})

		t.Run("suggest properties: single property with a concretizable value", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o {a: 1} = {}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 16)

			assert.EqualValues(t, []Completion{
				{
					ShownString:   "a: 1",
					Value:         "a: 1",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 16, End: 16}},
				},
			}, completions)
		})

		t.Run("suggest properties: single property with a concretizable value taking a lot of space when stringified", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o {a: \"1111111111111111111111\"} = {}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 39)

			assert.EqualValues(t, []Completion{
				{
					ShownString:   `a: "1111111111111111111111"`,
					Value:         "\na: \"1111111111111111111111\"\n",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 39, End: 39}},
				},
			}, completions)
		})

		t.Run("suggest properties: two properties with a concretizable value", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o {a: 1, b: 2} = {}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 22)

			assert.ElementsMatch(t, []Completion{
				{
					ShownString:   "a: 1",
					Value:         "a: 1",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 22, End: 22}},
				},
				{
					ShownString:   "b: 2",
					Value:         "b: 2",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 22, End: 22}},
				},
				{
					ShownString:   ALL_MISSING_OBJ_PROPS_LABEL,
					Value:         "{\na: 1\nb: 2\n}",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 21, End: 23}},
				},
			}, completions)
		})
	})

	t.Run("record literal interior", func(t *testing.T) {
		if mode != LspCompletions {
			return
		}

		t.Run("suggest properties: single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o #{a: int} = #{}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 20)

			assert.EqualValues(t, []Completion{
				{
					ShownString:   "a: ",
					Value:         "a: ",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 20, End: 20}},
				},
			}, completions)
		})

		t.Run("suggest properties: single property with a concretizable value", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o #{a: 1} = #{}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 18)

			assert.EqualValues(t, []Completion{
				{
					ShownString:   "a: 1",
					Value:         "a: 1",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 18}},
				},
			}, completions)
		})

		t.Run("suggest properties: single property with a concretizable value taking a lot of space when stringified", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o #{a: \"1111111111111111111111\"} = #{}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 41)

			assert.EqualValues(t, []Completion{
				{
					ShownString:   `a: "1111111111111111111111"`,
					Value:         "\na: \"1111111111111111111111\"\n",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 41, End: 41}},
				},
			}, completions)
		})

		t.Run("suggest properties: two properties with a concretizable value", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("var o #{a: 1, b: 2} = #{}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 24)

			assert.ElementsMatch(t, []Completion{
				{
					ShownString:   "a: 1",
					Value:         "a: 1",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 24, End: 24}},
				},
				{
					ShownString:   "b: 2",
					Value:         "b: 2",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 24, End: 24}},
				},
				{
					ShownString:   ALL_MISSING_REC_PROPS_LABEL,
					Value:         "#{\na: 1\nb: 2\n}",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 22, End: 25}},
				},
			}, completions)
		})
	})

	t.Run("member expression", func(t *testing.T) {
		t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("$obj.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("suggest object property: empty property name: object has single property that is not ident-like", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"c fé": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("$obj.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: `.("c fé")`, Value: `.("c fé")`, ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("suggest object property: start of property name: object has single property", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("$obj.n", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 6)
			assert.EqualValues(t, []Completion{
				{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 6}}},
			}, completions)
		})

		t.Run("suggest object property: start of property name: object has single property that is not ident-like", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{"c fé": core.String("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("$obj.c", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 6)
			assert.EqualValues(t, []Completion{
				{ShownString: `.("c fé")`, Value: `.("c fé")`, ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 6}}},
			}, completions)
		})

		t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := newState()
			obj := core.NewObjectFromMap(core.ValMap{
				"object": core.NewObjectFromMap(core.ValMap{"name": core.String("foo")}, state.Global.Ctx),
			}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)

			chunk, _ := parseChunkSource("$obj.object.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 12)
			assert.EqualValues(t, []Completion{
				{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 11, End: 12}}},
			}, completions)
		})

		t.Run("suggest property of shared object's property", func(t *testing.T) {
			if mode == ShellCompletions {
				//TODO: support
				t.Skip()
				return
			}

			state := newState()
			sharedObject := core.NewObjectFromMap(core.ValMap{"list": core.NewWrappedValueList()}, state.Global.Ctx)
			sharedObject.Share(state.Global)
			state.SetGlobal("obj", sharedObject, core.GlobalConst)

			chunk, _ := parseChunkSource("obj::list.", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 10)
			assert.Contains(t, completions, Completion{
				ShownString:   ".append",
				Value:         ".append",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}},
			}, completions)
		})
	})

	t.Run("double-colon expression with shared object on the left", func(t *testing.T) {

		if mode == ShellCompletions {
			t.Skip()
		}

		t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := newState()
			sharedObject := core.NewObjectFromMap(core.ValMap{"list": core.NewWrappedValueList()}, state.Global.Ctx)
			sharedObject.Share(state.Global)
			state.SetGlobal("obj", sharedObject, core.GlobalConst)
			chunk, _ := parseChunkSource("obj::", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: "list", Value: "list", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 5, End: 5}}},
			}, completions)
		})

		t.Run("suggest object property: start of property name: object has single property", func(t *testing.T) {
			state := newState()
			sharedObject := core.NewObjectFromMap(core.ValMap{"list": core.NewWrappedValueList()}, state.Global.Ctx)
			sharedObject.Share(state.Global)
			state.SetGlobal("obj", sharedObject, core.GlobalConst)
			chunk, _ := parseChunkSource("obj::l", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 6)
			assert.EqualValues(t, []Completion{
				{ShownString: "list", Value: "list", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 5, End: 6}}},
			}, completions)
		})
	})

	t.Run("double-colon expression with pattern-matching object on the left", func(t *testing.T) {
		if mode == ShellCompletions {
			t.Skip()
		}

		t.Run("empty property name", func(t *testing.T) {
			state := newState()
			state.Global.Ctx.AddNamedPattern("int", core.INT_PATTERN)
			chunk, _ := parseChunkSource("pattern o = {a: []int, b: 2}; extend o {c: 3}; var obj = {a: [1], b: 2}; obj::", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 78)
			assert.EqualValues(t, []Completion{
				{ShownString: "a", Value: "a", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 78, End: 78}}},
				{ShownString: "c", Value: "c", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 78, End: 78}}},
			}, completions)
		})
	})

	t.Run("double-colon expression with url on the left", func(t *testing.T) {
		if mode == ShellCompletions {
			t.Skip()
		}

		userPattern := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{
			"name": symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil),
			"data": symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil),
		}, nil)

		db := symbolic.NewDatabaseIL(symbolic.DatabaseILParams{
			Schema: symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{
				"user": userPattern,
			}, nil),
		})

		t.Run("empty property name", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("url = ldb://main/user; url::", "")

			doSymbolicCheck(chunk, state.Global, map[string]symbolic.Value{
				globalnames.DATABASES: symbolic.NewNamespace(map[string]symbolic.Value{"main": db}),
			})

			completions := findCompletions(state, chunk, 28)
			assert.EqualValues(t, []Completion{
				{ShownString: "data", Value: "data", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 28, End: 28}}},
				{ShownString: "name", Value: "name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 28, End: 28}}},
			}, completions)
		})

		t.Run("suggest property name from first letter", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("url = ldb://main/user; url::n", "")

			doSymbolicCheck(chunk, state.Global, map[string]symbolic.Value{
				globalnames.DATABASES: symbolic.NewNamespace(map[string]symbolic.Value{"main": db}),
			})

			completions := findCompletions(state, chunk, 29)
			assert.EqualValues(t, []Completion{
				{ShownString: "name", Value: "name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 28, End: 29}}},
			}, completions)
		})
	})

	t.Run("named patterns", func(t *testing.T) {
		if mode == ShellCompletions {
			t.Run("suggest pre-declared pattern from first letter", func(t *testing.T) {
				state := newState()
				state.Global.Ctx.AddNamedPattern("int", core.INT_PATTERN)
				chunk, _ := parseChunkSource("%i", "")

				completions := findCompletions(state, chunk, 2)
				assert.EqualValues(t, []Completion{
					{
						ShownString:   "%int",
						Value:         "%int",
						ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 2}},
					},
				}, completions)
			})

			t.Run("suggest pre-declared pattern namespace from first letter", func(t *testing.T) {
				state := newState()
				state.Global.Ctx.AddPatternNamespace("inox", core.DEFAULT_PATTERN_NAMESPACES["inox"])
				chunk, _ := parseChunkSource("%i", "")

				completions := findCompletions(state, chunk, 2)
				assert.EqualValues(t, []Completion{
					{
						ShownString:   "%inox",
						Value:         "%inox",
						ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 2}},
					},
				}, completions)
			})
			return
		}

		t.Run("suggest pattern from first letter", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("pattern patt = 1; %p", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 20)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "%patt",
					Value:         "%patt",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 20}},
				},
			}, completions)
		})

		t.Run("suggest pattern namespace from first letter", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("pnamespace namespace. = 1; %n", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 29)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "%namespace.",
					Value:         "%namespace.",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 27, End: 29}},
				},
			}, completions)
		})

		t.Run("suggest pattern namespace member from first letter", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("pnamespace namespace. = {patt: 1}; %namespace.p", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 37)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "%namespace.patt",
					Value:         "%namespace.patt",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 35, End: 47}},
				},
			}, completions)
		})
	})

	t.Run("manifest section", func(t *testing.T) {

		if mode == ShellCompletions {
			t.Skip()
		}

		t.Run("from prefix", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("manifest{e}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 10)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "env: %{}",
					Value:         "env: %{}",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}},
				},
			}, completions)
		})

		t.Run("in empty manifest", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("manifest{}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 9)
			assert.Contains(t, completions, Completion{
				ShownString:   "env: %{}",
				Value:         "env: %{}",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 9}},
			})
		})

		t.Run("in non-empty manifest", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("manifest{\nparameters:{}}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 9)
			assert.Contains(t, completions, Completion{
				ShownString:   "env: %{}",
				Value:         "env: %{}",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 9}},
			})

			for _, completion := range completions {
				if completion.ShownString == "parameters" {
					assert.Fail(t, "completion for 'parameters' should not be present")
				}
			}
		})

	})

	t.Run("database description", func(t *testing.T) {
		if mode == ShellCompletions {
			t.Skip()
		}

		t.Run("from prefix", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("manifest{databases:{main:{a}}}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 27)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "assert-schema: " + MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS["assert-schema"],
					Value:         "assert-schema: " + MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS["assert-schema"],
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 26, End: 27}},
				},
			}, completions)
		})

		t.Run("in empty description", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("manifest{databases:{main:{}}}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 26)
			assert.Contains(t, completions, Completion{
				ShownString:   "assert-schema: " + MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS["assert-schema"],
				Value:         "assert-schema: " + MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS["assert-schema"],
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 26, End: 26}},
			})
		})

		t.Run("in non-empty description", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("manifest{databases:{main:{\nresource:ldb://main}}}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 26)
			assert.Contains(t, completions, Completion{
				ShownString:   "assert-schema: " + MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS["assert-schema"],
				Value:         "assert-schema: " + MANIFEST_DB_DESC_DEFAULT_VALUE_COMPLETIONS["assert-schema"],
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 26, End: 26}},
			})

			for _, completion := range completions {
				if completion.ShownString == "resource" {
					assert.Fail(t, "completion for 'resource' should not be present")
				}
			}
		})

	})

	t.Run("module import config section", func(t *testing.T) {
		if mode == ShellCompletions {
			t.Skip()
		}

		t.Run("from prefix", func(t *testing.T) {
			state := newState()
			chunk := utils.Must(parseChunkSource("import lib /a.ix {a}", ""))
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 19)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
					Value:         inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 19}},
				},
				{
					ShownString:   inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME + ": {}",
					Value:         inoxconsts.IMPORT_CONFIG__ARGUMENTS_PROPNAME + ": {}",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 19}},
				},
			}, completions)
		})

		t.Run("in empty module import config", func(t *testing.T) {
			state := newState()
			chunk := utils.Must(parseChunkSource("import lib /a.ix {}", ""))
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 18)
			assert.Contains(t, completions, Completion{
				ShownString:   inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
				Value:         inoxconsts.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 18}},
			})
		})
	})

	t.Run("lthread meta section", func(t *testing.T) {
		t.Run("from prefix", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("go {a} do {}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   symbolic.LTHREAD_META_ALLOW_SECTION + ": {}",
					Value:         symbolic.LTHREAD_META_ALLOW_SECTION + ": {}",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}},
				},
			}, completions)
		})

		t.Run("in empty lthread meta", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("go {} do {}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 4)
			assert.Contains(t, completions, Completion{
				ShownString:   symbolic.LTHREAD_META_ALLOW_SECTION + ": {}",
				Value:         symbolic.LTHREAD_META_ALLOW_SECTION + ": {}",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 4}},
			})
		})

		t.Run("in non-empty manifest", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("go {\nglobals: .{}} do {}", "")
			doSymbolicCheck(chunk, state.Global)

			completions := findCompletions(state, chunk, 4)
			assert.Contains(t, completions, Completion{
				ShownString:   symbolic.LTHREAD_META_ALLOW_SECTION + ": {}",
				Value:         symbolic.LTHREAD_META_ALLOW_SECTION + ": {}",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 4}},
			})

			for _, completion := range completions {
				if completion.ShownString == "parameters" {
					assert.Fail(t, "completion for 'parameters' should not be present")
				}
			}
		})

	})

	t.Run("permission kind in manifest", func(t *testing.T) {
		state := newState()
		chunk, _ := parseChunkSource("manifest{permissions:{}}", "")
		doSymbolicCheck(chunk, state.Global)

		completions := findCompletions(state, chunk, 22)
		assert.Contains(t, completions, Completion{
			ShownString:   "read",
			Value:         "read",
			ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 22, End: 22}},
		})
	})

	t.Run("permission kind from prefix in manifest", func(t *testing.T) {
		state := newState()
		chunk, _ := parseChunkSource("manifest{permissions:{r}}", "")
		doSymbolicCheck(chunk, state.Global)

		completions := findCompletions(state, chunk, 23)
		assert.EqualValues(t, []Completion{
			{
				ShownString:   "read",
				Value:         "read",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 22, End: 23}},
			},
		}, completions)
	})

	t.Run("permission kind in module import", func(t *testing.T) {
		state := newState()
		chunk, _ := parseChunkSource("manifest{};import lib /lib.ix {allow:{}}", "")
		doSymbolicCheck(chunk, state.Global)

		completions := findCompletions(state, chunk, 38)
		assert.Contains(t, completions, Completion{
			ShownString:   "read",
			Value:         "read",
			ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 38, End: 38}},
		})
	})

	t.Run("permission kind from prefix in manifest", func(t *testing.T) {
		state := newState()
		chunk, _ := parseChunkSource("manifest{};import lib /lib.ix {allow:{r}}", "")
		doSymbolicCheck(chunk, state.Global)

		completions := findCompletions(state, chunk, 39)
		assert.EqualValues(t, []Completion{
			{
				ShownString:   "read",
				Value:         "read",
				ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 38, End: 39}},
			},
		}, completions)
	})

	t.Run("subcommand", func(t *testing.T) {
		t.Run("depth 0", func(t *testing.T) {
			//TODO: implement
			t.Skip()
			state := newState()
			chunk := utils.Must(parseChunkSource("cmd ", ""))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 4)
			assert.EqualValues(t, []Completion{
				{ShownString: "help", Value: "help", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("depth 1", func(t *testing.T) {
			state := newState()
			chunk := utils.Must(parseChunkSource("cmd help ", ""))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 9)
			assert.EqualValues(t, []Completion{
				{ShownString: "build", Value: "build", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
				{ShownString: "run", Value: "run", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
			}, completions)
		})

		t.Run("depth 0, subcommand of depth 1 is present ", func(t *testing.T) {
			state := newState()
			chunk := utils.Must(parseChunkSource("cmd  build", ""))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 4)
			assert.EqualValues(t, []Completion{
				{ShownString: "help", Value: "help", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("suggest subcommand from subcommand prefix : depth 0", func(t *testing.T) {
			state := newState()
			chunk := utils.Must(parseChunkSource("cmd h", ""))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: "help", Value: "help", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("suggest subcommand from subcommand prefix : depth 1", func(t *testing.T) {
			state := newState()
			chunk := utils.Must(parseChunkSource("cmd help b", ""))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 10)
			assert.EqualValues(t, []Completion{
				{ShownString: "build", Value: "build", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
			}, completions)
		})

	})

	t.Run("absolute path", func(t *testing.T) {
		state := newState()

		code := dir + "/f"
		chunk, _ := parseChunkSource(code, "")

		doSymbolicCheck(chunk, state.Global)
		completions := findCompletions(state, chunk, len(code))
		assert.EqualValues(t, []Completion{
			{
				ShownString: "file1.txt",
				Value:       dir + "/file1.txt",
				ReplacedRange: parse.SourcePositionRange{
					Span: parse.NodeSpan{Start: 0, End: int32(len(code))},
				},
			},
			{
				ShownString: "file2.txt",
				Value:       dir + "/file2.txt",
				ReplacedRange: parse.SourcePositionRange{
					Span: parse.NodeSpan{Start: 0, End: int32(len(code))},
				},
			},
		}, completions)
	})

	t.Run("relative path", func(t *testing.T) {
		state := newState()

		reldir, _ := filepath.Rel(wd, dir)
		code := reldir + "/f"
		chunk, _ := parseChunkSource(code, "")

		doSymbolicCheck(chunk, state.Global)
		completions := findCompletions(state, chunk, len(code))
		assert.EqualValues(t, []Completion{
			{
				ShownString: "file1.txt",
				Value:       reldir + "/file1.txt",
				ReplacedRange: parse.SourcePositionRange{
					Span: parse.NodeSpan{Start: 0, End: int32(len(code))},
				},
			},
			{
				ShownString: "file2.txt",
				Value:       reldir + "/file2.txt",
				ReplacedRange: parse.SourcePositionRange{
					Span: parse.NodeSpan{Start: 0, End: int32(len(code))},
				},
			},
		}, completions)
	})

	t.Run("host suggestions", func(t *testing.T) {
		t.Run("scheme literal", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("ldb://", "")

			state.Global.Ctx.AddHostDefinition("ldb://main", core.Host("ldb://main"))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 6)
			assert.EqualValues(t, []Completion{
				{
					ShownString: "ldb://main",
					Value:       "ldb://main",
					ReplacedRange: parse.SourcePositionRange{
						Span: parse.NodeSpan{Start: 0, End: 6},
					},
				},
			}, completions)
		})

		t.Run("host literal", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("ldb://m", "")

			state.Global.Ctx.AddHostDefinition("ldb://main", core.Host("ldb://main"))
			state.Global.Ctx.AddHostDefinition("ldb://secondary", core.Host("ldb://secondary"))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 7)
			assert.EqualValues(t, []Completion{
				{
					ShownString: "ldb://main",
					Value:       "ldb://main",
					ReplacedRange: parse.SourcePositionRange{
						Span: parse.NodeSpan{Start: 0, End: 7},
					},
				},
			}, completions)
		})

		t.Run("host pattern literal", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("%ldb://m", "")

			state.Global.Ctx.AddHostDefinition("ldb://main", core.Host("ldb://main"))
			state.Global.Ctx.AddHostDefinition("ldb://secondary", core.Host("ldb://secondary"))

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 8)
			assert.EqualValues(t, []Completion{
				{
					ShownString: "%ldb://main",
					Value:       "%ldb://main",
					ReplacedRange: parse.SourcePositionRange{
						Span: parse.NodeSpan{Start: 0, End: 8},
					},
				},
			}, completions)
		})
	})

	t.Run("break", func(t *testing.T) {

		t.Run("in for statement's block", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("for []{b}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 8)
			assert.EqualValues(t, []Completion{
				{ShownString: "break", Value: "break", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 7, End: 8}}},
			}, completions)
		})

		t.Run("in if's block within a for statement", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("for []{if true {b}}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 17)
			assert.EqualValues(t, []Completion{
				{ShownString: "break", Value: "break", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 16, End: 17}}},
			}, completions)
		})

	})

	t.Run("suggest continue", func(t *testing.T) {

		t.Run("in for statement's block", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("for []{cont}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 11)
			assert.EqualValues(t, []Completion{
				{ShownString: "continue", Value: "continue", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 7, End: 11}}},
			}, completions)
		})

		t.Run("in if's block within a for statement", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("for []{if true {cont}}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 20)
			assert.EqualValues(t, []Completion{
				{ShownString: "continue", Value: "continue", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 16, End: 20}}},
			}, completions)
		})

	})

	t.Run("prune", func(t *testing.T) {

		t.Run("in walk statement's block", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("walk ./ e {p}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 12)
			assert.EqualValues(t, []Completion{
				{ShownString: "prune", Value: "prune", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 11, End: 12}}},
			}, completions)
		})

		t.Run("in for statement's block within a walk statement", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("walk ./ e {for []{p}}", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 19)
			assert.EqualValues(t, []Completion{
				{ShownString: "prune", Value: "prune", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 19}}},
			}, completions)
		})

	})

	t.Run("context independent keywords that start statements", func(t *testing.T) {

		for _, keyword := range CONTEXT_INDEPENDENT_STMT_STARTING_KEYWORDS {
			t.Run(keyword, func(t *testing.T) {
				t.Run("in top module", func(t *testing.T) {
					state := newState()
					chunk, _ := parseChunkSource(string(keyword[0]), "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 1)
					//we remove other keyword completions
					completions = utils.FilterSlice(completions, func(s Completion) bool { return s.Value == keyword })

					assert.EqualValues(t, []Completion{
						{ShownString: keyword, Value: keyword, ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
					}, completions)
				})
			})
		}
	})

	t.Run("treedata", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("t", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 1)
			assert.EqualValues(t, []Completion{
				{ShownString: "treedata", Value: "treedata", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
			}, completions)
		})

		t.Run("in call", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("f(t)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 3)
			assert.EqualValues(t, []Completion{
				{ShownString: "treedata", Value: "treedata", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 2, End: 3}}},
			}, completions)
		})

	})

	t.Run("Mapping", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("M", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 1)
			assert.EqualValues(t, []Completion{
				{ShownString: "match", Value: "match", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
				{ShownString: "Mapping", Value: "Mapping", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
			}, completions)
		})

		t.Run("in call", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("f(M)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 3)
			assert.EqualValues(t, []Completion{
				{ShownString: "Mapping", Value: "Mapping", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 2, End: 3}}},
			}, completions)
		})

	})

	t.Run("concat", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("c", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 1)
			assert.EqualValues(t, []Completion{
				{ShownString: "concat", Value: "concat", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
			}, completions)
		})

		t.Run("in call", func(t *testing.T) {
			state := newState()
			chunk, _ := parseChunkSource("f(c)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 3)
			assert.EqualValues(t, []Completion{
				{ShownString: "concat", Value: "concat", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 2, End: 3}}},
			}, completions)
		})

	})

	t.Run("html attribute names", func(t *testing.T) {
		t.Run("local variable in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("html<img sr />", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 11)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "src",
					Value:         "src",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 11}},
				},
				{
					ShownString:   "srcset",
					Value:         "srcset",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 11}},
				},
			}, completions)
		})
	})

	t.Run("html attribute names in markup expression with implicit namespace", func(t *testing.T) {
		t.Run("local variable in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("(<img sr />)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := findCompletions(state, chunk, 8)
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "src",
					Value:         "src",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 6, End: 8}},
				},
				{
					ShownString:   "srcset",
					Value:         "srcset",
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 6, End: 8}},
				},
			}, completions)
		})
	})

	t.Run("html attribute values", func(t *testing.T) {
		if mode != LspCompletions {
			return
		}

		t.Run("src in <script>: empty", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("html<script src=\"\">", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 17, false, &InputData{
				StaticFileURLPaths: []string{"/index.css", "/index.js"},
			})
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "/index.js",
					Value:         `"/index.js"`,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 16, End: 18}},
				},
			}, completions)
		})

		t.Run("src in <script> with implicit namespace: empty", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("(<script src=\"\">)", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 13, false, &InputData{
				StaticFileURLPaths: []string{"/index.css", "/index.js"},
			})
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "/index.js",
					Value:         `"/index.js"`,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 13, End: 15}},
				},
			}, completions)
		})

		t.Run("src in <script>: with prefix", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("html<script src=\"/i\">", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 19, false, &InputData{
				StaticFileURLPaths: []string{"/index.css", "/index.js", "/main.js"},
			})
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "/index.js",
					Value:         `"/index.js"`,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 16, End: 20}},
				},
			}, completions)
		})

		t.Run("href in <link>: empty", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("html<link href=\"\">", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 16, false, &InputData{
				StaticFileURLPaths: []string{"/index.css", "/index.js"},
			})
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "/index.css",
					Value:         `"/index.css"`,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 15, End: 17}},
				},
			}, completions)
		})

		t.Run("href in <link>: with prefix", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("html<link href=\"/i\">", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 18, false, &InputData{
				StaticFileURLPaths: []string{"/index.css", "/index.js", "/main.js"},
			})
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "/index.css",
					Value:         `"/index.css"`,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 15, End: 19}},
				},
			}, completions)
		})

		t.Run("src in <img>: empty", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("html<img src=\"\">", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 14, false, &InputData{
				StaticFileURLPaths: []string{"/index.css", "/index.js", "/index.png"},
			})
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "/index.png",
					Value:         `"/index.png"`,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 13, End: 15}},
				},
			}, completions)
		})

		t.Run("src in <img>: with prefix", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			defer state.Global.Ctx.CancelGracefully()

			chunk, _ := parseChunkSource("html<img src=\"/i\">", "")

			doSymbolicCheck(chunk, state.Global)
			completions := _findCompletions(state, chunk, 16, false, &InputData{
				StaticFileURLPaths: []string{"/index.js", "/index.png", "/main.png"},
			})
			assert.EqualValues(t, []Completion{
				{
					ShownString:   "/index.png",
					Value:         `"/index.png"`,
					ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 13, End: 17}},
				},
			}, completions)
		})

		t.Run("hx-post-json in <form>: empty", func(t *testing.T) {
			t.Skip()

			//create API manually

			// state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			// defer state.Global.Ctx.CancelGracefully()

			// chunk, _ := parseChunkSource("html<form hx-post-json=\"\"></form>", "")

			// doSymbolicCheck(chunk, state.Global)
			// completions := _findCompletions(state, chunk, 16, false, &InputData{ServerAPI: api})

			// assert.EqualValues(t, []Completion{
			// 	{
			// 		ShownString:   "/users",
			// 		Value:         `"/users"`,
			// 		ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 13, End: 17}},
			// 	},
			// }, completions)
		})
	})

}

func makeFilesystem() *fs_ns.MemFilesystem {
	fls := fs_ns.NewMemFilesystem(1_000_000)

	fls.MkdirAll("/routes/", 0700)
	util.WriteFile(fls, "/routes/POST-users.ix", []byte("manifest {parameters: {}}; return html<div></div>"), 0600)

	return fls
}
