package project

import (
	"context"
	"strings"
	"testing"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
)

var (
	projectId        = ProjectID("test-project")
	cloudflareConfig = DevSideCloudflareConfig{
		AdditionalTokensApiToken: "",
		AccountID:                "",
	}
)

func TestGetTempCloudflareTokens(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	api, err := cloudflare.NewWithAPIToken(cloudflareConfig.AdditionalTokensApiToken)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	deleteTestRelatedTokens := func() {
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

	defer deleteTestRelatedTokens()
	deleteTestRelatedTokens()

	apiTokens, err := api.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	tokenCountBeforeTest := len(apiTokens)

	prevR2Token, err := GetTempCloudflareTokens(ctx, cloudflareConfig, TempCloudflareTokens{}, projectId)

	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotEmpty(t, prevR2Token) {
		return
	}

	//if a R2 API token is passed no tokens should be created
	r2Token, err := GetTempCloudflareTokens(ctx, cloudflareConfig, TempCloudflareTokens{
		R2Token: prevR2Token,
	}, projectId)

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, prevR2Token, r2Token)

	apiTokens, err = api.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	if !assert.Equal(t, 1+tokenCountBeforeTest, len(apiTokens)) {
		return
	}

	//if no R2 API token is passed and the token already exists it should be updated
	r2Token, err = GetTempCloudflareTokens(ctx, cloudflareConfig, TempCloudflareTokens{}, projectId)

	if !assert.NoError(t, err) {
		return
	}

	assert.NotEqual(t, prevR2Token, r2Token)
	assert.NotEmpty(t, r2Token)

	apiTokens, err = api.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	if !assert.Equal(t, 1+tokenCountBeforeTest, len(apiTokens)) {
		return
	}
}
