package spec

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAPIGetEndpoint(t *testing.T) {

	t.Run("parametric path", func(t *testing.T) {
		api := utils.Must(NewAPI(map[string]*ApiEndpoint{
			"/users/{user-id}": {
				path: "/users/{user-id}",
			},
		}))

		endpt, err := api.GetEndpoint("/users/0")
		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, api.endpoints["/users/{user-id}"], endpt)
	})
}
