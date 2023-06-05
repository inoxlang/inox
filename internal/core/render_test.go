package core

import (
	"testing"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestNodeRender(t *testing.T) {

	// SKIPPED FOR NOW !!!
	t.Skip()

	renderHTML := func(code string) string {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)
		return render(ctx, AstNode{Node: parse.MustParseChunk(code)}, HTML_CTYPE)
	}

	assert.Equal(t, "<div><span>1</span></div>", renderHTML("1"))
	assert.Equal(t, "<div><span>f</span></div>", renderHTML("f"))
	assert.Equal(t, "<div><span>f</span><span>(</span><span>)</span></div>", renderHTML("f()"))

}
