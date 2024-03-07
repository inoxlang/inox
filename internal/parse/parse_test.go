package parse

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestParseNoContext(t *testing.T) {
	testParse(t, func(t *testing.T, str string) (result *Chunk) {
		return mustParseChunkForgetTokens(str)
	}, func(t *testing.T, str, name string) (result *Chunk, err error) {
		return parseChunkForgetTokens(str, name)
	})
}

func TestParseSystematicCheckAndAlreadyDoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mustParseChunk := func(t *testing.T, str string) (result *Chunk) {
		err := (func() (err error) {
			defer func() {
				e := recover()
				if er, ok := e.(error); ok {
					err = er
				}
			}()
			mustParseChunkForgetTokens(str, ParserOptions{
				NoCheckFuel:   1, //check context every major function call during parsing.
				ParentContext: ctx,
			})
			return
		})()

		assert.ErrorContains(t, err, context.Canceled.Error())

		return mustParseChunkForgetTokens(str)
	}

	parseChunk := func(t *testing.T, str, name string) (result *Chunk, e error) {
		_, err := ParseChunk(str, name, ParserOptions{
			NoCheckFuel:   1, //check context every major function call during parsing.
			ParentContext: ctx,
		})

		assert.ErrorContains(t, err, context.Canceled.Error())

		return parseChunkForgetTokens(str, name)
	}

	testParse(t, mustParseChunk, parseChunk)
}

func TestParseNonSystematicCheckAndAlreadyDoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mustParseChunk := func(t *testing.T, str string) (result *Chunk) {
		err := (func() (err error) {
			defer func() {
				e := recover()
				if er, ok := e.(error); ok {
					err = er
				}
			}()
			mustParseChunkForgetTokens(str, ParserOptions{
				NoCheckFuel:   2, //check context every 2 major function calls during parsing.
				ParentContext: ctx,
			})
			return
		})()

		assert.ErrorContains(t, err, context.Canceled.Error())

		return mustParseChunkForgetTokens(str)
	}

	parseChunk := func(t *testing.T, str, name string) (result *Chunk, e error) {
		_, err := parseChunkForgetTokens(str, name, ParserOptions{
			NoCheckFuel:   2, //check context every 2 major function calls during parsing.
			ParentContext: ctx,
		})

		assert.ErrorContains(t, err, context.Canceled.Error())

		return parseChunkForgetTokens(str, name)
	}

	testParse(t, mustParseChunk, parseChunk)
}

func TestParseNonSystematicCheckAndAlreadyDoneContext2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	const MIN_CONTEXT_CHECK_TEST_NODE_COUNT = 6

	mustParseChunk := func(t *testing.T, str string) (result *Chunk) {
		n := mustParseChunkForgetTokens(str)
		nodeCount := CountNodes(n)

		if nodeCount < MIN_CONTEXT_CHECK_TEST_NODE_COUNT { //ignore context check test.
			return n
		}

		err := (func() (err error) {
			defer func() {
				e := recover()
				if er, ok := e.(error); ok {
					err = er
				}
			}()
			mustParseChunkForgetTokens(str, ParserOptions{
				NoCheckFuel:   nodeCount / 2, //check context somewhere during the parsing.
				ParentContext: ctx,
			})
			return
		})()

		assert.ErrorContains(t, err, context.Canceled.Error())

		return n
	}

	parseChunk := func(t *testing.T, str, name string) (result *Chunk, e error) {
		n, err := parseChunkForgetTokens(str, name)
		nodeCount := CountNodes(n)

		if nodeCount < MIN_CONTEXT_CHECK_TEST_NODE_COUNT { //ignore context check test.
			return n, err
		}

		_, err = parseChunkForgetTokens(str, name, ParserOptions{
			NoCheckFuel:   nodeCount / 2, //check context somewhere during the parsing.
			ParentContext: ctx,
		})

		assert.ErrorContains(t, err, context.Canceled.Error())

		return parseChunkForgetTokens(str, name)
	}

	testParse(t, mustParseChunk, parseChunk)
}

