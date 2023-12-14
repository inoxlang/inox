package env_ns

import (
	"os"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {

	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/user"
		os.Setenv("HOME", home)
	}

	ctx := core.NewContext(core.ContextConfig{})
	env, _ := NewEnvNamespace(ctx, nil, true)

	assert.Equal(t, core.Path(home+"/"), env.Prop(nil, "HOME"))

	newCtxNoPerms := func() *core.Context {
		ctx := core.NewContext(core.ContextConfig{})
		core.NewGlobalState(ctx)
		return ctx
	}

	newCtxWithPerms := func() *core.Context {
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.EnvVarPermission{Kind_: permkind.Read, Name: "*"},
				core.EnvVarPermission{Kind_: permkind.Create, Name: "*"},
				core.EnvVarPermission{Kind_: permkind.Update, Name: "*"},
				core.EnvVarPermission{Kind_: permkind.Delete, Name: "*"},
			},
		})
		core.NewGlobalState(ctx)
		return ctx
	}

	t.Run("envGet", func(t *testing.T) {
		os.Setenv("XYZ", "1")

		v, err := envGet(newCtxNoPerms(), "XYZ")
		assert.Error(t, err)
		assert.Empty(t, v)

		v, err = envGet(newCtxWithPerms(), "XYZ")
		assert.NoError(t, err)
		assert.EqualValues(t, "1", v)
	})

	t.Run("envSet", func(t *testing.T) {
		os.Setenv("XYZ", "1")

		err := envSet(newCtxNoPerms(), "XYZ", "2")
		assert.Error(t, err)
		assert.Equal(t, "1", os.Getenv("XYZ"))

		err = envSet(newCtxWithPerms(), "XYZ", "3")
		assert.NoError(t, err)
		assert.Equal(t, "3", os.Getenv("XYZ"))
	})

	t.Run("envDelete", func(t *testing.T) {
		os.Setenv("XYZ", "1")

		err := envDelete(newCtxNoPerms(), "XYZ")
		assert.Error(t, err)
		assert.Equal(t, "1", os.Getenv("XYZ"))

		err = envSet(newCtxWithPerms(), "XYZ", "2")
		assert.NoError(t, err)
		assert.Equal(t, "2", os.Getenv("XYZ"))
	})

	t.Run("envAll", func(t *testing.T) {
		env, err := envAll(newCtxNoPerms())
		assert.Error(t, err)
		assert.Nil(t, env)

		env, err = envAll(newCtxWithPerms())
		assert.NoError(t, err)
		assert.NotNil(t, env)
	})
}
