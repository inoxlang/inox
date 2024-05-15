package hsanalysis

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis/text"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
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

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
	})

	t.Run("empty init feature", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" {init}></div>`,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
	})

	t.Run("tell command containing an element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" {on click tell closest .A log :count}> </div> `,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD, chunk.GetSourcePosition(parse.NodeSpan{Start: 45, End: 51})),
		}, result.Errors)
	})

	t.Run("behaviors should not be defined in an Hyperscript attribute", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" {behavior X end}> </div> `,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Errors)
		assert.Equal(t, []Warning{
			MakeWarning(text.BEHAVIORS_SHOULD_BE_DEFINED_IN_HS_FILES, chunk.GetSourcePosition(parse.NodeSpan{Start: 16, End: 26})),
		}, result.Warnings)
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

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
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

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
	})

	t.Run("tell command containing an element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A">  <div {on click tell closest .A log :count}></div> </div>`,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD, chunk.GetSourcePosition(parse.NodeSpan{Start: 52, End: 58})),
		}, result.Errors)
	})

	t.Run("tell command containing an attribute reference", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A">  <div {on click tell closest .A log @name}></div> </div>`,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.ATTR_NOT_REF_TO_ATTR_OF_ELEM_REF_BY_TELL_CMD, chunk.GetSourcePosition(parse.NodeSpan{Start: 52, End: 57})),
		}, result.Errors)
	})

	t.Run("behaviors should not be defined in an Hyperscript attribute", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A">  <div {behavior X end}> </div> </div> `,
		})

		shorthand := parse.FindFirstNode(chunk.Node, (*parse.HyperscriptAttributeShorthand)(nil))

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Errors)
		assert.Equal(t, []Warning{
			MakeWarning(text.BEHAVIORS_SHOULD_BE_DEFINED_IN_HS_FILES, chunk.GetSourcePosition(parse.NodeSpan{Start: 23, End: 33})),
		}, result.Warnings)
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

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
	})

	t.Run("probably not-defined element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" x="((:a))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), ":a")).NodeData

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtElementScopeVarMayNotBeDefined(":a", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 20, End: 22})),
		}, result.Errors)
	})

	t.Run("reference to initialized attribute", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" y="((@data-x))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
	})

	t.Run("reference to an attribute that is not initialized", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" y="((@data-x))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtAttributeMayNotBeInitialized("data-x", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 20, End: 27})),
		}, result.Errors)
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

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
	})

	t.Run("probably not-defined element-scoped variable", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A"> ((:a)) </div> `,
		})

		markupText := parse.FindFirstNode(chunk.Node, (*parse.MarkupText)(nil))
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), ":a")).NodeData

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtElementScopeVarMayNotBeDefined(":a", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 18, End: 20})),
		}, result.Errors)
	})

	t.Run("reference to initialized attribute", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A"> ((@data-x)) </div> `,
		})

		markupText := parse.FindFirstNode(chunk.Node, (*parse.MarkupText)(nil))
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Empty(t, result.Errors)
	})

	t.Run("reference to an attribute that is not initialized", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: `<div class="A" y="((@data-x))">  </div> `,
		})

		strLit := parse.FindNodes(chunk.Node, (*parse.DoubleQuotedStringLiteral)(nil), nil)[1]
		hyperscriptExpr := utils.Ret0OutOf3(hsparse.ParseHyperScriptExpression(context.Background(), "@data-x")).NodeData

		result, err := Analyze(Parameters{
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

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.FmtAttributeMayNotBeInitialized("data-x", true), chunk.GetSourcePosition(parse.NodeSpan{Start: 20, End: 27})),
		}, result.Errors)
	})
}

