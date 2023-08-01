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
}

func TestSymbolicRoutineGroup(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		group := &RoutineGroup{}

		assert.True(t, group.Test(group))
		assert.False(t, group.Test(&Int{}))
	})

}
