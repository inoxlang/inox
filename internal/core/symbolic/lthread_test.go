package symbolic

import (
	"testing"
)

func TestSymbolicLThread(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		lthread := &LThread{}

		assertTest(t, lthread, lthread)
		assertTestFalse(t, lthread, &Int{})
	})
}

func TestSymbolicLThreadGroup(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		group := &LThreadGroup{}

		assertTest(t, group, group)
		assertTestFalse(t, group, &Int{})
	})

}
