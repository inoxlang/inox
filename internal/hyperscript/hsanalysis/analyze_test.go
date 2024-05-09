package hsanalysis

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis/text"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeHyperscriptAttributeOfComponent(t *testing.T) {

	locationKind := ComponentUnderscoreAttribute

	parse.RegisterParseHypercript(hsparse.ParseHyperScriptProgram)

	t.Run("empty", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" {}></div>`,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      shorthand.Span.Start + 1,
			ProgramOrExpression: shorthand.HyperscriptParsingResult.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("empty init feature", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" {init}></div>`,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      shorthand.Span.Start + 1,
			ProgramOrExpression: shorthand.HyperscriptParsingResult.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("tell command containing an element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" {on click tell closest .A log :count}> </div> `,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      shorthand.Span.Start + 1,
			ProgramOrExpression: shorthand.HyperscriptParsingResult.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Equal(t, []Error{
			MakeError(text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD, chunk.GetSourcePosition(parse.NodeSpan{Start: 45, End: 51})),
		}, errors)
	})
}

func TestAnalyzeHyperscriptAttributeOfNonComponent(t *testing.T) {

	locationKind := UnderscoreAttribute

	parse.RegisterParseHypercript(hsparse.ParseHyperScriptProgram)

	t.Run("empty", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `
				<div class="A">
					<div {}></div>
				</div>
			`,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      shorthand.Span.Start + 1,
			ProgramOrExpression: shorthand.HyperscriptParsingResult.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("empty init feature", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `
				<div class="A">
					<div {}></div>
				</div>
			`,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      shorthand.Span.Start + 1,
			ProgramOrExpression: shorthand.HyperscriptParsingResult.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("tell command containing an element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A">  <div {on click tell closest .A log :count}></div> </div> `,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      shorthand.Span.Start + 1,
			ProgramOrExpression: shorthand.HyperscriptParsingResult.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Equal(t, []Error{
			MakeError(text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD, chunk.GetSourcePosition(parse.NodeSpan{Start: 52, End: 58})),
		}, errors)
	})

	t.Run("tell command containing an attribute reference", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A">  <div {on click tell closest .A log @name}></div> </div> `,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      shorthand.Span.Start + 1,
			ProgramOrExpression: shorthand.HyperscriptParsingResult.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Equal(t, []Error{
			MakeError(text.ATTR_NOT_REF_TO_ATTR_OF_ELEM_REF_BY_TELL_CMD, chunk.GetSourcePosition(parse.NodeSpan{Start: 52, End: 57})),
		}, errors)
	})
}

func TestAnalyzeClientSideAttributeInterpolation(t *testing.T) {

	locationKind := ClientSideAttributeInterpolation

	parse.RegisterParseHypercript(hsparse.ParseHyperScriptProgram)

	t.Run("defined element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" x-for=":a in :list" y="((:a))"> </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[2]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), ":a")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name:                        "A",
				InitialElementScopeVarNames: []string{":a"},
			},
			Chunk:               chunk,
			CodeStartIndex:      strLit.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("probably not-defined element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" x="((:a))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), ":a")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      strLit.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtElementScopeVarMayNotBeDefined(":a", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 20, End: 22})),
		}, errors)
	})

	t.Run("reference to initialized attribute", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" y="((@data-x))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name:                          "A",
				InitializedDataAttributeNames: []string{"data-x"},
			},
			Chunk:               chunk,
			CodeStartIndex:      strLit.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("reference to an attribute that is not initialized", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" y="((@data-x))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      strLit.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtAttributeMayNotBeInitialized("data-x", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 20, End: 27})),
		}, errors)
	})
}

func TestAnalyzeClientSideTextInterpolation(t *testing.T) {

	locationKind := ClientSideTextInterpolation

	parse.RegisterParseHypercript(hsparse.ParseHyperScriptProgram)

	t.Run("defined element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" x-for=":a in :list"> ((:a)) </div> `,
		})

		markupText := parse.FindFirstNode(chunk.Node, (*parse.MarkupText)(nil))
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), ":a")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name:                        "A",
				InitialElementScopeVarNames: []string{":a"},
			},
			Chunk:               chunk,
			CodeStartIndex:      markupText.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("probably not-defined element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A"> ((:a)) </div> `,
		})

		markupText := parse.FindFirstNode(chunk.Node, (*parse.MarkupText)(nil))
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), ":a")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      markupText.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtElementScopeVarMayNotBeDefined(":a", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 18, End: 20})),
		}, errors)
	})

	t.Run("reference to initialized attribute", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A"> ((@data-x)) </div> `,
		})

		markupText := parse.FindFirstNode(chunk.Node, (*parse.MarkupText)(nil))
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name:                          "A",
				InitializedDataAttributeNames: []string{"data-x"},
			},
			Chunk:               chunk,
			CodeStartIndex:      markupText.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Empty(t, errors)
	})

	t.Run("reference to an attribute that is not initialized", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" y="((@data-x))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		errors, warnings, err := Analyze(Parameters{
			LocationKind: locationKind,
			Component: &Component{
				Name: "A",
			},
			Chunk:               chunk,
			CodeStartIndex:      strLit.Span.Start + 3,
			ProgramOrExpression: hyperscriptExpr,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtAttributeMayNotBeInitialized("data-x", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 20, End: 27})),
		}, errors)
	})
}
