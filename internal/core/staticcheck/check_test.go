package staticcheck_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/core/text"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/inoxlang/inox/internal/utils/fsutils"

	"github.com/stretchr/testify/assert"
)

func TestCheck(t *testing.T) {
	// {
	// 	runtime.GC()
	// 	startMemStats := new(runtime.MemStats)
	// 	runtime.ReadMemStats(startMemStats)

	// 	defer utils.AssertNoMemoryLeak(t, startMemStats, 120_000)
	// }

	mustParseCode := func(code string) (*ast.Chunk, *parse.ParsedChunkSource) {
		chunk := utils.Must(parse.ParseChunkSource(sourcecode.InMemorySource{
			NameString: "test",
			CodeString: code,
		}))

		return chunk.Node, chunk
	}

	parseCode := func(code string) (*ast.Chunk, *parse.ParsedChunkSource, error) {
		chunk, err := parse.ParseChunkSource(sourcecode.InMemorySource{
			NameString: "test",
			CodeString: code,
		})

		if chunk == nil {
			return nil, nil, err
		}
		return chunk.Node, chunk, err
	}

	makeError := func(node ast.Node, chunk *parse.ParsedChunkSource, s string) *StaticCheckError {
		return NewStaticCheckError(s, sourcecode.SourcePositionStack{chunk.GetSourcePosition(node.Base().Span)})
	}

	staticCheckNoData := func(input StaticCheckInput) error {
		ctx := NewContext(ContextConfig{})
		defer ctx.CancelGracefully()

		if input.State == nil {
			input.State = NewGlobalState(ctx)
		}
		_, err := StaticCheck(input)
		return err
	}

	t.Run("global constant declarations in modules", func(t *testing.T) {
		t.Run("methods of global constants are allowed to be called", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					PATH_A = /a/
					PATH_B = PATH_A.join(./b)
				)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("some node types are not allowed", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					A = go do {}
				)
			`)
			intLit := ast.FindNode(n, (*ast.SpawnExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(intLit, src, text.FmtFollowingNodeTypeNotAllowedInGlobalConstantDeclarations((*ast.SpawnExpression)(nil))),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("global constant declarations in includable files", func(t *testing.T) {
		t.Run("methods are not allowed to be called", func(t *testing.T) {
			n, src := mustParseCode(`
				includable-file

				const (
					PATH_A = /a/
					PATH_B = PATH_A.join(./b)
				)
			`)
			callExpr := ast.FindNode(n, (*ast.CallExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(callExpr, src, text.CALL_EXPRS_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS_OF_INCLUDABLE_FILES),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("functions are not allowed to be called", func(t *testing.T) {
			n, src := mustParseCode(`
				includable-file

				const (
					A = f()
				)
			`)
			callExpr := ast.FindNode(n, (*ast.CallExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				Globals: GlobalVariablesFromMap(map[string]Value{
					"f": WrapGoFunction(func(*Context) {}),
				}, []string{"f"}),
			})
			expectedErr := utils.CombineErrors(
				makeError(callExpr, src, text.CALL_EXPRS_NOT_ALLOWED_INSIDE_GLOBAL_CONST_DECLS_OF_INCLUDABLE_FILES),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("object literal", func(t *testing.T) {
		t.Run("two elements", func(t *testing.T) {
			n, src := mustParseCode(`{1, 2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("explicit empty property name + elements", func(t *testing.T) {
			n, src := mustParseCode(`{"": 'a', 1}`)
			intLit := ast.FindNode(n, (*ast.IntLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(intLit, src, text.ELEMENTS_NOT_ALLOWED_IF_EMPTY_PROP_NAME),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("elements + explicit empty property name", func(t *testing.T) {
			n, src := mustParseCode(`{1, "": 'a'}`)
			strLit := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(strLit, src, text.EMPTY_PROP_NAME_NOT_ALLOWED_IF_ELEMENTS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("identifier keys", func(t *testing.T) {
			n, src := mustParseCode(`{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys (two string literals)", func(t *testing.T) {
			n, src := mustParseCode(`{"0":1, "0": 1}`)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys (one identifier & one string)", func(t *testing.T) {
			n, src := mustParseCode(`{a:1, "a": 1}`)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys (one string & one identifier)", func(t *testing.T) {
			n, src := mustParseCode(`{a:1, "a": 1}`)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys (two identifiers)", func(t *testing.T) {
			n, src := mustParseCode(`{a:1, "a": 1}`)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys : one of the key is in an expanded object", func(t *testing.T) {
			n, src := mustParseCode(`
				e = {a: 1}
				{"a": 1, ... $e.{a}}
			`)
			keyNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("invalid spread element", func(t *testing.T) {
			chunk, err := parse.ParseChunkSource(sourcecode.InMemorySource{
				NameString: "test",
				CodeString: `
					e = {a: 1}
					{"a": 1, ... $e}
				`,
			})

			if !assert.Error(t, err) {
				return
			}

			err = staticCheckNoData(StaticCheckInput{Node: chunk.Node, Chunk: chunk})
			assert.NoError(t, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := strings.Replace(`{"a":1}`, "a", name, 1)
			n, src := mustParseCode(code)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("regular property having a metaproperty key", func(t *testing.T) {
			n, src := mustParseCode(`{_url_: https://example.com/}`)
			keyNode := ast.FindNode(n, (*ast.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty initialization : undefined variable in block", func(t *testing.T) {
			n, src := mustParseCode(`{ _url_ {a} }`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty initialization : local variables in the scope surrounding the object are not accessible from the block", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1 
				{ _url_ {a} }
			`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: missing description", func(t *testing.T) {
			n, src := mustParseCode(`{ _visibility_ {} }`)
			init := ast.FindNode(n, (*ast.InitializationBlock)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(init, src, text.INVALID_VISIB_INIT_BLOCK_SHOULD_CONT_OBJ),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: description should not have metaproperties", func(t *testing.T) {
			n, src := mustParseCode(`{ _visibility_ { { _url_ {} } } }`)
			innerObj := ast.FindNodes(n, (*ast.ObjectLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(innerObj, src, text.INVALID_VISIB_DESC_SHOULDNT_HAVE_METAPROPS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: description should not have elements (values without a key)", func(t *testing.T) {
			n, src := mustParseCode(`{ _visibility_ { {1} } }`)
			innerObj := ast.FindNodes(n, (*ast.ObjectLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(innerObj, src, text.INVALID_VISIB_DESC_SHOULDNT_HAVE_ELEMENTS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: description should not have have invalid keys", func(t *testing.T) {
			n, src := mustParseCode(`{ _visibility_ { {a: 1} } }`)
			prop := ast.FindNode(n, (*ast.ObjectProperty)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(prop, src, text.INVALID_VISIBILITY_DESC_KEY),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: .public should have a key list literal as value", func(t *testing.T) {
			n, src := mustParseCode(`{ _visibility_ { {public: 1} } }`)
			publicProp := ast.FindNode(n, (*ast.ObjectProperty)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(publicProp, src, text.VAL_SHOULD_BE_KEYLIST_LIT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: .visible_by should have a dict literal as value", func(t *testing.T) {
			n, src := mustParseCode(`{ _visibility_ { {visible_by: 1} } }`)
			publicProp := ast.FindNode(n, (*ast.ObjectProperty)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(publicProp, src, text.VAL_SHOULD_BE_DICT_LIT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: .visible_by[#self] should have a ket list literal as value", func(t *testing.T) {
			n, src := mustParseCode(`{ 
				_visibility_ { 
					{visible_by: :{#self: 1} } 
				} 
			}`)
			dictEntry := ast.FindNode(n, (*ast.DictionaryEntry)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(dictEntry, src, text.VAL_SHOULD_BE_KEYLIST_LIT),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("record literal", func(t *testing.T) {

		t.Run("identifier keys", func(t *testing.T) {
			n, src := mustParseCode(`#{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("two elements", func(t *testing.T) {
			n, src := mustParseCode(`#{1, 2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := mustParseCode(`#{"0":1, "0": 1}`)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys : one of the key is in an expanded object", func(t *testing.T) {
			n, src := mustParseCode(`
				e = {a: 1}
				#{"a": 1, ... $e.{a}}
			`)
			keyNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("invalid spread element", func(t *testing.T) {
			chunk, err := parse.ParseChunkSource(sourcecode.InMemorySource{
				NameString: "test",
				CodeString: `
					e = #{a: 1}
					#{"a": 1, ... $e}
				`,
			})

			if !assert.Error(t, err) {
				return
			}

			err = staticCheckNoData(StaticCheckInput{Node: chunk.Node, Chunk: chunk})
			assert.NoError(t, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := strings.Replace(`#{"a":1}`, "a", name, 1)
			n, src := mustParseCode(code)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty key", func(t *testing.T) {
			n, src := mustParseCode(`#{_url_: https://example.com/}`)
			keyNode := ast.FindNode(n, (*ast.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("object pattern literal", func(t *testing.T) {
		t.Run("identifier keys", func(t *testing.T) {
			n, src := mustParseCode(`%{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := mustParseCode(`%{"0":1, "0": 1}`)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := mustParseCode(`pattern p = %{a: 1}; %{...(%p).{a}, a:1}`)

			keyNodes := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), func(l *ast.IdentifierLiteral) bool {
				return l.Name == "a"
			})
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNodes[2], src, text.FmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := strings.Replace(`%{"a":1}`, "a", name, 1)
			n, src := mustParseCode(code)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty key", func(t *testing.T) {
			n, src := mustParseCode(`%{_url_: https://example.com/}`)
			keyNode := ast.FindNode(n, (*ast.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("unexpected otherprops expression", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern one = 1
				%{
					otherprops(no) 
					otherprops(one)
				}
			`)

			secondOtherPropsExpr := ast.FindNodes(n, (*ast.OtherPropsExpr)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(secondOtherPropsExpr, src, text.UNEXPECTED_OTHER_PROPS_EXPR_OTHERPROPS_NO_IS_PRESENT),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("record pattern literal", func(t *testing.T) {
		t.Run("identifier keys", func(t *testing.T) {
			n, src := mustParseCode(`pattern p = #{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := mustParseCode(`pattern p = #{"0":1, "0": 1}`)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := mustParseCode(`pattern p = %{a: 1}; pattern e = #{...(%p).{a}, a:1}`)

			keyNodes := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), func(l *ast.IdentifierLiteral) bool {
				return l.Name == "a"
			})
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNodes[2], src, text.FmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := `pattern p = ` + strings.Replace(`#{"a":1}`, "a", name, 1)
			n, src := mustParseCode(code)

			keyNode := ast.FindNode(n, (*ast.DoubleQuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty key", func(t *testing.T) {
			n, src := mustParseCode(`pattern p = #{_url_: https://example.com/}`)
			keyNode := ast.FindNode(n, (*ast.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("self expression", func(t *testing.T) {
		t.Run("is not defined in the top level", func(t *testing.T) {
			n, src := mustParseCode(`self`)

			selfExpr := ast.FindNode(n, (*ast.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is not defined if initial value of an object property", func(t *testing.T) {
			n, src := mustParseCode(`{a: self}`)

			selfExpr := ast.FindNode(n, (*ast.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is not defined in a function", func(t *testing.T) {
			n, src := mustParseCode(`fn() => self`)

			selfExpr := ast.FindNode(n, (*ast.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is not defined in a function that is the initial value of an object property", func(t *testing.T) {
			n, src := mustParseCode(`{f: fn() => self}`)
			selfExpr := ast.FindNode(n, (*ast.SelfExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is defined in a metaproperty's initialization block", func(t *testing.T) {
			n, src := mustParseCode(`{ _url_ { self } }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("is defined in a member expression in an extension method", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern o = {
					a: 1
				}
				extend o {
					f: fn() => self.a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("is not defined in a function that is a value of an object pattern", func(t *testing.T) {
			n, src := mustParseCode(`%{f: %(fn() => self)}`)

			selfExpr := ast.FindNode(n, (*ast.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is defined in reception handlers", func(t *testing.T) {
			n, src := mustParseCode(`
				{
					on received %{} fn(event){
						self
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("is not defined at the top level of an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`go do { self }`)

			selfExpr := ast.FindNode(n, (*ast.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is defined in the expression of an extension object's property", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = {
					a: 1
				}
			
				extend p {
					SELF_: (1 + self.a)
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

	})

	t.Run("sendval expression", func(t *testing.T) {
		t.Run("is not allowed at the top level", func(t *testing.T) {
			n, src := mustParseCode(`sendval 1 to {}`)

			sendValExpr := ast.FindNode(n, (*ast.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(sendValExpr, src, text.MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is not allowed inside the initial value expression of an object property", func(t *testing.T) {
			n, src := mustParseCode(`{a: sendval 1 to {}}`)

			sendValExpr := ast.FindNode(n, (*ast.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(sendValExpr, src, text.MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is not allowed in a function", func(t *testing.T) {
			n, src := mustParseCode(`fn() => sendval 1 to {}`)

			sendValExpr := ast.FindNode(n, (*ast.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(sendValExpr, src, text.MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is not allowed in a function that is the value of an object property", func(t *testing.T) {
			n, src := mustParseCode(`{f: fn() => sendval 1 to {}}`)

			sendValExpr := ast.FindNode(n, (*ast.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(sendValExpr, src, text.MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is allowed in an extension method", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern user = {}
				extend user { 
					send: fn(){ sendval 1 to { } } 
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("is allowed in a metaproperty's initialization block", func(t *testing.T) {
			n, src := mustParseCode(`{ _url_ { sendval 1 to {} } }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("is not allowed in a function that is a property value of an object pattern", func(t *testing.T) {
			n, src := mustParseCode(`%{f: %(fn() => sendval 1 to {})}`)

			sendValExpr := ast.FindNode(n, (*ast.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(sendValExpr, src, text.MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("is not allowed at at the top level of an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`go do { sendval 1 to {} }`)

			sendValExpr := ast.FindNode(n, (*ast.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(sendValExpr, src, text.MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("member expression", func(t *testing.T) {
		t.Run("existing property of self", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern obj = {a: 1}
				extend obj {
					f: fn() => self.a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		// t.Run("existing property of self due to a spread object", func(t *testing.T) {
		// 	n, src := mustParseCode(`{
		// 		f: fn() => self.name,
		// 		...({name: "foo"}).{name}
		// 	}`)
		// 	assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		// })

		t.Run("non existing property of self", func(t *testing.T) {
			n, src := mustParseCode(`{f: fn() => self.b}`)

			selfExpr := ast.FindNode(n, (*ast.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("computed member expression", func(t *testing.T) {
		t.Run("property name node is an undefined variable", func(t *testing.T) {
			n, src := mustParseCode(`
				a = {}
				a.(b)
			`)

			ident := ast.FindIdentWithName(n, "b")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.FmtVarIsNotDeclared("b")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("property name node is a defined variable", func(t *testing.T) {
			n, src := mustParseCode(`
				a = {}
				b = "a"
				a.(b)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("double-colon expression", func(t *testing.T) {
		t.Run("", func(t *testing.T) {
			n, src := mustParseCode(`a = 1; a::b`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("tuple literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, src := mustParseCode(`#[]`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
		t.Run("single & valid element", func(t *testing.T) {
			n, src := mustParseCode(`#[1]`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

	})

	t.Run("dictionary literal", func(t *testing.T) {
		t.Run("duplicate keys", func(t *testing.T) {
			n, src := mustParseCode(`:{./a: 0, ./a: 1}`)

			keyNode := ast.FindNodes(n, (*ast.RelativePathLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyNode, src, text.FmtDuplicateDictKey("./a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("parsing error in key: key is a simple value literal", func(t *testing.T) {
			n, src, err := parseCode(`:{'a`)
			if !assert.Error(t, err) {
				return
			}
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("parsing error in key: key is not a simple value literal", func(t *testing.T) {
			n, src, err := parseCode(`:{.`)
			if !assert.Error(t, err) {
				return
			}

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			n, src, err = parseCode(`:{.}`)
			if !assert.Error(t, err) {
				return
			}

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

	})

	t.Run("spawn expression", func(t *testing.T) {
		t.Run("single call expression: user declared function", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}
				go {} do f()
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("single call expression: identifier member expr: namespace method", func(t *testing.T) {
			n, src := mustParseCode(`
				go {} do http.read(https://example.com/)
			`)

			input := StaticCheckInput{Node: n, Chunk: src, Globals: GlobalVariablesFromMap(map[string]Value{
				"http": NewNamespace("http", map[string]Value{
					"read": WrapGoFunction(func(*Context, URL) String {
						return ""
					}),
				}),
			}, nil)}

			assert.NoError(t, staticCheckNoData(input))
		})

		t.Run("single call expression: embedded module should inherit start constants", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(arg){ return arg }
				go {} do f(myconst)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				Globals: GlobalVariablesFromMap(map[string]Value{
					"myconst": Int(1),
				}, nil),
			}))
		})

		t.Run("single call expression: embedded module should not inherit explictly defined global constants", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					myconst = 1
				)
				fn f(arg){ return arg }
				go {} do f(myconst)
			`)

			ident := ast.FindIdentWithName(n, "myconst")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.FmtVarIsNotDeclared("myconst")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single call expression: embedded module should not inherit global variables", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar myglobal = 1
				fn f(arg){ return arg }
				go {} do f(myglobal)
			`)

			ident := ast.FindIdentWithName(n, "myglobal")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.FmtVarIsNotDeclared("myglobal")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single call expression: embedded module should not inherit local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				var mylocal = 1
				fn f(arg){ return arg }
				go {} do f(mylocal)
			`)

			ident := ast.FindIdentWithName(n, "mylocal")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.FmtVarIsNotDeclared("mylocal")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("no additional provided globals (single call expression)", func(t *testing.T) {
			n, src := mustParseCode(`go {} do idt(a)`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				Globals: GlobalVariablesFromMap(map[string]Value{
					"a": Int(1),
					"idt": WrapGoFunction(func(ctx *Context, arg Value) Value {
						return Int(2)
					}),
				}, []string{"a"}),
			}))
		})

		t.Run("meta should be an object", func(t *testing.T) {
			n, src := mustParseCode(`go true do {
				return 1
			}`)

			boolLit := ast.FindNode(n, (*ast.BooleanLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(boolLit, src, text.INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("meta should be an object with no spread elements", func(t *testing.T) {
			n, src := mustParseCode(`obj = {a: 1}; go {...$obj.{a}} do {
				return 1
			}`)

			objLits := ast.FindNodes(n, (*ast.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(objLits[1], src, text.INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("meta should be an object with no implicit-key properties", func(t *testing.T) {
			n, src := mustParseCode(`go {1} do {
				return 1
			}`)

			objLit := ast.FindNode(n, (*ast.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(objLit, src, text.INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("no additional provided globals", func(t *testing.T) {
			n, src := mustParseCode(`go {} do {
				return a
			}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				Globals: GlobalVariablesFromMap(map[string]Value{
					"a": Int(1),
				}, []string{"a"}),
			}))
		})

		t.Run("additional globals provided with an object literal", func(t *testing.T) {
			n, src := mustParseCode(`
				global = 0
				go {globals: {global: global}} do {
					return global
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("description of globals should not contain spread elements", func(t *testing.T) {
			n, src := mustParseCode(`
				obj = {a: 1}
				global = 0
				go {globals: {global: global, ...$obj.{a}}} do {
					return global
				}
			`)
			objLit := ast.FindNode(n, (*ast.ObjectLiteral)(nil), func(lit *ast.ObjectLiteral, _ bool, _ []ast.Node) bool {
				return len(lit.SpreadElements) > 0
			})

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(objLit, src, text.INVALID_SPAWN_GLOBALS_SHOULD_BE),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("description of globals should not contain implicit-key properties", func(t *testing.T) {
			n, src := mustParseCode(`
				global = 0
				go {globals: {global: global, 1}} do {
					return global
				}
			`)
			objLit := ast.FindNode(n, (*ast.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(objLit, src, text.INVALID_SPAWN_GLOBALS_SHOULD_BE),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("global key list contains the name of a undefined global", func(t *testing.T) {
			n, src := mustParseCode(`
				go {globals: .{global}} do {
					return global
				}
			`)
			keyList := ast.FindNode(n, (*ast.KeyListExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyList, src, text.FmtCannotPassGlobalThatIsNotDeclaredToLThread("global")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("mapping expression", func(t *testing.T) {
		t.Run("valid static entry", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { 0 => 1 }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("static entry with invalid key", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { ({}) => 1 }`)

			obj := ast.FindNode(n, (*ast.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(obj, src, text.INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("static entry with pattern identifier key ", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { %int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("static entry with pattern namespace member key ", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { %ns.int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				PatternNamespaces: map[string][]string{
					"ns": {"int"},
				},
			}))
		})

		t.Run("static key entries have access to globals", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar g = 1
				Mapping { %int => g }
			`)

			data, err := StaticCheck(StaticCheckInput{
				State:    NewGlobalState(ctx),
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			})

			assert.NoError(t, err)

			mappingExpr := ast.FindNode(n, (*ast.MappingExpression)(nil), nil)
			assert.Equal(t, NewMappingStaticData([]string{"g"}), data.GetMappingData(mappingExpr))
		})

		t.Run("static key entries don't have access to locals", func(t *testing.T) {
			n, src := mustParseCode(`
				loc = 1
				Mapping { 0 => loc }
			`)

			ident := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.FmtVarIsNotDeclared("loc")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("dynamic entry returning its key", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { n 0 => n }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("dynamic entry returning its key and group matching result", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { p %/{:name} m => [p, m] }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("dynamic entry with pattern identifier key ", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { n %int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("dynamic entry with pattern namespace member key ", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { n %ns.int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				PatternNamespaces: map[string][]string{
					"ns": {"int"},
				},
			}))
		})

		t.Run("dynamic key entries have access to globals", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar g = 1
				Mapping { n %int => g }
			`)

			data, err := StaticCheck(StaticCheckInput{
				State:    NewGlobalState(ctx),
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			})

			assert.NoError(t, err)

			mappingExpr := ast.FindNode(n, (*ast.MappingExpression)(nil), nil)
			assert.Equal(t, NewMappingStaticData([]string{"g"}), data.GetMappingData(mappingExpr))
		})
	})
	t.Run("compute expression", func(t *testing.T) {
		t.Run("in right side of dynamic mapping entry", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { n 0 => comp 1 }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in right side of static mapping entry", func(t *testing.T) {
			n, src := mustParseCode(`Mapping { 0 => comp 1 }`)

			computeExpr := ast.FindNode(n, (*ast.ComputeExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(computeExpr, src, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("top level", func(t *testing.T) {
			n, src := mustParseCode(`comp 1`)

			computeExpr := ast.FindNode(n, (*ast.ComputeExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(computeExpr, src, text.MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("captured variable does not exist", func(t *testing.T) {
			n, src := mustParseCode(`
				fn[a](){

				}
			`)
			fnExprNode := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExprNode.CaptureList[0], src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable is not a local", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				fn[a](){}
			`)
			fnExprNode := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExprNode.CaptureList[0], src, text.FmtCannotPassGlobalToFunction("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate variable capture", func(t *testing.T) {
			n, src := mustParseCode(`
				var a = 1
				fn[a, a](){}
			`)
			fnExprNode := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExprNode.CaptureList[1], src, text.FmtVarIsAlreadyCaptured("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable should be accessible in body", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				fn[a](){ return a }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("invalid element in capture list", func(t *testing.T) {
			n, src, err := parseCode(`
				fn[1](){

				}
			`)
			assert.Error(t, err)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("globals captured by function should be listed", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar a = 1
				fn(){ return a }
			`)

			fnExpr := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			data, err := StaticCheck(StaticCheckInput{
				State: NewGlobalState(ctx),
				Node:  n,
				Chunk: src,
			})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, NewFunctionStaticData([]string{"a"}), data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function B referenced by a function A should be listed in A's data", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar a = 1
				fn f(){
					return a
				}
				fn(){ return f }
			`)

			fnExpr := ast.FindNodes(n, (*ast.FunctionExpression)(nil), nil)[1]
			data, err := StaticCheck(StaticCheckInput{
				State: NewGlobalState(ctx),
				Node:  n,
				Chunk: src,
			})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, NewFunctionStaticData([]string{"f", "a"}), data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function C referenced by a function B referenced by a function A should be listed in A's data", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar a = 1
				fn g(){
					return a
				}
				fn f(){
					return g
				}
				fn(){ return f }
			`)

			fnExpr := ast.FindNodes(n, (*ast.FunctionExpression)(nil), nil)[2]
			data, err := StaticCheck(StaticCheckInput{
				State: NewGlobalState(ctx),
				Node:  n,
				Chunk: src,
			})
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewFunctionStaticData([]string{"f", "g", "a"}), data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function B referenced by a method A should be listed in A's data", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar a = 1
				fn f(){
					return a
				}
				{
					m: fn(){ return f }
				}
			`)

			fnExpr := ast.FindNodes(n, (*ast.FunctionExpression)(nil), nil)[1]
			data, err := StaticCheck(StaticCheckInput{
				State: NewGlobalState(ctx),
				Node:  n,
				Chunk: src,
			})
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewFunctionStaticData([]string{"f", "a"}), data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function C referenced by a function B referenced by a method A should be listed in A's data", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar a = 1
				fn g(){
					return a
				}
				fn f(){
					return g
				}
				{
					m: fn(){ return f }
				}
			`)

			fnExpr := ast.FindNodes(n, (*ast.FunctionExpression)(nil), nil)[2]
			data, err := StaticCheck(StaticCheckInput{
				State: NewGlobalState(ctx),
				Node:  n,
				Chunk: src,
			})
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewFunctionStaticData([]string{"f", "g", "a"}), data.GetFnData(fnExpr))
		})

		t.Run("globals captured by function defined in spawn expression should be listed", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			n, src := mustParseCode(`
				globalvar a = 1

				go do {
					globalvar b = 1
					fn(){ return b }
				}
			`)

			fnExpr := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			data, err := StaticCheck(StaticCheckInput{
				State: NewGlobalState(ctx),
				Node:  n,
				Chunk: src,
			})
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewFunctionStaticData([]string{"b"}), data.GetFnData(fnExpr))
		})

	})

	t.Run("function declaration", func(t *testing.T) {

		t.Run("captured local variable does not exist", func(t *testing.T) {
			n, src := mustParseCode(`
				fn[a] f(){}
			`)
			fnDecl := ast.FindNode(n, (*ast.FunctionDeclaration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnDecl, src, text.FmtInvalidOrMisplacedFnDeclShouldBeAfterCapturedVarDeclaration("a")),
				makeError(fnDecl.Function.CaptureList[0], src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable is not a local", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				fn[a] f(){}
			`)
			fnExpr := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExpr.CaptureList[0], src, text.FmtCannotPassGlobalToFunction("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("parameter shadows a global", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				fn f(a){return a}
			`)
			fn := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fn.Parameters[0], src, text.FmtParameterCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable should be accessible in body", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				fn[a] f(){ return a }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("declaration in another function declaration", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){
					fn g(){
	
					}
				}
			`)
			declNode := ast.FindNodes(n, (*ast.FunctionDeclaration)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(declNode, src, text.INVALID_FN_DECL_SHOULD_BE_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("function declared twice", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}
				fn f(){}
			`)
			declNode := ast.FindNodes(n, (*ast.FunctionDeclaration)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(declNode, src, text.FmtInvalidFnDeclAlreadyDeclared("f")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("function with same name in an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}
	
				go do {
					fn f(){}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("function declaration with the same name as a global variable definition", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar f = 0
	
				fn f(){}
			`)
			globalVar := ast.FindNode(n, (*ast.GlobalVariableDeclarator)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(globalVar, src, text.FmtInvalidAssignmentNameIsFuncName("f")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("a function that does not capture locals nor access globals is callable anywhere: identifier callee", func(t *testing.T) {
			n, src := mustParseCode(`
				return (g() + f())

				fn g(){
					return f()
				}

				fn f(){
					return 1
				}
			`)

			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src, State: NewGlobalState(ctx)})
			if !assert.NoError(t, err) {
				return
			}

			returnStmt := ast.FindNode(n, (*ast.ReturnStatement)(nil), func(n *ast.ReturnStatement, isFirstFound bool, _ []ast.Node) bool {
				return isFirstFound
			})

			pos, _ := data.GetEarlyFunctionDeclarationsPosition(n)
			assert.Equal(t, returnStmt.Base().Span.Start, pos)

			decls := data.GetFunctionsToDeclareEarly(n)
			assert.Len(t, decls, 2)
		})

		t.Run("a function that does not capture locals nor access globals is callable anywhere: variable callee", func(t *testing.T) {
			n, src := mustParseCode(`
				return ($g() + $f())

				fn g(){
					return f()
				}

				fn f(){
					return 1
				}
			`)

			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src, State: NewGlobalState(ctx)})
			if !assert.NoError(t, err) {
				return
			}

			returnStmt := ast.FindFirstNode(n, (*ast.ReturnStatement)(nil))

			pos, _ := data.GetEarlyFunctionDeclarationsPosition(n)
			assert.Equal(t, returnStmt.Span.Start, pos)

			decls := data.GetFunctionsToDeclareEarly(n)
			assert.Len(t, decls, 2)
		})

		t.Run("in an embedded module a function that does not capture locals nor access globals is callable anywhere", func(t *testing.T) {
			n, src := mustParseCode(`
				go do {
					return (g() + f())

					fn g(){
						return f()
					}
	
					fn f(){
						return 1
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src, State: NewGlobalState(ctx)})
			if !assert.NoError(t, err) {
				return
			}

			returnStmt := ast.FindFirstNode(n, (*ast.ReturnStatement)(nil))

			embeddedMod := ast.FindNode(n, (*ast.EmbeddedModule)(nil), nil)

			pos, _ := data.GetEarlyFunctionDeclarationsPosition(embeddedMod)
			assert.Equal(t, returnStmt.Span.Start, pos)

			decls := data.GetFunctionsToDeclareEarly(n)
			assert.Len(t, decls, 0)

			decls = data.GetFunctionsToDeclareEarly(embeddedMod)
			assert.Len(t, decls, 2)
		})

		t.Run("the early declarations of functions that don't capture any local should happen at the top-level statement that is the closest"+
			" to the first reference to one of the functions", func(t *testing.T) {
			t.Run("base case", func(t *testing.T) {
				chunk, src := mustParseCode(`
					for true {
						f()
					}

					fn f(){
						return 1
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				data, err := StaticCheck(StaticCheckInput{Node: chunk, Chunk: src, State: NewGlobalState(ctx)})
				if !assert.NoError(t, err) {
					return
				}

				forStmt := ast.FindNode(chunk, (*ast.ForStatement)(nil), nil)

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.Equal(t, forStmt.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(chunk)
				assert.Len(t, decls, 1)
			})

			t.Run("reference in a function expression", func(t *testing.T) {
				chunk, src := mustParseCode(`
					for true {
						fn(){ return f() }
					}

					fn f(){
						return 1
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				data, err := StaticCheck(StaticCheckInput{Node: chunk, Chunk: src, State: NewGlobalState(ctx)})
				if !assert.NoError(t, err) {
					return
				}

				forStmt := ast.FindNode(chunk, (*ast.ForStatement)(nil), nil)

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.Equal(t, forStmt.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(chunk)
				assert.Len(t, decls, 1)
			})

			t.Run("reference in a return statement before the declaration", func(t *testing.T) {
				chunk, src := mustParseCode(`
					return f()

					fn f(){
						return 1
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				data, err := StaticCheck(StaticCheckInput{Node: chunk, Chunk: src, State: NewGlobalState(ctx)})
				if !assert.NoError(t, err) {
					return
				}

				returnStmt := chunk.Statements[0]

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.Equal(t, returnStmt.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(chunk)
				assert.Len(t, decls, 1)
			})

			t.Run("reference in a function expression in a return statement before the declaration", func(t *testing.T) {
				chunk, src := mustParseCode(`
					return fn(){ f() }

					fn f(){
						return 1
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				data, err := StaticCheck(StaticCheckInput{Node: chunk, Chunk: src, State: NewGlobalState(ctx)})
				if !assert.NoError(t, err) {
					return
				}

				returnStmt := chunk.Statements[0]

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.Equal(t, returnStmt.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(chunk)
				assert.Len(t, decls, 1)
			})

			t.Run("reference in an markup interpolation", func(t *testing.T) {
				chunk, src := mustParseCode(`
					html<div> {f()} </div>

					fn f(){
						return 1
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				state := NewGlobalState(ctx)

				data, err := StaticCheck(StaticCheckInput{
					Node:  chunk,
					Chunk: src,
					State: state,
					Globals: GlobalVariablesFromMap(
						map[string]Value{
							"html": NewNamespace("html", nil),
						},
						[]string{"html"},
					),
				})
				if !assert.NoError(t, err) {
					return
				}

				markupExpr := chunk.Statements[0]

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.Equal(t, markupExpr.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(chunk)
				assert.Len(t, decls, 1)
			})

			t.Run("reference in an markup interpolation in a return statement", func(t *testing.T) {
				chunk, src := mustParseCode(`
					return html<div> {f()} </div>

					fn f(){
						return 1
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				state := NewGlobalState(ctx)

				data, err := StaticCheck(StaticCheckInput{
					Node:  chunk,
					Chunk: src,
					State: state,
					Globals: GlobalVariablesFromMap(
						map[string]Value{
							"html": NewNamespace("html", nil),
						},
						[]string{"html"},
					),
				})
				if !assert.NoError(t, err) {
					return
				}

				returnStmt := chunk.Statements[0]

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.Equal(t, returnStmt.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(chunk)
				assert.Len(t, decls, 1)
			})

			t.Run("embedded module", func(t *testing.T) {
				chunk, src := mustParseCode(`
				go do {
						for true {
							f()
						}
		
						fn f(){
							return 1
						}
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				data, err := StaticCheck(StaticCheckInput{Node: chunk, Chunk: src, State: NewGlobalState(ctx)})
				if !assert.NoError(t, err) {
					return
				}

				//Check data for the embedded module.

				embeddedModule := chunk.Statements[0].(*ast.SpawnExpression).Module
				forStmt := ast.FindNode(chunk, (*ast.ForStatement)(nil), nil)

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(embeddedModule)
				assert.Equal(t, forStmt.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(embeddedModule)
				assert.Len(t, decls, 1)

				//Nothing should be defined in the top-level chunk.

				_, ok := data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.False(t, ok)

				decls = data.GetFunctionsToDeclareEarly(chunk)
				assert.Empty(t, decls)
			})

			t.Run("identical function declaration in an embedded module", func(t *testing.T) {
				chunk, src := mustParseCode(`
					go do {
						fn f(){
							return 2
						}
					}

					fn f(){
						return 1
					}
				`)

				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				data, err := StaticCheck(StaticCheckInput{Node: chunk, Chunk: src, State: NewGlobalState(ctx)})
				if !assert.NoError(t, err) {
					return
				}

				//Check the declaration that is inside the embedded module.

				embeddedModule := chunk.Statements[0].(*ast.SpawnExpression).Module
				fnDeclInEmbeddedModule := embeddedModule.Statements[0]

				pos, _ := data.GetEarlyFunctionDeclarationsPosition(embeddedModule)
				assert.Equal(t, fnDeclInEmbeddedModule.Base().Span.Start, pos)

				decls := data.GetFunctionsToDeclareEarly(embeddedModule)
				assert.Len(t, decls, 1)

				//Check the other declaration.

				topLevelFnDecl := chunk.Statements[1]

				pos, _ = data.GetEarlyFunctionDeclarationsPosition(chunk)
				assert.Equal(t, topLevelFnDecl.Base().Span.Start, pos)

				decls = data.GetFunctionsToDeclareEarly(chunk)
				assert.Len(t, decls, 1)
			})

		})

		t.Run("a function that captures a local variable should be declared after the declaration of the variable", func(t *testing.T) {
			n, src := mustParseCode(`
				fn[x] f(){
					return x
				}

				x = 1
			`)
			declNode := ast.FindNode(n, (*ast.FunctionDeclaration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(declNode, src, text.FmtInvalidOrMisplacedFnDeclShouldBeAfterCapturedVarDeclaration("x")),
				makeError(declNode.Function.CaptureList[0], src, text.FmtVarIsNotDeclared("x")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("a function that captures a local variable is only accessible after the function's declaration", func(t *testing.T) {
			n, src := mustParseCode(`
				x = 1

				val = f()

				fn g(){
					return f()
				}

				fn[x] f(){
					return x
				}

				f()
			`)
			callExprs := ast.FindNodes(n, (*ast.CallExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(callExprs[0].Callee, src, text.FmtVarIsNotDeclared("f")),
				makeError(callExprs[1].Callee, src, text.FmtVarIsNotDeclared("f")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("captured variable does not exist", func(t *testing.T) {
			n, src := mustParseCode(`
				fn[a](){

				}
			`)
			fnExprNode := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExprNode.CaptureList[0], src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable is not a local", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				fn[a](){}
			`)
			fnExprNode := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExprNode.CaptureList[0], src, text.FmtCannotPassGlobalToFunction("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate variable capture", func(t *testing.T) {
			n, src := mustParseCode(`
				var a = 1
				fn[a, a](){}
			`)
			fnExprNode := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExprNode.CaptureList[1], src, text.FmtVarIsAlreadyCaptured("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable should be accessible in body", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				fn[a](){ return a }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate parameter declaration", func(t *testing.T) {
			n, src := mustParseCode(`
				var a = 1
				fn(a, a){}
			`)
			fnExprNode := ast.FindNode(n, (*ast.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fnExprNode.Parameters[1], src, text.FmtParameterAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("function pattern expression", func(t *testing.T) {

		t.Run("parameter shadows a global", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				pattern one = 1
				%fn(a %one)
			`)
			fn := ast.FindNode(n, (*ast.FunctionPatternExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(fn.Parameters[0], src, text.FmtParameterCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("local variable declaration: regular", func(t *testing.T) {
		t.Run("declaration after assignment", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 0
				var a = 0
			`)
			decl := ast.FindNode(n, (*ast.LocalVariableDeclarator)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decl.Left, src, text.FmtInvalidLocalVarDeclAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("shadowing of global variable", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 0
				var a = 0
			`)
			decl := ast.FindNode(n, (*ast.LocalVariableDeclarator)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decl.Left, src, text.FmtCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate declarations", func(t *testing.T) {
			n, src := mustParseCode(`
				var a = 0
				var a = 1
			`)
			decl := ast.FindNodes(n, (*ast.LocalVariableDeclarator)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decl.Left, src, text.FmtInvalidLocalVarDeclAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("invalid LHS", func(t *testing.T) {
			n, src, err := parseCode(`
				var (1 = 1)
			`)
			assert.Error(t, err)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("global variable declaration: regular", func(t *testing.T) {

		t.Run("shadowing of local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				$a = 0
				globalvar a = 0
			`)
			decl := ast.FindNode(n, (*ast.GlobalVariableDeclarator)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decl, src, text.FmtCannotShadowLocalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate declarations", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 0
				globalvar a = 1
			`)
			decl := ast.FindNodes(n, (*ast.GlobalVariableDeclarator)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decl, src, text.FmtInvalidGlobalVarDeclAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should be a top-level statement", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){
					globalvar a = 0
				}
			`)
			decls := ast.FindNode(n, (*ast.GlobalVariableDeclarations)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decls, src, text.MISPLACED_GLOBAL_VAR_DECLS_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("declaring a global variable is not allowed after a function declaration", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}

				globalvar a = 0
			`)
			decls := ast.FindNode(n, (*ast.GlobalVariableDeclarations)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decls, src, text.MISPLACED_GLOBAL_VAR_DECLS_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("declaring a global variable is not allowed after a call to a function declared below", func(t *testing.T) {
			n, src := mustParseCode(`
				f()
				
				globalvar a = 0

				fn f(){}
			`)
			decls := ast.FindNode(n, (*ast.GlobalVariableDeclarations)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decls, src, text.MISPLACED_GLOBAL_VAR_DECLS_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definitions are not allowed after a reference (identifier) to a function declared below", func(t *testing.T) {
			n, src := mustParseCode(`
				f
				
				globalvar a = 0

				fn f(){}
			`)
			decls := ast.FindNode(n, (*ast.GlobalVariableDeclarations)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decls, src, text.MISPLACED_GLOBAL_VAR_DECLS_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definitions are not allowed after a reference (global variable node) to a function declared below", func(t *testing.T) {
			n, src := mustParseCode(`
				$f
				
				globalvar a = 0

				fn f(){}
			`)
			decls := ast.FindNode(n, (*ast.GlobalVariableDeclarations)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(decls, src, text.MISPLACED_GLOBAL_VAR_DECLS_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("declaring a global variable is allowed after a call to a function declared in an included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix

				f()
				globalvar a = 0
			`, map[string]string{"./dep.ix": "includable-file\n fn f(){}"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			assert.NoError(t, err)
		})

		t.Run("invalid LHS", func(t *testing.T) {
			n, src, err := parseCode(`
				var (1 = 1)
			`)
			assert.Error(t, err)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("global variable declaration: object destructuration", func(t *testing.T) {

		t.Run("shadowing of local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 0
				globalvar {a} = {}
			`)
			destructuration := ast.FindNode(n, (*ast.ObjectDestructuration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(destructuration.Properties[0], src, text.FmtCannotShadowLocalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate names", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar {a,a} = {}
			`)
			destructuration := ast.FindNode(n, (*ast.ObjectDestructuration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(destructuration.Properties[1], src, text.FmtInvalidGlobalVarDeclAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate names", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar {a as b,b} = {}
			`)
			destructuration := ast.FindNode(n, (*ast.ObjectDestructuration)(nil), nil)
			newName := destructuration.Properties[1].(*ast.ObjectDestructurationProperty).PropertyName

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(newName, src, text.FmtInvalidGlobalVarDeclAlreadyDeclared("b")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate declarations", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar {a} = {}
				globalvar {a} = {}
			`)
			destructuration := ast.FindNode(n, (*ast.ObjectDestructuration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(destructuration.Properties[0], src, text.FmtInvalidGlobalVarDeclAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("assignment", func(t *testing.T) {
		t.Run("assignment with a function's name (identifier)", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}
	
				f = 0
			`)
			assignment := ast.FindNodes(n, (*ast.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.FmtInvalidAssignmentNameIsFuncName("f")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment with a function's name (variable)", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}
	
				f = 0
			`)
			assignment := ast.FindNodes(n, (*ast.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.FmtInvalidAssignmentNameIsFuncName("f")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a constant in top level (identifier)", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					a = 1
				)
	
				a = 0
			`)
			assignment := ast.FindNodes(n, (*ast.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a constant in top level (variable)", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					a = 1
				)
	
				$a = 0
			`)
			assignment := ast.FindNodes(n, (*ast.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a global constant in a function", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					a = 0
				)
	
				fn f(){
					a = 1
				}
			`)

			assignment := ast.FindNodes(n, (*ast.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a global variable in a function", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 0

				fn f(){
					a = 1
				}
			`)

			assignment := ast.FindNodes(n, (*ast.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a local variable in embedded module: name of a global constant in parent module", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					a = 1
				)
	
				go do {
					a = 2
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global variable assignment is not allowed", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				a = 1
			`)

			assignment := ast.FindNode(n, (*ast.Assignment)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("undefined local variable += assignment", func(t *testing.T) {
			n, src := mustParseCode(`
				a += 1
			`)

			assignment := ast.FindNode(n, (*ast.Assignment)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.FmtInvalidVariableAssignmentVarDoesNotExist("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("slice expression LHS: += assignment", func(t *testing.T) {
			n, src := mustParseCode(`
				var s = [1, 2]
				s[0:1] += 2
			`)

			assignment := ast.FindNode(n, (*ast.Assignment)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.INVALID_ASSIGNMENT_EQUAL_ONLY_SUPPORTED_ASSIGNMENT_OPERATOR_FOR_SLICE_EXPRS),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("multi assignment", func(t *testing.T) {
		t.Run("global variable re-assignment", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				assign a b = [1, 2]
			`)

			assignment := ast.FindNode(n, (*ast.MultiAssignment)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(assignment, src, text.GLOBAL_VARS_AND_CONSTS_CANNOT_BE_REASSIGNED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("invalid LHS", func(t *testing.T) {
			n, src, err := parseCode(`
				assign 1 = 1
			`)
			assert.Error(t, err)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("global variable", func(t *testing.T) {
		t.Run("global is accessible in manifest", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					a = 1
				)
	
				manifest {
					limits: {
						"x": $a
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global is accessible in module", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					a = 1
				)
	
				return $a
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global is accessible in function", func(t *testing.T) {
			n, src := mustParseCode(`
				const (
					a = 1
				)
	
				fn f(){
					return $a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global variable defined by import statement", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import result ./dep.ix {}
				$result
			`, map[string]string{
				"./dep.ix": `
					manifest {}
				`,
			})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			ctx := NewContext(ContextConfig{
				Permissions: []Permission{
					FilesystemPermission{Kind_: permbase.Read, Entity: PathPattern("/...")},
				},
			})
			state := NewGlobalState(ctx)
			state.Module = mod
			defer ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})
	})

	t.Run("local variable", func(t *testing.T) {
		t.Run("local variable in a module : undefined", func(t *testing.T) {
			n, src := mustParseCode(`
				$a
			`)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(n.Statements[0], src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("local variable in a module : defined", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				$a
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("local variable in an embedded module : undefined", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				go do {
					$a
				}
			`)
			varNode := ast.FindNode(n, (*ast.Variable)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("local variable in a function : undefined", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				fn f(){
					$a
				}
			`)
			varNode := ast.FindNode(n, (*ast.Variable)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("local variable in a function : defined", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){
					a = 1
					$a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("local variable in a lazy expression", func(t *testing.T) {
			n, src := mustParseCode(`
				@($a)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("argument variable in a function", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(a){
					$a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("manifest", func(t *testing.T) {
		t.Run("parameters section not allowed in embedded module manifest", func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {
				}

				go do {
					manifest {
						parameters: {}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			n, src = mustParseCode(`
				manifest {}

				testsuite "" {
					manifest {
						parameters: {}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			n, src = mustParseCode(`
				manifest {}

				testsuite "" {
					testcase {
						manifest {
							parameters: {}
						}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("env section not allowed in embedded module manifest", func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {
				}

				go do {
					manifest {
						env: %{}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			n, src = mustParseCode(`
				manifest {}

				testsuite "" {
					manifest {
						env: %{}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			n, src = mustParseCode(`
				manifest {}

				testsuite "" {
					testcase {
						manifest {
							env: %{}
						}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("databases section not allowed in embedded module manifest", func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {
				}

				go do {
					manifest {
						databases: {}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			n, src = mustParseCode(`
				manifest {}

				testsuite "" {
					manifest {
						databases: {}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))

			n, src = mustParseCode(`
				manifest {}

				testsuite "" {
					testcase {
						manifest {
							databases: {}
						}
					}
				}
			`)
			assert.Error(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("test suite statements", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testsuite {
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("should have its own local scope", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				testsuite { 
					a
				}
			`)

			identLiteral := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(identLiteral, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should inherit globals", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				testsuite { 
					a
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("should inherit patterns", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = 1
				testsuite { 
					%p
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("should inherit pattern namespaces", func(t *testing.T) {
			n, src := mustParseCode(`
				pnamespace ns. = {a: %{a: 1}}
				testsuite { 
					%ns.
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("testcase", func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testsuite {
					testcase {

					}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("testcase should inherit globals of the test suite", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				testsuite { 
					globalvar b = 2
					testcase {
						a
						b
					}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("testcase should inherit patterns of the test suite", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p1 = 1
				testsuite { 
					pattern p2 = 1
					testcase {
						%p1
						%p2
					}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("testcase should inherit pattern namespaces of the test suite", func(t *testing.T) {
			n, src := mustParseCode(`
				pnamespace ns1. = {a: %{a: 1}}
				testsuite { 
					pnamespace ns2. = {a: %{a: 1}}
					testcase {
						%ns1.
						%ns2.
					}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("sub testsuite", func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testsuite {
					testsuite {
						
					}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run(text.TEST_CASES_NOT_ALLOWED_IF_SUBSUITES_ARE_PRESENT, func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testsuite {
					testcase {

					}
					testsuite {

					}
				}
			`)

			testCaseStmt := ast.FindNode(n, (*ast.TestCaseExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(testCaseStmt, src, text.TEST_CASES_NOT_ALLOWED_IF_SUBSUITES_ARE_PRESENT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run(text.TEST_CASES_NOT_ALLOWED_IF_SUBSUITES_ARE_PRESENT, func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testsuite {
					testsuite {

					}
					testcase {

					}
				}
			`)

			testCaseStmt := ast.FindNode(n, (*ast.TestCaseExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(testCaseStmt, src, text.TEST_CASES_NOT_ALLOWED_IF_SUBSUITES_ARE_PRESENT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run(text.TEST_SUITE_STMTS_NOT_ALLOWED_INSIDE_TEST_CASE_STMTS, func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testsuite {
					testcase {
						testsuite {

						}
					}
				}
			`)

			testCaseStmt := ast.FindNode(n, (*ast.TestCaseExpression)(nil), nil)
			testSuiteStmt := ast.FindNode(testCaseStmt, (*ast.TestSuiteExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(testSuiteStmt, src, text.TEST_SUITE_STMTS_NOT_ALLOWED_INSIDE_TEST_CASE_STMTS),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("test case statements", func(t *testing.T) {
		t.Run("allowed in test suite modules", func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testcase {}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				//test suite module
				Module: core.WrapLowerModule(&inoxmod.Module{
					MainChunk:    src,
					TopLevelNode: src.Node,
					Kind:         TestSuiteModule,
				}),
			}))
		})

		t.Run(text.TEST_CASE_STMTS_NOT_ALLOWED_OUTSIDE_OF_TEST_SUITES, func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				testcase {}
			`)

			testCaseStmt := ast.FindNode(n, (*ast.TestCaseExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(testCaseStmt, src, text.TEST_CASE_STMTS_NOT_ALLOWED_OUTSIDE_OF_TEST_SUITES),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run(text.TEST_CASE_STMTS_NOT_ALLOWED_OUTSIDE_OF_TEST_SUITES, func(t *testing.T) {
			n, src := mustParseCode(`
				manifest {}

				fn f(){
					testcase {}
				}
			`)

			testCaseStmt := ast.FindNode(n, (*ast.TestCaseExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(testCaseStmt, src, text.TEST_CASE_STMTS_NOT_ALLOWED_OUTSIDE_OF_TEST_SUITES),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("testsuite expression", func(t *testing.T) {

		t.Run("should have its own local scope", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				testsuite { a }
			`)

			identLiteral := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(identLiteral, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not inherit the `dbs` global", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar dbs = {}
				testsuite { 
					dbs
				 }
			`)

			identLiteral := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(identLiteral, src, text.FmtVarIsNotDeclared("dbs")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("testcase expression", func(t *testing.T) {

		t.Run("testsuite expression has its own local scope", func(t *testing.T) {
			n, src := mustParseCode(`
				a = 1
				return testcase { a }
			`)

			identLiteral := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(identLiteral, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("a __test global should be defined within test cases", func(t *testing.T) {
			n, src := mustParseCode(`
				return testcase { $__test }
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("should not inherit the `dbs` global", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar dbs = {}
				return testcase {
					dbs
				}
			`)

			identLiteral := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(identLiteral, src, text.FmtVarIsNotDeclared("dbs")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("inclusion import statement", func(t *testing.T) {
		t.Run("not allowed in functions", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				fn f(){
					import ./dep.ix
					return $a
				}
			`, map[string]string{"./dep.ix": "includable-file\n a = 1"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			importStmt := ast.FindNode(mod.MainChunk.Node, (*ast.InclusionImportStatement)(nil), nil)
			variable := ast.FindNode(mod.MainChunk.Node, (*ast.Variable)(nil), nil)

			expectedErr := utils.CombineErrors(
				makeError(importStmt, mod.MainChunk, text.MISPLACED_INCLUSION_IMPORT_STATEMENT_TOP_LEVEL_STMT),
				makeError(variable, mod.MainChunk, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single included file with no dependencies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-file\n const( a = 1 )"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("two included files with no dependecies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep1.ix
				import ./dep2.ix
				return (a + b)
			`, map[string]string{
				"./dep1.ix": `
					includable-file
					const (
						a = 1
					)
				`,
				"./dep2.ix": `
					includable-file
					const (
						b = 2
					)
				`,
			})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single included file with no dependencies: error in included file", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-file\n const(a = b)"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			expectedErr := utils.CombineErrors(
				NewStaticCheckError(text.FmtVarIsNotDeclared("b"), sourcecode.SourcePositionStack{
					sourcecode.PositionRange{
						SourceName:  mod.MainChunk.Name(),
						StartLine:   3,
						StartColumn: 5,
					},
					sourcecode.PositionRange{
						SourceName:  mod.FlattenedIncludedChunkList[0].ParsedChunkSource.Name(),
						StartLine:   2,
						StartColumn: 12,
					},
				}),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single included file with no dependencies: duplicate constant declaration", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				const a = 1
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-file\n const a = 2"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			expectedErr := utils.CombineErrors(
				NewStaticCheckError(text.FmtCannotShadowGlobalVariable("a"), sourcecode.SourcePositionStack{
					sourcecode.PositionRange{
						SourceName:  mod.MainChunk.Name(),
						StartLine:   4,
						StartColumn: 5,
					},
				}),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single included file which itself includes a file", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep2.ix
				return a
			`, map[string]string{
				"./dep2.ix": `
					includable-file
					import ./dep1.ix
				`,
				"./dep1.ix": `
					includable-file
					const (
						a = 1
					)
				`,
			})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("included file should not import modules", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
			`, map[string]string{
				"./dep.ix": `
					includable-file
					import res ./lib.ix {}
				`,
			})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})
			expectedErr := utils.CombineErrors(
				NewStaticCheckError(text.AN_INCLUDABLE_FILE_CAN_ONLY_CONTAIN_DEFINITIONS, sourcecode.SourcePositionStack{
					sourcecode.PositionRange{
						SourceName:  mod.MainChunk.Name(),
						StartLine:   3,
						StartColumn: 5,
					},
					sourcecode.PositionRange{
						SourceName:  mod.IncludedChunkForest[0].Name(),
						StartLine:   3,
						StartColumn: 6,
					},
				}),
				NewStaticCheckError(text.MODULE_IMPORTS_NOT_ALLOWED_IN_INCLUDABLE_FILES, sourcecode.SourcePositionStack{
					sourcecode.PositionRange{
						SourceName:  mod.MainChunk.Name(),
						StartLine:   3,
						StartColumn: 5,
					},
					sourcecode.PositionRange{
						SourceName:  mod.IncludedChunkForest[0].Name(),
						StartLine:   3,
						StartColumn: 6,
					},
				}),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("included file that does not exist", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
			`, map[string]string{})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{
				Context:                             createParsingContext(modpath),
				RecoverFromNonExistingIncludedFiles: true,
			})

			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})
	})

	t.Run("import statement", func(t *testing.T) {
		createState := func(mod *Module) *GlobalState {
			state := NewGlobalState(NewContext(ContextConfig{
				Permissions: []Permission{
					FilesystemPermission{Kind_: permbase.Read, Entity: PathPattern("/...")},
				},
			}))
			state.Module = mod
			return state
		}

		t.Run("not allowed in functions", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				fn f(){
					import res ./dep.ix {}
					return $res
				}
			`, map[string]string{"./dep.ix": "manifest {}\n a = 1"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			importStmt := ast.FindNode(mod.MainChunk.Node, (*ast.ImportStatement)(nil), nil)
			variable := ast.FindNode(mod.MainChunk.Node, (*ast.Variable)(nil), nil)

			expectedErr := utils.CombineErrors(
				makeError(importStmt, mod.MainChunk, text.MISPLACED_MOD_IMPORT_STATEMENT_TOP_LEVEL_STMT),
				makeError(variable, mod.MainChunk, text.FmtVarIsNotDeclared("res")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single imported module with no dependencies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep.ix {}
				return res
			`, map[string]string{"./dep.ix": "manifest {}\n a = 1"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			state := createState(mod)
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single imported module with parameter", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep.ix {}
				return res
			`, map[string]string{"./dep.ix": `
					manifest {
						parameters: {
							a: %str
						}
					}
					b = mod-args
			`})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			state := createState(mod)
			state.GetBasePatternsForImportedModule = func() (map[string]Pattern, map[string]*PatternNamespace) {
				return map[string]Pattern{"str": STR_PATTERN}, nil
			}
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single imported module should have access to base patterns if set", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep.ix {}
				return res
			`, map[string]string{"./dep.ix": `
				manifest {}
				a = 1
				$pattern = %x
				namespace = %ix.
			`})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			state := createState(mod)
			state.GetBasePatternsForImportedModule = func() (map[string]Pattern, map[string]*PatternNamespace) {
				return map[string]Pattern{
						"x": INT_PATTERN,
					}, map[string]*PatternNamespace{
						"ix": DEFAULT_PATTERN_NAMESPACES["inox"],
					}
			}
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single imported module should have access to base globals if set", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep.ix {}
				return res
			`, map[string]string{"./dep.ix": `
				manifest {}
				b = a
			`})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			state := createState(mod)
			state.SymbolicBaseGlobalsForImportedModule = map[string]symbolic.Value{"a": symbolic.NewInt(1)}
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("two imported module with no dependecies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res1 ./dep1.ix {}
				import res2 ./dep2.ix {}
			`, map[string]string{
				"./dep1.ix": `
					manifest {}
					a = 1
				`,
				"./dep2.ix": `
					manifest {}
					b = 2
				`,
			})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := createState(mod)
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single imported module with no dependencies: error in imported module", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep.ix {}
			`, map[string]string{"./dep.ix": "manifest {}\n a = b"})
			importedModulePath := filepath.Join(filepath.Dir(modpath), "dep.ix")

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := createState(mod)
			defer state.Ctx.CancelGracefully()

			err = staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			expectedErr := utils.CombineErrors(
				NewStaticCheckError(text.FmtVarIsNotDeclared("b"), sourcecode.SourcePositionStack{
					sourcecode.PositionRange{
						SourceName:  mod.MainChunk.Name(),
						StartLine:   3,
						StartColumn: 5,
					},
					sourcecode.PositionRange{
						SourceName:  importedModulePath,
						StartLine:   2,
						StartColumn: 6,
					},
				}),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single imported module with no dependencies: same constant declaration", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				const a = 1
				manifest {}
				import res ./dep.ix {}
			`, map[string]string{"./dep.ix": "const a = 2\nmanifest {}"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			state := createState(mod)
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single imported module which includes a file", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep2.ix {}
			`, map[string]string{
				"./dep2.ix": `
					manifest {}
					import ./dep1.ix
				`,
				"./dep1.ix": `
					includable-file
					const (a = 1)
				`,
			})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := createState(mod)
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single imported module which includes a file with a static check error", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep2.ix {}
			`, map[string]string{
				"./dep2.ix": `
					manifest {}
					import ./dep1.ix
				`,
				"./dep1.ix": `
					includable-file
					const(a = b)
				`,
			})
			importedModulePath := filepath.Join(filepath.Dir(modpath), "dep2.ix")
			includedFilePath := filepath.Join(filepath.Dir(modpath), "dep1.ix")

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			state := createState(mod)
			defer state.Ctx.CancelGracefully()

			err = staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			expectedErr := utils.CombineErrors(
				NewStaticCheckError(text.FmtVarIsNotDeclared("b"), sourcecode.SourcePositionStack{
					sourcecode.PositionRange{
						SourceName:  modpath,
						StartLine:   3,
						StartColumn: 5,
					},
					sourcecode.PositionRange{
						SourceName:  importedModulePath,
						StartLine:   3,
						StartColumn: 6,
					},
					sourcecode.PositionRange{
						SourceName:  includedFilePath,
						StartLine:   3,
						StartColumn: 16,
					},
				}),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("imported module which itself imports a module", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./dep.ix {}
			`, map[string]string{
				"./dep.ix": `
					manifest {}
					import res ./lib.ix {}
				`,
				"./lib.ix": `
					manifest {}
				`,
			})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := createState(mod)
			defer state.Ctx.CancelGracefully()

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				State:  state,
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})
	})

	t.Run("coyield statement", func(t *testing.T) {
		t.Run("in embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				go do { coyield }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in function in embedded modue", func(t *testing.T) {
			n, src := mustParseCode(`
				go do { fn f(){ coyield } }
			`)

			yieldStmt := ast.FindNode(n, (*ast.CoyieldStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(yieldStmt, src, text.MISPLACE_COYIELD_STATEMENT_ONLY_ALLOWED_IN_EMBEDDED_MODULES),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("break statement", func(t *testing.T) {
		t.Run("direct child of a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					break
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					break
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a switch statement's non-default case", func(t *testing.T) {
			n, src := mustParseCode(`
				switch 1 {
					1 {
						break
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a switch statement's default case", func(t *testing.T) {
			n, src := mustParseCode(`
				switch 1 {
					defaultcase {
						break
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a match statement's non-default case", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					1 {
						break
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a match statement's default case", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					defaultcase {
						break
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an if statement in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					if true {
						break
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an if statement in a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					if true {
						break
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an switch statement in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					switch i {
						1 {
							break
						}
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an match statement in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					match i {
						1 {
							break
						}
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a module", func(t *testing.T) {
			n, src := mustParseCode(`
				break
			`)
			breakStmt := ast.FindNode(n, (*ast.BreakStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(breakStmt, src, text.BREAK_STMTS_ONLY_ALLOWED_LOCATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				go do {
					break
				}
			`)
			breakStmt := ast.FindNode(n, (*ast.BreakStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(breakStmt, src, text.BREAK_STMTS_ONLY_ALLOWED_LOCATION),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("continue statement", func(t *testing.T) {
		t.Run("direct child of a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					continue
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					continue
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a walk statement", func(t *testing.T) {
			n, src := mustParseCode(`
				walk / entry {
					continue
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					continue
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an if statement in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					if true {
						continue
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a switch statement's non-default case", func(t *testing.T) {
			n, src := mustParseCode(`
				switch 1 {
					1 {
						continue
					}
				}
			`)

			continueStmt := ast.FindNode(n, (*ast.ContinueStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(continueStmt, src, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of a switch statement's non-default case in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for e in [] {
					switch 1 {
						1 {
							continue
						}
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a switch statement's default case", func(t *testing.T) {
			n, src := mustParseCode(`
				switch 1 {
					defaultcase {
						continue
					}
				}
			`)

			continueStmt := ast.FindNode(n, (*ast.ContinueStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(continueStmt, src, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of a switch statement's default case in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for e in [] {
					switch 1 {
						defaultcase {
							continue
						}
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a match statement's non-default case", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					1 {
						continue
					}
				}
			`)

			continueStmt := ast.FindNode(n, (*ast.ContinueStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(continueStmt, src, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of a match statement's default case", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					defaultcase {
						continue
					}
				}
			`)

			continueStmt := ast.FindNode(n, (*ast.ContinueStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(continueStmt, src, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("in an if statement in a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					if true {
						continue
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an switch statement in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					switch i {
						1 {
							continue
						}
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an match statement in a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					match i {
						1 {
							continue
						}
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a module", func(t *testing.T) {
			n, src := mustParseCode(`
				continue
			`)
			continueStmt := ast.FindNode(n, (*ast.ContinueStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(continueStmt, src, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				go do {
					continue
				}
			`)
			continueStmt := ast.FindNode(n, (*ast.ContinueStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(continueStmt, src, text.CONTINUE_STMTS_ONLY_ALLOWED_IN_BODY_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("prune statement", func(t *testing.T) {
		t.Run("direct child of a walk statement", func(t *testing.T) {
			n, src := mustParseCode(`
				walk / entry {
					prune
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					prune
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a for statement in a walk statement", func(t *testing.T) {
			n, src := mustParseCode(`
				walk / entry {
					for [] {
						prune
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					prune
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a for statement in a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					for [] {
						prune
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					prune
				}
			`)

			yieldStmt := ast.FindNode(n, (*ast.PruneStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(yieldStmt, src, text.PRUNE_STMTS_ARE_ONLY_ALLOWED_IN_WALK_STMTS_AND_EXPRS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					prune
				})
			`)

			yieldStmt := ast.FindNode(n, (*ast.PruneStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(yieldStmt, src, text.PRUNE_STMTS_ARE_ONLY_ALLOWED_IN_WALK_STMTS_AND_EXPRS),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("yield statement", func(t *testing.T) {
		t.Run("direct child of a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					yield
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					yield
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for statement inside a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					for j, el in [] {
						yield
					}
				})
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for statement inside a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					for j, el in [] {
						yield
					}
				})
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an if statement in a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					if true {
						yield
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an if statement in a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					if true {
						yield
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an switch statement in a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					switch i {
						1 {
							yield
						}
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an switch statement in a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					switch entry {
						1 {
							yield
						}
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an match statement in a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					match i {
						1 {
							yield
						}
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an match statement in a walk expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk / entry {
					match entry {
						1 {
							yield
						}
					}
				})
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					yield
				}
			`)

			yieldStmt := ast.FindNode(n, (*ast.YieldStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(yieldStmt, src, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_WALK_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of a walk statement", func(t *testing.T) {
			n, src := mustParseCode(`
				walk / entry {
					yield
				}
			`)

			yieldStmt := ast.FindNode(n, (*ast.YieldStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(yieldStmt, src, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_WALK_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of a module", func(t *testing.T) {
			n, src := mustParseCode(`
				yield
			`)
			yieldStmt := ast.FindNode(n, (*ast.YieldStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(yieldStmt, src, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_WALK_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				go do {
					yield
				}
			`)
			yieldStmt := ast.FindNode(n, (*ast.YieldStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(yieldStmt, src, text.YIELD_STMTS_ONLY_ALLOWED_IN_BODY_FOR_WALK_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("return statement", func(t *testing.T) {
		t.Run("direct child of a module", func(t *testing.T) {
			n, src := mustParseCode(`
				return 1
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				go do {
					return 
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for i, e in [] {
					return 1
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a for expression with a body", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					return 1
				})
			`)
			returnStmt := ast.FindNode(n, (*ast.ReturnStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(returnStmt, src, text.MISPLACED_RETURN_STATEMENT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of a for statement inside a for expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for i, e in [] {
					for j, el in [] {
						return 1
					}
				})
			`)

			returnStmt := ast.FindNode(n, (*ast.ReturnStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(returnStmt, src, text.MISPLACED_RETURN_STATEMENT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of the 'if' clause of an if statement", func(t *testing.T) {
			n, src := mustParseCode(`
				if true {
					return 1
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of the 'else' clause of an if-else statement", func(t *testing.T) {
			n, src := mustParseCode(`
				if true {

				} else {
					return 1
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a non-default case's body in an switch statement", func(t *testing.T) {
			n, src := mustParseCode(`
				switch 1 {
					1 {
						return 1
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a default case's body in an switch statement", func(t *testing.T) {
			n, src := mustParseCode(`
				switch 1 {
					defaultcase {
						return 1
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a non-default case's body in an match statement", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					1 {
						return 1
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("direct child of a default case's body in an match statement", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					defaultcase {
						return 1
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("call", func(t *testing.T) {
		t.Run("undefined callee", func(t *testing.T) {
			n, src := mustParseCode(`
				a 1
			`)
			varNode := ast.FindNode(n, (*ast.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("for statement", func(t *testing.T) {
		t.Run("variables defined in a for statement's head are not accessible after the statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for file in files {
					
				}
				return file
			`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("file")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("variables defined in a for statement's body are not accessible after the statement", func(t *testing.T) {
			n, src := mustParseCode(`
				for file in files {
					x = 3
				}
				return x
			`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[3]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("x")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key and value vars should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				k = 1
				v = 1
				for k, v in [] {}
			`)
			keyIdent := n.Statements[2].(*ast.ForStatement).KeyIndexIdent
			valueIdent := n.Statements[2].(*ast.ForStatement).ValueElemIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyIdent, src, text.FmtCannotShadowLocalVariable("k")),
				makeError(valueIdent, src, text.FmtCannotShadowLocalVariable("v")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key and value vars should not shadow global variables", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar k = 1
				globalvar v = 1
				for k, v in [] {}
			`)
			keyIdent := ast.FindIdentWithName(n, "k")
			valueIdent := ast.FindIdentWithName(n, "v")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyIdent, src, text.FmtCannotShadowGlobalVariable("k")),
				makeError(valueIdent, src, text.FmtCannotShadowGlobalVariable("v")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("for expression", func(t *testing.T) {
		t.Run("variables defined in a for expression's head are not accessible after the expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(for file in files => 0)
				return file
			`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("file")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("variables defined in a for expression's body are not accessible after the statement", func(t *testing.T) {
			n, src := mustParseCode(`
				(for file in files {
					x = 3
				})
				return x
			`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[3]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("x")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key and value vars should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				k = 1
				v = 1
				(for k, v in [] => 0)
			`)
			keyIdent := n.Statements[2].(*ast.ForExpression).KeyIndexIdent
			valueIdent := n.Statements[2].(*ast.ForExpression).ValueElemIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyIdent, src, text.FmtCannotShadowLocalVariable("k")),
				makeError(valueIdent, src, text.FmtCannotShadowLocalVariable("v")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key and value vars should not shadow global variables", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar k = 1
				globalvar v = 1
				(for k, v in [] => 0)
			`)
			keyIdent := ast.FindIdentWithName(n, "k")
			valueIdent := ast.FindIdentWithName(n, "v")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(keyIdent, src, text.FmtCannotShadowGlobalVariable("k")),
				makeError(valueIdent, src, text.FmtCannotShadowGlobalVariable("v")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("walk statement", func(t *testing.T) {
		t.Run("variables defined in walk statement's head are not accessible after the statement", func(t *testing.T) {
			n, src := mustParseCode(`
				walk ./ entry {
					
				}
				return entry
			`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("entry")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("variables defined in walk statement's body are not accessible after the statement", func(t *testing.T) {
			n, src := mustParseCode(`
				walk ./ entry {
					x = 3
				}
				return x
			`)

			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("x")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("entry var should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				e = 1
				walk ./ e {}
			`)
			entryIdent := n.Statements[1].(*ast.WalkStatement).EntryIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(entryIdent, src, text.FmtCannotShadowLocalVariable("e")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("entry var should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				var e = 1
				walk ./ e {}
			`)
			entryIdent := n.Statements[1].(*ast.WalkStatement).EntryIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(entryIdent, src, text.FmtCannotShadowLocalVariable("e")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("meta var should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				meta = 1
				walk ./ meta, e {}
			`)
			entryIdent := n.Statements[1].(*ast.WalkStatement).MetaIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(entryIdent, src, text.FmtCannotShadowLocalVariable("meta")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("walk expression", func(t *testing.T) {
		t.Run("variables defined in walk expression's head are not accessible after the expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk ./ entry {
					
				})
				return entry
			`)
			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("entry")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("variables defined in walk expression's body are not accessible after the expression", func(t *testing.T) {
			n, src := mustParseCode(`
				(walk ./ entry {
					x = 3
				})
				return x
			`)

			varNode := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(varNode, src, text.FmtVarIsNotDeclared("x")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("entry var should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				e = 1
				(walk ./ e {})
			`)
			entryIdent := n.Statements[1].(*ast.WalkExpression).EntryIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(entryIdent, src, text.FmtCannotShadowLocalVariable("e")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("entry var should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				var e = 1
				(walk ./ e {})
			`)
			entryIdent := n.Statements[1].(*ast.WalkExpression).EntryIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(entryIdent, src, text.FmtCannotShadowLocalVariable("e")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("meta var should not shadow local variables", func(t *testing.T) {
			n, src := mustParseCode(`
				meta = 1
				(walk ./ meta, e {})
			`)
			entryIdent := n.Statements[1].(*ast.WalkExpression).MetaIdent

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(entryIdent, src, text.FmtCannotShadowLocalVariable("meta")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("runtime typecheck", func(t *testing.T) {

		t.Run("as argument", func(t *testing.T) {
			n, src := mustParseCode(`map ~$ .title`)
			globals := GlobalVariablesFromMap(map[string]Value{"map": ValOf(MapIterable)}, nil)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals}))
		})

		t.Run("misplaced", func(t *testing.T) {
			n, src := mustParseCode(`~$`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(n.Statements[0], src, text.MISPLACED_RUNTIME_TYPECHECK_EXPRESSION),
			)
			assert.Equal(t, expectedErr, err)
		})
	})
	t.Run("assert statement", func(t *testing.T) {

		t.Run("no forbidden node in expression", func(t *testing.T) {
			n, src := mustParseCode(`
				x = 0
				assert (x > 0)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("forbidden node in expression", func(t *testing.T) {
			n, src := mustParseCode(`
				assert (1 + sideEffect())
			`)
			callNode := ast.FindNode(n, (*ast.CallExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(callNode, src, text.FmtFollowingNodeTypeNotAllowedInAssertions(callNode)),
				//makeError(callNode, src, text.FmtVarIsNotDeclared("sideEffect")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("reception handler expression", func(t *testing.T) {

		t.Run("misplaced", func(t *testing.T) {
			n, src := mustParseCode(`
				on received %{} fn(){}
			`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(n.Statements[0], src, text.MISPLACED_RECEPTION_HANDLER_EXPRESSION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("element of an object literal", func(t *testing.T) {
			n, src := mustParseCode(`
				{
					on received %{} fn(){}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

	})

	t.Run("pattern definition", func(t *testing.T) {
		t.Run("redeclaration", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = 0
				pattern p = 1
			`)
			def := ast.FindNodes(n, (*ast.PatternDefinition)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.FmtPatternAlreadyDeclared("p")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should be a top-levle statement", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){
					pattern p = 0
				}
			`)
			def := ast.FindNode(n, (*ast.PatternDefinition)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_DEF_NOT_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definitions are not allowed after a call to a function declared below", func(t *testing.T) {
			n, src := mustParseCode(`
				f()
				
				pattern p = 0

				fn f(){}
			`)
			def := ast.FindNode(n, (*ast.PatternDefinition)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_DEF_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definition are not allowed after a function definition", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}
				
				pattern p = 0
			`)
			def := ast.FindNode(n, (*ast.PatternDefinition)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_DEF_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("pattern namespace definition", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			n, src := mustParseCode(`
				pnamespace p. = {a: %(1)}

				%p.a
				%p.b
			`)
			namespaceMemberExprs := ast.FindNodes(n, (*ast.PatternNamespaceMemberExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})

			expectedErr := utils.CombineErrors(
				makeError(namespaceMemberExprs[1].MemberName, src, text.FmtPatternNamespaceDoesNotHaveMember("p", "b")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("redeclaration", func(t *testing.T) {
			n, src := mustParseCode(`
				pnamespace p. = {}
				pnamespace p. = {}
			`)
			def := ast.FindNodes(n, (*ast.PatternNamespaceDefinition)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.FmtPatternNamespaceAlreadyDeclared("p")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should be a top-level statement", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){
					pnamespace p. = {}
				}
			`)
			def := ast.FindNode(n, (*ast.PatternNamespaceDefinition)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_NS_DEF_NOT_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definitions are not allowed after a call to a function declared below", func(t *testing.T) {
			n, src := mustParseCode(`
				f()
				
				pnamespace p. = {}

				fn f(){}
			`)
			def := ast.FindNode(n, (*ast.PatternNamespaceDefinition)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_NS_DEF_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definitions are not allowed after a reference (identifier) to a function declared below", func(t *testing.T) {
			n, src := mustParseCode(`
				f
				
				pnamespace p. = {}

				fn f(){}
			`)
			def := ast.FindNode(n, (*ast.PatternNamespaceDefinition)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_NS_DEF_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definitions are not allowed after a reference (global variable node) to a function declared below", func(t *testing.T) {
			n, src := mustParseCode(`
				$f
				
				pnamespace p. = {}

				fn f(){}
			`)
			def := ast.FindNode(n, (*ast.PatternNamespaceDefinition)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_NS_DEF_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("definition are not allowed after a function definition", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){}
				
				pnamespace p. = {}
			`)
			def := ast.FindNode(n, (*ast.PatternNamespaceDefinition)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_PATTERN_NS_DEF_AFTER_FN_DECL_OR_REF_TO_FN),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("pattern identifier", func(t *testing.T) {

		t.Run("not declared", func(t *testing.T) {
			n, src := mustParseCode(`
				%p
			`)
			pattern := ast.FindNode(n, (*ast.PatternIdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(pattern, src, text.FmtPatternIsNotDeclared("p")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("not declared pattern in lazy pattern definition", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = @ str( s )
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("otherprops(no)", func(t *testing.T) {
			n, src := mustParseCode(`
				%{
					otherprops(no)
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("readonly pattern", func(t *testing.T) {

		t.Run("as type of function parameter", func(t *testing.T) {
			n, src := mustParseCode(`fn f(arg readonly int){}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("as type of function pattern parameter", func(t *testing.T) {
			n, src := mustParseCode(`%fn(arg readonly int)`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("should be the type of a function parameter", func(t *testing.T) {
			n, src := mustParseCode(`pattern p = readonly {}`)

			expr := ast.FindNode(n, (*ast.ReadonlyPatternExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(expr, src, text.MISPLACED_READONLY_PATTERN_EXPRESSION),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("quantity literal", func(t *testing.T) {

		testCases := []struct {
			input  string
			errors []string
		}{
			{"1x", nil},
			{"1s", nil},
			{"1h", nil},
			{"1h1s", nil},
			{"1h1s5ms10us15ns", nil},
			//
			{"-1s", []string{ErrNegQuantityNotSupported.Error()}},
			//{"1o1s", []string{text.INVALID_QUANTITY}},
			//{"1o2h", []string{text.INVALID_QUANTITY}},
			{"1s1x", []string{text.INVALID_QUANTITY}},
			{"1s1h", []string{text.INVALID_QUANTITY}},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, src := mustParseCode(testCase.input)
				lit := ast.FindNode(n, (*ast.QuantityLiteral)(nil), nil)
				err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})

				if len(testCase.errors) == 0 {
					assert.NoError(t, err)
				} else {
					var checkingErrs []error
					for _, err := range testCase.errors {
						checkingErrs = append(checkingErrs, makeError(lit, src, err))
					}
					expectedErr := utils.CombineErrors(checkingErrs...)
					assert.Equal(t, expectedErr, err)
				}
			})
		}

	})

	t.Run("rate literal", func(t *testing.T) {

		testCases := []struct {
			input  string
			errors []string
		}{

			{"1x/s", nil},
			{"1x/h", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			{"1s/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			{"1h/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			{"1h1s/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			{"1h1s5ms10us15ns/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			//
			{"1x1s/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			{"1x2h/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			{"1s1x/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
			{"1s1h/s", []string{text.INVALID_RATE, text.INVALID_QUANTITY}},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, src := mustParseCode(testCase.input)
				lit := ast.FindNode(n, (*ast.RateLiteral)(nil), nil)

				err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})

				if len(testCase.errors) == 0 {
					assert.NoError(t, err)
				} else {
					var checkingErrs []error
					for _, err := range testCase.errors {
						checkingErrs = append(checkingErrs, makeError(lit, src, err))
					}
					expectedErr := utils.CombineErrors(checkingErrs...)
					assert.Equal(t, expectedErr, err)
				}
			})

			///////////////////
			break
		}

	})

	t.Run("integer range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, src := mustParseCode(`1..2`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("no upper bound", func(t *testing.T) {
			n, src := mustParseCode(`1..`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("upper bound should be smaller than lower bound", func(t *testing.T) {
			n, src := mustParseCode(`1..0`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(n.Statements[0], src, text.LOWER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("float range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, src := mustParseCode(`1.0..2.0`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("no upper bound", func(t *testing.T) {
			n, src := mustParseCode(`1.0..`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("upper bound should be smaller than lower bound", func(t *testing.T) {
			n, src := mustParseCode(`1.0..0.0`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(n.Statements[0], src, text.LOWER_BOUND_OF_FLOAT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("quantity range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, src := mustParseCode(`1x..2x`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("no upper bound", func(t *testing.T) {
			n, src := mustParseCode(`1x..`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("switch statement", func(t *testing.T) {

		t.Run("variables defined inside cases are not accessible after the statement", func(t *testing.T) {
			n, src := mustParseCode(`
				switch 1 {
					0 {
						a = 1
					}
					defaultcase {
						b = 2
					}
				}
				a
				b
			`)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(n.Statements[1], src, text.FmtVarIsNotDeclared("a")),
				makeError(n.Statements[2], src, text.FmtVarIsNotDeclared("b")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("match statement", func(t *testing.T) {
		t.Run("group matching variable shadows a global", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar m = 1
				match 1 {
					%/{:a} m { }
				}
			`)
			variable := ast.FindNode(n, (*ast.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.FmtCannotShadowGlobalVariable("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("group matching variable shadows a local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				m = 1
				match 1 {
					%/{:a} m { }
				}
			`)
			variable := ast.FindNode(n, (*ast.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.FmtCannotShadowLocalVariable("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("group matching variables with same name", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					%/{:a} m { }
					%/a/{:a} m { }
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("group matching variable is not accessible after match statement", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					%/{:a} m { }
				}
				return m
			`)
			variable := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.FmtVarIsNotDeclared("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("variables defined inside cases are not accessible after the statement", func(t *testing.T) {
			n, src := mustParseCode(`
				match 1 {
					0 {
						a = 1
					}
					defaultcase {
						b = 2
					}
				}
				a
				b
			`)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(n.Statements[1], src, text.FmtVarIsNotDeclared("a")),
				makeError(n.Statements[2], src, text.FmtVarIsNotDeclared("b")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("markup element", func(t *testing.T) {

		t.Run("no variable used in elements", func(t *testing.T) {
			n, src := mustParseCode(`html<div a=1></div>`)

			globals := GlobalVariablesFromMap(map[string]Value{"html": Nil}, nil)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals}))
		})

		t.Run("variable used in elements", func(t *testing.T) {
			n, src := mustParseCode(`html<div a=b></div>`)

			globals := GlobalVariablesFromMap(map[string]Value{"html": Nil}, nil)
			variable := ast.FindNodes(n, (*ast.IdentifierLiteral)(nil), nil)[3]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.FmtVarIsNotDeclared("b")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("markup pattern", func(t *testing.T) {
		t.Run("no quantifiers, attributes nor wildcards", func(t *testing.T) {
			n, src := mustParseCode(`%<div></div>`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("quantifier", func(t *testing.T) {
			n, src := mustParseCode(`%<div+></div>`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("wildcard", func(t *testing.T) {
			n, src := mustParseCode(`%<div>*</div>`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("attribute", func(t *testing.T) {
			n, src := mustParseCode(`%<div a=int>*</div>`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}, "bool": {}},
			}))
		})

		t.Run("attribute without a type (no =)", func(t *testing.T) {
			n, src := mustParseCode(`%<div a>*</div>`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("attribute with missing type after =", func(t *testing.T) {
			n, src, _ := parseCode(`%<div a=>*</div>`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("named patterns are allowed as patterns for attributes and in interpolations", func(t *testing.T) {
			n, src := mustParseCode(`%<div a=int>{int}</div>`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("pattern members are allowed as pattern for attributes and in interpolations", func(t *testing.T) {
			n, src := mustParseCode(`%<div a=mynamespace.int>{mynamespace.int}</div>`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				PatternNamespaces: map[string][]string{
					"mynamespace": {"int"},
				},
			}))
		})

		t.Run("named patterns are allowed as pattern for attributes and in interpolations", func(t *testing.T) {
			n, src := mustParseCode(`%<div a=int>{int}</div>`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("variables are allowed as pattern for attributes and in interpolations", func(t *testing.T) {
			n, src := mustParseCode(`
				$int_pattern = %int
				%<div a=$int_pattern>{$int_pattern}</div>
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("member expressions are allowed as pattern for attributes and in interpolations", func(t *testing.T) {
			n, src := mustParseCode(`
				$patterns = {
					int: %int
				}
				%<div a=$patterns.int>{$patterns.int}</div>
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("forbidden node as pattern for attributes", func(t *testing.T) {
			n, src := mustParseCode(`%<div a=mypattern()>{mypattern()}</div>`)
			patternCalls := ast.FindNodes(n, (*ast.PatternCallExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				Patterns: map[string]struct{}{
					"mypattern": {},
				},
			})
			expectedErr := utils.CombineErrors(
				makeError(patternCalls[0], src, text.ONLY_X_ARE_SUPPORTED_AS_PATTERNS_FOR_MARKUP_PATTERN_ATTRIBUTES),
				makeError(patternCalls[1], src, text.ONLY_X_ARE_SUPPORTED_IN_MARKUP_PATTERN_INTERPOLATIONS),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("extend statement", func(t *testing.T) {
		t.Run("should be located at the top level: in function declaration", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = {a: 1}
				fn f(){
					extend p {}
				}
			`)

			globals := GlobalVariablesFromMap(map[string]Value{}, nil)
			extendStmt := ast.FindNode(n, (*ast.ExtendStatement)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := utils.CombineErrors(
				makeError(extendStmt, src, text.MISPLACED_EXTEND_STATEMENT_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should be located at the top level: in if statement's block", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = {a: 1}
				if true {
					extend p {}
				}
			`)

			globals := GlobalVariablesFromMap(map[string]Value{}, nil)
			extendStmt := ast.FindNode(n, (*ast.ExtendStatement)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := utils.CombineErrors(
				makeError(extendStmt, src, text.MISPLACED_EXTEND_STATEMENT_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in property expressions: identifier referring to a global variable", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = {a: 1}
				globalvar a = 1
				extend p {
					b: a
				}
			`)

			globals := GlobalVariablesFromMap(map[string]Value{}, nil)
			extendStmt := ast.FindNode(n, (*ast.ExtendStatement)(nil), nil)
			ident := ast.FindIdentWithName(extendStmt, "a")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in property expressions: identifier referring to a local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = {a: 1}
				a = 1
				extend p {
					b: a
				}
			`)

			globals := GlobalVariablesFromMap(map[string]Value{}, nil)
			extendStmt := ast.FindNode(n, (*ast.ExtendStatement)(nil), nil)
			ident := ast.FindIdentWithName(extendStmt, "a")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in property expressions: global variable", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = {a: 1}
				globalvar a = 1
				extend p {
					b: $a
				}
			`)

			globals := GlobalVariablesFromMap(map[string]Value{}, nil)
			extendStmt := ast.FindNode(n, (*ast.ExtendStatement)(nil), nil)
			variable := ast.FindNode(extendStmt, (*ast.Variable)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in property expressions: local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				pattern p = {a: 1}
				a = 1
				extend p {
					b: $a
				}
			`)

			globals := GlobalVariablesFromMap(map[string]Value{}, nil)
			extendStmt := ast.FindNode(n, (*ast.ExtendStatement)(nil), nil)
			variable := ast.FindLocalVarWithName(extendStmt, "a")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.VARS_NOT_ALLOWED_IN_PATTERN_AND_EXTENSION_OBJECT_PROPERTIES),
			)
			assert.Equal(t, expectedErr, err)
		})
	})
	t.Run("struct definition statement", func(t *testing.T) {
		t.Run("should be located at the top level: in function declaration", func(t *testing.T) {
			n, src := mustParseCode(`
				fn f(){
					struct MyStruct {}
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_STRUCT_DEF_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should be located at the top level: in if statement's block", func(t *testing.T) {
			n, src := mustParseCode(`
				if true {
					struct MyStruct {}
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(def, src, text.MISPLACED_STRUCT_DEF_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in field definitions: identifier referring to a global variable", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				struct MyStruct {
					value %(a)
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)
			ident := ast.FindIdentWithName(def, "a")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in field definitions: identifier referring to a local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				$a = 1
				struct MyStruct {
					value %(a)
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)
			ident := ast.FindIdentWithName(def, "a")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ident, src, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in field definitions: global variable", func(t *testing.T) {
			n, src := mustParseCode(`
				globalvar a = 1
				struct MyStruct {
					value %($a)
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)
			variable := ast.FindNode(def, (*ast.Variable)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have variables in field definitions: local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				$a = 1
				struct MyStruct {
					value %($a)
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)
			variable := ast.FindLocalVarWithName(def, "a")

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(variable, src, text.VARS_CANNOT_BE_USED_IN_STRUCT_FIELD_DEFS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have references to self in field definitions", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {
					value %(self)
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)
			selfExpr := ast.FindNode(def, (*ast.SelfExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(selfExpr, src, text.SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("should not have sendval expressions in methods", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {
					fn f(){
						sendval 1 to {}
					}
				}
			`)

			def := ast.FindNode(n, (*ast.StructDefinition)(nil), nil)
			sendValExpr := ast.FindNode(def, (*ast.SendValueExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(sendValExpr, src, text.MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("can have references to self in methods", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {
					fn f(){
						self
					}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate definition", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {

				}
				struct MyStruct {

				}
			`)

			duplicateDef := ast.FindNodes(n, (*ast.StructDefinition)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(duplicateDef.Name, src, text.FmtInvalidStructDefAlreadyDeclared("MyStruct")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate definition, first definition in included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				struct MyStruct {

				}
			`, map[string]string{"./dep.ix": "includable-file\n struct MyStruct {}"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			duplicateDef := ast.FindNode(mod.MainChunk.Node, (*ast.StructDefinition)(nil), nil)

			expectedErr := utils.CombineErrors(
				makeError(duplicateDef.Name, mod.MainChunk, text.FmtInvalidStructDefAlreadyDeclared("MyStruct")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate definition, first definition in included chunk, import after definition", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				# The duplicate definition error should be located here 
				# even if the definition is before the import.
				struct MyStruct { 

				}
				import ./dep.ix
			`, map[string]string{"./dep.ix": "includable-file\n struct MyStruct {}"})

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			duplicateDef := ast.FindNode(mod.MainChunk.Node, (*ast.StructDefinition)(nil), nil)

			expectedErr := utils.CombineErrors(
				makeError(duplicateDef.Name, mod.MainChunk, text.FmtInvalidStructDefAlreadyDeclared("MyStruct")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("same definition in embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {}
				go do {
					struct MyStruct {}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate field definition", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {
					a int
					a bool
				}
			`)

			secondStructDef := ast.FindNodes(n, (*ast.StructFieldDefinition)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}, "bool": {}},
			})
			expectedErr := utils.CombineErrors(
				makeError(secondStructDef.Name, src, text.FmtAnXFieldOrMethodIsAlreadyDefined("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate method definition", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {
					fn m(){

					}

					fn m(){

					}
				}
			`)

			secondMethodDecl := ast.FindNodes(n, (*ast.FunctionDeclaration)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})

			expectedErr := utils.CombineErrors(
				makeError(secondMethodDecl.Name, src, text.FmtAnXFieldOrMethodIsAlreadyDefined("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("method definition with name of field", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {
					m int
					fn m(){}
				}
			`)

			methodDecl := ast.FindNode(n, (*ast.FunctionDeclaration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			})

			expectedErr := utils.CombineErrors(
				makeError(methodDecl.Name, src, text.FmtAnXFieldOrMethodIsAlreadyDefined("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("field definition with name of method", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct {
					fn m(){}
					m int
				}
			`)

			fieldDef := ast.FindNode(n, (*ast.StructFieldDefinition)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			})

			expectedErr := utils.CombineErrors(
				makeError(fieldDef.Name, src, text.FmtAnXFieldOrMethodIsAlreadyDefined("m")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("new expression", func(t *testing.T) {
		t.Run("defined struct type", func(t *testing.T) {
			n, src := mustParseCode(`
				struct Lexer {}
				lexer = new Lexer
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("before struct type definition", func(t *testing.T) {
			n, src := mustParseCode(`
				lexer = new Lexer
				struct Lexer {}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				go do {
					struct Lexer {}
					lexer = new Lexer
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("initialization", func(t *testing.T) {
			n, src := mustParseCode(`
				struct Lexer {}
				lexer = new Lexer {index: 0}
			`)

			globals := GlobalVariablesFromMap(map[string]Value{}, nil)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals}))
		})

		t.Run("duplicate field in initialization", func(t *testing.T) {
			n, src := mustParseCode(`
				struct Lexer {}
				lexer = new Lexer {index: 0, index: 1}
			`)

			inits := ast.FindNodes(n, (*ast.StructFieldInitialization)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(inits[1].Name, src, text.FmtDuplicateFieldName("index")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("undefined struct type", func(t *testing.T) {
			n, src := mustParseCode(`
				lexer = new Lexer
			`)

			newExpr := ast.FindNode(n, (*ast.NewExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(newExpr.Type, src, text.FmtStructTypeIsNotDefined("Lexer")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("undefined struct type in an embedded module", func(t *testing.T) {
			n, src := mustParseCode(`
				struct Lexer {}
				go do {
					lexer = new Lexer
				}
			`)

			newExpr := ast.FindNode(n, (*ast.NewExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(newExpr.Type, src, text.FmtStructTypeIsNotDefined("Lexer")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("struct pointer type", func(t *testing.T) {
		t.Run("parameter, struct pointer", func(t *testing.T) {
			n, src := mustParseCode(`
				struct Int { value int }
				fn ptr(i *Int){}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("struct pointer with undefined struct type", func(t *testing.T) {
			n, src := mustParseCode(`
				fn ptr(i *Int){}
			`)

			ptrType := ast.FindNode(n, (*ast.PointerType)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ptrType.ValueType, src, text.FmtStructTypeIsNotDefined("Int")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("as return type", func(t *testing.T) {
			n, src := mustParseCode(`
				struct Int { value int }
				fn ptr() *Int {

				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]struct{}{"int": {}},
			}))
		})

		t.Run("not allowed in patterns", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct { }
				%{a: *MyStruct}
			`)

			ptrType := ast.FindNode(n, (*ast.PointerType)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ptrType, src, text.MISPLACED_POINTER_TYPE),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("allowed as declaration type of local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct { }
				var s *MyStruct = nil
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("not allowed as declaration type of global variable", func(t *testing.T) {
			n, src, _ := parseCode(`
				struct MyStruct { }
				globalvar s *MyStruct
			`)

			pointerType := ast.FindNode(n, (*ast.PointerType)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(pointerType, src, text.MISPLACED_POINTER_TYPE),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("builtin pointer type", func(t *testing.T) {
		t.Run("parameter, struct pointer", func(t *testing.T) {
			n, src := mustParseCode(`
				fn ptr(i *int){}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
			}))
		})

		t.Run("as return type", func(t *testing.T) {
			n, src := mustParseCode(`
				fn ptr() *int {

				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
			}))
		})

		t.Run("not allowed in patterns", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct { }
				%{a: *int}
			`)

			ptrType := ast.FindNode(n, (*ast.PointerType)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(ptrType, src, text.MISPLACED_POINTER_TYPE),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("allowed as declaration type of local variable", func(t *testing.T) {
			n, src := mustParseCode(`
				var s *int = nil
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("not allowed as declaration type of global variable", func(t *testing.T) {
			n, src, _ := parseCode(`
				globalvar s *int
			`)

			pointerType := ast.FindNode(n, (*ast.PointerType)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(pointerType, src, text.MISPLACED_POINTER_TYPE),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("dereference expression", func(t *testing.T) {
		n, src := mustParseCode(`
			fn ptr(i *Int){
				val = *i
			}
		`)

		err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
		assert.Error(t, err)
	})

	t.Run("struct type name", func(t *testing.T) {

		t.Run("not allowed in patterns", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct { }
				%{a: MyStruct}
			`)

			patternIdentLiteral := ast.FindNodes(n, (*ast.PatternIdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(patternIdentLiteral, src, text.MISPLACED_STRUCT_TYPE_NAME),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("not allowed as declaration type of local variable", func(t *testing.T) {
			n, src, _ := parseCode(`
				struct MyStruct { }
				var s MyStruct
			`)

			patternIdentLiteral := ast.FindNodes(n, (*ast.PatternIdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(patternIdentLiteral, src, text.MISPLACED_STRUCT_TYPE_NAME),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("not allowed as declaration type of global variable", func(t *testing.T) {
			n, src, _ := parseCode(`
				struct MyStruct { }
				var s MyStruct
			`)

			patternIdentLiteral := ast.FindNodes(n, (*ast.PatternIdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(patternIdentLiteral, src, text.MISPLACED_STRUCT_TYPE_NAME),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("not allowed as parameter type", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct { }
				fn f(s MyStruct){}
			`)

			patternIdentLiteral := ast.FindNodes(n, (*ast.PatternIdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(patternIdentLiteral, src, text.STRUCT_TYPES_NOT_ALLOWED_AS_PARAMETER_TYPES),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("not allowed as return type", func(t *testing.T) {
			n, src := mustParseCode(`
				struct MyStruct { }
				fn f() MyStruct {}
			`)

			patternIdentLiteral := ast.FindNodes(n, (*ast.PatternIdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := utils.CombineErrors(
				makeError(patternIdentLiteral, src, text.STRUCT_TYPES_NOT_ALLOWED_AS_RETURN_TYPES),
			)
			assert.Equal(t, expectedErr, err)
		})
	})
}

//TODO: add tests for static checking of remaining manifest sections.

func TestCheckPreinitFilesObject(t *testing.T) {

	parseObject := func(s string) *ast.ObjectLiteral {
		return parse.MustParseChunk(s).Statements[0].(*ast.ObjectLiteral)
	}

	t.Run("empty", func(t *testing.T) {
		objLiteral := parseObject("{}")

		CheckPreinitFilesObject(objLiteral, func(n ast.Node, msg string) {
			assert.Fail(t, msg)
		})
	})

	t.Run("single file with correct description", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				FILE: {
					path: /file.txt
					pattern: %str
				}
			}
		`)

		CheckPreinitFilesObject(objLiteral, func(n ast.Node, msg string) {
			assert.Fail(t, msg)
		})
	})

	t.Run("single file with invalid .path", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				FILE: {
					path: {}
					pattern: %str
				}
			}
		`)

		err := false

		CheckPreinitFilesObject(objLiteral, func(n ast.Node, msg string) {
			err = true
			assert.Equal(t, text.PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH, msg)
		})
		assert.True(t, err)
	})

	t.Run("single file with relative .path", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				FILE: {
					path: ./file.txt
					pattern: %str
				}
			}
		`)

		err := false

		CheckPreinitFilesObject(objLiteral, func(n ast.Node, msg string) {
			err = true
			assert.Equal(t, text.PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH, msg)
		})
		assert.True(t, err)
	})
}

type StaticCheckInput = core.StaticCheckInput
type StaticCheckError = staticcheck.Error
type Pattern = core.Pattern
type PatternNamespace = core.PatternNamespace
type FunctionStaticData = staticcheck.FunctionData
type MappingStaticData = staticcheck.MappingData

type GlobalState = core.GlobalState
type Context = core.Context
type ContextConfig = core.ContextConfig

type Value = core.Value
type Int = core.Int
type Rune = core.Rune
type URL = core.URL
type Host = core.Host
type String = core.String
type PathPattern = core.PathPattern

type GoFunction = core.GoFunction

type Module = core.Module
type ModuleParsingConfig = core.ModuleParsingConfig
type Permission = core.Permission
type FilesystemPermission = core.FilesystemPermission

type PrettyPrintConfig = core.PrettyPrintConfig
type ReprConfig = core.ReprConfig
type JSONSerializationConfig = core.JSONSerializationConfig

var (
	StaticCheck            = core.StaticCheck
	NewContext             = core.NewContext
	NewGlobalState         = core.NewGlobalState
	GlobalVariablesFromMap = core.GlobalVariablesFromMap
	NewFunctionStaticData  = staticcheck.NewFunctionStaticData
	NewMappingStaticData   = staticcheck.NewMappingStaticData
	ParseLocalModule       = core.ParseLocalModule
	NewStaticCheckError    = staticcheck.NewError
	NewNamespace           = core.NewNamespace
	WrapGoFunction         = core.WrapGoFunction
	WrapGoMethod           = core.WrapGoMethod
	ValOf                  = core.ValOf
	MapIterable            = core.MapIterable
	NewWrappedValueList    = core.NewWrappedValueList

	CheckPreinitFilesObject = staticcheck.CheckPreinitFilesObject

	STR_PATTERN                = core.STR_PATTERN
	INT_PATTERN                = core.INT_PATTERN
	BOOL_PATTERN               = core.BOOL_PATTERN
	OBJECT_PATTERN             = core.OBJECT_PATTERN
	MAX_NAME_BYTE_LEN          = staticcheck.MAX_NAME_BYTE_LEN
	DEFAULT_PATTERN_NAMESPACES = core.DEFAULT_PATTERN_NAMESPACES

	ErrNegQuantityNotSupported     = core.ErrNegQuantityNotSupported
	ErrCannotAddIrreversibleEffect = core.ErrCannotAddIrreversibleEffect
	ErrCannotSetProp               = core.ErrCannotSetProp
	ErrNotClonable                 = core.ErrNotClonable

	TestSuiteModule = core.TestSuiteModule

	Nil = core.Nil

	FormatErrPropertyDoesNotExist = core.FormatErrPropertyDoesNotExist
)

// writeModuleAndIncludedFiles write a module & it's included files in a temporary directory on the OS filesystem.
func writeModuleAndIncludedFiles(t *testing.T, mod string, modContent string, dependencies map[string]string) string {
	dir := t.TempDir()
	modPath := filepath.Join(dir, mod)

	assert.NoError(t, fsutils.WriteFileSync(modPath, []byte(modContent), 0o400))

	for name, content := range dependencies {
		assert.NoError(t, fsutils.WriteFileSync(filepath.Join(dir, name), []byte(content), 0o400))
	}

	return modPath
}

func createParsingContext(modpath string) *core.Context {
	pathPattern := PathPattern(core.Path(modpath).DirPath() + "...")
	return core.NewContextWithEmptyState(ContextConfig{
		Permissions: []Permission{core.CreateFsReadPerm(pathPattern)},
	}, nil)
}
