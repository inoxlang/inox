package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicRoutine(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		routine := &Routine{}

		assert.True(t, routine.Test(routine))
		assert.False(t, routine.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&Routine{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		routine := &Routine{}

		assert.False(t, routine.IsWidenable())

		widened, ok := routine.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}

func TestSymbolicRoutineGroup(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		group := &RoutineGroup{}

		assert.True(t, group.Test(group))
		assert.False(t, group.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&RoutineGroup{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		group := &RoutineGroup{}

		assert.False(t, group.IsWidenable())

		widened, ok := group.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}
