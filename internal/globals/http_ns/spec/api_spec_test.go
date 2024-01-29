package spec

import (
	_ "embed"
	"io"
	"net/http"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
)

const (
	OPEN_AI_API_SPEC_BASE_URL     = "https://raw.githubusercontent.com/APIs-guru/openapi-directory/main/APIs/openai.com/1.2.0/"
	OPEN_AI_API_SPEC_BASE_URL_URL = "https://raw.githubusercontent.com/APIs-guru/openapi-directory/main/APIs/openai.com/1.2.0/openapi.yaml"
)

func TestGetAPIFromOpenAPISpec(t *testing.T) {
	testconfig.AllowParallelization(t)

	resp, err := http.Get(OPEN_AI_API_SPEC_BASE_URL_URL)
	if !assert.NoError(t, err) {
		return
	}

	spec, err := io.ReadAll(resp.Body)
	if !assert.NoError(t, err) {
		return
	}

	api, err := createAPIFromOpenAPISpec(spec, OPEN_AI_API_SPEC_BASE_URL)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, api) {
		return
	}

	if !assert.Contains(t, maps.Keys(api.endpoints), "/answers") {
		return
	}

	endpt := api.endpoints["/answers"]

	if !assert.NotEmpty(t, endpt.operations) {
		return
	}

	op := endpt.operations[0]
	if !assert.Equal(t, "POST", op.httpMethod) {
		return
	}

	if !assert.Contains(t, maps.Keys(op.jsonResponseBodies), uint16(200)) {
		return
	}
	pattern := op.jsonResponseBodies[200]

	if !assert.IsType(t, pattern, (*core.ObjectPattern)(nil)) {
		return
	}

	found := false

	pattern.(*core.ObjectPattern).ForEachEntry(func(entry core.ObjectPatternEntry) error {
		if entry.Name == "answers" {
			found = true
		}
		return nil
	})

	assert.True(t, found)
}
