package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicEvalCheck(t *testing.T) {

	t.Run("predefined global variables do not cause an error", func(t *testing.T) {
		code := `return ($$var + 1)`
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "symbolic-core-test",
			CodeString: code,
		}))

		mod := &Module{MainChunk: chunk}

		_, err := symbolic.EvalCheck(symbolic.EvalCheckInput{
			Node:   chunk.Node,
			Module: mod.ToSymbolic(),
			Globals: map[string]symbolic.ConcreteGlobalValue{
				"var": {Value: Int(1), IsConstant: false},
			},
			Context: symbolic.NewSymbolicContext(nil, nil, nil),
		})

		assert.NoError(t, err)
	})

	t.Run("", func(t *testing.T) {
		code := `return ($$var + 1)`
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "symbolic-core-test",
			CodeString: code,
		}))

		mod := &Module{MainChunk: chunk}

		_, err := symbolic.EvalCheck(symbolic.EvalCheckInput{
			Node:   chunk.Node,
			Module: mod.ToSymbolic(),
			Globals: map[string]symbolic.ConcreteGlobalValue{
				"var": {Value: Int(1), IsConstant: false},
			},
			Context: symbolic.NewSymbolicContext(nil, nil, nil),
		})

		assert.NoError(t, err)
	})

	t.Run("spawn expression (permission ok)", func(t *testing.T) {
		code := `go {globals: {global2: 2}} do { return (global1 + global2)}`
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "symbolic-core-test",
			CodeString: code,
		}))

		mod := &Module{MainChunk: chunk}

		data, err := symbolic.EvalCheck(symbolic.EvalCheckInput{
			Node:   chunk.Node,
			Module: mod.ToSymbolic(),
			Globals: map[string]symbolic.ConcreteGlobalValue{
				"global1": {Value: Int(1), IsConstant: true},
			},
			Context: symbolic.NewSymbolicContext(NewContext(ContextConfig{
				Permissions: []Permission{LThreadPermission{Kind_: permkind.Create}},
			}), nil, nil),
		})

		assert.NoError(t, err)
		assert.Empty(t, data.Errors())
		assert.Empty(t, data.Warnings())
	})

	t.Run("spawn expression (missing permission)", func(t *testing.T) {
		code := `go {globals: {global2: 2}} do { return (global1 + global2)}`
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "symbolic-core-test",
			CodeString: code,
		}))

		mod := &Module{MainChunk: chunk}

		data, err := symbolic.EvalCheck(symbolic.EvalCheckInput{
			Node:   chunk.Node,
			Module: mod.ToSymbolic(),
			Globals: map[string]symbolic.ConcreteGlobalValue{
				"global1": {Value: Int(1), IsConstant: true},
			},
			Context: symbolic.NewSymbolicContext(NewContext(ContextConfig{}), nil, nil),
		})

		assert.NoError(t, err)
		assert.Empty(t, data.Errors())
		if !assert.NotEmpty(t, data.Warnings()) {
			return
		}
		warning := data.Warnings()[0]
		assert.Contains(t, symbolic.POSSIBLE_MISSING_PERM_TO_CREATE_A_LTHREAD, warning.Message)
	})

	t.Run("spawn expression within embedded module (missing permission)", func(t *testing.T) {
		code := `go {allow: {}} do { 
			go do {}
		}`

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "symbolic-core-test",
			CodeString: code,
		}))

		innerSpanwExpr := parse.FindNode(chunk.Node, (*parse.SpawnExpression)(nil), func(e *parse.SpawnExpression, _ bool) bool {
			return e.Meta == nil
		})

		innerSpanwExprPos := chunk.GetSourcePosition(innerSpanwExpr.Span)

		mod := &Module{MainChunk: chunk}

		data, err := symbolic.EvalCheck(symbolic.EvalCheckInput{
			Node:    chunk.Node,
			Module:  mod.ToSymbolic(),
			Globals: map[string]symbolic.ConcreteGlobalValue{},
			Context: symbolic.NewSymbolicContext(NewContext(ContextConfig{
				Permissions: []Permission{LThreadPermission{Kind_: permkind.Create}},
			}), nil, nil),
		})

		assert.NoError(t, err)
		assert.Empty(t, data.Errors())
		if !assert.NotEmpty(t, data.Warnings()) {
			return
		}
		warning := data.Warnings()[0]
		assert.Contains(t, symbolic.POSSIBLE_MISSING_PERM_TO_CREATE_A_LTHREAD, warning.Message)
		assert.Equal(t, innerSpanwExprPos, warning.Location[0])
	})
}

func TestToSymbolicValue(t *testing.T) {

	t.Run("dictionary", func(t *testing.T) {
		dict := NewDictionary(map[string]Serializable{`"name"`: Str("foo"), `./file`: True})
		v, err := ToSymbolicValue(nil, dict, false)
		assert.NoError(t, err)

		assert.IsType(t, &symbolic.Dictionary{}, v)
		symbolicDict := v.(*symbolic.Dictionary)
		assert.Len(t, symbolicDict.Entries(), len(dict.entries))

		assert.Equal(t, map[string]symbolic.Serializable{`"name"`: symbolic.NewString("foo"), `./file`: symbolic.TRUE}, symbolicDict.Entries())
		assert.Equal(t, map[string]symbolic.Serializable{`"name"`: symbolic.NewString("name"), `./file`: symbolic.NewPath("./file")}, symbolicDict.Keys())
	})

	t.Run("object pattern", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		patt := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		symb, err := patt.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})
		if assert.NoError(t, err) {
			expected := symbolic.NewInexactObjectPattern(
				map[string]symbolic.Pattern{
					"a": utils.Must(INT_PATTERN.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})).(symbolic.Pattern),
				}, nil)

			assert.Equal(t, symbolic.Stringify(expected), symbolic.Stringify(symb))
		}

		patt = NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		symb, err = patt.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})
		if assert.NoError(t, err) {
			expected := symbolic.NewInexactObjectPattern(
				map[string]symbolic.Pattern{
					"a": utils.Must(INT_PATTERN.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})).(symbolic.Pattern),
				}, map[string]struct{}{"a": {}})

			assert.Equal(t, symbolic.Stringify(expected), symbolic.Stringify(symb))
		}
	})

	t.Run("cycles", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := &Object{}
		pattern := &ExactValuePattern{value: obj}
		obj.SetProp(ctx, "self", obj)
		obj.SetProp(ctx, "exact_pattern", pattern)

		v, err := ToSymbolicValue(nil, pattern, false)
		assert.NoError(t, err)
		symPattern := v.(*symbolic.ExactValuePattern)
		symObject := symPattern.GetVal().(*symbolic.Object)

		self, _, _ := symObject.GetProperty("self")
		exact_pattern, _, _ := symObject.GetProperty("exact_pattern")

		assert.Same(t, symObject, self.(*symbolic.Object))
		assert.Same(t, symPattern, exact_pattern.(*symbolic.ExactValuePattern))
	})

}
