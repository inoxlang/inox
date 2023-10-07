package html_ns

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicCreateHTMLNodeFromXMLElement(t *testing.T) {

	globals := func() map[string]symbolic.SymbolicValue {
		return map[string]symbolic.SymbolicValue{
			"html": symbolic.NewNamespace(map[string]symbolic.SymbolicValue{
				symbolic.FROM_XML_FACTORY_NAME: symbolic.WrapGoFunction(CreateHTMLNodeFromXMLElement),
			}),
		}
	}
	t.Run("shallow: invalid interpolation value", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div>{1.0}</div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)

		errors := state.Errors()
		assert.NotEmpty(t, errors)
	})

	t.Run("shallow: HTML node", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div>{html<span></span>}</div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)
		assert.Empty(t, state.Errors())
	})

	t.Run("shallow: list of HTML nodes", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div>{ [ html<span></span> ] }</div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)
		assert.Empty(t, state.Errors())
	})

	t.Run("deep: invalid interpolation value", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div><span>{1.0}</span></div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)

		errors := state.Errors()
		assert.NotEmpty(t, errors)
	})

	t.Run("shallow: invalid XML attribute value", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div a=1.0></div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)

		errors := state.Errors()
		assert.NotEmpty(t, errors)
	})

	t.Run("deep: invalid XML attribute value", func(t *testing.T) {
		chunk, state := symbolic.MakeTestStateAndChunk("html<div><span a=1.0></span></div>", globals())

		_, err := symbolic.SymbolicEval(chunk, state)
		assert.NoError(t, err)

		errors := state.Errors()
		assert.NotEmpty(t, errors)
	})
}
