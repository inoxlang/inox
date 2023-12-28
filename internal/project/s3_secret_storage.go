package project

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/project/cloudflareprovider"
	"github.com/stretchr/testify/assert"
)

var (
	CLOUDFLARE_ACCOUNT_ID                  = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN = os.Getenv("CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN")

	objectStorageLimit = core.Limit{
		Name:  s3_ns.OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME,
		Kind:  core.SimpleRateLimit,
		Value: 100,
	}
)

func TestUpsertListSecrets(t *testing.T) {
	if CLOUDFLARE_ACCOUNT_ID == "" {
		t.Skip()
		return
	}

	t.Run("list secrets before any secret creation", func(t *testing.T) {
		projectName := "test-lists-secrets-before-creation"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Limits: []core.Limit{objectStorageLimit},
		}, nil)
		defer ctx.CancelGracefully()

		registry, err := OpenRegistry(t.TempDir(), ctx)
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
				Cloudflare: &cloudflareprovider.DevSideConfig{
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
			err := project.DeleteSecretsBucket(ctx)
			assert.NoError(t, err)

			api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
			if err != nil {
				return
			}

			deleteTestRelatedTokens(t, ctx, api, project.id)
		}()

		secrets, err := project.ListSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Empty(t, secrets) {
			return
		}

		secrets2, err := project.GetSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, secrets2)
	})

	t.Run("listing secrets while calling getCreateSecretsBucket() should be thread safe", func(t *testing.T) {
		projectName := "test-lists-secrets-before-creation"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Limits: []core.Limit{objectStorageLimit},
		}, nil)

		defer ctx.CancelGracefully()

		registry, err := OpenRegistry(t.TempDir(), ctx)
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
				Cloudflare: &cloudflareprovider.DevSideConfig{
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
			err := project.DeleteSecretsBucket(ctx)
			assert.NoError(t, err)

			api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
			if err != nil {
				return
			}

			deleteTestRelatedTokens(t, ctx, api, project.id)
		}()

		go project.getCreateSecretsBucket(ctx.BoundChild(), false)
		time.Sleep(time.Millisecond)

		secrets, err := project.ListSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Empty(t, secrets) {
			return
		}

		secrets, err = project.ListSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, secrets)
	})

	t.Run("listing secrets in parallel before any creation should be thread safe", func(t *testing.T) {

		projectName := "test-para-sec-list-bef-crea"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Limits: []core.Limit{objectStorageLimit},
		}, nil)
		defer ctx.CancelGracefully()

		registry, err := OpenRegistry(t.TempDir(), ctx)
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
				Cloudflare: &cloudflareprovider.DevSideConfig{
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
			err := project.DeleteSecretsBucket(ctx)
			assert.NoError(t, err)

			api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
			if err != nil {
				return
			}

			deleteTestRelatedTokens(t, ctx, api, project.id)
		}()

		listSecrets := func() {
			secrets, err := project.ListSecrets(ctx)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Empty(t, secrets) {
				return
			}

			secrets2, err := project.GetSecrets(ctx)
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, secrets2)
		}

		wg := new(sync.WaitGroup)
		wg.Add(2)

		go func() {
			defer wg.Done()
			listSecrets()
		}()
		go func() {
			defer wg.Done()
			listSecrets()
		}()
		time.Sleep(time.Millisecond)
		listSecrets()
		wg.Wait()
	})

	t.Run("list secrets after creation and after deletion", func(t *testing.T) {
		projectName := "test-sec-list-after-crea"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Limits: []core.Limit{objectStorageLimit},
		}, nil)
		defer ctx.CancelGracefully()

		registry, err := OpenRegistry(t.TempDir(), ctx)
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
				Cloudflare: &cloudflareprovider.DevSideConfig{
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
			err := project.DeleteSecretsBucket(ctx)
			assert.NoError(t, err)

			api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
			if err != nil {
				return
			}

			deleteTestRelatedTokens(t, ctx, api, project.id)
		}()

		err = project.UpsertSecret(ctx, "my-secret", "secret")
		if !assert.NoError(t, err) {
			return
		}

		secrets, err := project.ListSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, secrets, 1) {
			return
		}
		assert.Equal(t, "my-secret", secrets[0].Name)

		secrets2, err := project.GetSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, secrets2, 1) {
			return
		}
		assert.Equal(t, "my-secret", secrets2[0].Name)
		assert.Equal(t, "secret", secrets2[0].Value.StringValue().GetOrBuildString())

		err = project.DeleteSecret(ctx, "my-secret")
		if !assert.NoError(t, err) {
			return
		}

		secrets, err = project.ListSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Empty(t, secrets, 0) {
			return
		}

		secrets2, err = project.GetSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, secrets2, 0)
	})

	t.Run("listing secrets in parallel should be thread safe", func(t *testing.T) {

		projectName := "test-para-sec-list-aft-crea"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Limits: []core.Limit{objectStorageLimit},
		}, nil)
		defer ctx.CancelGracefully()

		registry, err := OpenRegistry(t.TempDir(), ctx)
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
				Cloudflare: &cloudflareprovider.DevSideConfig{
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
			err := project.DeleteSecretsBucket(ctx)
			assert.NoError(t, err)

			api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
			if err != nil {
				return
			}

			deleteTestRelatedTokens(t, ctx, api, project.id)
		}()

		err = project.UpsertSecret(ctx, "my-secret", "secret")
		if !assert.NoError(t, err) {
			return
		}

		listSecrets := func() {
			secrets, err := project.ListSecrets(ctx)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Len(t, secrets, 1) {
				return
			}
			assert.Equal(t, "my-secret", secrets[0].Name)

			secrets2, err := project.GetSecrets(ctx)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Len(t, secrets2, 1) {
				return
			}
			assert.Equal(t, "my-secret", secrets[0].Name)
		}

		wg := new(sync.WaitGroup)
		wg.Add(2)

		go func() {
			defer wg.Done()
			listSecrets()
		}()
		go func() {
			defer wg.Done()
			listSecrets()
		}()
		time.Sleep(time.Millisecond)
		listSecrets()
		wg.Wait()
	})

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
			if err != nil {
				t.Log(err)
			}
		}
	}
}
