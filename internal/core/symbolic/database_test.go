package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseIL(t *testing.T) {

	t.Run("Prop()", func(t *testing.T) {
		userPattern := NewExactObjectPattern(map[string]Pattern{}, nil)

		db := NewDatabaseIL(DatabaseILParams{
			Schema: NewExactObjectPattern(map[string]Pattern{
				"user": userPattern,
			}, nil),
			BaseURL: NewUrl("ldb://main/"),
		})

		expectedUser := userPattern.
			SymbolicValue().(*Object).
			Share(nil).(*Object).
			WithURL(NewUrl("ldb://main/user"))

		assert.Equal(t, expectedUser, db.Prop("user"))
	})
}
