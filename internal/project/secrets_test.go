package project

import (
	"os"
	"strings"
	"testing"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

var (
	CLOUDFLARE_ACCOUNT_ID                  = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN = os.Getenv("CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN")
)

func TestAddSecret(t *testing.T) {
	projectName := "test-add-secret"
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

	registry, err := OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000))
	if !assert.NoError(t, err) {
		return
	}

	id, err := registry.CreateProject(ctx, CreateProjectParams{
		Name: projectName,
	})

	if !assert.NoError(t, err) {
		return
	}

	project, err := registry.OpenProject(ctx, OpenProjectParams{
		Id: id,
		DevSideConfig: DevSideProjectConfig{
			Cloudflare: &DevSideCloudflareConfig{
				AdditionalTokensApiToken: CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN,
				AccountID:                CLOUDFLARE_ACCOUNT_ID,
			},
		},
	})

	if !assert.NoError(t, err) {
		return
	}

	defer func() {
		//delete tokens & bucket
		tokens := utils.Ret0(project.TempProjectTokens(ctx)).Cloudflare
		err = DeleteR2Bucket(ctx, project.secretsBucket, *tokens, project.devSideConfig.Cloudflare.AccountID)
		assert.NoError(t, err)

		api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
		if err != nil {
			return
		}

		apiTokens, err := api.APITokens(ctx)
		if err != nil {
			return
		}

		for _, token := range apiTokens {
			if strings.Contains(token.Name, projectName) {
				err := api.DeleteAPIToken(ctx, token.ID)
				if err != nil {
					t.Log(err)
				}
			}
		}
	}()

	err = project.AddSecret(ctx, "my-secret", "secret")
	assert.NoError(t, err)
}
