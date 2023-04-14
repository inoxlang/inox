package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	core "github.com/inox-project/inox/internal/core"
	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"
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

	findCompletions := func(state *core.TreeWalkState, chunk *parse.ParsedChunk, cursorIndex int) []Completion {
		completions := FindCompletions(state, chunk, cursorIndex)
		//in order to simplify tess we remove all information in replaced ranges except the node span
		for i, compl := range completions {
			completions[i].ReplacedRange = parse.SourcePositionRange{
				SourceName:  "",
				StartLine:   0,
				StartColumn: 0,
				Span:        compl.ReplacedRange.Span,
			}
		}
		return completions
	}

	parseChunkSource := func(s, name string) (*parse.ParsedChunk, error) {
		return parse.ParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: s,
		})
	}

	t.Run("identifier member expression", func(t *testing.T) {
		t.Run("suggest object property: object has no property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}), map[string]core.Value{
				"obj": core.NewObject(),
			})
			chunk, _ := parseChunkSource("obj.", "")

			completions := findCompletions(state, chunk, 4)
			assert.Empty(t, completions)
		})

		t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.", "")

			completions := findCompletions(state, chunk, 4)
			assert.EqualValues(t, []Completion{
				{ShownString: "obj.name", Value: "obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 4}}},
			}, completions)
		})

		t.Run("suggest object property: start of property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("obj.n", "")

			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: "obj.name", Value: "obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 5}}},
			}, completions)
		})

		t.Run("suggest struct's field: start of field name: struct has single field", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}), map[string]core.Value{
				"struct": core.ValOf(core.FileInfo{Name: "foo"}),
			})
			chunk, _ := parseChunkSource("struct.n", "")

			completions := findCompletions(state, chunk, 8)
			assert.EqualValues(t, []Completion{
				{ShownString: "struct.name", Value: "struct.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 8}}},
			}, completions)
		})

		t.Run("suggest struct's method: start of method's name", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}), map[string]core.Value{
				"struct": core.ValOf(&core.Routine{}),
			})
			chunk, _ := parseChunkSource("struct.c", "")

			completions := findCompletions(state, chunk, 8)
			assert.EqualValues(t, []Completion{
				{ShownString: "struct.cancel", Value: "struct.cancel", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 8}}},
			}, completions)
		})
	})

	t.Run("member expression", func(t *testing.T) {
		t.Run("member expression: suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("$$obj.", "")

			completions := findCompletions(state, chunk, 6)
			assert.EqualValues(t, []Completion{
				{ShownString: "$$obj.name", Value: "$$obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 6}}},
			}, completions)
		})

		t.Run("member expression: suggest object property: start of property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parseChunkSource("$$obj.n", "")

			completions := findCompletions(state, chunk, 7)
			assert.EqualValues(t, []Completion{
				{ShownString: "$$obj.name", Value: "$$obj.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 7}}},
			}, completions)
		})

		t.Run("member expression: suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{
				"object": core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx),
			}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)

			chunk, _ := parseChunkSource("$$obj.object.", "")

			completions := findCompletions(state, chunk, 13)
			assert.EqualValues(t, []Completion{
				{ShownString: "$$obj.object.name", Value: "$$obj.object.name", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 13}}},
			}, completions)
		})

	})

	t.Run("subcommand", func(t *testing.T) {
		t.Run("depth 0", func(t *testing.T) {
			//TODO: implement
			t.Skip()
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := utils.Must(parseChunkSource("cmd ", ""))

			completions := findCompletions(state, chunk, 4)
			assert.EqualValues(t, []Completion{
				{ShownString: "help", Value: "help", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("depth 1", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := utils.Must(parseChunkSource("cmd help ", ""))

			completions := findCompletions(state, chunk, 9)
			assert.EqualValues(t, []Completion{
				{ShownString: "build", Value: "build", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
				{ShownString: "run", Value: "run", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
			}, completions)
		})

		t.Run("depth 0, subcommand of depth 1 is present ", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := utils.Must(parseChunkSource("cmd  build", ""))

			completions := findCompletions(state, chunk, 4)
			assert.EqualValues(t, []Completion{
				{ShownString: "help", Value: "help", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("suggest subcommand from subcommand prefix : depth 0", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := utils.Must(parseChunkSource("cmd h", ""))

			completions := findCompletions(state, chunk, 5)
			assert.EqualValues(t, []Completion{
				{ShownString: "help", Value: "help", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 4, End: 5}}},
			}, completions)
		})

		t.Run("suggest subcommand from subcommand prefix : depth 1", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := utils.Must(parseChunkSource("cmd help b", ""))

			completions := findCompletions(state, chunk, 10)
			assert.EqualValues(t, []Completion{
				{ShownString: "build", Value: "build", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 9, End: 10}}},
			}, completions)
		})

	})

	t.Run("absolute path", func(t *testing.T) {
		state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))

		code := dir + "/f"
		chunk, _ := parseChunkSource(code, "")

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
		state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))

		reldir, _ := filepath.Rel(wd, dir)
		code := reldir + "/f"
		chunk, _ := parseChunkSource(code, "")

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
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("for []{b}", "")

			completions := findCompletions(state, chunk, 8)
			assert.EqualValues(t, []Completion{
				{ShownString: "break", Value: "break", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 7, End: 8}}},
			}, completions)
		})

		t.Run("in if's block within a for statement", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("for []{if true {b}}", "")

			completions := findCompletions(state, chunk, 17)
			assert.EqualValues(t, []Completion{
				{ShownString: "break", Value: "break", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 16, End: 17}}},
			}, completions)
		})

	})

	t.Run("suggest continue", func(t *testing.T) {

		t.Run("in for statement's block", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("for []{cont}", "")

			completions := findCompletions(state, chunk, 11)
			assert.EqualValues(t, []Completion{
				{ShownString: "continue", Value: "continue", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 7, End: 11}}},
			}, completions)
		})

		t.Run("in if's block within a for statement", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("for []{if true {cont}}", "")

			completions := findCompletions(state, chunk, 20)
			assert.EqualValues(t, []Completion{
				{ShownString: "continue", Value: "continue", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 16, End: 20}}},
			}, completions)
		})

	})

	t.Run("prune", func(t *testing.T) {

		t.Run("in walk statement's block", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("walk ./ e {p}", "")

			completions := findCompletions(state, chunk, 12)
			assert.EqualValues(t, []Completion{
				{ShownString: "prune", Value: "prune", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 11, End: 12}}},
			}, completions)
		})

		t.Run("in for statement's block within a walk statement", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("walk ./ e {for []{p}}", "")

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
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parseChunkSource(string(keyword[0]), "")

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
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("u", "")

			completions := findCompletions(state, chunk, 1)
			assert.EqualValues(t, []Completion{
				{ShownString: "udata", Value: "udata", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
			}, completions)
		})

		t.Run("in call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("f(u)", "")

			completions := findCompletions(state, chunk, 3)
			assert.EqualValues(t, []Completion{
				{ShownString: "udata", Value: "udata", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 2, End: 3}}},
			}, completions)
		})

	})

	t.Run("Mapping", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("M", "")

			completions := findCompletions(state, chunk, 1)
			assert.EqualValues(t, []Completion{
				{ShownString: "Mapping", Value: "Mapping", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
			}, completions)
		})

		t.Run("in call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("f(M)", "")

			completions := findCompletions(state, chunk, 3)
			assert.EqualValues(t, []Completion{
				{ShownString: "Mapping", Value: "Mapping", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 2, End: 3}}},
			}, completions)
		})

	})

	t.Run("concat", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("c", "")

			completions := findCompletions(state, chunk, 1)
			assert.EqualValues(t, []Completion{
				{ShownString: "concat", Value: "concat", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 0, End: 1}}},
			}, completions)
		})

		t.Run("in call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parseChunkSource("f(c)", "")

			completions := findCompletions(state, chunk, 3)
			assert.EqualValues(t, []Completion{
				{ShownString: "concat", Value: "concat", ReplacedRange: parse.SourcePositionRange{Span: parse.NodeSpan{Start: 2, End: 3}}},
			}, completions)
		})

	})

}
