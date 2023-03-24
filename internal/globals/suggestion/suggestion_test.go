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

func TestFindSuggestions(t *testing.T) {

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

	t.Run("identifier member expression", func(t *testing.T) {
		t.Run("suggest object property: object has no property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}), map[string]core.Value{
				"obj": core.NewObject(),
			})
			chunk, _ := parse.ParseChunk("obj.", "")

			suggestions := FindSuggestions(state, chunk, 4)
			assert.Empty(t, suggestions)
		})

		t.Run("suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parse.ParseChunk("obj.", "")

			suggestions := FindSuggestions(state, chunk, 4)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "obj.name", Value: "obj.name", Span: parse.NodeSpan{Start: 0, End: 4}},
			}, suggestions)
		})

		t.Run("suggest object property: start of property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parse.ParseChunk("obj.n", "")

			suggestions := FindSuggestions(state, chunk, 5)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "obj.name", Value: "obj.name", Span: parse.NodeSpan{Start: 0, End: 5}},
			}, suggestions)
		})

		t.Run("suggest struct's field: start of field name: struct has single field", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}), map[string]core.Value{
				"struct": core.ValOf(core.FileInfo{Name: "foo"}),
			})
			chunk, _ := parse.ParseChunk("struct.n", "")

			suggestions := FindSuggestions(state, chunk, 8)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "struct.name", Value: "struct.name", Span: parse.NodeSpan{Start: 0, End: 8}},
			}, suggestions)
		})

		t.Run("suggest struct's method: start of method's name", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}), map[string]core.Value{
				"struct": core.ValOf(&core.Routine{}),
			})
			chunk, _ := parse.ParseChunk("struct.c", "")

			suggestions := FindSuggestions(state, chunk, 8)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "struct.cancel", Value: "struct.cancel", Span: parse.NodeSpan{Start: 0, End: 8}},
			}, suggestions)
		})
	})

	t.Run("member expression", func(t *testing.T) {
		t.Run("member expression: suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parse.ParseChunk("$$obj.", "")

			suggestions := FindSuggestions(state, chunk, 6)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "$$obj.name", Value: "$$obj.name", Span: parse.NodeSpan{Start: 0, End: 6}},
			}, suggestions)
		})

		t.Run("member expression: suggest object property: start of property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)
			chunk, _ := parse.ParseChunk("$$obj.n", "")

			suggestions := FindSuggestions(state, chunk, 7)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "$$obj.name", Value: "$$obj.name", Span: parse.NodeSpan{Start: 0, End: 7}},
			}, suggestions)
		})

		t.Run("member expression: suggest object property: empty property name: object has single property", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			obj := core.NewObjectFromMap(core.ValMap{
				"object": core.NewObjectFromMap(core.ValMap{"name": core.Str("foo")}, state.Global.Ctx),
			}, state.Global.Ctx)
			state.SetGlobal("obj", obj, core.GlobalConst)

			chunk, _ := parse.ParseChunk("$$obj.object.", "")

			suggestions := FindSuggestions(state, chunk, 13)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "$$obj.object.name", Value: "$$obj.object.name", Span: parse.NodeSpan{Start: 0, End: 13}},
			}, suggestions)
		})

	})

	t.Run("subcommand", func(t *testing.T) {
		t.Run("depth 0", func(t *testing.T) {
			//TODO: implement
			t.Skip()
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := parse.MustParseChunk("cmd ")

			suggestions := FindSuggestions(state, chunk, 4)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "help", Value: "help", Span: parse.NodeSpan{Start: 4, End: 5}},
			}, suggestions)
		})

		t.Run("depth 1", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := parse.MustParseChunk("cmd help ")

			suggestions := FindSuggestions(state, chunk, 9)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "build", Value: "build", Span: parse.NodeSpan{Start: 9, End: 10}},
				{ShownString: "run", Value: "run", Span: parse.NodeSpan{Start: 9, End: 10}},
			}, suggestions)
		})

		t.Run("depth 0, subcommand of depth 1 is present ", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := parse.MustParseChunk("cmd  build")

			suggestions := FindSuggestions(state, chunk, 4)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "help", Value: "help", Span: parse.NodeSpan{Start: 4, End: 5}},
			}, suggestions)
		})

		t.Run("suggest subcommand from subcommand prefix : depth 0", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := parse.MustParseChunk("cmd h")

			suggestions := FindSuggestions(state, chunk, 5)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "help", Value: "help", Span: parse.NodeSpan{Start: 4, End: 5}},
			}, suggestions)
		})

		t.Run("suggest subcommand from subcommand prefix : depth 1", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk := parse.MustParseChunk("cmd help b")

			suggestions := FindSuggestions(state, chunk, 10)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "build", Value: "build", Span: parse.NodeSpan{Start: 9, End: 10}},
			}, suggestions)
		})

	})

	t.Run("absolute path", func(t *testing.T) {
		state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))

		code := dir + "/f"
		chunk, _ := parse.ParseChunk(code, "")

		suggestions := FindSuggestions(state, chunk, len(code))
		assert.EqualValues(t, []Suggestion{
			{ShownString: "file1.txt", Value: dir + "/file1.txt", Span: parse.NodeSpan{Start: 0, End: int32(len(code))}},
			{ShownString: "file2.txt", Value: dir + "/file2.txt", Span: parse.NodeSpan{Start: 0, End: int32(len(code))}},
		}, suggestions)
	})

	t.Run("relative path", func(t *testing.T) {
		state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))

		reldir, _ := filepath.Rel(wd, dir)
		code := reldir + "/f"
		chunk, _ := parse.ParseChunk(code, "")

		suggestions := FindSuggestions(state, chunk, len(code))
		assert.EqualValues(t, []Suggestion{
			{ShownString: "file1.txt", Value: reldir + "/file1.txt", Span: parse.NodeSpan{Start: 0, End: int32(len(code))}},
			{ShownString: "file2.txt", Value: reldir + "/file2.txt", Span: parse.NodeSpan{Start: 0, End: int32(len(code))}},
		}, suggestions)
	})

	t.Run("break", func(t *testing.T) {

		t.Run("in for statement's block", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("for []{b}", "")

			suggestions := FindSuggestions(state, chunk, 8)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "break", Value: "break", Span: parse.NodeSpan{Start: 7, End: 8}},
			}, suggestions)
		})

		t.Run("in if's block within a for statement", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("for []{if true {b}}", "")

			suggestions := FindSuggestions(state, chunk, 17)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "break", Value: "break", Span: parse.NodeSpan{Start: 16, End: 17}},
			}, suggestions)
		})

	})

	t.Run("suggest continue", func(t *testing.T) {

		t.Run("in for statement's block", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("for []{cont}", "")

			suggestions := FindSuggestions(state, chunk, 11)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "continue", Value: "continue", Span: parse.NodeSpan{Start: 7, End: 11}},
			}, suggestions)
		})

		t.Run("in if's block within a for statement", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("for []{if true {cont}}", "")

			suggestions := FindSuggestions(state, chunk, 20)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "continue", Value: "continue", Span: parse.NodeSpan{Start: 16, End: 20}},
			}, suggestions)
		})

	})

	t.Run("prune", func(t *testing.T) {

		t.Run("in walk statement's block", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("walk ./ e {p}", "")

			suggestions := FindSuggestions(state, chunk, 12)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "prune", Value: "prune", Span: parse.NodeSpan{Start: 11, End: 12}},
			}, suggestions)
		})

		t.Run("in for statement's block within a walk statement", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("walk ./ e {for []{p}}", "")

			suggestions := FindSuggestions(state, chunk, 19)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "prune", Value: "prune", Span: parse.NodeSpan{Start: 18, End: 19}},
			}, suggestions)
		})

	})

	t.Run("context independent keywords that start statements", func(t *testing.T) {

		for _, keyword := range CONTEXT_INDEPENDENT_STMT_STARTING_KEYWORDS {
			t.Run(keyword, func(t *testing.T) {
				t.Run("in top module", func(t *testing.T) {
					state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
					chunk, _ := parse.ParseChunk(string(keyword[0]), "")

					suggestions := FindSuggestions(state, chunk, 1)
					//we remove other keyword suggestions
					suggestions = utils.FilterSlice(suggestions, func(s Suggestion) bool { return s.Value == keyword })

					assert.EqualValues(t, []Suggestion{
						{ShownString: keyword, Value: keyword, Span: parse.NodeSpan{Start: 0, End: 1}},
					}, suggestions)
				})
			})
		}
	})

	t.Run("udata", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("u", "")

			suggestions := FindSuggestions(state, chunk, 1)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "udata", Value: "udata", Span: parse.NodeSpan{Start: 0, End: 1}},
			}, suggestions)
		})

		t.Run("in call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("f(u)", "")

			suggestions := FindSuggestions(state, chunk, 3)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "udata", Value: "udata", Span: parse.NodeSpan{Start: 2, End: 3}},
			}, suggestions)
		})

	})

	t.Run("Mapping", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("M", "")

			suggestions := FindSuggestions(state, chunk, 1)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "Mapping", Value: "Mapping", Span: parse.NodeSpan{Start: 0, End: 1}},
			}, suggestions)
		})

		t.Run("in call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("f(M)", "")

			suggestions := FindSuggestions(state, chunk, 3)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "Mapping", Value: "Mapping", Span: parse.NodeSpan{Start: 2, End: 3}},
			}, suggestions)
		})

	})

	t.Run("concat", func(t *testing.T) {

		t.Run("in top level module", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("c", "")

			suggestions := FindSuggestions(state, chunk, 1)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "concat", Value: "concat", Span: parse.NodeSpan{Start: 0, End: 1}},
			}, suggestions)
		})

		t.Run("in call", func(t *testing.T) {
			state := core.NewTreeWalkState(core.NewContext(core.ContextConfig{Permissions: perms}))
			chunk, _ := parse.ParseChunk("f(c)", "")

			suggestions := FindSuggestions(state, chunk, 3)
			assert.EqualValues(t, []Suggestion{
				{ShownString: "concat", Value: "concat", Span: parse.NodeSpan{Start: 2, End: 3}},
			}, suggestions)
		})

	})

}
