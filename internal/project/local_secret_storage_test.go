package project

import (
	"sync"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestLocalSecretStorage(t *testing.T) {

	t.Run("list secrets before any secret creation", func(t *testing.T) {
		projectName := "test-lists-secrets-before-creation"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Limits: []core.Limit{objectStorageLimit},
		}, nil)
		defer ctx.CancelGracefully()

		registry, err := OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000), ctx)
		if !assert.NoError(t, err) {
			return
		}
		defer registry.Close(ctx)

		id, err := registry.CreateProject(ctx, CreateProjectParams{
			Name: projectName,
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := registry.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

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

		registry, err := OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000), ctx)
		if !assert.NoError(t, err) {
			return
		}
		defer registry.Close(ctx)

		id, err := registry.CreateProject(ctx, CreateProjectParams{
			Name: projectName,
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := registry.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

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

		registry, err := OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000), ctx)
		if !assert.NoError(t, err) {
			return
		}
		defer registry.Close(ctx)

		id, err := registry.CreateProject(ctx, CreateProjectParams{
			Name: projectName,
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := registry.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

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

		registry, err := OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000), ctx)
		if !assert.NoError(t, err) {
			return
		}
		defer registry.Close(ctx)

		id, err := registry.CreateProject(ctx, CreateProjectParams{
			Name: projectName,
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := registry.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

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
		assert.EqualValues(t, "my-secret", secrets[0].Name)

		secrets2, err := project.GetSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, secrets2, 1) {
			return
		}
		assert.EqualValues(t, "my-secret", secrets2[0].Name)
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

	t.Run("list secrets after creation and after deletion + project re-opening", func(t *testing.T) {
		projectName := "test-sec-list-after-crea"
		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Limits: []core.Limit{objectStorageLimit},
		}, nil)
		defer ctx.CancelGracefully()

		fls := fs_ns.NewMemFilesystem(100_000_000)

		registry, err := OpenRegistry("/", fls, ctx)
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
		})

		if !assert.NoError(t, err) {
			return
		}

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
		assert.EqualValues(t, "my-secret", secrets[0].Name)

		secrets2, err := project.GetSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, secrets2, 1) {
			return
		}
		assert.EqualValues(t, "my-secret", secrets2[0].Name)
		assert.Equal(t, "secret", secrets2[0].Value.StringValue().GetOrBuildString())

		// reopen project
		registry.Close(ctx)

		registry, err = OpenRegistry("/", fls, ctx)
		if !assert.NoError(t, err) {
			return
		}

		project, err = registry.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		// check again

		secrets, err = project.ListSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, secrets, 1) {
			return
		}
		assert.EqualValues(t, "my-secret", secrets[0].Name)

		secrets2, err = project.GetSecrets(ctx)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, secrets2, 1) {
			return
		}
		assert.EqualValues(t, "my-secret", secrets2[0].Name)
		assert.Equal(t, "secret", secrets2[0].Value.StringValue().GetOrBuildString())

		//delete secret

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

		// reopen project

		registry.Close(ctx)
		registry, err = OpenRegistry("/", fls, ctx)
		if !assert.NoError(t, err) {
			return
		}
		defer registry.Close(ctx)

		project, err = registry.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		// check again

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

		registry, err := OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000), ctx)
		if !assert.NoError(t, err) {
			return
		}
		defer registry.Close(ctx)

		id, err := registry.CreateProject(ctx, CreateProjectParams{
			Name: projectName,
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := registry.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

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
			assert.EqualValues(t, "my-secret", secrets[0].Name)

			secrets2, err := project.GetSecrets(ctx)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Len(t, secrets2, 1) {
				return
			}
			assert.EqualValues(t, "my-secret", secrets[0].Name)
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
