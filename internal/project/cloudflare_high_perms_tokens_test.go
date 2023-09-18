package project

import (
	"context"
	"os"
	"strings"
	"testing"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

var (
	cloudflareConfig = DevSideCloudflareConfig{
		AdditionalTokensApiToken: os.Getenv("CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN"),
		AccountID:                os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
	}
)

func TestGetUpToDateTempCloudflareTokens(t *testing.T) {
	projectId := core.ProjectID("test-temp-cf-tokens")

	if cloudflareConfig.AdditionalTokensApiToken == "" {
		t.Skip()
		return
	}

	ctx := context.Background()

	cf, err := newCloudflare(projectId, &cloudflareConfig)
	if !assert.NoError(t, err) {
		return
	}

	//cleanup
	if !assert.NoError(t, cf.deleteHighPermsTokens(ctx)) {
		return
	}

	apiTokens, err := cf.apiTokensApi.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	tokenCountBeforeTest := len(apiTokens)

	prevTokens, err := cf.getUpToDateTempTokens(ctx)

	if !assert.NoError(t, err) {
		return
	}

	defer deleteTestRelatedTokens(t, ctx, cf.apiTokensApi, projectId)

	prevR2Token := prevTokens.R2Token

	if !assert.NotNil(t, prevR2Token) {
		return
	}

	if !assert.NotEmpty(t, prevR2Token.Value) {
		return
	}

	//if the R2 API token is already present no tokens should be created
	upToToDateTokens, err := cf.getUpToDateTempTokens(ctx)
	r2Token := upToToDateTokens.R2Token

	if !assert.NoError(t, err) {
		return
	}

	assert.Same(t, prevTokens.R2Token, r2Token)

	apiTokens, err = cf.apiTokensApi.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	if !assert.Equal(t, 1+tokenCountBeforeTest, len(apiTokens)) {
		return
	}

	cf.forgetHighPermsTokens(ctx)

	//if the R2 API token is not present but the token already exists it should be updated
	upToToDateTokens, err = cf.getUpToDateTempTokens(ctx)
	r2Token = upToToDateTokens.R2Token

	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, r2Token) {
		return
	}

	assert.NotEqual(t, prevR2Token, r2Token)
	assert.Equal(t, prevR2Token.Id, r2Token.Id)
	assert.NotEmpty(t, r2Token.Id)
	assert.NotEmpty(t, r2Token.Value)

	apiTokens, err = cf.apiTokensApi.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	if !assert.Equal(t, 1+tokenCountBeforeTest, len(apiTokens)) {
		return
	}
}

func deleteTestRelatedTokens(t *testing.T, ctx context.Context, api *cloudflare.API, projectId core.ProjectID) {
	apiTokens, err := api.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	for _, token := range apiTokens {
		if strings.Contains(token.Name, string(projectId)) {
			err := api.DeleteAPIToken(ctx, token.ID)
			_ = err
		}
	}
}
