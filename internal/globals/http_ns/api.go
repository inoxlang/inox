package http_ns

import (
	core "github.com/inoxlang/inox/internal/core"
)

type API struct {
	endpoints map[string]*ApiEndpoint
}

type ApiEndpoint struct {
	path string //may have parameters

	operations []ApiOperation
}

type ApiOperation struct {
	id         string //optional
	endpoint   *ApiEndpoint
	httpMethod string

	jsonRequestBody core.Pattern
}
