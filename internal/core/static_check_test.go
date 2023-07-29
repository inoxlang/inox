package core

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	jsoniter "github.com/json-iterator/go"

	"github.com/stretchr/testify/assert"
)

func TestCheck(t *testing.T) {

	parseCode := func(code string) (*parse.Chunk, *parse.ParsedChunk) {
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: code,
		}))

		return chunk.Node, chunk
	}

	makeError := func(node parse.Node, chunk *parse.ParsedChunk, s string) *StaticCheckError {
		return NewStaticCheckError(s, parse.SourcePositionStack{chunk.GetSourcePosition(node.Base().Span)})
	}

	staticCheckNoData := func(input StaticCheckInput) error {
		_, err := StaticCheck(input)
		return err
	}

	t.Run("object literal", func(t *testing.T) {
		t.Run("two implict keys", func(t *testing.T) {
			n, src := parseCode(`{1, 2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("explicit identifier keys", func(t *testing.T) {
			n, src := parseCode(`{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys (one implicit, one explicit)", func(t *testing.T) {
			n, src := parseCode(`{1, "0": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtObjLitExplicityDeclaresPropWithImplicitKey("0")),
			)
			assert.Equal(t, expectedErr, err)

			n, src = parseCode(`{"0": 1, 1}`)
			err = staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr = combineErrors(
				makeError(n, src, fmtObjLitExplicityDeclaresPropWithImplicitKey("0")),
			)
			assert.Error(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys (two string literals)", func(t *testing.T) {
			n, src := parseCode(`{"0":1, "0": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys (one identifier & one string)", func(t *testing.T) {
			n, src := parseCode(`{a:1, "a": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys (one string & one identifier)", func(t *testing.T) {
			n, src := parseCode(`{a:1, "a": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys (two identifiers)", func(t *testing.T) {
			n, src := parseCode(`{a:1, "a": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys : one of the key is in an expanded object", func(t *testing.T) {
			n, src := parseCode(`
				e = {a: 1}
				{"a": 1, ... $e.{a}}
			`)
			keyNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := strings.Replace(`{"a":1}`, "a", name, 1)
			n, src := parseCode(code)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("regular property having a metaproperty key", func(t *testing.T) {
			n, src := parseCode(`{_url_: https://example.com/}`)
			keyNode := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty initialization : undefined variable in block", func(t *testing.T) {
			n, src := parseCode(`{ _url_ {a} }`)
			varNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty initialization : local variables in the scope surrounding the object are not accessible from the block", func(t *testing.T) {
			n, src := parseCode(`
				a = 1 
				{ _url_ {a} }
			`)
			varNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: missing description", func(t *testing.T) {
			n, src := parseCode(`{ _visibility_ {} }`)
			init := parse.FindNode(n, (*parse.InitializationBlock)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(init, src, INVALID_VISIB_INIT_BLOCK_SHOULD_CONT_OBJ),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: description should not have metaproperties", func(t *testing.T) {
			n, src := parseCode(`{ _visibility_ { { _url_ {} } } }`)
			innerObj := parse.FindNodes(n, (*parse.ObjectLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(innerObj, src, INVALID_VISIB_DESC_SHOULDNT_HAVE_METAPROPS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: description should not have implicit keys", func(t *testing.T) {
			n, src := parseCode(`{ _visibility_ { {1} } }`)
			innerObj := parse.FindNodes(n, (*parse.ObjectLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(innerObj, src, INVALID_VISIB_DESC_SHOULDNT_HAVE_IMPLICIT_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: description should not have have invalid keys", func(t *testing.T) {
			n, src := parseCode(`{ _visibility_ { {a: 1} } }`)
			prop := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(prop, src, INVALID_VISIBILITY_DESC_KEY),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: .public should have a key list literal as value", func(t *testing.T) {
			n, src := parseCode(`{ _visibility_ { {public: 1} } }`)
			publicProp := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(publicProp, src, VAL_SHOULD_BE_KEYLIST_LIT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: .visible_by should have a dict literal as value", func(t *testing.T) {
			n, src := parseCode(`{ _visibility_ { {visible_by: 1} } }`)
			publicProp := parse.FindNode(n, (*parse.ObjectProperty)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(publicProp, src, VAL_SHOULD_BE_DICT_LIT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("visibility metaproperty initialization: .visible_by[#self] should have a ket list literal as value", func(t *testing.T) {
			n, src := parseCode(`{ 
				_visibility_ { 
					{visible_by: :{#self: 1} } 
				} 
			}`)
			dictEntry := parse.FindNode(n, (*parse.DictionaryEntry)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(dictEntry, src, VAL_SHOULD_BE_KEYLIST_LIT),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("record literal", func(t *testing.T) {
		t.Run("two implict keys", func(t *testing.T) {
			n, src := parseCode(`#{1, 2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("explicit identifier keys", func(t *testing.T) {
			n, src := parseCode(`#{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys (one implicit, one explicit)", func(t *testing.T) {
			n, src := parseCode(`#{1, "0": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtRecLitExplicityDeclaresPropWithImplicitKey("0")),
			)
			assert.Equal(t, expectedErr, err)

			n, src = parseCode(`#{"0": 1, 1}`)
			err = staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr = combineErrors(
				makeError(n, src, fmtRecLitExplicityDeclaresPropWithImplicitKey("0")),
			)
			assert.Error(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys", func(t *testing.T) {
			n, src := parseCode(`#{"0":1, "0": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate explicit keys : one of the key is in an expanded object", func(t *testing.T) {
			n, src := parseCode(`
				e = {a: 1}
				#{"a": 1, ... $e.{a}}
			`)
			keyNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := strings.Replace(`#{"a":1}`, "a", name, 1)
			n, src := parseCode(code)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty key", func(t *testing.T) {
			n, src := parseCode(`#{_url_: https://example.com/}`)
			keyNode := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("object pattern literal", func(t *testing.T) {
		t.Run("identifier keys", func(t *testing.T) {
			n, src := parseCode(`%{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := parseCode(`%{"0":1, "0": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := parseCode(`%p = %{a: 1}; %{a:1, ...(%p).{a}}`)

			keyNodes := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), func(l *parse.IdentifierLiteral) bool {
				return l.Name == "a"
			})
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNodes[2], src, fmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := strings.Replace(`%{"a":1}`, "a", name, 1)
			n, src := parseCode(code)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty key", func(t *testing.T) {
			n, src := parseCode(`%{_url_: https://example.com/}`)
			keyNode := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("record pattern literal", func(t *testing.T) {
		t.Run("identifier keys", func(t *testing.T) {
			n, src := parseCode(`%p = #{keyOne:1, keyTwo:2}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := parseCode(`%p = #{"0":1, "0": 1}`)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateKey("0")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate keys", func(t *testing.T) {
			n, src := parseCode(`%p = %{a: 1}; %e = #{a:1, ...(%p).{a}}`)

			keyNodes := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), func(l *parse.IdentifierLiteral) bool {
				return l.Name == "a"
			})
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNodes[2], src, fmtDuplicateKey("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("key is too long", func(t *testing.T) {
			name := strings.Repeat("a", MAX_NAME_BYTE_LEN+1)
			code := strings.Replace(`%p = #{"a":1}`, "a", name, 1)
			n, src := parseCode(code)

			keyNode := parse.FindNode(n, (*parse.QuotedStringLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtNameIsTooLong(name)),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("metaproperty key", func(t *testing.T) {
			n, src := parseCode(`%p = #{_url_: https://example.com/}`)
			keyNode := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, OBJ_REC_LIT_CANNOT_HAVE_METAPROP_KEYS),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("self expression", func(t *testing.T) {
		t.Run("in top level", func(t *testing.T) {
			n, src := parseCode(`self`)

			selfExpr := parse.FindNode(n, (*parse.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(selfExpr, src, SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("value of an object property", func(t *testing.T) {
			n, src := parseCode(`{a: self}`)

			selfExpr := parse.FindNode(n, (*parse.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(selfExpr, src, SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("in a function", func(t *testing.T) {
			n, src := parseCode(`fn() => self`)

			selfExpr := parse.FindNode(n, (*parse.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(selfExpr, src, SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("in a method", func(t *testing.T) {
			n, src := parseCode(`{f: fn() => self}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a metaproperty's initialization block", func(t *testing.T) {
			n, src := parseCode(`{ _url_ { self } }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a function that is a value of an object pattern", func(t *testing.T) {
			n, src := parseCode(`%{f: fn() => self}`)

			selfExpr := parse.FindNode(n, (*parse.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(selfExpr, src, SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("at top level of a lifetime job", func(t *testing.T) {
			n, src := parseCode(`
				lifetimejob #job for %{} { self }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a function expression in a reception handler expression", func(t *testing.T) {
			n, src := parseCode(`
				{
					on received %{} fn(event){
						self
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("at top level of an embedded module", func(t *testing.T) {
			n, src := parseCode(`go do { self }`)

			selfExpr := parse.FindNode(n, (*parse.SelfExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(selfExpr, src, SELF_ACCESSIBILITY_EXPLANATION),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("supersys expression", func(t *testing.T) {
		t.Run("in top level", func(t *testing.T) {
			n, src := parseCode(`supersys`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(n.Statements[0], src, MISPLACED_SUPERSYS_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("value of an object property", func(t *testing.T) {
			n, src := parseCode(`{a: supersys}`)

			supersysExpr := parse.FindNode(n, (*parse.SupersysExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(supersysExpr, src, MISPLACED_SUPERSYS_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("in a function", func(t *testing.T) {
			n, src := parseCode(`fn() => supersys`)

			supersysExpr := parse.FindNode(n, (*parse.SupersysExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(supersysExpr, src, MISPLACED_SUPERSYS_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("in a method", func(t *testing.T) {
			n, src := parseCode(`{f: fn() => supersys}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a metaproperty's initialization block", func(t *testing.T) {
			n, src := parseCode(`{ _url_ { supersys } }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a function that is a value of an object pattern", func(t *testing.T) {
			n, src := parseCode(`%{f: fn() => supersys}`)

			supersysExpr := parse.FindNode(n, (*parse.SupersysExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(supersysExpr, src, MISPLACED_SUPERSYS_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("at top level of a lifetime job", func(t *testing.T) {
			n, src := parseCode(`
				lifetimejob #job for %{} { supersys }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("at top level of an embedded module", func(t *testing.T) {
			n, src := parseCode(`go do { supersys }`)

			supersysExpr := parse.FindNode(n, (*parse.SupersysExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(supersysExpr, src, MISPLACED_SUPERSYS_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("sendval expression", func(t *testing.T) {
		t.Run("in top level", func(t *testing.T) {
			n, src := parseCode(`sendval 1 to {}`)

			sendValExpr := parse.FindNode(n, (*parse.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(sendValExpr, src, MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("value of an object property", func(t *testing.T) {
			n, src := parseCode(`{a: sendval 1 to {}}`)

			sendValExpr := parse.FindNode(n, (*parse.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(sendValExpr, src, MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("in a function", func(t *testing.T) {
			n, src := parseCode(`fn() => sendval 1 to {}`)

			sendValExpr := parse.FindNode(n, (*parse.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(sendValExpr, src, MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("in a method", func(t *testing.T) {
			n, src := parseCode(`{f: fn() => sendval 1 to {}}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a metaproperty's initialization block", func(t *testing.T) {
			n, src := parseCode(`{ _url_ { sendval 1 to {} } }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in a function that is a value of an object pattern", func(t *testing.T) {
			n, src := parseCode(`%{f: fn() => sendval 1 to {}}`)

			sendValExpr := parse.FindNode(n, (*parse.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(sendValExpr, src, MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("at top level of a lifetime job", func(t *testing.T) {
			n, src := parseCode(`
				lifetimejob #job for %{} { sendval 1 to {} }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("at top level of an embedded module", func(t *testing.T) {
			n, src := parseCode(`go do { sendval 1 to {} }`)

			sendValExpr := parse.FindNode(n, (*parse.SendValueExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(sendValExpr, src, MISPLACED_SENDVAL_EXPR),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("member expression", func(t *testing.T) {
		t.Run("existing property of self", func(t *testing.T) {
			n, src := parseCode(`{f: fn() => self.f}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("existing property of self due to a spread object", func(t *testing.T) {
			n, src := parseCode(`{
				f: fn() => self.name, 
				...({name: "foo"}).{name}
			}`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("non existing property of self", func(t *testing.T) {
			n, src := parseCode(`{f: fn() => self.b}`)

			membExpr := parse.FindNode(n, (*parse.MemberExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(membExpr, src, fmtObjectDoesNotHaveProp("b")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("compued member expression", func(t *testing.T) {
		t.Run("property name node is an undefined variable", func(t *testing.T) {
			n, src := parseCode(`
				a = {}
				a.(b)
			`)
			ident := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), func(ident *parse.IdentifierLiteral, _ bool) bool {
				return ident.Name == "b"
			})

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(ident, src, fmtVarIsNotDeclared("b")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("property name node is a defined variable", func(t *testing.T) {
			n, src := parseCode(`
				a = {}
				b = "a"
				a.(b)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("tuple literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			n, src := parseCode(`#[]`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
		t.Run("single & valid element", func(t *testing.T) {
			n, src := parseCode(`#[1]`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

	})

	t.Run("dictionary literal", func(t *testing.T) {
		t.Run("duplicate keys", func(t *testing.T) {
			n, src := parseCode(`:{./a:0, ./a:1}`)

			keyNode := parse.FindNodes(n, (*parse.RelativePathLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyNode, src, fmtDuplicateDictKey("./a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("spawn expression", func(t *testing.T) {
		t.Run("single call expression", func(t *testing.T) {
			n, src := parseCode(`
				fn f(){}
				go {} do f()
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("no additional provided globals (single call expression)", func(t *testing.T) {
			n, src := parseCode(`go {} do idt(a)`)
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
			n, src := parseCode(`go true do {
				return 1
			}`)

			boolLit := parse.FindNode(n, (*parse.BooleanLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(boolLit, src, INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("meta should be an object with no spread elements", func(t *testing.T) {
			n, src := parseCode(`obj = {a: 1}; go {...$obj.{a}} do {
				return 1
			}`)

			objLits := parse.FindNodes(n, (*parse.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(objLits[1], src, INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("meta should be an object with no implicit-key properties", func(t *testing.T) {
			n, src := parseCode(`go {1} do {
				return 1
			}`)

			objLit := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(objLit, src, INVALID_SPAWN_ONLY_OBJECT_LITERALS_WITH_NO_SPREAD_ELEMENTS_SUPPORTED),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("no additional provided globals", func(t *testing.T) {
			n, src := parseCode(`go {} do {
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
			n, src := parseCode(`
				$$global = 0
				go {globals: {global: global}} do {
					return global
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("description of globals should not contain spread elements", func(t *testing.T) {
			n, src := parseCode(`
				obj = {a: 1}
				$$global = 0
				go {globals: {global: global, ...$obj.{a}}} do {
					return global
				}
			`)
			objLit := parse.FindNode(n, (*parse.ObjectLiteral)(nil), func(lit *parse.ObjectLiteral, _ bool) bool {
				return len(lit.SpreadElements) > 0
			})
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(objLit, src, INVALID_SPAWN_GLOBALS_SHOULD_BE),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("description of globals should not contain implicit-key properties", func(t *testing.T) {
			n, src := parseCode(`
				$$global = 0
				go {globals: {global: global, 1}} do {
					return global
				}
			`)
			objLit := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(objLit, src, INVALID_SPAWN_GLOBALS_SHOULD_BE),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("global key list contains the name of a undefined global", func(t *testing.T) {
			n, src := parseCode(`
				go {globals: .{global}} do {
					return global
				}
			`)
			keyList := parse.FindNode(n, (*parse.KeyListExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(keyList, src, fmtCannotPassGlobalThatIsNotDeclaredToRoutine("global")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("mapping expression", func(t *testing.T) {
		t.Run("valid static entry", func(t *testing.T) {
			n, src := parseCode(`Mapping { 0 => 1 }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("static entry with invalid key", func(t *testing.T) {
			n, src := parseCode(`Mapping { ({}) => 1 }`)

			obj := parse.FindNode(n, (*parse.ObjectLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(obj, src, INVALID_MAPPING_ENTRY_KEY_ONLY_SIMPL_LITS_AND_PATT_IDENTS),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("static entry with pattern identifier key ", func(t *testing.T) {
			n, src := parseCode(`Mapping { %int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]Pattern{"int": INT_PATTERN},
			}))
		})

		t.Run("static entry with pattern namespace member key ", func(t *testing.T) {
			n, src := parseCode(`Mapping { %ns.int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				PatternNamespaces: map[string]*PatternNamespace{
					"ns": {
						Patterns: map[string]Pattern{
							"int": INT_PATTERN,
						},
					},
				},
			}))
		})

		t.Run("static key entries have access to globals", func(t *testing.T) {
			n, src := parseCode(`
				$$g = 1
				Mapping { %int => g }
			`)

			data, err := StaticCheck(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]Pattern{"int": INT_PATTERN},
			})

			assert.NoError(t, err)

			assert.Equal(t, map[*parse.MappingExpression]*MappingStaticData{
				parse.FindNode(n, (*parse.MappingExpression)(nil), nil): {referencedGlobals: []string{"g"}},
			}, data.mappingData)
		})

		t.Run("static key entries don't have access to locals", func(t *testing.T) {
			n, src := parseCode(`
				loc = 1
				Mapping { 0 => loc }
			`)

			ident := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(ident, src, fmtVarIsNotDeclared("loc")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("dynamic entry returning its key", func(t *testing.T) {
			n, src := parseCode(`Mapping { n 0 => n }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("dynamic entry returning its key and group matching result", func(t *testing.T) {
			n, src := parseCode(`Mapping { p %/{:name} m => [p, m] }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("dynamic entry with pattern identifier key ", func(t *testing.T) {
			n, src := parseCode(`Mapping { n %int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]Pattern{"int": INT_PATTERN},
			}))
		})

		t.Run("dynamic entry with pattern namespace member key ", func(t *testing.T) {
			n, src := parseCode(`Mapping { n %ns.int => 1 }`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				PatternNamespaces: map[string]*PatternNamespace{
					"ns": {
						Patterns: map[string]Pattern{
							"int": INT_PATTERN,
						},
					},
				},
			}))
		})

		t.Run("dynamic key entries have access to globals", func(t *testing.T) {
			n, src := parseCode(`
				$$g = 1
				Mapping { n %int => g }
			`)

			data, err := StaticCheck(StaticCheckInput{
				Node:     n,
				Chunk:    src,
				Patterns: map[string]Pattern{"int": INT_PATTERN},
			})

			assert.NoError(t, err)

			assert.Equal(t, map[*parse.MappingExpression]*MappingStaticData{
				parse.FindNode(n, (*parse.MappingExpression)(nil), nil): {referencedGlobals: []string{"g"}},
			}, data.mappingData)
		})
	})
	t.Run("compute expression", func(t *testing.T) {
		t.Run("in right side of dynamic mapping entry", func(t *testing.T) {
			n, src := parseCode(`Mapping { n 0 => comp 1 }`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in right side of static mapping entry", func(t *testing.T) {
			n, src := parseCode(`Mapping { 0 => comp 1 }`)

			computeExpr := parse.FindNode(n, (*parse.ComputeExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(computeExpr, src, MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("top level", func(t *testing.T) {
			n, src := parseCode(`comp 1`)

			computeExpr := parse.FindNode(n, (*parse.ComputeExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(computeExpr, src, MISPLACED_COMPUTE_EXPR_SHOULD_BE_IN_DYNAMIC_MAPPING_EXPR_ENTRY),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("captured variable does not exist", func(t *testing.T) {
			n, src := parseCode(`
				fn[a](){

				}
			`)
			fnExprNode := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fnExprNode, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable is not a local", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn[a](){}
			`)
			fnExprNode := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fnExprNode, src, fmtCannotPassGlobalToFunction("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable should be accessible in body", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				fn[a](){ return a }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("globals captured by function should be listed", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn(){ return a }
			`)

			fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, map[*parse.FunctionExpression]*FunctionStaticData{
				fnExpr: {capturedGlobals: []string{"a"}},
			}, data.fnData)
		})

		t.Run("globals referenced in lifetimejob expressions inside a function should be listed in the function's list", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn(){ 
					{
						lifetimejob #job {
							a
						}
					}
				}
			`)

			fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, &FunctionStaticData{
				capturedGlobals: []string{"a"},
			}, data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function B referenced by a function A should be listed in A's data", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn f(){
					return a
				}
				fn(){ return f }
			`)

			fnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[1]
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, &FunctionStaticData{
				capturedGlobals: []string{"f", "a"},
			}, data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function C referenced by a function B referenced by a function A should be listed in A's data", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn g(){
					return a
				}
				fn f(){
					return g
				}
				fn(){ return f }
			`)

			fnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[2]
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, &FunctionStaticData{
				capturedGlobals: []string{"f", "g", "a"},
			}, data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function B referenced by a method A should be listed in A's data", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn f(){
					return a
				}
				{
					m: fn(){ return f }
				}
			`)

			fnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[1]
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, &FunctionStaticData{
				capturedGlobals: []string{"f", "a"},
			}, data.GetFnData(fnExpr))
		})

		t.Run("a global captured by a global function C referenced by a function B referenced by a method A should be listed in A's data", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
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

			fnExpr := parse.FindNodes(n, (*parse.FunctionExpression)(nil), nil)[2]
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, &FunctionStaticData{
				capturedGlobals: []string{"f", "g", "a"},
			}, data.GetFnData(fnExpr))
		})

		t.Run("functions assigning a global should be detected", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn(){ $$a = 2 }
			`)

			fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, map[*parse.FunctionExpression]*FunctionStaticData{
				fnExpr: {assignGlobal: true},
			}, data.fnData)
		})

		t.Run("globals captured by function defined in spawn expression should be listed", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1

				go do {
					$$b = 1
					fn(){ return b }
				}
			`)

			fnExpr := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			data, err := StaticCheck(StaticCheckInput{Node: n, Chunk: src})
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, map[*parse.FunctionExpression]*FunctionStaticData{
				fnExpr: {capturedGlobals: []string{"b"}},
			}, data.fnData)
		})

	})

	t.Run("function declaration", func(t *testing.T) {

		t.Run("captured local variable does not exist", func(t *testing.T) {
			n, src := parseCode(`
				fn[a] f(){}
			`)
			fnDecl := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fnDecl, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured local variable is not a local", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn[a] f(){}
			`)
			fnDecl := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fnDecl, src, fmtCannotPassGlobalToFunction("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("parameter shadows a global", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn f(a){return a}
			`)
			fn := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fn.Parameters[0], src, fmtParameterCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable should be accessible in body", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				fn[a] f(){ return a }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("declaration in another function declaration", func(t *testing.T) {
			n, src := parseCode(`
				fn f(){
					fn g(){
	
					}
				}
			`)
			declNode := parse.FindNodes(n, (*parse.FunctionDeclaration)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(declNode, src, INVALID_FN_DECL_SHOULD_BE_TOP_LEVEL_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("function declared twice", func(t *testing.T) {
			n, src := parseCode(`
				fn f(){}
				fn f(){}
			`)
			declNode := parse.FindNodes(n, (*parse.FunctionDeclaration)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(declNode, src, fmtInvalidFnDeclAlreadyDeclared("f")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("function with same name in an embedded module", func(t *testing.T) {
			n, src := parseCode(`
				fn f(){}
	
				go do {
					fn f(){}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("function declaration with the same name as a global variable assignment", func(t *testing.T) {
			n, src := parseCode(`
				$$f = 0
	
				fn f(){}
			`)
			declNode := parse.FindNode(n, (*parse.FunctionDeclaration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(declNode, src, fmtInvalidFnDeclGlobVarExist("f")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("captured variable does not exist", func(t *testing.T) {
			n, src := parseCode(`
				fn[a](){

				}
			`)
			fnExprNode := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fnExprNode, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable is not a local", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				fn[a](){}
			`)
			fnExprNode := parse.FindNode(n, (*parse.FunctionExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fnExprNode, src, fmtCannotPassGlobalToFunction("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("captured variable should be accessible in body", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				fn[a](){ return a }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("function pattern expression", func(t *testing.T) {

		t.Run("parameter shadows a global", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				%fn(a){return a}
			`)
			fn := parse.FindNode(n, (*parse.FunctionPatternExpression)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(fn.Parameters[0], src, fmtParameterCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("local variable declaration", func(t *testing.T) {
		t.Run("declaration after assignment", func(t *testing.T) {
			n, src := parseCode(`
				a = 0
				var a = 0
			`)
			decl := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(decl, src, fmtInvalidLocalVarDeclAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("shadowing of global variable", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 0
				var a = 0
			`)
			decl := parse.FindNode(n, (*parse.LocalVariableDeclaration)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(decl, src, fmtCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("duplicate declarations", func(t *testing.T) {
			n, src := parseCode(`
				var a = 0
				var a = 1
			`)
			decl := parse.FindNodes(n, (*parse.LocalVariableDeclaration)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(decl, src, fmtInvalidLocalVarDeclAlreadyDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("assignment", func(t *testing.T) {
		t.Run("assignment with a function's name", func(t *testing.T) {
			n, src := parseCode(`
				fn f(){}
	
				$$f = 0
			`)
			assignment := parse.FindNodes(n, (*parse.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtInvalidGlobalVarAssignmentNameIsFuncName("f")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a constant in top level", func(t *testing.T) {
			n, src := parseCode(`
				const (
					a = 1
				)
	
				$$a = 0
			`)
			assignment := parse.FindNodes(n, (*parse.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtInvalidGlobalVarAssignmentNameIsConstant("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a constant in a function", func(t *testing.T) {
			n, src := parseCode(`
				const (
					a = 1
				)
	
				fn f(){
					$$a = 0
				}
			`)

			assignment := parse.FindNodes(n, (*parse.Assignment)(nil), nil)[0]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtInvalidGlobalVarAssignmentNameIsConstant("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("assignment of a global variable in embedded module: name of a global constant in parent module", func(t *testing.T) {
			n, src := parseCode(`
				const (
					a = 1
				)
	
				go do {
					$$a = 2
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global variable shadowing", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				a = 1
			`)

			assignment := parse.FindNodes(n, (*parse.Assignment)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("undefined global variable += assignment", func(t *testing.T) {
			n, src := parseCode(`
				$$a += 1
			`)

			assignment := parse.FindNode(n, (*parse.Assignment)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtInvalidGlobalVarAssignmentVarDoesNotExist("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("local variable shadowing", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				$$a = 1
			`)

			assignment := parse.FindNodes(n, (*parse.Assignment)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtCannotShadowLocalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("undefined local variable += assignment", func(t *testing.T) {
			n, src := parseCode(`
				a += 1
			`)

			assignment := parse.FindNode(n, (*parse.Assignment)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtInvalidVariableAssignmentVarDoesNotExist("a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("multi assignment", func(t *testing.T) {
		t.Run("global variable shadowing", func(t *testing.T) {
			n, src := parseCode(`
				$$a = 1
				assign a b = [1, 2]
			`)

			assignment := parse.FindNode(n, (*parse.MultiAssignment)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(assignment, src, fmtCannotShadowGlobalVariable("a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("global variable", func(t *testing.T) {
		t.Run("global is accessible in manifest", func(t *testing.T) {
			n, src := parseCode(`
				const (
					a = 1
				)
	
				manifest {
					limits: {
						"x": $$a
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global is accessible in module", func(t *testing.T) {
			n, src := parseCode(`
				const (
					a = 1
				)
	
				return $$a
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global is accessible in function", func(t *testing.T) {
			n, src := parseCode(`
				const (
					a = 1
				)
	
				fn f(){
					return $$a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("global variable defined by import statement", func(t *testing.T) {
			n, src := parseCode(`
				import result https://example.com/script.ix {}
				$$result
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("local variable", func(t *testing.T) {
		t.Run("local variable in a module : undefined", func(t *testing.T) {
			n, src := parseCode(`
				$a
			`)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(n.Statements[0], src, fmtLocalVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("local variable in a module : defined", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				$a
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("local variable in an embedded module : undefined", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				go do {
					$a
				}
			`)
			varNode := parse.FindNode(n, (*parse.Variable)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtLocalVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("local variable in a function : undefined", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				fn f(){
					$a
				}
			`)
			varNode := parse.FindNode(n, (*parse.Variable)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtLocalVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("local variable in a function : defined", func(t *testing.T) {
			n, src := parseCode(`
				fn f(){
					a = 1
					$a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("local variable in a lazy expression", func(t *testing.T) {
			n, src := parseCode(`
				@($a)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("argument variable in a function", func(t *testing.T) {
			n, src := parseCode(`
				fn f(a){
					$a
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("manifest", func(t *testing.T) {
		t.Run("invalid permission kind in manifest", func(t *testing.T) {
			n, src := parseCode(`
				manifest {
					permissions: {
						Read: %/...
					}
				}
			`)
			key := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), func(n *parse.IdentifierLiteral, unique bool) bool {
				return n.Name == "Read"
			})

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(key, src, fmtNotValidPermissionKindName("Read")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("inclusion import statement", func(t *testing.T) {
		t.Run("single included file with no dependecies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-chunk\n a = 1"})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			assert.NoError(t, err)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})

		t.Run("single included file with no dependecies: error in included file", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-chunk\n a = b"})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			expectedErr := combineErrors(
				NewStaticCheckError(fmtVarIsNotDeclared("b"), parse.SourcePositionStack{
					parse.SourcePositionRange{
						SourceName:  mod.MainChunk.Name(),
						StartLine:   3,
						StartColumn: 5,
					},
					parse.SourcePositionRange{
						SourceName:  mod.FlattenedIncludedChunkList[0].ParsedChunk.Name(),
						StartLine:   2,
						StartColumn: 6,
					},
				}),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("single included file with no dependecies: duplicate constant declaration", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				const a = 1
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-chunk\n const a = 2"})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			err = staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			})

			expectedErr := combineErrors(
				NewStaticCheckError(fmtCannotShadowGlobalVariable("a"), parse.SourcePositionStack{
					parse.SourcePositionRange{
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
					includable-chunk
					import ./dep1.ix
				`,
				"./dep1.ix": `
					includable-chunk
					a = 1
				`,
			})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
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
					includable-chunk
					a = 1
				`,
				"./dep2.ix": `
					includable-chunk
					b = 2
				`,
			})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			assert.NoError(t, err)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Module: mod,
				Node:   mod.MainChunk.Node,
				Chunk:  mod.MainChunk,
			}))
		})
	})

	t.Run("yield statement", func(t *testing.T) {
		t.Run("in embedded module", func(t *testing.T) {
			n, src := parseCode(`
				go do { yield }
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in function in embedded modue", func(t *testing.T) {
			n, src := parseCode(`
				go do { fn f(){ yield } }
			`)

			yieldStmt := parse.FindNode(n, (*parse.YieldStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(yieldStmt, src, MISPLACE_YIELD_STATEMENT_ONLY_ALLOWED_IN_EMBEDDED_MODULES),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("break statement", func(t *testing.T) {
		t.Run("direct child of a for statement", func(t *testing.T) {
			n, src := parseCode(`
				for i, e in [] {
					break
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an if statement in a for statement", func(t *testing.T) {
			n, src := parseCode(`
				for i, e in [] {
					if true {
						break
					}
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("in an switch statement in a for statement", func(t *testing.T) {
			n, src := parseCode(`
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
			n, src := parseCode(`
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
			n, src := parseCode(`
				break
			`)
			breakStmt := parse.FindNode(n, (*parse.BreakStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(breakStmt, src, INVALID_BREAK_OR_CONTINUE_STMT_SHOULD_BE_IN_A_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("direct child of an embedded module", func(t *testing.T) {
			n, src := parseCode(`
				go do {
					break
				}
			`)
			breakStmt := parse.FindNode(n, (*parse.BreakStatement)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(breakStmt, src, INVALID_BREAK_OR_CONTINUE_STMT_SHOULD_BE_IN_A_FOR_OR_WALK_STMT),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("call", func(t *testing.T) {
		t.Run("undefined callee", func(t *testing.T) {
			n, src := parseCode(`
				a 1
			`)
			varNode := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("for statement", func(t *testing.T) {
		t.Run("variables defined in for statement's head are not accessible after the statement", func(t *testing.T) {
			n, src := parseCode(`
				for file in files {
					
				}
				return file
			`)
			varNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtVarIsNotDeclared("file")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("variables defined in for statement's body are not accessible after the statement", func(t *testing.T) {
			n, src := parseCode(`
				for file in files {
					x = 3
				}
				return x
			`)
			varNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[3]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtVarIsNotDeclared("x")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("walk statement", func(t *testing.T) {
		t.Run("variables defined in walk statement's head are not accessible after the statement", func(t *testing.T) {
			n, src := parseCode(`
				walk ./ entry {
					
				}
				return entry
			`)
			varNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtVarIsNotDeclared("entry")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("variables defined in walk statement's body are not accessible after the statement", func(t *testing.T) {
			n, src := parseCode(`
				walk ./ entry {
					x = 3
				}
				return x
			`)

			varNode := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[2]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(varNode, src, fmtVarIsNotDeclared("x")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("runtime typecheck", func(t *testing.T) {

		t.Run("as argument", func(t *testing.T) {
			n, src := parseCode(`map ~$ .title`)
			globals := GlobalVariablesFromMap(map[string]Value{"map": ValOf(Map)}, nil)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals}))
		})

		t.Run("misplaced", func(t *testing.T) {
			n, src := parseCode(`~$`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(n.Statements[0], src, MISPLACED_RUNTIME_TYPECHECK_EXPRESSION),
			)
			assert.Equal(t, expectedErr, err)
		})
	})
	t.Run("assert statement", func(t *testing.T) {

		t.Run("no forbidden node in expression", func(t *testing.T) {
			n, src := parseCode(`
				x = 0
				assert (x > 0)
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("forbidden node in expression", func(t *testing.T) {
			n, src := parseCode(`
				assert (1 + sideEffect())
			`)
			callNode := parse.FindNode(n, (*parse.CallExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(callNode, src, fmtFollowingNodeTypeNotAllowedInAssertions(callNode)),
				makeError(callNode, src, fmtVarIsNotDeclared("sideEffect")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("testsuite expression", func(t *testing.T) {

		t.Run("testsuite expression has its own local scope", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				testsuite { a }
			`)

			identLiteral := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(identLiteral, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("testcase expression", func(t *testing.T) {

		t.Run("testsuite expression has its own local scope", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				testcase { a }
			`)

			identLiteral := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(identLiteral, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("lifetimejob expression", func(t *testing.T) {

		t.Run("lifetimejob expression has its own local scope", func(t *testing.T) {
			n, src := parseCode(`
				a = 1
				%p = %{}
				lifetimejob #job for %p { a }
			`)

			identLiteral := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(identLiteral, src, fmtVarIsNotDeclared("a")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("missing subject lifetime job as value of explicit object property", func(t *testing.T) {
			n, src := parseCode(`
				{
					job: lifetimejob #job { }
				}
			`)

			job := parse.FindNode(n, (*parse.LifetimejobExpression)(nil), nil)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(job, src, MISSING_LIFETIMEJOB_SUBJECT_PATTERN_NOT_AN_IMPLICIT_OBJ_PROP),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("subject lifetime job wih no subject as value of explicit object property", func(t *testing.T) {
			n, src := parseCode(`
				{
					lifetimejob #job { }
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("lifetime job should have access to parent module's patterns ", func(t *testing.T) {
			n, src := parseCode(`
				%p = 1
				lifetimejob #job for %object {
					[%p, %int, %dom.]
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{
				Node:  n,
				Chunk: src,
				Patterns: map[string]Pattern{
					"int":    INT_PATTERN,
					"object": OBJECT_PATTERN,
				},
				PatternNamespaces: map[string]*PatternNamespace{"dom": {}},
			}))
		})

		//TODO: add tests on globals
	})

	t.Run("reception handler expression", func(t *testing.T) {

		t.Run("misplaced", func(t *testing.T) {
			n, src := parseCode(`
				on received %{} fn(){}
			`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(n.Statements[0], src, MISPLACED_RECEPTION_HANDLER_EXPRESSION),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("implicit key property of an object literam", func(t *testing.T) {
			n, src := parseCode(`
				{
					on received %{} fn(){}
				}
			`)

			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

	})

	t.Run("pattern definition", func(t *testing.T) {
		t.Run("redeclaration", func(t *testing.T) {
			n, src := parseCode(`
				%p = 0
				%p = 1
			`)
			def := parse.FindNodes(n, (*parse.PatternDefinition)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(def, src, fmtPatternAlreadyDeclared("p")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("pattern namespace definition", func(t *testing.T) {
		t.Run("redeclaration", func(t *testing.T) {
			n, src := parseCode(`
				%p = 0
				%p = 1
			`)
			def := parse.FindNodes(n, (*parse.PatternDefinition)(nil), nil)[1]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(def, src, fmtPatternAlreadyDeclared("p")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("pattern identifier", func(t *testing.T) {

		t.Run("not declared", func(t *testing.T) {
			n, src := parseCode(`
				%p
			`)
			pattern := parse.FindNode(n, (*parse.PatternIdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(pattern, src, fmtPatternIsNotDeclared("p")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("not declared pattern in lazy pattern definition", func(t *testing.T) {
			n, src := parseCode(`
				%p = @ %str( %s )
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
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
			//{"1o1s", []string{INVALID_QUANTITY}},
			//{"1o2h", []string{INVALID_QUANTITY}},
			{"1s1x", []string{INVALID_QUANTITY}},
			{"1s1h", []string{INVALID_QUANTITY}},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, src := parseCode(testCase.input)
				lit := parse.FindNode(n, (*parse.QuantityLiteral)(nil), nil)
				err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})

				if len(testCase.errors) == 0 {
					assert.NoError(t, err)
				} else {
					var checkingErrs []error
					for _, err := range testCase.errors {
						checkingErrs = append(checkingErrs, makeError(lit, src, err))
					}
					expectedErr := combineErrors(checkingErrs...)
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
			{"1x/h", []string{INVALID_RATE, INVALID_QUANTITY}},
			{"1s/s", []string{INVALID_RATE, INVALID_QUANTITY}},
			{"1h/s", []string{INVALID_RATE, INVALID_QUANTITY}},
			{"1h1s/s", []string{INVALID_RATE, INVALID_QUANTITY}},
			{"1h1s5ms10us15ns/s", []string{INVALID_RATE, INVALID_QUANTITY}},
			//
			{"1x1s/s", []string{INVALID_RATE, INVALID_QUANTITY}},
			{"1x2h/s", []string{INVALID_RATE, INVALID_QUANTITY}},
			{"1s1x/s", []string{INVALID_RATE, INVALID_QUANTITY}},
			{"1s1h/s", []string{INVALID_RATE, INVALID_QUANTITY}},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				n, src := parseCode(testCase.input)
				lit := parse.FindNode(n, (*parse.RateLiteral)(nil), nil)

				err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})

				if len(testCase.errors) == 0 {
					assert.NoError(t, err)
				} else {
					var checkingErrs []error
					for _, err := range testCase.errors {
						checkingErrs = append(checkingErrs, makeError(lit, src, err))
					}
					expectedErr := combineErrors(checkingErrs...)
					assert.Equal(t, expectedErr, err)
				}
			})

			///////////////////
			break
		}

	})

	t.Run("integer range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, src := parseCode(`1..2`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("no upper bound", func(t *testing.T) {
			n, src := parseCode(`1..`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("upper bound should be smaller than lower bound", func(t *testing.T) {
			n, src := parseCode(`1..0`)

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(n.Statements[0], src, LOWER_BOUND_OF_INT_RANGE_LIT_SHOULD_BE_SMALLER_THAN_UPPER_BOUND),
			)
			assert.Equal(t, expectedErr, err)
		})
	})

	t.Run("quantity range literal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			n, src := parseCode(`1x..2x`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("no upper bound", func(t *testing.T) {
			n, src := parseCode(`1x..`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})
	})

	t.Run("match statement", func(t *testing.T) {
		t.Run("group matching variable shadows a global", func(t *testing.T) {
			n, src := parseCode(`
				$$m = 1
				match 1 {
					%/{:a} m { }
				}
			`)
			variable := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(variable, src, fmtCannotShadowGlobalVariable("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("group matching variable shadows a local variable", func(t *testing.T) {
			n, src := parseCode(`
				m = 1
				match 1 {
					%/{:a} m { }
				}
			`)
			variable := parse.FindNode(n, (*parse.IdentifierLiteral)(nil), nil)
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(variable, src, fmtCannotShadowLocalVariable("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

		t.Run("group matching variables with same name", func(t *testing.T) {
			n, src := parseCode(`
				match 1 {
					%/{:a} m { }
					%/a/{:a} m { }
				}
			`)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src}))
		})

		t.Run("group matching variable is not accessible after match statement", func(t *testing.T) {
			n, src := parseCode(`
				match 1 {
					%/{:a} m { }
				}
				return m
			`)
			variable := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[1]
			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src})
			expectedErr := combineErrors(
				makeError(variable, src, fmtVarIsNotDeclared("m")),
			)
			assert.Equal(t, expectedErr, err)
		})

	})

	t.Run("xml element", func(t *testing.T) {

		t.Run("no variable used in elements", func(t *testing.T) {
			n, src := parseCode(`html<div a=1></div>`)

			globals := GlobalVariablesFromMap(map[string]Value{"html": Nil}, nil)
			assert.NoError(t, staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals}))
		})

		t.Run("variable used in elements", func(t *testing.T) {
			n, src := parseCode(`html<div a=b></div>`)

			globals := GlobalVariablesFromMap(map[string]Value{"html": Nil}, nil)
			variable := parse.FindNodes(n, (*parse.IdentifierLiteral)(nil), nil)[3]

			err := staticCheckNoData(StaticCheckInput{Node: n, Chunk: src, Globals: globals})
			expectedErr := combineErrors(
				makeError(variable, src, fmtVarIsNotDeclared("b")),
			)
			assert.Equal(t, expectedErr, err)
		})
	})
}

//TODO: add tests for static checking of remaining manifest sections.

func TestCheckPreinitFilesObject(t *testing.T) {

	parseObject := func(s string) *parse.ObjectLiteral {
		return parse.MustParseChunk(s).Statements[0].(*parse.ObjectLiteral)
	}

	t.Run("empty", func(t *testing.T) {
		objLiteral := parseObject("{}")

		checkPreinitFilesObject(objLiteral, func(n parse.Node, msg string) {
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

		checkPreinitFilesObject(objLiteral, func(n parse.Node, msg string) {
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

		checkPreinitFilesObject(objLiteral, func(n parse.Node, msg string) {
			err = true
			assert.Equal(t, PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH, msg)
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

		checkPreinitFilesObject(objLiteral, func(n parse.Node, msg string) {
			err = true
			assert.Equal(t, PREINIT_FILES__FILE_CONFIG_PATH_SHOULD_BE_ABS_PATH, msg)
		})
		assert.True(t, err)
	})
}

func TestCheckDatabasesObject(t *testing.T) {

	parseObject := func(s string) *parse.ObjectLiteral {
		return parse.MustParseChunk(s).Statements[0].(*parse.ObjectLiteral)
	}

	t.Run("empty", func(t *testing.T) {
		objLiteral := parseObject("{}")

		checkDatabasesObject(objLiteral, func(n parse.Node, msg string) {
			assert.Fail(t, msg)
		})
	})

	t.Run("database with correct description", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				main: {
					resource: ldb://main
					resolution-data: /tmp/mydb/
				}
			}
		`)

		checkDatabasesObject(objLiteral, func(n parse.Node, msg string) {
			assert.Fail(t, msg)
		})
	})
	t.Run("database with missing resource property", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				main: {}
			}
		`)

		err := false

		checkDatabasesObject(objLiteral, func(n parse.Node, msg string) {
			err = true
			assert.Equal(t, fmtMissingPropInDatabaseDescription(MANIFEST_DATABASE__RESOURCE_PROP_NAME, "main"), msg)
		})

		assert.True(t, err)
	})

	t.Run("database with invalid value for the resource property", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				main: {
					resource: 1
				}
			}
		`)
		err := false

		checkDatabasesObject(objLiteral, func(n parse.Node, msg string) {
			err = true
			assert.Equal(t, DATABASES__DB_RESOURCE_SHOULD_BE_HOST_OR_URL, msg)
		})
		assert.True(t, err)
	})

	t.Run("database with path expression for the resolution-data property", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				main: {
					resource: ldb://main
					resolution-data: /{DB_DIR}/
				}
			}
		`)

		checkDatabasesObject(objLiteral, func(n parse.Node, msg string) {
			assert.Fail(t, msg)
		})
	})

	t.Run("database with unsupported value for the resolution-data property", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				main: {
					resource: ldb://main
					resolution-data: 1
				}
			}
		`)
		err := false

		checkDatabasesObject(objLiteral, func(n parse.Node, msg string) {
			err = true
			assert.Equal(t, DATABASES__DB_RESOLUTION_DATA_ONLY_PATHS_SUPPORTED, msg)
		})

		assert.True(t, err)
	})

	t.Run("database with incorrect value for the resolution-data property", func(t *testing.T) {
		objLiteral := parseObject(`
			{
				main: {
					resource: ldb://main
					resolution-data: /file
				}
			}
		`)
		pathNode := parse.FindNode(objLiteral, (*parse.AbsolutePathLiteral)(nil), nil)

		delete(staticallyCheckDbResolutionDataFnRegistry, "ldb")
		RegisterStaticallyCheckDbResolutionDataFn("ldb", func(node parse.Node) (errorMsg string) {
			return "bad"
		})

		checkData, _ := GetStaticallyCheckDbResolutionDataFn("ldb")
		errMsg := checkData(pathNode)

		err := false

		checkDatabasesObject(objLiteral, func(n parse.Node, msg string) {
			err = true
			assert.Equal(t, errMsg, msg)
		})

		assert.True(t, err)
	})
}

// testMutableGoValue implements the GoValue interface
type testMutableGoValue struct {
	Name   string
	secret string
}

func (v testMutableGoValue) HasRepresentation(encountered map[uintptr]int, config *ReprConfig) bool {
	return true
}

func (v testMutableGoValue) IsMutable() bool {
	return true
}

func (v testMutableGoValue) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *ReprConfig) error {
	_, err := w.Write([]byte("mygoval"))
	return err
}

func (v testMutableGoValue) HasJSONRepresentation(encountered map[uintptr]int, config JSONSerializationConfig) bool {
	return true
}

func (v testMutableGoValue) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, encountered map[uintptr]int, config JSONSerializationConfig) error {
	_, err := w.Write([]byte("\"mygoval\""))
	return err
}

func (r testMutableGoValue) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", r))
}

func (v testMutableGoValue) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY, nil
}

func (v testMutableGoValue) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "getName":
		return &GoFunction{fn: v.GetName}, true
	case "getNameNoCtx":
		return &GoFunction{fn: v.GetNameNoCtx}, true
	default:
		return nil, false
	}
}

func (v testMutableGoValue) Prop(ctx *Context, name string) Value {
	switch name {
	case "name":
		return Str(v.Name)
	default:
		method, ok := v.GetGoMethod(name)
		if !ok {
			panic(FormatErrPropertyDoesNotExist(name, v))
		}
		return method
	}
}

func (v testMutableGoValue) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (v testMutableGoValue) PropertyNames(ctx *Context) []string {
	return []string{"name", "getName", "getNameNoCtx"}
}

func (val testMutableGoValue) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherVal, ok := other.(*testMutableGoValue)
	return ok && val.Name == otherVal.Name && val.secret == otherVal.secret
}

func (user testMutableGoValue) GetName(ctx *Context) Str {
	return Str(user.Name)
}

func (user testMutableGoValue) GetNameNoCtx() Str {
	return Str(user.Name)
}

func (user testMutableGoValue) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return nil, ErrNotClonable
}
