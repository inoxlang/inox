package http_ns

import (
	_ "embed"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	OPEN_AI_API_SPEC_BASE_URL     = "https://raw.githubusercontent.com/APIs-guru/openapi-directory/main/APIs/openai.com/1.2.0/"
	OPEN_AI_API_SPEC_BASE_URL_URL = "https://raw.githubusercontent.com/APIs-guru/openapi-directory/main/APIs/openai.com/1.2.0/openapi.yaml"
)

func TestGetAPIFromOpenAPISpec(t *testing.T) {
	resp, err := http.Get(OPEN_AI_API_SPEC_BASE_URL_URL)
	if !assert.NoError(t, err) {
		return
	}

	spec, err := io.ReadAll(resp.Body)
	if !assert.NoError(t, err) {
		return
	}

	api, err := GetAPIFromOpenAPISpec(spec, OPEN_AI_API_SPEC_BASE_URL)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, api) {
		return
	}

	assert.NotEmpty(t, api.endpoints)
}
