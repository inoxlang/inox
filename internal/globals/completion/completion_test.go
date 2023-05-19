package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	_fs "github.com/inoxlang/inox/internal/globals/fs"

	core "github.com/inoxlang/inox/internal/core"
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
		core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern(dir)},
		core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern(dir + "/...")},
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
			Filesystem:  _fs.GetOsFilesystem(),
		}))
	}

	for _, mode := range []CompletionMode{LspCompletions, ShellCompletions} {
		t.Run(mode.String(), func(t *testing.T) {

			findCompletions := func(state *core.TreeWalkState, chunk *parse.ParsedChunk, cursorIndex int) []Completion {
				completions := FindCompletions(CompletionSearchArgs{
					State:       state,
					Chunk:       chunk,
					CursorIndex: cursorIndex,
					Mode:        mode,
				})
				//in order to simplify tests we remove some information like replaced ranges
				for i, compl := range completions {
					completions[i].ReplacedRange = parse.SourcePositionRange{
						SourceName:  "",
						StartLine:   0,
						StartColumn: 0,
						Span:        compl.ReplacedRange.Span,
					}
					completions[i].Kind = 0
					completions[i].Detail = ""
				}
				return completions
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

					data, _ := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
						Node: chunk.Node,
						Module: &symbolic.Module{
							MainChunk: chunk,
						},
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

				t.Run("local variable within a function", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource("fn(val){v}", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 9)
					assert.EqualValues(t, []Completion{
						{ShownString: "val", Value: "val", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 8, End: 9}}},
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
								create: {routines: {}}
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
								create: {routines: {}}
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
						{ShownString: "obj.name", Value: "obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 4}}},
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
						{ShownString: "obj.name", Value: "obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 5}}},
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
						{ShownString: "obj.inner.name", Value: "obj.inner.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 10}}},
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
						{ShownString: "obj.inner.name", Value: "obj.inner.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 11}}},
					}, completions)
				})
			})

			t.Run("member expression", func(t *testing.T) {
				t.Run("member expression: suggest object property: empty property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)
					chunk, _ := parseChunkSource("$$obj.", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 6)
					assert.EqualValues(t, []Completion{
						{ShownString: "$$obj.name", Value: "$$obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 6}}},
					}, completions)
				})

				t.Run("member expression: suggest object property: start of property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)
					chunk, _ := parseChunkSource("$$obj.n", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 7)
					assert.EqualValues(t, []Completion{
						{ShownString: "$$obj.name", Value: "$$obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 7}}},
					}, completions)
				})

				t.Run("member expression: suggest object property: empty property name: object has single property", func(t *testing.T) {
					state := newState()
					obj := core.NewObjectFromMap(core.ValMap{
						"object": core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx),
					}, state.Global.Ctx)
					state.SetGlobal("obj", obj, core.GlobalConst)

					chunk, _ := parseChunkSource("$$obj.object.", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 13)
					assert.EqualValues(t, []Completion{
						{ShownString: "$$obj.object.name", Value: "$$obj.object.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 13}}},
					}, completions)
				})

			})

			t.Run("named patterns", func(t *testing.T) {
				if mode != LspCompletions {
					t.Skip()
					return
				}

				t.Run("suggest pattern from first letter", func(t *testing.T) {
					state := newState()
					chunk, _ := parseChunkSource("%patt = 1; %p", "")
					doSymbolicCheck(chunk, state.Global)

					completions := findCompletions(state, chunk, 13)
					assert.EqualValues(t, []Completion{
						{
							ShownString:   "%patt",
							Value:         "%patt",
							ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 11, End: 13}},
						},
					}, completions)
				})

				t.Run("suggest pattern namespace from first letter", func(t *testing.T) {
					state := newState()
					chunk, _ := parseChunkSource("%namespace. = 1; %n", "")
					doSymbolicCheck(chunk, state.Global)

					completions := findCompletions(state, chunk, 19)
					assert.EqualValues(t, []Completion{
						{
							ShownString:   "%namespace.",
							Value:         "%namespace.",
							ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 17, End: 19}},
						},
					}, completions)
				})

				t.Run("suggest pattern namespace member from first letter", func(t *testing.T) {
					state := newState()
					chunk, _ := parseChunkSource("%namespace. = {patt: 1}; %namespace.p", "")
					doSymbolicCheck(chunk, state.Global)

					completions := findCompletions(state, chunk, 37)
					assert.EqualValues(t, []Completion{
						{
							ShownString:   "%namespace.patt",
							Value:         "%namespace.patt",
							ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 25, End: 37}},
						},
					}, completions)
				})
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

			t.Run("udata", func(t *testing.T) {

				t.Run("in top level module", func(t *testing.T) {
					state := newState()
					chunk, _ := parseChunkSource("u", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 1)
					assert.EqualValues(t, []Completion{
						{ShownString: "udata", Value: "udata", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
					}, completions)
				})

				t.Run("in call", func(t *testing.T) {
					state := newState()
					chunk, _ := parseChunkSource("f(u)", "")

					doSymbolicCheck(chunk, state.Global)
					completions := findCompletions(state, chunk, 3)
					assert.EqualValues(t, []Completion{
						{ShownString: "udata", Value: "udata", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 2, End: 3}}},
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
		})
	}

}
