package parse

import (
	"context"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestParseNoContext(t *testing.T) {
	testParse(t, func(t *testing.T, str string) (result *Chunk) {
		return MustParseChunk(str)
	}, func(t *testing.T, str, name string) (result *Chunk, err error) {
		return ParseChunk(str, name)
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
			MustParseChunk(str, parserOptions{
				noCheckFuel: 1, //check context every major function call during parsing.
				context:     ctx,
			})
			return
		})()

		assert.ErrorContains(t, err, context.Canceled.Error())

		return MustParseChunk(str)
	}

	parseChunk := func(t *testing.T, str, name string) (result *Chunk, e error) {
		_, err := ParseChunk(str, name, parserOptions{
			noCheckFuel: 1, //check context every major function call during parsing.
			context:     ctx,
		})

		assert.ErrorContains(t, err, context.Canceled.Error())

		return ParseChunk(str, name)
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
			MustParseChunk(str, parserOptions{
				noCheckFuel: 2, //check context every 2 major function calls during parsing.
				context:     ctx,
			})
			return
		})()

		assert.ErrorContains(t, err, context.Canceled.Error())

		return MustParseChunk(str)
	}

	parseChunk := func(t *testing.T, str, name string) (result *Chunk, e error) {
		_, err := ParseChunk(str, name, parserOptions{
			noCheckFuel: 2, //check context every 2 major function calls during parsing.
			context:     ctx,
		})

		assert.ErrorContains(t, err, context.Canceled.Error())

		return ParseChunk(str, name)
	}

	testParse(t, mustParseChunk, parseChunk)
}

