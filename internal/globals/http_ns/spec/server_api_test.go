package spec

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils"
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
				core.GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				core.GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
				core.GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				core.LThreadPermission{Kind_: permbase.Create},
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			}

			permissions = append(permissions, config.Permissions...)

			ctx := core.NewContext(core.ContextConfig{
				Permissions:          permissions,
				ForbiddenPermissions: config.ForbiddenPermissions,
				HostDefinitions:      config.HostDefinitions,
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
	testconfig.AllowParallelization(t)

	//Create a context and a filesystem with the passed file contents.
	//if no content is defined for /main.ix the file containing `manifest {}` is created.
	//A state for an in-memory module is created by default.
	setup := func(files map[string]string, noState ...bool) *core.Context {
		fls := fs_ns.NewMemFilesystem(10_000)

		var ctx *core.Context
		perms := append(core.GetDefaultGlobalVarPermissions(),
			core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			core.FilesystemPermission{Kind_: permbase.Write, Entity: core.PathPattern("/...")},
			core.LThreadPermission{Kind_: permbase.Create},
		)

		if len(noState) == 0 || !noState[0] {
			ctx = core.NewContextWithEmptyState(core.ContextConfig{
				Permissions: perms,
				Filesystem:  fls,
			}, nil)

			state := ctx.MustGetClosestState()

			state.Module = utils.Must(core.ParseInMemoryModule("manifest {}", core.InMemoryModuleParsingConfig{
				Name:    "in-mem-module",
				Context: ctx,
			}))

		} else {
			ctx = core.NewContext(core.ContextConfig{
				Permissions: perms,
				Filesystem:  fls,
			})
		}

		fls.MkdirAll("/routes/", 0o700)

		if _, ok := files["/main.ix"]; !ok {
			util.WriteFile(fls, "/main.ix", []byte("manifest {}"), 0700)
		}

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
		testconfig.AllowParallelization(t)

		t.Run("root index.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/index.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
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
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/users/index.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
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

		t.Run("root index.ix requires access to databases defined in main", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/main.ix": `
					manifest {
						# No need to define a database, we just want GetFSRoutingServerAPI 
						# to prepare the module.
					}
				`,
				"/routes/index.ix": `
					manifest {
						databases: /main.ix
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
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

		t.Run("root index.ix requires access to databases defined in main and the initiator of the retrieval is the /main.ix module ", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			noState := true
			ctx := setup(map[string]string{
				"/main.ix": `
					manifest {}
				`,
				"/routes/index.ix": `
					manifest {
						databases: /main.ix
						parameters: {}
					}
				`,
			}, noState)
			defer ctx.CancelGracefully()

			mainModState := core.NewGlobalState(ctx)
			mainModState.OutputFieldsInitialized.Store(true)
			mainModState.Module = utils.Must(core.ParseLocalModule("/main.ix", core.ModuleParsingConfig{
				Context: ctx,
			}))
			mainModState.Manifest = core.NewEmptyManifest()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
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

		t.Run("root GET.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/GET.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/") {
				return
			}

			endpt := api.endpoints["/"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			assert.NotNil(t, endpt.operations[0].handlerModule)
		})

		t.Run("non root GET.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/users/GET.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}

			endpt := api.endpoints["/users"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			assert.NotNil(t, endpt.operations[0].handlerModule)
		})

		t.Run("root GET-users.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}

			endpt := api.endpoints["/users"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			assert.NotNil(t, endpt.operations[0].handlerModule)
		})

		t.Run("non root GET-users.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}

			endpt := api.endpoints["/users"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			assert.NotNil(t, endpt.operations[0].handlerModule)
		})

		t.Run("GET-users.ix in a `users` directory", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/users/GET-users.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}

			endpt := api.endpoints["/users"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			operation := endpt.operations[0]
			assert.NotNil(t, operation.handlerModule)
			assert.Equal(t, "GET", operation.httpMethod)
		})

		t.Run("POST-users.ix in a `users` directory", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/users/POST-users.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users") {
				return
			}

			endpt := api.endpoints["/users"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			operation := endpt.operations[0]
			assert.NotNil(t, operation.handlerModule)
			assert.Equal(t, "POST", operation.httpMethod)
		})

		t.Run("non-root users.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/x/users.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/x/users") {
				return
			}

			endpt := api.endpoints["/x/users"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			assert.NotNil(t, endpt.operations[0].handlerModule)
			assert.Nil(t, endpt.methodAgnosticHandler)

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
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/a.spec.ix": `
					manifest {
						parameters: {}
					}
				`,
				"/routes/b.spec.ix": `
					manifest {}
				`,
				"/routes/c.ix": `
					manifest {
						parameters: {}
					}
				`,
				"/routes/d/e.ix": `
					manifest {
						parameters: {}
					}
				`,
				"/routes/d/f.spec.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
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

		t.Run("GET.ix in a parametric directory", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/users/:user-id/GET.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			if !assert.NoError(t, err) {
				return
			}

			if !assert.Contains(t, api.endpoints, "/users/{user-id}") {
				return
			}

			endpt := api.endpoints["/users/{user-id}"]
			if !assert.Len(t, endpt.operations, 1) {
				return
			}

			assert.NotNil(t, endpt.operations[0].handlerModule)
		})
	})

	t.Run("cache", func(t *testing.T) {
		testconfig.AllowParallelization(t)
		code := `
			manifest {
				parameters: {}
			}
		`

		ctx := setup(map[string]string{
			"/routes/index.ix": code,
		})
		defer ctx.CancelGracefully()

		//Create the cache and parse the module.

		cache := parse.NewChunkCache()

		cachedChunk, err := parse.ParseChunkSource(parse.SourceFile{
			NameString:             "/routes/index.ix",
			UserFriendlyNameString: "/routes/index.ix",
			Resource:               "/routes/index.ix",
			ResourceDir:            "/",
			CodeString:             code,
		}, parse.ParserOptions{
			ParsedFileCache: cache,
		})

		if !assert.NoError(t, err) {
			return
		}

		//Get the API

		api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{
			DynamicDir:     "/routes/",
			InoxChunkCache: cache,
		})
		if !assert.NoError(t, err) {
			return
		}

		endpoint, err := api.GetOperation("GET", "/")

		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, endpoint.handlerModule.MainChunkTopLevelNodeIs(cachedChunk.Node))
	})

	t.Run("caller is not a module (no state)", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			noState := true
			ctx := setup(map[string]string{
				"/routes/index.ix": `
				manifest {
					parameters: {}
				}
			`,
			}, noState)

			defer ctx.CancelGracefully()

			//Get the API

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{
				DynamicDir:              "/routes/",
				FallbackMainProgramPath: "/main.ix",
				FallbackProject:         project.NewDummyProject("test", ctx.GetFileSystem().(core.SnapshotableFilesystem)),
			})

			if !assert.NoError(t, err) {
				return
			}

			_, err = api.GetOperation("GET", "/")

			assert.NoError(t, err)
		})

		t.Run("an error should be returned if the fallback project is not specified", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			noState := true
			ctx := setup(map[string]string{
				"/routes/index.ix": `
				manifest {
					parameters: {}
				}
			`,
			}, noState)

			defer ctx.CancelGracefully()

			//Get the API

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{
				DynamicDir:              "/routes/",
				FallbackMainProgramPath: "/main.ix",
			})

			assert.ErrorIs(t, err, ErrFallbackProjectNotSet)
		})

		t.Run("an error should be returned if the fallback main program path is not specified", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			noState := true
			ctx := setup(map[string]string{
				"/routes/index.ix": `
				manifest {
					parameters: {}
				}
			`,
			}, noState)

			defer ctx.CancelGracefully()

			//Get the API

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{
				DynamicDir:      "/routes/",
				FallbackProject: project.NewDummyProject("test", ctx.GetFileSystem().(core.SnapshotableFilesystem)),
			})

			assert.ErrorIs(t, err, ErrFallbackMainProgramPathNotSet)
		})

	})

	t.Run("an error is expected if at least two modules handle the same API operation", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("GET-users.ix + /users/index.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/GET.ix": `
					manifest {
						parameters: {}
					}
				`,
				"/routes/index.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			assert.ErrorContains(t, err, "already implemented")
		})

		t.Run("GET-users.ix + /users/index.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {}
					}
				`,
				"/routes/users/index.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			assert.ErrorContains(t, err, "already implemented")
		})

		t.Run("GET-users.ix + /users/GET.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/GET-users.ix": `
					manifest {
						parameters: {}
					}
				`,
				"/routes/users/GET.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			assert.ErrorContains(t, err, "already implemented")
		})

		t.Run("users.ix + /users/GET.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			ctx := setup(map[string]string{
				"/routes/users.ix": `
					manifest {
						parameters: {}
					}
				`,
				"/routes/users/GET.ix": `
					manifest {
						parameters: {}
					}
				`,
			})
			defer ctx.CancelGracefully()

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})
			assert.ErrorContains(t, err, "already implemented")
		})

	})

	t.Run("GET & OPTIONS handler should not have request body parameters", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("GET.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

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

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInGETHandler)
		})

		t.Run("GET-users.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

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

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInGETHandler)
		})

		t.Run("OPTIONS.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

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

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInOPTIONSHandler)
		})

		t.Run("OPTIONS-users.ix", func(t *testing.T) {
			testconfig.AllowParallelization(t)

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

			_, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

			assert.ErrorIs(t, err, ErrUnexpectedBodyParamsInOPTIONSHandler)
		})
	})

	t.Run("parameters", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("POST with a request body parameter", func(t *testing.T) {
			testconfig.AllowParallelization(t)

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

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

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
			testconfig.AllowParallelization(t)

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

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

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
			testconfig.AllowParallelization(t)

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

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

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
			testconfig.AllowParallelization(t)

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

			api, err := GetFSRoutingServerAPI(ctx, ServerApiResolutionConfig{DynamicDir: "/routes/"})

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
