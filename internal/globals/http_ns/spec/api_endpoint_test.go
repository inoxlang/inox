package spec

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAPIEndpointGetPathParams(t *testing.T) {

	t.Run("single parameter at the end", func(t *testing.T) {
		api := utils.Must(NewAPI(map[string]*ApiEndpoint{
			"/users/{user-id}": {
				path: "/users/{user-id}",
			},
		}))

		path := "/users/0"

		endpt, err := api.GetEndpoint(path)
		if !assert.NoError(t, err) {
			return
		}

		params, count, err := endpt.GetPathParams(path)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Equal(t, 1, count) {
			return
		}

		assert.Equal(t, PathParam{Name: "user-id", Value: "0"}, params[0])
	})

	t.Run("single parameter not at the end", func(t *testing.T) {
		api := utils.Must(NewAPI(map[string]*ApiEndpoint{
			"/users/{user-id}/name": {
				path: "/users/{user-id}/name",
			},
		}))

		path := "/users/0/name"

		endpt, err := api.GetEndpoint(path)
		if !assert.NoError(t, err) {
			return
		}

		params, count, err := endpt.GetPathParams(path)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Equal(t, 1, count) {
			return
		}

		assert.Equal(t, PathParam{Name: "user-id", Value: "0"}, params[0])
	})

	t.Run("two parameters", func(t *testing.T) {
		api := utils.Must(NewAPI(map[string]*ApiEndpoint{
			"/users/{user-id}/friends/{friend-id}": {
				path: "/users/{user-id}/friends/{friend-id}",
			},
		}))

		path := "/users/0/friends/1"

		endpt, err := api.GetEndpoint(path)
		if !assert.NoError(t, err) {
			return
		}

		params, count, err := endpt.GetPathParams(path)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Equal(t, 2, count) {
			return
		}

		assert.Equal(t, PathParam{Name: "user-id", Value: "0"}, params[0])
		assert.Equal(t, PathParam{Name: "friend-id", Value: "1"}, params[1])
	})
}
