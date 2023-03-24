package internal

import (
	"testing"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"
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

		_, err := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
			Node:   chunk.Node,
			Module: mod.ToSymbolic(),
			GlobalConsts: map[string]interface{}{
				"var": Int(1),
			},
			Context: symbolic.NewSymbolicContext(),
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

		_, err := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
			Node:   chunk.Node,
			Module: mod.ToSymbolic(),
			GlobalConsts: map[string]interface{}{
				"var": Int(1),
			},
			Context: symbolic.NewSymbolicContext(),
		})

		assert.NoError(t, err)
	})

}

func TestToSymbolicValue(t *testing.T) {

	t.Run("dictionary", func(t *testing.T) {
		dict := NewDictionary(map[string]Value{`"name"`: Str("foo"), `./file`: True})
		v, err := ToSymbolicValue(dict, false)
		assert.NoError(t, err)

		assert.IsType(t, &symbolic.Dictionary{}, v)
		symbolicDict := v.(*symbolic.Dictionary)
		assert.Len(t, symbolicDict.Entries, len(dict.Entries))

		assert.Equal(t, &symbolic.Dictionary{
			Entries: map[string]symbolic.SymbolicValue{`"name"`: &symbolic.String{}, `./file`: &symbolic.Bool{}},
			Keys:    map[string]symbolic.SymbolicValue{`"name"`: &symbolic.String{}, `./file`: &symbolic.Path{}},
		}, symbolicDict)
	})

	t.Run("cycles", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := &Object{}
		pattern := &ExactValuePattern{value: obj}
		obj.SetProp(ctx, "self", obj)
		obj.SetProp(ctx, "exact_pattern", pattern)

		v, err := ToSymbolicValue(pattern, false)
		assert.NoError(t, err)
		symPattern := v.(*symbolic.ExactValuePattern)
		symObject := symPattern.GetVal().(*symbolic.Object)

		self, _, _ := symObject.GetProperty("self")
		exact_pattern, _, _ := symObject.GetProperty("exact_pattern")

		assert.Same(t, symObject, self.(*symbolic.Object))
		assert.Same(t, symPattern, exact_pattern.(*symbolic.ExactValuePattern))
	})

}
