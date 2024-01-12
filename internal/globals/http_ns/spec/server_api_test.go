package spec

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/stretchr/testify/assert"
)

func init() {
	if !core.AreDefaultScriptLimitsSet() {
		core.SetDefaultScriptLimits([]core.Limit{})
	}

	core.RegisterDefaultPatternNamespace("http", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"method": METHOD_PATTERN,
		},
	})

	if core.NewDefaultContext == nil {
		core.SetNewDefaultContext(func(config core.DefaultContextConfig) (*core.Context, error) {

			if len(config.OwnedDatabases) != 0 {
				panic(errors.New("not supported"))
			}

			permissions := []core.Permission{
				core.GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				core.GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
				core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				core.LThreadPermission{Kind_: permkind.Create},
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			}

			permissions = append(permissions, config.Permissions...)

			ctx := core.NewContext(core.ContextConfig{
				Permissions:          permissions,
				ForbiddenPermissions: config.ForbiddenPermissions,
				HostResolutions:      config.HostResolutions,
				ParentContext:        config.ParentContext,
			})

			for k, v := range core.DEFAULT_NAMED_PATTERNS {
				ctx.AddNamedPattern(k, v)
			}

			for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
				ctx.AddPatternNamespace(k, v)
			}

			return ctx, nil
		})

		core.SetNewDefaultGlobalStateFn(func(ctx *core.Context, conf core.DefaultGlobalStateConfig) (*core.GlobalState, error) {
			state := core.NewGlobalState(ctx, map[string]core.Value{
				"html":  core.ValOf(html_ns.NewHTMLNamespace()),
				"sleep": core.WrapGoFunction(core.Sleep),
			})

			return state, nil
		})
	}

	if !core.IsSymbolicEquivalentOfGoFunctionRegistered(core.Sleep) {
		core.RegisterSymbolicGoFunction(core.Sleep, func(ctx *symbolic.Context, _ *symbolic.Duration) {

		})
	}
}

