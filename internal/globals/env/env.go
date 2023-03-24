package internal

import (
	"os"
	"strings"

	core "github.com/inox-project/inox/internal/core"
)

// envHas returns (True, nil) if the environment variable with the provided name exists, a permission is required.
func envHas(ctx *core.Context, _name core.Str) (core.Bool, error) {
	name := string(_name)
	perm := core.EnvVarPermission{Kind_: core.ReadPerm, Name: name}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return false, err
	}

	_, ok := os.LookupEnv(name)
	return core.Bool(ok), nil
}

// envGet returns the value of the environment variable with the provided name, a permission is required.
func envGet(ctx *core.Context, _name core.Str) (core.Str, error) {
	name := string(_name)
	perm := core.EnvVarPermission{Kind_: core.ReadPerm, Name: name}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return "", err
	}

	return core.Str(os.Getenv(name)), nil
}

// envAll returns an Object containing all environment variables and their values, a permission is required.
func envAll(ctx *core.Context) (*core.Object, error) {
	perm := core.EnvVarPermission{Kind_: core.ReadPerm, Name: "*"}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	environ := os.Environ()
	environObject := core.NewObject()

	for _, envs := range environ {
		key, value, _ := strings.Cut(envs, "=")
		environObject.SetProp(ctx, key, core.Str(value))
	}

	return environObject, nil
}

// envSet sets an environemment variable to the provided value, a permission is required.
func envSet(ctx *core.Context, name, value core.Str) error {
	perm := core.EnvVarPermission{Kind_: core.CreatePerm, Name: string(name)}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	return os.Setenv(string(name), string(value))
}

// envDelete deletes an environemment  variable, a permission is required.
func envDelete(ctx *core.Context, _name core.Str) error {
	name := string(_name)

	perm := core.EnvVarPermission{Kind_: core.DeletePerm, Name: name}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	return os.Unsetenv(name)
}