func TestAnalyzeHyperscripFile(t *testing.T) {

	locationKind := HyperscriptScriptFile

	parse.RegisterParseHypercript(hsparse.ParseHyperScriptProgram)

	t.Run("behavior", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "behavior A end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]

			assert.Equal(t, "A", behavior.Name)
			assert.Equal(t, "A", behavior.FullName)
			assert.Empty(t, behavior.Namespace)
			assert.Empty(t, behavior.Features)

			assert.Equal(t, sourcecode.PositionRange{
				SourceName:  "/a._hs",
				StartLine:   1,
				StartColumn: 1,
				EndLine:     1,
				EndColumn:   11,
				Span:        sourcecode.NodeSpan{Start: 0, End: 10},
			}, behavior.Location)
		})

		t.Run("namespaced named", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "behavior A.B end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]

			assert.Equal(t, "B", behavior.Name)
			assert.Equal(t, "A.B", behavior.FullName)
			assert.Equal(t, []string{"A"}, behavior.Namespace)
			assert.Empty(t, behavior.Features)
		})

		t.Run("one feature", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "behavior A init end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]
			assert.Equal(t, "A", behavior.Name)
			assert.Equal(t, "A", behavior.FullName)
			assert.Empty(t, behavior.Namespace)

			if !assert.Len(t, behavior.Features, 1) {
				return
			}
			feature := behavior.Features[0]
			assert.True(t, hscode.IsNodeOfType(feature, hscode.InitFeature))
		})

		t.Run("two features", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "behavior A\ninit\n init\n end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]
			assert.Equal(t, "A", behavior.Name)
			assert.Equal(t, "A", behavior.FullName)
			assert.Empty(t, behavior.Namespace)

			if !assert.Len(t, behavior.Features, 2) {
				return
			}
			feature0 := behavior.Features[0]
			assert.True(t, hscode.IsNodeOfType(feature0, hscode.InitFeature))

			feature1 := behavior.Features[1]
			assert.True(t, hscode.IsNodeOfType(feature1, hscode.InitFeature))
		})

		t.Run("initialization of an element scoped variable and an attribute", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString: `
					behavior A
						init 
							set :a to 0
							set @data-x to 0
					end
				`,
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]
			assert.Equal(t, []string{":a"}, behavior.InitialElementScopeVarNames)
			assert.Equal(t, []string{"data-x"}, behavior.InitializedDataAttributeNames)
		})

		t.Run("event handler", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString: `
					behavior A
						on click
					end
				`,
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]
			assert.Equal(t, []DOMEvent{{Type: "click"}}, behavior.HandledEvents)
		})

		t.Run("event handler", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString: `
					behavior A
						on click end
					end
				`,
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]
			assert.Equal(t, []DOMEvent{{Type: "click"}}, behavior.HandledEvents)
		})

		t.Run("behavior install", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "behavior A\ninstall B(x: 1) end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.Behaviors, 1) {
				return
			}

			behavior := result.Behaviors[0]
			if !assert.Len(t, behavior.Installs, 1) {
				return
			}

			install := behavior.Installs[0]

			assert.Equal(t, "B", install.BehaviorFullName)
			if !assert.Len(t, install.Fields, 1) {
				return
			}
			field := install.Fields[0]

			assert.Equal(t, "x", field.Name)
			assert.True(t, hscode.IsNodeOfType(field.Value, hscode.NumberLiteral))

			assert.Equal(t, sourcecode.PositionRange{
				SourceName:  "/a._hs",
				StartLine:   2,
				StartColumn: 1,
				EndLine:     2,
				EndColumn:   16,
				Span:        sourcecode.NodeSpan{Start: 11, End: 26},
			}, install.Location)
		})
	})

	t.Run("function definition", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "def f() end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.FunctionDefinitions, 1) {
				return
			}

			definition := result.FunctionDefinitions[0]

			assert.Equal(t, "f", definition.Name)
			assert.Empty(t, definition.CommandList)
			assert.Empty(t, definition.ArgNames)
		})

		t.Run("one argument", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "def f(arg) end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.FunctionDefinitions, 1) {
				return
			}

			definition := result.FunctionDefinitions[0]

			assert.Equal(t, "f", definition.Name)
			assert.Empty(t, definition.CommandList)
			assert.Equal(t, []string{"arg"}, definition.ArgNames)
		})

		t.Run("one command", func(t *testing.T) {

			file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
				NameString:  "/a._hs",
				Resource:    "/a._hs",
				ResourceDir: "/",
				CodeString:  "def f() log 1 end",
			}, nil, nil))

			result, err := Analyze(Parameters{
				LocationKind:        locationKind,
				Chunk:               file,
				CodeStartIndex:      0,
				ProgramOrExpression: file.Result.NodeData,
			})

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Len(t, result.FunctionDefinitions, 1) {
				return
			}

			definition := result.FunctionDefinitions[0]

			assert.Equal(t, "f", definition.Name)
			if !assert.Len(t, definition.CommandList, 1) {
				return
			}
			assert.True(t, hscode.IsNodeOfType(definition.CommandList[0], hscode.LogCommand))
			assert.Empty(t, definition.ArgNames)
		})
	})

	t.Run("unexpected top-level install", func(t *testing.T) {

		file := utils.Must(hsparse.ParseFile(context.Background(), sourcecode.File{
			NameString:  "/a._hs",
			Resource:    "/a._hs",
			ResourceDir: "/",
			CodeString:  "install X",
		}, nil, nil))

		result, err := Analyze(Parameters{
			LocationKind:        locationKind,
			Chunk:               file,
			CodeStartIndex:      0,
			ProgramOrExpression: file.Result.NodeData,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, result.Warnings)
		assert.Equal(t, []Error{
			MakeError(text.BEHAVIOR_CAN_ONLY_BE_INSTALLED_FROM_HS_ATTR_OR_BEHAVIOR, file.GetSourcePosition(parse.NodeSpan{Start: 0, End: 9})),
		}, result.Errors)
	})
}
