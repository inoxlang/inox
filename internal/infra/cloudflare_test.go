package infra

import (
	"context"
	"strings"
	"testing"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/project"
	"github.com/stretchr/testify/assert"
)

var (
	projectId        = project.ProjectID("test-project")
	cloudflareConfig = project.DevSideCloudflareConfig{
		AdditionalTokensApiToken: "",
		AccountID:                "",
	}
)

func TestGetTempCloudflareTokens(t *testing.T) {
	t.Skip()
	ctx := context.Background()

	deleteTestRelatedTokens := func() {
		api, err := cloudflare.NewWithAPIToken(cloudflareConfig.AdditionalTokensApiToken)
		if err != nil {
			assert.Fail(t, err.Error())
			return
		}

		apiTokens, err := api.APITokens(ctx)
		if err != nil {
			assert.Fail(t, err.Error())
			return
		}

		for _, token := range apiTokens {
			if strings.Contains(token.ID, string(projectId)) {
				api.DeleteAPIToken(ctx, token.ID)
			}
		}
	}

	defer deleteTestRelatedTokens()

	prevR2Token, err := GetTempCloudflareTokens(ctx, cloudflareConfig, project.TempCloudflareTokens{}, projectId)

	if !assert.NoError(t, err) {
		return
	}

	assert.NotEmpty(t, prevR2Token)

	r2Token, err := GetTempCloudflareTokens(ctx, cloudflareConfig, project.TempCloudflareTokens{
		R2Token: prevR2Token,
	}, projectId)

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, prevR2Token, r2Token)
}
