package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
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
		code := `go {allow: {}} do {   go do {}  }`

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "symbolic-core-test",
			CodeString: code,
		}))

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
		assert.Equal(t, parse.SourcePositionRange{
			SourceName:  chunk.Source.Name(),
			StartLine:   1,
			StartColumn: 23,
			EndLine:     1,
			EndColumn:   25,
			Span:        parse.NodeSpan{Start: 22, End: 24},
		}, warning.Location[0])
	})
}

func TestBidirectionalSymbolicConversion(t *testing.T) {

	t.Run("object", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			obj := NewObject()
			v, err := ToSymbolicValue(nil, obj, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Object)(nil), v)
			symbolicObj := v.(*symbolic.Object)

			entries := symbolicObj.ValueEntryMap()
			assert.Empty(t, entries)

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assertEqualInoxValues(t, obj, concreteValue.(Value), ctx)
		})

		t.Run("single property", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			obj := NewObjectFromMapNoInit(ValMap{"a": Int(1)})
			v, err := ToSymbolicValue(nil, obj, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Object)(nil), v)
			symbolicObj := v.(*symbolic.Object)

			entries := symbolicObj.ValueEntryMap()
			assert.Equal(t, map[string]symbolic.Value{"a": symbolic.INT_1}, entries)

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assertEqualInoxValues(t, obj, concreteValue.(Value), ctx)
		})

		t.Run("two properties", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			obj := NewObjectFromMapNoInit(ValMap{"a": Int(1), "b": Str("a")})
			v, err := ToSymbolicValue(nil, obj, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Object)(nil), v)
			symbolicObj := v.(*symbolic.Object)

			entries := symbolicObj.ValueEntryMap()
			assert.Equal(t, map[string]symbolic.Value{"a": symbolic.INT_1, "b": symbolic.NewString("a")}, entries)

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assertEqualInoxValues(t, obj, concreteValue.(Value), ctx)
		})
	})

	t.Run("record", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			record := NewEmptyRecord()
			v, err := ToSymbolicValue(nil, record, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Record)(nil), v)
			symbolicObj := v.(*symbolic.Record)

			entries := symbolicObj.ValueEntryMap()
			assert.Empty(t, entries)

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, record, concreteValue)
		})

		t.Run("single property", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			record := NewRecordFromMap(ValMap{"a": Int(1)})
			v, err := ToSymbolicValue(nil, record, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Record)(nil), v)
			symbolicObj := v.(*symbolic.Record)

			entries := symbolicObj.ValueEntryMap()
			assert.Equal(t, map[string]symbolic.Value{"a": symbolic.INT_1}, entries)

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, record, concreteValue)
		})

		t.Run("two properties", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			record := NewRecordFromMap(ValMap{"a": Int(1), "b": Str("a")})
			v, err := ToSymbolicValue(nil, record, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Record)(nil), v)
			symbolicObj := v.(*symbolic.Record)

			entries := symbolicObj.ValueEntryMap()
			assert.Equal(t, map[string]symbolic.Value{"a": symbolic.INT_1, "b": symbolic.NewString("a")}, entries)

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, record, concreteValue)
		})
	})

	t.Run("tuple", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			tuple := NewTuple([]Serializable{})
			v, err := ToSymbolicValue(nil, tuple, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Tuple)(nil), v)
			symbolicTuple := v.(*symbolic.Tuple)

			assert.True(t, symbolicTuple.HasKnownLen())
			assert.Equal(t, 0, symbolicTuple.KnownLen())

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, tuple, concreteValue)
		})
	})

	t.Run("ordered pair", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		pair := NewOrderedPair(Int(1), Int(2))
		v, err := ToSymbolicValue(nil, pair, false)
		assert.NoError(t, err)

		assert.IsType(t, (*symbolic.OrderedPair)(nil), v)
		symbolicOrderedPair := v.(*symbolic.OrderedPair)

		assert.True(t, symbolicOrderedPair.HasKnownLen())
		assert.Equal(t, 2, symbolicOrderedPair.KnownLen())

		concreteValue, err := symbolic.Concretize(v, ctx)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, pair, concreteValue)
	})

	t.Run("dictionary", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			dict := NewDictionary(map[string]Serializable{})
			v, err := ToSymbolicValue(nil, dict, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Dictionary)(nil), v)
			symbolicDict := v.(*symbolic.Dictionary)
			assert.Len(t, symbolicDict.Entries(), len(dict.entries))

			assert.Empty(t, symbolicDict.Entries())
			assert.Empty(t, symbolicDict.Keys())

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, dict, concreteValue)
		})

		t.Run("single entry", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stringJSONRepr := GetJSONRepresentation(Str("name"), nil, nil)

			dict := NewDictionary(map[string]Serializable{
				stringJSONRepr: Str("foo"),
			})
			v, err := ToSymbolicValue(nil, dict, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Dictionary)(nil), v)
			symbolicDict := v.(*symbolic.Dictionary)
			assert.Len(t, symbolicDict.Entries(), len(dict.entries))

			assert.Equal(t, map[string]symbolic.Serializable{`"name"`: symbolic.NewString("foo")}, symbolicDict.Entries())
			assert.Equal(t, map[string]symbolic.Serializable{`"name"`: symbolic.NewString("name")}, symbolicDict.Keys())

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, dict, concreteValue)
		})

		t.Run("two entries", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			stringRepr := GetJSONRepresentation(Str("name"), nil, nil)
			pathRepr := GetJSONRepresentation(Path("./file"), nil, nil)

			dict := NewDictionary(map[string]Serializable{
				stringRepr: Str("foo"),
				pathRepr:   True,
			})
			v, err := ToSymbolicValue(nil, dict, false)
			assert.NoError(t, err)

			assert.IsType(t, (*symbolic.Dictionary)(nil), v)
			symbolicDict := v.(*symbolic.Dictionary)
			assert.Len(t, symbolicDict.Entries(), len(dict.entries))

			assert.Equal(t, map[string]symbolic.Serializable{
				stringRepr: symbolic.NewString("foo"),
				pathRepr:   symbolic.TRUE,
			}, symbolicDict.Entries())
			assert.Equal(t, map[string]symbolic.Serializable{
				stringRepr: symbolic.NewString("name"),
				pathRepr:   symbolic.NewPath("./file"),
			}, symbolicDict.Keys())

			concreteValue, err := symbolic.Concretize(v, ctx)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, dict, concreteValue)
		})
	})

	t.Run("object pattern", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		patt := NewInexactObjectPattern([]ObjectPatternEntry{
			{
				Name:    "a",
				Pattern: INT_PATTERN,
			},
		})
		symb, err := patt.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})
		if assert.NoError(t, err) {
			expected := symbolic.NewInexactObjectPattern(
				map[string]symbolic.Pattern{
					"a": utils.Must(INT_PATTERN.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})).(symbolic.Pattern),
				}, nil)

			assert.Equal(t, symbolic.Stringify(expected), symbolic.Stringify(symb))
		}

		patt = NewInexactObjectPattern([]ObjectPatternEntry{
			{
				Name:       "a",
				Pattern:    INT_PATTERN,
				IsOptional: true,
			},
		})

		symb, err = patt.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})
		if !assert.NoError(t, err) {
			return
		}

		expected := symbolic.NewInexactObjectPattern(
			map[string]symbolic.Pattern{
				"a": utils.Must(INT_PATTERN.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{})).(symbolic.Pattern),
			},
			//optional entries.
			map[string]struct{}{"a": {}},
		)

		assert.Equal(t, symbolic.Stringify(expected), symbolic.Stringify(symb))

		concreteValue, err := symbolic.Concretize(symb, ctx)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, patt, concreteValue)
	})

	t.Run("cycles", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)
		defer ctx.CancelGracefully()

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
