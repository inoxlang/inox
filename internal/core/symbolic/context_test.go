package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicContext(t *testing.T) {

	t.Run("forked", func(t *testing.T) {
		t.Run("AddHostAlias()", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			fork := ctx.fork()

			fork.AddHostAlias("site", &Host{}, false)
			assert.Equal(t, &Host{}, fork.ResolveHostAlias("site"))
			assert.Nil(t, ctx.ResolveHostAlias("site"))
		})

		t.Run("AddNamedPattern()", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			fork := ctx.fork()

			fork.AddNamedPattern("p", &AnyPattern{}, false)
			assert.Equal(t, &AnyPattern{}, fork.ResolveNamedPattern("p"))
			assert.Nil(t, ctx.ResolveNamedPattern("p"))
		})
	})
}
