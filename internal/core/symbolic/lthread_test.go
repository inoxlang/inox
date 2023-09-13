package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicLThread(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		lthread := &LThread{}

		assert.True(t, lthread.Test(lthread))
		assert.False(t, lthread.Test(&Int{}))
	})
}

func TestSymbolicLThreadGroup(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		group := &LThreadGroup{}

		assert.True(t, group.Test(group))
		assert.False(t, group.Test(&Int{}))
	})

}
