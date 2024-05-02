package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicCreateHTMLNodeFromMarkupElement(t *testing.T) {

	globals := func() map[string]symbolic.Value {
		return map[string]symbolic.Value{
			"html": symbolic.NewNamespace(map[string]symbolic.Value{
				symbolic.FROM_MARKUP_FACTORY_NAME: symbolic.WrapGoFunction(CreateHTMLNodeFromMarkupElement),
			}),
		}
	}

	t.Run("shallow", func(t *testing.T) {
		t.Run("invalid interpolation value", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk("html<div>{1.0}</div>", globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)

			errors := state.Errors()
			assert.NotEmpty(t, errors)
		})

		t.Run("interpolation with a Go string value", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk("html<div>{https://localhost}</div>", globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)
			assert.Empty(t, state.Errors())
		})

		t.Run("HTML node", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk("html<div>{html<span></span>}</div>", globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)
			assert.Empty(t, state.Errors())
		})

		t.Run("list of HTML nodes", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk("html<div>{ [ html<span></span> ] }</div>", globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)
			assert.Empty(t, state.Errors())
		})

		t.Run("multi value with one case being a list of HTML nodes", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk(`
				html<div>
				{ if true  [ html<span></span> ] else 2 }
				</div>
			`, globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)
			assert.Empty(t, state.Errors())
		})

		t.Run("list of string-like values", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk(`html<div>{ [ "a", (concat "b" "c")  ] }</div>`, globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)
			assert.Empty(t, state.Errors())
		})

		t.Run("list of invalid values", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk(`html<div>{ [ 1.0 ] }</div>`, globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)
			assert.NotEmpty(t, state.Errors())
		})

		t.Run("invalid markup attribute value", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk("html<div a=1.0></div>", globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)

			errors := state.Errors()
			if !assert.Len(t, errors, 1) {
				return
			}
			assert.Contains(t, errors[0].Error(), fmtAttrValueNotAccepted(symbolic.FLOAT_1, "a"))
		})

		t.Run("attribute with a Go string value", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk("html<div a=https://localhost></div>", globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)
		})
	})

	t.Run("deep: invalid interpolation value", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div><span>{1.0}</span></div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)

		errors := state.Errors()
		if !assert.Len(t, errors, 1) {
			return
		}
		assert.Contains(t, errors[0].Error(), INTERPOLATION_LIMITATION_ERROR_MSG)
	})

	t.Run("deep: invalid markup attribute value", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div><span a=1.0></span></div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)

		errors := state.Errors()
		assert.NotEmpty(t, errors)
	})

	t.Run("deep: complex", func(t *testing.T) {

		globals := globals()
		globals["unknown_len_node_list"] = symbolic.NewListOf(&HTMLNode{
			tagName:            "a",
			requiredAttributes: []HTMLAttribute{{name: "href", stringValue: symbolic.ANY_STRING}},
		})

		chunk, state := symbolic.MakeTestStateAndChunk(`
			known_len_node_list = [<div b></div>, <div c></div>]

			return html<div>
				<span a="true"></span>
				<form hx-post-json=""></form>
				{known_len_node_list}
				{unknown_len_node_list}
			</div>
		`, globals)

		node, err := symbolic.SymbolicEval(chunk, state)
		if !assert.NoError(t, err) {
			return
		}

		errors := state.Errors()
		if !assert.Empty(t, errors) {
			return
		}

		markupElements := parse.FindNodes(chunk, (*parse.MarkupElement)(nil), nil)

		expectedNode := &HTMLNode{
			tagName:    "div",
			sourceNode: markupElements[2],
			requiredChildren: []*HTMLNode{
				{
					tagName:            "span",
					sourceNode:         markupElements[3],
					requiredAttributes: []HTMLAttribute{{name: "a", stringValue: symbolic.NewString("true")}},
				},
				{
					tagName:    "form",
					sourceNode: markupElements[4],
					requiredAttributes: []HTMLAttribute{
						{name: "hx-post", stringValue: symbolic.ANY_STRING},
						{name: "hx-ext", stringValue: symbolic.ANY_STRING},
					},
				},
				//elements of known-length list
				{
					tagName:            "div",
					sourceNode:         markupElements[0],
					requiredAttributes: []HTMLAttribute{{name: "b", stringValue: symbolic.EMPTY_STRING}},
				},
				{
					tagName:            "div",
					sourceNode:         markupElements[1],
					requiredAttributes: []HTMLAttribute{{name: "c", stringValue: symbolic.EMPTY_STRING}},
				},
				//elements of unknown-length list
				{
					tagName:            "a",
					requiredAttributes: []HTMLAttribute{{name: "href", stringValue: symbolic.ANY_STRING}},
				},
			},
		}

		assert.Equal(t, expectedNode, node)
	})
}
