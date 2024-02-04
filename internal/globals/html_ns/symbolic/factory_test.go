package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicCreateHTMLNodeFromXMLElement(t *testing.T) {

	globals := func() map[string]symbolic.Value {
		return map[string]symbolic.Value{
			"html": symbolic.NewNamespace(map[string]symbolic.Value{
				symbolic.FROM_XML_FACTORY_NAME: symbolic.WrapGoFunction(CreateHTMLNodeFromXMLElement),
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

		t.Run("invalid XML attribute value", func(t *testing.T) {
			chunk, state := symbolic.MakeTestStateAndChunk("html<div a=1.0></div>", globals())

			_, err := symbolic.SymbolicEval(chunk, state)
			assert.NoError(t, err)

			errors := state.Errors()
			if !assert.Len(t, errors, 1) {
				return
			}
			assert.Contains(t, errors[0].Error(), fmtAttrValueNotAccepted(symbolic.FLOAT_1, "a"))
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

	t.Run("deep: invalid XML attribute value", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div><span a=1.0></span></div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)

		errors := state.Errors()
		assert.NotEmpty(t, errors)
	})
}
