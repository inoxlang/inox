package compl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/permkind"

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

	perms := []core.Permission{
		core.CommandPermission{CommandName: core.Str("cmd"), SubcommandNameChain: []string{"help", "build"}},
		core.CommandPermission{CommandName: core.Str("cmd"), SubcommandNameChain: []string{"help", "run"}},
		core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(dir)},
		core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(dir + "/...")},
	}

	parseChunkSource := func(s, name string) (*parse.ParsedChunk, error) {
		return parse.ParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: s,
		})
	}

	newState := func() *core.TreeWalkState {
		return core.NewTreeWalkState(core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  fs_ns.GetOsFilesystem(),
		}))
	}

	for _, mode := range []CompletionMode{LspCompletions, ShellCompletions} {
		t.Run(mode.String(), func(t *testing.T) {

			_findCompletions := func(state *core.TreeWalkState, chunk *parse.ParsedChunk, cursorIndex int, keepDoc bool) []Completion {
				completions := FindCompletions(CompletionSearchArgs{
					State:       state,
					Chunk:       chunk,
					CursorIndex: cursorIndex,
					Mode:        mode,
				})
				//in order to simplify tests we remove/simplify some information like replaced ranges
				for i, compl := range completions {
					completions[i].ReplacedRange = parse.SourcePositionRange{
						SourceName:  "",
						StartLine:   0,
						StartColumn: 0,
						Span:        compl.ReplacedRange.Span,
					}
					completions[i].Kind = 0
					completions[i].LabelDetail = ""
					if !keepDoc {
						completions[i].MarkdownDocumentation = ""
					}
				}
				return completions
			}

			findCompletions := func(state *core.TreeWalkState, chunk *parse.ParsedChunk, cursorIndex int) []Completion {
				return _findCompletions(state, chunk, cursorIndex, false)
			}

			doSymbolicCheck := func(chunk *parse.ParsedChunk, state *core.GlobalState) {}
			if mode == LspCompletions {
				doSymbolicCheck = func(chunk *parse.ParsedChunk, state *core.GlobalState) {

					globals := map[string]symbolic.ConcreteGlobalValue{}
					state.Globals.Foreach(func(name string, v core.Value, isConst bool) error {
						globals[name] = symbolic.ConcreteGlobalValue{
							Value:      v,
							IsConstant: isConst,
						}
						return nil
					})

					data, _ := symbolic.EvalCheck(symbolic.EvalCheckInput{
						Node:         chunk.Node,
						Module:       symbolic.NewModule(chunk, nil, nil),
						Globals:      globals,
						IsShellChunk: false,
						Context:      utils.Must(state.Ctx.ToSymbolicValue()),
					})
					if data != nil {
						state.SymbolicData.AddData(data)
					}
				}
			}

			// tests

			t.Run("variables", func(t *testing.T) {
				if mode != LspCompletions {
					t.Skip()
					return
				}

				t.Run("local variable in top level module", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource("val = 1; v", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 10)
					assert.EqualValues(t, []Completion{
						{ShownString: "val", Value: "val", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
					}, completions)
				})

				t.Run("local variable in top level module: different letter casse", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource("val = 1; V", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 10)
					assert.EqualValues(t, []Completion{
						{ShownString: "val", Value: "val", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
					}, completions)
				})

				t.Run("local variable within a function", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource("fn(val){v}", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 9)
					assert.EqualValues(t, []Completion{
						{ShownString: "val", Value: "val", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 8, End: 9}}},
					}, completions)
				})

				t.Run("local variable in a function call", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
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

				t.Run("global variable ($$) in top level module", func(t *testing.T) {
					if mode != LspCompletions {
						t.Skip()
						return
					}

					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource(`
						manifest {
							permissions: {
								create: {threads: {}}
							}
						}
						
						import test ./test.ix {}
						t
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

				t.Run("global variable ($$) in top level module", func(t *testing.T) {
					if mode != LspCompletions {
						t.Skip()
						return
					}

					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource(`
						manifest {
							permissions: {
								create: {threads: {}}
							}
						}
						
						import test ./test.ix {}
						$$t
					`, "")

					globalVarIdent := parse.FindNode(chunk.Node, (*parse.GlobalVariable)(nil), nil)
					span := globalVarIdent.Span

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, int(globalVarIdent.Span.End))
					assert.EqualValues(t, []Completion{
						{ShownString: "test", Value: "$$test", ReplacedRange: parse.SourcePositionRange{Span: span}},
					}, completions)
				})

				t.Run("global variable in a command-liked function call", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource("$$val = 1; print v", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 18)
					assert.EqualValues(t, []Completion{
						{
							ShownString:   "$$val",
							Value:         "$$val",
							ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 17, End: 18}}},
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
					completions := _findCompletions(state, chunk, 3, true /*keep documentation*/)

					assert.EqualValues(t, []Completion{
						{
							ShownString:           "sleep",
							Value:                 "sleep",
							ReplacedRange:         parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 3}},
							MarkdownDocumentation: utils.MustGet(help.HelpFor("sleep", helpMessageConfig)),
						},
					}, completions)
				})
			})

			t.Run("identifier member expression", func(t *testing.T) {
				t.Run("suggest object property: object has no property", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					state.Global.Globals.Set("obj", core.NewObject())
					chunk, _ := parseChunkSource("obj.", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 4)
					assert.Empty(t, completions)
				})

				t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)
					chunk, _ := parseChunkSource("obj.", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 4)
					assert.EqualValues(t, []Completion{
						{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 3, End: 4}}},
					}, completions)
				})

				t.Run("suggest object property: start of property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)
					chunk, _ := parseChunkSource("obj.n", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 5)
					assert.EqualValues(t, []Completion{
						{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 3, End: 5}}},
					}, completions)
				})

				t.Run("suggest object property (length 2): empty property name: object has single property", func(t *testing.T) {
					state := newState()
					inner := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
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
					inner := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
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
					completions := _findCompletions(state, chunk, 4, true /*keep documentation*/)

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

			t.Run("member expression", func(t *testing.T) {
				t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)
					chunk, _ := parseChunkSource("$$obj.", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 6)
					assert.EqualValues(t, []Completion{
						{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 5, End: 6}}},
					}, completions)
				})

				t.Run("suggest object property: start of property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)
					chunk, _ := parseChunkSource("$$obj.n", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 7)
					assert.EqualValues(t, []Completion{
						{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 5, End: 7}}},
					}, completions)
				})

				t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{
						"object": core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx),
					}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)

					chunk, _ := parseChunkSource("$$obj.object.", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 13)
					assert.EqualValues(t, []Completion{
						{ShownString: ".name", Value: ".name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 12, End: 13}}},
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
					assert.EqualValues(t, []Completion{
						{ShownString: ".append", Value: ".append", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
					}, completions)
				})
			})

			t.Run("double-colon expression with shared object LHS", func(t *testing.T) {

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

			t.Run("double-colon expression with patternm-matching object LHS", func(t *testing.T) {
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
							assert.Fail(t, "completion for 'parameters' should be present")
						}
					}
				})

			})

			t.Run("module import config section", func(t *testing.T) {
				t.Run("from prefix", func(t *testing.T) {
					state := newState()
					chunk := utils.Must(parseChunkSource("import lib /a.ix {a}", ""))
					doSymbolicCheck(chunk, state.Global)

					completions := findCompletions(state, chunk, 19)
					assert.EqualValues(t, []Completion{
						{
							ShownString:   core.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
							Value:         core.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
							ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 18, End: 19}},
						},
						{
							ShownString:   core.IMPORT_CONFIG__ARGUMENTS_PROPNAME + ": {}",
							Value:         core.IMPORT_CONFIG__ARGUMENTS_PROPNAME + ": {}",
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
						ShownString:   core.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
						Value:         core.IMPORT_CONFIG__ALLOW_PROPNAME + ": {}",
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
							assert.Fail(t, "completion for 'parameters' should be present")
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
					chunk, _ := parseChunkSource("u", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 1)
					assert.EqualValues(t, []Completion{
						{ShownString: "treedata", Value: "treedata", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
					}, completions)
				})

				t.Run("in call", func(t *testing.T) {
					state := newState()
					chunk, _ := parseChunkSource("f(u)", "")

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
		})
	}

}