func TestGetFilesystemRoutingServerAPI(t *testing.T) {
	t.Parallel()

	//create a context and a filesystem with the passed file contents.
	setup := func(files map[string]string) *core.Context {
		fls := fs_ns.NewMemFilesystem(10_000)

		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
				core.LThreadPermission{Kind_: permkind.Create},
			),
			Filesystem: fls,
		}, nil)

		fls.MkdirAll("/routes/", 0o700)

		for file, content := range files {
			fls.MkdirAll(filepath.Dir(file), 0700)

			err := util.WriteFile(fls, file, []byte(content), 0o700)
			if err != nil {
				assert.FailNow(t, err.Error())
			}
		}

		return ctx
	}

	t.Run("base cases", func(t *testing.T) {
		t.Parallel()

		t.Run("root index.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/index.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/") {
				return
			}
			if !assert.NotNil(t, api.tree) {
				return
			}

			assert.NotNil(t, api.tree.endpoint)
			assert.Equal(t, "/", api.tree.path)
			assert.Equal(t, "", api.tree.segment)
			assert.Empty(t, api.tree.namedChildren)
			assert.Nil(t, api.tree.parametrizedChild)
		})

		t.Run("non root index.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/users/index.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}
			assert.Nil(t, api.tree.endpoint)
			assert.Equal(t, "/", api.tree.path)
			assert.Equal(t, "", api.tree.segment)

			if !assert.Contains(t, api.tree.namedChildren, "users") {
				return
			}

			childNode := api.tree.namedChildren["users"]
			assert.Same(t, api.endpoints["/users"], childNode.endpoint)
			assert.Equal(t, "/users", childNode.path)
			assert.Equal(t, "users", childNode.segment)
		})

		t.Run("root GET.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/") {
				return
			}
		})

		t.Run("non root GET.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/users/GET.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}
		})

		t.Run("root GET-users.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}
		})

		t.Run("non root GET-users.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}
		})

		t.Run("deep GET.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/x/users.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/x/users") {
				return
			}

			assert.Nil(t, api.tree.endpoint)
			assert.Equal(t, "/", api.tree.path)
			assert.Equal(t, "", api.tree.segment)

			if !assert.Contains(t, api.tree.namedChildren, "x") {
				return
			}

			childNode := api.tree.namedChildren["x"]
			assert.Nil(t, childNode.endpoint)
			assert.Equal(t, "/x", childNode.path)
			assert.Equal(t, "x", childNode.segment)

			if !assert.Contains(t, childNode.namedChildren, "users") {
				return
			}

			childNode = childNode.namedChildren["users"]
			assert.Same(t, api.endpoints["/x/users"], childNode.endpoint)
			assert.Equal(t, "/x/users", childNode.path)
			assert.Equal(t, "users", childNode.segment)
		})

		t.Run(".spec.ix files should be ignored", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/a.spec.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
				"/routes/b.spec.ix": `
					manifest {
					}
				`,
				"/routes/c.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
				"/routes/d/e.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
				"/routes/d/f.spec.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.NotContains(t, api.endpoints, "/a") {
				return
			}

			if !assert.NotContains(t, api.endpoints, "/b") {
				return
			}

			if !assert.Contains(t, api.endpoints, "/c") {
				return
			}

			if !assert.Contains(t, api.endpoints, "/d/e") {
				return
			}

			if !assert.NotContains(t, api.endpoints, "/d/f") {
				return
			}

			assert.NotNil(t, api.tree)
		})
	})

	t.Run("an error is expected if at least two modules handle the same API operation", func(t *testing.T) {
		t.Parallel()

		t.Run("GET-users.ix + /users/index.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET.ix": `
					manifest {
						parameters: {

						}
					}
				`,
				"/routes/index.ix": `
					manifest {
						parameters: {

						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			assert.ErrorContains(t, err, "already implemented")
		})

		t.Run("GET-users.ix + /users/index.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {

						}
					}
				`,
				"/routes/users/index.ix": `
					manifest {
						parameters: {

						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			assert.ErrorContains(t, err, "already implemented")
		})

		t.Run("GET-users.ix + /users/GET.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {

						}
					}
				`,
				"/routes/users/GET.ix": `
					manifest {
						parameters: {

						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			assert.ErrorContains(t, err, "already implemented")
		})

		t.Run("users.ix + /users/GET.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/users.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
				"/routes/users/GET.ix": `
					manifest {
						parameters: {
	
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})
			assert.ErrorContains(t, err, "already implemented")
		})

	})

	t.Run("GET & OPTIONS handler should not have request body parameters", func(t *testing.T) {
		t.Parallel()

		t.Run("GET.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET.ix": `
					manifest {
						parameters: {
							name: %str
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInGETHandler)
		})

		t.Run("GET-users.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {
							id: %str
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInGETHandler)
		})

		t.Run("OPTIONS.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/OPTIONS.ix": `
					manifest {
						parameters: {
							name: %str
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInOPTIONSHandler)
		})

		t.Run("OPTIONS-users.ix", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/OPTIONS-users.ix": `
					manifest {
						parameters: {
							id: %str
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInOPTIONSHandler)
		})
	})

	t.Run("parameters", func(t *testing.T) {
		t.Parallel()

		t.Run("POST with a request body parameter", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/POST-users.ix": `
					manifest {
						parameters: {
							name: %str
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			if !assert.NoError(t, err, ErrUnexpectedBodyParamsInGETHandler) {
				return
			}

			usersEndpt := api.endpoints["/users"]
			if !assert.Len(t, usersEndpt.operations, 1) {
				return
			}

			operation := usersEndpt.operations[0]
			if !assert.Equal(t, operation.httpMethod, "POST") {
				return
			}

			if !assert.IsType(t, (*core.ObjectPattern)(nil), operation.jsonRequestBody) {
				return
			}

			pattern := operation.jsonRequestBody.(*core.ObjectPattern)
			if !assert.Equal(t, 1, pattern.EntryCount()) {
				return
			}

			namePattern, optional, ok := pattern.Entry("name")
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, namePattern, core.STR_PATTERN)
			assert.False(t, optional)
		})

		t.Run("POST with an injected parameter and no request body parameters", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/POST-users.ix": `
					manifest {
						parameters: {
							_body: %reader
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			if !assert.NoError(t, err, ErrUnexpectedBodyParamsInGETHandler) {
				return
			}

			usersEndpt := api.endpoints["/users"]
			if !assert.Len(t, usersEndpt.operations, 1) {
				return
			}

			operation := usersEndpt.operations[0]
			if !assert.Equal(t, operation.httpMethod, "POST") {
				return
			}

			assert.Nil(t, operation.jsonRequestBody)
		})

		t.Run("POST with a request body parameter and an injected parameter", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/POST-users.ix": `
					manifest {
						parameters: {
							name: %str
							_method: %http.method
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			if !assert.NoError(t, err, ErrUnexpectedBodyParamsInGETHandler) {
				return
			}

			usersEndpt := api.endpoints["/users"]
			if !assert.Len(t, usersEndpt.operations, 1) {
				return
			}

			operation := usersEndpt.operations[0]
			if !assert.Equal(t, operation.httpMethod, "POST") {
				return
			}

			if !assert.IsType(t, (*core.ObjectPattern)(nil), operation.jsonRequestBody) {
				return
			}

			pattern := operation.jsonRequestBody.(*core.ObjectPattern)
			if !assert.Equal(t, 1, pattern.EntryCount()) {
				return
			}

			namePattern, optional, ok := pattern.Entry("name")
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, namePattern, core.STR_PATTERN)
			assert.False(t, optional)
		})

		t.Run("POST with two request body parameters", func(t *testing.T) {
			t.Parallel()

			ctx := setup(map[string]string{
				"/routes/POST-users.ix": `
					manifest {
						parameters: {
							name: %str
							age: %int
						}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, "/routes/", ServerApiResolutionConfig{})

			if !assert.NoError(t, err, ErrUnexpectedBodyParamsInGETHandler) {
				return
			}

			usersEndpt := api.endpoints["/users"]
			if !assert.Len(t, usersEndpt.operations, 1) {
				return
			}

			operation := usersEndpt.operations[0]
			if !assert.Equal(t, operation.httpMethod, "POST") {
				return
			}

			if !assert.IsType(t, (*core.ObjectPattern)(nil), operation.jsonRequestBody) {
				return
			}

			pattern := operation.jsonRequestBody.(*core.ObjectPattern)
			if !assert.Equal(t, 2, pattern.EntryCount()) {
				return
			}

			namePattern, optional, ok := pattern.Entry("name")
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, core.STR_PATTERN, namePattern)
			assert.False(t, optional)

			agePattern, optional, ok := pattern.Entry("age")
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, agePattern, core.INT_PATTERN)
			assert.False(t, optional)
		})
	})
}