func TestParseNxxxx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	const MIN_CONTEXT_CHECK_TEST_NODE_COUNT = 6

	mustParseChunk := func(t *testing.T, str string) (result *Chunk) {
		n := MustParseChunk(str)
		nodeCount := countNodes(n)

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
			MustParseChunk(str, parserOptions{
				noCheckFuel: nodeCount / 2, //check context somewhere during the parsing.
				context:     ctx,
			})
			return
		})()

		assert.ErrorContains(t, err, context.Canceled.Error())

		return n
	}

	parseChunk := func(t *testing.T, str, name string) (result *Chunk, e error) {
		n, err := ParseChunk(str, name)
		nodeCount := countNodes(n)

		if nodeCount < MIN_CONTEXT_CHECK_TEST_NODE_COUNT { //ignore context check test.
			return n, err
		}

		_, err = ParseChunk(str, name, parserOptions{
			noCheckFuel: nodeCount / 2, //check context somewhere during the parsing.
			context:     ctx,
		})

		assert.ErrorContains(t, err, context.Canceled.Error())

		return ParseChunk(str, name)
	}

	testParse(t, mustParseChunk, parseChunk)
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
				NodeBase: NodeBase{NodeSpan{0, 0}, nil, nil},
			}, n)
		})

		t.Run("comment with missing space", func(t *testing.T) {
			n, err := parseChunk(t, "#", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
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
				NodeBase:   NodeBase{NodeSpan{0, 21}, nil, nil},
				Statements: nil,
			}, n)
		})

		t.Run("unexpected char", func(t *testing.T) {
			n, err := parseChunk(t, "]", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
							[]Token{{Type: UNEXPECTED_CHAR, Raw: "]", Span: NodeSpan{0, 1}}},
						},
					},
				},
			}, n)
		})

		t.Run("non regular space", func(t *testing.T) {
			n, err := parseChunk(t, " ", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(' ')},
							[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{0, 1}, Raw: " "}},
						},
					},
				},
			}, n)
		})

		t.Run("carriage return", func(t *testing.T) {
			n := mustparseChunk(t, "\r")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
			}, n)
		})

		t.Run("line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 1},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{0, 1}}},
				},
			}, n)
		})

		t.Run("two line feeds", func(t *testing.T) {
			n := mustparseChunk(t, "\n\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{0, 1}},
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
					},
				},
			}, n)
		})

		t.Run("carriage return + line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\r\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{1, 2}}},
				},
			}, n)
		})

		t.Run("twice: carriage return + line feed", func(t *testing.T) {
			n := mustparseChunk(t, "\r\n\r\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 4},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
						{Type: NEWLINE, Span: NodeSpan{3, 4}},
					},
				},
			}, n)
		})

		t.Run("two lines with one statement per line", func(t *testing.T) {
			n := mustparseChunk(t, "1\n2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 3},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
					},
				},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Raw:      "1",
						Value:    1,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
						Raw:      "2",
						Value:    2,
					},
				},
			}, n)
		})

		t.Run("two lines with one statement per line, followed by newline character", func(t *testing.T) {
			n := mustparseChunk(t, "1\n2\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 4},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{1, 2}},
						{Type: NEWLINE, Span: NodeSpan{3, 4}},
					},
				},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Raw:      "1",
						Value:    1,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Raw:      "1",
						Value:    1,
					},
					&Variable{
						NodeBase: NodeBase{
							NodeSpan{1, 3},
							&ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY},
							nil,
						},
						Name: "v",
					},
				},
			}, n)
		})

		t.Run("empty preinit", func(t *testing.T) {
			n := mustparseChunk(t, "preinit {}")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span: NodeSpan{0, 10},
						Tokens: []Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{0, 7}},
						},
					},
					Block: &Block{
						NodeBase: NodeBase{
							NodeSpan{8, 10},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty preinit after newline", func(t *testing.T) {
			n := mustparseChunk(t, "\npreinit {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 11},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{0, 1}}},
				},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span: NodeSpan{1, 11},
						Tokens: []Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{1, 8}},
						},
					},
					Block: &Block{
						NodeBase: NodeBase{
							NodeSpan{9, 11},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("preinit with missing block", func(t *testing.T) {
			n, err := parseChunk(t, "preinit", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span: NodeSpan{0, 7},
						Err:  &ParsingError{UnspecifiedParsingError, PREINIT_KEYWORD_SHOULD_BE_FOLLOWED_BY_A_BLOCK},
						Tokens: []Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{0, 7}},
						},
					},
				},
			}, n)
		})

		t.Run("empty manifest", func(t *testing.T) {
			n := mustparseChunk(t, "manifest {}")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: nil,
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span: NodeSpan{0, 11},
						Tokens: []Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{0, 8}},
						},
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{9, 11},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
							},
						},
						Properties: nil,
					},
				},
			}, n)
		})

		t.Run("empty manifest after newline", func(t *testing.T) {
			n := mustparseChunk(t, "\nmanifest {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 12},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{0, 1}},
					},
				},
				Statements: nil,
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span: NodeSpan{1, 12},
						Tokens: []Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{1, 9}},
						},
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{10, 12},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
							},
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
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{10, 11}},
					},
				},
				Statements: nil,
				Preinit: &PreinitStatement{
					NodeBase: NodeBase{
						Span: NodeSpan{0, 10},
						Tokens: []Token{
							{Type: PREINIT_KEYWORD, Span: NodeSpan{0, 7}},
						},
					},
					Block: &Block{
						NodeBase: NodeBase{
							NodeSpan{8, 10},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
							},
						},
					},
				},
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span: NodeSpan{11, 22},
						Tokens: []Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{11, 19}},
						},
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{20, 22},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
							},
						},
						Properties: nil,
					},
				},
			}, n)
		})

		t.Run("includable-chunk", func(t *testing.T) {
			n := mustparseChunk(t, "includable-chunk")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: nil,
				IncludableChunkDesc: &IncludableChunkDescription{
					NodeBase: NodeBase{
						Span:   NodeSpan{0, 16},
						Tokens: []Token{{Type: INCLUDABLE_CHUNK_KEYWORD, Span: NodeSpan{0, 16}}},
					},
				},
			}, n)
		})

		t.Run("empty manifest after newline", func(t *testing.T) {
			n := mustparseChunk(t, "\nincludable-chunk")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 17},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{0, 1}},
					},
				},
				Statements: nil,
				IncludableChunkDesc: &IncludableChunkDescription{
					NodeBase: NodeBase{
						Span:   NodeSpan{1, 17},
						Tokens: []Token{{Type: INCLUDABLE_CHUNK_KEYWORD, Span: NodeSpan{1, 17}}},
					},
				},
			}, n)
		})
	})

	t.Run("top level constant declarations", func(t *testing.T) {
		t.Run("empty const declarations", func(t *testing.T) {
			n := mustparseChunk(t, "const ()")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 8},
						nil,
						[]Token{
							{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}},
							{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
							{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
						},
					},
					Declarations: nil,
				},
			}, n)
		})

		t.Run("single declaration with parenthesis", func(t *testing.T) {
			n := mustparseChunk(t, "const ( a = 1 )")
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 15},
						nil,
						[]Token{
							{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}},
							{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
							{Type: CLOSING_PARENTHESIS, Span: NodeSpan{14, 15}},
						},
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{8, 13},
								nil,
								[]Token{{Type: EQUAL, Span: NodeSpan{10, 11}}},
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
								Name:     "a",
							},
							Right: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
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
				NodeBase:   NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 11},
						nil,
						[]Token{{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}}},
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{6, 11},
								nil,
								[]Token{{Type: EQUAL, Span: NodeSpan{8, 9}}},
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
								Name:     "a",
							},
							Right: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
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
				NodeBase:   NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 5},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
						[]Token{{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}}},
					},
					Declarations: nil,
				},
			}, n)
		})

		t.Run("const keyword followed by space + EOF", func(t *testing.T) {
			n, err := parseChunk(t, "const ", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 6},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
						[]Token{{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}}},
					},
					Declarations: nil,
				},
			}, n)
		})

		t.Run("const keyword followed by a literal", func(t *testing.T) {
			n, err := parseChunk(t, "const 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase:   NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 7},
						nil,
						[]Token{{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}}},
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{6, 7},
								&ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_MISSING_EQL_SIGN},
								nil,
							},
							Left: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase:   NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: nil,
				Manifest:   nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 9},
						nil,
						[]Token{{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}}},
					},
					Declarations: []*GlobalConstantDeclaration{
						{
							NodeBase: NodeBase{
								NodeSpan{6, 9},
								&ParsingError{UnspecifiedParsingError, INVALID_GLOBAL_CONST_DECL_LHS_MUST_BE_AN_IDENT},
								[]Token{{Type: EQUAL, Span: NodeSpan{8, 9}}},
							},
							Left: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Right: &MissingExpression{
								NodeBase: NodeBase{
									NodeSpan{8, 9},
									&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("const 1 ="), 9, true)},
									nil,
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
					[]Token{{Type: NEWLINE, Span: NodeSpan{5, 6}}},
				},
				Statements: nil,
				GlobalConstantDeclarations: &GlobalConstantDeclarations{
					NodeBase: NodeBase{
						NodeSpan{0, 5},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_GLOBAL_CONS_DECLS},
						[]Token{{Type: CONST_KEYWORD, Span: NodeSpan{0, 5}}},
					},
					Declarations: nil,
				},
				Manifest: &Manifest{
					NodeBase: NodeBase{
						Span: NodeSpan{6, 17},
						Tokens: []Token{
							{Type: MANIFEST_KEYWORD, Span: NodeSpan{6, 14}},
						},
					},
					Object: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{15, 17},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: nil,
					},
				},
			}, n)
		})

		t.Run("single declaration", func(t *testing.T) {
			n := mustparseChunk(t, "var ( a = 1 )")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{6, 11},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{8, 9}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Name:     "a",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 9},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{6, 7}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 14},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{11, 12}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 10}, nil, nil},
									Name:     "int",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 13},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{10, 11}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 9}, nil, nil},
									Unprefixed: true,
									Name:       "int",
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 13},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{10, 11}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
								Type: &PatternNamespaceMemberExpression{
									NodeBase: NodeBase{NodeSpan{6, 9}, nil, nil},
									Namespace: &PatternNamespaceIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{6, 8}, nil, nil},
										Unprefixed: true,
										Name:       "x",
									},
									MemberName: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
										Name:     "y",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 15},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{12, 13}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{
										Span: NodeSpan{6, 11},
										Tokens: []Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
										},
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
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 14},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{11, 12}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
								Type: &OptionalPatternExpression{
									NodeBase: NodeBase{
										Span: NodeSpan{6, 10},
									},
									Pattern: &PatternIdentifierLiteral{
										NodeBase:   NodeBase{NodeSpan{6, 9}, nil, nil},
										Unprefixed: true,
										Name:       "int",
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 15},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{12, 13}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
								Type: &PatternCallExpression{
									NodeBase: NodeBase{
										Span: NodeSpan{6, 11},
										Tokens: []Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
										},
									},
									Callee: &PatternNamespaceMemberExpression{
										NodeBase: NodeBase{NodeSpan{6, 9}, nil, nil},
										Namespace: &PatternNamespaceIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{6, 8}, nil, nil},
											Unprefixed: true,
											Name:       "a",
										},
										MemberName: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
											Name:     "b",
										},
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
						Declarations: []*LocalVariableDeclaration{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 15},
									nil,
									[]Token{{Type: EQUAL, Span: NodeSpan{12, 13}}},
								},
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
												Span: NodeSpan{9, 11},
												Tokens: []Token{
													{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
												},
											},
										},
									},
								},
								Right: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
					Statements: []Node{
						&LocalVariableDeclarations{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								nil,
								[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
							},
							Declarations: []*LocalVariableDeclaration{
								{
									NodeBase: NodeBase{
										NodeSpan{4, 15},
										nil,
										[]Token{{Type: EQUAL, Span: NodeSpan{12, 13}}},
									},
									Left: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
										Name:     "a",
									},
									Type: &PatternCallExpression{
										NodeBase: NodeBase{Span: NodeSpan{6, 11}},
										Callee: &PatternNamespaceMemberExpression{
											NodeBase: NodeBase{NodeSpan{6, 9}, nil, nil},
											Namespace: &PatternNamespaceIdentifierLiteral{
												NodeBase:   NodeBase{NodeSpan{6, 8}, nil, nil},
												Unprefixed: true,
												Name:       "a",
											},
											MemberName: &IdentifierLiteral{
												NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
												Name:     "b",
											},
										},
										Arguments: []Node{
											&ObjectPatternLiteral{
												NodeBase: NodeBase{
													Span: NodeSpan{9, 11},
													Tokens: []Token{
														{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
														{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
													},
												},
											},
										},
									},
									Right: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
					},
				},
			}, n)
		})

		t.Run("var keyword followed by newline", func(t *testing.T) {
			n, err := parseChunk(t, "var\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 4},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{3, 4}}},
				},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
					},
				},
			}, n)
		})

		t.Run("var keyword followed by newline + expression", func(t *testing.T) {
			n, err := parseChunk(t, "var\n1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 5},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{3, 4}}},
				},
				Statements: []Node{
					&LocalVariableDeclarations{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LOCAL_VAR_DECLS},
							[]Token{{Type: VAR_KEYWORD, Span: NodeSpan{0, 3}}},
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("single declaration with invalid LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "var %| %int | %str", "")
			assert.NotNil(t, mod)
			assert.Error(t, err)
		})

		t.Run("single declaration with keyword LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "var manifest", "")
			assert.NotNil(t, mod)
			assert.Error(t, err)
		})

		t.Run("single declaration with invalid LHS", func(t *testing.T) {
			mod, err := parseChunk(t, "var 1 = 1", "")
			assert.NotNil(t, mod)
			assert.Error(t, err)
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

	t.Run("variable", func(t *testing.T) {
		n := mustparseChunk(t, "$a")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
			Statements: []Node{
				&Variable{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Name:     "a",
				},
			},
		}, n)
	})

	t.Run("identifier", func(t *testing.T) {

		t.Run("single letter", func(t *testing.T) {
			n := mustparseChunk(t, "a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Name:     "a",
					},
				},
			}, n)
		})

		t.Run("followed by newline", func(t *testing.T) {
			n := mustparseChunk(t, "a\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{1, 2}}},
				},
				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&BooleanLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Value:    true,
					},
				},
			}, n)
		})

		t.Run("false", func(t *testing.T) {
			n := mustparseChunk(t, "false")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&BooleanLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Value:    false,
					},
				},
			}, n)
		})

	})

	t.Run("property name", func(t *testing.T) {
		n := mustparseChunk(t, ".a")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
			Statements: []Node{
				&PropertyNameLiteral{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Name:     "a",
				},
			},
		}, n)
	})

	t.Run("flag literal", func(t *testing.T) {
		t.Run("single hyphen followed by a single letter", func(t *testing.T) {
			n := mustparseChunk(t, "-a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&FlagLiteral{
						NodeBase:   NodeBase{NodeSpan{0, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&FlagLiteral{
						NodeBase:   NodeBase{NodeSpan{0, 3}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&FlagLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, OPTION_NAME_CAN_ONLY_CONTAIN_ALPHANUM_CHARS},
							nil,
						},
						Name:       "",
						SingleDash: true,
						Raw:        "-",
					},
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{1, 2},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule('?')},
							[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{1, 2}, Raw: "?"}},
						},
					},
				},
			}, n)
		})

		t.Run("flag literal : double dash", func(t *testing.T) {
			n := mustparseChunk(t, "--abc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&FlagLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&OptionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{6, 7}}},
						},
						Name: "name",
						Value: &QuotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{7, 12}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&OptionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnspecifiedParsingError, "unterminated option expression, '=' should be followed by an expression"},
							[]Token{{Type: EQUAL, Span: NodeSpan{6, 7}}},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&OptionPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPION_PATTERN_A_VALUE_IS_EXPECTED},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&OptionPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPION_PATT_EQUAL_ASSIGN_SHOULD_BE_FOLLOWED_BY_EXPR},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&OptionPatternLiteral{
						NodeBase:   NodeBase{NodeSpan{0, 12}, nil, nil},
						Name:       "name",
						SingleDash: false,
						Value: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 12}, nil, nil},
							Name:     "foo",
						},
					},
				},
			}, n)
		})

		t.Run("unprefixed", func(t *testing.T) {
			n := mustparseChunk(t, `%p = --name=int`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							Span:   NodeSpan{0, 15},
							Tokens: []Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 2}},
							Name:     "p",
						},
						Right: &OptionPatternLiteral{
							NodeBase:   NodeBase{NodeSpan{5, 15}, nil, nil},
							Name:       "name",
							SingleDash: false,
							Unprefixed: true,
							Value: &PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{12, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Raw:      "/",
						Value:    "/",
					},
				},
			}, n)
		})

		t.Run("quoted absolute path literal : /`[]`", func(t *testing.T) {
			n := mustparseChunk(t, "/`[]`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Raw:      "/`[]`",
						Value:    "/[]",
					},
				},
			}, n)
		})

		t.Run("unquoted absolute path literal : /a", func(t *testing.T) {
			n := mustparseChunk(t, "/a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&AbsolutePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
						Raw:      "/a",
						Value:    "/a",
					},
				},
			}, n)
		})

		t.Run("relative path literal : ./", func(t *testing.T) {
			n := mustparseChunk(t, "./")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&RelativePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
						Raw:      "./",
						Value:    "./",
					},
				},
			}, n)
		})

		t.Run("relative path literal : ./a", func(t *testing.T) {
			n := mustparseChunk(t, "./a")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&RelativePathLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
						Raw:      "./a",
						Value:    "./a",
					},
				},
			}, n)
		})

		t.Run("relative path literal in list : [./]", func(t *testing.T) {
			n := mustparseChunk(t, "[./]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&ListLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{
								{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
							},
						},
						Elements: []Node{
							&RelativePathLiteral{
								NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
								Raw:      "./",
								Value:    "./",
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("path pattern", func(t *testing.T) {
		t.Run("absolute path pattern literal : /a*", func(t *testing.T) {
			n := mustparseChunk(t, "%/a*")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Raw:      "%/a*",
						Value:    "/a*",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal : /a[a-z]", func(t *testing.T) {
			n := mustparseChunk(t, "%/`a[a-z]`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
						Raw:      "%/`a[a-z]`",
						Value:    "/a[a-z]",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal ending with /... ", func(t *testing.T) {
			n := mustparseChunk(t, "%/a/...")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Raw:      "%/a/...",
						Value:    "/a/...",
					},
				},
			}, n)
		})

		t.Run("absolute path pattern literal : /... ", func(t *testing.T) {
			n := mustparseChunk(t, "%/...")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, fmtSlashDotDotDotCanOnlyBePresentAtEndOfPathPattern("/a/.../b")},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&AbsolutePathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnspecifiedParsingError, fmtSlashDotDotDotCanOnlyBePresentAtEndOfPathPattern("/a/.../...")},
							nil,
						},
						Raw:   "%/a/.../...",
						Value: "/a/.../...",
					},
				},
			}, n)
		})

	})

	t.Run("named-segment path pattern literal  ", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, "%/home/{:username}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&NamedSegmentPathPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{
								{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{7, 8}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{17, 18}},
							}},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 7}, nil, nil},
								Value:    "/home/",
							},
							&NamedPathSegment{
								NodeBase: NodeBase{NodeSpan{8, 17}, nil, nil},
								Name:     "username",
							},
						},
						Raw:         "%/home/{:username}",
						StringValue: "%/home/{:username}",
					},
				},
			}, n)
		})

		//TODO: improve following tests

		t.Run("invalid named-segment path pattern literals", func(t *testing.T) {
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
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&PathPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{
								{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{7, 8}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{17, 18}},
							},
						},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 7}, nil, nil},
								Value:    "/home/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{8, 17}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&PathPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{7, 8}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{8, 9}},
							},
						},
						Slices: []Node{
							&PathPatternSlice{
								NodeBase: NodeBase{NodeSpan{1, 7}, nil, nil},
								Value:    "/home/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{8, 8},
									&ParsingError{UnspecifiedParsingError, EMPTY_PATH_INTERP},
									[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{8, 8}}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{16, 17}},
							},
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
								Value:    "/home/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{16, 17}},
							},
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
								Value:    "/home/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, nil},
								Name:     "username",
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{17, 26}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{15, 16}},
							},
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
								Value:    "/home/",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
							},
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
								Value:    "/home/",
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{7, 7}, &ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP}, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, []Token{{Type: NEWLINE, Span: NodeSpan{7, 8}}}},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
							},
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
								Value:    "/home/",
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{7, 7}, &ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP}, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
				Statements: []Node{
					&AbsolutePathExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							&ParsingError{UnspecifiedParsingError, ONLY_PATH_PATTERNS_CAN_CONTAIN_NAMED_SEGMENTS},
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{6, 7}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{16, 17}},
							},
						},
						Slices: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
								Value:    "/home/",
							},
							&NamedPathSegment{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&RegularExpressionLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
						Value:    "",
						Raw:      "%``",
					},
				},
			}, n)
		})

		t.Run("not empty", func(t *testing.T) {
			n := mustparseChunk(t, "%`a+`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&RegularExpressionLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&RegularExpressionLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, &ParsingError{UnspecifiedParsingError, UNTERMINATED_REGEX_LIT}, nil},
						Value:    "",
						Raw:      "%`",
					},
				},
			}, n)
		})
	})

	t.Run("nil literal", func(t *testing.T) {
		n := mustparseChunk(t, "nil")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
			Statements: []Node{
				&NilLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				},
			},
		}, n)
	})

	t.Run("self expression", func(t *testing.T) {
		n := mustparseChunk(t, "self")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
			Statements: []Node{
				&SelfExpression{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				},
			},
		}, n)
	})

	t.Run("member expression", func(t *testing.T) {
		t.Run("variable '.' <single letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 5}, nil, nil},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run(" variable '.' <propname> '.' <single-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b.c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.?' <name>", func(t *testing.T) {
			n := mustparseChunk(t, "$a.?b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
							Name:     "cd",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.?' <prop> '.' <prop name> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.?b.c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
								Name:     "b",
							},
							Optional: true,
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
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
					[]Token{{Type: NEWLINE, Span: NodeSpan{3, 4}}},
				},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: nil,
					},
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
						Name:     "b",
					},
				},
			}, n)
		})

		t.Run("missing property name: followed by closing delim", func(t *testing.T) {
			n, err := parseChunk(t, "$a.]", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: nil,
					},
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{3, 4},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
							[]Token{{Type: UNEXPECTED_CHAR, Raw: "]", Span: NodeSpan{3, 4}}},
						},
					},
				},
			}, n)
		})

		t.Run("long member expression : unterminated", func(t *testing.T) {
			n, err := parseChunk(t, "$a.b.", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 8},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Left: &SelfExpression{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, nil},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("call '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "a().bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Left: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{2, 3}},
								},
							},
							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, nil},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("member of a parenthesized expression", func(t *testing.T) {
			n := mustparseChunk(t, "($a).name")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{
								NodeSpan{1, 3},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},
							},
							Name: "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
							Name:     "name",
						},
					},
				},
			}, n)
		})

		t.Run("optional member of an identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.b.?c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Left: &IdentifierMemberExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								nil,
							},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Name:     "b",
								},
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
							Name:     "c",
						},
						Optional: true,
					},
				},
			}, n)
		})
	})

	t.Run("computed member expression", func(t *testing.T) {
		t.Run("variable '.' '(' <var> ')'", func(t *testing.T) {
			n := mustparseChunk(t, "$a.(b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&ComputedMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{
								NodeSpan{4, 5},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&ComputedMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{
								NodeSpan{3, 4},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&ComputedMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
						Left: &ComputedMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{
									NodeSpan{4, 5},
									nil,
									[]Token{
										{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
										{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
									},
								},
								Name: "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{
								NodeSpan{8, 9},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{7, 8}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.<' <single letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.<b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.<bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, nil},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run(" variable '.' <propname> '.<' <single-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b.<c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.' <propname> '.<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.b.<cd")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
							Name:     "cd",
						},
					},
				},
			}, n)
		})

		t.Run("variable '.<' <propname> '<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "$a.<b.cd")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
						Left: &DynamicMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
							Name:     "cd",
						},
					},
				},
			}, n)
		})

		t.Run("identifier '.<' <propname> '<' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "a.<b.cd")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&MemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Left: &DynamicMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_DYN_MEMB_OR_INDEX_EXPR},
							nil,
						},
						Left: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Left: &SelfExpression{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, nil},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 9}, nil, nil},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("call '.' <two-letter propname> ", func(t *testing.T) {
			n := mustparseChunk(t, "a().<bc")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Left: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{2, 3}},
								},
							},
							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
							Name:     "bc",
						},
					},
				},
			}, n)
		})

		t.Run("member of a parenthesized expression", func(t *testing.T) {
			n := mustparseChunk(t, "($a).<name")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&DynamicMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
						Left: &Variable{
							NodeBase: NodeBase{
								NodeSpan{1, 3},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},
							},
							Name: "a",
						},
						PropertyName: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "http",
						},
						PropertyNames: []*IdentifierLiteral{
							{
								NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, nil},
							Name:     "http",
						},
						PropertyNames: []*IdentifierLiteral{
							{
								NodeBase: NodeBase{NodeSpan{6, 9}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{1, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, nil},
							Name:     "http",
						},
						PropertyNames: []*IdentifierLiteral{
							{
								NodeBase: NodeBase{NodeSpan{6, 9}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR},
							[]Token{{Type: DOT, Span: NodeSpan{4, 5}}},
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
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
					[]Token{{Type: NEWLINE, Span: NodeSpan{5, 6}}},
				},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR},
							[]Token{{Type: DOT, Span: NodeSpan{4, 5}}},
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "http",
						},
						PropertyNames: nil,
					},
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
						Name:     "a",
					},
				},
			}, n)
		})

		t.Run("missing last property name, followed by a closing delimiter", func(t *testing.T) {
			n, err := parseChunk(t, "http.]", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&IdentifierMemberExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnterminatedMemberExpr, UNTERMINATED_IDENT_MEMB_EXPR},
							[]Token{{Type: DOT, Span: NodeSpan{4, 5}}},
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "http",
						},
						PropertyNames: nil,
					},
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{5, 6},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
							[]Token{{Type: UNEXPECTED_CHAR, Raw: "]", Span: NodeSpan{5, 6}}},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
						Object: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{2, 9},
								nil,
								[]Token{{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{2, 4}}, {Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}}},
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
						Object: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "a",
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{1, 8},
								nil,
								[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{1, 3}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
						Object: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Name:     "b",
								},
							},
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{3, 10},
								nil,
								[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{3, 5}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
						Object: &ComputedMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyName: &QuotedStringLiteral{
								NodeBase: NodeBase{
									NodeSpan{3, 6},
									nil,
									[]Token{
										{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
										{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
									},
								},
								Raw:   `"b"`,
								Value: "b",
							},
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{7, 14},
								nil,
								[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{7, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&ExtractionExpression{
						NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
						Object: &DynamicMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						Keys: &KeyListExpression{
							NodeBase: NodeBase{
								NodeSpan{4, 11},
								nil,
								[]Token{
									{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{4, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Keys: []Node{
								&IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 10}, nil, nil},
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
			NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
			Statements: []Node{
				&Variable{
					NodeBase: NodeBase{
						NodeSpan{1, 3},
						nil,
						[]Token{
							{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
							{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
						},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Indexed: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Indexed: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&InvalidMemberLike{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, "unterminated member/index expression"},
							nil,
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
					},
				},
			}, n)
		})

		t.Run("identifier '[' <integer literal> '] ", func(t *testing.T) {
			n := mustparseChunk(t, "a[0]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Indexed: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "a",
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Indexed: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Name:     "b",
								},
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
						Indexed: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Name:     "b",
								},
								{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "c",
								},
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&IndexExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Indexed: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{2, 3}},
								},
							},

							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
						},
						Index: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})
	})

	t.Run("slice expression", func(t *testing.T) {
		t.Run("slice expression : variable '[' <integer literal> ':' ] ", func(t *testing.T) {
			n := mustparseChunk(t, "$a[0:]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&SliceExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Indexed: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						StartIndex: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("slice expression : variable '['  ':' <integer literal> ] ", func(t *testing.T) {
			n := mustparseChunk(t, "$a[:1]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&SliceExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Indexed: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						EndIndex: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("slice expression : variable '[' ':' ']' : invalid ", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "$a[:]")
			})
		})

		t.Run("slice expression : variable '[' ':' <integer literal> ':' ']' : invalid ", func(t *testing.T) {
			assert.Panics(t, func() {
				mustparseChunk(t, "$a[:1:]")
			})
		})

	})

	t.Run("key list expression", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, ".{}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							nil,
							[]Token{{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{0, 2}}, {Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}}},
						},
						Keys: nil,
					},
				},
			}, n)
		})

		t.Run("one key", func(t *testing.T) {
			n := mustparseChunk(t, ".{name}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{0, 2}}, {Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}}},
						},
						Keys: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{0, 2}}, {Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}}},
						},
						Keys: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{2, 3},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInKeyList(':')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{2, 3}, Raw: ":"}},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&KeyListExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{0, 2}}, {Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}}},
						},
						Keys: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, nil},
								Name:     "name",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
						Value:    "https://an-example.com/",
					},
				},
			}, n)
		})

		t.Run("subdomain", func(t *testing.T) {
			n := mustparseChunk(t, `https://sub.example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
						Value:    "https://sub.example.com/",
					},
				},
			}, n)
		})

		t.Run("subdomain contains -", func(t *testing.T) {
			n := mustparseChunk(t, `https://sub-x.example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
						Value:    "https://sub-x.example.com/",
					},
				},
			}, n)
		})

		t.Run("root path", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
						Value:    "https://example.com/",
					},
				},
			}, n)
		})

		t.Run("path ends with ..", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/..`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
						Value:    "https://example.com/..",
					},
				},
			}, n)
		})

		t.Run("path ends with ...", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
						Value:    "https://example.com/...",
					},
				},
			}, n)
		})

		t.Run("empty query", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
						Value:    "https://example.com/?",
					},
				},
			}, n)
		})

		t.Run("not empty query", func(t *testing.T) {
			n := mustparseChunk(t, `https://example.com/?a=1`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 106}, nil, nil},
				Statements: []Node{
					&URLLiteral{
						NodeBase: NodeBase{NodeSpan{0, 106}, nil, nil},
						Value:    "https://example.com/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
			}, n)
		})

	})

	t.Run("url pattern literal", func(t *testing.T) {
		t.Run("prefix pattern, root", func(t *testing.T) {
			n := mustparseChunk(t, `%https://example.com/...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
						Value:    "https://example.com/...",
						Raw:      "%https://example.com/...",
					},
				},
			}, n)
		})

		t.Run("prefix pattern", func(t *testing.T) {
			n := mustparseChunk(t, `%https://example.com/a/...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
						Value:    "https://example.com/a/...",
						Raw:      "%https://example.com/a/...",
					},
				},
			}, n)
		})

		t.Run("prefix pattern containing two dots", func(t *testing.T) {
			n := mustparseChunk(t, `%https://example.com/../...`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							&ParsingError{UnspecifiedParsingError, URL_PATTERN_SUBSEQUENT_DOT_EXPLANATION},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							&ParsingError{UnspecifiedParsingError, URL_PATTERN_SUBSEQUENT_DOT_EXPLANATION},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&URLPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							&ParsingError{UnspecifiedParsingError, URL_PATTERNS_CANNOT_END_WITH_SLASH_MORE_THAN_4_DOTS},
							nil,
						},
						Value: "https://example.com/....",
						Raw:   "%https://example.com/....",
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
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
					Statements: []Node{
						&HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
					},
				},
			},
			`wss://example.com`: {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
					Statements: []Node{
						&HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
							Value:    "://example.com",
						},
					},
				},
			},
			`https://*.com`: {
				err: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&SchemeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Name:     "http",
					},
				},
			}, n)
		})

		t.Run("Websocket", func(t *testing.T) {
			n := mustparseChunk(t, "wss://")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&SchemeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Name:     "wss",
					},
				},
			}, n)
		})

		t.Run("host with no scheme", func(t *testing.T) {
			n, err := parseChunk(t, `://`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&SchemeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, INVALID_SCHEME_LIT_MISSING_SCHEME},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							&ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT_SUGGEST_DOUBLE_STAR},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_HOST_PATT},
							nil,
						},
						Value: "https://*.*",
						Raw:   "%https://*.*",
					},
				},
			}, n)
		})

	})

	t.Run("host pattern", func(t *testing.T) {

		t.Run("HTTP host pattern : %https://**:443", func(t *testing.T) {
			n := mustparseChunk(t, `%https://**:443`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
						Value:    "https://**:443",
						Raw:      "%https://**:443",
					},
				},
			}, n)
		})

		t.Run("HTTP host pattern : %https://*.<tld>", func(t *testing.T) {
			n := mustparseChunk(t, `%https://*.com`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
						Value:    "https://*.com",
						Raw:      "%https://*.com",
					},
				},
			}, n)
		})

		t.Run("HTTP host pattern : %https://a*.<tld>", func(t *testing.T) {
			n := mustparseChunk(t, `%https://a*.com`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&HostPatternLiteral{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
						Value:    "wss://**",
						Raw:      "%wss://**",
					},
				},
			}, n)
		})
	})

	t.Run("email address literal", func(t *testing.T) {
		t.Run("only letters in username", func(t *testing.T) {
			n := mustparseChunk(t, `foo@mail.com`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&EmailAddressLiteral{
						NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
						Value:    "foo@mail.com",
					},
				},
			}, n)
		})

		t.Run("letters, dots & numbers", func(t *testing.T) {
			n := mustparseChunk(t, `foo.e.9@mail.com`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&EmailAddressLiteral{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
						Value:    "foo.e.9@mail.com",
					},
				},
			}, n)
		})

		t.Run("letters, dots & numbers", func(t *testing.T) {
			n := mustparseChunk(t, `foo+e%9@mail.com`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&EmailAddressLiteral{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
						Value:    "foo+e%9@mail.com",
					},
				},
			}, n)
		})
	})

	t.Run("url expressions", func(t *testing.T) {
		t.Run("no query, host interpolation", func(t *testing.T) {
			n := mustparseChunk(t, `https://{$host}/`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{8, 9}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{14, 15}},
							},
						},
						Raw: "https://{$host}/",
						HostPart: &HostExpression{
							NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
							Scheme: &SchemeLiteral{
								NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
								Name:     "https",
							},
							Raw: `https://{$host}`,
							Host: &Variable{
								NodeBase: NodeBase{NodeSpan{9, 14}, nil, nil},
								Name:     "host",
							},
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{19, 20}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},
						},
						Raw: "https://example.com{$path}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 19}, nil, nil},
								Value:    "",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{20, 25}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{8, 9}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{14, 15}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{15, 16}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{21, 22}},
							},
						},
						Raw: "https://{$host}{$path}",
						HostPart: &HostExpression{
							NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},

							Scheme: &SchemeLiteral{
								NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
								Name:     "https",
							},
							Raw: `https://{$host}`,
							Host: &Variable{
								NodeBase: NodeBase{NodeSpan{9, 14}, nil, nil},
								Name:     "host",
							},
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{15, 15}, nil, nil},
								Value:    "",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{16, 21}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 27},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},
						},
						Raw: "https://example.com/{$path}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{24, 25}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{27, 28}},
							},
						},
						Raw: "https://example.com/{$a}{$b}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{21, 23}, nil, nil},
								Name:     "a",
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{24, 24}, nil, nil},
								Value:    "",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{25, 27}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							[]Token{{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}}},
						},
						Raw: "https://example.com/{",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{21, 21},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
									nil,
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
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, []Token{{Type: NEWLINE, Span: NodeSpan{21, 22}}}},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							[]Token{{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}}},
						},
						Raw: "https://example.com/{",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{21, 21},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP},
									nil,
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
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							[]Token{{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}}},
						},
						Raw: "https://example.com/{1",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{21, 22}, nil, nil},
								Value:    1,
								Raw:      "1",
							},
							&PathSlice{
								NodeBase: NodeBase{
									NodeSpan{22, 22},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATH_INTERP_MISSING_CLOSING_BRACE},
									nil,
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
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{21, 22}},
							},
						},
						Raw: "https://example.com/{}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 21},
									&ParsingError{UnspecifiedParsingError, EMPTY_PATH_INTERP},
									[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{21, 21}}},
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
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},
						},
						Raw: "https://example.com/{.}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, INVALID_PATH_INTERP},
									[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{21, 22}, Raw: "."}},
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
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},
						},
						Raw: "https://example.com/{.}/",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, INVALID_PATH_INTERP},
									[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{21, 22}, Raw: "."}},
								},
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{23, 24}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},
						},
						Raw: "https://example.com/{@}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, PATH_INTERP_EXPLANATION},
									[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{21, 22}, Raw: "@"}},
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
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{20, 21}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{22, 23}},
							},
						},
						Raw: "https://example.com/{@}/",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{21, 22},
									&ParsingError{UnspecifiedParsingError, PATH_INTERP_EXPLANATION},
									[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{21, 22}, Raw: "@"}},
								},
							},
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{23, 24}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 27},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},
						},
						Raw: "https://example.com/?v={$x}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, nil},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, nil},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{22, 23}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},
						},
						Raw: "https://example.com?v={$x}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{20, 26}, nil, nil},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{22, 22}, nil, nil},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{23, 25}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},
						},
						Raw: "https://example.com/?v={$x}&",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, nil},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, nil},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 29}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 29},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},
						},
						Raw: "https://example.com/?v={$x}&&",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, nil},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, nil},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
							},
						},
						Raw: "https://example.com/?v={$x}&=3",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, nil},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, nil},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, nil},
										Name:     "x",
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{28, 30}, nil, nil},
								Name:     "",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 34}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 34},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{26, 27}},
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{30, 31}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{33, 34}},
							},
						},
						Raw: "https://example.com/?v={$x}&w={$y}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 27}, nil, nil},
								Name:     "v",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{23, 23}, nil, nil},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{24, 26}, nil, nil},
										Name:     "x",
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{28, 34}, nil, nil},
								Name:     "w",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{30, 30}, nil, nil},
										Value:    "",
									},
									&Variable{
										NodeBase: NodeBase{NodeSpan{31, 33}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							[]Token{{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}}},
						},
						Raw: "https://example.com/?v={",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
											nil,
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}}},
						},
						Raw: "https://example.com/?v={1",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
										NodeBase: NodeBase{NodeSpan{24, 25}, nil, nil},
										Value:    1,
										Raw:      "1",
									},
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{
											NodeSpan{25, 25},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUERY_PARAM_INTERP_MISSING_CLOSING_BRACE},
											nil,
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{24, 25}},
							},
						},
						Raw: "https://example.com/?v={}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
											[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{24, 24}}},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},
						},
						Raw: "https://example.com/?v={:}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, nil},
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
											[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{24, 25}, Raw: ":"}},
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
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},
						},
						Raw: "https://example.com/?v={:}&w=3",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, nil},
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
											[]Token{{Type: INVALID_INTERP_SLICE, Span: NodeSpan{24, 25}, Raw: ":"}},
										},
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{27, 30}, nil, nil},
								Name:     "w",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},
						},
						Raw: "https://example.com/?v={?}",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, nil},
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
											nil,
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
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, nil},
				Statements: []Node{
					&URLExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							[]Token{
								{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{23, 24}},
								{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{25, 26}},
							},
						},
						Raw: "https://example.com/?v={?}&w=3",
						HostPart: &HostLiteral{
							NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
							Value:    "https://example.com",
						},
						Path: []Node{
							&PathSlice{
								NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
								Value:    "/",
							},
						},
						QueryParams: []Node{
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{21, 26}, nil, nil},
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
											nil,
										},
										Value: "?",
									},
								},
							},
							&URLQueryParameter{
								NodeBase: NodeBase{NodeSpan{27, 30}, nil, nil},
								Name:     "w",
								Value: []Node{
									&URLQueryParameterValueSlice{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&InvalidAliasRelatedNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, "unterminated AtHostLiteral | URLExpression | HostAliasDefinition"},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&ListLiteral{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 4},
							Tokens: []Token{
								{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
							},
						},
						Elements: []Node{
							&InvalidAliasRelatedNode{
								NodeBase: NodeBase{
									NodeSpan{1, 3},
									&ParsingError{UnspecifiedParsingError, "unterminated AtHostLiteral | URLExpression | HostAliasDefinition"},
									nil,
								},
								Raw: "@a",
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("host alias definition", func(t *testing.T) {
		t.Run("missing value after equal sign", func(t *testing.T) {
			n, err := parseChunk(t, `@a =`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&HostAliasDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, INVALID_HOST_ALIAS_DEF_MISSING_VALUE_AFTER_EQL_SIGN},
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &AtHostLiteral{
							NodeBase: NodeBase{Span: NodeSpan{0, 2}},
							Value:    "@a",
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
						Raw:      "12",
						Value:    12,
					},
				},
			}, n)
		})

		t.Run("hexadecimal", func(t *testing.T) {
			n := mustparseChunk(t, "0x33")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Raw:      "0x33",
						Value:    0x33,
					},
				},
			}, n)
		})

		t.Run("octal", func(t *testing.T) {
			n := mustparseChunk(t, "0o33")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Raw:      "0o33",
						Value:    0o33,
					},
				},
			}, n)
		})
	})

	t.Run("float literal", func(t *testing.T) {
		t.Run("float literal", func(t *testing.T) {
			n := mustparseChunk(t, "12.0")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Raw:      "12.0",
						Value:    12.0,
					},
				},
			}, n)
		})

		t.Run("underscore in whole part", func(t *testing.T) {
			n := mustparseChunk(t, "1_000.0")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Raw:      "1_000.0",
						Value:    1_000.0,
					},
				},
			}, n)
		})

		t.Run("underscore in fractionam part", func(t *testing.T) {
			n := mustparseChunk(t, "1.000_000")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
						Raw:      "1.000_000",
						Value:    1.0,
					},
				},
			}, n)
		})

		t.Run("positive exponent", func(t *testing.T) {
			n := mustparseChunk(t, "12.0e2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Raw:      "12.0e2",
						Value:    1200.0,
					},
				},
			}, n)
		})

		t.Run("negative exponent", func(t *testing.T) {
			n := mustparseChunk(t, "12.0e-2")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&FloatLiteral{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, QUANTITY_LIT_NOT_ALLOWED_WITH_HEXADECIMAL_NUM},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, QUANTITY_LIT_NOT_ALLOWED_WITH_OCTAL_NUM},
							nil,
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&QuantityLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Raw:      "1s10ms",
						Units:    []string{"s", "ms"},
						Values:   []float64{1.0, 10},
					},
				},
			}, n)
		})
	})

	t.Run("date literal", func(t *testing.T) {
		t.Run("year only", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
						Raw:      "2020y-UTC",
						Value:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			}, n)
		})

		t.Run("year and month", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-5mt-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
						Raw:      "2020y-5mt-UTC",
						Value:    time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			}, n)
		})

		t.Run("missing location part", func(t *testing.T) {
			n, err := parseChunk(t, "2020y-5mt", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, INVALID_DATE_LITERAL_MISSING_LOCATION_PART_AT_THE_END},
							nil,
						},
						Raw: "2020y-5mt",
					},
				},
			}, n)
		})

		t.Run("year and microseconds", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-5us-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
						Raw:      "2020y-5us-UTC",
						Value:    time.Date(2020, 1, 1, 0, 0, 0, 5_000, time.UTC),
					},
				},
			}, n)
		})

		t.Run("up to minutes", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-10mt-5d-5h-4m-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
						Raw:      "2020y-10mt-5d-5h-4m-UTC",
						Value:    time.Date(2020, 10, 5, 5, 4, 0, 0, time.UTC),
					},
				},
			}, n)
		})

		t.Run("up to microseconds", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-10mt-5d-5h-4m-5s-400ms-100us-UTC")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 38}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 38}, nil, nil},
						Raw:      "2020y-10mt-5d-5h-4m-5s-400ms-100us-UTC",
						Value:    time.Date(2020, 10, 5, 5, 4, 5, 400_000_000+100_000, time.UTC),
					},
				},
			}, n)
		})

		t.Run("up to microseconds (longer)", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-6mt-12d-18h-4m-4s-349ms-665us-Local")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 41}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 41}, nil, nil},
						Raw:      "2020y-6mt-12d-18h-4m-4s-349ms-665us-Local",
						Value:    time.Date(2020, 6, 12, 18, 4, 4, (349*1_000_000)+(665*1000), time.Local),
					},
				},
			}, n)
		})

		t.Run("up to microseconds (long location)", func(t *testing.T) {
			n := mustparseChunk(t, "2020y-6mt-12d-18h-4m-4s-349ms-665us-America/Los_Angeles")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 55}, nil, nil},
				Statements: []Node{
					&DateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 55}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&RateLiteral{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
					Statements: []Node{
						&RateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								&ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT_DIV_SYMBOL_SHOULD_BE_FOLLOWED_BY_UNIT},
								nil,
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
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
					Statements: []Node{
						&RateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								&ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT},
								nil,
							},
							Units:  []string{"kB"},
							Values: []float64{1.0},
							Raw:    "1kB/",
						},
						&IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{4, 5},
								&ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY},
								nil,
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
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Statements: []Node{
						&RateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 5},
								&ParsingError{UnspecifiedParsingError, INVALID_RATE_LIT},
								nil,
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
								nil,
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
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Raw:      `""`,
					Value:    ``,
				},
			},

			`" "`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Raw:      `" "`,
					Value:    ` `,
				},
			},

			`"é"`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Raw:      `"é"`,
					Value:    `é`,
				},
			},

			`"\\"`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
					Raw:      `"\\"`,
					Value:    `\`,
				},
			},

			`"\\\\"`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Raw:      `"\\\\"`,
					Value:    `\\`,
				},
			},

			`"ab`: {
				result: &QuotedStringLiteral{
					NodeBase: NodeBase{
						NodeSpan{0, 3},
						&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STRING_LIT},
						nil,
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
						[]Token{{Type: NEWLINE, Span: NodeSpan{3, 4}}},
					},
					Statements: []Node{
						&QuotedStringLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_QUOTED_STRING_LIT},
								nil,
							},
							Raw:   `"ab`,
							Value: ``,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
					},
				},

				error: true,
			},

			`+`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
					Raw:      `+`,
					Value:    `+`,
				},
			},

			`-`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
					Raw:      `-`,
					Value:    `-`,
				},
			},

			`--`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Raw:      `--`,
					Value:    `--`,
				},
			},

			`[--]`: {
				result: &ListLiteral{
					NodeBase: NodeBase{
						NodeSpan{0, 4},
						nil,
						[]Token{
							{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
							{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
						},
					},
					Elements: []Node{
						&UnquotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Raw:      `--`,
							Value:    `--`,
						},
					},
				},
			},

			`+\:`: {
				result: &UnquotedStringLiteral{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Raw:      `+\:`,
					Value:    `+:`,
				},
			},

			`- 2`: {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&UnquotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Raw:      `-`,
							Value:    `-`,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			},

			`-- 2`: {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
					Statements: []Node{
						&UnquotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Raw:      `--`,
							Value:    `--`,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
							Raw:      "2",
							Value:    2,
						},
					},
				},
			},

			"``": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Raw:      "``",
							Value:    "",
						},
					},
				},
			},

			"`1`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Raw:      "`1`",
							Value:    "1",
						},
					},
				},
			},
			"`\n`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Raw:      "`\n`",
							Value:    "\n",
						},
					},
				},
			},
			"`\n\r\n`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Raw:      "`\n\r\n`",
							Value:    "\n\r\n",
						},
					},
				},
			},

			"`\\n\\r\\t`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
							Raw:      "`\\n\\r\\t`",
							Value:    "\n\r\t",
						},
					},
				},
			},
			"`\"`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Raw:      "`\"`",
							Value:    "\"",
						},
					},
				},
			},
			"`\"a\"`": {
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
					Statements: []Node{
						&MultilineStringLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Raw:      "`\"a\"`",
							Value:    "\"a\"",
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
						NodeBase:   NodeBase{NodeSpan{0, testCase.result.Base().Span.End}, nil, nil},
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
					assert.Contains(t, literal.Err.message, testCase.err)
				}
			})
		}
	})

	t.Run("rune literal", func(t *testing.T) {

		t.Run("rune literal : simple character", func(t *testing.T) {
			n := mustparseChunk(t, `'a'`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&RuneLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
						Value:    'a',
					},
				},
			}, n)
		})

		t.Run("rune literal : valid escaped character", func(t *testing.T) {
			n := mustparseChunk(t, `'\n'`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&RuneLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},

				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Name:     "e",
					},
				},
			}, n)
		})

		t.Run("letter followed by a digit", func(t *testing.T) {
			n := mustparseChunk(t, `e2`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
						Name:     "e2",
					},
				},
			}, n)
		})

		t.Run("empty unambiguous identifier", func(t *testing.T) {
			n, err := parseChunk(t, `#`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},

				Statements: []Node{
					&UnambiguousIdentifierLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IDENTIFIER_LIT},
							nil,
						},
						Name: "",
					},
				},
			}, n)
		})

		t.Run("single letter unambiguous identifier", func(t *testing.T) {
			n := mustparseChunk(t, `#e`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},

				Statements: []Node{
					&UnambiguousIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
						Name:     "e",
					},
				},
			}, n)
		})

		t.Run("unambiguous identifier literal : letter followed by a digit", func(t *testing.T) {
			n := mustparseChunk(t, `#e2`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&UnambiguousIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{{Type: PLUS_EQUAL, Span: NodeSpan{4, 5}}},
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{2, 3}}},
						},
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{6, 7}}},
						},
						Left: &IndexExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Indexed: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							Index: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Raw:      "0",
								Value:    0,
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{8, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: EQUAL, Span: NodeSpan{3, 4}},
								{Type: PIPE, Span: NodeSpan{5, 6}},
							},
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						Right: &PipelineExpression{
							NodeBase: NodeBase{
								NodeSpan{7, 12},
								nil,
								[]Token{{Type: PIPE, Span: NodeSpan{9, 10}}},
							},
							Stages: []*PipelineStage{
								{
									Kind: NormalStage,
									Expr: &CallExpression{
										NodeBase: NodeBase{NodeSpan{7, 9}, nil, nil},
										Callee: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
											Name:     "a",
										},
										Must:              true,
										CommandLikeSyntax: true,
									},
								},
								{
									Kind: NormalStage,
									Expr: &CallExpression{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
										Callee: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{4, 5}}},
						},
						Left: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Name:     "b",
								},
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
							Name:     "b",
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&Assignment{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{InvalidNext, UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR},
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
							Name:     "b",
						},
						Operator: Assign,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
						Raw:      "2",
						Value:    2,
					},
				},
			}, n)
		})

	})

	t.Run("multi assignement statement", func(t *testing.T) {
		t.Run("assign <ident> = <var>", func(t *testing.T) {
			n := mustparseChunk(t, "assign a = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{9, 10}},
							},
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Name:     "a",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{11, 13}, nil, nil},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("assign var var = var", func(t *testing.T) {
			n := mustparseChunk(t, "assign a b = $c")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{11, 12}},
							},
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Name:     "a",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
								Name:     "b",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{13, 15}, nil, nil},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("nillable", func(t *testing.T) {
			n := mustparseChunk(t, "assign? a = $b")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: QUESTION_MARK, Span: NodeSpan{6, 7}},
								{Type: EQUAL, Span: NodeSpan{10, 11}},
							},
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
								Name:     "a",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							&ParsingError{InvalidNext, UNTERMINATED_ASSIGNMENT_MISSING_TERMINATOR},
							[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{9, 10}},
							},
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Name:     "a",
							},
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{11, 13}, nil, nil},
							Name:     "b",
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MULTI_ASSIGN_MISSING_EQL_SIGN},
							[]Token{{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}}},
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&MultiAssignment{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: ASSIGN_KEYWORD, Span: NodeSpan{0, 6}},
								{Type: EQUAL, Span: NodeSpan{9, 10}},
							},
						},
						Variables: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Name:     "a",
							},
						},
						Right: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{9, 10},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("assign a ="), 10, true)},
								nil,
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{
								{Type: EXCLAMATION_MARK, Span: NodeSpan{5, 6}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
								Name:     "a",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{9, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&SpreadArgument{
								NodeBase: NodeBase{
									NodeSpan{6, 11},
									nil,
									[]Token{{Type: THREE_DOTS, Span: NodeSpan{6, 9}}},
								},
								Expr: &Variable{
									NodeBase: NodeBase{NodeSpan{9, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{6, 7},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCallArguments('?')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{6, 7}, Raw: "?"}},
								},
							},
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Callee: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
								Name:     "http",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Callee: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "b",
							},
						},
						Arguments: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&CallExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{7, 8}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Callee: &CallExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
								},
							},
							Callee: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 6}, nil, nil},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 8}, nil, nil},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
					[]Token{{Type: NEWLINE, Span: NodeSpan{8, 9}}},
				},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 8}, nil, nil},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 11}, nil, nil},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
								Name:     "a",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{9, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 8}, nil, nil},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&ListLiteral{
								NodeBase: NodeBase{
									NodeSpan{6, 8},
									nil,
									[]Token{
										{Type: OPENING_BRACKET, Span: NodeSpan{6, 7}},
										{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
									},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 7}, nil, nil},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&AbsolutePathLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							[]Token{{Type: COMMENT, Span: NodeSpan{12, 21}, Raw: "# comment"}},
						},
						Callee: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Name:     "print",
						},
						Arguments: []Node{
							&Variable{
								NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
								Name:     "a",
							},
							&Variable{
								NodeBase: NodeBase{NodeSpan{9, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&CallExpression{
						Must:              true,
						CommandLikeSyntax: true,
						NodeBase:          NodeBase{NodeSpan{0, 7}, nil, nil},
						Callee: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Name:     "b",
								},
							},
						},
						Arguments: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							[]Token{{Type: PIPE, Span: NodeSpan{9, 10}}},
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
									NodeBase:          NodeBase{NodeSpan{11, 23}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, nil},
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
					[]Token{{Type: SEMICOLON, Span: NodeSpan{23, 24}}},
				},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							[]Token{{Type: PIPE, Span: NodeSpan{9, 10}}},
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
									NodeBase:          NodeBase{NodeSpan{11, 23}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, nil},
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
					[]Token{{Type: NEWLINE, Span: NodeSpan{23, 24}}},
				},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							[]Token{{Type: PIPE, Span: NodeSpan{9, 10}}},
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
									NodeBase:          NodeBase{NodeSpan{11, 23}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, nil},
										Name:     "do-something",
									},
									Arguments: nil,
								},
							},
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{24, 25}, nil, nil},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("first and second stages are calls with no arguments", func(t *testing.T) {
			n := mustparseChunk(t, "print | do-something")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							[]Token{{Type: PIPE, Span: NodeSpan{6, 7}}},
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 6}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
										Name:     "print",
									},
								},
							},
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{8, 20}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 20}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{{Type: PIPE, Span: NodeSpan{9, 10}}},
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
									NodeBase:          NodeBase{NodeSpan{11, 25}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, nil},
										Name:     "do-something",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{24, 25}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 45}, nil, nil},
				Statements: []Node{
					&PipelineStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 45},
							nil,
							[]Token{{Type: PIPE, Span: NodeSpan{9, 10}}, {Type: PIPE, Span: NodeSpan{26, 27}}},
						},
						Stages: []*PipelineStage{
							{
								Kind: NormalStage,
								Expr: &CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{0, 9}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
										Name:     "print",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
									NodeBase:          NodeBase{NodeSpan{11, 25}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 23}, nil, nil},
										Name:     "do-something",
									},
									Arguments: []Node{
										&Variable{
											NodeBase: NodeBase{NodeSpan{24, 25}, nil, nil},
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
									NodeBase:          NodeBase{NodeSpan{28, 45}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{28, 45}, nil, nil},
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
			NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
			Statements: []Node{
				&CallExpression{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Must:     true,
					Callee: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Name:     "mime",
					},
					Arguments: []Node{
						&QuotedStringLiteral{
							NodeBase: NodeBase{NodeSpan{4, 10}, nil, nil},
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
			NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
			Statements: []Node{
				&CallExpression{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Must:     true,
					Callee: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Name:     "f",
					},
					Arguments: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{1, 3},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{1, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{1, 2}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{1, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{1, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{1, 2}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 6},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{3, 4}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 5},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{3, 4}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 6},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{3, 4}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{8, 12},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{9, 10}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{Span: NodeSpan{1, 2}},
									Value: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										&ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON},
										[]Token{
											{Type: COLON, Span: NodeSpan{4, 5}},
											{Type: NEWLINE, Span: NodeSpan{5, 6}},
										},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				input:    "{ a %int: 1 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 11},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{8, 9}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Type: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
										Name:     "int",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMENT, Span: NodeSpan{2, 12}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{12, 13}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMENT, Span: NodeSpan{8, 18}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{18, 19}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										[]Token{
											{Type: COLON, Span: NodeSpan{4, 5}},
										},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMENT, Span: NodeSpan{2, 12}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{12, 13}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{14, 19},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{16, 17}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 19},
										&ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyCommentBeforeValueOfKey("a")},
										[]Token{
											{Type: COLON, Span: NodeSpan{4, 5}},
											{Type: COMMENT, Span: NodeSpan{6, 16}, Raw: "# comment "},
											{Type: NEWLINE, Span: NodeSpan{16, 17}},
										},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 5},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 4}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{1, 2}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_REC_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{3, 8}, nil, nil},
									Key:      nil,
									Value: &QuotedStringLiteral{
										NodeBase: NodeBase{
											NodeSpan{3, 6},
											nil,
											[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
											},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 8},
										&ParsingError{UnspecifiedParsingError, ONLY_EXPLICIT_KEY_CAN_HAVE_A_TYPE_ANNOT},
										nil,
									},
									Key: nil,
									Type: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
										Name:     "int",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 4},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										nil,
									},
									Key: nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{4, 6}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 16},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{9, 14},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{11, 12}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{13, 14}, nil, nil},
										Raw:      "2",
										Value:    2,
									},
								},
							},
						},
					},
				},
			}, {
				input:    "{ a : 1 , b : 2 }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{10, 15},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{12, 13}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{4, 9},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{6, 7}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 8}, nil, nil},
									Value: &PropertyNameLiteral{
										NodeBase: NodeBase{NodeSpan{2, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{10, 15},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{12, 13}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 15},
										nil,
										[]Token{{Type: THREE_DOTS, Span: NodeSpan{2, 5}}},
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{6, 15}, nil, nil},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 15},
												nil,
												[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{8, 10}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
												},
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{10, 14}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},
							},
							MetaProperties: []*ObjectMetaProperty{
								{
									NodeBase: NodeBase{NodeSpan{2, 19}, nil, nil},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{2, 15}, nil, nil},
										Name:     "_constraints_",
									},
									Initialization: &InitializationBlock{
										NodeBase: NodeBase{
											NodeSpan{16, 19},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
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
				input:    "{ ... $e }",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{2, 8},
										&ParsingError{UnspecifiedParsingError, fmtInvalidSpreadElemExprShouldBeExtrExprNot((*Variable)(nil))},
										[]Token{{Type: THREE_DOTS, Span: NodeSpan{2, 5}}},
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
					Statements: []Node{
						&ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 19},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{16, 18}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{16, 17}, nil, nil},
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
										[]Token{{Type: THREE_DOTS, Span: NodeSpan{2, 5}}},
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{6, 15}, nil, nil},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 15},
												nil,
												[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{8, 10}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
												},
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{10, 14}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},
							},
							Properties: nil,
						},
					},
				},
			}, {
				input:    "#{ }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{2, 3}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 6},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{5, 6}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{9, 13},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{10, 11}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										&ParsingError{UnspecifiedParsingError, UNEXPECTED_NEWLINE_AFTER_COLON},
										[]Token{
											{Type: COLON, Span: NodeSpan{5, 6}},
											{Type: NEWLINE, Span: NodeSpan{6, 7}},
										},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
				input:    "#{ # comment \n}",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMENT, Span: NodeSpan{3, 13}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{13, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMENT, Span: NodeSpan{9, 19}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{5, 6}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMENT, Span: NodeSpan{3, 13}, Raw: "# comment "},
									{Type: NEWLINE, Span: NodeSpan{13, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{15, 20},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{17, 18}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 20},
										&ParsingError{UnspecifiedParsingError, fmtInvalidObjRecordKeyCommentBeforeValueOfKey("a")},
										[]Token{
											{Type: COLON, Span: NodeSpan{5, 6}},
											{Type: COMMENT, Span: NodeSpan{7, 17}, Raw: "# comment "},
											{Type: NEWLINE, Span: NodeSpan{17, 18}},
										},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{3, 5}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{4, 9}, nil, nil},
									Key:      nil,
									Value: &QuotedStringLiteral{
										NodeBase: NodeBase{
											NodeSpan{4, 7},
											nil,
											[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
											},
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
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 5},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										nil,
									},
									Key: nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 17},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										&ParsingError{UnspecifiedParsingError, INVALID_OBJ_REC_LIT_ENTRY_SEPARATION},
										[]Token{{Type: COLON, Span: NodeSpan{5, 6}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{10, 15},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{12, 13}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{5, 6}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{11, 16},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{13, 14}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{5, 6}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{3, 4}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{5, 10},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{5, 6}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{11, 16},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{13, 14}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
										Name:     "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{3, 9}, nil, nil},
									Value: &PropertyNameLiteral{
										NodeBase: NodeBase{NodeSpan{3, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 16},
										nil,
										[]Token{{Type: THREE_DOTS, Span: NodeSpan{3, 6}}},
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{7, 16}, nil, nil},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{7, 9}, nil, nil},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{9, 16},
												nil,
												[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{9, 11}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
												},
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{11, 15}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 11},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Properties: nil,
							SpreadElements: []*PropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 9},
										&ParsingError{UnspecifiedParsingError, fmtInvalidSpreadElemExprShouldBeExtrExprNot((*Variable)(nil))},
										[]Token{{Type: THREE_DOTS, Span: NodeSpan{3, 6}}},
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{7, 9}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
					Statements: []Node{
						&RecordLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 20},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},
							},
							Properties: []*ObjectProperty{
								{
									NodeBase: NodeBase{NodeSpan{17, 19}, nil, nil},
									Key:      nil,
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{17, 18}, nil, nil},
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
										[]Token{{Type: THREE_DOTS, Span: NodeSpan{3, 6}}},
									},
									Expr: &ExtractionExpression{
										NodeBase: NodeBase{NodeSpan{7, 16}, nil, nil},
										Object: &Variable{
											NodeBase: NodeBase{NodeSpan{7, 9}, nil, nil},
											Name:     "e",
										},
										Keys: &KeyListExpression{
											NodeBase: NodeBase{
												NodeSpan{9, 16},
												nil,
												[]Token{
													{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{9, 11}},
													{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
												},
											},
											Keys: []Node{
												&IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{11, 15}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
								},
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "[ ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
								},
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "[ 1 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 5},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{4, 5}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{6, 7}},
								},
							}, Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{4, 5}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: NEWLINE, Span: NodeSpan{4, 5}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{3, 4}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								&ParsingError{UnspecifiedParsingError, "unterminated list literal, missing closing bracket ']'"},
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{3, 4}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{2, 7},
										nil,
										[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{2, 5}},
										},
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: COMMA, Span: NodeSpan{5, 6}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{2, 6},
										nil,
										[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{2, 5}},
										},
									},
									Expr: &MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{5, 6},
											&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("[ ..., ]"), 5, true)},
											nil,
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
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
									{Type: OPENING_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_OPENING_BRACKET_AFTER_TYPE},
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
					Statements: []Node{
						&ListLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIST_LIT_MISSING_CLOSING_BRACKET},
								[]Token{
									{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{1, 2}},
									{Type: OPENING_BRACKET, Span: NodeSpan{6, 7}},
								},
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
								},
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "#[ ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
								},
							},
							Elements: nil,
						},
					},
				},
			},
			{
				input: "#[ 1 ]",
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},
							}, Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{5, 6}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{5, 6}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{4, 5}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{6, 7}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 7},
								&ParsingError{UnspecifiedParsingError, "unterminated list literal, missing closing bracket ']'"},
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{4, 5}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{3, 8},
										nil,
										[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{3, 6}},
										},
									},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
					Statements: []Node{
						&TupleLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 9},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{6, 7}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Elements: []Node{
								&ElementSpreadElement{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										nil,
										[]Token{
											{Type: THREE_DOTS, Span: NodeSpan{3, 6}},
										},
									},
									Expr: &MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{6, 7},
											&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("[ ..., ]"), 5, true)},
											nil,
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
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 3},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
								},
							},
						},
					},
				},
			},
			{
				input:    ":{ }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 4},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
								},
							},
						},
					},
				},
			},
			{
				input:    `:{ "a" : 1 }`,
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 10},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 12},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
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
											nil,
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
					NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 10},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 9},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY},
										nil,
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 8},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 7},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY},
										nil,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_DICT_MISSING_CLOSING_BRACE},
								[]Token{{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}}},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 6},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_ENTRY_MISSING_COLON_AFTER_KEY},
										nil,
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 10},
										&ParsingError{UnspecifiedParsingError, INVALID_DICT_LIT_ENTRY_SEPARATION},
										[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{12, 19},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{16, 17}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{12, 15}, nil, nil},
										Raw:      `"b"`,
										Value:    "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{18, 19}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 22},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: COMMA, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 10},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{13, 20},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{17, 18}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{13, 16}, nil, nil},
										Raw:      `"b"`,
										Value:    "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 10},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{3, 4}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{5, 12},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{9, 10}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
					Statements: []Node{
						&DictionaryLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 22},
								nil,
								[]Token{
									{Type: OPENING_DICTIONARY_BRACKET, Span: NodeSpan{0, 2}},
									{Type: NEWLINE, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
								},
							},
							Entries: []*DictionaryEntry{
								{
									NodeBase: NodeBase{
										NodeSpan{3, 10},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
										Raw:      `"a"`,
										Value:    "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								{
									NodeBase: NodeBase{
										NodeSpan{13, 20},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{17, 18}}},
									},
									Key: &QuotedStringLiteral{
										NodeBase: NodeBase{NodeSpan{13, 16}, nil, nil},
										Raw:      `"b"`,
										Value:    "b",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
							},
							Statements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 15},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
								},
							},
							Statements: []Node{
								&CallExpression{
									Must:              true,
									CommandLikeSyntax: true,
									NodeBase:          NodeBase{NodeSpan{10, 14}, nil, nil},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
										Name:     "a",
									},
									Arguments: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{MissingBlock, UNTERMINATED_IF_STMT_MISSING_BLOCK},
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
					},
				},
			}, n)
		})

		t.Run("multiline", func(t *testing.T) {
			n := mustparseChunk(t, "if true { \n }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
							},
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: NEWLINE, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{12, 16}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Statements: nil,
						},
						Alternate: &Block{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 38}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 38},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{31, 35}},
							},
						},
						Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 30},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{29, 30}},
								},
							},
							Statements: []Node{
								&IfStatement{
									NodeBase: NodeBase{
										NodeSpan{10, 28},
										nil,
										[]Token{
											{Type: IF_KEYWORD, Span: NodeSpan{10, 12}},
											{Type: ELSE_KEYWORD, Span: NodeSpan{21, 25}},
										},
									},
									Test: &BooleanLiteral{
										NodeBase: NodeBase{NodeSpan{13, 17}, nil, nil},
										Value:    true,
									},
									Consequent: &Block{
										NodeBase: NodeBase{
											NodeSpan{18, 20},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
											},
										},
										Statements: nil,
									},
									Alternate: &Block{
										NodeBase: NodeBase{
											NodeSpan{26, 28},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{26, 27}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
											},
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
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{36, 37}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{37, 38}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
				Statements: []Node{
					&IfStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 27},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{12, 16}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Value:    true,
						},
						Consequent: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Statements: nil,
						},
						Alternate: &IfStatement{
							NodeBase: NodeBase{
								NodeSpan{17, 27},
								nil,
								[]Token{{Type: IF_KEYWORD, Span: NodeSpan{17, 19}}},
							}, Test: &BooleanLiteral{
								NodeBase: NodeBase{NodeSpan{20, 24}, nil, nil},
								Value:    true,
							},
							Consequent: &Block{
								NodeBase: NodeBase{
									NodeSpan{25, 27},
									nil,
									[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{26, 27}},
									},
								},
							},
						},
					},
				},
			}, n)
		})

	})

	t.Run("if expression", func(t *testing.T) {

		t.Run("(if <test> <consequent>)", func(t *testing.T) {
			n := mustparseChunk(t, "(if true 1)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{1, 3}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{1, 3}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
							Value:    true,
						},
						Consequent: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{7, 8},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune(code), 8, true)},
								nil,
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN},
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{1, 3}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{1, 3}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{11, 15}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						Alternate: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{16, 17}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN},
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{1, 3}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{11, 15}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						Alternate: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{16, 17}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&IfExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_IF_EXPR_MISSING_CLOSING_PAREN},
							[]Token{
								{Type: IF_KEYWORD, Span: NodeSpan{1, 3}},
								{Type: ELSE_KEYWORD, Span: NodeSpan{11, 15}},
							},
						}, Test: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
							Value:    true,
						},
						Consequent: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						Alternate: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{14, 15},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune(code), 15, true)},
								nil,
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
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{9, 11}},
							},
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 18}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{19, 22},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{11, 12}},
								{Type: IN_KEYWORD, Span: NodeSpan{15, 17}},
							},
						},
						KeyPattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 9}, nil, nil},
							Name:     "even",
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{13, 14}, nil, nil},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{18, 24}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{25, 28},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 31}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 31},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{11, 12}},
								{Type: IN_KEYWORD, Span: NodeSpan{18, 20}},
							},
						},
						KeyPattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 9}, nil, nil},
							Name:     "even",
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
							Name:     "i",
						},
						ValuePattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{13, 15}, nil, nil},
							Name:     "p",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{16, 17}, nil, nil},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{21, 27}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{28, 31},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{28, 29}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{30, 31}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{12, 14}},
							},
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Name:     "i",
						},
						ValuePattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 9}, nil, nil},
							Name:     "p",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{15, 21}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{22, 25},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: IN_KEYWORD, Span: NodeSpan{6, 8}},
							},
						},
						KeyIndexIdent: nil,
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{9, 15}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{16, 19},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 27},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: CHUNKED_KEYWORD, Span: NodeSpan{4, 11}},
								{Type: IN_KEYWORD, Span: NodeSpan{14, 16}},
							},
						},
						KeyIndexIdent: nil,
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
							Name:     "u",
						},
						Chunked: true,
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{17, 23}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{24, 27},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{26, 27}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{9, 11}},
							},
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 18}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{19, 28},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
								},
							},
							Statements: []Node{
								&BreakStatement{
									NodeBase: NodeBase{
										NodeSpan{21, 26},
										nil,
										[]Token{{Type: BREAK_KEYWORD, Span: NodeSpan{21, 26}}},
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
				NodeBase: NodeBase{NodeSpan{0, 31}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 31},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: IN_KEYWORD, Span: NodeSpan{9, 11}},
							},
						},
						KeyIndexIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
							Name:     "i",
						},
						ValueElemIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
							Name:     "u",
						},
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{12, 18}, nil, nil},
							Name:     "users",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{19, 31},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{30, 31}},
								},
							},
							Statements: []Node{
								&ContinueStatement{
									NodeBase: NodeBase{
										NodeSpan{21, 29},
										nil,
										[]Token{{Type: CONTINUE_KEYWORD, Span: NodeSpan{21, 29}}},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
							},
						},
						KeyIndexIdent:  nil,
						ValueElemIdent: nil,
						IteratedValue: &Variable{
							NodeBase: NodeBase{NodeSpan{4, 10}, nil, nil},
							Name:     "array",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{11, 14},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&ForStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: FOR_KEYWORD, Span: NodeSpan{0, 3}},
							},
						},
						KeyIndexIdent:  nil,
						ValueElemIdent: nil,
						IteratedValue: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, nil},
							Name:     "p",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 10},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Statements: nil,
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
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
				Statements: []Node{
					&WalkStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							[]Token{{Type: WALK_KEYWORD, Span: NodeSpan{0, 4}}},
						},
						Walked: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
							Raw:      "./",
							Value:    "./",
						},
						EntryIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 13}, nil, nil},
							Name:     "entry",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{14, 17},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&WalkStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							[]Token{
								{Type: WALK_KEYWORD, Span: NodeSpan{0, 4}},
								{Type: COMMA, Span: NodeSpan{12, 13}},
							},
						},
						Walked: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
							Raw:      "./",
							Value:    "./",
						},
						MetaIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{8, 12}, nil, nil},
							Name:     "meta",
						},
						EntryIdent: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{14, 19}, nil, nil},
							Name:     "entry",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{20, 23},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&UnaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							[]Token{
								{Type: EXCLAMATION_MARK, Span: NodeSpan{0, 1}},
							},
						},
						Operator: BoolNegate,
						Operand: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, nil},
							Value:    true,
						},
					},
				},
			}, n)
		})

		t.Run("unary expression : number negate", func(t *testing.T) {
			n := mustparseChunk(t, "(- 2)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&UnaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: MINUS, Span: NodeSpan{1, 2}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},
						},
						Operator: NumberNegate,
						Operand: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{15, 16}},
							},
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{3, 4}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 15},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{12, 13}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
								Name:     "c",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{3, 4}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Right: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
							Name:     "c",
						},
					},
				},
			}, n)
		})

		t.Run("OR(variable, bin ex)", func(t *testing.T) {
			n := mustparseChunk(t, "(a or b > c)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{3, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},
						},
						Operator: Or,
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
							Name:     "a",
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{6, 11},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{8, 9}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
								Name:     "b",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{24, 25}},
							},
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{3, 4}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 24},
								nil,
								[]Token{
									{Type: OR_KEYWORD, Span: NodeSpan{16, 18}},
								},
							},
							Operator: Or,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{{Type: GREATER_THAN, Span: NodeSpan{12, 13}}},
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Name:     "c",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
									Name:     "d",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{19, 24},
									nil,
									[]Token{{Type: GREATER_THAN, Span: NodeSpan{21, 22}}},
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
									Name:     "e",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{23, 24}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{3, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{20, 21}},
							},
						},
						Operator: Or,
						Left: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
							Name:     "a",
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{6, 20},
								nil,
								[]Token{{Type: OR_KEYWORD, Span: NodeSpan{12, 14}}},
							},
							Operator: Or,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{6, 12},
									nil,
									[]Token{{Type: GREATER_THAN, Span: NodeSpan{8, 9}}},
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Name:     "b",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Name:     "c",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{15, 20},
									nil,
									[]Token{{Type: GREATER_THAN, Span: NodeSpan{17, 18}}},
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
									Name:     "d",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
			// 	NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
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
			// 				NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
			// 				Name:     "a",
			// 			},
			// 			Right: &BinaryExpression{
			// 				NodeBase: NodeBase{
			// 					NodeSpan{6, 20},
			// 					nil,
			// 					[]Token{{Type: OR_KEYWORD, Span: NodeSpan{12, 14}}},
			// 				},
			// 				Operator: Or,
			// 				Left: &IdentifierLiteral{
			// 					NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
			// 					Name:     "b",
			// 				},
			// 				Right: &BinaryExpression{
			// 					NodeBase: NodeBase{
			// 						NodeSpan{15, 20},
			// 						nil,
			// 						[]Token{{Type: GREATER_THAN, Span: NodeSpan{17, 18}}},
			// 					},
			// 					Operator: GreaterThan,
			// 					Left: &IdentifierLiteral{
			// 						NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
			// 						Name:     "c",
			// 					},
			// 					Right: &IdentifierLiteral{
			// 						NodeBase: NodeBase{NodeSpan{19, 20}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							&ParsingError{UnspecifiedParsingError, ALL_BIN_EXPR_CHAIN_SHOULD_HAVE_THE_SAME_OPERATOR},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{25, 26}},
							},
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{3, 4}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 25},
								nil,
								[]Token{
									{Type: AND_KEYWORD, Span: NodeSpan{16, 19}},
								},
							},
							Operator: And,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{{Type: GREATER_THAN, Span: NodeSpan{12, 13}}},
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Name:     "c",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
									Name:     "d",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{20, 25},
									nil,
									[]Token{{Type: GREATER_THAN, Span: NodeSpan{22, 23}}},
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, nil},
									Name:     "e",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{24, 25}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 35}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 35},
							&ParsingError{UnspecifiedParsingError, ALL_BIN_EXPR_CHAIN_SHOULD_HAVE_THE_SAME_OPERATOR},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{34, 35}},
							},
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{3, 4}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 34},
								nil,
								[]Token{{Type: AND_KEYWORD, Span: NodeSpan{16, 19}}},
							},
							Operator: And,
							Left: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{{Type: GREATER_THAN, Span: NodeSpan{12, 13}}},
								},
								Operator: GreaterThan,
								Left: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Name:     "c",
								},
								Right: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
									Name:     "d",
								},
							},
							Right: &BinaryExpression{
								NodeBase: NodeBase{
									NodeSpan{20, 34},
									nil,
									[]Token{{Type: OR_KEYWORD, Span: NodeSpan{26, 28}}},
								},
								Operator: Or,
								Left: &BinaryExpression{
									NodeBase: NodeBase{
										NodeSpan{20, 26},
										nil,
										[]Token{{Type: GREATER_THAN, Span: NodeSpan{22, 23}}},
									},
									Operator: GreaterThan,
									Left: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{20, 21}, nil, nil},
										Name:     "e",
									},
									Right: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{24, 25}, nil, nil},
										Name:     "f",
									},
								},
								Right: &BinaryExpression{
									NodeBase: NodeBase{
										NodeSpan{29, 34},
										nil,
										[]Token{{Type: GREATER_THAN, Span: NodeSpan{31, 32}}},
									},
									Operator: GreaterThan,
									Left: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{29, 30}, nil, nil},
										Name:     "g",
									},
									Right: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{33, 34}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
							},
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{3, 4}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Right: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{8, 9},
								&ParsingError{UnspecifiedParsingError, "an expression was expected: ... b or<<here>>..."},
								nil,
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_PAREN},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: OR_KEYWORD, Span: NodeSpan{7, 9}},
							},
						},
						Operator: Or,
						Left: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 6},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{3, 4}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
								Name:     "a",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Right: &BinaryExpression{
							NodeBase: NodeBase{
								NodeSpan{10, 15},
								nil,
								[]Token{{Type: GREATER_THAN, Span: NodeSpan{12, 13}}},
							},
							Operator: GreaterThan,
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
								Name:     "c",
							},
							Right: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
								Name:     "d",
							},
						},
					},
				},
			}, n)
		})

		t.Run("binary expression", func(t *testing.T) {
			n := mustparseChunk(t, "($a + $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Operator: Add,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("range", func(t *testing.T) {
			n := mustparseChunk(t, "($a .. $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: TWO_DOTS, Span: NodeSpan{4, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Operator: Range,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{7, 9}, nil, nil},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("exclusive end range", func(t *testing.T) {
			n := mustparseChunk(t, "($a ..< $b)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: DOT_DOT_LESS_THAN, Span: NodeSpan{4, 7}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},
						},
						Operator: ExclEndRange,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{8, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERAND},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},
						},
						Operator: Add,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{5, 6},
								&ParsingError{UnspecifiedParsingError, "an expression was expected: ...($a +<<here>>)..."},
								nil,
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{4, 5}, Raw: "?"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{4, 5}, Raw: "!"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{6, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{3, 4}, Raw: "!"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, INVALID_BIN_EXPR_NON_EXISTING_OPERATOR},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: INVALID_OPERATOR, Span: NodeSpan{4, 8}, Raw: "ande"},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{9, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_BIN_EXPR_MISSING_OPERATOR},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},
						},
						Operator: -1,
						Left: &Variable{
							NodeBase: NodeBase{NodeSpan{1, 3}, nil, nil},
							Name:     "a",
						},
						Right: &Variable{
							NodeBase: NodeBase{NodeSpan{3, 5}, nil, nil},
							Name:     "b",
						},
					},
				},
			}, n)
		})

		t.Run("only opening parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, "(", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("("), 1, true)},
							[]Token{{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}}},
						},
					},
				},
			}, n)
		})

		t.Run("opening parenthesis followed by newline", func(t *testing.T) {
			n, err := parseChunk(t, "(\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("(\n"), 2, true)},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: NEWLINE, Span: NodeSpan{1, 2}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("opening parenthesis followed by an unexpected character", func(t *testing.T) {
			n, err := parseChunk(t, "(;", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParenthesizedExpression(';')},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: UNEXPECTED_CHAR, Raw: ";", Span: NodeSpan{1, 2}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing expression in between parenthesis", func(t *testing.T) {
			n, err := parseChunk(t, "()", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("()"), 1, true)},
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{1, 2}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&RuntimeTypeCheckExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							nil,
							[]Token{
								{Type: TILDE, Span: NodeSpan{0, 1}},
							},
						},
						Expr: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{1, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
				Statements: []Node{
					&RuntimeTypeCheckExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							nil,
							[]Token{
								{Type: TILDE, Span: NodeSpan{0, 1}},
							},
						},
						Expr: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("~"), 1, true)},
								nil,
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&UpperBoundRangeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{{Type: TWO_DOTS, Span: NodeSpan{0, 2}}},
						},
						UpperBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&UpperBoundRangeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, INVALID_UPPER_BOUND_RANGE_EXPR},
							[]Token{{Type: TWO_DOTS, Span: NodeSpan{0, 2}}},
						},
						UpperBound: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{2, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&IntegerRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, []Token{{Type: TWO_DOTS, Span: NodeSpan{1, 3}}}},
						LowerBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						UpperBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&IntegerRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 3}, nil, []Token{{Type: TWO_DOTS, Span: NodeSpan{1, 3}}}},
						LowerBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&IntegerRangeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_INT_LIT},
							[]Token{{Type: TWO_DOTS, Span: NodeSpan{1, 3}}},
						},
						LowerBound: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
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

	t.Run("quantity range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, "1x..2x")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&QuantityRangeLiteral{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, []Token{{Type: TWO_DOTS, Span: NodeSpan{2, 4}}}},
						LowerBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Raw:      "1x",
							Values:   []float64{1},
							Units:    []string{"x"},
						},
						UpperBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{4, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&QuantityRangeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{{Type: TWO_DOTS, Span: NodeSpan{2, 4}}},
						},
						LowerBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&QuantityRangeLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_BE_QTY_LIT},
							[]Token{{Type: TWO_DOTS, Span: NodeSpan{2, 4}}},
						},
						LowerBound: &QuantityLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&RuneRangeExpression{
						NodeBase: NodeBase{NodeSpan{0, 8}, nil, []Token{{Type: TWO_DOTS, Span: NodeSpan{3, 5}}}},
						Lower: &RuneLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Value:    'a',
						},
						Upper: &RuneLiteral{
							NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{4, 6},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
							Name:     "int",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{5, 8}, nil, nil},
							Unprefixed: true,
							Name:       "int",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, empty capture list, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "fn[](){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},
						},
						CaptureList: nil,
						Parameters:  nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{6, 8},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{4, 5}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
						},
						CaptureList: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "a",
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: COMMA, Span: NodeSpan{4, 5}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{6, 7}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{7, 8}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						CaptureList: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     "a",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Name:     "b",
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_BRACKET, Span: NodeSpan{2, 3}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{4, 5}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
						},
						CaptureList: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{3, 4},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInCaptureList('?')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{3, 4}, Raw: "?"}},
								},
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "x",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{5, 7},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 9}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
									Name:     "int",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 8}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{5, 8}, nil, nil},
									Unprefixed: true,
									Name:       "int",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: COMMA, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "x",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
									Name:     "n",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: ARROW, Span: NodeSpan{6, 8}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "x",
								},
							},
						},
						IsBodyExpression: true,
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 2},
							&ParsingError{InvalidNext, FN_KEYWORD_OR_FUNC_NAME_SHOULD_BE_FOLLOWED_BY_PARAMS},
							[]Token{{Type: FN_KEYWORD, Span: NodeSpan{0, 2}}},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{4, 5},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{4, 6},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{4, 5}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},
						},
						Parameters: nil,
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{3, 4},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{3, 4}, Raw: ":"}},
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{5, 7},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "a",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
									Name:     "b",
								},
							},
						},
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{4, 5},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{4, 5}, Raw: ":"}},
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{7, 9},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW},
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},
						},
						Parameters: nil,
						Body:       nil,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN},
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
							},
						},
						Parameters: nil,
						Body:       nil,
					},
				},
			}, n)
		})

		t.Run("unterminated parameter list: followed by newline", func(t *testing.T) {
			n, err := parseChunk(t, "fn(\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PARAM_LIST_MISSING_CLOSING_PAREN},
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: NEWLINE, Span: NodeSpan{3, 4}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&FunctionExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{
								{Type: FN_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{2, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 11},
									&ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES},
									nil,
								},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, nil},
									Name:     "manifest",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{12, 14},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
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
		t.Run("no parameters, no manifest, empty body", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},
						},
						Parameters: nil,
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{5, 7},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, empty body, return type", func(t *testing.T) {
			n := mustparseChunk(t, "%fn() %int {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{6, 10}, nil, nil},
							Name:     "int",
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{11, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("no parameters, empty body, unprefixed return type", func(t *testing.T) {
			n := mustparseChunk(t, "%fn() int {}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},
						},
						Parameters: nil,
						ReturnType: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{6, 9}, nil, nil},
							Name:       "int",
							Unprefixed: true,
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "x",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{6, 8},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single typed parameter, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x %int){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 10}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 10}, nil, nil},
									Name:     "int",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{11, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single typed parameter with unprefixed type, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x int){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 9}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "x",
								},
								Type: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 9}, nil, nil},
									Name:       "int",
									Unprefixed: true,
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single parameter with no name, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(%int){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
								Type: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
									Name:     "int",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("two parameters, empty body ", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x,n){}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: COMMA, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "x",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Name:     "n",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 10},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("single parameter, body is an expression", func(t *testing.T) {
			n := mustparseChunk(t, "%fn(x) => x")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: ARROW, Span: NodeSpan{7, 9}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "x",
								},
							},
						},
						IsBodyExpression: true,
						Body: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
							Name:     "x",
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char in empty parameter list", func(t *testing.T) {
			n, err := parseChunk(t, "%fn(:){}", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{5, 6}},
							},
						},
						Parameters: nil,
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{4, 5},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{4, 5}, Raw: ":"}},
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{6, 8},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("unexpected char in non-empty parameter list", func(t *testing.T) {
			n, err := parseChunk(t, "%fn(a:b){}", "")

			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
									Name:     "a",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Name:     "b",
								},
							},
						},
						AdditionalInvalidNodes: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{5, 6},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInParameters(':')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{5, 6}, Raw: ":"}},
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{8, 10},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})

		t.Run("parameter list not followed by a block", func(t *testing.T) {
			n, err := parseChunk(t, "%fn()1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							&ParsingError{InvalidNext, PARAM_LIST_OF_FUNC_PATT_SHOULD_BE_FOLLOWED_BY_BLOCK_OR_ARROW},
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{4, 5}},
							},
						},
						Parameters: nil,
						Body:       nil,
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("parameter name should not be a keyword ", func(t *testing.T) {
			n, err := parseChunk(t, "%fn(manifest){}", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&FunctionPatternExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{
								{Type: PERCENT_FN, Span: NodeSpan{0, 3}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
							},
						},
						Parameters: []*FunctionParameter{
							{
								NodeBase: NodeBase{
									NodeSpan{4, 12},
									&ParsingError{UnspecifiedParsingError, KEYWORDS_SHOULD_NOT_BE_USED_AS_PARAM_NAMES},
									nil,
								},
								Var: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{4, 12}, nil, nil},
									Name:     "manifest",
								},
							},
						},
						Body: &Block{
							NodeBase: NodeBase{
								NodeSpan{13, 15},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
								},
							},
							Statements: nil,
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern conversion expression", func(t *testing.T) {
		n := mustparseChunk(t, "%(1)")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
			Statements: []Node{
				&PatternConversionExpression{
					NodeBase: NodeBase{
						NodeSpan{0, 3},
						nil,
						[]Token{
							{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}},
						},
					},
					Value: &IntLiteral{
						NodeBase: NodeBase{
							NodeSpan{2, 3},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&LazyExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{{Type: AT_SIGN, Span: NodeSpan{0, 1}}},
						},
						Expression: &IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&LazyExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							nil,
							[]Token{{Type: AT_SIGN, Span: NodeSpan{0, 1}}},
						},
						Expression: &IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_PARENTHESIZED_EXPR_MISSING_CLOSING_PAREN},
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&LazyExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{{Type: AT_SIGN, Span: NodeSpan{0, 1}}},
						},
						Expression: &IntLiteral{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								nil,
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{1, 2}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{3, 4}},
								},
							},
							Raw:   "1",
							Value: 1,
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 12},
								nil,
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
					NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 18},
								nil,
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 16}, nil, nil},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 16},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											},
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
					NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 28},
								nil,
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{},
							DefaultCases: []*DefaultCase{
								{
									NodeBase: NodeBase{
										NodeSpan{11, 26},
										nil,
										[]Token{{Type: DEFAULTCASE_KEYWORD, Span: NodeSpan{11, 22}}},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{23, 26},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
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
				input:    "switch 1 { 1 { } 2 { } }",
				hasError: false,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 24},
								nil,
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 16}, nil, nil},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 16},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											},
										},
										Statements: nil,
									},
								},
								{
									NodeBase: NodeBase{NodeSpan{17, 22}, nil, nil},
									Values: []Node{

										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{17, 18}, nil, nil},
											Raw:      "2",
											Value:    2,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{19, 22},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
											},
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
					NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 21},
								nil,
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 19}, nil, []Token{{Type: COMMA, Span: NodeSpan{12, 13}}}},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
											Raw:      "1",
											Value:    1,
										},
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{14, 15}, nil, nil},
											Raw:      "2",
											Value:    2,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{16, 19},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
											},
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
					NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 16},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 16}, nil, nil},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 16},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											},
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
					NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 26},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{},
							DefaultCases: []*DefaultCase{
								{
									NodeBase: NodeBase{
										NodeSpan{11, 26},
										nil,
										[]Token{{Type: DEFAULTCASE_KEYWORD, Span: NodeSpan{11, 22}}},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{23, 26},
											nil,
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
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
				input:    "switch 1 { 1 {",
				hasError: true,
				result: &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 14},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, nil},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 14},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
											},
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
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
					Statements: []Node{
						&SwitchStatement{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								&ParsingError{UnspecifiedParsingError, UNTERMINATED_SWITCH_STMT_MISSING_CLOSING_BRACE},
								[]Token{
									{Type: SWITCH_KEYWORD, Span: NodeSpan{0, 6}},
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Discriminant: &IntLiteral{
								NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							Cases: []*SwitchCase{
								{
									NodeBase: NodeBase{NodeSpan{11, 15}, nil, nil},
									Values: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
											Raw:      "1",
											Value:    1,
										},
									},
									Block: &Block{
										NodeBase: NodeBase{
											NodeSpan{13, 15},
											&ParsingError{UnspecifiedParsingError, UNTERMINATED_BLOCK_MISSING_BRACE},
											[]Token{
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
											},
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
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
				Statements: []Node{
					&MatchStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							[]Token{
								{Type: MATCH_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
							},
						},
						Discriminant: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						Cases: []*MatchCase{
							{
								NodeBase: NodeBase{NodeSpan{10, 18}, nil, nil},
								Values: []Node{
									&ObjectLiteral{
										NodeBase: NodeBase{
											NodeSpan{11, 13},
											nil,
											[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{10, 11}},
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
											},
										},
									},
								},
								Block: &Block{
									NodeBase: NodeBase{
										NodeSpan{15, 18},
										nil,
										[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
										},
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
				NodeBase: NodeBase{NodeSpan{0, 36}, nil, nil},
				Statements: []Node{
					&MatchStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 36},
							nil,
							[]Token{
								{Type: MATCH_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
							},
						},
						Discriminant: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						Cases: []*MatchCase{
							{
								NodeBase: NodeBase{NodeSpan{10, 34}, nil, nil},
								Values: []Node{
									&NamedSegmentPathPatternLiteral{
										NodeBase: NodeBase{
											NodeSpan{10, 28},
											nil,
											[]Token{
												{Type: PERCENT_SYMBOL, Span: NodeSpan{10, 11}},
												{Type: SINGLE_INTERP_OPENING_BRACE, Span: NodeSpan{17, 18}},
												{Type: SINGLE_INTERP_CLOSING_BRACE, Span: NodeSpan{27, 28}},
											},
										},
										Slices: []Node{
											&PathPatternSlice{
												NodeBase: NodeBase{NodeSpan{11, 17}, nil, nil},
												Value:    "/home/",
											},
											&NamedPathSegment{
												NodeBase: NodeBase{NodeSpan{18, 27}, nil, nil},
												Name:     "username",
											},
										},
										Raw:         "%/home/{:username}",
										StringValue: "%/home/{:username}",
									},
								},
								GroupMatchingVariable: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{29, 30}, nil, nil},
									Name:     "m",
								},
								Block: &Block{
									NodeBase: NodeBase{
										NodeSpan{31, 34},
										nil,
										[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{31, 32}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{33, 34}},
										},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&MatchStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{
								{Type: MATCH_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
							},
						},
						Discriminant: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						Cases: []*MatchCase{
							{
								NodeBase: NodeBase{NodeSpan{10, 12}, nil, nil},
								Values: []Node{
									&MissingExpression{
										NodeBase: NodeBase{
											NodeSpan{10, 11},
											&ParsingError{UnspecifiedParsingError, fmtCaseValueExpectedHere([]rune(s), 10, true)},
											nil,
										},
									},
								},
								Block: &Block{
									NodeBase: NodeBase{
										NodeSpan{10, 12},
										nil,
										[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
										},
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
				[]Token{{Type: COMMENT, Span: NodeSpan{0, 2}, Raw: "# "}},
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
				[]Token{{Type: COMMENT, Span: NodeSpan{0, 11}, Raw: "# some text"}},
			},
			Statements: nil,
		}, n)
	})

	t.Run("import statement", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, `import a https://example.com/a.ix {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 36}, nil, nil},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 36},
							nil,
							[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
							Name:     "a",
						},
						Source: &URLLiteral{
							NodeBase: NodeBase{NodeSpan{9, 33}, nil, nil},
							Value:    "https://example.com/a.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{34, 36},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{34, 35}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{35, 36}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&ImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							&ParsingError{UnspecifiedParsingError, IMPORT_STMT_SRC_SHOULD_BE_AN_URL_OR_PATH_LIT},
							[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},
						},
						Identifier: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{7, 10}, nil, nil},
							Name:     "res",
						},
						Source: &InvalidURL{
							NodeBase: NodeBase{
								NodeSpan{11, 22},
								&ParsingError{UnspecifiedParsingError, INVALID_URL_OR_HOST},
								nil,
							},
							Value: "https://.ix",
						},
						Configuration: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{23, 25},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&InclusionImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},
						},
						Source: &RelativePathLiteral{
							NodeBase: NodeBase{NodeSpan{7, 16}, nil, nil},
							Value:    "./file.ix",
							Raw:      "./file.ix",
						},
					},
				},
			}, n)
		})

		t.Run("absolute path literal", func(t *testing.T) {
			n := mustparseChunk(t, `import /file.ix`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&InclusionImportStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{
								{Type: IMPORT_KEYWORD, Span: NodeSpan{0, 6}},
							},
						},
						Source: &AbsolutePathLiteral{
							NodeBase: NodeBase{NodeSpan{7, 15}, nil, nil},
							Value:    "/file.ix",
							Raw:      "/file.ix",
						},
					},
				},
			}, n)
		})

	})

	t.Run("spawn expression", func(t *testing.T) {
		t.Run("call expression", func(t *testing.T) {
			n := mustparseChunk(t, `go nil do f()`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
						},
						Module: &EmbeddedModule{
							NodeBase:       NodeBase{NodeSpan{10, 13}, nil, nil},
							SingleCallExpr: true,
							Statements: []Node{
								&CallExpression{
									NodeBase: NodeBase{
										NodeSpan{10, 13},
										nil,
										[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{11, 12}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
										},
									},
									Callee: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
										Name:     "f",
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 25},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},
							},
							Manifest: &Manifest{
								NodeBase: NodeBase{
									Span: NodeSpan{12, 23},
									Tokens: []Token{
										{Type: MANIFEST_KEYWORD, Span: NodeSpan{12, 20}},
									},
								},
								Object: &ObjectLiteral{
									NodeBase: NodeBase{
										NodeSpan{21, 23},
										nil,
										[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
										},
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
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 17},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},
							},
							Statements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
								&Variable{
									NodeBase: NodeBase{
										NodeSpan{13, 15},
										&ParsingError{UnspecifiedParsingError, STMTS_SHOULD_BE_SEPARATED_BY},
										nil,
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&SpawnExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_SPAWN_EXPRESSION_MISSING_EMBEDDED_MODULE_AFTER_DO_KEYWORD},
							[]Token{
								{Type: GO_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: DO_KEYWORD, Span: NodeSpan{7, 9}},
							},
						},
						Meta: &NilLiteral{
							NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty, missing closing brace", func(t *testing.T) {
			n, err := parseChunk(t, `Mapping {`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE},
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("static entry", func(t *testing.T) {
			n := mustparseChunk(t, "Mapping { 0 => 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
							},
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{{Type: ARROW, Span: NodeSpan{12, 14}}},
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
									Raw:      "1",
									Value:    1,
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
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
							},
						},
						Entries: []Node{
							&DynamicMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 18},
									nil,
									[]Token{{Type: ARROW, Span: NodeSpan{14, 16}}},
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
								KeyVar: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Name:     "n",
								},
								ValueComputation: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{17, 18}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							nil,
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
							},
						},
						Entries: []Node{
							&DynamicMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 21},
									nil,
									[]Token{{Type: ARROW, Span: NodeSpan{17, 19}}},
								},
								Key: &AbsolutePathPatternLiteral{
									NodeBase: NodeBase{NodeSpan{12, 14}, nil, nil},
									Raw:      "%/",
									Value:    "/",
								},
								KeyVar: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Name:     "p",
								},
								GroupMatchingVariable: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
									Name:     "m",
								},
								ValueComputation: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, nil},
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

		t.Run("static entry, missing closing brace", func(t *testing.T) {
			n, err := parseChunk(t, "Mapping { 0 => 1", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_MAPPING_EXPRESSION_MISSING_CLOSING_BRACE},
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
							},
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{{Type: ARROW, Span: NodeSpan{12, 14}}},
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{{Type: ARROW, Span: NodeSpan{12, 14}}},
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
								Value: &MissingExpression{
									NodeBase: NodeBase{
										NodeSpan{15, 16},
										&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("Mapping { 0 => }"), 15, true)},
										nil,
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
				NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
				Statements: []Node{
					&MappingExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 28},
							nil,
							[]Token{
								{Type: MAPPING_KEYWORD, Span: NodeSpan{0, 7}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{27, 28}},
							},
						},
						Entries: []Node{
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{{Type: ARROW, Span: NodeSpan{12, 14}}},
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
							},
							&StaticMappingEntry{
								NodeBase: NodeBase{
									NodeSpan{20, 26},
									nil,
									[]Token{{Type: ARROW, Span: NodeSpan{22, 24}}},
								},
								Key: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, nil},
									Raw:      "2",
									Value:    2,
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{25, 26}, nil, nil},
									Raw:      "3",
									Value:    3,
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("udata expression", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n := mustparseChunk(t, `udata 0 {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&UDataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: UDATA_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
							},
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("empty, missing closing brace", func(t *testing.T) {
			n, err := parseChunk(t, `udata 0 {`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&UDataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_UDATA_LIT_MISSING_CLOSING_BRACE},
							[]Token{
								{Type: UDATA_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
							},
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
					},
				},
			}, n)
		})

		t.Run("single entry", func(t *testing.T) {
			n := mustparseChunk(t, "udata 0 { 0 {} }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&UDataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: UDATA_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
						Children: []*UDataEntry{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 14},
									nil,
									[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
									},
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("single entry without braces", func(t *testing.T) {
			n := mustparseChunk(t, "udata 0 { 0 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&UDataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: UDATA_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
							},
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
						Children: []*UDataEntry{
							{
								NodeBase: NodeBase{NodeSpan{10, 12}, nil, nil},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("two entries", func(t *testing.T) {
			n := mustparseChunk(t, "udata 0 { 0 {} 1 {} }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
				Statements: []Node{
					&UDataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							[]Token{
								{Type: UDATA_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{20, 21}},
							},
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
						Children: []*UDataEntry{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 14},
									nil,
									[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
									},
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{15, 19},
									nil,
									[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
									},
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
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
			n := mustparseChunk(t, "udata 0 { 0 {}, 1 {} }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
				Statements: []Node{
					&UDataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							[]Token{
								{Type: UDATA_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: COMMA, Span: NodeSpan{14, 15}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
							},
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
						Children: []*UDataEntry{
							{
								NodeBase: NodeBase{
									NodeSpan{10, 14},
									nil,
									[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
									},
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{16, 20},
									nil,
									[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
									},
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{16, 17}, nil, nil},
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
			n := mustparseChunk(t, "udata 0 { 0 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&UDataLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							nil,
							[]Token{
								{Type: UDATA_KEYWORD, Span: NodeSpan{0, 5}},
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{14, 15}},
							},
						},
						Root: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
							Raw:      "0",
							Value:    0,
						},
						Children: []*UDataEntry{
							{
								NodeBase: NodeBase{NodeSpan{10, 12}, nil, nil},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
									Raw:      "0",
									Value:    0,
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{12, 14}, nil, nil},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{{Type: TESTSUITE_KEYWORD, Span: NodeSpan{0, 9}}},
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 12},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("with meta", func(t *testing.T) {
			n := mustparseChunk(t, `testsuite "name" {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							[]Token{{Type: TESTSUITE_KEYWORD, Span: NodeSpan{0, 9}}},
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
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("embedded module with manifest", func(t *testing.T) {
			n := mustparseChunk(t, `testsuite { manifest {} }`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&TestSuiteExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{{Type: TESTSUITE_KEYWORD, Span: NodeSpan{0, 9}}},
						},
						IsStatement: true,
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{10, 25},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},
							},
							Manifest: &Manifest{
								NodeBase: NodeBase{
									Span: NodeSpan{12, 23},
									Tokens: []Token{
										{Type: MANIFEST_KEYWORD, Span: NodeSpan{12, 20}},
									},
								},
								Object: &ObjectLiteral{
									NodeBase: NodeBase{
										NodeSpan{21, 23},
										nil,
										[]Token{
											{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{21, 22}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{22, 23}},
										},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							&ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
							[]Token{{Type: TESTSUITE_KEYWORD, Span: NodeSpan{0, 9}}},
						},
					},
				},
			}, n)
		})

		t.Run("with meta but missing embedded module", func(t *testing.T) {
			n, err := parseChunk(t, `testsuite "name"`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&TestSuiteExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{MissingBlock, UNTERMINATED_TESTSUITE_EXPRESSION_MISSING_BLOCK},
							[]Token{{Type: TESTSUITE_KEYWORD, Span: NodeSpan{0, 9}}},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&TestCaseExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{{Type: TESTCASE_KEYWORD, Span: NodeSpan{0, 8}}},
						},
						IsStatement: true,
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{9, 11},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("with meta", func(t *testing.T) {
			n := mustparseChunk(t, `testcase "name" {}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&TestCaseExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{{Type: TESTCASE_KEYWORD, Span: NodeSpan{0, 8}}},
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
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&TestCaseExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{MissingBlock, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
							[]Token{{Type: TESTCASE_KEYWORD, Span: NodeSpan{0, 8}}},
						},
					},
				},
			}, n)
		})

		t.Run("with meta but missing embedded module", func(t *testing.T) {
			n, err := parseChunk(t, `testcase "name"`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&TestCaseExpression{
						IsStatement: true,
						NodeBase: NodeBase{
							NodeSpan{0, 15},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_TESTCASE_EXPRESSION_MISSING_BLOCK},
							[]Token{{Type: TESTCASE_KEYWORD, Span: NodeSpan{0, 8}}},
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
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							[]Token{{Type: LIFETIMEJOB_KEYWORD, Span: NodeSpan{0, 11}}},
						},
						Meta: &UnambiguousIdentifierLiteral{
							NodeBase: NodeBase{Span: NodeSpan{12, 16}},
							Name:     "job",
						},
						Module: &EmbeddedModule{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_META},
							[]Token{{Type: LIFETIMEJOB_KEYWORD, Span: NodeSpan{0, 11}}},
						},
					},
				},
			}, n)
		})

		t.Run("missing embedded module after meta", func(t *testing.T) {
			n, err := parseChunk(t, `lifetimejob #job`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_EMBEDDED_MODULE},
							[]Token{{Type: LIFETIMEJOB_KEYWORD, Span: NodeSpan{0, 11}}},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							[]Token{
								{Type: LIFETIMEJOB_KEYWORD, Span: NodeSpan{0, 11}},
								{Type: FOR_KEYWORD, Span: NodeSpan{17, 20}},
							},
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
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 23}, nil, nil},
				Statements: []Node{
					&LifetimejobExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 23},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_LIFETIMEJOB_EXPRESSION_MISSING_EMBEDDED_MODULE},
							[]Token{
								{Type: LIFETIMEJOB_KEYWORD, Span: NodeSpan{0, 11}},
								{Type: FOR_KEYWORD, Span: NodeSpan{17, 20}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
				Statements: []Node{
					&ReceptionHandlerExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 20},
							nil,
							[]Token{
								{Type: ON_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: RECEIVED_KEYWORD, Span: NodeSpan{3, 11}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&ReceptionHandlerExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_PATTERN},
							[]Token{
								{Type: ON_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: RECEIVED_KEYWORD, Span: NodeSpan{3, 11}},
							},
						},
					},
				},
			}, n)
		})

		t.Run("missing body after 'do' keyword", func(t *testing.T) {
			n, err := parseChunk(t, `on received %event`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&ReceptionHandlerExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_RECEP_HANDLER_MISSING_HANDLER_OR_PATTERN},
							[]Token{
								{Type: ON_KEYWORD, Span: NodeSpan{0, 2}},
								{Type: RECEIVED_KEYWORD, Span: NodeSpan{3, 11}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&ComputeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{{Type: COMP_KEYWORD, Span: NodeSpan{0, 4}}},
						},
						Arg: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{3, 4},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("comp"), 4, true)},
								nil,
							},
						},
					},
				},
			}, n)
		})

		t.Run("ok", func(t *testing.T) {
			n := mustparseChunk(t, `comp 1`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&ComputeExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{{Type: COMP_KEYWORD, Span: NodeSpan{0, 4}}},
						},
						Arg: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{5, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&PermissionDroppingStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{{Type: DROP_PERMS_KEYWORD, Span: NodeSpan{0, 10}}},
						},
						Object: &ObjectLiteral{
							NodeBase: NodeBase{
								NodeSpan{11, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&ReturnStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{{Type: RETURN_KEYWORD, Span: NodeSpan{0, 6}}},
						},
						Expr: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&ReturnStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{{Type: RETURN_KEYWORD, Span: NodeSpan{0, 6}}},
						},
					},
				},
			}, n)
		})

		t.Run("no value, followed by newline", func(t *testing.T) {
			n := mustparseChunk(t, "return\n")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 7},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{6, 7}},
					},
				},
				Statements: []Node{
					&ReturnStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{{Type: RETURN_KEYWORD, Span: NodeSpan{0, 6}}},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&YieldStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{{Type: YIELD_KEYWORD, Span: NodeSpan{0, 5}}},
						},
						Expr: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&YieldStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							[]Token{{Type: YIELD_KEYWORD, Span: NodeSpan{0, 5}}},
						},
					},
				},
			}, n)
		})

		t.Run("no value, followed by newline", func(t *testing.T) {
			n := mustparseChunk(t, "yield\n")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 6},
					nil,
					[]Token{
						{Type: NEWLINE, Span: NodeSpan{5, 6}},
					},
				},
				Statements: []Node{
					&YieldStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 5},
							nil,
							[]Token{{Type: YIELD_KEYWORD, Span: NodeSpan{0, 5}}},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Expr: &Variable{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "err",
						},
					},
				},
			}, n)
		})

		t.Run("identifier", func(t *testing.T) {
			n := mustparseChunk(t, "err?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Expr: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Name:     "err",
						},
					},
				},
			}, n)
		})

		t.Run("identifier member expression", func(t *testing.T) {
			n := mustparseChunk(t, "a.b?")

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Expr: &IdentifierMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyNames: []*IdentifierLiteral{
								{
									NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Expr: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Expr: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&BooleanConversionExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Expr: &MemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Left: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_CONCAT_EXPR_ELEMS_EXPECTED},
							[]Token{{Type: CONCAT_KEYWORD, Span: NodeSpan{0, 6}}},
						},
						Elements: nil,
					},
				},
			}, n)
		})

		t.Run("missing elements: newline", func(t *testing.T) {
			n, err := parseChunk(t, "concat\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 7},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{6, 7}}},
				},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_CONCAT_EXPR_ELEMS_EXPECTED},
							[]Token{{Type: CONCAT_KEYWORD, Span: NodeSpan{0, 6}}},
						},
						Elements: nil,
					},
				},
			}, n)
		})

		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, `concat "a"`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{{Type: CONCAT_KEYWORD, Span: NodeSpan{0, 6}}},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{{Type: CONCAT_KEYWORD, Span: NodeSpan{0, 6}}},
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
				NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
				Statements: []Node{
					&ListLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 21},
							nil,
							[]Token{
								{Type: OPENING_BRACKET, Span: NodeSpan{0, 1}},
								{Type: COMMA, Span: NodeSpan{15, 16}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{20, 21}},
							},
						},
						Elements: []Node{
							&ConcatenationExpression{
								NodeBase: NodeBase{
									NodeSpan{1, 15},
									nil,
									[]Token{{Type: CONCAT_KEYWORD, Span: NodeSpan{1, 7}}},
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

		t.Run("spread element", func(t *testing.T) {
			n := mustparseChunk(t, `concat ...a`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&ConcatenationExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{{Type: CONCAT_KEYWORD, Span: NodeSpan{0, 6}}},
						},
						Elements: []Node{
							&ElementSpreadElement{
								NodeBase: NodeBase{
									Span:   NodeSpan{7, 11},
									Tokens: []Token{{Type: THREE_DOTS, Span: NodeSpan{7, 10}}},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&PatternIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
						Name:     "int",
					},
				},
			}, n)
		})

		t.Run("percent only", func(t *testing.T) {
			n, err := parseChunk(t, "%", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
							[]Token{{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}}},
						},
					},
				},
			}, n)
		})

		t.Run("percent followed by newline", func(t *testing.T) {
			n, err := parseChunk(t, "%\n", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 2},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{1, 2}}},
				},
				Statements: []Node{
					&UnknownNode{
						NodeBase: NodeBase{
							NodeSpan{0, 1},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT},
							[]Token{{Type: PERCENT_SYMBOL, Span: NodeSpan{0, 1}}},
						},
					},
				},
			}, n)
		})
	})

	t.Run("pattern namespace identifier literal", func(t *testing.T) {
		n := mustparseChunk(t, "%mynamespace.")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
			Statements: []Node{
				&PatternNamespaceIdentifierLiteral{
					NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
					Name:     "mynamespace",
				},
			},
		}, n)
	})

	t.Run("object pattern", func(t *testing.T) {

		// t.Run("{ ... } ", func(t *testing.T) {
		// 	n := mustparseChunk(t,"%{ ... }")
		// 	assert.EqualValues(t, &Chunk{
		// 		NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
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
		// 				Exact: false,
		// 			},
		// 		},
		// 	}, n)
		// })

		// t.Run("{ ... , name: %str } ", func(t *testing.T) {
		// 	n := mustparseChunk(t,"%{ ... , name: %str }")
		// 	assert.EqualValues(t, &Chunk{
		// 		NodeBase: NodeBase{NodeSpan{0, 21}, nil, nil},
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
		// 				Exact: false,
		// 				Properties: []*ObjectPatternProperty{
		// 					{
		// 						NodeBase: NodeBase{
		// 							NodeSpan{9, 19},
		// 							nil,
		// 							[]Token{{Type: COLON, Span: NodeSpan{13, 14}}},
		// 						},
		// 						Key: &IdentifierLiteral{
		// 							NodeBase: NodeBase{NodeSpan{9, 13}, nil, nil},
		// 							Name:     "name",
		// 						},
		// 						Value: &PatternIdentifierLiteral{
		// 							NodeBase: NodeBase{NodeSpan{15, 19}, nil, nil},
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
		// 		NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
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
		// 				Exact: false,
		// 			},
		// 		},
		// 	}, n)
		// })

		t.Run("{ ...named-pattern } ", func(t *testing.T) {
			n := mustparseChunk(t, "%{ ...%patt }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
							},
						},
						Exact: false,
						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 11},
									nil,
									[]Token{{Type: THREE_DOTS, Span: NodeSpan{3, 6}}},
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
							},
						},
						Exact: false,
						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 10},
									nil,
									[]Token{{Type: THREE_DOTS, Span: NodeSpan{3, 6}}},
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{6, 10}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 26}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 26},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{13, 14}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{25, 26}},
							},
						},
						Exact: false,
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 13},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 13}, nil, nil},
									Name:     "str",
								},
							},
						},
						SpreadElements: []*PatternPropertySpreadElement{
							{
								NodeBase: NodeBase{
									NodeSpan{16, 24},
									&ParsingError{UnspecifiedParsingError, SPREAD_SHOULD_BE_LOCATED_AT_THE_START},
									[]Token{{Type: THREE_DOTS, Span: NodeSpan{16, 19}}},
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{19, 24}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{12, 13}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
							},
						},
						Exact: false,
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 12},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{7, 8}}},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{9, 12}, nil, nil},
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
									[]Token{{Type: THREE_DOTS, Span: NodeSpan{15, 18}}},
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{18, 23}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{11, 12}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
							},
						},
						Exact: false,
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{13, 22},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{17, 18}}},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{13, 17}, nil, nil},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase:   NodeBase{NodeSpan{19, 22}, nil, nil},
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
									[]Token{{Type: THREE_DOTS, Span: NodeSpan{3, 6}}},
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{6, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
							},
						},
						Exact: false,
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{2, 12},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{6, 7}}},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 6}, nil, nil},
									Name:     "keys",
								},
								Value: &KeyListExpression{
									NodeBase: NodeBase{
										NodeSpan{Start: 8, End: 12},
										nil,
										[]Token{
											{Type: OPENING_KEYLIST_BRACKET, Span: NodeSpan{8, 10}},
											{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
										},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},
						},
						Exact: false,
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 14},
									nil,
									[]Token{
										{Type: QUESTION_MARK, Span: NodeSpan{7, 8}},
										{Type: COLON, Span: NodeSpan{8, 9}},
									},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
									Name:     "name",
								},
								Value: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 14},
									nil,
									[]Token{
										{Type: COLON, Span: NodeSpan{7, 8}},
									},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{Start: 3, End: 7}},
									Name:     "list",
								},
								Value: &ListPatternLiteral{
									NodeBase: NodeBase{
										NodeSpan{9, 14},
										nil,
										[]Token{
											{Type: OPENING_BRACKET, Span: NodeSpan{9, 10}},
											{Type: CLOSING_BRACKET, Span: NodeSpan{13, 14}},
										},
									},
									Elements: []Node{
										&IntLiteral{
											NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
							},
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 17},
									nil,
									[]Token{
										{Type: COLON, Span: NodeSpan{7, 8}},
									},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{Start: 3, End: 7}},
									Name:     "prop",
								},
								Value: &PatternUnion{
									NodeBase: NodeBase{
										NodeSpan{9, 17},
										nil,
										[]Token{
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{9, 10}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{13, 14}},
										},
									},
									Cases: []Node{
										&PatternIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{11, 12}, nil, nil},
											Unprefixed: true,
											Name:       "a",
										},
										&PatternIdentifierLiteral{
											NodeBase:   NodeBase{NodeSpan{15, 16}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 16},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
							},
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 14},
									nil,
									[]Token{
										{Type: COLON, Span: NodeSpan{7, 8}},
									},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{Span: NodeSpan{Start: 3, End: 7}},
									Name:     "prop",
								},
								Value: &PatternConversionExpression{
									NodeBase: NodeBase{
										NodeSpan{9, 13},
										nil,
										[]Token{
											{Type: PERCENT_SYMBOL, Span: NodeSpan{9, 10}},
										},
									},
									Value: &ObjectLiteral{
										NodeBase: NodeBase{
											NodeSpan{11, 13},
											nil,
											[]Token{
												{Type: OPENING_PARENTHESIS, Span: NodeSpan{10, 11}},
												{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{11, 12}},
												{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
												{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
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

		t.Run("duplicate key", func(t *testing.T) {
			n, err := parseChunk(t, "%{ a: 1, a: 2}", "")
			assert.NoError(t, err)

			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{7, 8}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
							},
						},
						Properties: []*ObjectPatternProperty{
							{
								NodeBase: NodeBase{
									NodeSpan{3, 7},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{4, 5}}},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "a",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
							},
							{
								NodeBase: NodeBase{
									NodeSpan{9, 13},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{10, 11}}},
								},
								Key: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
									Name:     "a",
								},
								Value: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 3},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_OBJ_PATTERN_MISSING_CLOSING_BRACE},
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{2, 3}},
							},
						},
						Exact: false,
					},
				},
			}, n)
		})
		t.Run("%{,}", func(t *testing.T) {
			n := mustparseChunk(t, "%{,}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&ObjectPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							nil,
							[]Token{
								{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{2, 3}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
							},
						},
						Exact: false,
					},
				},
			}, n)
		})
	})

	t.Run("list pattern", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, "%[ 1 ]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
							},
						},
						Elements: []Node{
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{5, 6}},
							},
						},
						Elements: []Node{
							&PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{17, 18}},
							},
						},
						Elements: []Node{
							&ObjectPatternLiteral{
								NodeBase: NodeBase{
									NodeSpan{2, 17},
									nil,
									[]Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{2, 3}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
									},
								},
								Exact: false,
								Properties: []*ObjectPatternProperty{
									{
										NodeBase: NodeBase{
											NodeSpan{4, 15},
											nil,
											[]Token{
												{Type: QUESTION_MARK, Span: NodeSpan{8, 9}},
												{Type: COLON, Span: NodeSpan{9, 10}},
											},
										},
										Key: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
											Name:     "name",
										},
										Value: &PatternIdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{11, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: COMMA, Span: NodeSpan{4, 5}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{8, 9}},
							},
						},
						Elements: []Node{
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
							},
						},
						Elements: nil,
						GeneralElement: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{3, 7}, nil, nil},
							Name:     "int",
						},
					},
				},
			}, n)
		})

		t.Run("general element is an unprefixed named pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%[]int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
							},
						},
						Elements: nil,
						GeneralElement: &PatternIdentifierLiteral{
							NodeBase:   NodeBase{NodeSpan{3, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{2, 3}},
							},
						},
						Elements: nil,
						GeneralElement: &ObjectPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{3, 18},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{3, 4}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},
							},
							Exact: false,
							Properties: []*ObjectPatternProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{5, 16},
										nil,
										[]Token{
											{Type: QUESTION_MARK, Span: NodeSpan{9, 10}},
											{Type: COLON, Span: NodeSpan{10, 11}},
										},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
										Name:     "name",
									},
									Value: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{12, 16}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&ListPatternLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							&ParsingError{UnspecifiedParsingError, INVALID_LIST_TUPLE_PATT_GENERAL_ELEMENT_IF_ELEMENTS},
							[]Token{
								{Type: OPENING_LIST_PATTERN_BRACKET, Span: NodeSpan{0, 2}},
								{Type: CLOSING_BRACKET, Span: NodeSpan{3, 4}},
							},
						},
						Elements: []Node{
							&IntLiteral{
								NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
								Raw:      "1",
								Value:    1,
							},
						},
						GeneralElement: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{4, 8}, nil, nil},
							Name:     "int",
						},
					},
				},
			}, n)
		})
	})

	t.Run("tuple pattern", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			n := mustparseChunk(t, "%p = #[ 1 ]")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "p",
						},
						Right: &TuplePatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{5, 11},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{5, 7}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{10, 11}},
								},
							},
							Elements: []Node{
								&IntLiteral{
									NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
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
			n := mustparseChunk(t, "%p = #[]int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "p",
						},
						Right: &TuplePatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{5, 11},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{5, 7}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Elements: nil,
							GeneralElement: &PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{8, 11}, nil, nil},
								Name:       "int",
								Unprefixed: true,
							},
						},
					},
				},
			}, n)
		})

		t.Run("general element: empty tuple pattern", func(t *testing.T) {
			n := mustparseChunk(t, "%p = #[]#{}")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "p",
						},
						Right: &TuplePatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{5, 11},
								nil,
								[]Token{
									{Type: OPENING_TUPLE_BRACKET, Span: NodeSpan{5, 7}},
									{Type: CLOSING_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Elements: nil,
							GeneralElement: &RecordPatternLiteral{
								NodeBase: NodeBase{
									NodeSpan{8, 11},
									nil,
									[]Token{
										{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{8, 10}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{10, 11}},
									},
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
			n := mustparseChunk(t, "%i = %int")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "i",
						},
						Right: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
							Name:     "int",
						},
					},
				},
			}, n)
		})

		t.Run("lazy", func(t *testing.T) {
			n := mustparseChunk(t, "%i = @ 1")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						IsLazy: true,
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "i",
						},
						Right: &IntLiteral{
							NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
					},
				},
			}, n)
		})

		t.Run("RHS is an object pattern literal", func(t *testing.T) {

			n := mustparseChunk(t, "%i = %{ a: 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 14},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "i",
						},
						Right: &ObjectPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{5, 14},
								nil,
								[]Token{
									{Type: OPENING_OBJECT_PATTERN_BRACKET, Span: NodeSpan{5, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{13, 14}},
								},
							},
							Properties: []*ObjectPatternProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{8, 12},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{9, 10}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{11, 12}, nil, nil},
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

			n := mustparseChunk(t, "%i = { a: 1 }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "i",
						},
						Right: &ObjectPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{5, 13},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{12, 13}},
								},
							},
							Properties: []*ObjectPatternProperty{
								{
									NodeBase: NodeBase{
										NodeSpan{7, 11},
										nil,
										[]Token{{Type: COLON, Span: NodeSpan{8, 9}}},
									},
									Key: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
										Name:     "a",
									},
									Value: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{10, 11}, nil, nil},
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
			n, err := parseChunk(t, "%i =", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 4},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_PATT_DEF_MISSING_RHS},
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "i",
						},
					},
				},
			}, n)
		})

	})

	t.Run("pattern namespace definition", func(t *testing.T) {
		n := mustparseChunk(t, "%mynamespace. = {}")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
			Statements: []Node{
				&PatternNamespaceDefinition{
					NodeBase: NodeBase{
						NodeSpan{0, 18},
						nil,
						[]Token{{Type: EQUAL, Span: NodeSpan{14, 15}}},
					},
					Left: &PatternNamespaceIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
						Name:     "mynamespace",
					},
					Right: &ObjectLiteral{
						NodeBase: NodeBase{
							NodeSpan{16, 18},
							nil,
							[]Token{
								{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
							},
						},
					},
				},
			},
		}, n)
	})

	t.Run("record pattern", func(t *testing.T) {

		t.Run("{ ...named-pattern } ", func(t *testing.T) {
			n := mustparseChunk(t, "%p = #{ ...%patt }")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&PatternDefinition{
						NodeBase: NodeBase{
							NodeSpan{0, 18},
							nil,
							[]Token{{Type: EQUAL, Span: NodeSpan{3, 4}}},
						},
						Left: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "p",
						},
						Right: &RecordPatternLiteral{
							NodeBase: NodeBase{
								NodeSpan{5, 18},
								nil,
								[]Token{
									{Type: OPENING_RECORD_BRACKET, Span: NodeSpan{5, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
								},
							},
							Exact: false,
							SpreadElements: []*PatternPropertySpreadElement{
								{
									NodeBase: NodeBase{
										NodeSpan{8, 16},
										nil,
										[]Token{{Type: THREE_DOTS, Span: NodeSpan{8, 11}}},
									},
									Expr: &PatternIdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{11, 16}, nil, nil},
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
			NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
			Statements: []Node{
				&PatternNamespaceMemberExpression{
					NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
					Namespace: &PatternNamespaceIdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
						Name:     "mynamespace",
					},
					MemberName: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{13, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 8}, nil, nil},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: NEWLINE, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 8}, nil, nil},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 7},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 6}, nil, nil},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &InvalidComplexStringPatternElement{
									NodeBase: NodeBase{
										NodeSpan{5, 6},
										&ParsingError{UnspecifiedParsingError, INVALID_COMPLEX_PATTERN_ELEMENT},
										nil,
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 9},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{8, 9}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase:  NodeBase{NodeSpan{5, 8}, nil, nil},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &RuneLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{11, 12}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 11},
									nil,
									[]Token{{Type: OCCURRENCE_MODIFIER, Span: NodeSpan{10, 11}, Raw: "*"}},
								},
								Ocurrence: ZeroOrMoreOcurrence,
								Expr: &ComplexStringPatternPiece{
									NodeBase: NodeBase{
										NodeSpan{5, 10},
										nil,
										[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
										},
									},
									Elements: []*PatternPieceElement{
										{
											NodeBase:  NodeBase{NodeSpan{6, 9}, nil, nil},
											Ocurrence: ExactlyOneOcurrence,
											Expr: &QuotedStringLiteral{
												NodeBase: NodeBase{NodeSpan{6, 9}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 12},
									nil,
									[]Token{{Type: OCCURRENCE_MODIFIER, Span: NodeSpan{10, 12}, Raw: "=2"}},
								},
								Ocurrence:           ExactOcurrence,
								ExactOcurrenceCount: 2,
								Expr: &ComplexStringPatternPiece{
									NodeBase: NodeBase{
										NodeSpan{5, 10},
										nil,
										[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
										},
									},
									Elements: []*PatternPieceElement{
										{
											NodeBase:  NodeBase{NodeSpan{6, 9}, nil, nil},
											Ocurrence: ExactlyOneOcurrence,
											Expr: &QuotedStringLiteral{
												NodeBase: NodeBase{NodeSpan{6, 9}, nil, nil},
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
			n := mustparseChunk(t, `%str(%s=2)`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								Ocurrence:           ExactOcurrence,
								ExactOcurrenceCount: 2,
								NodeBase: NodeBase{
									NodeSpan{5, 9},
									nil,
									[]Token{{Type: OCCURRENCE_MODIFIER, Span: NodeSpan{7, 9}, Raw: "=2"}},
								},
								Expr: &PatternIdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
									Name:     "s",
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 10},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{6, 7}}},
								},
								Ocurrence: ExactlyOneOcurrence,
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{7, 10}, nil, nil},
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

		t.Run("element name without element", func(t *testing.T) {
			n, err := parseChunk(t, `%str(l:)`, "")
			assert.Error(t, err)
			runes := []rune("%str(l:)")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 8},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{
									NodeSpan{5, 7},
									nil,
									[]Token{{Type: COLON, Span: NodeSpan{6, 7}}},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 13},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{12, 13}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
									Raw:      "\"a\"",
									Value:    "a",
								},
							},
							{
								NodeBase: NodeBase{NodeSpan{9, 12}, nil, nil},
								Expr: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{9, 12}, nil, nil},
									Raw:      "\"b\"",
									Value:    "b",
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("pattern union", func(t *testing.T) {
			n := mustparseChunk(t, `%str( (| "a" | "b" ) )`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 22}, nil, nil},
				Statements: []Node{
					&ComplexStringPatternPiece{
						NodeBase: NodeBase{
							NodeSpan{0, 22},
							nil,
							[]Token{
								{Type: PERCENT_STR, Span: NodeSpan{0, 4}},
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{4, 5}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{21, 22}},
							},
						},
						Elements: []*PatternPieceElement{
							{
								NodeBase: NodeBase{NodeSpan{6, 20}, nil, nil},
								Expr: &PatternUnion{
									NodeBase: NodeBase{
										NodeSpan{6, 20},
										nil,
										[]Token{
											{Type: OPENING_PARENTHESIS, Span: NodeSpan{6, 7}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{7, 8}},
											{Type: PATTERN_UNION_PIPE, Span: NodeSpan{13, 14}},
											{Type: CLOSING_PARENTHESIS, Span: NodeSpan{19, 20}},
										},
									},
									Cases: []Node{
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{9, 12}, nil, nil},
											Raw:      `"a"`,
											Value:    "a",
										},
										&QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{15, 18}, nil, nil},
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
	})

	t.Run("pattern call", func(t *testing.T) {
		t.Run("pattern identifier callee, no arguments", func(t *testing.T) {
			n := mustparseChunk(t, `%text()`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 7},
							Tokens: []Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{6, 7}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 11},
							Tokens: []Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{9, 10}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{10, 11}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 8},
							Tokens: []Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 10},
							Tokens: []Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: COMMA, Span: NodeSpan{7, 8}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
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

		t.Run("unexpected char in arguments", func(t *testing.T) {
			n, err := parseChunk(t, `%text(:)`, "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&PatternCallExpression{
						NodeBase: NodeBase{
							Span: NodeSpan{0, 8},
							Tokens: []Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{5, 6}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
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
									Span:   NodeSpan{6, 7},
									Err:    &ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInPatternCallArguments(':')},
									Tokens: []Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{6, 7}, Raw: string(':')}},
								},
							},
						},
					},
				},
			}, n)
		})

		t.Run("shorthand syntax for object pattern argument", func(t *testing.T) {
			n := mustparseChunk(t, `%text{}`)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
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
									Span: NodeSpan{5, 7},
									Tokens: []Token{
										{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{5, 6}},
										{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, []Token{{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{0, 2}}}},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{NodeSpan{0, 4}, nil, []Token{{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{0, 2}}}},
						Cases: []Node{
							&PatternIdentifierLiteral{
								NodeBase:   NodeBase{NodeSpan{3, 4}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{1, 7},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{1, 3}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{7, 8}},
							},
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							nil,
							[]Token{
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{0, 2}},
								{Type: PATTERN_UNION_PIPE, Span: NodeSpan{7, 8}},
							},
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{3, 6}, nil, nil},
								Raw:      `"a"`,
								Value:    "a",
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{9, 12}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&PatternUnion{
						NodeBase: NodeBase{
							NodeSpan{1, 13},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PATTERN_UNION_OPENING_PIPE, Span: NodeSpan{1, 3}},
								{Type: PATTERN_UNION_PIPE, Span: NodeSpan{8, 9}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{13, 14}},
							},
						},
						Cases: []Node{
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{4, 7}, nil, nil},
								Raw:      `"a"`,
								Value:    "a",
							},
							&QuotedStringLiteral{
								NodeBase: NodeBase{NodeSpan{10, 13}, nil, nil},
								Raw:      `"b"`,
								Value:    "b",
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
				NodeBase: NodeBase{NodeSpan{0, 11}, nil, nil},
				Statements: []Node{
					&AssertionStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 11},
							nil,
							[]Token{{Type: ASSERT_KEYWORD, Span: NodeSpan{0, 6}}},
						},
						Expr: &BooleanLiteral{
							NodeBase: NodeBase{NodeSpan{7, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&AssertionStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 6},
							nil,
							[]Token{{Type: ASSERT_KEYWORD, Span: NodeSpan{0, 6}}},
						},
						Expr: &MissingExpression{
							NodeBase: NodeBase{
								NodeSpan{5, 6},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune(code), 6, true)},
								nil,
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 12},
							&ParsingError{UnspecifiedParsingError, SYNCHRONIZED_KEYWORD_SHOULD_BE_FOLLOWED_BY_SYNC_VALUES},
							[]Token{{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}}},
						},
					},
				},
			}, n)
		})

		t.Run("single value", func(t *testing.T) {
			code := "synchronized val {}"
			n := mustparseChunk(t, code)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},
						},
						SynchronizedValues: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{13, 16}, nil, nil},
								Name:     "val",
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 19},
							nil,
							[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},
						},
						SynchronizedValues: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{
									NodeSpan{13, 16},
									nil,
									[]Token{
										{Type: OPENING_PARENTHESIS, Span: NodeSpan{12, 13}},
										{Type: CLOSING_PARENTHESIS, Span: NodeSpan{16, 17}},
									},
								},
								Name: "val",
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{17, 19},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{17, 18}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{18, 19}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},
						},
						SynchronizedValues: []Node{
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{13, 17}, nil, nil},
								Name:     "val1",
							},
							&IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{18, 22}, nil, nil},
								Name:     "val2",
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{23, 25},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{23, 24}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{24, 25}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 17}, nil, nil},
				Statements: []Node{
					&SynchronizedBlockStatement{
						NodeBase: NodeBase{
							NodeSpan{0, 17},
							nil,
							[]Token{
								{Type: SYNCHRONIZED_KEYWORD, Span: NodeSpan{0, 12}},
							},
						},
						SynchronizedValues: []Node{
							&UnknownNode{
								NodeBase: NodeBase{
									NodeSpan{13, 14},
									&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInSynchronizedValueList('?')},
									[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{13, 14}, Raw: "?"}},
								},
							},
						},
						Block: &Block{
							NodeBase: NodeBase{
								NodeSpan{15, 17},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{15, 16}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{16, 17}},
								},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
								Name:     "div",
							},
						},
					},
				},
			}, n)
		})

		t.Run("selector followed by newline", func(t *testing.T) {

			n := mustparseChunk(t, "s!div\n")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{
					NodeSpan{0, 6},
					nil,
					[]Token{{Type: NEWLINE, Span: NodeSpan{5, 6}}},
				},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 8}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
								Name:     "div",
							},
						},
					},
					&IntLiteral{
						NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
						Raw:      "1",
						Value:    1,
					},
				},
			}, n)
		})

		t.Run("single element : class selector", func(t *testing.T) {
			n := mustparseChunk(t, "s!.ab")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Elements: []Node{
							&CssClassSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Elements: []Node{
							&CssPseudoClassSelector{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Elements: []Node{
							&CssPseudoElementSelector{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Elements: []Node{
							&CssPseudoElementSelector{
								NodeBase: NodeBase{NodeSpan{2, 6}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
						Elements: []Node{
							&CssAttributeSelector{
								NodeBase: NodeBase{NodeSpan{2, 9}, nil, nil},
								AttributeName: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
									Name:     "a",
								},
								Pattern: "=",
								Value: &QuotedStringLiteral{
									NodeBase: NodeBase{NodeSpan{5, 8}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
								Name:     "a",
							},
							&CssCombinator{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
								Name:     ">",
							},
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&CssSelectorExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Elements: []Node{
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
								Name:     "a",
							},
							&CssCombinator{
								NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
								Name:     " ",
							},
							&CssTypeSelector{
								NodeBase: NodeBase{NodeSpan{4, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 10}, nil, nil},
				Statements: []Node{
					&BinaryExpression{
						NodeBase: NodeBase{
							NodeSpan{0, 10},
							nil,
							[]Token{
								{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
								{Type: PLUS, Span: NodeSpan{3, 4}},
								{Type: CLOSING_PARENTHESIS, Span: NodeSpan{9, 10}},
							},
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
							NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
							Left: &Variable{
								NodeBase: NodeBase{NodeSpan{5, 7}, nil, nil},
								Name:     "a",
							},
							PropertyName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{0, 1}},
						},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{0, 1}},
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
							{Type: SEMICOLON, Span: NodeSpan{2, 3}},
						},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
							{Type: SEMICOLON, Span: NodeSpan{3, 4}},
						},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{2, 3}},
						},
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{2, 3}, nil, nil},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{1, 2}},
						},
					},
					Statements: []Node{
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Raw:      "1",
							Value:    1,
						},
						&IntLiteral{
							NodeBase: NodeBase{NodeSpan{3, 4}, nil, nil},
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
						[]Token{
							{Type: SEMICOLON, Span: NodeSpan{2, 3}},
						},
					},
					Statements: []Node{
						&Variable{
							NodeBase: NodeBase{NodeSpan{0, 2}, nil, nil},
							Name:     "a",
						},
						&Variable{
							NodeBase: NodeBase{NodeSpan{3, 5}, nil, nil},
							Name:     "b",
						},
					},
				},
			},
			{
				"()]",
				&Chunk{
					NodeBase: NodeBase{NodeSpan{0, 3}, nil, nil},
					Statements: []Node{
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{0, 2},
								&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("()]"), 1, true)},
								[]Token{
									{Type: OPENING_PARENTHESIS, Span: NodeSpan{0, 1}},
									{Type: CLOSING_PARENTHESIS, Span: NodeSpan{1, 2}},
								},
							},
						},
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{2, 3},
								&ParsingError{UnspecifiedParsingError, fmtUnexpectedCharInBlockOrModule(']')},
								[]Token{
									{Type: UNEXPECTED_CHAR, Raw: "]", Span: NodeSpan{2, 3}},
								},
							},
						},
					},
				},
			},
			{
				".",
				&Chunk{
					NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
					Statements: []Node{
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, DOT_SHOULD_BE_FOLLOWED_BY},
								[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{0, 1}, Raw: "."}},
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
						[]Token{{Type: SEMICOLON, Span: NodeSpan{1, 2}}},
					},
					Statements: []Node{
						&UnknownNode{
							NodeBase: NodeBase{
								NodeSpan{0, 1},
								&ParsingError{UnspecifiedParsingError, AT_SYMBOL_SHOULD_BE_FOLLOWED_BY},
								[]Token{{Type: UNEXPECTED_CHAR, Span: NodeSpan{0, 1}, Raw: "@"}},
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
				NodeBase: NodeBase{NodeSpan{0, 25}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 25},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: BACKQUOTE, Span: NodeSpan{24, 25}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 30}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 30},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{9, 10}},
								{Type: BACKQUOTE, Span: NodeSpan{29, 30}},
							},
						},
						Pattern: &PatternNamespaceMemberExpression{
							NodeBase: NodeBase{NodeSpan{0, 9}, nil, nil},
							Namespace: &PatternNamespaceIdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
								Name:     "sql",
							},
							MemberName: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{5, 9}, nil, nil},
								Name:     "stmt",
							},
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{10, 29}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_TEMPL_LIT},
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, nil},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation at the start", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`{{nothing:$nothing}}SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 45}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 45},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{5, 7}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{23, 25}},
								{Type: BACKQUOTE, Span: NodeSpan{44, 45}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 5}, nil, nil},
								Raw:      "",
								Value:    "",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{7, 23}, nil, []Token{{Type: STR_TEMPLATE_INTERP_TYPE, Raw: "nothing:", Span: NodeSpan{7, 15}}}},
								Type:     "nothing",
								Expr: &Variable{
									NodeBase: NodeBase{NodeSpan{15, 23}, nil, nil},
									Name:     "nothing",
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{25, 44}, nil, nil},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation (variable) at the end", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`SELECT * from users{{nothing:$nothing}}`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 45}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 45},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{24, 26}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{42, 44}},
								{Type: BACKQUOTE, Span: NodeSpan{44, 45}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, nil},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{26, 42}, nil, []Token{{Type: STR_TEMPLATE_INTERP_TYPE, Raw: "nothing:", Span: NodeSpan{26, 34}}}},
								Type:     "nothing",
								Expr: &Variable{
									NodeBase: NodeBase{NodeSpan{34, 42}, nil, nil},
									Name:     "nothing",
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{44, 44}, nil, nil},
								Raw:      "",
								Value:    "",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation (identifier) at the end", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`SELECT * from users{{nothing:nothing}}`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 44}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 44},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{24, 26}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{41, 43}},
								{Type: BACKQUOTE, Span: NodeSpan{43, 44}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, nil},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{26, 41}, nil, []Token{{Type: STR_TEMPLATE_INTERP_TYPE, Raw: "nothing:", Span: NodeSpan{26, 34}}}},
								Type:     "nothing",
								Expr: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{34, 41}, nil, nil},
									Name:     "nothing",
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{43, 43}, nil, nil},
								Raw:      "",
								Value:    "",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation type containing a '.'", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`{{ab.cdef:1}}SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 38}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 38},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{5, 7}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{16, 18}},
								{Type: BACKQUOTE, Span: NodeSpan{37, 38}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 5}, nil, nil},
								Raw:      "",
								Value:    "",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, []Token{{Type: STR_TEMPLATE_INTERP_TYPE, Raw: "ab.cdef:", Span: NodeSpan{7, 15}}}},
								Type:     "ab.cdef",
								Expr: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{18, 37}, nil, nil},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("interpolation with expression of len 1", func(t *testing.T) {
			n := mustparseChunk(t, "%sql`{{nothing:1}}SELECT * from users`")
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 38}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 38},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{5, 7}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{16, 18}},
								{Type: BACKQUOTE, Span: NodeSpan{37, 38}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 5}, nil, nil},
								Raw:      "",
								Value:    "",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{NodeSpan{7, 16}, nil, []Token{{Type: STR_TEMPLATE_INTERP_TYPE, Raw: "nothing:", Span: NodeSpan{7, 15}}}},
								Type:     "nothing",
								Expr: &IntLiteral{
									NodeBase: NodeBase{NodeSpan{15, 16}, nil, nil},
									Raw:      "1",
									Value:    1,
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{18, 37}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 24},
							&ParsingError{UnspecifiedParsingError, UNTERMINATED_STRING_TEMPL_LIT},
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, nil},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
						},
					},
				},
			}, n)
		})

		t.Run("empty interpolation at the end", func(t *testing.T) {
			n, err := parseChunk(t, "%sql`SELECT * from users{{}}`", "")
			assert.Error(t, err)
			assert.EqualValues(t, &Chunk{
				NodeBase: NodeBase{NodeSpan{0, 29}, nil, nil},
				Statements: []Node{
					&StringTemplateLiteral{
						NodeBase: NodeBase{
							NodeSpan{0, 29},
							nil,
							[]Token{
								{Type: BACKQUOTE, Span: NodeSpan{4, 5}},
								{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{24, 26}},
								{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{26, 28}},
								{Type: BACKQUOTE, Span: NodeSpan{28, 29}},
							},
						},
						Pattern: &PatternIdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 4}, nil, nil},
							Name:     "sql",
						},
						Slices: []Node{
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{5, 24}, nil, nil},
								Raw:      "SELECT * from users",
								Value:    "SELECT * from users",
							},
							&StringTemplateInterpolation{
								NodeBase: NodeBase{
									NodeSpan{26, 26},
									&ParsingError{UnspecifiedParsingError, INVALID_STRING_INTERPOLATION_SHOULD_NOT_BE_EMPTY},
									nil,
								},
							},
							&StringTemplateSlice{
								NodeBase: NodeBase{NodeSpan{28, 28}, nil, nil},
								Raw:      "",
								Value:    "",
							},
						},
					},
				},
			}, n)

			t.Run("no pattern, interpolation at the start", func(t *testing.T) {
				n := mustparseChunk(t, "`{{$nothing}}SELECT * from users`")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 33}, nil, nil},
					Statements: []Node{
						&StringTemplateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 33},
								nil,
								[]Token{
									{Type: BACKQUOTE, Span: NodeSpan{0, 1}},
									{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{1, 3}},
									{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{11, 13}},
									{Type: BACKQUOTE, Span: NodeSpan{32, 33}},
								},
							},
							Slices: []Node{
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{1, 1}, nil, nil},
									Raw:      "",
									Value:    "",
								},
								&StringTemplateInterpolation{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, nil},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{3, 11}, nil, nil},
										Name:     "nothing",
									},
								},
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{13, 32}, nil, nil},
									Raw:      "SELECT * from users",
									Value:    "SELECT * from users",
								},
							},
						},
					},
				}, n)
			})

			t.Run("no pattern, interpolation + line feed", func(t *testing.T) {
				n := mustparseChunk(t, "`{{$nothing}}\n`")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
					Statements: []Node{
						&StringTemplateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 15},
								nil,
								[]Token{
									{Type: BACKQUOTE, Span: NodeSpan{0, 1}},
									{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{1, 3}},
									{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{11, 13}},
									{Type: BACKQUOTE, Span: NodeSpan{14, 15}},
								},
							},
							Slices: []Node{
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{1, 1}, nil, nil},
									Raw:      "",
									Value:    "",
								},
								&StringTemplateInterpolation{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, nil},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{3, 11}, nil, nil},
										Name:     "nothing",
									},
								},
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{13, 14}, nil, nil},
									Raw:      "\n",
									Value:    "\n",
								},
							},
						},
					},
				}, n)
			})

			t.Run("no pattern, interpolation + escaped n", func(t *testing.T) {
				n := mustparseChunk(t, "`{{$nothing}}\\n`")
				assert.EqualValues(t, &Chunk{
					NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
					Statements: []Node{
						&StringTemplateLiteral{
							NodeBase: NodeBase{
								NodeSpan{0, 16},
								nil,
								[]Token{
									{Type: BACKQUOTE, Span: NodeSpan{0, 1}},
									{Type: STR_INTERP_OPENING_BRACKETS, Span: NodeSpan{1, 3}},
									{Type: STR_INTERP_CLOSING_BRACKETS, Span: NodeSpan{11, 13}},
									{Type: BACKQUOTE, Span: NodeSpan{15, 16}},
								},
							},
							Slices: []Node{
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{1, 1}, nil, nil},
									Raw:      "",
									Value:    "",
								},
								&StringTemplateInterpolation{
									NodeBase: NodeBase{NodeSpan{3, 11}, nil, nil},
									Expr: &Variable{
										NodeBase: NodeBase{NodeSpan{3, 11}, nil, nil},
										Name:     "nothing",
									},
								},
								&StringTemplateSlice{
									NodeBase: NodeBase{NodeSpan{13, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 12}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 12}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{6, 12},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{6, 8}},
										{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{8, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 5}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 5}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 5},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING},
									[]Token{{Type: LESS_THAN, Span: NodeSpan{1, 2}}},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 18}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 18}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 12},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{7, 8}}},
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, nil},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{12, 12}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{12, 18},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{12, 14}},
										{Type: GREATER_THAN, Span: NodeSpan{17, 18}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{14, 17}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 13}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 13},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: SELF_CLOSING_TAG_TERMINATOR, Span: NodeSpan{11, 13}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{7, 8}}},
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 19}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 19}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 13},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{7, 8}}},
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, nil},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{13, 13}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{13, 19},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{13, 15}},
										{Type: GREATER_THAN, Span: NodeSpan{18, 19}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{15, 18}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 20}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 20}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 14},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{13, 14}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{
											NodeSpan{6, 13},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{9, 10}}},
										},
										Name: &QuotedStringLiteral{
											NodeBase: NodeBase{
												NodeSpan{6, 9},
												&ParsingError{UnspecifiedParsingError, XML_ATTRIBUTE_NAME_SHOULD_BE_IDENT},
												nil,
											},
											Raw:   `"a"`,
											Value: "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{10, 13}, nil, nil},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{14, 14}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{14, 20},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{14, 16}},
										{Type: GREATER_THAN, Span: NodeSpan{19, 20}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{16, 19}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 15}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 9},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{8, 9}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{
											NodeSpan{6, 8},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{7, 8}}},
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
											Name:     "a",
										},
										Value: &MissingExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 9},
												&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("h<div a=></div>"), 8, true)},
												nil,
											},
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 9}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{9, 15},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{9, 11}},
										{Type: GREATER_THAN, Span: NodeSpan{14, 15}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 15}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 9},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{8, 9}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{
											NodeSpan{6, 8},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{7, 8}}},
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
											Name:     "a",
										},
										Value: &MissingExpression{
											NodeBase: NodeBase{
												NodeSpan{8, 9},
												&ParsingError{UnspecifiedParsingError, fmtExprExpectedHere([]rune("h<div a=></div>"), 8, true)},
												nil,
											},
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 9}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{9, 15},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{9, 11}},
										{Type: GREATER_THAN, Span: NodeSpan{14, 15}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 14}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 8},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{7, 8}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
											Name:     "a",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{8, 8}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{8, 14},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{8, 10}},
										{Type: GREATER_THAN, Span: NodeSpan{13, 14}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 7}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 7},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_OPENING_XML_TAG_MISSING_CLOSING},
									[]Token{{Type: LESS_THAN, Span: NodeSpan{1, 2}}},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 24}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 24}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 18},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{17, 18}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
								Attributes: []*XMLAttribute{
									{
										NodeBase: NodeBase{
											NodeSpan{6, 11},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{7, 8}}},
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
											Name:     "a",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{8, 11}, nil, nil},
											Raw:      `"b"`,
											Value:    "b",
										},
									},
									{
										NodeBase: NodeBase{
											NodeSpan{12, 17},
											nil,
											[]Token{{Type: EQUAL, Span: NodeSpan{13, 14}}},
										},
										Name: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
											Name:     "c",
										},
										Value: &QuotedStringLiteral{
											NodeBase: NodeBase{NodeSpan{14, 17}, nil, nil},
											Raw:      `"d"`,
											Value:    "d",
										},
									},
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{18, 18}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{18, 24},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{18, 20}},
										{Type: GREATER_THAN, Span: NodeSpan{23, 24}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{20, 23}, nil, nil},
									Name:     "div",
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
				NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 7}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 7}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 7},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: SELF_CLOSING_TAG_TERMINATOR, Span: NodeSpan{5, 7}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 6}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 6}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									&ParsingError{UnspecifiedParsingError, UNTERMINATED_SELF_CLOSING_XML_TAG_MISSING_CLOSING},
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: SLASH, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 13}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Raw:      " ",
									Value:    " ",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{7, 13},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{7, 9}},
										{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 12}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 13}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{NodeSpan{1, 13}, nil, nil},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Raw:      "\n",
									Value:    "\n",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{7, 13},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{7, 9}},
										{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{9, 12}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
									Expr: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
										Raw:      "1",
										Value:    1,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 10}, nil, nil},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
									Raw:      "1",
									Value:    "1",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
									Expr: &IntLiteral{
										NodeBase: NodeBase{NodeSpan{8, 9}, nil, nil},
										Raw:      "2",
										Value:    2,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, nil},
									Name:     "div",
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
				NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 27},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{19, 20}},
								},
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{NodeSpan{7, 19}, nil, nil},
									Expr: &XMLExpression{
										NodeBase: NodeBase{NodeSpan{7, 19}, nil, nil},
										Namespace: &IdentifierLiteral{
											NodeBase: NodeBase{NodeSpan{7, 8}, nil, nil},
											Name:     "h",
										},
										Element: &XMLElement{
											NodeBase: NodeBase{NodeSpan{8, 19}, nil, nil},
											Opening: &XMLOpeningElement{
												NodeBase: NodeBase{
													NodeSpan{8, 13},
													nil,
													[]Token{
														{Type: LESS_THAN, Span: NodeSpan{8, 9}},
														{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
													},
												},
												Name: &IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{9, 12}, nil, nil},
													Name:     "div",
												},
											},
											Children: []Node{
												&XMLText{
													NodeBase: NodeBase{NodeSpan{13, 13}, nil, nil},
													Raw:      "",
													Value:    "",
												},
											},
											Closing: &XMLClosingElement{
												NodeBase: NodeBase{
													NodeSpan{13, 19},
													nil,
													[]Token{
														{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{13, 15}},
														{Type: GREATER_THAN, Span: NodeSpan{18, 19}},
													},
												},
												Name: &IdentifierLiteral{
													NodeBase: NodeBase{NodeSpan{15, 18}, nil, nil},
													Name:     "div",
												},
											},
										},
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{20, 21}, nil, nil},
									Raw:      "2",
									Value:    "2",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{21, 27},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{21, 23}},
										{Type: GREATER_THAN, Span: NodeSpan{26, 27}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{23, 26}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 14}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 14},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{7, 8}},
								},
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{
										NodeSpan{7, 7},
										&ParsingError{UnspecifiedParsingError, EMPTY_XML_INTERP},
										nil,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{8, 8}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{8, 14},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{8, 10}},
										{Type: GREATER_THAN, Span: NodeSpan{13, 14}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{10, 13}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 15}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 15},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{8, 9}},
								},
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
									Raw:      "",
									Value:    "",
								},
								&XMLInterpolation{
									NodeBase: NodeBase{
										NodeSpan{7, 8},
										&ParsingError{UnspecifiedParsingError, EMPTY_XML_INTERP},
										nil,
									},
								},
								&XMLText{
									NodeBase: NodeBase{NodeSpan{9, 9}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{9, 15},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{9, 11}},
										{Type: GREATER_THAN, Span: NodeSpan{14, 15}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{11, 14}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: NEWLINE, Span: NodeSpan{7, 8}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
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
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, nil},
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
				NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
				Statements: []Node{
					&XMLExpression{
						NodeBase: NodeBase{NodeSpan{0, 16}, nil, nil},
						Namespace: &IdentifierLiteral{
							NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
							Name:     "h",
						},
						Element: &XMLElement{
							NodeBase: NodeBase{
								NodeSpan{1, 16},
								nil,
								[]Token{
									{Type: OPENING_CURLY_BRACKET, Span: NodeSpan{6, 7}},
									{Type: NEWLINE, Span: NodeSpan{8, 9}},
									{Type: CLOSING_CURLY_BRACKET, Span: NodeSpan{9, 10}},
								},
							},
							Opening: &XMLOpeningElement{
								NodeBase: NodeBase{
									NodeSpan{1, 6},
									nil,
									[]Token{
										{Type: LESS_THAN, Span: NodeSpan{1, 2}},
										{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
									Name:     "div",
								},
							},
							Children: []Node{
								&XMLText{
									NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
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
									NodeBase: NodeBase{NodeSpan{10, 10}, nil, nil},
									Raw:      "",
									Value:    "",
								},
							},
							Closing: &XMLClosingElement{
								NodeBase: NodeBase{
									NodeSpan{10, 16},
									nil,
									[]Token{
										{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{10, 12}},
										{Type: GREATER_THAN, Span: NodeSpan{15, 16}},
									},
								},
								Name: &IdentifierLiteral{
									NodeBase: NodeBase{NodeSpan{12, 15}, nil, nil},
									Name:     "div",
								},
							},
						},
					},
				},
			}, n)
		})
	})

	t.Run("leading child element", func(t *testing.T) {
		n := mustparseChunk(t, "h<div><span>1</span>2</div>")
		assert.EqualValues(t, &Chunk{
			NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
			Statements: []Node{
				&XMLExpression{
					NodeBase: NodeBase{NodeSpan{0, 27}, nil, nil},
					Namespace: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Name:     "h",
					},
					Element: &XMLElement{
						NodeBase: NodeBase{NodeSpan{1, 27}, nil, nil},
						Opening: &XMLOpeningElement{
							NodeBase: NodeBase{
								NodeSpan{1, 6},
								nil,
								[]Token{
									{Type: LESS_THAN, Span: NodeSpan{1, 2}},
									{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
								},
							},
							Name: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
								Name:     "div",
							},
						},
						Children: []Node{
							&XMLText{
								NodeBase: NodeBase{NodeSpan{6, 6}, nil, nil},
								Raw:      "",
								Value:    "",
							},
							&XMLElement{
								NodeBase: NodeBase{NodeSpan{6, 20}, nil, nil},
								Opening: &XMLOpeningElement{
									NodeBase: NodeBase{
										NodeSpan{6, 12},
										nil,
										[]Token{
											{Type: LESS_THAN, Span: NodeSpan{6, 7}},
											{Type: GREATER_THAN, Span: NodeSpan{11, 12}},
										},
									},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{7, 11}, nil, nil},
										Name:     "span",
									},
								},
								Children: []Node{
									&XMLText{
										NodeBase: NodeBase{NodeSpan{12, 13}, nil, nil},
										Raw:      "1",
										Value:    "1",
									},
								},
								Closing: &XMLClosingElement{
									NodeBase: NodeBase{
										NodeSpan{13, 20},
										nil,
										[]Token{
											{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{13, 15}},
											{Type: GREATER_THAN, Span: NodeSpan{19, 20}},
										},
									},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{15, 19}, nil, nil},
										Name:     "span",
									},
								},
							},
							&XMLText{
								NodeBase: NodeBase{NodeSpan{20, 21}, nil, nil},
								Raw:      "2",
								Value:    "2",
							},
						},
						Closing: &XMLClosingElement{
							NodeBase: NodeBase{
								NodeSpan{21, 27},
								nil,
								[]Token{
									{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{21, 23}},
									{Type: GREATER_THAN, Span: NodeSpan{26, 27}},
								},
							},
							Name: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{23, 26}, nil, nil},
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
			NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
			Statements: []Node{
				&XMLExpression{
					NodeBase: NodeBase{NodeSpan{0, 28}, nil, nil},
					Namespace: &IdentifierLiteral{
						NodeBase: NodeBase{NodeSpan{0, 1}, nil, nil},
						Name:     "h",
					},
					Element: &XMLElement{
						NodeBase: NodeBase{NodeSpan{1, 28}, nil, nil},
						Opening: &XMLOpeningElement{
							NodeBase: NodeBase{
								NodeSpan{1, 6},
								nil,
								[]Token{
									{Type: LESS_THAN, Span: NodeSpan{1, 2}},
									{Type: GREATER_THAN, Span: NodeSpan{5, 6}},
								},
							},
							Name: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{2, 5}, nil, nil},
								Name:     "div",
							},
						},
						Children: []Node{
							&XMLText{
								NodeBase: NodeBase{NodeSpan{6, 7}, nil, nil},
								Raw:      "\n",
								Value:    "\n",
							},
							&XMLElement{
								NodeBase: NodeBase{NodeSpan{7, 21}, nil, nil},
								Opening: &XMLOpeningElement{
									NodeBase: NodeBase{
										NodeSpan{7, 13},
										nil,
										[]Token{
											{Type: LESS_THAN, Span: NodeSpan{7, 8}},
											{Type: GREATER_THAN, Span: NodeSpan{12, 13}},
										},
									},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{8, 12}, nil, nil},
										Name:     "span",
									},
								},
								Children: []Node{
									&XMLText{
										NodeBase: NodeBase{NodeSpan{13, 14}, nil, nil},
										Raw:      "1",
										Value:    "1",
									},
								},
								Closing: &XMLClosingElement{
									NodeBase: NodeBase{
										NodeSpan{14, 21},
										nil,
										[]Token{
											{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{14, 16}},
											{Type: GREATER_THAN, Span: NodeSpan{20, 21}},
										},
									},
									Name: &IdentifierLiteral{
										NodeBase: NodeBase{NodeSpan{16, 20}, nil, nil},
										Name:     "span",
									},
								},
							},
							&XMLText{
								NodeBase: NodeBase{NodeSpan{21, 22}, nil, nil},
								Raw:      "2",
								Value:    "2",
							},
						},
						Closing: &XMLClosingElement{
							NodeBase: NodeBase{
								NodeSpan{22, 28},
								nil,
								[]Token{
									{Type: END_TAG_OPEN_DELIMITER, Span: NodeSpan{22, 24}},
									{Type: GREATER_THAN, Span: NodeSpan{27, 28}},
								},
							},
							Name: &IdentifierLiteral{
								NodeBase: NodeBase{NodeSpan{24, 27}, nil, nil},
								Name:     "div",
							},
						},
					},
				},
			},
		}, n)
	})
}

func TestParsePath(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		p, ok := ParsePath("")
		assert.False(t, ok)
		assert.Empty(t, p)
	})
}