func TestParseSystematicCheckAndVeryShortTimeout(t *testing.T) {
	code := "[" + strings.Repeat("111,", 20_000) + "]"

	_, err := ParseChunk(code, "test", ParserOptions{
		NoCheckFuel:   1, //check context every major function call during parsing.
		ParentContext: context.Background(),
		Timeout:       time.Millisecond,
	})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestParseSystematicCheckAndDefaultTimeout(t *testing.T) {
	code := "[" + strings.Repeat("111,", 200_000) + "]"

	_, err := ParseChunk(code, "test", ParserOptions{})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCheckEmbddedModuleTokens(t *testing.T) {
	t.Run("empty: no tokens", func(t *testing.T) {
		chunk := MustParseChunk(`go do {}`)

		embeddedMod := FindNode(chunk, (*EmbeddedModule)(nil), nil)
		assert.Empty(t, embeddedMod.Tokens)
	})

	t.Run("empty: no tokens, missing closing bracket", func(t *testing.T) {
		chunk, _ := ParseChunk(`go do {`, "test")

		embeddedMod := FindNode(chunk, (*EmbeddedModule)(nil), nil)
		assert.Empty(t, embeddedMod.Tokens)
	})

	t.Run("empty: single non-stored token", func(t *testing.T) {
		chunk := MustParseChunk(`go do {1}`)

		embeddedMod := FindNode(chunk, (*EmbeddedModule)(nil), nil)
		assert.Empty(t, embeddedMod.Tokens)
	})

	t.Run("empty: single non-stored token, missing closing bracket", func(t *testing.T) {
		chunk, _ := ParseChunk(`go do {1`, "test")

		embeddedMod := FindNode(chunk, (*EmbeddedModule)(nil), nil)
		assert.Empty(t, embeddedMod.Tokens)
	})

	t.Run("empty: single stored token", func(t *testing.T) {
		chunk, _ := ParseChunk(`go do {?}`, "test")

		embeddedMod := FindNode(chunk, (*EmbeddedModule)(nil), nil)
		assert.Equal(t, []Token{{Type: UNEXPECTED_CHAR, Raw: "?", Span: NodeSpan{7, 8}}}, embeddedMod.Tokens)
	})

	t.Run("empty: single stored token, missing closing bracket", func(t *testing.T) {
		chunk, _ := ParseChunk(`go do {?`, "test")

		embeddedMod := FindNode(chunk, (*EmbeddedModule)(nil), nil)
		assert.Equal(t, []Token{{Type: UNEXPECTED_CHAR, Raw: "?", Span: NodeSpan{7, 8}}}, embeddedMod.Tokens)
	})
}

func TestParseChunkStart(t *testing.T) {
	opts := ParserOptions{Start: true}
	chunk := MustParseChunk("manifest {}", opts)
	assert.NotNil(t, chunk.Manifest)
	assert.Empty(t, chunk.Statements)

	chunk = MustParseChunk("manifest {}\na = 1", opts)
	assert.NotNil(t, chunk.Manifest)
	assert.Empty(t, chunk.Statements)

	chunk = MustParseChunk("manifest {};a = 1", opts)
	assert.NotNil(t, chunk.Manifest)
	assert.Empty(t, chunk.Statements)

	chunk = MustParseChunk("const(C=1)\nmanifest {}; a = 1", opts)
	assert.NotNil(t, chunk.Manifest)
	assert.NotNil(t, chunk.GlobalConstantDeclarations)
	assert.Empty(t, chunk.Statements)

	chunk = MustParseChunk("includable-file", opts)
	assert.NotNil(t, chunk.IncludableChunkDesc)
	assert.Empty(t, chunk.Statements)

	chunk = MustParseChunk("includable-file\nconst(A = 1)\na = 1", opts)
	assert.NotNil(t, chunk.IncludableChunkDesc)
	assert.NotNil(t, chunk.GlobalConstantDeclarations)
	assert.Empty(t, chunk.Statements)

	chunk = MustParseChunk("includable-file;const(A = 1)\na = 1", opts)
	assert.NotNil(t, chunk.IncludableChunkDesc)
	assert.NotNil(t, chunk.GlobalConstantDeclarations)
	assert.Empty(t, chunk.Statements)
}

//TODO: add more specific tests for testing context checks.

func testParse(
	t *testing.T,
	mustparseChunk func(t *testing.T, str string) (result *Chunk),
	parseChunk func(t *testing.T, str string, name string) (result *Chunk, err error),
) {

	t.Run("module", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, "")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 0}, nil, false},
			}, n)
		})

		t.Run("comment with missing space", func(t *testing.T) {
			n, err := parseChunk(t, "#", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&UnambiguousIdentifierLiteral{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 1},
							Err:  &ParsingError{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT},
						},
					},
				},
			}, n)

			aggregation := err.(*ParsingErrorAggregation)
			assert.Equal(t, []*ParsingError{{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT}}, aggregation.Errors)
			assert.Equal(t, []SourcePositionRange{
				{StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 2, Span: NodeSpan{0, 1}},
			}, aggregation.ErrorPositions)
		})

		t.Run("shebang", func(t *testing.T) {
			n := mustparseChunk(t, "#!/usr/local/bin/inox")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: nil,
			}, n)
		})

		t.Run("unexpected char", func(t *testing.T) {
			n, err := parseChunk(t, "]", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("non regular space", func(t *testing.T) {
			n, err := parseChunk(t, " ", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(' ')},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("carriage return", func(t *testing.T) {
			n := mustparseChunk(t, "\r")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
			}, n)
		})

		t.Run("line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 1},
					nil,
					false,
				},
			}, n)
		})

		t.Run("two line feeds", func(t *testing.T) {
			n := mustparseChunk(t, "\n\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{0, 1}},
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
					},*/
				},
			}, n)
		})

		t.Run("carriage return + line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\r\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					false,
				},
			}, n)
		})

		t.Run("twice: carriage return + line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\r\n\r\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 4},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
						{Type: NEWLINE, Span: NodeSpan{3, 4}},
					},*/
				},
			}, n)
		})

		t.Run("two lines with one statement per line", func(t *testing.T) {
			n := mustparseChunk(t, "1\n2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 3},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
					},*/
				},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Raw:      "1",
						Value:    1,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
						Raw:      "2",
						Value:    2,
					},
				},
			}, n)
		})

		t.Run("two lines with one statement per line, followed by line feed character", func(t *testing.T) {
			n := mustparseChunk(t, "1\n2\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 4},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
						{Type: NEWLINE, Span: NodeSpan{3, 4}},
					},*/
				},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Raw:      "1",
						Value:    1,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
						Raw:      "2",
						Value:    2,
					},
				},
			}, n)
		})

		t.Run("statements next to each other", func(t *testing.T) {
			n, err := parseChunk(t, "1$v", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Raw:      "1",
						Value:    1,
					},
					&Variable{
						NodeBase: NodeBase{
							NodeSpan{1, 3},
							&ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY},
							false,
						},
						Name: "v",
					},
				},
			}, n)
		})

		t.Run("empty preinit", func(t *testing.T) {
			n := mustparseChunk(t, "preinit {}")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{0, 10},
						IsParenthesized: false,
						/*[]Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{0, 7}},
						},*/
					},
					Block: &Block{
						NodeBase: NodeBase{
							NodeSpan{8, 10},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("empty preinit after line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\npreinit {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 11},
					nil,
					false,
				},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{1, 11},
						IsParenthesized: false,
						/*[]Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{1, 8}},
						},*/
					},
					Block: &Block{
						NodeBase: NodeBase{
							NodeSpan{9, 11},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("preinit with missing block", func(t *testing.T) {
			n, err := parseChunk(t, "preinit", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{0, 7},
						Err:             &ParsingError{UnspecifiedParsingError, PREINIT_KEYWORD_SHOULD_BE_FOLLOWED_BY_A_BLOCK},
						IsParenthesized: false,
						/*[]Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{0, 7}},
						},*/
					},
				},
			}, n)
		})

		t.Run("empty manifest", func(t *testing.T) {
			n := mustparseChunk(t, "manifest {}")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: nil,
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span:            NodeSpan{0, 11},
						IsParenthesized: false,
						/*[]Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{0, 8}},
						},*/
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{9, 11},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
							},*/
						},
						Properties: nil,
					},
				},
			}, n)
		})

		t.Run("empty manifest after line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\nmanifest {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 12},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{0, 1}},
					},*/
				},
				Statements: nil,
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span:            NodeSpan{1, 12},
						IsParenthesized: false,
						/*[]Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{1, 9}},
						},*/
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{10, 12},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
							},*/
						},
						Properties: nil,
					},
				},
			}, n)
		})

		t.Run("empty manifest after preinit", func(t *testing.T) {
			n := mustparseChunk(t, "preinit {}\nmanifest {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 22},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{10, 11}},
					},*/
				},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span:            NodeSpan{0, 10},
						IsParenthesized: false,
						/*[]Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{0, 7}},
						},*/
					},
					Block: &Block{
						NodeBase: NodeBase{
							NodeSpan{8, 10},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
							},*/
						},
					},
				},
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span:            NodeSpan{11, 22},
						IsParenthesized: false,
						/*[]Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{11, 19}},
						},*/
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{20, 22},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
							},*/
						},
						Properties: nil,
					},
				},
			}, n)
		})

		t.Run("manifest with multiline object literal", func(t *testing.T) {
			n := mustparseChunk(t, "manifest {a:1\nb:2}")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: nil,
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span:            NodeSpan{0, 18},
						IsParenthesized: false,
						/*[]Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{0, 8}},
						},*/
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{9, 18},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								{Type: NEWLINE, Span: NodeSpan{13, 14}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
							},*/
						},
						Properties: []*ObjectProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 13},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{10, 11}},
									Name:     "a",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{12, 13}},
									Value:    1,
									Raw:      "1",
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{14, 17},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{14, 15}},
									Name:     "b",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{16, 17}},
									Value:    2,
									Raw:      "2",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("includable-file", func(t *testing.T) {
			n := mustparseChunk(t, "includable-file")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: nil,
				IncludableChunkDesc: &IncludableChunkDescription{
					NodeBase: NodeBase{
						Span:            NodeSpan{0, 15},
						IsParenthesized: false,
					},
				},
			}, n)
		})

		t.Run("includable-file after line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\nincludable-file")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 16},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{0, 1}},
					},*/
				},
				Statements: nil,
				IncludableChunkDesc: &IncludableChunkDescription{
					NodeBase: NodeBase{
						Span:            NodeSpan{1, 16},
						IsParenthesized: false,
					},
				},
			}, n)
		})
	})

	t.Run("top level constant declarations", func(t *testing.T) {
		t.Run("empty const declarations", func(t *testing.T) {
			n := mustparseChunk(t, "const ()")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 8},
						nil,
						false,
						/*[]Token{
							{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}},
							{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
							{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
						},*/
					},
					Declarations: nil,
				},
			}, n)
		})

		t.Run("single declaration with parenthesis", func(t *testing.T) {
			n := mustparseChunk(t, "const ( a = 1 )")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 15},
						nil,
						false,
						/*[]Token{
							{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}},
							{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
							{Type: CLOSING_PARENTHESIS, Span: NodeSpan{14, 15}},
						},*/
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{8, 13},
								nil,
								false,
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
								Name:     "a",
							},
							Right: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
								Raw:      "1",
								Value:    1,
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis", func(t *testing.T) {
			n := mustparseChunk(t, "const a = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 11},
						nil,
						false,
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{6, 11},
								nil,
								false,
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Name:     "a",
							},
							Right: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
								Raw:      "1",
								Value:    1,
							},
						},
					},
				},
			}, n)
		})

		t.Run("variable identifiers should not be keywords", func(t *testing.T) {
			n, err := parseChunk(t, "const manifest = 1", "")
			assert.NotNil(t, n)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("const keyword followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "const", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 5},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
						false,
					},
					Declarations: nil,
				},
			}, n)
		})

		t.Run("const keyword followed by space + EOF", func(t *testing.T) {
			n, err := parseChunk(t, "const ", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 6},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
						false,
					},
					Declarations: nil,
				},
			}, n)
		})

		t.Run("const keyword followed by a literal", func(t *testing.T) {
			n, err := parseChunk(t, "const 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 7},
						nil,
						false,
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{6, 7},
								&ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_MISSING_EQL_SIGN},
								false,
							},
							Left: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Raw:      "1",
								Value:    1,
							},
						},
					},
				},
			}, n)
		})

		t.Run("const keyword followed by a literal + equal sign", func(t *testing.T) {
			n, err := parseChunk(t, "const 1 =", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 9},
						nil,
						false,
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{6, 9},
								&ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_LHS_MUST_BE_AN_IDENT},
								false,
							},
							Left: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Right: &MissingExpression{
								NodeBase: NodeBase{
									NodeSpan{8, 9},
									&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("const 1 ="), 9, true)},
									false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("const keyword followed by linefeed + manifest", func(t *testing.T) {
			n, err := parseChunk(t, "const\nmanifest {}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 17},
					nil,
					false,
				},
				Statements: nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 5},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
						false,
					},
					Declarations: nil,
				},
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span:            NodeSpan{6, 17},
						IsParenthesized: false,
						/*[]Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{6, 14}},
						},*/
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{15, 17},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
							},*/
						},
						Properties: nil,
					},
				},
			}, n)
		})
	})

	t.Run("top level local variables declarations", func(t *testing.T) {

		t.Run("empty declarations", func(t *testing.T) {
			n := mustparseChunk(t, "var ()")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
						},
						Declarations: nil,
					},
				},
			}, n)
		})

		t.Run("single declaration", func(t *testing.T) {
			n := mustparseChunk(t, "var ( a = 1 )")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{6, 11},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Name:     "a",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis", func(t *testing.T) {
			n := mustparseChunk(t, "var a = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 9},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with percent-prefixed type", func(t *testing.T) {
			n := mustparseChunk(t, "var a %int = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 14},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 10}, nil, false},
									Name:     "int",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed named pattern", func(t *testing.T) {
			n := mustparseChunk(t, "var a int = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 13},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 9}, nil, false},
									Unprefixed: true,
									Name:       "int",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern namespace member", func(t *testing.T) {
			n := mustparseChunk(t, "var a x.y = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 13},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Type: &PatternNamespaceMemberExpression{
									NodeBase: NodeBase{NodeSpan{6, 9}, nil, false},
									Namespace: &PatternNamespaceIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{6, 8}, nil, false},
										Unprefixed: true,
										Name:       "x",
									},
									MemberName: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
										Name:     "y",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern call", func(t *testing.T) {
			n := mustparseChunk(t, "var a int() = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 15},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{
										Span:            NodeSpan{6, 11},
										IsParenthesized: false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
										},*/
									},
									Callee: &PatternIdentifierLiteral{
										NodeBase: NodeBase{
											Span: NodeSpan{6, 9},
										},
										Unprefixed: true,
										Name:       "int",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis with an optional pattern expression as type", func(t *testing.T) {
			n := mustparseChunk(t, "var a int? = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 14},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Type: &OptionalPatternExpression{
									NodeBase: NodeBase{
										Span: NodeSpan{6, 10},
									},
									Pattern: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{6, 9}, nil, false},
										Unprefixed: true,
										Name:       "int",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern call (namespace member)", func(t *testing.T) {
			n := mustparseChunk(t, "var a a.b() = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 15},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{
										Span:            NodeSpan{6, 11},
										IsParenthesized: false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
										},*/
									},
									Callee: &PatternNamespaceMemberExpression{
										NodeBase: NodeBase{NodeSpan{6, 9}, nil, false},
										Namespace: &PatternNamespaceIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{6, 8}, nil, false},
											Unprefixed: true,
											Name:       "a",
										},
										MemberName: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
											Name:     "b",
										},
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern call (object pattern argument shorthand)", func(t *testing.T) {
			n := mustparseChunk(t, "var a int{} = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 15},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{Span: NodeSpan{6, 11}},
									Callee: &PatternIdentifierLiteral{
										NodeBase: NodeBase{
											Span: NodeSpan{6, 9},
										},
										Unprefixed: true,
										Name:       "int",
									},
									Arguments: []Node{
										&ObjectPatternLiteral{
											NodeBase: NodeBase{
												Span:            NodeSpan{9, 11},
												IsParenthesized: false,
												/*[]Token{
													{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
												},*/
											},
										},
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)

			t.Run("single declaration without parenthesis and with unprefixed pattern call (namespace member, object pattern argument shorthand))", func(t *testing.T) {
				n := mustparseChunk(t, "var a a.b{} = 1")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
					Statements: []Node{
						&LocalVariableDeclarations{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								nil,
								false,
							},
							Declarations: []*LocalVariableDeclaration{
								{
									NodeBase: NodeBase{
										NodeSpan{4, 15},
										nil,
										false,
									},
									Left: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
										Name:     "a",
									},
									Type: &PatternCallExpression{
										NodeBase: NodeBase{Span: NodeSpan{6, 11}},
										Callee: &PatternNamespaceMemberExpression{
											NodeBase: NodeBase{NodeSpan{6, 9}, nil, false},
											Namespace: &PatternNamespaceIdentifierLiteral{
												NodeBase:   NodeBase{NodeSpan{6, 8}, nil, false},
												Unprefixed: true,
												Name:       "a",
											},
											MemberName: &IdentifierLiteral{
												NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
												Name:     "b",
											},
										},
										Arguments: []Node{
											&ObjectPatternLiteral{
												NodeBase: NodeBase{
													Span:            NodeSpan{9, 11},
													IsParenthesized: false,
													/*[]Token{
														{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
														{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
													},*/
												},
											},
										},
									},
									Right: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				}, n)
			})
		})

		t.Run("var keyword at end of file", func(t *testing.T) {
			n, err := parseChunk(t, "var", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("var keyword followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "var\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 4},
					nil,
					false,
				},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("var keyword followed by line feed + expression", func(t *testing.T) {
			n, err := parseChunk(t, "var\n1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 5}},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
							false,
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("single declaration with invalid LHS", func(t *testing.T) {
			n, err := parseChunk(t, "var 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 5}},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{4, 5},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration with invalid LHS followed by a space", func(t *testing.T) {
			n, err := parseChunk(t, "var 1 ", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 6},
					nil,
					false,
				},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{4, 6},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration with invalid LHS followed by a linefeed and an expression", func(t *testing.T) {
			n, err := parseChunk(t, "var 1\n1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 7},
					nil,
					false,
				},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
							false,
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{4, 6},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("single declaration with keyword LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "var manifest", "")
			assert.NotNil(t, mod)
			assert.Error(t, err)
		})

		t.Run("single parenthesized declaration with invalid LHS and valid RHS", func(t *testing.T) {
			n, err := parseChunk(t, "var (1 = 2)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 11}},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{Span: NodeSpan{0, 11}},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{5, 10},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{5, 6}},
									Raw:      "1",
									Value:    1,
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{9, 10}},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single non-parenthesized declaration with invalid LHS and valid RHS", func(t *testing.T) {
			n, err := parseChunk(t, "var 1 = 2", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 9}},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 9},
							Err:  &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{4, 9},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_LOCAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{4, 5}},
									Raw:      "1",
									Value:    1,
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{8, 9}},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration with keyword LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "var manifest = 1", "")
			assert.NotNil(t, mod)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("single declaration with unexpected char as LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "var ? = 1", "")
			assert.NotNil(t, mod)
			assert.Error(t, err)
		})

		t.Run("miscellaneous", func(t *testing.T) {
			_, err := parseChunk(t, "var a #{} = 1", "")
			assert.NoError(t, err)
		})
	})

	t.Run("top level global variables declarations", func(t *testing.T) {

		t.Run("empty declarations", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar ()")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
						},
						Declarations: nil,
					},
				},
			}, n)
		})

		t.Run("single declaration", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar ( a = 1 )")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{12, 17},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
									Name:     "a",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 15},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with percent-prefixed type", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a %int = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 20},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 16}, nil, false},
									Name:     "int",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed named pattern", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a int = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 19},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{12, 15}, nil, false},
									Unprefixed: true,
									Name:       "int",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern namespace member", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a x.y = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 19},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Type: &PatternNamespaceMemberExpression{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Namespace: &PatternNamespaceIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{12, 14}, nil, false},
										Unprefixed: true,
										Name:       "x",
									},
									MemberName: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
										Name:     "y",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern call", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a int() = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 21},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{
										Span:            NodeSpan{12, 17},
										IsParenthesized: false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
										},*/
									},
									Callee: &PatternIdentifierLiteral{
										NodeBase: NodeBase{
											Span: NodeSpan{12, 15},
										},
										Unprefixed: true,
										Name:       "int",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis with an optional pattern expression as type", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a int? = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 20},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Type: &OptionalPatternExpression{
									NodeBase: NodeBase{
										Span: NodeSpan{12, 16},
									},
									Pattern: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{12, 15}, nil, false},
										Unprefixed: true,
										Name:       "int",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern call (namespace member)", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a a.b() = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 21},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{
										Span:            NodeSpan{12, 17},
										IsParenthesized: false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
										},*/
									},
									Callee: &PatternNamespaceMemberExpression{
										NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
										Namespace: &PatternNamespaceIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{12, 14}, nil, false},
											Unprefixed: true,
											Name:       "a",
										},
										MemberName: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
											Name:     "b",
										},
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration without parenthesis and with unprefixed pattern call (object pattern argument shorthand)", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar a int{} = 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 21},
									nil,
									false,
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{Span: NodeSpan{12, 17}},
									Callee: &PatternIdentifierLiteral{
										NodeBase: NodeBase{
											Span: NodeSpan{12, 15},
										},
										Unprefixed: true,
										Name:       "int",
									},
									Arguments: []Node{
										&ObjectPatternLiteral{
											NodeBase: NodeBase{
												Span:            NodeSpan{15, 17},
												IsParenthesized: false,
												/*[]Token{
													{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
												},*/
											},
										},
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)

			t.Run("single declaration without parenthesis and with unprefixed pattern call (namespace member, object pattern argument shorthand))", func(t *testing.T) {
				n := mustparseChunk(t, "globalvar a a.b{} = 1")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
					Statements: []Node{
						&GlobalVariableDeclarations{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								false,
							},
							Declarations: []*GlobalVariableDeclaration{
								{
									NodeBase: NodeBase{
										NodeSpan{10, 21},
										nil,
										false,
									},
									Left: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
										Name:     "a",
									},
									Type: &PatternCallExpression{
										NodeBase: NodeBase{Span: NodeSpan{12, 17}},
										Callee: &PatternNamespaceMemberExpression{
											NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
											Namespace: &PatternNamespaceIdentifierLiteral{
												NodeBase:   NodeBase{NodeSpan{12, 14}, nil, false},
												Unprefixed: true,
												Name:       "a",
											},
											MemberName: &IdentifierLiteral{
												NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
												Name:     "b",
											},
										},
										Arguments: []Node{
											&ObjectPatternLiteral{
												NodeBase: NodeBase{
													Span:            NodeSpan{15, 17},
													IsParenthesized: false,
													/*[]Token{
														{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
														{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
													},*/
												},
											},
										},
									},
									Right: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				}, n)
			})
		})

		t.Run("globalvar keyword at end of file", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_VAR_DECLS},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("globalvar keyword followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 10},
					nil,
					false,
				},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_VAR_DECLS},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("globalvar keyword followed by line feed + expression", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar\n1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 11},
					nil,
					false,
				},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_VAR_DECLS},
							false,
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("single declaration with invalid LHS", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 11}},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{10, 11},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration with invalid LHS followed by a space", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar 1 ", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 12}},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{10, 12},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration with invalid LHS followed by a space", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar 1\n1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 13}},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
							false,
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{10, 12},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("single declaration with keyword LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "globalvar manifest", "")
			assert.NotNil(t, mod)
			assert.Error(t, err)
		})

		t.Run("single parenthesized declaration with invalid LHS and valid RHS", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar (1 = 2)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 17}},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{Span: NodeSpan{0, 17}},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{11, 16},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{11, 12}},
									Raw:      "1",
									Value:    1,
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{15, 16}},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single non-parenthesized declaration with invalid LHS and valid RHS", func(t *testing.T) {
			n, err := parseChunk(t, "globalvar 1 = 2", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{Span: NodeSpan{0, 15}},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 15},
							Err:  &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECLS_OPENING_PAREN_EXPECTED},
						},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{
									Span: NodeSpan{10, 15},
									Err:  &ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_VAR_DECL_LHS_MUST_BE_AN_IDENT},
								},
								Left: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{10, 11}},
									Raw:      "1",
									Value:    1,
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{Span: NodeSpan{14, 15}},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single declaration with keyword LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "globalvar manifest = 1", "")
			assert.NotNil(t, mod)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("single declaration with unexpected char as LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "globalvar ? = 1", "")
			assert.NotNil(t, mod)
			assert.Error(t, err)
		})

		t.Run("miscellaneous", func(t *testing.T) {
			_, err := parseChunk(t, "globalvar a #{} = 1", "")
			assert.NoError(t, err)
		})
	})

	t.Run("variable", func(t *testing.T) {
		n := mustparseChunk(t, "$a")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
			Statements: []Node{
				&Variable{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Name:     "a",
				},
			},
		}, n)
	})

	t.Run("identifier", func(t *testing.T) {

		t.Run("single letter", func(t *testing.T) {
			n := mustparseChunk(t, "a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Name:     "a",
					},
				},
			}, n)
		})

		t.Run("ending with a hyphen", func(t *testing.T) {
			n, err := parseChunk(t, "a-", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, IDENTIFIER_LITERAL_MUST_NO_END_WITH_A_HYPHEN},
							false,
						},
						Name: "a-",
					},
				},
			}, n)
		})

		t.Run("followed by line feed", func(t *testing.T) {
			n := mustparseChunk(t, "a\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					false,
				},
				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Name:     "a",
					},
				},
			}, n)
		})
	})

	t.Run("boolean literals", func(t *testing.T) {
		t.Run("true", func(t *testing.T) {
			n := mustparseChunk(t, "true")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&BooleanLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Value:    true,
					},
				},
			}, n)
		})

		t.Run("false", func(t *testing.T) {
			n := mustparseChunk(t, "false")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&BooleanLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Value:    false,
					},
				},
			}, n)
		})

	})

	t.Run("property name", func(t *testing.T) {
		n := mustparseChunk(t, ".a")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
			Statements: []Node{
				&PropertyNameLiteral{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Name:     "a",
				},
			},
		}, n)
	})

	t.Run("long value path literal", func(t *testing.T) {
		t.Run("2 property names", func(t *testing.T) {
			n := mustparseChunk(t, ".a.b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&LongValuePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Segments: []SimpleValueLiteral{
							&PropertyNameLiteral{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							&PropertyNameLiteral{
								NodeBase: NodeBase{NodeSpan{2, 4}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("3 property names", func(t *testing.T) {
			n := mustparseChunk(t, ".a.b.c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&LongValuePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Segments: []SimpleValueLiteral{
							&PropertyNameLiteral{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							&PropertyNameLiteral{
								NodeBase: NodeBase{NodeSpan{2, 4}, nil, false},
								Name:     "b",
							},
							&PropertyNameLiteral{
								NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
								Name:     "c",
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated", func(t *testing.T) {
			n, err := parseChunk(t, ".a.", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&LongValuePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
						Segments: []SimpleValueLiteral{
							&PropertyNameLiteral{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							&PropertyNameLiteral{
								NodeBase: NodeBase{
									NodeSpan{2, 3},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_VALUE_PATH_LITERAL},
									false,
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("flag literal", func(t *testing.T) {
		t.Run("single hyphen followed by a single letter", func(t *testing.T) {
			n := mustparseChunk(t, "-a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&FlagLiteral{
						NodeBase:   NodeBase{NodeSpan{0, 2}, nil, false},
						Name:       "a",
						SingleDash: true,
						Raw:        "-a",
					},
				},
			}, n)
		})

		t.Run("single hyphen followed by several letters", func(t *testing.T) {
			n := mustparseChunk(t, "-ab")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&FlagLiteral{
						NodeBase:   NodeBase{NodeSpan{0, 3}, nil, false},
						Name:       "ab",
						SingleDash: true,
						Raw:        "-ab",
					},
				},
			}, n)
		})

		t.Run("single hyphen followed by an unexpected character", func(t *testing.T) {
			n, err := parseChunk(t, "-?", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&FlagLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, OPTION_NAME_CAN_ONLY_CONTAIN_ALPHANUM_CHARS},
							false,
						},
						Name:       "",
						SingleDash: true,
						Raw:        "-",
					},
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{1, 2},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule('?')},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("flag literal : double dash", func(t *testing.T) {
			n := mustparseChunk(t, "--abc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&FlagLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Name:     "abc",
						Raw:      "--abc",
					},
				},
			}, n)
		})
	})

	t.Run("option expression", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, `--name="foo"`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&OptionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
						},
						Name: "name",
						Value: &QuotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{7, 12}, nil, false},
							Raw:      `"foo"`,
							Value:    "foo",
						},
						SingleDash: false,
					},
				},
			}, n)
		})

		t.Run("unterminated", func(t *testing.T) {
			n, err := parseChunk(t, `--name=`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&OptionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnspecifiedParsingError, "unterminated option expression, '=' should be followed by an expression"},
							false,
						},
						Name:       "name",
						SingleDash: false,
					},
				},
			}, n)
		})

	})

	t.Run("option patterns", func(t *testing.T) {
		t.Run("missing '='", func(t *testing.T) {
			n, err := parseChunk(t, `%--name`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&OptionPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPION_PATTERN_A_VALUE_IS_EXPECTED_AFTER_EQUAKL_SIGN},
							false,
						},
						Name:       "name",
						SingleDash: false,
					},
				},
			}, n)
		})

		t.Run("missing value after '='", func(t *testing.T) {
			n, err := parseChunk(t, `%--name=`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&OptionPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPION_PATT_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR},
							false,
						},
						Name:       "name",
						SingleDash: false,
					},
				},
			}, n)
		})

		t.Run("valid option pattern", func(t *testing.T) {
			n := mustparseChunk(t, `%--name=%foo`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&OptionPatternLiteral{
						NodeBase:   NodeBase{NodeSpan{0, 12}, nil, false},
						Name:       "name",
						SingleDash: false,
						Value: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 12}, nil, false},
							Name:     "foo",
						},
					},
				},
			}, n)
		})

		t.Run("unprefixed", func(t *testing.T) {
			n := mustparseChunk(t, `pattern p = --name=int`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 22},
							IsParenthesized: false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{8, 9}},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &OptionPatternLiteral{
							NodeBase:   NodeBase{NodeSpan{12, 22}, nil, false},
							Name:       "name",
							SingleDash: false,
							Unprefixed: true,
							Value: &PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{19, 22}, nil, false},
								Name:       "int",
								Unprefixed: true,
							},
						},
					},
				},
			}, n)
		})
	})
	t.Run("path literal", func(t *testing.T) {

		t.Run("unquoted absolute path literal : /", func(t *testing.T) {
			n := mustparseChunk(t, "/")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Raw:      "/",
						Value:    "/",
					},
				},
			}, n)
		})

		t.Run("quoted absolute path literal : /`[]`", func(t *testing.T) {
			n := mustparseChunk(t, "/`[]`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Raw:      "/`[]`",
						Value:    "/[]",
					},
				},
			}, n)
		})

		t.Run("unquoted absolute path literal : /a", func(t *testing.T) {
			n := mustparseChunk(t, "/a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Raw:      "/a",
						Value:    "/a",
					},
				},
			}, n)
		})

		t.Run("relative path literal : ./", func(t *testing.T) {
			n := mustparseChunk(t, "./")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&RelativePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Raw:      "./",
						Value:    "./",
					},
				},
			}, n)
		})

		t.Run("relative path literal : ./a", func(t *testing.T) {
			n := mustparseChunk(t, "./a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&RelativePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
						Raw:      "./a",
						Value:    "./a",
					},
				},
			}, n)
		})

		t.Run("relative path literal in list : [./]", func(t *testing.T) {
			n := mustparseChunk(t, "[./]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&ListLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
							},*/
						},
						Elements: []Node{
							&RelativePathLiteral{
								NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
								Raw:      "./",
								Value:    "./",
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated quoted path literal: missing closing backtick + followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "/`[]", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_PATH_LIT_MISSING_CLOSING_BACTICK},
							false,
						},
						Raw:   "/`[]",
						Value: "/[]",
					},
				},
			}, n)
		})

		t.Run("unterminated quoted path literal: missing closing backtick + followed by linefeed", func(t *testing.T) {
			n, err := parseChunk(t, "/`[]\na", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_PATH_LIT_MISSING_CLOSING_BACTICK},
							false,
						},
						Raw:   "/`[]",
						Value: "/[]",
					},
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
						Name:     "a",
					},
				},
			}, n)
		})

		t.Run("non-trailing colon", func(t *testing.T) {
			n := mustparseChunk(t, "/a:b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Raw:      "/a:b",
						Value:    "/a:b",
					},
				},
			}, n)
		})

	})

	t.Run("path pattern", func(t *testing.T) {
		t.Run("absolute path pattern literal : /a*", func(t *testing.T) {
			n := mustparseChunk(t, "%/a*")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Raw:      "%/a*",
						Value:    "/a*",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal : /a[a-z]", func(t *testing.T) {
			n := mustparseChunk(t, "%/`a[a-z]`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
						Raw:      "%/`a[a-z]`",
						Value:    "/a[a-z]",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal ending with /... ", func(t *testing.T) {
			n := mustparseChunk(t, "%/a/...")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Raw:      "%/a/...",
						Value:    "/a/...",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal : /... ", func(t *testing.T) {
			n := mustparseChunk(t, "%/...")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Raw:      "%/...",
						Value:    "/...",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal with /... in the middle ", func(t *testing.T) {
			n, err := parseChunk(t, "%/a/.../b", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, fmtSlashDotDotDotCanOnlyBePresentAtEndOfPathPattern("/a/.../b")},
							false,
						},
						Raw:   "%/a/.../b",
						Value: "/a/.../b",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal with /... in the middle and at the end", func(t *testing.T) {
			n, err := parseChunk(t, "%/a/.../...", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnspecifiedParsingError, fmtSlashDotDotDotCanOnlyBePresentAtEndOfPathPattern("/a/.../...")},
							false,
						},
						Raw:   "%/a/.../...",
						Value: "/a/.../...",
					},
				},
			}, n)
		})

		t.Run("unterminated quoted path pattern literal: missing closing backtick + followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "%/`[a-z]", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_PATH_PATTERN_LIT_MISSING_CLOSING_BACTICK},
							false,
						},
						Raw:   "%/`[a-z]",
						Value: "/[a-z]",
					},
				},
			}, n)
		})

		t.Run("unterminated quoted path pattern literal: missing closing backtick + followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "%/`[a-z]\na", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_PATH_PATTERN_LIT_MISSING_CLOSING_BACTICK},
							false,
						},
						Raw:   "%/`[a-z]",
						Value: "/[a-z]",
					},
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
						Name:     "a",
					},
				},
			}, n)
		})

		t.Run("non-trailing colon", func(t *testing.T) {
			n := mustparseChunk(t, "%/a:b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Raw:      "%/a:b",
						Value:    "/a:b",
					},
				},
			}, n)
		})
	})

	t.Run("named-segment path pattern literal  ", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, "%/home/{:username}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&NamedSegmentPathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/* []Token{
								{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{7, 8}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{17, 18}},
							}, */
						},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 7}, nil, false},
								Value:    "/home/",
							},
							&NamedPathSegment{
								NodeBase: NodeBase{NodeSpan{8, 17}, nil, false},
								Name:     "username",
							},
						},
						Raw:         "%/home/{:username}",
						StringValue: "%/home/{:username}",
					},
				},
			}, n)
		})

		t.Run("quoting is not suppported yet", func(t *testing.T) {
			n, err := parseChunk(t, "%/`home/{:username}`", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&NamedSegmentPathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							&ParsingError{UnspecifiedParsingError, QUOTED_NAMED_SEGMENT_PATH_PATTERNS_ARE_NOT_SUPPORTED_YET},
							false,
						},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 8}, nil, false},
								Value:    "/`home/",
							},
							&NamedPathSegment{
								NodeBase: NodeBase{NodeSpan{9, 18}, nil, false},
								Name:     "username",
							},
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "`",
							},
						},
						Raw:         "",
						StringValue: "",
					},
				},
			}, n)
		})

		//TODO: improve following tests

		t.Run("invalid named-segment path pattern literals", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:}")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:u:}")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:username-}")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:-username}")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:username}")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/{:username}e")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:username}e")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:username}e/{$a}/")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:username}e/{}")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/e{:username}e/{}/")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/{")
			})
			assert.Panics(t, func() {
				mustparseChunk(t, "%/home/{:")
			})
		})
	})

	t.Run("path pattern expression", func(t *testing.T) {
		t.Run("trailing interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "%/home/{$username}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&PathPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{7, 8}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{17, 18}},
							},*/
						},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 7}, nil, false},
								Value:    "/home/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{8, 17}, nil, false},
								Name:     "username",
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty trailing interpolation", func(t *testing.T) {
			n, err := parseChunk(t, "%/home/{}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&PathPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{7, 8}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{8, 9}},
							},*/
						},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 7}, nil, false},
								Value:    "/home/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{8, 8},
									&ParsingError{UnspecifiedParsingError, EMPTY_PATH_INTERP},
									false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("named segments are not allowed", func(t *testing.T) {
			n, err := parseChunk(t, "%/`home/{$username}`", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&PathPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							&ParsingError{UnspecifiedParsingError, QUOTED_PATH_PATTERN_EXPRS_ARE_NOT_SUPPORTED_YET},
							false,
						},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 8}, nil, false},
								Value:    "/`home/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{9, 18}, nil, false},
								Name:     "username",
							},
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "`",
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("path expression", func(t *testing.T) {
		t.Run("single trailing interpolation (variable)", func(t *testing.T) {
			n := mustparseChunk(t, "/home/{$username}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{16, 17}},
							},*/
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
								Value:    "/home/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
								Name:     "username",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single embedded interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "/home/{$username}/projects")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{16, 17}},
							},*/
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
								Value:    "/home/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
								Name:     "username",
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{17, 26}, nil, false},
								Value:    "/projects",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single trailing interpolation (identifier)", func(t *testing.T) {
			n := mustparseChunk(t, "/home/{username}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{15, 16}},
							},*/
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
								Value:    "/home/",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 15}, nil, false},
								Name:     "username",
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated interpolation: code ends after '{'", func(t *testing.T) {
			n, err := parseChunk(t, "/home/{", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
							},*/
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
								Value:    "/home/",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{7, 7},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
									false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated interpolation: linefeed after '{'", func(t *testing.T) {
			n, err := parseChunk(t, "/home/{\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
							},*/
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
								Value:    "/home/",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{7, 7},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
									false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("named segments are not allowed", func(t *testing.T) {
			n, err := parseChunk(t, "/home/{:username}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							&ParsingError{UnspecifiedParsingError, ONLY_PATH_PATTERNS_CAN_CONTAIN_NAMED_SEGMENTS},
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{16, 17}},
							},*/
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
								Value:    "/home/",
							},
							&NamedPathSegment{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
								Name:     "username",
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("regex literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, "%``")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&RegularExpressionLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
						Value:    "",
						Raw:      "%``",
					},
				},
			}, n)
		})

		t.Run("not empty", func(t *testing.T) {
			n := mustparseChunk(t, "%`a+`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&RegularExpressionLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Value:    "a+",
						Raw:      "%`a+`",
					},
				},
			}, n)
		})

		t.Run("unterminated", func(t *testing.T) {
			n, err := parseChunk(t, "%`", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&RegularExpressionLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_REGEX_LIT},
							false,
						},
						Value: "",
						Raw:   "%`",
					},
				},
			}, n)
		})
	})

	t.Run("nil literal", func(t *testing.T) {
		n := mustparseChunk(t, "nil")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
			Statements: []Node{
				&NilLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				},
			},
		}, n)
	})

	t.Run("self expression", func(t *testing.T) {
		n := mustparseChunk(t, "self")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
			Statements: []Node{
				&SelfExpression{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				},
			},
		}, n)
	})

	t.Run("member expression", func(t *testing.T) {
		t.Run("variable '.' <single letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 5}, nil, false},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run(" variable '.' <propname> '.' <single-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b.c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.?' <name>", func(t *testing.T) {
			n := mustparseChunk(t, "$a.?b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Name:     "b",
						},
						Optional: true,
					},
				},
			}, n)
		})

		t.Run("variable '.?'", func(t *testing.T) {
			n, err := parseChunk(t, "$a.?", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Optional: true,
					},
				},
			}, n)
		})

		t.Run("variable '.' <prop name> '.' <two-letter prop name> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b.cd")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Name:     "cd",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.?' <prop> '.' <prop name> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.?b.c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Name:     "b",
							},
							Optional: true,
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("missing property name: followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "$a.", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: nil,
					},
				},
			}, n)
		})

		t.Run("missing property name: followed by identifier on next line", func(t *testing.T) {
			n, err := parseChunk(t, "$a.\nb", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 5},
					nil,
					false,
				},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: nil,
					},
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
						Name:     "b",
					},
				},
			}, n)
		})

		t.Run("missing property name: followed by closing delim", func(t *testing.T) {
			n, err := parseChunk(t, "$a.]", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: nil,
					},
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{3, 4},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("long member expression : unterminated", func(t *testing.T) {
			n, err := parseChunk(t, "$a.b.", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: nil,
					},
				},
			}, n)
		})

		t.Run("self '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "(self.bc)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 8},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Left: &SelfExpression{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("call '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "a().bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Left: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{2, 3}},
								},*/
							},
							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("member of a parenthesized expression", func(t *testing.T) {
			n := mustparseChunk(t, "($a).name")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{
								NodeSpan{1, 3},
								nil,
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},*/
							},
							Name: "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
							Name:     "name",
						},
					},
				},
			}, n)
		})

		t.Run("optional member of an identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.b.?c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Left: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "c",
						},
						Optional: true,
					},
				},
			}, n)
		})

		t.Run("double-colon expression", func(t *testing.T) {
			n := mustparseChunk(t, "a::b.c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Left: &DoubleColonExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								false,
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							Element: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

	})

	t.Run("computed member expression", func(t *testing.T) {
		t.Run("variable '.' '(' <var> ')'", func(t *testing.T) {
			n := mustparseChunk(t, "$a.(b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ComputedMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{
								NodeSpan{4, 5},
								nil,
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
								},*/
							},
							Name: "b",
						},
					},
				},
			}, n)
		})

		t.Run("identifier '.' '(' <var> ')'", func(t *testing.T) {
			n := mustparseChunk(t, "a.(b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&ComputedMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{
								NodeSpan{3, 4},
								nil,
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
								},*/
							},
							Name: "b",
						},
					},
				},
			}, n)
		})

		t.Run(" variable '.' '(' <var> ')' '.'  '(' <var> ')' ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.(b).(c)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&ComputedMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
						Left: &ComputedMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{
									NodeSpan{4, 5},
									nil,
									true,
									/*[]Token{
										{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
										{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{
								NodeSpan{8, 9},
								nil,
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{7, 8}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
								},*/
							},
							Name: "c",
						},
					},
				},
			}, n)
		})

		//TODO: add tests
	})

	t.Run("dynamic member expression", func(t *testing.T) {

		t.Run("identifier '.<' <single letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "a.<b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.<' <single letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.<b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.<bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run(" variable '.' <propname> '.<' <single-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b.<c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.' <propname> '.<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b.<cd")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "cd",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.<' <propname> '<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.<b.cd")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Left: &DynamicMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "cd",
						},
					},
				},
			}, n)
		})

		t.Run("identifier '.<' <propname> '<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "a.<b.cd")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Left: &DynamicMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Name:     "cd",
						},
					},
				},
			}, n)
		})

		t.Run("unterminated", func(t *testing.T) {
			n, err := parseChunk(t, "$a.<", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						PropertyName: nil,
					},
				},
			}, n)
		})

		t.Run("long member expression : unterminated", func(t *testing.T) {
			n, err := parseChunk(t, "$a.b.<", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR},
							false,
						},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						PropertyName: nil,
					},
				},
			}, n)
		})

		t.Run("self '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "(self.<bc)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 9},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Left: &SelfExpression{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("call '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "a().<bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Left: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{2, 3}},
								},*/
							},
							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("member of a parenthesized expression", func(t *testing.T) {
			n := mustparseChunk(t, "($a).<name")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
						Left: &Variable{
							NodeBase: NodeBase{
								NodeSpan{1, 3},
								nil,
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},*/
							},
							Name: "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 10}, nil, false},
							Name:     "name",
						},
					},
				},
			}, n)
		})

	})

	t.Run("identifier member expression", func(t *testing.T) {
		t.Run("identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "http.get")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "http",
						},
						PropertyNames: []*IdentifierLiteral{
							{
								NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
								Name:     "get",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "(http.get)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 9},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
							Name:     "http",
						},
						PropertyNames: []*IdentifierLiteral{
							{
								NodeBase: NodeBase{NodeSpan{6, 9}, nil, false},
								Name:     "get",
							},
						},
					},
				},
			}, n)
		})
		t.Run("parenthesized identifier member expression followed by a space", func(t *testing.T) {
			n := mustparseChunk(t, "(http.get) ")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 9},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
							Name:     "http",
						},
						PropertyNames: []*IdentifierLiteral{
							{
								NodeBase: NodeBase{NodeSpan{6, 9}, nil, false},
								Name:     "get",
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing last property name: followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "http.", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR},
							false,
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "http",
						},
						PropertyNames: nil,
					},
				},
			}, n)
		})

		t.Run("missing last property name, followed by an identifier on the next line", func(t *testing.T) {
			n, err := parseChunk(t, "http.\na", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 7},
					nil,
					false,
				},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR},
							false,
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "http",
						},
						PropertyNames: nil,
					},
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
						Name:     "a",
					},
				},
			}, n)
		})

		t.Run("missing last property name, followed by a closing delimiter", func(t *testing.T) {
			n, err := parseChunk(t, "http.]", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR},
							false,
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "http",
						},
						PropertyNames: nil,
					},
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{5, 6},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
							false,
						},
					},
				},
			}, n)
		})

	})

	t.Run("extraction expression", func(t *testing.T) {
		t.Run("variable", func(t *testing.T) {
			n := mustparseChunk(t, "$a.{name}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
						Object: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{2, 9},
								nil,
								false,
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
									Name:     "name",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("identifier", func(t *testing.T) {
			n := mustparseChunk(t, "a.{name}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Object: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "a",
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{1, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{1, 3}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
									Name:     "name",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.b.{name}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
						Object: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
							},
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{3, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{3, 5}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
									Name:     "name",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("computed member expression", func(t *testing.T) {
			n := mustparseChunk(t, `a.("b").{name}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
						Object: &ComputedMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyName: &QuotedStringLiteral{
								NodeBase: NodeBase{
									NodeSpan{3, 6},
									nil,
									true,
									/*[]Token{
										{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
										{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
									},*/
								},
								Raw:   `"b"`,
								Value: "b",
							},
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{7, 14},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{7, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},*/
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 13}, nil, false},
									Name:     "name",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("dynamic member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.<b.{name}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
						Object: &DynamicMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{4, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{4, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 10}, nil, false},
									Name:     "name",
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("parenthesized expression", func(t *testing.T) {
		n := mustparseChunk(t, "($a)")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
			Statements: []Node{
				&Variable{
					NodeBase: NodeBase{
						NodeSpan{1, 3},
						nil,
						true,
						/*[]Token{
							{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
							{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
						},*/
					},
					Name: "a",
				},
			},
		}, n)
	})

	t.Run("index expression", func(t *testing.T) {

		t.Run("variable '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "$a[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Indexed: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("<member expression> '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Indexed: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("<double-colon expression> '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "a::b[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Indexed: &DoubleColonExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								false,
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							Element: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("unterminated : variable '[' ", func(t *testing.T) {
			n, err := parseChunk(t, "$a[", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, "unterminated member/index expression"},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
					},
				},
			}, n)
		})

		t.Run("identifier '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "a[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Indexed: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "a",
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("short identifier member expression '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "a.b[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Indexed: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("long identifier member expression '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "a.b.c[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Indexed: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
								{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "c",
								},
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("call '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "a()[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Indexed: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{2, 3}},
								},*/
							},

							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})
	})

	t.Run("slice expression", func(t *testing.T) {
		t.Run("variable '[' <integer literal> ':' ] ", func(t *testing.T) {
			n := mustparseChunk(t, "$a[0:]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&SliceExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Indexed: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						StartIndex: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("variable '['  ':' <integer literal> ] ", func(t *testing.T) {
			n := mustparseChunk(t, "$a[:1]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&SliceExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Indexed: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						EndIndex: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("variable '[' ':' ']' : invalid ", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "$a[:]")
			})
		})

		t.Run("variable '[' ':' <integer literal> ':' ']' : invalid ", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "$a[:1:]")
			})
		})

	})

	t.Run("double-colon expression", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, "a::b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&DoubleColonExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "a",
						},
						Element: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("single element: unterminated", func(t *testing.T) {
			n, err := parseChunk(t, "a::", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&DoubleColonExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnterminatedDoubleColonExpr, UNTERMINATED_DOUBLE_COLON_EXPR},
							false,
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "a",
						},
					},
				},
			}, n)
		})

		t.Run("element: identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.b::c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&DoubleColonExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
						},
						Left: &IdentifierMemberExpression{
							NodeBase: NodeBase{Span: NodeSpan{0, 3}},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
							},
						},
						Element: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("two elements", func(t *testing.T) {
			n := mustparseChunk(t, "a::b::c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&DoubleColonExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
						},
						Left: &DoubleColonExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								false,
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							Element: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						Element: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("two elements: unterminated", func(t *testing.T) {
			n, err := parseChunk(t, "a::b::", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&DoubleColonExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnterminatedDoubleColonExpr, UNTERMINATED_DOUBLE_COLON_EXPR},
							false,
						},
						Left: &DoubleColonExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								false,
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							Element: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("key list expression", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, ".{}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							nil,
							false,
						},
						Keys: nil,
					},
				},
			}, n)
		})

		t.Run("one key", func(t *testing.T) {
			n := mustparseChunk(t, ".{name}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
						},
						Keys: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, false},
								Name:     "name",
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char", func(t *testing.T) {
			n, err := parseChunk(t, ".{:}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
						},
						Keys: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{2, 3},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInKeyList(':')},
									false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two keys separated by space", func(t *testing.T) {
			n := mustparseChunk(t, ".{name age}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
						},
						Keys: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, false},
								Name:     "name",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 10}, nil, false},
								Name:     "age",
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("url literal", func(t *testing.T) {

		t.Run("host contains a -", func(t *testing.T) {
			n := mustparseChunk(t, `https://an-example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
						Value:    "https://an-example.com/",
					},
				},
			}, n)
		})

		t.Run("long sub domain", func(t *testing.T) {
			n := mustparseChunk(t, `https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 60}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 60}, nil, false},
						Value:    "https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com/",
					},
				},
			}, n)
		})

		t.Run("long domain", func(t *testing.T) {
			n := mustparseChunk(t, `https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 52}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 52}, nil, false},
						Value:    "https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com/",
					},
				},
			}, n)
		})

		t.Run("subdomain", func(t *testing.T) {
			n := mustparseChunk(t, `https://sub.example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
						Value:    "https://sub.example.com/",
					},
				},
			}, n)
		})

		t.Run("subdomain contains -", func(t *testing.T) {
			n := mustparseChunk(t, `https://sub-x.example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
						Value:    "https://sub-x.example.com/",
					},
				},
			}, n)
		})

		t.Run("root path", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
						Value:    "https://example.com/",
					},
				},
			}, n)
		})

		t.Run("path ends with ..", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/..`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
						Value:    "https://example.com/..",
					},
				},
			}, n)
		})

		t.Run("path ends with ...", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
						Value:    "https://example.com/...",
					},
				},
			}, n)
		})

		t.Run("empty query", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
						Value:    "https://example.com/?",
					},
				},
			}, n)
		})

		t.Run("not empty query", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?a=1`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
						Value:    "https://example.com/?a=1",
					},
				},
			}, n)
		})

		t.Run("host followed by ')'", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, `https://example.com)`)
			})
		})

		t.Run("long path", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 106}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 106}, nil, false},
						Value:    "https://example.com/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
			}, n)
		})

		t.Run("non-trailing colon", func(t *testing.T) {
			n := mustparseChunk(t, "https://example.com/a:b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
						Value:    "https://example.com/a:b",
					},
				},
			}, n)
		})
	})

	t.Run("url pattern literal", func(t *testing.T) {
		t.Run("prefix pattern, root", func(t *testing.T) {
			n := mustparseChunk(t, `%https://example.com/...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
						Value:    "https://example.com/...",
						Raw:      "%https://example.com/...",
					},
				},
			}, n)
		})

		t.Run("prefix pattern", func(t *testing.T) {
			n := mustparseChunk(t, `%https://example.com/a/...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
						Value:    "https://example.com/a/...",
						Raw:      "%https://example.com/a/...",
					},
				},
			}, n)
		})

		t.Run("prefix pattern containing two dots", func(t *testing.T) {
			n := mustparseChunk(t, `%https://example.com/../...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
						Value:    "https://example.com/../...",
						Raw:      "%https://example.com/../...",
					},
				},
			}, n)
		})

		t.Run("prefix pattern containing non trailing /...", func(t *testing.T) {
			n, err := parseChunk(t, `%https://example.com/.../a`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							&ParsingError{UnspecifiedParsingError, URL_PATTERN_SUBSEQUENT_DOT_EXPLANATION},
							false,
						},
						Value: "https://example.com/.../a",
						Raw:   "%https://example.com/.../a",
					},
				},
			}, n)
		})

		t.Run("prefix pattern containing non trailing /... and trailing /...", func(t *testing.T) {
			n, err := parseChunk(t, `%https://example.com/.../...`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							&ParsingError{UnspecifiedParsingError, URL_PATTERN_SUBSEQUENT_DOT_EXPLANATION},
							false,
						},
						Value: "https://example.com/.../...",
						Raw:   "%https://example.com/.../...",
					},
				},
			}, n)
		})

		t.Run("trailing /....", func(t *testing.T) {
			n, err := parseChunk(t, `%https://example.com/....`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							&ParsingError{UnspecifiedParsingError, URL_PATTERNS_CANNOT_END_WITH_SLASH_MORE_THAN_4_DOTS},
							false,
						},
						Value: "https://example.com/....",
						Raw:   "%https://example.com/....",
					},
				},
			}, n)
		})

		t.Run("non-trailing colon", func(t *testing.T) {
			n := mustparseChunk(t, "%https://example.com/a:b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
						Raw:      "%https://example.com/a:b",
						Value:    "https://example.com/a:b",
					},
				},
			}, n)
		})
	})

	t.Run("host literal", func(t *testing.T) {

		testCases := map[string]struct {
			result *Chunk
			err    bool
		}{
			`https://example.com`: {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
					Statements: []Node{
						&HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
					},
				},
			},
			`wss://example.com`: {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
					Statements: []Node{
						&HostLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 17},
							},
							Value: "wss://example.com",
						},
					},
				},
			},
			"://example.com": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
					Statements: []Node{
						&HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
							Value:    "://example.com",
						},
					},
				},
			},
			`https://*.com`: {
				err: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
					Statements: []Node{
						&InvalidURL{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 13},
								Err:  &ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST},
							},
							Value: "https://*.com",
						},
					},
				},
			},
			`https://**`: {
				err: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&InvalidURL{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 10},
								Err:  &ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST},
							},
							Value: "https://**",
						},
					},
				},
			},
		}

		for name, testCase := range testCases {
			t.Run(name, func(t *testing.T) {
				if testCase.err {
					n, err := parseChunk(t, name, "")
					if assert.Error(t, err) {
						assert.EqualValues(t, testCase.result, n)
					}
				} else {
					n := mustparseChunk(t, name)
					assert.EqualValues(t, testCase.result, n)
				}
			})
		}
	})

	t.Run("scheme literal", func(t *testing.T) {
		t.Run("HTTP", func(t *testing.T) {
			n := mustparseChunk(t, `http://`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&SchemeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Name:     "http",
					},
				},
			}, n)
		})

		t.Run("Websocket", func(t *testing.T) {
			n := mustparseChunk(t, "wss://")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&SchemeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Name:     "wss",
					},
				},
			}, n)
		})

		t.Run("host with no scheme", func(t *testing.T) {
			n, err := parseChunk(t, `://`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&SchemeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, INVALID_SCHEME_LIT_MISSING_SCHEME},
							false,
						},
						Name: "",
					},
				},
			}, n)
		})
	})

	t.Run("host pattern", func(t *testing.T) {
		t.Run("%https://* (invalid)", func(t *testing.T) {
			n, err := parseChunk(t, `%https://*`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							&ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT_SUGGEST_DOUBLE_STAR},
							false,
						},
						Value: "https://*",
						Raw:   "%https://*",
					},
				},
			}, n)
		})

		t.Run("%https://**", func(t *testing.T) {
			n := mustparseChunk(t, `%https://**`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
						Value:    "https://**",
						Raw:      "%https://**",
					},
				},
			}, n)
		})

		t.Run("%https://*.* (invalid)", func(t *testing.T) {
			n, err := parseChunk(t, `%https://*.*`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT},
							false,
						},
						Value: "https://*.*",
						Raw:   "%https://*.*",
					},
				},
			}, n)
		})

		t.Run("%https://localhost", func(t *testing.T) {
			n := mustparseChunk(t, `%https://localhost`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{Span: NodeSpan{0, 18}},
						Value:    "https://localhost",
						Raw:      "%https://localhost",
					},
				},
			}, n)
		})
	})

	t.Run("host pattern", func(t *testing.T) {

		t.Run("HTTP host pattern : %https://**:443", func(t *testing.T) {
			n := mustparseChunk(t, `%https://**:443`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Value:    "https://**:443",
						Raw:      "%https://**:443",
					},
				},
			}, n)
		})

		t.Run("HTTP host pattern : %https://*.<tld>", func(t *testing.T) {
			n := mustparseChunk(t, `%https://*.com`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
						Value:    "https://*.com",
						Raw:      "%https://*.com",
					},
				},
			}, n)
		})

		t.Run("HTTP host pattern : %https://a*.<tld>", func(t *testing.T) {
			n := mustparseChunk(t, `%https://a*.com`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Value:    "https://a*.com",
						Raw:      "%https://a*.com",
					},
				},
			}, n)
		})

		// t.Run("invalid HTTP host pattern : TLD is a number", func(t *testing.T) {
		// })

		t.Run("Websocket host pattern : %wss://*", func(t *testing.T) {
			n := mustparseChunk(t, `%wss://**`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
						Value:    "wss://**",
						Raw:      "%wss://**",
					},
				},
			}, n)
		})
	})

	t.Run("url expressions", func(t *testing.T) {
		t.Run("no query, host interpolation", func(t *testing.T) {
			n := mustparseChunk(t, `https://{$host}/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{8, 9}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{14, 15}},
							},*/
						},
						Raw: "https://{$host}/",
						HostPart: &HostExpression{
							NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
							Scheme: &SchemeLiteral{
								NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
								Name:     "https",
							},
							Raw: `https://{$host}`,
							Host: &Variable{
								NodeBase: NodeBase{NodeSpan{9, 14}, nil, false},
								Name:     "host",
							},
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("whole host interpolation", func(t *testing.T) {
			n := mustparseChunk(t, `@host/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 6}},
						Raw:      "@host/",
						HostPart: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
							Name:     "host",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("whole host interpolation: uppercase", func(t *testing.T) {
			n := mustparseChunk(t, `@HOST/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 6}},
						Raw:      "@HOST/",
						HostPart: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
							Name:     "HOST",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("no query, single trailing path interpolation, no '/'", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com{$path}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{19, 20}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},*/
						},
						Raw: "https://example.com{$path}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 19}, nil, false},
								Value:    "",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{20, 25}, nil, false},
								Name:     "path",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("no query, host interpolation & path interpolation, no '/'", func(t *testing.T) {
			n := mustparseChunk(t, `https://{$host}{$path}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{8, 9}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{14, 15}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{15, 16}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{21, 22}},
							},*/
						},
						Raw: "https://{$host}{$path}",
						HostPart: &HostExpression{
							NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},

							Scheme: &SchemeLiteral{
								NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
								Name:     "https",
							},
							Raw: `https://{$host}`,
							Host: &Variable{
								NodeBase: NodeBase{NodeSpan{9, 14}, nil, false},
								Name:     "host",
							},
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{15, 15}, nil, false},
								Value:    "",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{16, 21}, nil, false},
								Name:     "path",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("trailing path interpolation after '/'", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/{$path}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 27},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},*/
						},
						Raw: "https://example.com/{$path}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, false},
								Name:     "path",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("two path interpolations", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/{$a}{$b}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{24, 25}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{27, 28}},
							},*/
						},
						Raw: "https://example.com/{$a}{$b}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{21, 23}, nil, false},
								Name:     "a",
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{24, 24}, nil, false},
								Value:    "",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{25, 27}, nil, false},
								Name:     "b",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("unterminated path interpolation: missing value after '{'", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/{`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Raw: "https://example.com/{",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{21, 21},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
									false,
								},
								Value: "",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("unterminated path interpolation: linefeed after '{'", func(t *testing.T) {
			n, err := parseChunk(t, "https://example.com/{\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Raw: "https://example.com/{",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{21, 21},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
									false,
								},
								Value: "",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("unterminated path interpolation: missing '}'", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/{1`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							false,
						},
						Raw: "https://example.com/{1",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{21, 22}, nil, false},
								Value:    1,
								Raw:      "1",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{22, 22},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP_MISSING_CLOSING_BRACE},
									false,
								},
								Value: "",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("empty path interpolation", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/{}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{21, 22}},
							},*/
						},
						Raw: "https://example.com/{}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 21},
									&ParsingError{UnspecifiedParsingError, EMPTY_PATH_INTERP},
									false,
								},
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("invalid path interpolation", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/{.}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},*/
						},
						Raw: "https://example.com/{.}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, INVALID_PATH_INTERP},
									false,
								},
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("invalid path interpolation followed by a path slice", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/{.}/`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},*/
						},
						Raw: "https://example.com/{.}/",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, INVALID_PATH_INTERP},
									false,
								},
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{23, 24}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("path interpolation with a forbidden character", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/{@}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},*/
						},
						Raw: "https://example.com/{@}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, PATH_INTERP_EXPLANATION},
									false,
								},
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("path interpolation with a forbidden character followed by a path slice", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/{@}/`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},*/
						},
						Raw: "https://example.com/{@}/",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, PATH_INTERP_EXPLANATION},
									false,
								},
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{23, 24}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{},
					},
				},
			}, n)
		})

		t.Run("trailing query interpolation", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?v={$x}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 27},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},*/
						},
						Raw: "https://example.com/?v={$x}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, false},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, false},
										Name:     "x",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("trailing query interpolation, no path", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com?v={$x}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{22, 23}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},*/
						},
						Raw: "https://example.com?v={$x}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{20, 26}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{22, 22}, nil, false},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{23, 25}, nil, false},
										Name:     "x",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("query interpolation followed by ampersand", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?v={$x}&`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},*/
						},
						Raw: "https://example.com/?v={$x}&",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, false},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, false},
										Name:     "x",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("query interpolation followed by two ampersands", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?v={$x}&&`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 29}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 29},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},*/
						},
						Raw: "https://example.com/?v={$x}&&",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, false},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, false},
										Name:     "x",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("query interpolation followed by parameter with empty name", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?v={$x}&=3`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},*/
						},
						Raw: "https://example.com/?v={$x}&=3",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, false},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, false},
										Name:     "x",
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{28, 30}, nil, false},
								Name:     "",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, false},
										Value:    "3",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two query interpolations", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?v={$x}&w={$y}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 34}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 34},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{30, 31}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{33, 34}},
							},*/
						},
						Raw: "https://example.com/?v={$x}&w={$y}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, false},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, false},
										Name:     "x",
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{28, 34}, nil, false},
								Name:     "w",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{30, 30}, nil, false},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{31, 33}, nil, false},
										Name:     "y",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated query interpolation: missing value after '{'", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/?v={`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							false,
						},
						Raw: "https://example.com/?v={",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{Span: NodeSpan{21, 24}},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{Span: NodeSpan{23, 23}},
										Value:    "",
									},
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{
											NodeSpan{24, 24},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUERY_PARAM_INTERP},
											false,
										},
										Value: "",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated query interpolation: missing '}'", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/?v={1`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
						},
						Raw: "https://example.com/?v={1",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{Span: NodeSpan{21, 25}},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{Span: NodeSpan{23, 23}},
										Value:    "",
									},
									&IntLiteral{
										NodeBase: NodeBase{NodeSpan{24, 25}, nil, false},
										Value:    1,
										Raw:      "1",
									},
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{
											NodeSpan{25, 25},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUERY_PARAM_INTERP_MISSING_CLOSING_BRACE},
											false,
										},
										Value: "",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty query interpolation", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/?v={}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{24, 25}},
							},*/
						},
						Raw: "https://example.com/?v={}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{Span: NodeSpan{21, 25}},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{Span: NodeSpan{23, 23}},
										Value:    "",
									},
									&UnknownNode{
										NodeBase: NodeBase{
											NodeSpan{24, 24},
											&ParsingError{UnspecifiedParsingError, EMPTY_QUERY_PARAM_INTERP},
											false,
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("invalid query interpolation", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/?v={:}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},*/
						},
						Raw: "https://example.com/?v={:}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{Span: NodeSpan{23, 23}},
										Value:    "",
									},
									&UnknownNode{
										NodeBase: NodeBase{
											NodeSpan{24, 25},
											&ParsingError{UnspecifiedParsingError, INVALID_QUERY_PARAM_INTERP},
											false,
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("invalid query interpolation followed by a query parameter", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/?v={:}&w=3`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},*/
						},
						Raw: "https://example.com/?v={:}&w=3",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{Span: NodeSpan{23, 23}},
										Value:    "",
									},
									&UnknownNode{
										NodeBase: NodeBase{
											NodeSpan{24, 25},
											&ParsingError{UnspecifiedParsingError, INVALID_QUERY_PARAM_INTERP},
											false,
										},
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{27, 30}, nil, false},
								Name:     "w",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, false},
										Value:    "3",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("query interpolation with a forbidden character", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/?v={?}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},*/
						},
						Raw: "https://example.com/?v={?}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{Span: NodeSpan{23, 23}},
										Value:    "",
									},
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{
											NodeSpan{24, 25},
											&ParsingError{UnspecifiedParsingError, QUERY_PARAM_INTERP_EXPLANATION},
											false,
										},
										Value: "?",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("query interpolation with a forbidden character followed by a query parameter", func(t *testing.T) {
			n, err := parseChunk(t, `https://example.com/?v={?}&w=3`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, false},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							false,
							/*[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},*/
						},
						Raw: "https://example.com/?v={?}&w=3",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, false},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{Span: NodeSpan{23, 23}},
										Value:    "",
									},
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{
											NodeSpan{24, 25},
											&ParsingError{UnspecifiedParsingError, QUERY_PARAM_INTERP_EXPLANATION},
											false,
										},
										Value: "?",
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{27, 30}, nil, false},
								Name:     "w",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, false},
										Value:    "3",
									},
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("invalid host alias stuff", func(t *testing.T) {
		t.Run("", func(t *testing.T) {
			n, err := parseChunk(t, `@a`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&InvalidAliasRelatedNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_URL_EXPRESSION},
							false,
						},
						Raw: "@a",
					},
				},
			}, n)
		})

		t.Run("in list", func(t *testing.T) {
			n, err := parseChunk(t, `[@a]`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&ListLiteral{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 4},
							IsParenthesized: false,
							/*[]Token{
								{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
							},*/
						},
						Elements: []Node{
							&InvalidAliasRelatedNode{
								NodeBase: NodeBase{
									NodeSpan{1, 3},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_URL_EXPRESSION},
									false,
								},
								Raw: "@a",
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("integer literal", func(t *testing.T) {
		t.Run("decimal", func(t *testing.T) {
			n := mustparseChunk(t, "12")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Raw:      "12",
						Value:    12,
					},
				},
			}, n)
		})

		t.Run("hexadecimal", func(t *testing.T) {
			n := mustparseChunk(t, "0x33")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Raw:      "0x33",
						Value:    0x33,
					},
				},
			}, n)
		})

		t.Run("octal", func(t *testing.T) {
			n := mustparseChunk(t, "0o33")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Raw:      "0o33",
						Value:    0o33,
					},
				},
			}, n)
		})

		t.Run("negative", func(t *testing.T) {
			n := mustparseChunk(t, "-0")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Raw:      "-0",
						Value:    -0,
					},
				},
			}, n)
		})

		t.Run("minimum", func(t *testing.T) {
			n := mustparseChunk(t, "-9223372036854775808")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
						Raw:      "-9223372036854775808",
						Value:    -9223372036854775808,
					},
				},
			}, n)
		})
	})

	t.Run("float literal", func(t *testing.T) {
		t.Run("float literal", func(t *testing.T) {
			n := mustparseChunk(t, "12.0")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Raw:      "12.0",
						Value:    12.0,
					},
				},
			}, n)
		})

		t.Run("underscore in whole part", func(t *testing.T) {
			n := mustparseChunk(t, "1_000.0")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Raw:      "1_000.0",
						Value:    1_000.0,
					},
				},
			}, n)
		})

		t.Run("underscore in fractionam part", func(t *testing.T) {
			n := mustparseChunk(t, "1.000_000")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
						Raw:      "1.000_000",
						Value:    1.0,
					},
				},
			}, n)
		})

		t.Run("positive exponent", func(t *testing.T) {
			n := mustparseChunk(t, "12.0e2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Raw:      "12.0e2",
						Value:    1200.0,
					},
				},
			}, n)
		})

		t.Run("negative exponent", func(t *testing.T) {
			n := mustparseChunk(t, "12.0e-2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Raw:      "12.0e-2",
						Value:    0.12,
					},
				},
			}, n)
		})
	})

	t.Run("quantity literal", func(t *testing.T) {
		t.Run("non zero integer", func(t *testing.T) {
			n := mustparseChunk(t, "1s")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Raw:      "1s",
						Units:    []string{"s"},
						Values:   []float64{1.0},
					},
				},
			}, n)
		})

		t.Run("zero integer", func(t *testing.T) {
			n := mustparseChunk(t, "0s")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Raw:      "0s",
						Units:    []string{"s"},
						Values:   []float64{0},
					},
				},
			}, n)
		})

		t.Run("hexadecimal integer", func(t *testing.T) {
			n, err := parseChunk(t, "0x3s", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, QUANTITY_LIT_NOT_ALLOWED_WITH_HEXADECIMAL_NUM},
							false,
						},
						Raw:    "0x3s",
						Units:  []string{"s"},
						Values: []float64{3},
					},
				},
			}, n)
		})

		t.Run("octal integer", func(t *testing.T) {
			n, err := parseChunk(t, "0o3s", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, QUANTITY_LIT_NOT_ALLOWED_WITH_OCTAL_NUM},
							false,
						},
						Raw:    "0o3s",
						Units:  []string{"s"},
						Values: []float64{3},
					},
				},
			}, n)
		})

		t.Run("non-zero float", func(t *testing.T) {
			n := mustparseChunk(t, "1.5s")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Raw:      "1.5s",
						Units:    []string{"s"},
						Values:   []float64{1.5},
					},
				},
			}, n)
		})

		t.Run("zero float", func(t *testing.T) {
			n := mustparseChunk(t, "0.0s")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Raw:      "0.0s",
						Units:    []string{"s"},
						Values:   []float64{0},
					},
				},
			}, n)
		})

		t.Run("multiplier", func(t *testing.T) {
			n := mustparseChunk(t, "1ks")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
						Raw:      "1ks",
						Units:    []string{"ks"},
						Values:   []float64{1.0},
					},
				},
			}, n)
		})

		t.Run("multiple parts", func(t *testing.T) {
			n := mustparseChunk(t, "1s10ms")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Raw:      "1s10ms",
						Units:    []string{"s", "ms"},
						Values:   []float64{1.0, 10},
					},
				},
			}, n)
		})
	})

	t.Run("date-like literals", func(t *testing.T) {
		t.Run("year literal", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&YearLiteral{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
						Raw:      "2020y-UTC",
						Value:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			}, n)
		})

		t.Run("year literal: missing location after dash", func(t *testing.T) {
			n, err := parseChunk(t, "2020y-", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&YearLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, INVALID_DATELIKE_LITERAL_MISSING_LOCATION_PART_AT_THE_END},
							false,
						},
						Raw: "2020y-",
					},
				},
			}, n)
		})

		t.Run("year literal: parenthesized, missing location after dash", func(t *testing.T) {
			n, err := parseChunk(t, "(2020y-)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&YearLiteral{
						NodeBase: NodeBase{
							NodeSpan{1, 7},
							&ParsingError{UnspecifiedParsingError, INVALID_DATELIKE_LITERAL_MISSING_LOCATION_PART_AT_THE_END},
							true,
						},
						Raw: "2020y-",
					},
				},
				// Tokens: []Token{
				// 	{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
				// 	{Type: OPENING_PARENTHESIS, Span: NodeSpan{7, 8}},
				// },
			}, n)
		})

		t.Run("date: missing day", func(t *testing.T) {
			n, err := parseChunk(t, "2020y-5mt-UTC", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							&ParsingError{UnspecifiedParsingError, INVALID_DATE_LITERAL_DAY_COUNT_PROBABLY_MISSING},
							false,
						},
						Raw: "2020y-5mt-UTC",
					},
				},
			}, n)
		})

		t.Run("date: invalid day: 0", func(t *testing.T) {
			n, err := parseChunk(t, "2020y-1mt-0d-UTC", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{UnspecifiedParsingError, INVALID_DAY_VALUE},
							false,
						},
						Raw: "2020y-1mt-0d-UTC",
					},
				},
			}, n)
		})

		t.Run("date: day 05", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-1mt-05d-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
						Raw:      "2020y-1mt-05d-UTC",
						Value:    time.Date(2020, 1, 5, 0, 0, 0, 0, time.UTC),
					},
				},
			}, n)
		})

		t.Run("date: missing month", func(t *testing.T) {
			n, err := parseChunk(t, "2020y-5d-UTC", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_DATE_LITERAL_MONTH_COUNT_PROBABLY_MISSING},
							false,
						},
						Raw: "2020y-5d-UTC",
					},
				},
			}, n)
		})

		t.Run("date: invalid month value: 0", func(t *testing.T) {
			n, err := parseChunk(t, "2020y-0mt-1d-UTC", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{UnspecifiedParsingError, INVALID_MONTH_VALUE},
							false,
						},
						Raw: "2020y-0mt-1d-UTC",
					},
				},
			}, n)
		})

		t.Run("date: month 05", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-05mt-1d-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
						Raw:      "2020y-05mt-1d-UTC",
						Value:    time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			}, n)
		})

		t.Run("date: missing location part", func(t *testing.T) {
			n, err := parseChunk(t, "2020y-5mt-1d", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_DATELIKE_LITERAL_MISSING_LOCATION_PART_AT_THE_END},
							false,
						},
						Raw: "2020y-5mt-1d",
					},
				},
			}, n)
		})

		t.Run("datetime: microseconds", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-1mt-1d-5us-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&DateTimeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
						Raw:      "2020y-1mt-1d-5us-UTC",
						Value:    time.Date(2020, 1, 1, 0, 0, 0, 5_000, time.UTC),
					},
				},
			}, n)
		})

		t.Run("datetime: up to minutes", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-10mt-5d-5h-4m-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&DateTimeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
						Raw:      "2020y-10mt-5d-5h-4m-UTC",
						Value:    time.Date(2020, 10, 5, 5, 4, 0, 0, time.UTC),
					},
				},
			}, n)
		})

		t.Run("datetime: up to microseconds", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-10mt-5d-5h-4m-5s-400ms-100us-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 38}, nil, false},
				Statements: []Node{
					&DateTimeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 38}, nil, false},
						Raw:      "2020y-10mt-5d-5h-4m-5s-400ms-100us-UTC",
						Value:    time.Date(2020, 10, 5, 5, 4, 5, 400_000_000+100_000, time.UTC),
					},
				},
			}, n)
		})

		t.Run("datetime: up to microseconds (longer)", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-6mt-12d-18h-4m-4s-349ms-665us-Local")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 41}, nil, false},
				Statements: []Node{
					&DateTimeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 41}, nil, false},
						Raw:      "2020y-6mt-12d-18h-4m-4s-349ms-665us-Local",
						Value:    time.Date(2020, 6, 12, 18, 4, 4, (349*1_000_000)+(665*1000), time.Local),
					},
				},
			}, n)
		})

		t.Run("datetime: up to microseconds (long location)", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-6mt-12d-18h-4m-4s-349ms-665us-America/Los_Angeles")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 55}, nil, false},
				Statements: []Node{
					&DateTimeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 55}, nil, false},
						Raw:      "2020y-6mt-12d-18h-4m-4s-349ms-665us-America/Los_Angeles",
						Value:    time.Date(2020, 6, 12, 18, 4, 4, (349*1_000_000)+(665*1000), utils.Must(time.LoadLocation("America/Los_Angeles"))),
					},
				},
			}, n)
		})

	})

	t.Run("rate literal", func(t *testing.T) {
		t.Run("rate literal", func(t *testing.T) {
			n := mustparseChunk(t, "1kB/s")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&RateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Units:    []string{"kB"},
						Values:   []float64{1.0},
						DivUnit:  "s",
						Raw:      "1kB/s",
					},
				},
			}, n)

			t.Run("missing unit after '/'", func(t *testing.T) {
				n, err := parseChunk(t, "1kB/", "")
				assert.Error(t, err)
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&RateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								&ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT_DIV_SYMBOL_SHOULD_BE_FOLLOWED_BY_UNIT},
								false,
							},
							Units:  []string{"kB"},
							Values: []float64{1.0},
							Raw:    "1kB/",
						},
					},
				}, n)
			})

			t.Run("invalid unit after '/'", func(t *testing.T) {
				n, err := parseChunk(t, "1kB/1", "")
				assert.Error(t, err)
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&RateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								&ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT},
								false,
							},
							Units:  []string{"kB"},
							Values: []float64{1.0},
							Raw:    "1kB/",
						},
						&IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{4, 5},
								&ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY},
								false,
							},
							Raw:   "1",
							Value: 1,
						},
					},
				}, n)
			})

			t.Run("invalid unit after '/'", func(t *testing.T) {
				n, err := parseChunk(t, "1kB/a1", "")
				assert.Error(t, err)
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&RateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 5},
								&ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT},
								false,
							},
							Units:   []string{"kB"},
							Values:  []float64{1.0},
							DivUnit: "a",
							Raw:     "1kB/a",
						},
						&IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{5, 6},
								&ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY},
								false,
							},
							Raw:   "1",
							Value: 1,
						},
					},
				}, n)
			})
		})

		t.Run("unterminated rate literal", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "1kB/")
			})
		})
	})

	t.Run("string literal", func(t *testing.T) {

		testCases := map[string]struct {
			result Node
			error  bool
		}{
			`""`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Raw:      `""`,
					Value:    ``,
				},
			},

			`" "`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Raw:      `" "`,
					Value:    ` `,
				},
			},

			`"é"`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Raw:      `"é"`,
					Value:    `é`,
				},
			},

			`"\\"`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Raw:      `"\\"`,
					Value:    `\`,
				},
			},

			`"\\\\"`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Raw:      `"\\\\"`,
					Value:    `\\`,
				},
			},

			`"\u0061"`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Raw:      `"\u0061"`,
					Value:    `a`,
				},
			},

			`"ab`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{
						NodeSpan{0, 3},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STRING_LIT},
						false,
					},
					Raw:   `"ab`,
					Value: ``,
				},
				error: true,
			},
			"\"ab\n1": {
				result: &Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 5},
						nil,
						false,
					},
					Statements: []Node{
						&QuotedStringLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STRING_LIT},
								false,
							},
							Raw:   `"ab`,
							Value: ``,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},

				error: true,
			},

			`+`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
					Raw:      `+`,
					Value:    `+`,
				},
			},

			`-`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
					Raw:      `-`,
					Value:    `-`,
				},
			},

			`--`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Raw:      `--`,
					Value:    `--`,
				},
			},

			`[--]`: {
				result: &ListLiteral{
					NodeBase: NodeBase{
						NodeSpan{0, 4},
						nil,
						false,
						/*[]Token{
							{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
							{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
						},*/
					},
					Elements: []Node{
						&UnquotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Raw:      `--`,
							Value:    `--`,
						},
					},
				},
			},

			`+\:`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Raw:      `+\:`,
					Value:    `+:`,
				},
			},

			`-- 2`: {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&UnquotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Raw:      `--`,
							Value:    `--`,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			},

			"``": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Raw:      "``",
							Value:    "",
						},
					},
				},
			},

			"`1`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Raw:      "`1`",
							Value:    "1",
						},
					},
				},
			},
			"`\n`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Raw:      "`\n`",
							Value:    "\n",
						},
					},
				},
			},
			"`\n\r\n`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Raw:      "`\n\r\n`",
							Value:    "\n\r\n",
						},
					},
				},
			},

			"`\\n\\r\\t`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
							Raw:      "`\\n\\r\\t`",
							Value:    "\n\r\t",
						},
					},
				},
			},
			"`\"`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Raw:      "`\"`",
							Value:    "\"",
						},
					},
				},
			},
			"`\"a\"`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Raw:      "`\"a\"`",
							Value:    "\"a\"",
						},
					},
				},
			},
			"`\\u0061`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
							Raw:      "`\\u0061`",
							Value:    "a",
						},
					},
				},
			},
			"`\\``": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Raw:      "`\\``",
							Value:    "`",
						},
					},
				},
			},
			"`\\\\\\``": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
							Raw:      "`\\\\\\``",
							Value:    "\\`",
						},
					},
				},
			},
			"`\\`\\``": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
							Raw:      "`\\`\\``",
							Value:    "``",
						},
					},
				},
			},
			"`\\\\`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Raw:      "`\\\\`",
							Value:    "\\",
						},
					},
				},
			},
			"`\\`e`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Raw:      "`\\`e`",
							Value:    "`e",
						},
					},
				},
			},
			"`e\\``": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Raw:      "`e\\``",
							Value:    "e`",
						},
					},
				},
			},
			"`": {
				error: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_MULTILINE_STRING_LIT},
								false,
							},
							Raw:   "`",
							Value: "",
						},
					},
				},
			},
		}

		for input, testCase := range testCases {
			t.Run(input, func(t *testing.T) {
				n, err := parseChunk(t, input, "")

				if !testCase.error {
					if !assert.NoError(t, err) {
						return
					}
				} else {
					assert.Error(t, err)
				}

				if _, ok := testCase.result.(*Chunk); ok {
					assert.Equal(t, testCase.result, n)
				} else {
					assert.Equal(t, &Chunk{
						NodeBase:   NodeBase{NodeSpan{0, testCase.result.Base().Span.End}, nil, false},
						Statements: []Node{testCase.result},
					}, n)
				}
			})
		}

	})

	t.Run("byte slice literal", func(t *testing.T) {
		testCases := []struct {
			input string
			value []byte
			err   string
		}{
			//hexadecimal
			{
				"0x[]",
				[]byte{},
				"",
			},
			{
				"0x[1]",
				[]byte{},
				INVALID_HEX_BYTE_SICE_LIT_LENGTH_SHOULD_BE_EVEN,
			},
			{
				"0x[12]",
				[]byte{0x12},
				"",
			},
			{
				"0x[12 12]",
				[]byte{0x12, 0x12},
				"",
			},
			{
				"0x[121 2]",
				[]byte{0x12, 0x12},
				"",
			},
			{
				"0x[1 212]",
				[]byte{0x12, 0x12},
				"",
			},
			{
				"(0x[12)",
				[]byte{0x12},
				UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET,
			},

			//binary
			{
				"0b[]",
				[]byte{},
				"",
			},
			{
				"0b[1]",
				[]byte{1},
				"",
			},
			{
				"0b[0]",
				[]byte{0},
				"",
			},
			{
				"0b[01]",
				[]byte{0b1},
				"",
			},
			{
				"0b[10]",
				[]byte{0b10},
				"",
			},
			{
				"0b[1000 0000]",
				[]byte{0b1000_0000},
				"",
			},
			{
				"0b[0000 0000]",
				[]byte{0b0000_0000},
				"",
			},
			{
				"0b[1000 0000 1]",
				[]byte{0b1000_0000, 1},
				"",
			},
			{
				"0b[0000 0000 1]",
				[]byte{0b0000_0000, 1},
				"",
			},
			{
				"0b[0000 0000 0000 0000]",
				[]byte{0b0000_0000, 0b0000_0000},
				"",
			},
			{
				"(0b[1)",
				[]byte{0x1},
				UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET,
			},

			//decimal
			{
				"0d[]",
				[]byte{},
				"",
			},
			{
				"0d[1]",
				[]byte{1},
				"",
			},
			{
				"0d[12]",
				[]byte{12},
				"",
			},
			{
				"0d[12 12]",
				[]byte{12, 12},
				"",
			},
			{
				"0d[121 2]",
				[]byte{121, 2},
				"",
			},
			{
				"0d[1 212]",
				[]byte{1, 212},
				"",
			},
			{
				"0d[1 256]",
				nil,
				fmtInvalidByteInDecimalByteSliceLiteral([]byte("256")),
			},
			{
				"(0d[1)",
				[]byte{0x1},
				UNTERMINATED_BYTE_SICE_LIT_MISSING_CLOSING_BRACKET,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, err := parseChunk(t, testCase.input, "")
				assert.IsType(t, &ByteSliceLiteral{}, n.Statements[0])

				literal := n.Statements[0].(*ByteSliceLiteral)

				if testCase.err == "" {
					assert.NoError(t, err)
					assert.Equal(t, testCase.value, literal.Value)
				} else {
					assert.Contains(t, literal.Err.Message, testCase.err)
				}
			})
		}
	})

	t.Run("rune literal", func(t *testing.T) {

		t.Run("rune literal : simple character", func(t *testing.T) {
			n := mustparseChunk(t, `'a'`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&RuneLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
						Value:    'a',
					},
				},
			}, n)
		})

		t.Run("rune literal : valid escaped character", func(t *testing.T) {
			n := mustparseChunk(t, `'\n'`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&RuneLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Value:    '\n',
					},
				},
			}, n)
		})

		t.Run("rune literal : invalid escaped character", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, `'\z'`)
			})
		})

		t.Run("rune literal : missing character", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, `''`)
			})
		})

	})

	t.Run("single letter", func(t *testing.T) {
		t.Run("single letter", func(t *testing.T) {
			n := mustparseChunk(t, `e`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},

				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Name:     "e",
					},
				},
			}, n)
		})

		t.Run("letter followed by a digit", func(t *testing.T) {
			n := mustparseChunk(t, `e2`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Name:     "e2",
					},
				},
			}, n)
		})

		t.Run("empty unambiguous identifier", func(t *testing.T) {
			n, err := parseChunk(t, `#`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},

				Statements: []Node{
					&UnambiguousIdentifierLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT},
							false,
						},
						Name: "",
					},
				},
			}, n)
		})

		t.Run("single letter unambiguous identifier", func(t *testing.T) {
			n := mustparseChunk(t, `#e`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},

				Statements: []Node{
					&UnambiguousIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Name:     "e",
					},
				},
			}, n)
		})

		t.Run("unambiguous identifier literal : letter followed by a digit", func(t *testing.T) {
			n := mustparseChunk(t, `#e2`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&UnambiguousIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
						Name:     "e2",
					},
				},
			}, n)
		})

	})

	t.Run("assignment", func(t *testing.T) {
		t.Run("var = <value>", func(t *testing.T) {
			n := mustparseChunk(t, "$a = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Name:     "b",
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("var += <value>", func(t *testing.T) {
			n := mustparseChunk(t, "$a += $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "b",
						},
						Operator: PlusAssign,
					},
				},
			}, n)
		})

		t.Run("identifier = <value>", func(t *testing.T) {
			n := mustparseChunk(t, "a = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
							Name:     "b",
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("keyword = <value>", func(t *testing.T) {
			res, err := parseChunk(t, "const ()\nmanifest {}\nmanifest = $b", "")
			assert.Error(t, err)
			assert.NotNil(t, res)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("<index expr> = <value>", func(t *testing.T) {
			n := mustparseChunk(t, "$a[0] = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
						},
						Left: &IndexExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Indexed: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							Index: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Raw:      "0",
								Value:    0,
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{8, 10}, nil, false},
							Name:     "b",
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("var = | <pipeline>", func(t *testing.T) {
			n := mustparseChunk(t, "$a = | a | b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: EQUAL, Span: NodeSpan{3, 4}},
								{Type: PIPE, Span: NodeSpan{5, 6}},
							},*/
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Right: &PipelineExpression{
							NodeBase: NodeBase{
								NodeSpan{7, 12},
								nil,
								false,
							},
							Stages: []*PipelineStage{
								{
									Kind: NormalStage,
									Expr: &CallExpression{
										NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
										Callee: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
											Name:     "a",
										},
										Must:              true,
										CommandLikeSyntax: true,
									},
								},
								{
									Kind: NormalStage,
									Expr: &CallExpression{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
										Callee: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Name:     "b",
										},
										Must:              true,
										CommandLikeSyntax: true,
									},
								},
							},
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("<identifier member expr> = <value>", func(t *testing.T) {
			n := mustparseChunk(t, "a.b = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
						},
						Left: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "b",
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("var = new <type>", func(t *testing.T) {
			n := mustparseChunk(t, "$a = new Lexer")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Right: &NewExpression{
							NodeBase: NodeBase{NodeSpan{5, 14}, nil, false},
							Type: &PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{9, 14}, nil, false},
								Name:       "Lexer",
								Unprefixed: true,
							},
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("missing terminator", func(t *testing.T) {
			n, err := parseChunk(t, "$a = $b 2", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{InvalidNext, UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Name:     "b",
						},
						Operator: Assign,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
						Raw:      "2",
						Value:    2,
					},
				},
			}, n)
		})

		t.Run("missing RHS: '=' followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "$a =", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_ASSIGNMENT_MISSING_VALUE_AFTER_EQL_SIGN},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("missing RHS: '=' followed by space + EOF", func(t *testing.T) {
			n, err := parseChunk(t, "$a = ", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_ASSIGNMENT_MISSING_VALUE_AFTER_EQL_SIGN},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Operator: Assign,
					},
				},
			}, n)
		})

		t.Run("missing RHS: '=' followed by linefeed", func(t *testing.T) {
			n, err := parseChunk(t, "$a =\n", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_ASSIGNMENT_MISSING_VALUE_AFTER_EQL_SIGN},
							false,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						Operator: Assign,
					},
				},
			}, n)
		})
	})

	t.Run("multi assignement statement", func(t *testing.T) {
		t.Run("assign <ident> = <var>", func(t *testing.T) {
			n := mustparseChunk(t, "assign a = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{9, 10}},
							},*/
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Name:     "a",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{11, 13}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("assign var var = var", func(t *testing.T) {
			n := mustparseChunk(t, "assign a b = $c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
							/*[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{11, 12}},
							},*/
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Name:     "a",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
								Name:     "b",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{13, 15}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("nillable", func(t *testing.T) {
			n := mustparseChunk(t, "assign? a = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
							/*[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: QUESTION_MARK, Span: NodeSpan{6, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
								Name:     "a",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 14}, nil, false},
							Name:     "b",
						},
						Nillable: true,
					},
				},
			}, n)
		})

		t.Run("keyword LHS", func(t *testing.T) {
			res, err := parseChunk(t, "const ()\nmanifest {}\nassign manifest = $b", "")
			assert.Error(t, err)
			assert.NotNil(t, res)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("missing terminator", func(t *testing.T) {
			n, err := parseChunk(t, "assign a = $b 2", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							&ParsingError{InvalidNext, UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR},
							false,
							/*[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{9, 10}},
							},*/
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Name:     "a",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{11, 13}, nil, false},
							Name:     "b",
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
						Raw:      "2",
						Value:    2,
					},
				},
			}, n)
		})

		t.Run("only LHS", func(t *testing.T) {
			n, err := parseChunk(t, "assign a", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MULTI_ASSIGN_MISSING_EQL_SIGN},
							false,
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Name:     "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing value after equal sign", func(t *testing.T) {
			n, err := parseChunk(t, "assign a =", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{9, 10}},
							},*/
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Name:     "a",
							},
						},
						Right: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{9, 10},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("assign a ="), 10, true)},
								false,
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("call with parenthesis", func(t *testing.T) {
		t.Run("no args", func(t *testing.T) {
			n := mustparseChunk(t, "print()")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: nil,
					},
				},
			}, n)
		})

		t.Run("no args 2", func(t *testing.T) {
			n := mustparseChunk(t, "print( )")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: nil,
					},
				},
			}, n)
		})

		t.Run("exclamation mark", func(t *testing.T) {
			n := mustparseChunk(t, "print!()")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
							/*[]Token{
								{Type: EXCLAMATION_MARK, Span: NodeSpan{5, 6}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: nil,
						Must:      true,
					},
				},
			}, n)
		})

		t.Run("single arg", func(t *testing.T) {
			n := mustparseChunk(t, "print($a)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
								Name:     "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("two args", func(t *testing.T) {
			n := mustparseChunk(t, "print($a $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
								Name:     "a",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{9, 11}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single arg: spread argument", func(t *testing.T) {
			n := mustparseChunk(t, "print(...$a)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&SpreadArgument{
								NodeBase: NodeBase{
									NodeSpan{6, 11},
									nil,
									false,
								},
								Expr: &Variable{
									NodeBase: NodeBase{NodeSpan{9, 11}, nil, false},
									Name:     "a",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char", func(t *testing.T) {
			n, err := parseChunk(t, "print(?1)", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{6, 7},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCallArguments('?')},
									false,
								},
							},
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
						},
					},
				},
			}, n)
		})

		t.Run("callee is an identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "http.get()")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Callee: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
								Name:     "http",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
									Name:     "get",
								},
							},
						},
						Arguments: nil,
					},
				},
			}, n)
		})

		t.Run("callee is a member expression", func(t *testing.T) {
			n := mustparseChunk(t, `$a.b("a")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Callee: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
						Arguments: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("double call", func(t *testing.T) {
			n := mustparseChunk(t, "print()()")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{7, 8}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Callee: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
								},*/
							},
							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
								Name:     "print",
							},
							Arguments: nil,
						},
						Arguments: nil,
					},
				},
			}, n)
		})
	})

	t.Run("command-like call", func(t *testing.T) {

		t.Run("no arg", func(t *testing.T) {
			n := mustparseChunk(t, "print;")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 6}, nil, false},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: nil,
					},
				},
			}, n)
		})

		t.Run("one arg", func(t *testing.T) {
			n := mustparseChunk(t, "print $a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 8}, nil, false},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
								Name:     "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("one arg followed by a line feed", func(t *testing.T) {
			n := mustparseChunk(t, "print $a\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 9},
					nil,
					false,
				},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 8}, nil, false},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
								Name:     "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("two args", func(t *testing.T) {
			n := mustparseChunk(t, "print $a $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 11}, nil, false},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
								Name:     "a",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{9, 11}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single arg with a delimiter", func(t *testing.T) {
			n := mustparseChunk(t, "print []")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 8}, nil, false},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&ListLiteral{
								NodeBase: NodeBase{
									NodeSpan{6, 8},
									nil,
									false,
									/*[]Token{
										{Type: OPENING_BRACKET, Span: NodeSpan{6, 7}},
										{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
									},*/
								},
								Elements: nil,
							},
						},
					},
				},
			}, n)
		})

		t.Run("single arg starting with the same character as an assignment operator", func(t *testing.T) {
			n := mustparseChunk(t, "print /")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 7}, nil, false},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&AbsolutePathLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Raw:      "/",
								Value:    "/",
							},
						},
					},
				},
			}, n)
		})

		t.Run("call followed by a single line comment", func(t *testing.T) {
			n := mustparseChunk(t, "print $a $b # comment")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
								Name:     "a",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{9, 11}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("callee is an identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, `a.b "a"`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 7}, nil, false},
						Callee: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
							},
						},
						Arguments: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("pipeline statement", func(t *testing.T) {
		t.Run("empty second stage", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "print $a |")
			})
		})

		t.Run("second stage is not a call", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "print $a | 1")
			})
		})

		t.Run("second stage is a call with no arguments", func(t *testing.T) {
			n := mustparseChunk(t, "print $a | do-something")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
											Name:     "a",
										},
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{11, 23}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, false},
										Name:     "do-something",
									},
									Arguments: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("second stage is a call with no arguments, followed by a ';'", func(t *testing.T) {
			n := mustparseChunk(t, "print $a | do-something;")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 24},
					nil,
					false,
				},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
											Name:     "a",
										},
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{11, 23}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, false},
										Name:     "do-something",
									},
									Arguments: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("second stage is a call with no arguments, followed by another statement on the following line", func(t *testing.T) {
			n := mustparseChunk(t, "print $a | do-something\n1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 25},
					nil,
					false,
				},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
											Name:     "a",
										},
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{11, 23}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, false},
										Name:     "do-something",
									},
									Arguments: nil,
								},
							},
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{24, 25}, nil, false},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("first and second stages are calls with no arguments", func(t *testing.T) {
			n := mustparseChunk(t, "print | do-something")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 6}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
										Name:     "print",
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{8, 20}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 20}, nil, false},
										Name:     "do-something",
									},
									Arguments: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("second stage is a call with a single argument", func(t *testing.T) {
			n := mustparseChunk(t, "print $a | do-something $")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
											Name:     "a",
										},
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{11, 25}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, false},
										Name:     "do-something",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{24, 25}, nil, false},
											Name:     "",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("third stage is a call with no arguments", func(t *testing.T) {
			n := mustparseChunk(t, "print $a | do-something $ | do-something-else")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 45}, nil, false},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 45},
							nil,
							false,
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
											Name:     "a",
										},
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{11, 25}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, false},
										Name:     "do-something",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{24, 25}, nil, false},
											Name:     "",
										},
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{28, 45}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{28, 45}, nil, false},
										Name:     "do-something-else",
									},
									Arguments: nil,
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("call <string> shorthand", func(t *testing.T) {
		n := mustparseChunk(t, `mime"json"`)
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
			Statements: []Node{
				&CallExpression{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Must:     true,
					Callee: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Name:     "mime",
					},
					Arguments: []Node{
						&QuotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{4, 10}, nil, false},
							Raw:      `"json"`,
							Value:    "json",
						},
					},
				},
			},
		}, n)
	})

	t.Run("call <object> shorthand", func(t *testing.T) {
		n := mustparseChunk(t, `f{}`)
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
			Statements: []Node{
				&CallExpression{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Must:     true,
					Callee: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
						Name:     "f",
					},
					Arguments: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{1, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{1, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},*/
							},
						},
					},
				},
			},
		}, n)
	})

	t.Run("object literal", func(t *testing.T) {

		testCases := []struct {
			input    string
			hasError bool
			result   Node
		}{
			{
				input:    "{}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{1, 2}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{ ",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{ }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{\n}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{1, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{,}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{1, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{,",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{1, 2}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "({)",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{1, 2},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								true,
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{ a: 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 6},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a:1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 5},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{a:1?}",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 6}},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{1, 4},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{4, 5},
										&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectRecord('?')},
										false,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "({a:1)",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{1, 5},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								true,
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{Span: NodeSpan{2, 5}},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a: 1, a: 2}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 13},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 6},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{8, 12},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{a\n",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{Span: NodeSpan{1, 2}},
									Value: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
										Name:     "a",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a :\n1 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{5, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 5},
										&ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON},
										false,
										/*[]Token{
											{Type: COLON, Span: NodeSpan{4, 5}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a:\n}",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{4, 5}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 4},
										&ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON},
										false,
										/*[]Token{
											{Type: COLON, Span: NodeSpan{3, 4}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a:}",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 5},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 4},
										&ParsingError{MissingObjectPropertyValue, MISSING_PROPERTY_VALUE},
										false,
										/*[]Token{
											{Type: COLON, Span: NodeSpan{3, 4}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a %int: 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 13},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 11},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Type: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
										Name:     "int",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ # comment \n}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMENT, Span: NodeSpan{2, 12}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{12, 13}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "{ a : 1 # comment \n}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMENT, Span: NodeSpan{8, 18}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{18, 19}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
										/*[]Token{
											{Type: COLON, Span: NodeSpan{4, 5}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ # comment \n a : 1}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMENT, Span: NodeSpan{2, 12}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{12, 13}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{14, 19},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : # comment \n 1}",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 19},
										&ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyCommentBeforeValueOfKey("a")},
										false,
										/*[]Token{
											{Type: COLON, Span: NodeSpan{4, 5}},
											{Type: COMMENT, Span: NodeSpan{6, 16}, Raw: "# comment "},
											{Type: NEWLINE, Span: NodeSpan{16, 17}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 5},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 4}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{1",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ 1",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{\n1",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{1, 2}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{1\n",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ (\"1\") }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{3, 8}, nil, false},
									Key:      nil,
									Value: &QuotedStringLiteral{
										NodeBase: NodeBase{
											NodeSpan{3, 6},
											nil,
											true,
											/*[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
											},*/
										},
										Raw:   `"1"`,
										Value: "1",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ 1 %int }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 8},
										&ParsingError{UnspecifiedParsingError, ONLY_EXPLICIT_KEY_CAN_HAVE_A_TYPE_ANNOT},
										false,
									},
									Key: nil,
									Type: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
										Name:     "int",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ 1 2 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 4},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										false,
									},
									Key: nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1  b : 2 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 16},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{9, 14},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1 , b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{10, 15},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1 \n }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			}, {
				input:    "{ \n a : 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{4, 9},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ .name }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 8}, nil, false},
									Value: &PropertyNameLiteral{
										NodeBase: NodeBase{NodeSpan{2, 7}, nil, false},
										Name:     "name",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1 \n b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{10, 15},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1 \n\n b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: NEWLINE, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{11, 16},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1 \n \n b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: NEWLINE, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{12, 17},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ a : 1 \n  \n b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: NEWLINE, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{13, 18},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{17, 18}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ ... $e.{name} }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},*/
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 15},
										nil,
										false,
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{6, 15}, nil, false},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 15},
												nil,
												false,
												/*[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{8, 10}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
												},*/
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{10, 14}, nil, false},
													Name:     "name",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ _constraints_ { } }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							MetaProperties: []*ObjectMetaProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 19}, nil, false},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 15}, nil, false},
										Name:     "_constraints_",
									},
									Initialization: &InitializationBlock{
										NodeBase: NodeBase{
											NodeSpan{16, 19},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
											},*/
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ ... $e }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 8},
										&ParsingError{ExtractionExpressionExpected, fmtInvalidSpreadElemExprShouldBeExtrExprNot((*Variable)(nil))},
										false,
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
										Name:     "e",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "{ ... $e.{name} 1 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{16, 18}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 15},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										false,
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{6, 15}, nil, false},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 15},
												nil,
												false,
												/*[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{8, 10}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
												},*/
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{10, 14}, nil, false},
													Name:     "name",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},

			{
				input:    "{]}",
				hasError: true,
			},
			{
				input:    "{] }",
				hasError: true,
			},
			{
				input:    "{ ]}",
				hasError: true,
			},
			{
				input:    "{ ] }",
				hasError: true,
			},
			//
			{
				input:    "{], a: 1}",
				hasError: true,
			},
			{
				input:    "{] a: 1}",
				hasError: true,
			},
			//
			{
				input:    "{ a : ] }",
				hasError: true,
			},
			{
				input:    "{ a : 1] }",
				hasError: true,
			},
			{
				input:    "{ a : 1,] }",
				hasError: true,
			},
			{
				input:    "{ a : 1 ] }",
				hasError: true,
			},
			//
			{
				input:    "{ a : ]b: 2 }",
				hasError: true,
			},
			{
				input:    "{ a : ] b: 2 }",
				hasError: true,
			},
			{
				input:    "{ a : 1]b: 2 }",
				hasError: true,
			},
			{
				input:    "{ a : 1] b: 2 }",
				hasError: true,
			},
			{
				input:    "{ a : 1,]b: 2 }",
				hasError: true,
			},
			{
				input:    "{ a : 1 ]b: 2 }",
				hasError: true,
			},
			//
			{
				input:    "{:}",
				hasError: true,
			},
			{
				input:    "{: }",
				hasError: true,
			},
			{
				input:    "{ :}",
				hasError: true,
			},
			{
				input:    "{ : }",
				hasError: true,
			},
			//
			{
				input:    "{:, a: 1}",
				hasError: true,
			},
			{
				input:    "{: a: 1}",
				hasError: true,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, err := parseChunk(t, testCase.input, "")
				if testCase.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if testCase.result != nil {
					assert.Equal(t, testCase.result, n)
				}
			})
		}
	})

	t.Run("record literal", func(t *testing.T) {

		testCases := []struct {
			input    string
			hasError bool
			result   Node
		}{
			{
				input:    "#{}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, {
				input:    "#{ }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "#{\n}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "#{ a: 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a:1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 6},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a : 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a: 1, a: 2}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{9, 13},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a :\n1 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 6},
										&ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON},
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a:}",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 5},
										&ParsingError{MissingObjectPropertyValue, MISSING_PROPERTY_VALUE},
										false,
										/*[]Token{
											{Type: COLON, Span: NodeSpan{4, 5}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ # comment \n}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMENT, Span: NodeSpan{3, 13}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{13, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			},
			{
				input:    "#{ a : 1 # comment \n}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMENT, Span: NodeSpan{9, 19}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ # comment \n a : 1}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMENT, Span: NodeSpan{3, 13}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{13, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{15, 20},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a : # comment \n 1}",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 20},
										&ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyCommentBeforeValueOfKey("a")},
										false,
										/*[]Token{
											{Type: COLON, Span: NodeSpan{5, 6}},
											{Type: COMMENT, Span: NodeSpan{7, 17}, Raw: "# comment "},
											{Type: NEWLINE, Span: NodeSpan{17, 18}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{3, 5}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ (\"1\") }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{4, 9}, nil, false},
									Key:      nil,
									Value: &QuotedStringLiteral{
										NodeBase: NodeBase{
											NodeSpan{4, 7},
											nil,
											true,
											/*[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
											},*/
										},
										Raw:   `"1"`,
										Value: "1",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ 1 2 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 5},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										false,
									},
									Key: nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a : 1  b : 2 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{10, 15},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			}, {
				input:    "#{ a : 1 , b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{11, 16},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a : 1 \n }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			}, {
				input:    "#{ \n a : 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{3, 4}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{5, 10},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ a : 1 \n b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{11, 16},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ .name }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{3, 9}, nil, false},
									Value: &PropertyNameLiteral{
										NodeBase: NodeBase{NodeSpan{3, 8}, nil, false},
										Name:     "name",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ ... $e.{name} }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 16},
										nil,
										false,
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{9, 16},
												nil,
												false,
												/*[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{9, 11}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
												},*/
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{11, 15}, nil, false},
													Name:     "name",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ ... $e }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 9},
										&ParsingError{ExtractionExpressionExpected, fmtInvalidSpreadElemExprShouldBeExtrExprNot((*Variable)(nil))},
										false,
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
										Name:     "e",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{ ... $e.{name} 1 }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},*/
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{17, 19}, nil, false},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{17, 18}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 16},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										false,
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{9, 16},
												nil,
												false,
												/*[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{9, 11}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
												},*/
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{11, 15}, nil, false},
													Name:     "name",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#{]}",
				hasError: true,
			},
			{
				input:    "#{] }",
				hasError: true,
			},
			{
				input:    "#{ ]}",
				hasError: true,
			},
			{
				input:    "#{ ] }",
				hasError: true,
			},
			//
			{
				input:    "#{], a: 1}",
				hasError: true,
			},
			{
				input:    "#{] a: 1}",
				hasError: true,
			},
			//
			{
				input:    "#{ a : ] }",
				hasError: true,
			},
			{
				input:    "#{ a : 1] }",
				hasError: true,
			},
			{
				input:    "#{ a : 1,] }",
				hasError: true,
			},
			{
				input:    "#{ a : 1 ] }",
				hasError: true,
			},
			//
			{
				input:    "#{ a : ]b: 2 }",
				hasError: true,
			},
			{
				input:    "#{ a : ] b: 2 }",
				hasError: true,
			},
			{
				input:    "#{ a : 1]b: 2 }",
				hasError: true,
			},
			{
				input:    "#{ a : 1] b: 2 }",
				hasError: true,
			},
			{
				input:    "#{ a : 1,]b: 2 }",
				hasError: true,
			},
			{
				input:    "#{ a : 1 ]b: 2 }",
				hasError: true,
			},
			//
			{
				input:    "#{:}",
				hasError: true,
			},
			{
				input:    "#{: }",
				hasError: true,
			},
			{
				input:    "#{ :}",
				hasError: true,
			},
			{
				input:    "#{ : }",
				hasError: true,
			},
			//
			{
				input:    "#{:, a: 1}",
				hasError: true,
			},
			{
				input:    "#{: a: 1}",
				hasError: true,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, err := parseChunk(t, testCase.input, "")
				if testCase.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if testCase.result != nil {
					assert.Equal(t, testCase.result, n)
				}
			})
		}
	})

	t.Run("list literal", func(t *testing.T) {

		testCases := []struct {
			input    string
			hasError bool
			result   Node
		}{
			{
				input: "[]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
								},*/
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "[ ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
								},*/
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "[ 1 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 5},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{4, 5}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			},
			{
				input: "[ 1 2 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{6, 7}},
								},*/
							}, Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "[ 1 , 2 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{4, 5}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "[ 1 \n 2 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{4, 5}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "[ 1, ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{3, 4}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			},
			{
				input:    "[ 1, 2",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								&ParsingError{UnspecifiedParsingError, "unterminated list literal, missing closing bracket ']'"},
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{3, 4}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "[ ...$a ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										false,
										/*[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{2, 5}},
										},*/
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
										Name:     "a",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "[ ..., ]",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{5, 6}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{2, 6},
										nil,
										false,
										/*[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{2, 5}},
										},*/
									},
									Expr: &MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{5, 6},
											&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("[ ..., ]"), 5, true)},
											false,
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input: "[]%int[]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
									{Type: OPENING_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Elements: nil,
							TypeAnnotation: &PatternIdentifierLiteral{
								NodeBase: NodeBase{Span: NodeSpan{2, 6}},
								Name:     "int",
							},
						},
					},
				},
			},
			{
				input:    "[]%int",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_OPENING_BRACKET_AFTER_TYPE},
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
								},*/
							},
							Elements: nil,
							TypeAnnotation: &PatternIdentifierLiteral{
								NodeBase: NodeBase{Span: NodeSpan{2, 6}},
								Name:     "int",
							},
						},
					},
				},
			},
			{
				input:    "[]%int[",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_CLOSING_BRACKET},
								false,
								/*[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
									{Type: OPENING_BRACKET, Span: NodeSpan{6, 7}},
								},*/
							},
							Elements: nil,
							TypeAnnotation: &PatternIdentifierLiteral{
								NodeBase: NodeBase{Span: NodeSpan{2, 6}},
								Name:     "int",
							},
						},
					},
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, err := parseChunk(t, testCase.input, "")
				if testCase.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				assert.Equal(t, testCase.result, n)
			})
		}
	})

	t.Run("tuple literal", func(t *testing.T) {

		testCases := []struct {
			input    string
			hasError bool
			result   Node
		}{
			{
				input: "#[]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
								},*/
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "#[ ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
								},*/
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "#[ 1 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			},
			{
				input: "#[ 1 2 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							}, Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "#[ 1 , 2 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{5, 6}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "#[ 1 \n 2 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{5, 6}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "#[ 1, ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{4, 5}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{6, 7}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			},
			{
				input:    "#[ 1, 2",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								&ParsingError{UnspecifiedParsingError, "unterminated list literal, missing closing bracket ']'"},
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{4, 5}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			},
			{
				input: "#[ ...$a ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										false,
										/*[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{3, 6}},
										},*/
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
										Name:     "a",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "#[ ..., ]",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{6, 7}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										nil,
										false,
										/*[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{3, 6}},
										},*/
									},
									Expr: &MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{6, 7},
											&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("[ ..., ]"), 5, true)},
											false,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, err := parseChunk(t, testCase.input, "")
				if testCase.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				assert.Equal(t, testCase.result, n)
			})
		}
	})

	t.Run("dictionary literal", func(t *testing.T) {

		testCases := []struct {
			input    string
			hasError bool
			result   Node
		}{
			{
				input:    ":{}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 3}},
						},
					},
				},
			},
			{
				input:    ":{ }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 4}},
						},
					},
				},
			},
			{
				input:    `:{ "a" : 1 }`,
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 12}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 10}},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input: `:{ https://aa/: 1 }`,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 19}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 17}},
									Key: &URLLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 14}},
										Value:    "https://aa/",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input: `:{ /aa: 1 }`,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 11}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 9}},
									Key: &AbsolutePathLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 6}},
										Value:    "/aa",
										Raw:      "/aa",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ /aa:1 }`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 10}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										Span: NodeSpan{3, 9},
										Err:  &ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_SPACE_BETWEEN_KEY_AND_COLON},
									},
									Key: &AbsolutePathLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 8}},
										Value:    "/aa:1",
										Raw:      "/aa:1",
									},
								},
							},
						},
					},
				},
			},
			{
				input: `:{ %/aa: 1 }`,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 12}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 10}},
									Key: &AbsolutePathPatternLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 7}},
										Value:    "/aa",
										Raw:      "%/aa",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input: `:{ https://aa/: 1 }`,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 19}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 17}},
									Key: &URLLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 14}},
										Value:    "https://aa/",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input: `:{ %https://aa/: 1 }`,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 20}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 18}},
									Key: &URLPatternLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 15}},
										Value:    "https://aa/",
										Raw:      "%https://aa/",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{17, 18}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input: `:{ https://aa: 1 }`,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 18}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 16}},
									Key: &HostLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 13}},
										Value:    "https://aa",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ https://aa:1 }`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 17}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										Span: NodeSpan{3, 16},
										Err:  &ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_SPACE_BETWEEN_KEY_AND_COLON},
									},
									Key: &HostLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 15}},
										Value:    "https://aa:1",
									},
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ https://aa:1:1 }`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 19}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										Span: NodeSpan{3, 18},
										Err:  &ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_SPACE_BETWEEN_KEY_AND_COLON},
									},
									Key: &InvalidURL{
										NodeBase: NodeBase{
											Span: NodeSpan{3, 17},
											Err:  &ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST},
										},
										Value: "https://aa:1:1",
									},
								},
							},
						},
					},
				},
			},
			{
				input: `:{ %https://aa: 1 }`,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 19}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 17}},
									Key: &HostPatternLiteral{
										NodeBase: NodeBase{Span: NodeSpan{3, 14}},
										Value:    "https://aa",
										Raw:      "%https://aa",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ "a" :   }`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 12}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 12},
										nil,
										false,
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{11, 12},
											&ParsingError{
												UnspecifiedParsingError,
												fmtExprExpectedHere([]rune(`:{ "a" :   }`), 11, true),
											},
											false,
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    `(:{ "a":)`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{1, 8},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_DICT_MISSING_CLOSING_BRACE},
								true,
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{4, 9},
										nil,
										false,
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{4, 7}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{8, 9},
											&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune(`(:{ "a":)`), 8, true)},
											false,
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ "a"   }`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 10}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 9},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY},
										false,
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: nil,
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ a   }`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 8}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY},
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: nil,
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ a  `,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_DICT_MISSING_CLOSING_BRACE},
								false,
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 6},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY},
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
										Name:     "a",
									},
									Value: nil,
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ "a" : 1  "b" : 2 }`,
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 21}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 10},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_LIT_ENTRY_SEPARATION},
										false,
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{Span: NodeSpan{12, 19}},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
										Raw:      `"b"`,
										Value:    "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ "a" : 1 , "b" : 2 }`,
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 22}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 10}},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{Span: NodeSpan{13, 20}},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
										Raw:      `"b"`,
										Value:    "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    ":{ \"a\" : 1 \n }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 14}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 10}},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    ":{ \n \"a\" : 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{Span: NodeSpan{0, 14}},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 14}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{5, 12}},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    ":{ \"a\" : 1 \n \"b\" : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 22}},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{Span: NodeSpan{3, 10}},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{13, 20},
										nil,
										false,
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
										Raw:      `"b"`,
										Value:    "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, err := parseChunk(t, testCase.input, "")
				if testCase.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if testCase.result != nil {
					assert.Equal(t, testCase.result, n)
				}
			})
		}
	})

	t.Run("if statement", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, "if true { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
							/*[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},*/
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		//also used for checking block parsing
		t.Run("non empty", func(t *testing.T) {
			n := mustparseChunk(t, "if true { 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},*/
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 13},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},*/
							},
							Statements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		//also used for checking call parsing
		t.Run("body contains a call without parenthesis", func(t *testing.T) {
			n := mustparseChunk(t, "if true { a 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
							/*[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},*/
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 15},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
								},*/
							},
							Statements: []Node{
								&CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{10, 14}, nil, false},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
										Name:     "a",
									},
									Arguments: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
											Raw:      `1`,
											Value:    1,
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing block after if", func(t *testing.T) {
			n, err := parseChunk(t, "if true", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{MissingBlock, UNTERMINATED_IF_STMT_MISSING_BLOCK},
							false,
							/*[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},*/
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
					},
				},
			}, n)
		})

		t.Run("multiline", func(t *testing.T) {
			n := mustparseChunk(t, "if true { \n }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},*/
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 13},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: NEWLINE, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("if-else", func(t *testing.T) {
			n := mustparseChunk(t, "if true { } else {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
							/*[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{12, 16}},
							},*/
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Statements: nil,
						},
						Alternate: &Block{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("if-else within an if-else statement", func(t *testing.T) {
			n := mustparseChunk(t, "if true { if true {} else {} } else {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 38}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 38},
							nil,
							false,
							/*[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{31, 35}},
							},*/
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 30},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{29, 30}},
								},*/
							},
							Statements: []Node{
								&IfStatement{
									NodeBase: NodeBase{
										NodeSpan{10, 28},
										nil,
										false,
										/*[]Token{
											{Type: IF_KEYWORD, Span: NodeSpan{10, 12}},
											{Type: ELSE_KEYWORD, Span: NodeSpan{21, 25}},
										},*/
									},
									Test: &BooleanLiteral{
										NodeBase: NodeBase{NodeSpan{13, 17}, nil, false},
										Value:    true,
									},
									Consequent: &Block{
										NodeBase: NodeBase{
											NodeSpan{18, 20},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
											},*/
										},
										Statements: nil,
									},
									Alternate: &Block{
										NodeBase: NodeBase{
											NodeSpan{26, 28},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{26, 27}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
											},*/
										},
										Statements: nil,
									},
								},
							},
						},
						Alternate: &Block{
							NodeBase: NodeBase{
								NodeSpan{36, 38},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{36, 37}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{37, 38}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("(multiline) if-else within an if-else statement", func(t *testing.T) {
			n := mustparseChunk(t, `
				if a {
					if true {

					} else {
						false
					}
				} else {
					b
				}
			`)

			outerIfStmt := n.Statements[0].(*IfStatement)
			assert.IsType(t, &IdentifierLiteral{}, outerIfStmt.Test)
			assert.IsType(t, &IdentifierLiteral{}, outerIfStmt.Alternate.(*Block).Statements[0])

			innerIfStmt := FindNode(outerIfStmt, &IfStatement{}, nil)
			assert.IsType(t, &BooleanLiteral{}, innerIfStmt.Test)
			assert.IsType(t, &BooleanLiteral{}, innerIfStmt.Alternate.(*Block).Statements[0])
		})

		t.Run("if-else-if", func(t *testing.T) {
			n := mustparseChunk(t, "if true { } else if true {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{Span: NodeSpan{0, 27}}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase:   NodeBase{Span: NodeSpan{8, 11}},
							Statements: nil,
						},
						Alternate: &IfStatement{
							NodeBase: NodeBase{Span: NodeSpan{17, 27}}, Test: &BooleanLiteral{
								NodeBase: NodeBase{NodeSpan{20, 24}, nil, false},
								Value:    true,
							},
							Consequent: &Block{
								NodeBase: NodeBase{Span: NodeSpan{25, 27}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("if-else-if<ident char>", func(t *testing.T) {
			n, err := parseChunk(t, "if true { } else if9", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{MissingBlock, fmtUnterminatedIfStmtElseShouldBeFollowedByBlock('i')},
							false,
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase:   NodeBase{Span: NodeSpan{8, 11}},
							Statements: nil,
						},
					},
					&IdentifierLiteral{
						NodeBase: NodeBase{Span: NodeSpan{17, 20}},
						Name:     "if9",
					},
				},
			}, n)
		})
	})

	t.Run("if expression", func(t *testing.T) {

		t.Run("(if <test> <consequent>)", func(t *testing.T) {
			n := mustparseChunk(t, "(if true 1)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							true,
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("(if <test> (missing value)", func(t *testing.T) {
			code := "(if true"

			n, err := parseChunk(t, code, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							true,
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
							Value:    true,
						},
						Consequent: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{7, 8},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune(code), 8, true)},
								false,
							},
						},
					},
				},
			}, n)
		})

		t.Run("(if <test> <consequent> (missing parenthesis)", func(t *testing.T) {
			n, err := parseChunk(t, "(if true 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN},
							true,
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("(if <test> <consequent> else <alternate>)", func(t *testing.T) {
			n := mustparseChunk(t, "(if true 1 else 2)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							true,
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						Alternate: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			}, n)
		})

		t.Run("(if <test> <consequent> else <alternate> (missing parenthesis)", func(t *testing.T) {
			n, err := parseChunk(t, "(if true 1 else 2", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN},
							true,
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						Alternate: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			}, n)
		})

		t.Run("(if <test> <consequent> else (missing vallue)", func(t *testing.T) {
			code := "(if true 1 else"
			n, err := parseChunk(t, code, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN},
							true,
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						Alternate: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{14, 15},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune(code), 15, true)},
								false,
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("for statement", func(t *testing.T) {
		t.Run("empty for <index>, <elem> ... in statement", func(t *testing.T) {
			n := mustparseChunk(t, "for i, u in $users { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{9, 11}},
							},*/
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 18}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{19, 22},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("empty for <index pattern> <index>, <elem> ... in statement", func(t *testing.T) {
			n := mustparseChunk(t, "for %even i, u in $users { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{11, 12}},
								{Type: IN_KEYWORD, Span: NodeSpan{15, 17}},
							},*/
						},
						KeyPattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 9}, nil, false},
							Name:     "even",
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{18, 24}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{25, 28},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("empty for <index pattern> <index>, <elem pattern> <elem> ... in statement", func(t *testing.T) {
			n := mustparseChunk(t, "for %even i, %p u in $users { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 31}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 31},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{11, 12}},
								{Type: IN_KEYWORD, Span: NodeSpan{18, 20}},
							},*/
						},
						KeyPattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 9}, nil, false},
							Name:     "even",
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
							Name:     "i",
						},
						ValuePattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{13, 15}, nil, false},
							Name:     "p",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{21, 27}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{28, 31},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{28, 29}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{30, 31}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("empty for <index>, <elem pattern> <elem> ... in statement", func(t *testing.T) {
			n := mustparseChunk(t, "for i, %p u in $users { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{12, 14}},
							},*/
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Name:     "i",
						},
						ValuePattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
							Name:     "p",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{15, 21}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{22, 25},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("empty for <elem> ... in statement", func(t *testing.T) {
			n := mustparseChunk(t, "for u in $users { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: IN_KEYWORD, Span: NodeSpan{6, 8}},
							},*/
						},
						KeyIndexIdent: nil,
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{9, 15}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{16, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("empty for <elem> ... in chunked statement", func(t *testing.T) {
			n := mustparseChunk(t, "for chunked u in $users { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 27},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: CHUNKED_KEYWORD, Span: NodeSpan{4, 11}},
								{Type: IN_KEYWORD, Span: NodeSpan{14, 16}},
							},*/
						},
						KeyIndexIdent: nil,
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
							Name:     "u",
						},
						Chunked: true,
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{17, 23}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{24, 27},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{26, 27}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("for .. in with break statement", func(t *testing.T) {
			n := mustparseChunk(t, "for i, u in $users { break }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{9, 11}},
							},*/
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 18}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{19, 28},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
								},*/
							},
							Statements: []Node{
								&BreakStatement{
									NodeBase: NodeBase{
										NodeSpan{21, 26},
										nil,
										false,
									},
									Label: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("for .. in with continue statement", func(t *testing.T) {
			n := mustparseChunk(t, "for i, u in $users { continue }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 31}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 31},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{9, 11}},
							},*/
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 18}, nil, false},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{19, 31},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{30, 31}},
								},*/
							},
							Statements: []Node{
								&ContinueStatement{
									NodeBase: NodeBase{
										NodeSpan{21, 29},
										nil,
										false,
									},
									Label: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("for <expr>", func(t *testing.T) {
			n := mustparseChunk(t, "for $array { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
							},*/
						},
						KeyIndexIdent:  nil,
						ValueElemIdent: nil,
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{4, 10}, nil, false},
							Name:     "array",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{11, 14},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("for <pattern>", func(t *testing.T) {
			n := mustparseChunk(t, "for %p { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
							},*/
						},
						KeyIndexIdent:  nil,
						ValueElemIdent: nil,
						IteratedValue: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
							Name:     "p",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 10},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

	})

	t.Run("for expression", func(t *testing.T) {
		t.Run("for <index>, <elem> ... in", func(t *testing.T) {
			n := mustparseChunk(t, "(for i, u in $users: i)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 23}, IsParenthesized: true},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{13, 19}, nil, false},
							Name:     "users",
						},
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{21, 22}, nil, false},
							Name:     "i",
						},
					},
				},
			}, n)
		})

		t.Run("for <index pattern> <index>, <elem> ... in", func(t *testing.T) {
			n := mustparseChunk(t, "(for %even i, u in $users: i)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 29}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 29}, IsParenthesized: true},
						KeyPattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 10}, nil, false},
							Name:     "even",
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{19, 25}, nil, false},
							Name:     "users",
						},
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{27, 28}, nil, false},
							Name:     "i",
						},
					},
				},
			}, n)
		})

		t.Run("for <index pattern> <index>, <elem pattern> <elem> ... in", func(t *testing.T) {
			n := mustparseChunk(t, "(for %even i, %p u in $users: i)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 32}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 32}, IsParenthesized: true},
						KeyPattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 10}, nil, false},
							Name:     "even",
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
							Name:     "i",
						},
						ValuePattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{14, 16}, nil, false},
							Name:     "p",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{17, 18}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{22, 28}, nil, false},
							Name:     "users",
						},
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{30, 31}, nil, false},
							Name:     "i",
						},
					},
				},
			}, n)
		})

		t.Run("for <index>, <elem pattern> <elem> ... in", func(t *testing.T) {
			n := mustparseChunk(t, "(for i, %p u in $users: i)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 26}, IsParenthesized: true},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "i",
						},
						ValuePattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 10}, nil, false},
							Name:     "p",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{16, 22}, nil, false},
							Name:     "users",
						},
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{24, 25}, nil, false},
							Name:     "i",
						},
					},
				},
			}, n)
		})

		t.Run("for <elem> ... in", func(t *testing.T) {
			n := mustparseChunk(t, "(for u in $users: i)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase:      NodeBase{Span: NodeSpan{0, 20}, IsParenthesized: true},
						KeyIndexIdent: nil,
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{10, 16}, nil, false},
							Name:     "users",
						},
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
							Name:     "i",
						},
					},
				},
			}, n)
		})

		t.Run("for <elem> ... in chunked", func(t *testing.T) {
			n := mustparseChunk(t, "(for chunked u in $users: u)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase:      NodeBase{Span: NodeSpan{0, 28}, IsParenthesized: true},
						KeyIndexIdent: nil,
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
							Name:     "u",
						},
						Chunked: true,
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{18, 24}, nil, false},
							Name:     "users",
						},
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{26, 27}, nil, false},
							Name:     "u",
						},
					},
				},
			}, n)
		})

		t.Run("missing body", func(t *testing.T) {
			n, err := parseChunk(t, "(for i, u in $users:)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 21}, IsParenthesized: true},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{13, 19}, nil, false},
							Name:     "users",
						},
						Body: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{20, 21},
								&ParsingError{UnspecifiedParsingError, "an expression was expected: ...sers:<<here>>)..."},
								false,
							},
						},
					},
				},
			}, n)
		})

		t.Run("misisng closing parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, "(for i, u in $users: i", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&ForExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_FOR_EXPR_MISSING_CLOSIN_PAREN},
							true,
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{13, 19}, nil, false},
							Name:     "users",
						},
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{21, 22}, nil, false},
							Name:     "i",
						},
					},
				},
			}, n)
		})
	})

	t.Run("walk statement", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, "walk ./ entry { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&WalkStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
						},
						Walked: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Raw:      "./",
							Value:    "./",
						},
						EntryIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 13}, nil, false},
							Name:     "entry",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{14, 17},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("meta & entry variable identifiers", func(t *testing.T) {
			n := mustparseChunk(t, "walk ./ meta, entry { }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&WalkStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
							/*[]Token{
								{Type: WALK_KEYWORD, Span: NodeSpan{0, 4}},
								{Type: COMMA, Span: NodeSpan{12, 13}},
							},*/
						},
						Walked: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Raw:      "./",
							Value:    "./",
						},
						MetaIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 12}, nil, false},
							Name:     "meta",
						},
						EntryIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{14, 19}, nil, false},
							Name:     "entry",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{20, 23},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})
	})

	t.Run("unary expression", func(t *testing.T) {

		t.Run("unary expression : boolean negate", func(t *testing.T) {
			n := mustparseChunk(t, "!true")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&UnaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							false,
							/*[]Token{
								{Type: EXCLAMATION_MARK, Span: NodeSpan{0, 1}},
							},*/
						},
						Operator: BoolNegate,
						Operand: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
							Value:    true,
						},
					},
				},
			}, n)
		})

		t.Run("unary expression: number negation", func(t *testing.T) {
			n := mustparseChunk(t, "- 2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&UnaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							nil,
							false,
						},
						Operator: NumberNegate,
						Operand: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			}, n)
		})

		t.Run("unary expression: variable negation", func(t *testing.T) {
			n := mustparseChunk(t, "- a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&UnaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							nil,
							false,
						},
						Operator: NumberNegate,
						Operand: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
							Name:     "a",
						},
					},
				},
			}, n)
		})

		t.Run("unary expression: parenthesized number negation", func(t *testing.T) {
			n := mustparseChunk(t, "(- 2)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&UnaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: MINUS, Span: NodeSpan{1, 2}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},*/
						},
						Operator: NumberNegate,
						Operand: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			}, n)
		})

	})

	t.Run("binary expression", func(t *testing.T) {

		t.Run("OR(bin ex 1, bin ex 2)", func(t *testing.T) {
			n := mustparseChunk(t, "(a > b or c > d)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{15, 16}},
							},*/
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 15},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
								Name:     "c",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
								Name:     "d",
							},
						},
					},
				},
			}, n)
		})

		t.Run("OR(bin ex 1, variable)", func(t *testing.T) {
			n := mustparseChunk(t, "(a > b or c)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Right: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("OR(variable, bin ex)", func(t *testing.T) {
			n := mustparseChunk(t, "(a or b > c)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{3, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Operator: Or,
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
							Name:     "a",
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{6, 11},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Name:     "b",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
								Name:     "c",
							},
						},
					},
				},
			}, n)
		})

		t.Run("OR(bin ex 1, bin ex 2, bin ex 3)", func(t *testing.T) {
			n := mustparseChunk(t, "(a > b or c > d or e > f)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{24, 25}},
							},*/
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 24},
								nil,
								false,
								/*[]Token{
									{Type: OR_KEYWORD, Span: NodeSpan{16, 18}},
								},*/
							},
							Operator: Or,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "c",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Name:     "d",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{19, 24},
									nil,
									false,
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
									Name:     "e",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{23, 24}, nil, false},
									Name:     "f",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("OR(var, bin ex 1, bin ex 2)", func(t *testing.T) {
			n := mustparseChunk(t, "(a or b > c or d > e)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{3, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{20, 21}},
							},*/
						},
						Operator: Or,
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
							Name:     "a",
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{6, 20},
								nil,
								false,
							},
							Operator: Or,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{6, 12},
									nil,
									false,
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Name:     "b",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "c",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{15, 20},
									nil,
									false,
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Name:     "d",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
									Name:     "e",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("OR(var 1, var 2, bin ex 1)", func(t *testing.T) {
			mustparseChunk(t, "(a or b or c > d)")
			//TODO: after the parsing of the chain modify the resulting output
			//in order for the AST to have the following shape (possible errors in spans):

			// assert.EqualValues(t, &Chunk{
			// 	NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
			// 	Statements: []Node{
			// 		&BinaryExpression{
			// 			NodeBase: NodeBase{
			// 				NodeSpan{0, 17},
			// 				nil,
			// 				[]Token{
			// 					{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
			// 					{Type: OR_KEYWORD, Span: NodeSpan{3, 5}},
			// 					{Type: CLOSING_PARENTHESIS, Span: NodeSpan{16, 17}},
			// 				},
			// 			},
			// 			Operator: Or,
			// 			Left: &IdentifierLiteral{
			// 				NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
			// 				Name:     "a",
			// 			},
			// 			Right: &BinaryExpression{
			// 				NodeBase: NodeBase{
			// 					NodeSpan{6, 20},
			// 					nil,
			// 					false,
			// 				},
			// 				Operator: Or,
			// 				Left: &IdentifierLiteral{
			// 					NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
			// 					Name:     "b",
			// 				},
			// 				Right: &BinaryExpression{
			// 					NodeBase: NodeBase{
			// 						NodeSpan{15, 20},
			// 						nil,
			// 						false,
			// 					},
			// 					Operator: GreaterThan,
			// 					Left: &IdentifierLiteral{
			// 						NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
			// 						Name:     "c",
			// 					},
			// 					Right: &IdentifierLiteral{
			// 						NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
			// 						Name:     "d",
			// 					},
			// 				},
			// 			},
			// 		},
			// 	},
			// }, n)
		})

		t.Run("OR(bin ex 1, AND(bin ex 2, bin ex 3))", func(t *testing.T) {
			n, err := parseChunk(t, "(a > b or c > d and e > f)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							&ParsingError{UnspecifiedParsingError, BIN_EXPR_CHAIN_OPERATORS_SHOULD_BE_THE_SAME},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{25, 26}},
							},*/
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 25},
								nil,
								false,
								/*[]Token{
									{Type: AND_KEYWORD, Span: NodeSpan{16, 19}},
								},*/
							},
							Operator: And,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "c",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Name:     "d",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{20, 25},
									nil,
									false,
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Name:     "e",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{24, 25}, nil, false},
									Name:     "f",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("OR(bin ex 1, AND(bin ex 2, bin ex 3), bin ex 4)", func(t *testing.T) {
			n, err := parseChunk(t, "(a > b or c > d and e > f or g > h)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 35}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 35},
							&ParsingError{UnspecifiedParsingError, BIN_EXPR_CHAIN_OPERATORS_SHOULD_BE_THE_SAME},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{34, 35}},
							},*/
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 34},
								nil,
								false,
							},
							Operator: And,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "c",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Name:     "d",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{20, 34},
									nil,
									false,
								},
								Operator: Or,
								Left: &BinaryExpression{
									NodeBase: NodeBase{
										NodeSpan{20, 26},
										nil,
										false,
									},
									Operator: GreaterThan,
									Left: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
										Name:     "e",
									},
									Right: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{24, 25}, nil, false},
										Name:     "f",
									},
								},
								Right: &BinaryExpression{
									NodeBase: NodeBase{
										NodeSpan{29, 34},
										nil,
										false,
									},
									Operator: GreaterThan,
									Left: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, false},
										Name:     "g",
									},
									Right: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{33, 34}, nil, false},
										Name:     "h",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("OR(bin ex 1, ...missing operand ", func(t *testing.T) {
			n, err := parseChunk(t, "(a > b or", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
							},*/
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Right: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{8, 9},
								&ParsingError{UnspecifiedParsingError, "an expression was expected: ... b or<<here>>..."},
								false,
							},
						},
					},
				},
			}, n)
		})

		t.Run("OR(bin ex 1, bin ex 2 <missing parenthesis>", func(t *testing.T) {
			n, err := parseChunk(t, "(a > b or c > d", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
							},*/
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 15},
								nil,
								false,
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
								Name:     "c",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
								Name:     "d",
							},
						},
					},
				},
			}, n)
		})

		t.Run("addition", func(t *testing.T) {
			n := mustparseChunk(t, "($a + $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Operator: Add,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("addition with first operand being an unparenthesized number negation", func(t *testing.T) {
			n := mustparseChunk(t, "(-$a + $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Operator: Add,
						Left: &UnaryExpression{
							NodeBase: NodeBase{
								NodeSpan{1, 4},
								nil,
								false,
							},
							Operator: NumberNegate,
							Operand: &Variable{
								NodeBase: NodeBase{NodeSpan{2, 4}, nil, false},
								Name:     "a",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("addition with second operand being an unparenthesized number negation", func(t *testing.T) {
			n := mustparseChunk(t, "($a + -$b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Operator: Add,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &UnaryExpression{
							NodeBase: NodeBase{
								NodeSpan{6, 9},
								nil,
								false,
							},
							Operator: NumberNegate,
							Operand: &Variable{
								NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("match with unprefixed pattern", func(t *testing.T) {
			n := mustparseChunk(t, "(o match {})")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: MATCH_KEYWORD, Span: NodeSpan{3, 8}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Operator: Match,
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
							Name:     "o",
						},
						Right: &ObjectPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("range", func(t *testing.T) {
			n := mustparseChunk(t, "($a .. $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: TWO_DOTS, Span: NodeSpan{4, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Operator: Range,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("exclusive end range", func(t *testing.T) {
			n := mustparseChunk(t, "($a ..< $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: DOT_DOT_LESS_THAN, Span: NodeSpan{4, 7}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},*/
						},
						Operator: ExclEndRange,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{8, 10}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("pair comma: space around operator", func(t *testing.T) {
			n := mustparseChunk(t, "($a , $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Operator: PairComma,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("pair comma: space only before operator", func(t *testing.T) {
			n := mustparseChunk(t, "($a ,$b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Operator: PairComma,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("pair comma: space only after operator", func(t *testing.T) {
			n := mustparseChunk(t, "($a, $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Operator: PairComma,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("pair comma: no space around operator", func(t *testing.T) {
			n := mustparseChunk(t, "($a,$b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Operator: PairComma,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("missing right operand", func(t *testing.T) {
			n, err := parseChunk(t, "($a +)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERAND},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},*/
						},
						Operator: Add,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{5, 6},
								&ParsingError{UnspecifiedParsingError, "an expression was expected: ...($a +<<here>>)..."},
								false,
							},
						},
					},
				},
			}, n)
		})
		t.Run("unexpected operator", func(t *testing.T) {
			n, err := parseChunk(t, "($a ? $b)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{4, 5}, Raw: "?"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("unexpected operator starting like an existing one", func(t *testing.T) {
			n, err := parseChunk(t, "($a ! $b)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{4, 5}, Raw: "!"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("unexpected operator starting like an existing one (no spaces)", func(t *testing.T) {
			n, err := parseChunk(t, "($a!$b)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{3, 4}, Raw: "!"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("unexpected word operator : <and>e", func(t *testing.T) {
			n, err := parseChunk(t, "($a ande $b)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{4, 8}, Raw: "ande"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{9, 11}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("missing operator", func(t *testing.T) {
			n, err := parseChunk(t, "($a$b)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERATOR},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},*/
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, false},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{3, 5}, nil, false},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("+ chain", func(t *testing.T) {
			_, err := parseChunk(t, "(1 + 2 + 3)", "")
			assert.ErrorContains(t, err, MOST_BINARY_EXPRS_MUST_BE_PARENTHESIZED)
		})

		t.Run("only opening parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, "(", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("("), 1, true)},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("opening parenthesis followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "(\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("(\n"), 2, true)},
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: NEWLINE, Span: NodeSpan{1, 2}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("opening parenthesis followed by an unexpected character", func(t *testing.T) {
			n, err := parseChunk(t, "(;", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParenthesizedExpression(';')},
							false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: UNEXPECTED_CHAR, Raw: ";", Span: NodeSpan{1, 2}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("missing expression in between parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, "()", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("()"), 1, true)},
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{1, 2}},
							},*/
						},
					},
				},
			}, n)
		})

	})

	t.Run("runtime typecheck expression", func(t *testing.T) {

		t.Run("variable", func(t *testing.T) {
			n := mustparseChunk(t, "~a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&RuntimeTypeCheckExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							nil,
							false,
							/*[]Token{
								{Type: TILDE, Span: NodeSpan{0, 1}},
							},*/
						},
						Expr: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
							Name:     "a",
						},
					},
				},
			}, n)
		})

		t.Run("missing expression", func(t *testing.T) {
			n, err := parseChunk(t, "~", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&RuntimeTypeCheckExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							nil,
							false,
							/*[]Token{
								{Type: TILDE, Span: NodeSpan{0, 1}},
							},*/
						},
						Expr: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("~"), 1, true)},
								false,
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("upper bound range expression", func(t *testing.T) {
		t.Run("integer", func(t *testing.T) {
			n := mustparseChunk(t, "..10")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&UpperBoundRangeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
						},
						UpperBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 4}, nil, false},
							Raw:      "10",
							Value:    10,
						},
					},
				},
			}, n)
		})

		t.Run("upper-bound expression should not start with '.'", func(t *testing.T) {
			n, err := parseChunk(t, ".../", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&UpperBoundRangeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, INVALID_UPPER_BOUND_RANGE_EXPR},
							false,
						},
						UpperBound: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{2, 4}, nil, false},
							Raw:      "./",
							Value:    "./",
						},
					},
				},
			}, n)
		})
	})

	t.Run("integer range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, "1..2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&IntegerRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						LowerBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						UpperBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			}, n)
		})

		t.Run("no upper bound", func(t *testing.T) {
			n := mustparseChunk(t, "1..")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&IntegerRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
						LowerBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("invalid upper bound", func(t *testing.T) {
			n, err := parseChunk(t, "1..$a", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&IntegerRangeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_INT_LIT},
							false,
						},
						LowerBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						UpperBound: &Variable{
							NodeBase: NodeBase{Span: NodeSpan{3, 5}},
							Name:     "a",
						},
					},
				},
			}, n)
		})
	})

	t.Run("float range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, "1.0..2.0")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&FloatRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						LowerBound: &FloatLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Raw:      "1.0",
							Value:    1.0,
						},
						UpperBound: &FloatLiteral{
							NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
							Raw:      "2.0",
							Value:    2.0,
						},
					},
				},
			}, n)
		})

		t.Run("no upper bound", func(t *testing.T) {
			n := mustparseChunk(t, "1.0..")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&FloatRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						LowerBound: &FloatLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Raw:      "1.0",
							Value:    1.0,
						},
					},
				},
			}, n)
		})

		t.Run("invalid upper bound", func(t *testing.T) {
			n, err := parseChunk(t, "1.0..$a", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&FloatRangeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_FLOAT_LIT},
							false,
						},
						LowerBound: &FloatLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Raw:      "1.0",
							Value:    1.0,
						},
						UpperBound: &Variable{
							NodeBase: NodeBase{Span: NodeSpan{5, 7}},
							Name:     "a",
						},
					},
				},
			}, n)
		})
	})

	t.Run("quantity range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, "1x..2x")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&QuantityRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						LowerBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Raw:      "1x",
							Values:   []float64{1},
							Units:    []string{"x"},
						},
						UpperBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, false},
							Raw:      "2x",
							Values:   []float64{2},
							Units:    []string{"x"},
						},
					},
				},
			}, n)
		})

		t.Run("no upper bound", func(t *testing.T) {
			n := mustparseChunk(t, "1x..")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&QuantityRangeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
						},
						LowerBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Raw:      "1x",
							Values:   []float64{1},
							Units:    []string{"x"},
						},
					},
				},
			}, n)
		})

		t.Run("invalid upper bound", func(t *testing.T) {
			n, err := parseChunk(t, "1x..$a", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&QuantityRangeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_BE_QTY_LIT},
							false,
						},
						LowerBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Raw:      "1x",
							Values:   []float64{1},
							Units:    []string{"x"},
						},
						UpperBound: &Variable{
							NodeBase: NodeBase{Span: NodeSpan{4, 6}},
							Name:     "a",
						},
					},
				},
			}, n)
		})

	})

	t.Run("rune range expression", func(t *testing.T) {
		t.Run("rune range expression", func(t *testing.T) {
			n := mustparseChunk(t, "'a'..'z'")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&RuneRangeExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Lower: &RuneLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Value:    'a',
						},
						Upper: &RuneLiteral{
							NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
							Value:    'z',
						},
					},
				},
			}, n)
		})

		//TODO: improve tests
		t.Run("invalid rune range expression : <rune> '.'", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "'a'.")
			})
		})

		t.Run("invalid rune range expression : <rune> '.' '.' ", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "'a'..")
			})
		})
	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("no parameters, no manifest, empty body", func(t *testing.T) {
			n := mustparseChunk(t, "fn(){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},*/
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{4, 6},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, no manifest, empty body, return type", func(t *testing.T) {
			n := mustparseChunk(t, "fn() %int {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},*/
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
							Name:     "int",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, no manifest, empty body, unprefixed return type", func(t *testing.T) {
			n := mustparseChunk(t, "fn() int {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},*/
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{5, 8}, nil, false},
							Unprefixed: true,
							Name:       "int",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("various accepted return types", func(t *testing.T) {
			_, err := parseChunk(t, "fn() [int] {}", "")
			assert.NoError(t, err)

			_, err = parseChunk(t, "fn() #[int] {}", "")
			assert.NoError(t, err)

			_, err = parseChunk(t, "fn() #{a: int} {}", "")
			assert.NoError(t, err)
		})

		t.Run("no parameters, empty capture list, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn[](){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},*/
						},
						CaptureList: nil,
						Parameters:  nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{6, 8},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, capture list with single identifier, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn[a](){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{4, 5}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						CaptureList: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "a",
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, capture list with two identifiers, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn[a,b](){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: COMMA, Span: NodeSpan{4, 5}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{6, 7}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{7, 8}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						CaptureList: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "a",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Name:     "b",
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, capture list with unexpected char, empty body ", func(t *testing.T) {
			n, err := parseChunk(t, "fn[?](){}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{4, 5}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						CaptureList: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{3, 4},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCaptureList('?')},
									false,
								},
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "x",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{5, 7},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single typed parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x %int){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 9}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
									Name:     "int",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single readonly typed parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x readonly %int){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{18, 19}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{Span: NodeSpan{3, 18}},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "x",
								},
								Type: &ReadonlyPatternExpression{
									NodeBase: NodeBase{
										NodeSpan{5, 18},
										nil,
										false,
									},
									Pattern: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{14, 18}, nil, false},
										Name:     "int",
									},
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{19, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single unprefix typed parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x int){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 8}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{5, 8}, nil, false},
									Unprefixed: true,
									Name:       "int",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("two parameters, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x,n){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: COMMA, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "x",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
									Name:     "n",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single parameter, body is an expression", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x) => x")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: ARROW, Span: NodeSpan{6, 8}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "x",
								},
							},
						},
						IsBodyExpression: true,
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Name:     "x",
						},
					},
				},
			}, n)
		})

		t.Run("return type, body is an expression", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x) int => x")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: ARROW, Span: NodeSpan{10, 12}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "x",
								},
							},
						},
						ReturnType: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{6, 9}, nil, false},
							Name:       "int",
							Unprefixed: true,
						},
						IsBodyExpression: true,
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
							Name:     "x",
						},
					},
				},
			}, n)
		})

		t.Run("only fn keyword", func(t *testing.T) {
			n, err := parseChunk(t, "fn", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{InvalidNext, FN_KEYWORD_OR_FUNC_NAME_SHOULD_BE_FOLLOWED_BY_PARAMS},
							false,
						},
						Parameters: nil,
						Body:       nil,
					},
				},
			}, n)
		})

		t.Run("missing block's closing brace", func(t *testing.T) {
			n, err := parseChunk(t, "fn(){", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},*/
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{4, 5},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("missing block's closing brace, trailing space", func(t *testing.T) {
			n, err := parseChunk(t, "fn(){ ", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},*/
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{4, 6},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("missing block's closing brace before closing parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, "(fn(){)", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase:   NodeBase{Span: NodeSpan{1, 6}, IsParenthesized: true},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{5, 6},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
								false,
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char in empty parameter list", func(t *testing.T) {
			n, err := parseChunk(t, "fn(:){}", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},*/
						},
						Parameters: nil,
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{3, 4},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									false,
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{5, 7},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char in non-empty parameter list", func(t *testing.T) {
			n, err := parseChunk(t, "fn(a:b){}", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "a",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
									Name:     "b",
								},
							},
						},
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{4, 5},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									false,
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("parameter list not followed by a block", func(t *testing.T) {
			n, err := parseChunk(t, "fn()1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW},
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},*/
						},
						Parameters: nil,
						Body:       nil,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("unterminated parameter list: end of module", func(t *testing.T) {
			n, err := parseChunk(t, "fn(", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN},
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
							},*/
						},
						Parameters: nil,
						Body:       nil,
					},
				},
			}, n)
		})

		t.Run("unterminated parameter list: followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "fn(\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN},
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: NEWLINE, Span: NodeSpan{3, 4}},
							},*/
						},
						Parameters: nil,
						Body:       nil,
					},
				},
			}, n)
		})

		t.Run("parameter name should not be a keyword ", func(t *testing.T) {
			n, err := parseChunk(t, "fn(manifest){}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
							/*[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 11},
									&ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES},
									false,
								},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, false},
									Name:     "manifest",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{12, 14},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},*/
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("unterminated arrow", func(t *testing.T) {
			//The error should be recoverable.
			n, err := parseChunk(t, "fn(x) =", "")
			assert.Error(t, err)
			assert.NotNil(t, n)
		})
	})

	t.Run("function declaration", func(t *testing.T) {
		t.Run("keyword name", func(t *testing.T) {
			res, err := parseChunk(t, "fn manifest(){}", "")
			assert.Error(t, err)
			assert.NotNil(t, res)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_AS_FN_NAMES)
		})
	})

	t.Run("function pattern expression", func(t *testing.T) {
		t.Run("no parameters", func(t *testing.T) {
			n := mustparseChunk(t, "%fn()")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},*/
						},
						Parameters: nil,
					},
				},
			}, n)
		})

		t.Run("no parameters, return type", func(t *testing.T) {
			n := mustparseChunk(t, "%fn() %int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},*/
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 10}, nil, false},
							Name:     "int",
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, empty body, unprefixed return type", func(t *testing.T) {
			n := mustparseChunk(t, "%fn() int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},*/
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{6, 9}, nil, false},
							Name:       "int",
							Unprefixed: true,
						},
					},
				},
			}, n)
		})

		t.Run("various accepted return types", func(t *testing.T) {
			_, err := parseChunk(t, "%fn() [int] {}", "")
			assert.NoError(t, err)

			_, err = parseChunk(t, "%fn() #[int] {}", "")
			assert.NoError(t, err)

			_, err = parseChunk(t, "%fn() #{a: int} {}", "")
			assert.NoError(t, err)
		})

		t.Run("single parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{4, 5}, nil, false},
									Name:       "x",
									Unprefixed: true,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single typed parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x %int)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 10}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 10}, nil, false},
									Name:     "int",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single readonly typed parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x readonly %int)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{19, 20}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{Span: NodeSpan{4, 19}},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "x",
								},
								Type: &ReadonlyPatternExpression{
									NodeBase: NodeBase{
										NodeSpan{6, 19},
										nil,
										false,
									},
									Pattern: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{15, 19}, nil, false},
										Name:     "int",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single typed parameter with unprefixed type, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x int)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 9}, nil, false},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 9}, nil, false},
									Name:       "int",
									Unprefixed: true,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single parameter with no name, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(%int)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
									Name:     "int",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two parameters, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x,n)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{4, 5}, nil, false},
									Name:       "x",
									Unprefixed: true,
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 7}, nil, false},
									Name:       "n",
									Unprefixed: true,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char in empty parameter list", func(t *testing.T) {
			n, err := parseChunk(t, "%fn(:)", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},*/
						},
						Parameters: nil,
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{4, 5},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char in non-empty parameter list", func(t *testing.T) {
			n, err := parseChunk(t, "%fn(a:b)", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{4, 5}, nil, false},
									Name:       "a",
									Unprefixed: true,
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 7}, nil, false},
									Name:       "b",
									Unprefixed: true,
								},
							},
						},
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{5, 6},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("parameter name should not be a keyword ", func(t *testing.T) {
			n, err := parseChunk(t, "%fn(manifest int)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
							},*/
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 16},
									&ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES},
									false,
								},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 12}, nil, false},
									Name:     "manifest",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{13, 16}, nil, false},
									Name:       "int",
									Unprefixed: true,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated arrow", func(t *testing.T) {
			//The error should be recoverable
			n, err := parseChunk(t, "%fn(x) =", "")
			assert.Error(t, err)
			assert.NotNil(t, n)
		})
	})

	t.Run("pattern conversion expression", func(t *testing.T) {
		n := mustparseChunk(t, "%(1)")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
			Statements: []Node{
				&PatternConversionExpression{
					NodeBase: NodeBase{
						NodeSpan{0, 3},
						nil,
						false,
						/*[]Token{
							{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
						},*/
					},
					Value: &IntLiteral{
						NodeBase: NodeBase{
							NodeSpan{2, 3},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},*/
						},
						Raw:   "1",
						Value: 1,
					},
				},
			},
		}, n)
	})

	t.Run("lazy expression", func(t *testing.T) {

		t.Run("integer value", func(t *testing.T) {
			n := mustparseChunk(t, "@(1)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&LazyExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
						},
						Expression: &IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								nil,
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},*/
							},
							Raw:   "1",
							Value: 1,
						},
					},
				},
			}, n)
		})

		t.Run("missing closing parenthesis ", func(t *testing.T) {
			n, err := parseChunk(t, "@(1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&LazyExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							nil,
							false,
						},
						Expression: &IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_PARENTHESIZED_EXPR_MISSING_CLOSING_PAREN},
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
								},*/
							},
							Raw:   "1",
							Value: 1,
						},
					},
				},
			}, n)
		})

		t.Run("lazy expression followed by another expression", func(t *testing.T) {
			n := mustparseChunk(t, "@(1) 2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&LazyExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
						},
						Expression: &IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								nil,
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},*/
							},
							Raw:   "1",
							Value: 1,
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
						Raw:      "2",
						Value:    2,
					},
				},
			}, n)
		})

	})
	t.Run("switch statement", func(t *testing.T) {

		testCases := []struct {
			input    string
			hasError bool
			result   Node
		}{
			{
				input:    "switch 1 { }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: nil,
						},
					},
				},
			},
			{
				input:    "switch 1 { 1 { } }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 16}, nil, false},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 16},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											},*/
										},
										Statements: nil,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { defaultcase { } }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 28},
								nil,
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{},
							DefaultCases: []*DefaultCase{
								{
									NodeBase: NodeBase{
										NodeSpan{11, 26},
										nil,
										false,
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{23, 26},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
											},*/
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { 1 { } 2 { } }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 24},
								nil,
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 16}, nil, false},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 16},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											},*/
										},
										Statements: nil,
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{17, 22}, nil, false},
									Values: []Node{

										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{17, 18}, nil, false},
											Raw:      "2",
											Value:    2,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{19, 22},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
											},*/
										},
										Statements: nil,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { 1, 2 { } }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 19}, nil, false},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Raw:      "1",
											Value:    1,
										},
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
											Raw:      "2",
											Value:    2,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{16, 19},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
											},*/
										},
										Statements: nil,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { 1 { }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 16},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 16}, nil, false},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 16},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											},*/
										},
										Statements: nil,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { defaultcase { }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 26},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{},
							DefaultCases: []*DefaultCase{
								{
									NodeBase: NodeBase{
										NodeSpan{11, 26},
										nil,
										false,
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{23, 26},
											nil,
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
											},*/
										},
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { 1 {",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, false},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 14},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
											},*/
										},
										Statements: nil,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { 1 { ",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								false,
								/*[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 15}, nil, false},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 15},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
											false,
											/*[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
											},*/
										},
										Statements: nil,
									},
								},
							},
						},
					},
				},
			},
			{
				input:    "switch 1 { ) }",
				hasError: true,
			},
			{
				input:    "switch 1 { % }",
				hasError: true,
			},
			{
				input:    "switch 1 { 1 ) }",
				hasError: true,
			},
			{
				input:    "switch 1 { 1 ) {} }",
				hasError: true,
			},
			{
				input:    "switch 1 { 1 {} ) }",
				hasError: true,
			},
			{
				input:    "switch 1 { $a { } }",
				hasError: true,
			},
			{
				input:    "switch 1 { defaultcase ) }",
				hasError: true,
			},
			{
				input:    "switch 1 { defaultcase ) {} }",
				hasError: true,
			},
			{
				input:    "switch 1 { defaultcase {} ) }",
				hasError: true,
			},
			{
				input:    "switch 1 { defaultcase {}\n defaultcase {} }",
				hasError: true,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, err := parseChunk(t, testCase.input, "")
				if testCase.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if testCase.result != nil {
					assert.Equal(t, testCase.result, n)
				}
			})
		}
	})

	t.Run("match statement", func(t *testing.T) {
		t.Run("case is not a simple literal and is not statically known", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "match 1 { $a { } }")
			})
		})

		t.Run("case is not a simple literal but is statically known", func(t *testing.T) {

			n := mustparseChunk(t, "match 1 { ({}) { } }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&MatchStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
							/*[]Token{
								{Type: MATCH_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
							},*/
						},
						Discriminant: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						Cases: []*MatchCase{
							{
								NodeBase: NodeBase{NodeSpan{10, 18}, nil, false},
								Values: []Node{
									&ObjectLiteral{
										NodeBase: NodeBase{
											NodeSpan{11, 13},
											nil,
											true,
											/*[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{10, 11}},
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
											},*/
										},
									},
								},
								Block: &Block{
									NodeBase: NodeBase{
										NodeSpan{15, 18},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
										},*/
									},
									Statements: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("case with group match variable", func(t *testing.T) {
			n := mustparseChunk(t, "match 1 { %/home/{:username} m { } }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 36}, nil, false},
				Statements: []Node{
					&MatchStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 36},
							nil,
							false,
							/*[]Token{
								{Type: MATCH_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
							},*/
						},
						Discriminant: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						Cases: []*MatchCase{
							{
								NodeBase: NodeBase{NodeSpan{10, 34}, nil, false},
								Values: []Node{
									&NamedSegmentPathPatternLiteral{
										NodeBase: NodeBase{
											NodeSpan{10, 28},
											nil,
											false,
											/*[]Token{
												{Type: PERCENT_SYMBOL, Span: NodeSpan{10, 11}},
												{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{17, 18}},
												{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{27, 28}},
											},*/
										},
										Slices: []Node{
											&PathPatternSlice{
												NodeBase: NodeBase{NodeSpan{11, 17}, nil, false},
												Value:    "/home/",
											},
											&NamedPathSegment{
												NodeBase: NodeBase{NodeSpan{18, 27}, nil, false},
												Name:     "username",
											},
										},
										Raw:         "%/home/{:username}",
										StringValue: "%/home/{:username}",
									},
								},
								GroupMatchingVariable: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{29, 30}, nil, false},
									Name:     "m",
								},
								Block: &Block{
									NodeBase: NodeBase{
										NodeSpan{31, 34},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{31, 32}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{33, 34}},
										},*/
									},
									Statements: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("group match variable should not be a keyword", func(t *testing.T) {
			n, err := parseChunk(t, "match 1 { %/home/{:username} manifest { } }", "")
			assert.NotNil(t, n)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("missing value before block of case", func(t *testing.T) {
			s := "match 1 { {} }"

			n, err := parseChunk(t, s, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&MatchStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
							/*[]Token{
								{Type: MATCH_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
							},*/
						},
						Discriminant: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						Cases: []*MatchCase{
							{
								NodeBase: NodeBase{NodeSpan{10, 12}, nil, false},
								Values: []Node{
									&MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{10, 11},
											&ParsingError{UnspecifiedParsingError, fmtCaseValueExpectedHere([]rune(s), 10, true)},
											false,
										},
									},
								},
								Block: &Block{
									NodeBase: NodeBase{
										NodeSpan{10, 12},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
										},*/
									},
									Statements: nil,
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("empty single line comment", func(t *testing.T) {
		n := mustparseChunk(t, "# ")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{
				NodeSpan{0, 2},
				nil,
				false,
			},
			Statements: nil,
		}, n)
	})

	t.Run("not empty single line comment", func(t *testing.T) {
		n := mustparseChunk(t, "# some text")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{
				NodeSpan{0, 11},
				nil,
				false,
			},
			Statements: nil,
		}, n)
	})

	t.Run("import statement", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, `import a https://example.com/a.ix {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 36}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 36},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{NodeSpan{9, 33}, nil, false},
							Value:    "https://example.com/a.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{34, 36},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid URL as source", func(t *testing.T) {
			n, err := parseChunk(t, `import res https://.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							&ParsingError{UnspecifiedParsingError, IMPORT_STMT_SRC_SHOULD_BE_AN_URL_OR_PATH_LIT},
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 10}, nil, false},
							Name:     "res",
						},
						Source: &InvalidURL{
							NodeBase: NodeBase{
								NodeSpan{11, 22},
								&ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST},
								false,
							},
							Value: "https://.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{23, 25},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid absolute path as source: presence of a '//' segment at the start", func(t *testing.T) {
			n, err := parseChunk(t, `import a //x.ix {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 15},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Raw:   "//x.ix",
							Value: "//x.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{16, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid absolute path as source: presence of a '//' segment", func(t *testing.T) {
			n, err := parseChunk(t, `import a /x//y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 17},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Raw:   "/x//y.ix",
							Value: "/x//y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{18, 20},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: presence of a '//' segment at the start", func(t *testing.T) {
			n, err := parseChunk(t, `import a .//x.ix {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 16},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Raw:   ".//x.ix",
							Value: ".//x.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: presence of a '//' segment", func(t *testing.T) {
			n, err := parseChunk(t, `import a ./x//y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 18},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Raw:   "./x//y.ix",
							Value: "./x//y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{19, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		//

		t.Run("invalid absolute path as source: presence of a '/../' segment at the start", func(t *testing.T) {
			n, err := parseChunk(t, `import a /../x.ix {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 17},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH},
								false,
							},
							Raw:   "/../x.ix",
							Value: "/../x.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{18, 20},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid absolute path as source: presence of a '/../' segment", func(t *testing.T) {
			n, err := parseChunk(t, `import a /x/../y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 19},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH},
								false,
							},
							Raw:   "/x/../y.ix",
							Value: "/x/../y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{20, 22},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: presence of a '/../' segment at the start", func(t *testing.T) {
			n, err := parseChunk(t, `import a ./../y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 18},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH},
								false,
							},
							Raw:   "./../y.ix",
							Value: "./../y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{19, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: path starting with ../", func(t *testing.T) {
			n, err := parseChunk(t, `import a ../y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 16},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH},
								false,
							},
							Raw:   "../y.ix",
							Value: "../y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: presence of a '/../' segment after a dirname", func(t *testing.T) {
			n, err := parseChunk(t, `import a ./x/../y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 20},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH},
								false,
							},
							Raw:   "./x/../y.ix",
							Value: "./x/../y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{21, 23},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		//

		t.Run("invalid absolute path as source: presence of a '/./' segment at the start", func(t *testing.T) {
			n, err := parseChunk(t, `import a /./x.ix {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 16},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS},
								false,
							},
							Raw:   "/./x.ix",
							Value: "/./x.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid absolute path as source: presence of a '/./' segment", func(t *testing.T) {
			n, err := parseChunk(t, `import a /x/./y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 18},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS},
								false,
							},
							Raw:   "/x/./y.ix",
							Value: "/x/./y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{19, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},*/
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: presence of a '/./' segment at the start", func(t *testing.T) {
			n, err := parseChunk(t, `import a ././y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 17},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS},
								false,
							},
							Raw:   "././y.ix",
							Value: "././y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{18, 20},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: presence of a '/./' segment after a dirname", func(t *testing.T) {
			n, err := parseChunk(t, `import a ./x/./y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 19},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS},
								false,
							},
							Raw:   "./x/./y.ix",
							Value: "./x/./y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{20, 22},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid absolute path as source: invalid file extension", func(t *testing.T) {
			n, err := parseChunk(t, `import a /y {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								&ParsingError{UnspecifiedParsingError, URL_LITS_AND_PATH_LITS_USED_AS_IMPORT_SRCS_SHOULD_END_WITH_IX},
								false,
							},
							Raw:   "/y",
							Value: "/y",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 14},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path as source: invalid file extension", func(t *testing.T) {
			n, err := parseChunk(t, `import a ./y {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "a",
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 12},
								&ParsingError{UnspecifiedParsingError, URL_LITS_AND_PATH_LITS_USED_AS_IMPORT_SRCS_SHOULD_END_WITH_IX},
								false,
							},
							Raw:   "./y",
							Value: "./y",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{13, 15},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		//

		t.Run("invalid URL path as source: presence of a '//' segment at the start", func(t *testing.T) {
			n, err := parseChunk(t, `import x https://example.com//x.ix {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 37}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 37},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "x",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 34},
								&ParsingError{UnspecifiedParsingError, PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Value: "https://example.com//x.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{35, 37},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid URL source: presence of a '//' segment in the path", func(t *testing.T) {
			n, err := parseChunk(t, `import x https://example.com/x//y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 39}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 39},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "x",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 36},
								&ParsingError{UnspecifiedParsingError, PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Value: "https://example.com/x//y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{37, 39},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid URL as source: presence of a '/../' segment at the start of the path", func(t *testing.T) {
			n, err := parseChunk(t, `import x https://example.com/../x.ix {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 39}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 39},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "x",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 36},
								&ParsingError{UnspecifiedParsingError, PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH},
								false,
							},
							Value: "https://example.com/../x.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{37, 39},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid URL path as source: presence of a '/../' segment in the path", func(t *testing.T) {
			n, err := parseChunk(t, `import x https://example.com/x/../y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 41}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 41},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "x",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 38},
								&ParsingError{UnspecifiedParsingError, PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SLASHSLASH},
								false,
							},
							Value: "https://example.com/x/../y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{39, 41},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid URL as source: presence of a '/./' segment at the start of the path", func(t *testing.T) {
			n, err := parseChunk(t, `import x https://example.com/./x.ix {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 38}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 38},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "x",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 35},
								&ParsingError{UnspecifiedParsingError, PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS},
								false,
							},
							Value: "https://example.com/./x.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{36, 38},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid URL as source: presence of a '/./' segment", func(t *testing.T) {
			n, err := parseChunk(t, `import x https://example.com/x/./y.ix {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 40}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 40},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "x",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 37},
								&ParsingError{UnspecifiedParsingError, PATH_OF_URL_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_DOT_SEGMENTS},
								false,
							},
							Value: "https://example.com/x/./y.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{38, 40},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})

		t.Run("invalid URL as source: invalid file extension", func(t *testing.T) {
			n, err := parseChunk(t, `import x https://example.com/x {}`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 33}, nil, false},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 33},
							nil,
							false,
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Name:     "x",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{
								NodeSpan{9, 30},
								&ParsingError{UnspecifiedParsingError, URL_LITS_AND_PATH_LITS_USED_AS_IMPORT_SRCS_SHOULD_END_WITH_IX},
								false,
							},
							Value: "https://example.com/x",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{31, 33},
								nil,
								false,
							},
							Properties: nil,
						},
					},
				},
			}, n)
		})
	})

	t.Run("inclusion import statement", func(t *testing.T) {
		t.Run("relative path literal", func(t *testing.T) {
			n := mustparseChunk(t, `import ./file.ix`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&InclusionImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
							Value:    "./file.ix",
							Raw:      "./file.ix",
						},
					},
				},
			}, n)
		})

		t.Run("invalid relative path literal", func(t *testing.T) {
			//we only check a single bad case because the same logic is used for module imports.

			n, err := parseChunk(t, `import .//file.ix`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&InclusionImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{7, 17},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Value: ".//file.ix",
							Raw:   ".//file.ix",
						},
					},
				},
			}, n)
		})

		t.Run("absolute path literal", func(t *testing.T) {
			n := mustparseChunk(t, `import /file.ix`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&InclusionImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{NodeSpan{7, 15}, nil, false},
							Value:    "/file.ix",
							Raw:      "/file.ix",
						},
					},
				},
			}, n)
		})

		t.Run("invalid absolute path literal", func(t *testing.T) {
			//we only check a single bad case because the same logic is used for module imports.

			n, err := parseChunk(t, `import //file.ix`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&InclusionImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},*/
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{
								NodeSpan{7, 16},
								&ParsingError{UnspecifiedParsingError, PATH_LITERALS_USED_AS_IMPORT_SRCS_SHOULD_NOT_CONTAIN_SLASHSLASH},
								false,
							},
							Value: "//file.ix",
							Raw:   "//file.ix",
						},
					},
				},
			}, n)
		})
	})

	t.Run("spawn expression", func(t *testing.T) {
		t.Run("call expression: called is an identifier literal", func(t *testing.T) {
			n := mustparseChunk(t, `go nil do f()`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},*/
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
						},
						Module: &EmbeddedModule{
							NodeBase:       NodeBase{NodeSpan{10, 13}, nil, false},
							SingleCallExpr: true,
							Statements: []Node{
								&CallExpression{
									NodeBase: NodeBase{
										NodeSpan{10, 13},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{11, 12}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
										},*/
									},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
										Name:     "f",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("call expression: calee is an identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, `go nil do http.read()`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
						},
						Module: &EmbeddedModule{
							NodeBase:       NodeBase{NodeSpan{10, 21}, nil, false},
							SingleCallExpr: true,
							Statements: []Node{
								&CallExpression{
									NodeBase: NodeBase{
										NodeSpan{10, 21},
										nil,
										false,
									},
									Callee: &IdentifierMemberExpression{
										NodeBase: NodeBase{NodeSpan{10, 19}, nil, false},
										Left: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{10, 14}, nil, false},
											Name:     "http",
										},
										PropertyNames: []*IdentifierLiteral{
											{
												NodeBase: NodeBase{NodeSpan{15, 19}, nil, false},
												Name:     "read",
											},
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("embedded module", func(t *testing.T) {
			n := mustparseChunk(t, `go nil do { manifest {} }`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
							/*[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},*/
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 25},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},*/
							},
							Manifest: &Manifest{
								NodeBase: NodeBase{
									Span:            NodeSpan{12, 23},
									IsParenthesized: false,
									/*[]Token{
										{Type: MANIFEST_KEYWORD, Span: NodeSpan{12, 20}},
									},*/
								},
								Object: &ObjectLiteral{
									NodeBase: NodeBase{
										NodeSpan{21, 23},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
										},*/
									},
									Properties: nil,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("statements next to each other in embedded module", func(t *testing.T) {
			n, err := parseChunk(t, `go nil do { 1$v }`, "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
							/*[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},*/
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 17},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},*/
							},
							Statements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
									Raw:      "1",
									Value:    1,
								},
								&Variable{
									NodeBase: NodeBase{
										NodeSpan{13, 15},
										&ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY},
										false,
									},
									Name: "v",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing expression/module after 'do' keyword", func(t *testing.T) {
			n, err := parseChunk(t, `go nil do`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_DO_KEYWORD},
							false,
							/*[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},*/
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
						},
					},
				},
			}, n)
		})

	})

	t.Run("mapping expression", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, `Mapping {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("empty, missing closing brace", func(t *testing.T) {
			n, err := parseChunk(t, `Mapping {`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE},
							false,
							/*[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("empty, missing closing brace before closing parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, `(Mapping {)`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 10},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE},
							true,
						},
					},
				},
			}, n)
		})

		t.Run("empty, missing closing brace before closing bracket", func(t *testing.T) {
			_, err := parseChunk(t, `[Mapping {]`, "")
			assert.ErrorContains(t, err, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE)
		})

		t.Run("static entry", func(t *testing.T) {
			n := mustparseChunk(t, "Mapping { 0 => 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 18}},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "0",
									Value:    0,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("static entry: missing closing brace before closing parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, `(Mapping { 0 => 1)`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 17},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE},
							true,
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{11, 17},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
									Raw:      "0",
									Value:    0,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("static entry: missing closing brace before closing bracket", func(t *testing.T) {
			_, err := parseChunk(t, `[Mapping { 0 => 1]`, "")
			assert.ErrorContains(t, err, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE)
		})

		t.Run("static entry: missing closing brace", func(t *testing.T) {
			n, err := parseChunk(t, "Mapping { 0 => 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE},
							false,
							/*[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
							},*/
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "0",
									Value:    0,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("static entry: missing value", func(t *testing.T) {
			n, err := parseChunk(t, "Mapping { 0 => }", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},*/
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "0",
									Value:    0,
								},
								Value: &MissingExpression{
									NodeBase: NodeBase{
										NodeSpan{15, 16},
										&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("Mapping { 0 => }"), 15, true)},
										false,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two static entries", func(t *testing.T) {
			n := mustparseChunk(t, "Mapping { 0 => 1    2 => 3 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							false,
							/*[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
							},*/
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "0",
									Value:    0,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{20, 26},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Raw:      "2",
									Value:    2,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{25, 26}, nil, false},
									Raw:      "3",
									Value:    3,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("dynamic entry", func(t *testing.T) {
			n := mustparseChunk(t, "Mapping { n 0 => n }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
							/*[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
							},*/
						},
						Entries: []Node{
							&DynamicMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 18},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
									Raw:      "0",
									Value:    0,
								},
								KeyVar: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "n",
								},
								ValueComputation: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{17, 18}, nil, false},
									Name:     "n",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("dynamic entry var should not be a keyword", func(t *testing.T) {
			n, err := parseChunk(t, "Mapping { manifest 0 => n }", "")
			assert.NotNil(t, n)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("dynamic entry with group matching variable", func(t *testing.T) {
			n := mustparseChunk(t, "Mapping { p %/ m => m }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							false,
							/*[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
							},*/
						},
						Entries: []Node{
							&DynamicMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 21},
									nil,
									false,
								},
								Key: &AbsolutePathPatternLiteral{
									NodeBase: NodeBase{NodeSpan{12, 14}, nil, false},
									Raw:      "%/",
									Value:    "/",
								},
								KeyVar: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Name:     "p",
								},
								GroupMatchingVariable: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Name:     "m",
								},
								ValueComputation: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Name:     "m",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("group matching variable should not be a keyword", func(t *testing.T) {
			n, err := parseChunk(t, "Mapping { p %/ manifest => m  }", "")
			assert.NotNil(t, n)
			assert.ErrorContains(t, err, KEYWORDS_SHOULD_NOT_BE_USED_IN_ASSIGNMENT_LHS)
		})

		t.Run("dynamic entry: missing closing brace before closing parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, `(Mapping { n 0 => n)`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 19},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE},
							true,
						},
						Entries: []Node{
							&DynamicMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{11, 19},
									nil,
									false,
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "0",
									Value:    0,
								},
								KeyVar: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
									Name:     "n",
								},
								ValueComputation: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
									Name:     "n",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("dynamic entry: missing closing brace before closing bracket", func(t *testing.T) {
			_, err := parseChunk(t, `[Mapping { n 0 => n]`, "")
			assert.ErrorContains(t, err, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE)
		})

	})

	t.Run("treedata expression", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, `treedata 0 {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("empty, missing closing brace", func(t *testing.T) {
			n, err := parseChunk(t, `treedata 0 {`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_TREEDATA_LIT_MISSING_CLOSING_BRACE},
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("empty, missing closing brace before closing parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, `(treedata 0 {)`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{1, 13},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_TREEDATA_LIT_MISSING_CLOSING_BRACE},
							true,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("single entry with children", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0 {} }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{
									NodeSpan{13, 17},
									nil,
									false,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "0",
									Value:    0,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single entry without children", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{NodeSpan{13, 15}, nil, false},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "0",
									Value:    0,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing closing brace after entry and before closing parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, "(treedata 0 { 0 )", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{1, 16},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_TREEDATA_LIT_MISSING_CLOSING_BRACE},
							true,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{NodeSpan{14, 16}, nil, false},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
									Raw:      "0",
									Value:    0,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single pair entry: space around colon", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0 : 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{NodeSpan{13, 18}, nil, false},
								Value: &TreedataPair{
									NodeBase: NodeBase{NodeSpan{13, 18}, nil, false},
									Key: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
										Raw:      "0",
										Value:    0,
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{17, 18}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single pair entry: no space around colon", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0:1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
								Value: &TreedataPair{
									NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
									Key: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
										Raw:      "0",
										Value:    0,
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single pair entry: space before colon", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0 :1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{NodeSpan{13, 17}, nil, false},
								Value: &TreedataPair{
									NodeBase: NodeBase{NodeSpan{13, 17}, nil, false},
									Key: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
										Raw:      "0",
										Value:    0,
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{16, 17}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two entries", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0 {} 1 {} }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{
									NodeSpan{13, 17},
									nil,
									false,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "0",
									Value:    0,
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{18, 22},
									nil,
									false,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two entries separated by a comma", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0 {}, 1 {} }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{
									NodeSpan{13, 17},
									nil,
									false,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "0",
									Value:    0,
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{19, 23},
									nil,
									false,
									/*[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									},*/
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{19, 20}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two entries without braces", func(t *testing.T) {
			n := mustparseChunk(t, "treedata 0 { 0 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&TreedataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
							Raw:      "0",
							Value:    0,
						},
						Children: []*TreedataEntry{
							{
								NodeBase: NodeBase{NodeSpan{13, 15}, nil, false},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
									Raw:      "0",
									Value:    0,
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{15, 17}, nil, false},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("testsuite expression", func(t *testing.T) {
		t.Run("no meta", func(t *testing.T) {
			n := mustparseChunk(t, `testsuite {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("with meta", func(t *testing.T) {
			n := mustparseChunk(t, `testsuite "name" {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Meta: &QuotedStringLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{10, 16},
							},
							Raw:   `"name"`,
							Value: "name",
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("embedded module with manifest", func(t *testing.T) {
			n := mustparseChunk(t, `testsuite { manifest {} }`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&TestSuiteExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
						},
						IsStatement: true,
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 25},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},*/
							},
							Manifest: &Manifest{
								NodeBase: NodeBase{
									Span:            NodeSpan{12, 23},
									IsParenthesized: false,
									/*[]Token{
										{Type: MANIFEST_KEYWORD, Span: NodeSpan{12, 20}},
									},*/
								},
								Object: &ObjectLiteral{
									NodeBase: NodeBase{
										NodeSpan{21, 23},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
										},*/
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing embedded module and no meta", func(t *testing.T) {
			n, err := parseChunk(t, `testsuite`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("with meta but missing embedded module", func(t *testing.T) {
			n, err := parseChunk(t, `testsuite "name"`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
							false,
						},
						Meta: &QuotedStringLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{10, 16},
							},
							Raw:   `"name"`,
							Value: "name",
						},
					},
				},
			}, n)
		})

	})

	t.Run("testcase expression", func(t *testing.T) {
		t.Run("no meta", func(t *testing.T) {
			n := mustparseChunk(t, `testcase {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&TestCaseExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
						},
						IsStatement: true,
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("with meta", func(t *testing.T) {
			n := mustparseChunk(t, `testcase "name" {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&TestCaseExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
						},
						Meta: &QuotedStringLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{9, 15},
							},
							Raw:   `"name"`,
							Value: "name",
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{16, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing embedded module and no meta", func(t *testing.T) {
			n, err := parseChunk(t, `testcase`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&TestCaseExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{MissingBlock, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("with meta but missing embedded module", func(t *testing.T) {
			n, err := parseChunk(t, `testcase "name"`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&TestCaseExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
							false,
						},
						Meta: &QuotedStringLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{9, 15},
							},
							Raw:   `"name"`,
							Value: "name",
						},
					},
				},
			}, n)
		})

	})

	t.Run("lifetimejob expression", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, `lifetimejob #job {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
						},
						Meta: &UnambiguousIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{12, 16}},
							Name:     "job",
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing meta", func(t *testing.T) {
			n, err := parseChunk(t, `lifetimejob`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_META},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("missing embedded module after meta", func(t *testing.T) {
			n, err := parseChunk(t, `lifetimejob #job`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_EMBEDDED_MODULE},
							false,
						},
						Meta: &UnambiguousIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{12, 16}},
							Name:     "job",
						},
					},
				},
			}, n)
		})

		t.Run("with subject", func(t *testing.T) {
			n := mustparseChunk(t, `lifetimejob #job for %p {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							false,
							/*[]Token{
								{Type: LIFETIMEJOB_KEYWORD, Span: NodeSpan{0, 11}},
								{Type: FOR_KEYWORD, Span: NodeSpan{17, 20}},
							},*/
						},
						Meta: &UnambiguousIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{12, 16}},
							Name:     "job",
						},
						Subject: &PatternIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{21, 23}},
							Name:     "p",
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{24, 26},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing embedded module after subject", func(t *testing.T) {
			n, err := parseChunk(t, `lifetimejob #job for %p`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_EMBEDDED_MODULE},
							false,
							/*[]Token{
								{Type: LIFETIMEJOB_KEYWORD, Span: NodeSpan{0, 11}},
								{Type: FOR_KEYWORD, Span: NodeSpan{17, 20}},
							},*/
						},
						Meta: &UnambiguousIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{12, 16}},
							Name:     "job",
						},
						Subject: &PatternIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{21, 23}},
							Name:     "p",
						},
					},
				},
			}, n)
		})
	})

	t.Run("reception handler expression", func(t *testing.T) {

		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, `on received %event h`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&ReceptionHandlerExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							false,
							/*[]Token{
								{Type: ON_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: RECEIVED_KEYWORD, Span: NodeSpan{3, 11}},
							},*/
						},

						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{12, 18}},
							Name:     "event",
						},
						Handler: &IdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{19, 20}},
							Name:     "h",
						},
					},
				},
			}, n)
		})

		t.Run("missing pattern", func(t *testing.T) {
			n, err := parseChunk(t, `on received`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&ReceptionHandlerExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_PATTERN},
							false,
							/*[]Token{
								{Type: ON_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: RECEIVED_KEYWORD, Span: NodeSpan{3, 11}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("missing body after 'do' keyword", func(t *testing.T) {
			n, err := parseChunk(t, `on received %event`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&ReceptionHandlerExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_HANDLER_OR_PATTERN},
							false,
							/*[]Token{
								{Type: ON_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: RECEIVED_KEYWORD, Span: NodeSpan{3, 11}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{12, 18}},
							Name:     "event",
						},
					},
				},
			}, n)
		})
	})

	//TODO: test sendval expression

	t.Run("compute expression", func(t *testing.T) {
		t.Run("missing expr", func(t *testing.T) {
			n, err := parseChunk(t, `comp`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&ComputeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
						},
						Arg: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{3, 4},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("comp"), 4, true)},
								false,
							},
						},
					},
				},
			}, n)
		})

		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, `comp 1`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ComputeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
						},
						Arg: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})
	})

	t.Run("permission dropping statement", func(t *testing.T) {
		t.Run("empty object literal", func(t *testing.T) {
			n := mustparseChunk(t, "drop-perms {}")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&PermissionDroppingStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
						},
						Object: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{11, 13},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("value is not an object literal", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "drop-perms 1")
			})
		})

		t.Run("value is not an object literal", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "drop-perms")
			})
		})

	})

	t.Run("return statement", func(t *testing.T) {
		t.Run("value", func(t *testing.T) {
			n := mustparseChunk(t, "return 1")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&ReturnStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
						},
						Expr: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("no value", func(t *testing.T) {
			n := mustparseChunk(t, "return")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ReturnStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
						},
					},
				},
			}, n)
		})

		t.Run("no value, followed by line feed", func(t *testing.T) {
			n := mustparseChunk(t, "return\n")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 7},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{6, 7}},
					},*/
				},
				Statements: []Node{
					&ReturnStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
						},
					},
				},
			}, n)
		})

	})

	t.Run("yield statement", func(t *testing.T) {
		t.Run("value", func(t *testing.T) {
			n := mustparseChunk(t, "yield 1")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&YieldStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
						},
						Expr: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("no value", func(t *testing.T) {
			n := mustparseChunk(t, "yield")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&YieldStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							false,
						},
					},
				},
			}, n)
		})

		t.Run("no value, followed by line feed", func(t *testing.T) {
			n := mustparseChunk(t, "yield\n")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 6},
					nil,
					false,
					/*[]Token{
						{Type: NEWLINE, Span: NodeSpan{5, 6}},
					},*/
				},
				Statements: []Node{
					&YieldStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							false,
						},
					},
				},
			}, n)
		})

	})

	t.Run("boolean conversion expression", func(t *testing.T) {
		t.Run("variable", func(t *testing.T) {
			n := mustparseChunk(t, "$err?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Expr: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "err",
						},
					},
				},
			}, n)
		})

		t.Run("identifier", func(t *testing.T) {
			n := mustparseChunk(t, "err?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Expr: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Name:     "err",
						},
					},
				},
			}, n)
		})

		t.Run("identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.b?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Expr: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "b",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("member expression", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Expr: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("optional member expression", func(t *testing.T) {
			n := mustparseChunk(t, "$a.?b?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Expr: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Name:     "b",
							},
							Optional: true,
						},
					},
				},
			}, n)
		})

		t.Run("optional member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.?b?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Expr: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     "b",
							},
							Optional: true,
						},
					},
				},
			}, n)
		})
	})

	t.Run("concatenation expression", func(t *testing.T) {
		t.Run("missing elements: end of chunk", func(t *testing.T) {
			n, err := parseChunk(t, `concat`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_CONCAT_EXPR_ELEMS_EXPECTED},
							false,
						},
						Elements: nil,
					},
				},
			}, n)
		})

		t.Run("missing elements: line feed", func(t *testing.T) {
			n, err := parseChunk(t, "concat\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 7},
					nil,
					false,
				},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_CONCAT_EXPR_ELEMS_EXPECTED},
							false,
						},
						Elements: nil,
					},
				},
			}, n)
		})

		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, `concat "a"`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
						},
						Elements: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{Span: NodeSpan{7, 10}},
								Raw:      `"a"`,
								Value:    "a",
							},
						},
					},
				},
			}, n)
		})
		t.Run("two elements", func(t *testing.T) {
			n := mustparseChunk(t, `concat "a" "b"`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
						},
						Elements: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{Span: NodeSpan{7, 10}},
								Raw:      `"a"`,
								Value:    "a",
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{Span: NodeSpan{11, 14}},
								Raw:      `"b"`,
								Value:    "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("expression is followed by a comma in a list", func(t *testing.T) {
			n := mustparseChunk(t, `[concat "a" "b", "c"]`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&ListLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
								{Type: COMMA, Span: NodeSpan{15, 16}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{20, 21}},
							},*/
						},
						Elements: []Node{
							&ConcatenationExpression{
								NodeBase: NodeBase{
									NodeSpan{1, 15},
									nil,
									false,
								},
								Elements: []Node{
									&QuotedStringLiteral{
										NodeBase: NodeBase{Span: NodeSpan{8, 11}},
										Raw:      `"a"`,
										Value:    "a",
									},
									&QuotedStringLiteral{
										NodeBase: NodeBase{Span: NodeSpan{12, 15}},
										Raw:      `"b"`,
										Value:    "b",
									},
								},
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{Span: NodeSpan{17, 20}},
								Raw:      `"c"`,
								Value:    "c",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized with a linefeed after the keyword", func(t *testing.T) {
			n := mustparseChunk(t, "(concat\na)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 9},
							nil,
							true,
						},
						Elements: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{Span: NodeSpan{8, 9}},
								Name:     "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized with a comment and linefeed after the keyword", func(t *testing.T) {
			n := mustparseChunk(t, "(concat # comment\na)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 19},
							nil,
							true,
						},
						Elements: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{Span: NodeSpan{18, 19}},
								Name:     "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("spread element", func(t *testing.T) {
			n := mustparseChunk(t, `concat ...a`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
						},
						Elements: []Node{
							&ElementSpreadElement{
								NodeBase: NodeBase{
									Span:            NodeSpan{7, 11},
									IsParenthesized: false,
								},
								Expr: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{10, 11}},
									Name:     "a",
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern identifier literal", func(t *testing.T) {
		t.Run("pattern identifier literal", func(t *testing.T) {
			n := mustparseChunk(t, "%int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&PatternIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Name:     "int",
					},
				},
			}, n)
		})

		t.Run("percent only", func(t *testing.T) {
			n, err := parseChunk(t, "%", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("percent followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "%\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					false,
				},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
							false,
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern namespace identifier literal", func(t *testing.T) {
		n := mustparseChunk(t, "%mynamespace.")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
			Statements: []Node{
				&PatternNamespaceIdentifierLiteral{
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
					Name:     "mynamespace",
				},
			},
		}, n)
	})

	t.Run("object pattern", func(t *testing.T) {

		// t.Run("{ ... } ", func(t *testing.T) {
		// 	n := mustparseChunk(t,"%{ ... }")
		// 	assert.EqualValues(t, &Chunk{
		// 		NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
		// 		Statements: []Node{
		// 			&ObjectPatternLiteral{
		// 				NodeBase: NodeBase{
		// 					NodeSpan{0, 8},
		// 					nil,
		// 					[]Token{
		// 						{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
		// 						{Type: THREE_DOTS, Span: NodeSpan{3, 6}},
		// 						{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
		// 					},
		// 				},
		//
		// 			},
		// 		},
		// 	}, n)
		// })

		// t.Run("{ ... , name: %str } ", func(t *testing.T) {
		// 	n := mustparseChunk(t,"%{ ... , name: %str }")
		// 	assert.EqualValues(t, &Chunk{
		// 		NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
		// 		Statements: []Node{
		// 			&ObjectPatternLiteral{
		// 				NodeBase: NodeBase{
		// 					NodeSpan{0, 21},
		// 					nil,
		// 					[]Token{
		// 						{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
		// 						{Type: THREE_DOTS, Span: NodeSpan{3, 6}},
		// 						{Type: COMMA, Span: NodeSpan{7, 8}},
		// 						{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
		// 					},
		// 				},
		//
		// 				Properties: []*ObjectPatternProperty{
		// 					{
		// 						NodeBase: NodeBase{
		// 							NodeSpan{9, 19},
		// 							nil,
		// 							false,
		// 						},
		// 						Key: &IdentifierLiteral{
		// 							NodeBase: NodeBase{NodeSpan{9, 13}, nil, false},
		// 							Name:     "name",
		// 						},
		// 						Value: &PatternIdentifierLiteral{
		// 							NodeBase: NodeBase{NodeSpan{15, 19}, nil, false},
		// 							Name:     "str",
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	}, n)
		// })

		// t.Run("{ ... \n } ", func(t *testing.T) {
		// 	n := mustparseChunk(t,"%{ ... \n }")
		// 	assert.EqualValues(t, &Chunk{
		// 		NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
		// 		Statements: []Node{
		// 			&ObjectPatternLiteral{
		// 				NodeBase: NodeBase{
		// 					NodeSpan{0, 10},
		// 					nil,
		// 					[]Token{
		// 						{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
		// 						{Type: THREE_DOTS, Span: NodeSpan{3, 6}},
		// 						{Type: NEWLINE, Span: NodeSpan{7, 8}},
		// 						{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
		// 					},
		// 				},
		//
		// 			},
		// 		},
		// 	}, n)
		// })

		t.Run("{ ...named-pattern } ", func(t *testing.T) {
			n := mustparseChunk(t, "%{ ...%patt }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
							},*/
						},

						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 11},
									nil,
									false,
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 11}, nil, false},
									Name:     "patt",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("{ ...unprefixed named-pattern } ", func(t *testing.T) {
			n := mustparseChunk(t, "%{ ...patt }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
							},*/
						},

						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 10},
									nil,
									false,
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 10}, nil, false},
									Unprefixed: true,
									Name:       "patt",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("{ prop, ...named-pattern } ", func(t *testing.T) {
			n, err := parseChunk(t, "%{ name: %str,  ...%patt }", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{13, 14}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
							},*/
						},

						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 13},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 13}, nil, false},
									Name:     "str",
								},
							},
						},
						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{16, 24},
									&ParsingError{UnspecifiedParsingError, SPREAD_SHOULD_BE_LOCATED_AT_THE_START},
									false,
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{19, 24}, nil, false},
									Name:     "patt",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("{ prop with unprefixed named pattern, ...named-pattern } ", func(t *testing.T) {
			n, err := parseChunk(t, "%{ name: str,  ...%patt }", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{12, 13}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
							},*/
						},

						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 12},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{9, 12}, nil, false},
									Unprefixed: true,
									Name:       "str",
								},
							},
						},
						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{15, 23},
									&ParsingError{UnspecifiedParsingError, SPREAD_SHOULD_BE_LOCATED_AT_THE_START},
									false,
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{18, 23}, nil, false},
									Name:     "patt",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("{  ...named-pattern, prop with unprefixed named pattern } ", func(t *testing.T) {
			n := mustparseChunk(t, "%{ ...%patt, name: str }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{11, 12}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
							},*/
						},

						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{13, 22},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{13, 17}, nil, false},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{19, 22}, nil, false},
									Unprefixed: true,
									Name:       "str",
								},
							},
						},
						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 11},
									nil,
									false,
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 11}, nil, false},
									Name:     "patt",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("{ prop with keylist value } ", func(t *testing.T) {
			n := mustparseChunk(t, "%{keys: .{a}}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
							},*/
						},

						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{2, 12},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 6}, nil, false},
									Name:     "keys",
								},
								Value: &KeyListExpression{
									NodeBase: NodeBase{
										NodeSpan{Start: 8, End: 12},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{8, 10}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
										},*/
									},
									Keys: []Node{
										&IdentifierLiteral{
											NodeBase: NodeBase{Span: NodeSpan{10, 11}},
											Name:     "a",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("{ optional prop } ", func(t *testing.T) {
			n := mustparseChunk(t, "%{ name?: %str }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},*/
						},

						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 14},
									nil,
									false,
									/*[]Token{
										{Type: QUESTION_MARK, Span: NodeSpan{7, 8}},
										{Type: COLON, Span: NodeSpan{8, 9}},
									},*/
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 14}, nil, false},
									Name:     "str",
								},
								Optional: true,
							},
						},
					},
				},
			}, n)
		})

		t.Run("property value is an unprefixed list pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%{ list: [ 1 ] }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},*/
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 14},
									nil,
									false,
									/*[]Token{
										{Type: COLON, Span: NodeSpan{7, 8}},
									},*/
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{Start: 3, End: 7}},
									Name:     "list",
								},
								Value: &ListPatternLiteral{
									NodeBase: NodeBase{
										NodeSpan{9, 14},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_BRACKET, Span: NodeSpan{9, 10}},
											{Type: CLOSING_BRACKET, Span: NodeSpan{13, 14}},
										},*/
									},
									Elements: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Raw:      "1",
											Value:    1,
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("property value is an unprefixed union pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%{ prop: | a | b }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
							},*/
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 17},
									nil,
									false,
									/*[]Token{
										{Type: COLON, Span: NodeSpan{7, 8}},
									},*/
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{Start: 3, End: 7}},
									Name:     "prop",
								},
								Value: &PatternUnion{
									NodeBase: NodeBase{
										NodeSpan{9, 17},
										nil,
										false,
										/*[]Token{
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{9, 10}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{13, 14}},
										},*/
									},
									Cases: []Node{
										&PatternIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{11, 12}, nil, false},
											Unprefixed: true,
											Name:       "a",
										},
										&PatternIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{15, 16}, nil, false},
											Unprefixed: true,
											Name:       "b",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("property value is an exact value pattern for an object (pattern conversion)", func(t *testing.T) {
			n := mustparseChunk(t, "%{ prop: %({}) }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},*/
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 14},
									nil,
									false,
									/*[]Token{
										{Type: COLON, Span: NodeSpan{7, 8}},
									},*/
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{Start: 3, End: 7}},
									Name:     "prop",
								},
								Value: &PatternConversionExpression{
									NodeBase: NodeBase{
										NodeSpan{9, 13},
										nil,
										false,
										/*[]Token{
											{Type: PERCENT_SYMBOL, Span: NodeSpan{9, 10}},
										},*/
									},
									Value: &ObjectLiteral{
										NodeBase: NodeBase{
											NodeSpan{11, 13},
											nil,
											true,
											/*[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{10, 11}},
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
											},*/
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("otherprops", func(t *testing.T) {
			n := mustparseChunk(t, "%{otherprops int}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
							},*/
						},
						OtherProperties: []*OtherPropsExpr{
							{
								NodeBase: NodeBase{
									Span:            NodeSpan{2, 16},
									IsParenthesized: false,
								},
								Pattern: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{Span: NodeSpan{13, 16}},
									Unprefixed: true,
									Name:       "int",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("otherprops no", func(t *testing.T) {
			n := mustparseChunk(t, "%{otherprops no}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},*/
						},
						OtherProperties: []*OtherPropsExpr{
							{
								NodeBase: NodeBase{
									Span:            NodeSpan{2, 15},
									IsParenthesized: false,
								},
								No: true,
								Pattern: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{Span: NodeSpan{13, 15}},
									Unprefixed: true,
									Name:       "no",
								},
							},
						},
					},
				},
			}, n)
		})
		t.Run("otherprops no: parenthesized", func(t *testing.T) {
			n := mustparseChunk(t, "%{otherprops(no)}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
							},*/
						},
						OtherProperties: []*OtherPropsExpr{
							{
								NodeBase: NodeBase{
									Span:            NodeSpan{2, 15},
									IsParenthesized: false,
								},
								No: true,
								Pattern: &PatternIdentifierLiteral{
									NodeBase: NodeBase{
										Span:            NodeSpan{13, 15},
										IsParenthesized: true,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{12, 13}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{15, 16}},
										},*/
									},
									Unprefixed: true,
									Name:       "no",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated otherprops followed by '}'", func(t *testing.T) {
			n, err := parseChunk(t, "%{otherprops}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
							},*/
						},
						OtherProperties: []*OtherPropsExpr{
							{
								NodeBase: NodeBase{
									Span:            NodeSpan{2, 13},
									IsParenthesized: false,
								},
								Pattern: &MissingExpression{
									NodeBase: NodeBase{
										NodeSpan{12, 13},
										&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("%{otherprops}"), 12, true)},
										false,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated otherprops at end of file", func(t *testing.T) {
			n, err := parseChunk(t, "%{otherprops", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE},
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
							},*/
						},
						OtherProperties: []*OtherPropsExpr{
							{
								NodeBase: NodeBase{
									Span:            NodeSpan{2, 12},
									IsParenthesized: false,
								},
								Pattern: &MissingExpression{
									NodeBase: NodeBase{
										NodeSpan{11, 12},
										&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("%{otherprops"), 12, true)},
										false,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("duplicate key", func(t *testing.T) {
			n, err := parseChunk(t, "%{ a: 1, a: 2}", "")
			assert.NoError(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{7, 8}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
							},*/
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 7},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "a",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{9, 13},
									nil,
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
									Name:     "a",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
									Raw:      "2",
									Value:    2,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("%{,", func(t *testing.T) {
			n, err := parseChunk(t, "%{,", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE},
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{2, 3}},
							},*/
						},
					},
				},
			}, n)
		})
		t.Run("%{,}", func(t *testing.T) {
			n := mustparseChunk(t, "%{,}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{2, 3}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
							},*/
						},
					},
				},
			}, n)
		})

		t.Run("line feed after colon", func(t *testing.T) {
			n, err := parseChunk(t, "%{ a:\n}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: NEWLINE, Span: NodeSpan{5, 6}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
							},*/
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 5},
									&ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON},
									false,
									/*[]Token{
										{Type: COLON, Span: NodeSpan{4, 5}},
									},*/
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "a",
								},
							},
						},
					},
				},
			}, n)

		})

		t.Run("missing property pattern", func(t *testing.T) {
			n, err := parseChunk(t, "%{a:}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							false,
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{2, 4},
									&ParsingError{MissingObjectPatternProperty, MISSING_PROPERTY_PATTERN},
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "a",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty pattern with missing closing brace before parenthesis of parent", func(t *testing.T) {
			n, err := parseChunk(t, "(%{)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{1, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE},
							true,
						},
					},
				},
			}, n)
		})

		t.Run("non-empty pattern with missing closing brace before parenthesis of parent", func(t *testing.T) {
			n, err := parseChunk(t, "(%{a:1)", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{1, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE},
							true,
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{Span: NodeSpan{3, 6}},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "a",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("non-closing unexpected char after property", func(t *testing.T) {
			n, err := parseChunk(t, "%{a:1?}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{Span: NodeSpan{0, 7}},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{2, 5},
									&ParsingError{UnspecifiedParsingError, INVALID_OBJ_PATT_LIT_ENTRY_SEPARATION},
									false,
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
									Name:     "a",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{5, 6},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInObjectPattern('?')},
									false,
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("list pattern", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, "%[ 1 ]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
							},*/
						},
						Elements: []Node{
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Raw:      "1",
								Value:    1,
							},
						},
					},
				},
			}, n)
		})

		t.Run("single element is an unprefixed named pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%[ a ]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
							},*/
						},
						Elements: []Node{
							&PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{3, 4}, nil, false},
								Unprefixed: true,
								Name:       "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single element is an unprefixed object pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%[{ name?: %str }]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{17, 18}},
							},*/
						},
						Elements: []Node{
							&ObjectPatternLiteral{
								NodeBase: NodeBase{
									NodeSpan{2, 17},
									nil,
									false,
									/*[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
									},*/
								},

								Properties: []*ObjectPatternProperty{
									{
										NodeBase: NodeBase{
											NodeSpan{4, 15},
											nil,
											false,
											/*[]Token{
												{Type: QUESTION_MARK, Span: NodeSpan{8, 9}},
												{Type: COLON, Span: NodeSpan{9, 10}},
											},*/
										},
										Key: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
											Name:     "name",
										},
										Value: &PatternIdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{11, 15}, nil, false},
											Name:     "str",
										},
										Optional: true,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two elements", func(t *testing.T) {
			n := mustparseChunk(t, "%[ 1, 2 ]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{4, 5}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
							},*/
						},
						Elements: []Node{
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Raw:      "1",
								Value:    1,
							},
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Raw:      "2",
								Value:    2,
							},
						},
					},
				},
			}, n)
		})

		t.Run("general element", func(t *testing.T) {
			n := mustparseChunk(t, "%[]%int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
							},*/
						},
						Elements: nil,
						GeneralElement: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, false},
							Name:     "int",
						},
					},
				},
			}, n)
		})

		t.Run("general element is an unprefixed named pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%[]int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
							},*/
						},
						Elements: nil,
						GeneralElement: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{3, 6}, nil, false},
							Unprefixed: true,
							Name:       "int",
						},
					},
				},
			}, n)
		})

		t.Run("general element is an unprefixed object pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%[]{ name?: %str }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
							},*/
						},
						Elements: nil,
						GeneralElement: &ObjectPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{3, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},

							Properties: []*ObjectPatternProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{5, 16},
										nil,
										false,
										/*[]Token{
											{Type: QUESTION_MARK, Span: NodeSpan{9, 10}},
											{Type: COLON, Span: NodeSpan{10, 11}},
										},*/
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
										Name:     "name",
									},
									Value: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{12, 16}, nil, false},
										Name:     "str",
									},
									Optional: true,
								},
							},
						},
					},
				},
			}, n)
		})

		//TODO: add more tests

		t.Run("elements and general element", func(t *testing.T) {
			n, err := parseChunk(t, "%[1]%int", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, INVALID_LIST_TUPLE_PATT_GENERAL_ELEMENT_IF_ELEMENTS},
							false,
							/*[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
							},*/
						},
						Elements: []Node{
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
								Raw:      "1",
								Value:    1,
							},
						},
						GeneralElement: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, false},
							Name:     "int",
						},
					},
				},
			}, n)
		})
	})

	t.Run("tuple pattern", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = #[ 1 ]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &TuplePatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{12, 14}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{17, 18}},
								},*/
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("general element", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = #[]int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &TuplePatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{12, 14}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{14, 15}},
								},*/
							},
							Elements: nil,
							GeneralElement: &PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{15, 18}, nil, false},
								Name:       "int",
								Unprefixed: true,
							},
						},
					},
				},
			}, n)
		})

		t.Run("general element: empty tuple pattern", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = #[]#{}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &TuplePatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 18},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{12, 14}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{14, 15}},
								},*/
							},
							Elements: nil,
							GeneralElement: &RecordPatternLiteral{
								NodeBase: NodeBase{
									NodeSpan{15, 18},
									nil,
									false,
									/*[]Token{
										{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{15, 17}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									},*/
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern definition", func(t *testing.T) {
		t.Run("RHS is a pattern identifier literal ", func(t *testing.T) {
			n := mustparseChunk(t, "pattern i = %int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "i",
							Unprefixed: true,
						},
						Right: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{12, 16}, nil, false},
							Name:     "int",
						},
					},
				},
			}, n)
		})

		t.Run("lazy", func(t *testing.T) {
			n := mustparseChunk(t, "pattern i = @ 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						IsLazy: true,
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "i",
							Unprefixed: true,
						},
						Right: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{14, 15}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("RHS is an object pattern literal", func(t *testing.T) {
			n := mustparseChunk(t, "pattern i = %{ a: 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "i",
							Unprefixed: true,
						},
						Right: &ObjectPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{12, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							Properties: []*ObjectPatternProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{15, 19},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("RHS is an unprefixed object pattern literal", func(t *testing.T) {
			n := mustparseChunk(t, "pattern i = %{ a: 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "i",
							Unprefixed: true,
						},
						Right: &ObjectPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 21},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{12, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},*/
							},
							Properties: []*ObjectPatternProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{15, 19},
										nil,
										false,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern definition : missing RHS", func(t *testing.T) {
			n, err := parseChunk(t, "pattern i =", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnterminatedPatternDefinition, UNTERMINATED_PATT_DEF_MISSING_RHS},
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "i",
							Unprefixed: true,
						},
					},
				},
			}, n)
		})

	})

	t.Run("pattern namespace definition", func(t *testing.T) {
		n := mustparseChunk(t, "pnamespace mynamespace. = {}")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
			Statements: []Node{
				&PatternNamespaceDefinition{
					NodeBase: NodeBase{
						NodeSpan{0, 28},
						nil,
						false,
						/*[]Token{
							{Type: PNAMESPACE_KEYWORD, Span: NodeSpan{0, 10}},
							{Type: EQUAL, Span: NodeSpan{24, 25}},
						},*/
					},
					Left: &PatternNamespaceIdentifierLiteral{
						NodeBase:   NodeBase{NodeSpan{11, 23}, nil, false},
						Name:       "mynamespace",
						Unprefixed: true,
					},
					Right: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{26, 28},
							nil,
							false,
							/*[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{26, 27}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
							},*/
						},
					},
				},
			},
		}, n)
	})

	t.Run("record pattern", func(t *testing.T) {

		t.Run("{ ...named-pattern } ", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = #{ ...%patt }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},*/
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &RecordPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 25},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{12, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},*/
							},

							SpreadElements: []*PatternPropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{15, 23},
										nil,
										false,
									},
									Expr: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{18, 23}, nil, false},
										Name:     "patt",
									},
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern namespace member expression", func(t *testing.T) {
		n := mustparseChunk(t, "%mynamespace.a")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
			Statements: []Node{
				&PatternNamespaceMemberExpression{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
					Namespace: &PatternNamespaceIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
						Name:     "mynamespace",
					},
					MemberName: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
						Name:     "a",
					},
				},
			},
		}, n)
	})

	t.Run("complex string pattern", func(t *testing.T) {
		t.Run("one element: string literal", func(t *testing.T) {
			n := mustparseChunk(t, `%str("a")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 8}, nil, false},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
									Raw:      "\"a\"",
									Value:    "a",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one element: string literal followed by linefeed", func(t *testing.T) {
			n := mustparseChunk(t, "%str(\"a\"\n)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: NEWLINE, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 8}, nil, false},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
									Raw:      "\"a\"",
									Value:    "a",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one element: int literal (should fail)", func(t *testing.T) {
			n, err := parseChunk(t, `%str(1)`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 6}, nil, false},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &InvalidComplexStringPatternElement{
									NodeBase: NodeBase{
										NodeSpan{5, 6},
										&ParsingError{UnspecifiedParsingError, INVALID_COMPLEX_PATTERN_ELEMENT},
										false,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one element: rune literal", func(t *testing.T) {
			n := mustparseChunk(t, "%str('a')")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 8}, nil, false},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &RuneLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
									Value:    'a',
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one element: element is a parenthesized string literal with '*' as ocurrence", func(t *testing.T) {
			n := mustparseChunk(t, `%str(("a")*)`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 11},
									nil,
									false,
								},
								Ocurrence: ZeroOrMoreOcurrence,
								Expr: &ComplexStringPatternPiece{
									NodeBase: NodeBase{
										NodeSpan{5, 10},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
										},*/
									},
									Elements: []*PatternPieceElement{
										{
											NodeBase:  NodeBase{NodeSpan{6, 9}, nil, false},
											Ocurrence: ExactlyOneOcurrence,
											Expr: &QuotedStringLiteral{
												NodeBase: NodeBase{NodeSpan{6, 9}, nil, false},
												Raw:      "\"a\"",
												Value:    "a",
											},
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one element: element is a parenthesized string literal with '=2' as ocurrence", func(t *testing.T) {
			n := mustparseChunk(t, `%str(("a")=2)`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 12},
									nil,
									false,
								},
								Ocurrence:           ExactOcurrence,
								ExactOcurrenceCount: 2,
								Expr: &ComplexStringPatternPiece{
									NodeBase: NodeBase{
										NodeSpan{5, 10},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
										},*/
									},
									Elements: []*PatternPieceElement{
										{
											NodeBase:  NodeBase{NodeSpan{6, 9}, nil, false},
											Ocurrence: ExactlyOneOcurrence,
											Expr: &QuotedStringLiteral{
												NodeBase: NodeBase{NodeSpan{6, 9}, nil, false},
												Raw:      "\"a\"",
												Value:    "a",
											},
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one element: element is a pattern identifier literal with '=2' as ocurrence", func(t *testing.T) {
			n := mustparseChunk(t, `%str(s=2)`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							false,
						},
						Elements: []*PatternPieceElement{
							{
								Ocurrence:           ExactOcurrence,
								ExactOcurrenceCount: 2,
								NodeBase: NodeBase{
									NodeSpan{5, 8},
									nil,
									false,
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{5, 6}, nil, false},
									Name:       "s",
									Unprefixed: true,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one element: element is a regex literal with '=2' as ocurrence", func(t *testing.T) {
			n := mustparseChunk(t, "%str(%`e`=2)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
						},
						Elements: []*PatternPieceElement{
							{
								Ocurrence:           ExactOcurrence,
								ExactOcurrenceCount: 2,
								NodeBase: NodeBase{
									NodeSpan{5, 11},
									nil,
									false,
								},
								Expr: &RegularExpressionLiteral{
									NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
									Value:    "e",
									Raw:      "%`e`",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one named element", func(t *testing.T) {
			n := mustparseChunk(t, `%str(l:"a")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 10},
									nil,
									false,
								},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{7, 10}, nil, false},
									Raw:      "\"a\"",
									Value:    "a",
								},
								GroupName: &PatternGroupName{
									NodeBase: NodeBase{Span: NodeSpan{5, 6}},
									Name:     "l",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one named element: space after name", func(t *testing.T) {
			n := mustparseChunk(t, `%str(l: "a")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{Span: NodeSpan{0, 12}},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{Span: NodeSpan{5, 11}},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
									Raw:      "\"a\"",
									Value:    "a",
								},
								GroupName: &PatternGroupName{
									NodeBase: NodeBase{Span: NodeSpan{5, 6}},
									Name:     "l",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one named element: invalid name", func(t *testing.T) {
			n, err := parseChunk(t, `%str(name-0-: "a")`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{Span: NodeSpan{0, 18}},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{Span: NodeSpan{5, 17}},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{14, 17}, nil, false},
									Raw:      "\"a\"",
									Value:    "a",
								},
								GroupName: &PatternGroupName{
									NodeBase: NodeBase{
										Span: NodeSpan{5, 12},
										Err:  &ParsingError{UnspecifiedParsingError, INVALID_GROUP_NAME_SHOULD_NOT_END_WITH_DASH},
									},
									Name: "name-0-",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("element name without element", func(t *testing.T) {
			n, err := parseChunk(t, `%str(l:)`, "")
			assert.Error(t, err)
			runes := []rune("%str(l:)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 7},
									nil,
									false,
								},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &InvalidComplexStringPatternElement{
									NodeBase: NodeBase{
										Span: NodeSpan{7, 7},
										Err:  &ParsingError{UnspecifiedParsingError, fmtAPatternWasExpectedHere(runes, 7)},
									},
								},
								GroupName: &PatternGroupName{
									NodeBase: NodeBase{Span: NodeSpan{5, 6}},
									Name:     "l",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two elements string literal elements", func(t *testing.T) {

			n := mustparseChunk(t, `%str("a" "b")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
									Raw:      "\"a\"",
									Value:    "a",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
									Raw:      "\"b\"",
									Value:    "b",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern union: 2 elements", func(t *testing.T) {
			n := mustparseChunk(t, `%str( (| "a" | "b" ) )`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{21, 22}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{6, 20}, nil, false},
								Expr: &PatternUnion{
									NodeBase: NodeBase{
										NodeSpan{6, 20},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{7, 8}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{13, 14}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{19, 20}},
										},*/
									},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern union: 3 elements", func(t *testing.T) {
			n := mustparseChunk(t, `%str( (| "a" | "b" | "c" ) )`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{27, 28}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{6, 26}, nil, false},
								Expr: &PatternUnion{
									NodeBase: NodeBase{
										NodeSpan{6, 26},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{7, 8}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{13, 14}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{19, 20}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{25, 26}},
										},*/
									},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{21, 24}, nil, false},
											Raw:      `"c"`,
											Value:    "c",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern union: 3 elements but last element is missing", func(t *testing.T) {
			n, err := parseChunk(t, `%str( (| "a" | "b" | ) )`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							false,
							/*[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{23, 24}},
							},*/
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{6, 22}, nil, false},
								Expr: &PatternUnion{
									NodeBase: NodeBase{
										NodeSpan{6, 22},
										nil,
										false,
										/*[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{7, 8}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{13, 14}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{19, 20}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{21, 22}},
										},*/
									},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
										&InvalidComplexStringPatternElement{
											NodeBase: NodeBase{
												NodeSpan{21, 21},
												&ParsingError{
													UnspecifiedParsingError,
													fmtAPatternWasExpectedHere([]rune(`%str( (| "a" | "b" | ) )`), 21),
												},
												false,
											},
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern union with newline after each pipe symbol", func(t *testing.T) {
			n := mustparseChunk(t, "%str( (|\n\"a\" |\n\"b\" ) )")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{Span: NodeSpan{0, 22}},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{6, 20}, nil, false},
								Expr: &PatternUnion{
									NodeBase: NodeBase{Span: NodeSpan{6, 20}},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("shorthand pattern union", func(t *testing.T) {
			n := mustparseChunk(t, `%str(| "a" | "b")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{Span: NodeSpan{0, 17}},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{5, 16}, nil, false},
								Expr: &PatternUnion{
									NodeBase: NodeBase{Span: NodeSpan{5, 16}},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{7, 10}, nil, false},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("shorthand pattern union with newline before first pipe", func(t *testing.T) {
			n := mustparseChunk(t, "%str(\n| \"a\" | \"b\")")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{Span: NodeSpan{0, 18}},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{6, 17}, nil, false},
								Expr: &PatternUnion{
									NodeBase: NodeBase{Span: NodeSpan{6, 17}},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{14, 17}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("shorthand pattern union with newline after each pipe symbol", func(t *testing.T) {
			n := mustparseChunk(t, "%str(|\n\"a\" | \"b\")")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{Span: NodeSpan{0, 17}},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{5, 16}, nil, false},
								Expr: &PatternUnion{
									NodeBase: NodeBase{Span: NodeSpan{5, 16}},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{7, 10}, nil, false},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("in pattern definition", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = %str(\"a\")")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{Span: NodeSpan{0, 21}},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &ComplexStringPatternPiece{
							NodeBase: NodeBase{Span: NodeSpan{12, 21}},
							Elements: []*PatternPieceElement{
								{
									NodeBase:  NodeBase{NodeSpan{17, 20}, nil, false},
									Ocurrence: ExactlyOneOcurrence,
									Expr: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{17, 20}, nil, false},
										Raw:      "\"a\"",
										Value:    "a",
									},
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern call", func(t *testing.T) {
		t.Run("pattern identifier callee, no arguments", func(t *testing.T) {
			n := mustparseChunk(t, `%text()`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 7},
							IsParenthesized: false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},*/
						},
						Callee: &PatternIdentifierLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 5},
							},
							Name: "text",
						},
					},
				},
			}, n)
		})

		t.Run("pattern namespace member callee, no arguments", func(t *testing.T) {
			n := mustparseChunk(t, `%std.text()`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 11},
							IsParenthesized: false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},*/
						},
						Callee: &PatternNamespaceMemberExpression{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 9},
							},
							Namespace: &PatternNamespaceIdentifierLiteral{
								NodeBase: NodeBase{Span: NodeSpan{0, 5}},
								Name:     "std",
							},
							MemberName: &IdentifierLiteral{
								NodeBase: NodeBase{Span: NodeSpan{5, 9}},
								Name:     "text",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single argument", func(t *testing.T) {
			n := mustparseChunk(t, `%text(1)`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 8},
							IsParenthesized: false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Callee: &PatternIdentifierLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 5},
							},
							Name: "text",
						},
						Arguments: []Node{
							&IntLiteral{
								NodeBase: NodeBase{
									Span: NodeSpan{6, 7},
								},
								Raw:   "1",
								Value: 1,
							},
						},
					},
				},
			}, n)
		})

		t.Run("two arguments", func(t *testing.T) {
			n := mustparseChunk(t, `%text(1,2)`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 10},
							IsParenthesized: false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: COMMA, Span: NodeSpan{7, 8}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Callee: &PatternIdentifierLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 5},
							},
							Name: "text",
						},
						Arguments: []Node{
							&IntLiteral{
								NodeBase: NodeBase{
									Span: NodeSpan{6, 7},
								},
								Raw:   "1",
								Value: 1,
							},
							&IntLiteral{
								NodeBase: NodeBase{
									Span: NodeSpan{8, 9},
								},
								Raw:   "2",
								Value: 2,
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern identifier as argument", func(t *testing.T) {
			n := mustparseChunk(t, `%text(i)`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 8},
							IsParenthesized: false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Callee: &PatternIdentifierLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 5},
							},
							Name: "text",
						},
						Arguments: []Node{
							&PatternIdentifierLiteral{
								NodeBase: NodeBase{
									Span: NodeSpan{6, 7},
								},
								Unprefixed: true,
								Name:       "i",
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char in arguments", func(t *testing.T) {
			n, err := parseChunk(t, `%text(:)`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 8},
							IsParenthesized: false,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Callee: &PatternIdentifierLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{0, 5},
							},
							Name: "text",
						},
						Arguments: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									Span:            NodeSpan{6, 7},
									Err:             &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInPatternCallArguments(':')},
									IsParenthesized: false,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("shorthand syntax for object pattern argument: empty", func(t *testing.T) {
			n := mustparseChunk(t, `%text{}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 7}},
						Callee: &PatternIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 5}},
							Name:     "text",
						},
						Arguments: []Node{
							&ObjectPatternLiteral{
								NodeBase: NodeBase{
									Span:            NodeSpan{5, 7},
									IsParenthesized: false,
									/*[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									},*/
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("shorthand syntax for object pattern argument: one property", func(t *testing.T) {
			n := mustparseChunk(t, `%text{a: int}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 13}},
						Callee: &PatternIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 5}},
							Name:     "text",
						},
						Arguments: []Node{
							&ObjectPatternLiteral{
								NodeBase: NodeBase{
									Span:            NodeSpan{5, 13},
									IsParenthesized: false,
								},
								Properties: []*ObjectPatternProperty{
									{
										NodeBase: NodeBase{Span: NodeSpan{6, 12}},
										Key: &IdentifierLiteral{
											NodeBase: NodeBase{Span: NodeSpan{6, 7}},
											Name:     "a",
										},
										Value: &PatternIdentifierLiteral{
											NodeBase:   NodeBase{Span: NodeSpan{9, 12}},
											Unprefixed: true,
											Name:       "int",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern union", func(t *testing.T) {

		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, `%| "a"`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single element is an unprefixed pattern", func(t *testing.T) {
			n := mustparseChunk(t, `%| a`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
						Cases: []Node{
							&PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{3, 4}, nil, false},
								Unprefixed: true,
								Name:       "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized, single element", func(t *testing.T) {
			n := mustparseChunk(t, `(%| "a")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{1, 7},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{1, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},*/
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
						},
					},
				},
			}, n)
		})

		t.Run("two elements", func(t *testing.T) {
			n := mustparseChunk(t, `%| "a" | "b"`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							false,
							/*[]Token{
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{0, 2}},
								{Type: PATTERN_UNION_PIPE, Span: NodeSpan{7, 8}},
							},*/
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{3, 6}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
								Raw:      `"b"`,
								Value:    "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized, two elements", func(t *testing.T) {
			n := mustparseChunk(t, `(%| "a" | "b")`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{1, 13},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{1, 3}},
								{Type: PATTERN_UNION_PIPE, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
							},*/
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{10, 13}, nil, false},
								Raw:      `"b"`,
								Value:    "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized, linefeed after first pipe", func(t *testing.T) {
			n := mustparseChunk(t, "(%|\n\"a\" | \"b\")")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{1, 13},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{1, 3}},
								{Type: PATTERN_UNION_PIPE, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
							},*/
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{10, 13}, nil, false},
								Raw:      `"b"`,
								Value:    "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized, linefeed before second pipe", func(t *testing.T) {
			n := mustparseChunk(t, "(%| \"a\"\n| \"b\")")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{1, 13},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{1, 3}},
								{Type: PATTERN_UNION_PIPE, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
							},*/
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, false},
								Raw:      `"a"`,
								Value:    "a",
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{10, 13}, nil, false},
								Raw:      `"b"`,
								Value:    "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized and unprefixed, linefeed after first pipe", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = (|\n\"a\" | \"b\")")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 25},
							IsParenthesized: false,
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{8, 9}},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &PatternUnion{
							NodeBase: NodeBase{
								NodeSpan{13, 24},
								nil,
								true,
							},
							Cases: []Node{
								&QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
									Raw:      `"a"`,
									Value:    "a",
								},
								&QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{21, 24}, nil, false},
									Raw:      `"b"`,
									Value:    "b",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized and unprefixed, linefeed before second pipe", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = (| \"a\"\n| \"b\")")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							Span:            NodeSpan{0, 25},
							IsParenthesized: false,
						},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{8, 9}},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &PatternUnion{
							NodeBase: NodeBase{
								NodeSpan{13, 24},
								nil,
								true,
							},
							Cases: []Node{
								&QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
									Raw:      `"a"`,
									Value:    "a",
								},
								&QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{21, 24}, nil, false},
									Raw:      `"b"`,
									Value:    "b",
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("assert statement", func(t *testing.T) {
		t.Run("assert statement", func(t *testing.T) {
			n := mustparseChunk(t, "assert true")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&AssertionStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							false,
						},
						Expr: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{7, 11}, nil, false},
							Value:    true,
						},
					},
				},
			}, n)
		})

		t.Run("missing expr", func(t *testing.T) {
			code := "assert"
			n, err := parseChunk(t, code, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&AssertionStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							false,
						},
						Expr: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{5, 6},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune(code), 6, true)},
								false,
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("synchronized block", func(t *testing.T) {
		t.Run("keyword only", func(t *testing.T) {
			n, err := parseChunk(t, "synchronized", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, SYNCHRONIZED_KEYWORD_SHOULD_BE_FOLLOWED_BY_SYNC_VALUES},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("single value", func(t *testing.T) {
			code := "synchronized val {}"
			n := mustparseChunk(t, code)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
							/*[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},*/
						},
						SynchronizedValues: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
								Name:     "val",
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("single value in parenthesis", func(t *testing.T) {
			code := "synchronized(val){}"
			n := mustparseChunk(t, code)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							false,
							/*[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},*/
						},
						SynchronizedValues: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{
									NodeSpan{13, 16},
									nil,
									true,
									/*[]Token{
										{Type: OPENING_PARENTHESIS, Span: NodeSpan{12, 13}},
										{Type: CLOSING_PARENTHESIS, Span: NodeSpan{16, 17}},
									},*/
								},
								Name: "val",
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("two values", func(t *testing.T) {
			code := "synchronized val1 val2 {}"
			n := mustparseChunk(t, code)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
							/*[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},*/
						},
						SynchronizedValues: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{13, 17}, nil, false},
								Name:     "val1",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{18, 22}, nil, false},
								Name:     "val2",
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{23, 25},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char", func(t *testing.T) {
			code := "synchronized ? {}"
			n, err := parseChunk(t, code, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							false,
							/*[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},*/
						},
						SynchronizedValues: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{13, 14},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSynchronizedValueList('?')},
									false,
								},
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{15, 17},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},*/
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("css selector", func(t *testing.T) {

		t.Run("single element : type selector", func(t *testing.T) {
			n := mustparseChunk(t, "s!div")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
								Name:     "div",
							},
						},
					},
				},
			}, n)
		})

		t.Run("selector followed by line feed", func(t *testing.T) {

			n := mustparseChunk(t, "s!div\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 6},
					nil,
					false,
				},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
								Name:     "div",
							},
						},
					},
				},
			}, n)
		})

		t.Run("selector followed by exclamation mark", func(t *testing.T) {

			n := mustparseChunk(t, "s!div!")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
								Name:     "div",
							},
						},
					},
				},
			}, n)
		})

		t.Run("selector followed by exclamation mark and an expression", func(t *testing.T) {

			n := mustparseChunk(t, "s!div! 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
								Name:     "div",
							},
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("single element : class selector", func(t *testing.T) {
			n := mustparseChunk(t, "s!.ab")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Elements: []Node{
							&CssClassSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
								Name:     "ab",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single element : pseudo class selector", func(t *testing.T) {
			n := mustparseChunk(t, "s!:ab")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Elements: []Node{
							&CssPseudoClassSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
								Name:     "ab",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single element : pseudo element selector", func(t *testing.T) {
			n := mustparseChunk(t, "s!::ab")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Elements: []Node{
							&CssPseudoElementSelector{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, false},
								Name:     "ab",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single element : pseudo element selector", func(t *testing.T) {
			n := mustparseChunk(t, "s!::ab")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Elements: []Node{
							&CssPseudoElementSelector{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, false},
								Name:     "ab",
							},
						},
					},
				},
			}, n)
		})

		t.Run("single element : attribute selector", func(t *testing.T) {
			n := mustparseChunk(t, `s![a="1"]`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
						Elements: []Node{
							&CssAttributeSelector{
								NodeBase: NodeBase{NodeSpan{2, 9}, nil, false},
								AttributeName: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
									Name:     "a",
								},
								Pattern: "=",
								Value: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, false},
									Raw:      `"1"`,
									Value:    "1",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("direct child", func(t *testing.T) {
			n := mustparseChunk(t, "s!a > b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
								Name:     "a",
							},
							&CssCombinator{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Name:     ">",
							},
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})

		t.Run("descendant", func(t *testing.T) {
			n := mustparseChunk(t, "s!a b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
								Name:     "a",
							},
							&CssCombinator{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
								Name:     " ",
							},
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, false},
								Name:     "b",
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("various", func(t *testing.T) {

		testCases := map[string]Node{
			"(1 + $a.a)": &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, false},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							true,
							/*[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},*/
						},
						Left: &IntLiteral{
							NodeBase: NodeBase{
								Span: NodeSpan{1, 2},
							},
							Raw:   "1",
							Value: 1,
						},
						Operator: Add,
						Right: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{5, 7}, nil, false},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
								Name:     "a",
							},
						},
					},
				},
			},
		}

		for input, expectedOutput := range testCases {
			t.Run("", func(t *testing.T) {
				n := mustparseChunk(t, input)
				assert.EqualValues(t, expectedOutput, n)
			})
		}
	})

	t.Run("various parsing errors", func(t *testing.T) {
		testCases := []struct {
			input  string
			output *Chunk
		}{
			{
				";",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 1},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{0, 1}},
						},*/
					},
					Statements: nil,
				},
			},
			{
				";;",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 2},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{0, 1}},
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},*/
					},
					Statements: nil,
				},
			},
			{
				" ;",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 2},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},*/
					},
					Statements: nil,
				},
			},
			{
				" ;;",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 3},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
							{Type: SEMICOLON, Span: NodeSpan{2, 3}},
						},*/
					},
					Statements: nil,
				},
			},
			{
				" ; ;",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 4},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
							{Type: SEMICOLON, Span: NodeSpan{3, 4}},
						},*/
					},
					Statements: nil,
				},
			},
			{
				"1;",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 2},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},*/
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			},
			{
				"1 ;",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 3},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{2, 3}},
						},*/
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			},
			{
				"1;2",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 3},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},*/
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			},
			{
				"1; 2",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 4},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},*/
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Raw:      "1",
							Value:    1,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, false},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			},
			{
				"$a;$b",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 5},
						nil,
						false,
						/*[]Token{
							{Type: SEMICOLON, Span: NodeSpan{2, 3}},
						},*/
					},
					Statements: []Node{
						&Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
							Name:     "a",
						},
						&Variable{
							NodeBase: NodeBase{NodeSpan{3, 5}, nil, false},
							Name:     "b",
						},
					},
				},
			},
			{
				"()]",
				&Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, false},
					Statements: []Node{
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("()]"), 1, true)},
								true,
								/*[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{1, 2}},
								},*/
							},
						},
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
								false,
								/*[]Token{
									{Type: UNEXPECTED_CHAR, Raw: "]", Span: NodeSpan{2, 3}},
								},*/
							},
						},
					},
				},
			},
			{
				".",
				&Chunk{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
					Statements: []Node{
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, DOT_SHOULD_BE_FOLLOWED_BY},
								false,
							},
						},
					},
				},
			},
			{
				"@;",
				&Chunk{
					NodeBase: NodeBase{
						NodeSpan{0, 2},
						nil,
						false,
					},
					Statements: []Node{
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
								false,
							},
						},
					},
				},
			},
		}

		for _, testCase := range testCases {

			t.Run(testCase.input, func(t *testing.T) {
				n, _ := parseChunk(t, testCase.input, "")
				assert.EqualValues(t, testCase.output, n)
			})
		}
	})

	t.Run("string template literal", func(t *testing.T) {
		t.Run("pattern identifier, no interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: BACKQUOTE, Span: NodeSpan{24, 25}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern namespace's member, no interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "%sql.stmt`SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{9, 10}},
								{Type: BACKQUOTE, Span: NodeSpan{29, 30}},
							},*/
						},
						Pattern: &PatternNamespaceMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 9}, nil, false},
							Namespace: &PatternNamespaceIdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
								Name:     "sql",
							},
							MemberName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
								Name:     "stmt",
							},
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{10, 29}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("no interpolation", func(t *testing.T) {
			n, err := parseChunk(t, "%sql`SELECT * from users", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_TEMPL_LIT},
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation at the start", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`${nothing:$nothing}SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 44}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 44},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{5, 7}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{23, 25}},
								{Type: BACKQUOTE, Span: NodeSpan{44, 45}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 5}, nil, false},
								Raw:      "",
								Value:    "",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{7, 23}, nil, false},
								Type:     "nothing",
								Expr: &Variable{
									NodeBase: NodeBase{NodeSpan{15, 23}, nil, false},
									Name:     "nothing",
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{24, 43}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation (variable) at the end", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`SELECT * from users${nothing:$nothing}`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 44}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 44},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{24, 26}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{42, 44}},
								{Type: BACKQUOTE, Span: NodeSpan{44, 45}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{26, 42}, nil, false},
								Type:     "nothing",
								Expr: &Variable{
									NodeBase: NodeBase{NodeSpan{34, 42}, nil, false},
									Name:     "nothing",
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{43, 43}, nil, false},
								Raw:      "",
								Value:    "",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation (identifier) at the end", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`SELECT * from users${nothing:nothing}`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 43}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 43},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{24, 26}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{41, 43}},
								{Type: BACKQUOTE, Span: NodeSpan{43, 44}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{26, 41}, nil, false},
								Type:     "nothing",
								Expr: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{34, 41}, nil, false},
									Name:     "nothing",
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{42, 42}, nil, false},
								Raw:      "",
								Value:    "",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation type containing a '.'", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`${ab.cdef:1}SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 37}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 37},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{5, 7}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{16, 18}},
								{Type: BACKQUOTE, Span: NodeSpan{37, 38}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 5}, nil, false},
								Raw:      "",
								Value:    "",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
								Type:     "ab.cdef",
								Expr: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{17, 36}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation with expression of len 1", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`${nothing:1}SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 37}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 37},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{5, 7}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{16, 18}},
								{Type: BACKQUOTE, Span: NodeSpan{37, 38}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 5}, nil, false},
								Raw:      "",
								Value:    "",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, false},
								Type:     "nothing",
								Expr: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, false},
									Raw:      "1",
									Value:    1,
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{17, 36}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated (no interpolatipn)", func(t *testing.T) {
			n, err := parseChunk(t, "%sql`SELECT * from users", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_TEMPL_LIT},
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty interpolation at the end", func(t *testing.T) {
			n, err := parseChunk(t, "%sql`SELECT * from users${}`", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							false,
							/*[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{24, 26}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{26, 28}},
								{Type: BACKQUOTE, Span: NodeSpan{28, 29}},
							},*/
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, false},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{
									NodeSpan{26, 26},
									&ParsingError{UnspecifiedParsingError, INVALID_STRING_INTERPOLATION_SHOULD_NOT_BE_EMPTY},
									false,
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{27, 27}, nil, false},
								Raw:      "",
								Value:    "",
							},
						},
					},
				},
			}, n)

			t.Run("no pattern, interpolation at the start", func(t *testing.T) {
				n := mustparseChunk(t, "`${$nothing}SELECT * from users`")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 32}, nil, false},
					Statements: []Node{
						&StringTemplateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 32},
								nil,
								false,
								/*[]Token{
									{Type: BACKQUOTE, Span: NodeSpan{0, 1}},
									{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{1, 3}},
									{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{11, 13}},
									{Type: BACKQUOTE, Span: NodeSpan{32, 33}},
								},*/
							},
							Slices: []Node{
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{1, 1}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&StringTemplateInterpolation{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, false},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{3, 11}, nil, false},
										Name:     "nothing",
									},
								},
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{12, 31}, nil, false},
									Raw:      "SELECT * from users",
									Value:    "SELECT * from users",
								},
							},
						},
					},
				}, n)
			})

			t.Run("no pattern, interpolation + line feed", func(t *testing.T) {
				n := mustparseChunk(t, "`${$nothing}\n`")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
					Statements: []Node{
						&StringTemplateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								nil,
								false,
								/*[]Token{
									{Type: BACKQUOTE, Span: NodeSpan{0, 1}},
									{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{1, 3}},
									{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{11, 13}},
									{Type: BACKQUOTE, Span: NodeSpan{14, 15}},
								},*/
							},
							Slices: []Node{
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{1, 1}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&StringTemplateInterpolation{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, false},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{3, 11}, nil, false},
										Name:     "nothing",
									},
								},
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
									Raw:      "\n",
									Value:    "\n",
								},
							},
						},
					},
				}, n)
			})

			t.Run("no pattern, interpolation + escaped n", func(t *testing.T) {
				n := mustparseChunk(t, "`${$nothing}\\n`")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
					Statements: []Node{
						&StringTemplateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								nil,
								false,
								/*[]Token{
									{Type: BACKQUOTE, Span: NodeSpan{0, 1}},
									{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{1, 3}},
									{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{11, 13}},
									{Type: BACKQUOTE, Span: NodeSpan{15, 16}},
								},*/
							},
							Slices: []Node{
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{1, 1}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&StringTemplateInterpolation{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, false},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{3, 11}, nil, false},
										Name:     "nothing",
									},
								},
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{12, 14}, nil, false},
									Raw:      "\\n",
									Value:    "\n",
								},
							},
						},
					},
				}, n)
			})
		})
	})

	t.Run("XML expression", func(t *testing.T) {

		t.Run("no children: 0 characters", func(t *testing.T) {
			n := mustparseChunk(t, "h<div></div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 12}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{6, 12},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{6, 8}},
										{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("parenthesized with no namespace", func(t *testing.T) {
			n := mustparseChunk(t, "(<div></div>)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{1, 12}, nil, true},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 12}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{6, 12}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated opening tag", func(t *testing.T) {
			n, err := parseChunk(t, "h<div", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 5},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING},
									false,
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated opening tag of child element", func(t *testing.T) {
			n, err := parseChunk(t, "h<div><span</div>", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 17}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{
										Span: NodeSpan{6, 6},
									},
								},
								&XMLElement{
									NodeBase: NodeBase{NodeSpan{6, 11}, nil, false},
									Opening: &XMLOpeningElement{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING},
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{7, 11}, nil, false},
											Name:     "span",
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{
										Span: NodeSpan{11, 11},
									},
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{11, 17}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated opening tag of nested child element", func(t *testing.T) {
			n, err := parseChunk(t, "h<div><div><span</div></div>", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 28}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{
										Span: NodeSpan{6, 6},
									},
								},
								&XMLElement{
									NodeBase: NodeBase{NodeSpan{6, 22}, nil, false},
									Opening: &XMLOpeningElement{
										NodeBase: NodeBase{Span: NodeSpan{6, 11}},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{7, 10}, nil, false},
											Name:     "div",
										},
									},
									Children: []Node{
										&XMLText{
											NodeBase: NodeBase{
												Span: NodeSpan{11, 11},
											},
										},
										&XMLElement{
											NodeBase: NodeBase{NodeSpan{11, 16}, nil, false},
											Opening: &XMLOpeningElement{
												NodeBase: NodeBase{
													NodeSpan{11, 16},
													&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING},
													false,
												},
												Name: &IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{12, 16}, nil, false},
													Name:     "span",
												},
											},
										},
										&XMLText{
											NodeBase: NodeBase{
												Span: NodeSpan{16, 16},
											},
										},
									},
									Closing: &XMLClosingElement{
										NodeBase: NodeBase{Span: NodeSpan{16, 22}},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{18, 21}, nil, false},
											Name:     "div",
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{
										Span: NodeSpan{22, 22},
									},
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{22, 28}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{24, 27}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("closing bracket of opening tag is on the next line", func(t *testing.T) {
			n := mustparseChunk(t, "h<div\n></div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 13}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 7}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{7, 7}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{7, 13},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{12, 14}},
										{Type: GREATER_THAN, Span: NodeSpan{17, 18}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with value", func(t *testing.T) {
			n := mustparseChunk(t, `h<div a="b"></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 18}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 12},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{12, 12}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{12, 18},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{12, 14}},
										{Type: GREATER_THAN, Span: NodeSpan{17, 18}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 17}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with value on next line", func(t *testing.T) {
			n := mustparseChunk(t, "h<div\na=\"b\"></div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 18}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 12},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{12, 12}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{12, 18},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{12, 14}},
										{Type: GREATER_THAN, Span: NodeSpan{17, 18}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 17}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute without value on next line", func(t *testing.T) {
			n := mustparseChunk(t, "h<div\na></div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 14}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 8},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 7},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{8, 8}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{8, 14},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{12, 14}},
										{Type: GREATER_THAN, Span: NodeSpan{17, 18}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 13}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("self-closing: attribute with value", func(t *testing.T) {
			n := mustparseChunk(t, `h<div a="b"/>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 13}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 13},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: SELF_CLOSING_TAG_TERMINATOR, Span: NodeSpan{11, 13}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with value, followed by space", func(t *testing.T) {
			n := mustparseChunk(t, `h<div a="b" ></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 19}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 13},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{13, 13}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{13, 19},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{13, 15}},
										{Type: GREATER_THAN, Span: NodeSpan{18, 19}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with invalid name with value", func(t *testing.T) {
			n, err := parseChunk(t, `h<div "a"="b"></div>`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 20}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 14},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{13, 14}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 13},
											nil,
											false,
										},
										Name: &QuotedStringLiteral{
											NodeBase: NodeBase{
												NodeSpan{6, 9},
												&ParsingError{UnspecifiedParsingError, XML_ATTRIBUTE_NAME_SHOULD_BE_IDENT},
												false,
											},
											Raw:   `"a"`,
											Value: "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{10, 13}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{14, 14}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{14, 20},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{14, 16}},
										{Type: GREATER_THAN, Span: NodeSpan{19, 20}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{16, 19}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with missing value after '='", func(t *testing.T) {
			n, err := parseChunk(t, `h<div a=></div>`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 15}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 9},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{8, 9}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 8},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
										Value: &MissingExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 9},
												&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("h<div a=></div>"), 8, true)},
												false,
											},
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 9}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{9, 15},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{9, 11}},
										{Type: GREATER_THAN, Span: NodeSpan{14, 15}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with missing value after '='", func(t *testing.T) {
			n, err := parseChunk(t, `h<div a=></div>`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 15}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 9},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{8, 9}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 8},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
										Value: &MissingExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 9},
												&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("h<div a=></div>"), 8, true)},
												false,
											},
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 9}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{9, 15},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{9, 11}},
										{Type: GREATER_THAN, Span: NodeSpan{14, 15}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with only name", func(t *testing.T) {
			n := mustparseChunk(t, `h<div a></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 14}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 8},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{7, 8}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{8, 8}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{8, 14},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{8, 10}},
										{Type: GREATER_THAN, Span: NodeSpan{13, 14}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 13}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("attribute with only name: unterminated opening tag", func(t *testing.T) {
			n, err := parseChunk(t, `h<div a`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 7}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 7},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING},
									false,
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two attributes with value", func(t *testing.T) {
			n := mustparseChunk(t, `h<div a="b" c="d"></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 24}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 18},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{17, 18}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, false},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
									&XMLAttribute{
										NodeBase: NodeBase{
											NodeSpan{12, 17},
											nil,
											false,
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
											Name:     "c",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{14, 17}, nil, false},
											Raw:      `"d"`,
											Value:    "d",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{18, 18}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{18, 24},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{18, 20}},
										{Type: GREATER_THAN, Span: NodeSpan{23, 24}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{20, 23}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty hyperscript attribute shorthand", func(t *testing.T) {
			n := mustparseChunk(t, `h<div {}></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 15}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 9}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{Span: NodeSpan{6, 8}},
										Value:    "",
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 9}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{9, 15}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("non-empty hyperscript attribute shorthand", func(t *testing.T) {
			n := mustparseChunk(t, `h<div {1}></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 16}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 10}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{Span: NodeSpan{6, 9}},
										Value:    "1",
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{10, 16}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("hyperscript attribute shorthand followed by a space", func(t *testing.T) {
			n := mustparseChunk(t, `h<div {} ></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 16}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 10}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{Span: NodeSpan{6, 8}},
										Value:    "",
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{10, 16}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("hyperscript attribute shorthand followed by a dot", func(t *testing.T) {
			n := mustparseChunk(t, `h<div {}.></div>`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 16}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 10}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{Span: NodeSpan{6, 8}},
										Value:    "",
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{10, 16}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("hyperscript attribute shorthand followed by end of line", func(t *testing.T) {
			n, err := parseChunk(t, `h<div {}`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 8}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 8},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING},
									false,
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{Span: NodeSpan{6, 8}},
										Value:    "",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated hyperscript attribute shorthand: end of file", func(t *testing.T) {
			n, err := parseChunk(t, `h<div {`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 7}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 7}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{
											NodeSpan{6, 7},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_HYPERSCRIPT_ATTRIBUTE_SHORTHAND},
											false,
										},
										IsUnterminated: true,
										Value:          "",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("non-empty unterminated hyperscript attribute shorthand: end of file", func(t *testing.T) {
			n, err := parseChunk(t, `h<div {1`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 8}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 8}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{
											NodeSpan{6, 8},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_HYPERSCRIPT_ATTRIBUTE_SHORTHAND},
											false,
										},
										IsUnterminated: true,
										Value:          "1",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("non-empty unterminated hyperscript attribute shorthand: end of file", func(t *testing.T) {
			n, err := parseChunk(t, `h<div {1></div>`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 15}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 15}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{
											NodeSpan{6, 15},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_HYPERSCRIPT_ATTRIBUTE_SHORTHAND},
											false,
										},
										IsUnterminated: true,
										Value:          "1></div>",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("non-empty unterminated hyperscript attribute shorthand: ending with space + end of file", func(t *testing.T) {
			n, err := parseChunk(t, `h<div { `, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 8}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 8}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
								Attributes: []Node{
									&HyperscriptAttributeShorthand{
										NodeBase: NodeBase{
											NodeSpan{6, 8},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_HYPERSCRIPT_ATTRIBUTE_SHORTHAND},
											false,
										},
										IsUnterminated: true,
										Value:          " ",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("self closing", func(t *testing.T) {
			n := mustparseChunk(t, "h<div/>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 7}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 7},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: SELF_CLOSING_TAG_TERMINATOR, Span: NodeSpan{5, 7}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated self closing", func(t *testing.T) {
			n, err := parseChunk(t, "h<div/", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 6}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_SELF_CLOSING_XML_TAG_MISSING_CLOSING},
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: SLASH, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single space", func(t *testing.T) {
			n := mustparseChunk(t, "h<div> </div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 13}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      " ",
									Value:    " ",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{7, 13},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{7, 9}},
										{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("linefeed", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>\n</div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 13}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      "\n",
									Value:    "\n",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{7, 13},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{7, 9}},
										{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("leading interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{1}2</div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
									Expr: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
										Raw:      "1",
										Value:    1,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("trailing interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>1{2}</div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      "1",
									Value:    "1",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
									Expr: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, false},
										Raw:      "2",
										Value:    2,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single curly bracket interpolations should not be parsed in script tags", func(t *testing.T) {
			n := mustparseChunk(t, "h<script>{1}2</script>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 22}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 9},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{8, 9}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 8}, nil, false},
									Name:     "script",
								},
							},
							RawElementContent:       "{1}2",
							RawElementContentStart:  9,
							RawElementContentEnd:    13,
							EstimatedRawElementType: JsScript,
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{13, 22},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{13, 15}},
										{Type: GREATER_THAN, Span: NodeSpan{21, 22}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{15, 21}, nil, false},
									Name:     "script",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("content in script tags should be parsed as raw text", func(t *testing.T) {
			n := mustparseChunk(t, "h<script><a></script>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 21}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 9},
									nil,
									false,
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 8}, nil, false},
									Name:     "script",
								},
							},
							RawElementContent:       "<a>",
							RawElementContentStart:  9,
							RawElementContentEnd:    12,
							EstimatedRawElementType: JsScript,
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{12, 21},
									nil,
									false,
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 20}, nil, false},
									Name:     "script",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("hyperscript script: h marker", func(t *testing.T) {
			n := mustparseChunk(t, "h<script h><a></script>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 23}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 11}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 8}, nil, false},
									Name:     "script",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
											Name:     "h",
										},
									},
								},
							},
							RawElementContent:       "<a>",
							RawElementContentStart:  11,
							RawElementContentEnd:    14,
							EstimatedRawElementType: HyperscriptScript,
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{14, 23}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{16, 22}, nil, false},
									Name:     "script",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("hyperscript script: type=text/hyperscript", func(t *testing.T) {
			n := mustparseChunk(t, "h<script type=\"text/hyperscript\"><a></script>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 45}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 45}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 45}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 33}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 8}, nil, false},
									Name:     "script",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{NodeSpan{9, 32}, nil, false},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{9, 13}, nil, false},
											Name:     "type",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{14, 32}, nil, false},
											Value:    "text/hyperscript",
											Raw:      `"text/hyperscript"`,
										},
									},
								},
							},
							RawElementContent:       "<a>",
							RawElementContentStart:  33,
							RawElementContentEnd:    36,
							EstimatedRawElementType: HyperscriptScript,
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{36, 45}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{38, 44}, nil, false},
									Name:     "script",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("hyperscript script: type=text/hyperscript followed by an attribute", func(t *testing.T) {
			n := mustparseChunk(t, "h<script type=\"text/hyperscript\" n><a></script>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 47}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 47}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 47}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 35}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 8}, nil, false},
									Name:     "script",
								},
								Attributes: []Node{
									&XMLAttribute{
										NodeBase: NodeBase{NodeSpan{9, 32}, nil, false},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{9, 13}, nil, false},
											Name:     "type",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{14, 32}, nil, false},
											Value:    "text/hyperscript",
											Raw:      `"text/hyperscript"`,
										},
									},
									&XMLAttribute{
										NodeBase: NodeBase{NodeSpan{33, 34}, nil, false},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{33, 34}, nil, false},
											Name:     "n",
										},
									},
								},
							},
							RawElementContent:       "<a>",
							RawElementContentStart:  35,
							RawElementContentEnd:    38,
							EstimatedRawElementType: HyperscriptScript,
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{38, 47}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{40, 46}, nil, false},
									Name:     "script",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single curly bracket interpolations should not be parsed in style tags", func(t *testing.T) {
			n := mustparseChunk(t, "h<style>{1}2</style>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 20}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 8},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{7, 8}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 7}, nil, false},
									Name:     "style",
								},
							},
							RawElementContent:       "{1}2",
							RawElementContentStart:  8,
							RawElementContentEnd:    12,
							EstimatedRawElementType: CssStyleElem,
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{12, 20},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{12, 14}},
										{Type: GREATER_THAN, Span: NodeSpan{19, 20}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 19}, nil, false},
									Name:     "style",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("content in style tags should be parsed as raw text", func(t *testing.T) {
			n := mustparseChunk(t, "h<style><a></style>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 19}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 8},
									nil,
									false,
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 7}, nil, false},
									Name:     "style",
								},
							},
							RawElementContent:       "<a>",
							RawElementContentStart:  8,
							RawElementContentEnd:    11,
							EstimatedRawElementType: CssStyleElem,
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{11, 19},
									nil,
									false,
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{13, 18}, nil, false},
									Name:     "style",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("XML expression within interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{h<div></div>}2</div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 27}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{7, 19}, nil, false},
									Expr: &XMLExpression{
										NodeBase: NodeBase{NodeSpan{7, 19}, nil, false},
										Namespace: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
											Name:     "h",
										},
										Element: &XMLElement{
											NodeBase: NodeBase{NodeSpan{8, 19}, nil, false},
											Opening: &XMLOpeningElement{
												NodeBase: NodeBase{Span: NodeSpan{8, 13}},
												Name: &IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{9, 12}, nil, false},
													Name:     "div",
												},
											},
											Children: []Node{
												&XMLText{
													NodeBase: NodeBase{NodeSpan{13, 13}, nil, false},
													Raw:      "",
													Value:    "",
												},
											},
											Closing: &XMLClosingElement{
												NodeBase: NodeBase{Span: NodeSpan{13, 19}},
												Name: &IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{15, 18}, nil, false},
													Name:     "div",
												},
											},
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{21, 27}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{23, 26}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("opening bracket within interpolation", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{{}}2</div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},

						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 17}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
									Expr: &ObjectLiteral{
										NodeBase: NodeBase{NodeSpan{7, 9}, nil, false},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, false},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{11, 17}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty interpolation", func(t *testing.T) {
			n, err := parseChunk(t, "h<div>{}</div>", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 14},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},*/
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{
										NodeSpan{7, 7},
										&ParsingError{UnspecifiedParsingError, EMPTY_XML_INTERP},
										false,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{8, 8}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{8, 14},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{8, 10}},
										{Type: GREATER_THAN, Span: NodeSpan{13, 14}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 13}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty interpolation: linefeed", func(t *testing.T) {
			n, err := parseChunk(t, "h<div>{\n}</div>", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 15},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},*/
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{
										NodeSpan{7, 8},
										&ParsingError{UnspecifiedParsingError, EMPTY_XML_INTERP},
										false,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 9}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{9, 15},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{9, 11}},
										{Type: GREATER_THAN, Span: NodeSpan{14, 15}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: literal preceded by a linefeed", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{\n1}</div>")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: NEWLINE, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 9}},
									Expr: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{8, 9}},
										Raw:      "1",
										Value:    1,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: literal followed by a linefeed", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{1\n}</div>")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},*/
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 9}},
									Expr: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{7, 8}},
										Raw:      "1",
										Value:    1,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: literal followed by a linefeed followed by a literal", func(t *testing.T) {
			n, err := parseChunk(t, "h<div>{1\n2}</div>", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 17}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{
										NodeSpan{7, 10},
										&ParsingError{UnspecifiedParsingError, XML_INTERP_SHOULD_CONTAIN_A_SINGLE_EXPR},
										false,
									},
									Expr: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{7, 8}},
										Raw:      "1",
										Value:    1,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{11, 11}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{11, 17}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{13, 16}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: if expression", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{if true 1 else 2}</div>")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 30}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 30}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 23}},
									Expr: &IfExpression{
										NodeBase: NodeBase{Span: NodeSpan{7, 23}},
										Test: &BooleanLiteral{
											NodeBase: NodeBase{Span: NodeSpan{10, 14}},
											Value:    true,
										},
										Consequent: &IntLiteral{
											NodeBase: NodeBase{Span: NodeSpan{15, 16}},
											Raw:      "1",
											Value:    1,
										},
										Alternate: &IntLiteral{
											NodeBase: NodeBase{Span: NodeSpan{22, 23}},
											Raw:      "2",
											Value:    2,
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{24, 24}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{24, 30}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{26, 29}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: if expression followed by a linefeed", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{if true 1 else 2\n}</div>")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 31}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 31}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 31}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 24}},
									Expr: &IfExpression{
										NodeBase: NodeBase{Span: NodeSpan{7, 23}},
										Test: &BooleanLiteral{
											NodeBase: NodeBase{Span: NodeSpan{10, 14}},
											Value:    true,
										},
										Consequent: &IntLiteral{
											NodeBase: NodeBase{Span: NodeSpan{15, 16}},
											Raw:      "1",
											Value:    1,
										},
										Alternate: &IntLiteral{
											NodeBase: NodeBase{Span: NodeSpan{22, 23}},
											Raw:      "2",
											Value:    2,
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{25, 25}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{25, 31}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{27, 30}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: unterminated if expression", func(t *testing.T) {
			n, err := parseChunk(t, "h<div>{if true}</div>", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 21}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 21}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 14}},
									Expr: &IfExpression{
										NodeBase: NodeBase{Span: NodeSpan{7, 14}},
										Test: &BooleanLiteral{
											NodeBase: NodeBase{Span: NodeSpan{10, 14}},
											Value:    true,
										},
										Consequent: &MissingExpression{
											NodeBase: NodeBase{
												NodeSpan{13, 14},
												&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("h<div>{if true"), 14, true)},
												false,
											},
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{15, 15}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{15, 21}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{17, 20}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: for expression", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{for i in []: i}</div>")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 28}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 21}},
									Expr: &ForExpression{
										NodeBase: NodeBase{Span: NodeSpan{7, 21}, IsParenthesized: true},
										ValueElemIdent: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Name:     "i",
										},
										IteratedValue: &ListLiteral{
											NodeBase: NodeBase{NodeSpan{16, 18}, nil, false},
										},
										Body: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
											Name:     "i",
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{22, 22}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{22, 28}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{24, 27}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: for expression followed by a linefeed", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>{for i in []: i\n}</div>")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 29}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 29}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 29}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 22}},
									Expr: &ForExpression{
										NodeBase: NodeBase{Span: NodeSpan{7, 22}, IsParenthesized: true},
										ValueElemIdent: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Name:     "i",
										},
										IteratedValue: &ListLiteral{
											NodeBase: NodeBase{NodeSpan{16, 18}, nil, false},
										},
										Body: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
											Name:     "i",
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{23, 23}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{23, 29}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{25, 28}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation: unterminated for expression: missing body", func(t *testing.T) {
			n, err := parseChunk(t, "h<div>{for i in []:}</div>", "")
			assert.Error(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 26}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{Span: NodeSpan{7, 19}},
									Expr: &ForExpression{
										NodeBase: NodeBase{Span: NodeSpan{7, 19}, IsParenthesized: true},
										ValueElemIdent: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, false},
											Name:     "i",
										},
										IteratedValue: &ListLiteral{
											NodeBase: NodeBase{NodeSpan{16, 18}, nil, false},
										},
										Body: &MissingExpression{
											NodeBase: NodeBase{
												NodeSpan{18, 19},
												&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("h<div>{for i in []:"), 19, true)},
												false,
											},
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{20, 20}, nil, false},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{20, 26}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{22, 25}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("error within interpolation", func(t *testing.T) {
			n, err := parseChunk(t, "h<div>{?}2</div>", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},

						Element: &XMLElement{
							NodeBase: NodeBase{Span: NodeSpan{1, 16}},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{Span: NodeSpan{1, 6}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{7, 8}, nil, false},
									Expr: &MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{7, 8},
											&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("...div>{?"), 8, true)},
											false,
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 10}, nil, false},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{Span: NodeSpan{10, 16}},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("leading child element", func(t *testing.T) {
			n := mustparseChunk(t, "h<div><span>1</span>2</div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 27}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, false},
									Raw:      "",
									Value:    "",
								},
								&XMLElement{
									NodeBase: NodeBase{NodeSpan{6, 20}, nil, false},
									Opening: &XMLOpeningElement{
										NodeBase: NodeBase{
											NodeSpan{6, 12},
											nil,
											false,
											/*[]Token{
												{Type: LESS_THAN, Span: NodeSpan{6, 7}},
												{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
											},*/
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{7, 11}, nil, false},
											Name:     "span",
										},
									},
									Children: []Node{
										&XMLText{
											NodeBase: NodeBase{NodeSpan{12, 13}, nil, false},
											Raw:      "1",
											Value:    "1",
										},
									},
									Closing: &XMLClosingElement{
										NodeBase: NodeBase{
											NodeSpan{13, 20},
											nil,
											false,
											/*[]Token{
												{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{13, 15}},
												{Type: GREATER_THAN, Span: NodeSpan{19, 20}},
											},*/
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{15, 19}, nil, false},
											Name:     "span",
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, false},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{21, 27},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{21, 23}},
										{Type: GREATER_THAN, Span: NodeSpan{26, 27}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{23, 26}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("linefeed followed by child element", func(t *testing.T) {
			n := mustparseChunk(t, "h<div>\n<span>1</span>2</div>")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, false},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 28}, nil, false},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									false,
									/*[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, false},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, false},
									Raw:      "\n",
									Value:    "\n",
								},
								&XMLElement{
									NodeBase: NodeBase{NodeSpan{7, 21}, nil, false},
									Opening: &XMLOpeningElement{
										NodeBase: NodeBase{
											NodeSpan{7, 13},
											nil,
											false,
											/*[]Token{
												{Type: LESS_THAN, Span: NodeSpan{7, 8}},
												{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
											},*/
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{8, 12}, nil, false},
											Name:     "span",
										},
									},
									Children: []Node{
										&XMLText{
											NodeBase: NodeBase{NodeSpan{13, 14}, nil, false},
											Raw:      "1",
											Value:    "1",
										},
									},
									Closing: &XMLClosingElement{
										NodeBase: NodeBase{
											NodeSpan{14, 21},
											nil,
											false,
											/*[]Token{
												{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{14, 16}},
												{Type: GREATER_THAN, Span: NodeSpan{20, 21}},
											},*/
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{16, 20}, nil, false},
											Name:     "span",
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{21, 22}, nil, false},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{22, 28},
									nil,
									false,
									/*[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{22, 24}},
										{Type: GREATER_THAN, Span: NodeSpan{27, 28}},
									},*/
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{24, 27}, nil, false},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("extend statement", func(t *testing.T) {
		t.Run("unprefixed named pattern", func(t *testing.T) {
			n := mustparseChunk(t, "extend user {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							false,
						},
						ExtendedPattern: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{7, 11}, nil, false},
							Unprefixed: true,
							Name:       "user",
						},
						Extension: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 14},
								nil,
								false,
								/*[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},*/
							},
						},
					},
				},
			}, n)
		})

		t.Run("extension should be an object literal", func(t *testing.T) {
			n, err := parseChunk(t, "extend user 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							false,
						},
						ExtendedPattern: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{7, 11}, nil, false},
							Unprefixed: true,
							Name:       "user",
						},
						Extension: &IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{12, 13},
								&ParsingError{UnspecifiedParsingError, INVALID_EXTENSION_VALUE_AN_OBJECT_LITERAL_WAS_EXPECTED},
								false,
							},
							Raw:   "1",
							Value: 1,
						},
					},
				},
			}, n)
		})

		t.Run("missing extended pattern: 'extend' at end of file", func(t *testing.T) {
			n, err := parseChunk(t, "extend", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, false},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_PATTERN_TO_EXTEND_AFTER_KEYWORD},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("missing extended pattern: 'extend' followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "extend\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 7},
					nil,
					false,
				},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_PATTERN_TO_EXTEND_AFTER_KEYWORD},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("missing extended pattern: 'extend' followed by carriage return + line feed", func(t *testing.T) {
			n, err := parseChunk(t, "extend\r\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 8},
					nil,
					false,
				},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_PATTERN_TO_EXTEND_AFTER_KEYWORD},
							false,
						},
					},
				},
			}, n)
		})

		t.Run("missing extension: pattern at end of file", func(t *testing.T) {
			n, err := parseChunk(t, "extend user", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_OBJECT_LITERAL_AFTER_EXTENDED_PATTERN},
							false,
						},
						ExtendedPattern: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{7, 11}, nil, false},
							Unprefixed: true,
							Name:       "user",
						},
					},
				},
			}, n)
		})

		t.Run("missing extension: pattern followed by line feed", func(t *testing.T) {
			n, err := parseChunk(t, "extend user\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 12},
					nil,
					false,
				},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_OBJECT_LITERAL_AFTER_EXTENDED_PATTERN},
							false,
						},
						ExtendedPattern: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{7, 11}, nil, false},
							Unprefixed: true,
							Name:       "user",
						},
					},
				},
			}, n)
		})

		t.Run("missing extension: pattern followed by carriage return + line feed", func(t *testing.T) {
			n, err := parseChunk(t, "extend user\r\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 13},
					nil,
					false,
				},
				Statements: []Node{
					&ExtendStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnterminatedExtendStmt, UNTERMINATED_EXTEND_STMT_MISSING_OBJECT_LITERAL_AFTER_EXTENDED_PATTERN},
							false,
						},
						ExtendedPattern: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{7, 11}, nil, false},
							Unprefixed: true,
							Name:       "user",
						},
					},
				},
			}, n)
		})
	})

	t.Run("struct definition", func(t *testing.T) {
		t.Run("empty body", func(t *testing.T) {
			n := mustparseChunk(t, "struct Lexer {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, false},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 15}},
						},
					},
				},
			}, n)
		})

		t.Run("body only containing empty lines", func(t *testing.T) {
			n := mustparseChunk(t, "struct Lexer {\n\n}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 17}, nil, false,
						},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 17}},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated empty body: EOF", func(t *testing.T) {
			n, err := parseChunk(t, "struct Lexer {", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							&ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_BODY_MISSING_CLOSING_BRACE},
							false,
						},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 14}},
						},
					},
				},
			}, n)
		})

		t.Run("one field", func(t *testing.T) {
			n := mustparseChunk(t, "struct Lexer {\nindex int\n}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{Span: NodeSpan{0, 26}},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 26}},
							Definitions: []Node{
								&StructFieldDefinition{
									NodeBase: NodeBase{Span: NodeSpan{15, 24}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{15, 20}},
										Name:     "index",
									},
									Type: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{Span: NodeSpan{21, 24}},
										Unprefixed: true,
										Name:       "int",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one field followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "struct Lexer {\nindex int", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							&ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_BODY_MISSING_CLOSING_BRACE},
							false,
						},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 24}},
							Definitions: []Node{
								&StructFieldDefinition{
									NodeBase: NodeBase{Span: NodeSpan{15, 24}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{15, 20}},
										Name:     "index",
									},
									Type: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{Span: NodeSpan{21, 24}},
										Unprefixed: true,
										Name:       "int",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one method", func(t *testing.T) {
			n := mustparseChunk(t, "struct Lexer {\nfn init(){}\n}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{Span: NodeSpan{0, 28}},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 28}},
							Definitions: []Node{
								&FunctionDeclaration{
									NodeBase: NodeBase{Span: NodeSpan{15, 26}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{18, 22}},
										Name:     "init",
									},
									Function: &FunctionExpression{
										NodeBase: NodeBase{Span: NodeSpan{15, 26}},
										Body: &Block{
											NodeBase: NodeBase{Span: NodeSpan{24, 26}},
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char inside body", func(t *testing.T) {
			n, err := parseChunk(t, "struct Lexer {]}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{Span: NodeSpan{0, 16}},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 16}},
							Definitions: []Node{
								&UnknownNode{
									NodeBase: NodeBase{
										NodeSpan{14, 15},
										&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInStructBody(']')},
										false,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char followed by a field", func(t *testing.T) {
			n, err := parseChunk(t, "struct Lexer {] index int}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{Span: NodeSpan{0, 26}},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{13, 26}},
							Definitions: []Node{
								&UnknownNode{
									NodeBase: NodeBase{
										NodeSpan{14, 15},
										&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInStructBody(']')},
										false,
									},
								},
								&StructFieldDefinition{
									NodeBase: NodeBase{Span: NodeSpan{16, 25}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{16, 21}},
										Name:     "index",
									},
									Type: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{Span: NodeSpan{22, 25}},
										Unprefixed: true,
										Name:       "int",
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing body: EOF", func(t *testing.T) {
			n, err := parseChunk(t, "struct Lexer", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_DEF_MISSING_BODY},
							false,
						},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
					},
				},
			}, n)
		})

		t.Run("missing body: linefeed", func(t *testing.T) {
			n, err := parseChunk(t, "struct Lexer\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_DEF_MISSING_BODY},
							false,
						},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 12}},
							Name:       "Lexer",
							Unprefixed: true,
						},
					},
				},
			}, n)
		})
	})

	t.Run("new expression with struct type", func(t *testing.T) {
		t.Run("empty initialization literal", func(t *testing.T) {
			n := mustparseChunk(t, "new Lexer {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&NewExpression{
						NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
						Type: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{4, 9}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Initialization: &StructInitializationLiteral{
							NodeBase: NodeBase{Span: NodeSpan{10, 12}},
						},
					},
				},
			}, n)
		})

		t.Run("body only containing empty lines", func(t *testing.T) {
			n := mustparseChunk(t, "new Lexer {\n\n}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
				Statements: []Node{
					&NewExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, false},
						Type: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{4, 9}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Initialization: &StructInitializationLiteral{
							NodeBase: NodeBase{Span: NodeSpan{10, 14}},
						},
					},
				},
			}, n)
		})

		t.Run("unterminated empty body: EOF", func(t *testing.T) {
			n, err := parseChunk(t, "new Lexer {", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
				Statements: []Node{
					&NewExpression{
						NodeBase: NodeBase{NodeSpan{0, 11}, nil, false},
						Type: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{4, 9}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Initialization: &StructInitializationLiteral{
							NodeBase: NodeBase{
								NodeSpan{10, 11},
								&ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_INIT_LIT_MISSING_CLOSING_BRACE},
								false,
							},
						},
					},
				},
			}, n)
		})

		t.Run("one field", func(t *testing.T) {
			n := mustparseChunk(t, "new Lexer {index: 0}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, false},
				Statements: []Node{
					&NewExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 20}},
						Type: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{4, 9}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Initialization: &StructInitializationLiteral{
							NodeBase: NodeBase{Span: NodeSpan{10, 20}},
							Fields: []Node{
								&StructFieldInitialization{
									NodeBase: NodeBase{Span: NodeSpan{11, 19}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{11, 16}},
										Name:     "index",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{18, 19}},
										Raw:      "0",
										Value:    0,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("one field followed by EOF", func(t *testing.T) {
			n, err := parseChunk(t, "new Lexer {index: 0", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, false},
				Statements: []Node{
					&NewExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 19}},
						Type: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{4, 9}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Initialization: &StructInitializationLiteral{
							NodeBase: NodeBase{
								NodeSpan{10, 19},
								&ParsingError{UnterminatedStructDefinition, UNTERMINATED_STRUCT_INIT_LIT_MISSING_CLOSING_BRACE},
								false,
							},
							Fields: []Node{
								&StructFieldInitialization{
									NodeBase: NodeBase{Span: NodeSpan{11, 19}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{11, 16}},
										Name:     "index",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{18, 19}},
										Raw:      "0",
										Value:    0,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two fields on the same line", func(t *testing.T) {
			n := mustparseChunk(t, "new Lexer {index: 0, id: 0}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, false},
				Statements: []Node{
					&NewExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 27}},
						Type: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{4, 9}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Initialization: &StructInitializationLiteral{
							NodeBase: NodeBase{Span: NodeSpan{10, 27}},
							Fields: []Node{
								&StructFieldInitialization{
									NodeBase: NodeBase{Span: NodeSpan{11, 19}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{11, 16}},
										Name:     "index",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{18, 19}},
										Raw:      "0",
										Value:    0,
									},
								},
								&StructFieldInitialization{
									NodeBase: NodeBase{Span: NodeSpan{21, 26}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{21, 23}},
										Name:     "id",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{25, 26}},
										Raw:      "0",
										Value:    0,
									},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two fields on separate lines", func(t *testing.T) {
			n := mustparseChunk(t, "new Lexer {index: 0\nid: 0}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, false},
				Statements: []Node{
					&NewExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 26}},
						Type: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{4, 9}},
							Name:       "Lexer",
							Unprefixed: true,
						},
						Initialization: &StructInitializationLiteral{
							NodeBase: NodeBase{Span: NodeSpan{10, 26}},
							Fields: []Node{
								&StructFieldInitialization{
									NodeBase: NodeBase{Span: NodeSpan{11, 19}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{11, 16}},
										Name:     "index",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{18, 19}},
										Raw:      "0",
										Value:    0,
									},
								},
								&StructFieldInitialization{
									NodeBase: NodeBase{Span: NodeSpan{20, 25}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{20, 22}},
										Name:     "id",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{Span: NodeSpan{24, 25}},
										Raw:      "0",
										Value:    0,
									},
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("pointer type", func(t *testing.T) {
		t.Run("in pattern definition", func(t *testing.T) {
			n := mustparseChunk(t, "pattern p = *Lexer")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, false},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{Span: NodeSpan{0, 18}},
						Left: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{8, 9}, nil, false},
							Name:       "p",
							Unprefixed: true,
						},
						Right: &PointerType{
							NodeBase: NodeBase{NodeSpan{12, 18}, nil, false},
							ValueType: &PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{13, 18}, nil, false},
								Unprefixed: true,
								Name:       "Lexer",
							},
						},
					},
				},
			}, n)
		})

		t.Run("as parameter type", func(t *testing.T) {
			n := mustparseChunk(t, "fn(x *int){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 12}},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{Span: NodeSpan{3, 9}},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{3, 4}},
									Name:     "x",
								},
								Type: &PointerType{
									NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
									ValueType: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{6, 9}, nil, false},
										Unprefixed: true,
										Name:       "int",
									},
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{Span: NodeSpan{10, 12}},
						},
					},
				},
			}, n)
		})

		t.Run("as return type", func(t *testing.T) {
			n := mustparseChunk(t, "fn() *int {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, false},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{Span: NodeSpan{0, 12}},
						ReturnType: &PointerType{
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, false},
							ValueType: &PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{6, 9}, nil, false},
								Unprefixed: true,
								Name:       "int",
							},
						},
						Body: &Block{
							NodeBase: NodeBase{Span: NodeSpan{10, 12}},
						},
					},
				},
			}, n)
		})

		t.Run("as a local variable's type", func(t *testing.T) {
			n := mustparseChunk(t, "var i *int = nil")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{Span: NodeSpan{0, 16}},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{Span: NodeSpan{4, 16}},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{4, 5}},
									Name:     "i",
								},
								Type: &PointerType{
									NodeBase: NodeBase{Span: NodeSpan{6, 10}},
									ValueType: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{7, 10}, nil, false},
										Unprefixed: true,
										Name:       "int",
									},
								},
								Right: &NilLiteral{
									NodeBase: NodeBase{Span: NodeSpan{13, 16}},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("as a global variable's type", func(t *testing.T) {
			n := mustparseChunk(t, "globalvar i *int = nil")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, false},
				Statements: []Node{
					&GlobalVariableDeclarations{
						NodeBase: NodeBase{Span: NodeSpan{0, 22}},
						Declarations: []*GlobalVariableDeclaration{
							{
								NodeBase: NodeBase{Span: NodeSpan{10, 22}},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{10, 11}},
									Name:     "i",
								},
								Type: &PointerType{
									NodeBase: NodeBase{Span: NodeSpan{12, 16}},
									ValueType: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{13, 16}, nil, false},
										Unprefixed: true,
										Name:       "int",
									},
								},
								Right: &NilLiteral{
									NodeBase: NodeBase{Span: NodeSpan{19, 22}},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("as a struct field's type", func(t *testing.T) {
			n := mustparseChunk(t, "struct I{v *int}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, false},
				Statements: []Node{
					&StructDefinition{
						NodeBase: NodeBase{Span: NodeSpan{0, 16}},
						Name: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{Span: NodeSpan{7, 8}},
							Unprefixed: true,
							Name:       "I",
						},
						Body: &StructBody{
							NodeBase: NodeBase{Span: NodeSpan{8, 16}},
							Definitions: []Node{
								&StructFieldDefinition{
									NodeBase: NodeBase{Span: NodeSpan{9, 15}},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{Span: NodeSpan{9, 10}},
										Name:     "v",
									},
									Type: &PointerType{
										NodeBase: NodeBase{NodeSpan{11, 15}, nil, false},
										ValueType: &PatternIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{12, 15}, nil, false},
											Unprefixed: true,
											Name:       "int",
										},
									},
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("dereference expression", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			n := mustparseChunk(t, "*x")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
				Statements: []Node{
					&DereferenceExpression{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, false},
						Pointer: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, false},
							Name:     "x",
						},
					},
				},
			}, n)
		})

		t.Run("parenthsized", func(t *testing.T) {
			n := mustparseChunk(t, "(*x)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, false},
				Statements: []Node{
					&DereferenceExpression{
						NodeBase: NodeBase{NodeSpan{1, 3}, nil, true},
						Pointer: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, false},
							Name:     "x",
						},
					},
				},
			}, n)
		})
	})
}

func TestParsePath(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		p, ok := ParsePath("")
		assert.False(t, ok)
		assert.Empty(t, p)
	})
}

func parseChunkForgetTokens(s, name string, opts ...ParserOptions) (*Chunk, error) {
	c, err := ParseChunk(s, name, opts...)
	if c != nil {
		c.Tokens = nil
		Walk(c, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
			if mod, ok := node.(*EmbeddedModule); ok {
				mod.Tokens = nil
			}
			return ContinueTraversal, nil
		}, nil)
	}
	return c, err
}

func mustParseChunkForgetTokens(s string, opts ...ParserOptions) *Chunk {
	c := MustParseChunk(s, opts...)
	c.Tokens = nil
	Walk(c, func(node, parent, scopeNode Node, ancestorChain []Node, after bool) (TraversalAction, error) {
		if mod, ok := node.(*EmbeddedModule); ok {
			mod.Tokens = nil
		}
		return ContinueTraversal, nil
	}, nil)
	return c
}
