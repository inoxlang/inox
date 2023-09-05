package internal

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/go-git/go-billy/v5/util"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/default_state"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/utils"

	_ "github.com/inoxlang/inox/internal/obs_db"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"

	"github.com/stretchr/testify/assert"
)

var (
	CLOUDFLARE_ACCOUNT_ID                  = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN = os.Getenv("CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN")

	OS_DB_TEST_ACCESS_KEY_ENV_VARNAME = "OS_DB_TEST_ACCESS_KEY"
	OS_DB_TEST_ACCESS_KEY             = os.Getenv(OS_DB_TEST_ACCESS_KEY_ENV_VARNAME)
	OS_DB_TEST_SECRET_KEY             = os.Getenv("OS_DB_TEST_SECRET_KEY")
	OS_DB_TEST_ENDPOINT               = os.Getenv("OS_DB_TEST_ENDPOINT")
)

func TestPrepareLocalScript(t *testing.T) {

	t.Run("recoverable parsing error", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
			a = ;
			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present because we can still make perform static check
		if !assert.NotNil(t, state) {
			return
		}
		if !assert.True(t, state.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("preinit block defines a pattern used in the manifest", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			preinit {
				%patt = %/...
			}
			manifest {
				permissions: {
					read: %patt
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.True(t, state.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("preinit block defines a host alias used in the manifest", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			preinit {
				@host = https://localhost
			}
			manifest {
				permissions: {
					read: @host/
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.True(t, state.Ctx.HasPermission(core.CreateHttpReadPerm(core.URL("https://localhost/")))) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("preinit-files", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				preinit-files: {
					FILE: {
						path: /file.txt
						pattern: %str
					}
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		preinitFs := fs_ns.NewMemFilesystem(100)
		util.WriteFile(preinitFs, "/file.txt", nil, 0o600)

		state, mod, manifest, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem: preinitFs,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		//the manifest should contain the preinit config.

		if !assert.Len(t, manifest.PreinitFiles, 1) {
			return
		}

		assert.Equal(t, &core.PreinitFile{
			Name:    "FILE",
			Path:    "/file.txt",
			Pattern: core.STR_PATTERN,
			Content: []byte{},
			Parsed:  core.Str(""),
			RequiredPermission: core.FilesystemPermission{
				Kind_:  permkind.Read,
				Entity: core.Path("/file.txt"),
			},
		}, manifest.PreinitFiles[0])

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("local database", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
					write: %/...
				}
				databases: {
					local: {
						resource: ldb://main
						resolution-data: /
					}
				}
			}
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(1000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
			),
			Filesystem: fs,
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		//the state should contain the database.

		if !assert.Contains(t, state.Databases, "local") {
			return
		}

		assert.Equal(t, core.Host("ldb://main"), state.Databases["local"].Resource())
	})

	t.Run("local database + expected schema update: the entities should not be loaded", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
					write: %/...
				}
				databases: {
					local: {
						resource: ldb://main
						resolution-data: /
						expected-schema-update: true
					}
				}
			}

			dbs.local.update_schema(%{})
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(1000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
			),
			Filesystem: fs,
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		//the state should contain the database.

		if !assert.Contains(t, state.Databases, "local") {
			return
		}
		db := state.Databases["local"]

		assert.Equal(t, core.Host("ldb://main"), db.Resource())
		assert.False(t, db.TopLevelEntitiesLoaded())
	})

	t.Run("local database set by main module and accessed by external module", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(10_000)

		util.WriteFile(fls, "/main.ix", []byte(`
			manifest {
				permissions: {
					read: %/...
					write: %/...
				}
				databases: {
					local: {
						resource: ldb://main
						resolution-data: /
					}
				}
			}
		`), 0o600)

		util.WriteFile(fls, "/executed.ix", []byte(`
			manifest {
				databases: /main.ix
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
			),
			Filesystem: fls,
		})
		core.NewGlobalState(ctx)

		mainState, _, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     "/main.ix",
			ParsingCompilationContext: ctx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:     fls,
			FullAccessToDatabases: true,
		})

		if !assert.NoError(t, err) {
			return
		}

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     "/executed.ix",
			ParsingCompilationContext: mainState.Ctx,
			ParentContext:             mainState.Ctx,
			UseParentStateAsMainState: true,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:     fls,
			FullAccessToDatabases: true,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		//the state should contain the database.

		if !assert.Contains(t, state.Databases, "local") {
			return
		}

		assert.Equal(t, core.Host("ldb://main"), state.Databases["local"].Resource())
	})

	t.Run("manifest & symbolic eval should be ignored when there is a preinit check error: dev mode", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			preinit {
				go do {}
			}
			manifest {
				permissions: {
					read: https://localhost
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DevMode:                   true,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// manifest should be empty
		if !assert.False(t, state.Ctx.HasPermission(core.CreateHttpReadPerm(core.URL("https://localhost/")))) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("object storage database", func(t *testing.T) {
		if OS_DB_TEST_ACCESS_KEY == "" {
			t.SkipNow()
			return
		}

		s3Host := randS3Host()
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {}
				host-resolution: :{
					`+string(s3Host)+` : {
						bucket: "test"
						provider: "cloudflare"
						host: `+OS_DB_TEST_ENDPOINT+`
						access-key: access-key
						secret-key: secret-key
					}
				}
				databases: {
					db: {
						resource: odb://main
						resolution-data: `+string(s3Host)+`
					}
				}
			}
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(1000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			),
			Filesystem: fs,
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
			AdditionalGlobalsTestOnly: map[string]core.Value{
				"access-key": core.Str(OS_DB_TEST_ACCESS_KEY),
				"secret-key": utils.Must(core.SECRET_STRING_PATTERN.NewSecret(
					core.NewContexWithEmptyState(core.ContextConfig{}, nil),
					OS_DB_TEST_SECRET_KEY,
				)),
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		//the state should contain the database.

		if !assert.Contains(t, state.Databases, "db") {
			return
		}

		assert.Equal(t, core.Host("odb://main"), state.Databases["db"].Resource())
	})

	t.Run("object storage database + expected schema update: the entities should not be loaded", func(t *testing.T) {
		if OS_DB_TEST_ACCESS_KEY == "" {
			t.SkipNow()
			return
		}

		s3Host := randS3Host()
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {}
				host-resolution: :{
					`+string(s3Host)+` : {
						bucket: "test"
						provider: "cloudflare"
						host: `+OS_DB_TEST_ENDPOINT+`
						access-key: access-key
						secret-key: secret-key
					}
				}
				databases: {
					db: {
						resource: odb://main
						resolution-data: `+string(s3Host)+`
						expected-schema-update: true
					}
				}
			}

			dbs.db.update_schema(%{})
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(1000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			),
			Filesystem: fs,
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,

			AdditionalGlobalsTestOnly: map[string]core.Value{
				"access-key": core.Str(OS_DB_TEST_ACCESS_KEY),
				"secret-key": utils.Must(core.SECRET_STRING_PATTERN.NewSecret(
					core.NewContexWithEmptyState(core.ContextConfig{}, nil),
					OS_DB_TEST_SECRET_KEY,
				)),
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		//the state should contain the database.

		if !assert.Contains(t, state.Databases, "db") {
			return
		}
		db := state.Databases["db"]

		assert.Equal(t, core.Host("odb://main"), db.Resource())
		assert.False(t, db.TopLevelEntitiesLoaded())
	})

	t.Run("project", func(t *testing.T) {
		if CLOUDFLARE_ACCOUNT_ID == "" {
			t.Skip()
			return
		}

		//create project with a secret
		var proj *project.Project
		projectName := "test-mod-prep"
		{
			tempCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			registry, err := project.OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000))
			if !assert.NoError(t, err) {
				return
			}

			id, err := registry.CreateProject(tempCtx, project.CreateProjectParams{
				Name: projectName,
			})

			if !assert.NoError(t, err) {
				return
			}

			p, err := registry.OpenProject(tempCtx, project.OpenProjectParams{
				Id: id,
				DevSideConfig: project.DevSideProjectConfig{
					Cloudflare: &project.DevSideCloudflareConfig{
						AdditionalTokensApiToken: CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN,
						AccountID:                CLOUDFLARE_ACCOUNT_ID,
					},
				},
			})

			if !assert.NoError(t, err) {
				return
			}

			proj = p

			err = p.UpsertSecret(tempCtx, "my-secret", "secret")
			if !assert.NoError(t, err) {
				return
			}

			defer func() {
				//delete tokens & bucket

				err := proj.DeleteSecretsBucket(tempCtx)
				assert.NoError(t, err)

				api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
				if err != nil {
					return
				}

				apiTokens, err := api.APITokens(tempCtx)
				if err != nil {
					return
				}

				for _, token := range apiTokens {
					if strings.Contains(token.Name, projectName) {
						err := api.DeleteAPIToken(tempCtx, token.ID)
						if err != nil {
							t.Log(err)
						}
					}
				}

			}()
		}

		fs := fs_ns.NewMemFilesystem(10000)

		util.WriteFile(fs, "/script.ix", []byte(`
			manifest {
				permissions: {}
			}
			return `+default_state.PROJECT_SECRETS_GLOBAL_NAME+`.my-secret
		`), 0o600)

		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     "/script.ix",
			ParsingCompilationContext: ctx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			Project: proj,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		projectSecrets, ok := state.Globals.CheckedGet(default_state.PROJECT_SECRETS_GLOBAL_NAME)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, []string{"my-secret"}, projectSecrets.(*core.Record).Keys())
	})

	t.Run("object storage database with credentials provided by the project", func(t *testing.T) {
		if CLOUDFLARE_ACCOUNT_ID == "" {
			t.Skip()
			return
		}

		//create project
		var proj *project.Project
		projectName := "test-mod-prep-creds-from-project"
		{
			tempCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			registry, err := project.OpenRegistry("/", fs_ns.NewMemFilesystem(100_000_000))
			if !assert.NoError(t, err) {
				return
			}

			id, err := registry.CreateProject(tempCtx, project.CreateProjectParams{
				Name: projectName,
			})

			if !assert.NoError(t, err) {
				return
			}

			p, err := registry.OpenProject(tempCtx, project.OpenProjectParams{
				Id: id,
				DevSideConfig: project.DevSideProjectConfig{
					Cloudflare: &project.DevSideCloudflareConfig{
						AdditionalTokensApiToken: CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN,
						AccountID:                CLOUDFLARE_ACCOUNT_ID,
					},
				},
			})

			if !assert.NoError(t, err) {
				return
			}

			proj = p

			defer func() {
				//delete tokens & bucket
				api, err := cloudflare.NewWithAPIToken(CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN)
				if err != nil {
					return
				}

				apiTokens, err := api.APITokens(tempCtx)
				if err != nil {
					return
				}

				for _, token := range apiTokens {
					if strings.Contains(token.Name, projectName) {
						err := api.DeleteAPIToken(tempCtx, token.ID)
						if err != nil {
							t.Log(err)
						}
					}
				}

			}()
		}

		fs := fs_ns.NewMemFilesystem(10000)
		s3Host := randS3Host()

		util.WriteFile(fs, "/script.ix", []byte(`
			manifest {
				permissions: {}
				host-resolution: :{
					`+string(s3Host)+` : {
						bucket: "test"
						provider: "cloudflare"
						host: `+OS_DB_TEST_ENDPOINT+`
					}
				}
				databases: {
					db: {
						resource: odb://main
						resolution-data: `+string(s3Host)+`
					}
				}
			}
		`), 0o600)

		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     "/script.ix",
			ParsingCompilationContext: ctx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,

			Project: proj,
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		// static check should have been performed
		if !assert.Empty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())

		//the state should contain the database.

		if !assert.Contains(t, state.Databases, "db") {
			return
		}

		assert.Equal(t, core.Host("odb://main"), state.Databases["db"].Resource())
	})

	t.Run("manifest & symbolic eval should be ignored when there is a preinit check error: regular mode", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			preinit {
				go do {}
			}
			manifest {
				permissions: {
					read: https://localhost
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DevMode:                   false,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should not be present as we are not in dev mode
		assert.Nil(t, state)
	})

	t.Run("manifest & symbolic eval should be ignored when there is a manifest check error: dev mode", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: https://localhost
				}
				env: 1
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DevMode:                   true,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.NotEmpty(t, state.PrenitStaticCheckErrors) {
			return
		}

		//there should not be duplicate errors
		{
			msgs := map[string]struct{}{}
			for _, err := range state.PrenitStaticCheckErrors {
				if _, ok := msgs[err.Message]; ok {
					assert.Fail(t, "there should not be duplicate errors: duplicate error found: %s", err.Message)
					return
				}
				msgs[err.Message] = struct{}{}
			}
		}

		// manifest should be empty
		if !assert.False(t, state.Ctx.HasPermission(core.CreateHttpReadPerm(core.URL("https://localhost/")))) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("manifest & symbolic eval should be ignored when there is a manifest check error: regular mode", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: https://localhost
				}
				env: 1
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.CreateHttpReadPerm(core.Host("https://localhost")),
			),
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DevMode:                   false,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should not be present as we are not in dev mode
		assert.Nil(t, state)
	})

	t.Run("invalid CLI arguments: missing positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path}
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{}, //missing file argument
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("CLI: too many positional arguments", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{"true"}, //too many arguments
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("CLI: unknown argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			CliArgs:                   []string{"-x"}, //unknown argument
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("missing positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path}
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     file,
			Args:                      core.NewStructFromMap(map[string]core.Value{}),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("missing non positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path},
					output: %path
				}
			}
		
		`), 0o600)

		state := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(state)

		res, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewStructFromMap(map[string]core.Value{
				"0": core.Path("./a.txt"),
			}),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             state,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, res) {
			return
		}
	})

	t.Run("invalid value for positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path}
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewStructFromMap(map[string]core.Value{
				"0": core.True,
			}),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("too many positional arguments", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewStructFromMap(map[string]core.Value{
				"0": core.True,
			}),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("invalid value for non positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					{name: #file, pattern: %path},
					output: %path
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewStructFromMap(map[string]core.Value{
				"0":      core.Path("./a.txt"),
				"output": core.True,
			}),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("unknown non positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {
					output: %path
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewStructFromMap(map[string]core.Value{
				"outpu": core.True, //unknown argument
			}),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

	t.Run("unknown argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath: file,
			Args: core.NewStructFromMap(map[string]core.Value{
				"x": core.True, //unknown argument
			}),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present
		if !assert.NotNil(t, state) {
			return
		}
	})

}

func TestPrepareDevModeIncludableChunkFile(t *testing.T) {

	t.Run("recoverable parsing error", func(t *testing.T) {
		fs := fs_ns.NewMemFilesystem(10000)

		util.WriteFile(fs, "/included.ix", []byte(`
			includable-chunk

			a = ;
			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		
		`), 0o600)

		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, _, _, err := inox_ns.PrepareDevModeIncludableChunkfile(inox_ns.IncludableChunkfilePreparationArgs{
			Fpath:                          "/included.ix",
			ParsingContext:                 ctx,
			LogOut:                         io.Discard,
			Out:                            io.Discard,
			IncludedChunkContextFileSystem: fs,
		})

		if !assert.Error(t, err) {
			return
		}

		// the state should be present because we can still make perform static check
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.NotEmpty(t, state.Module.ParsingErrors) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("static check error", func(t *testing.T) {
		fs := fs_ns.NewMemFilesystem(10000)

		util.WriteFile(fs, "/included.ix", []byte(`
			includable-chunk

			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		`), 0o600)

		ctx := core.NewContexWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, _, _, err := inox_ns.PrepareDevModeIncludableChunkfile(inox_ns.IncludableChunkfilePreparationArgs{
			Fpath:                          "/included.ix",
			ParsingContext:                 ctx,
			LogOut:                         io.Discard,
			Out:                            io.Discard,
			IncludedChunkContextFileSystem: fs,
		})

		if !assert.Error(t, err) {
			return
		}

		// the state should be present because we can still make perform static check
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.Empty(t, state.Module.ParsingErrors) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})
}

func TestRunLocalScript(t *testing.T) {

	createEvaluationCtx := func(dir string) *core.Context {
		perms := core.GetDefaultGlobalVarPermissions()
		perms = append(perms, core.CreateFsReadPerm(core.PathPattern(dir+"/...")))

		ctx := core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)
		return ctx
	}

	//TODO: improve tests

	t.Run("a script with static check errors should not be runned", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		os.WriteFile(file, []byte("fn(){self}; return 1"), 0o600)

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
		})

		assert.Error(t, err)
		assert.Nil(t, state)
	})

	t.Run("a script with static check errors in the preinit-files section should not be runned", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		os.WriteFile(file, []byte(`
			manifest {
				preinit-files: {
					A: {path: /a, pattern: %str},
					A: {path: /b, pattern: %str},
				}
			}
		`), 0o600)

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
		})

		assert.Error(t, err)
		assert.Nil(t, state)
	})

	t.Run("too many warnings", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		manySpawnExprs := strings.Repeat("go do idt(1)\n", inox_ns.DEFAULT_MAX_ALLOWED_WARNINGS+1)

		os.WriteFile(file, []byte("manifest {}\n"+manySpawnExprs), 0o600)

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       true,
		})

		if !assert.ErrorIs(t, err, inox_ns.ErrExecutionAbortedTooManyWarnings) {
			return
		}

		assert.Nil(t, state)
	})

	t.Run("too wide preinit permissions", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")

		os.WriteFile(file, []byte(`
			manifest {
				preinit-files: {
					FILE1: {
						path: /file1.txt
						pattern: %str
					}
					FILE2: {
						path: /file2.txt
						pattern: %str
					}
					FILE3: {
						path: /file3.txt
						pattern: %str
					}
					FILE4: {
						path: /file4.txt
						pattern: %str
					}
					FILE5: {
						path: /file5.txt
						pattern: %str
					}
					FILE6: {
						path: /file6.txt
						pattern: %str
					}
					FILE7: {
						path: /file7.txt
						pattern: %str
					}
					FILE8: {
						path: /file8.txt
						pattern: %str
					}
					FILE9: {
						path: /file9.txt
						pattern: %str
					}
				}
			}
		`), 0o600)

		preinitFs := fs_ns.NewMemFilesystem(100)
		for i := 1; i <= 9; i++ {
			util.WriteFile(preinitFs, fmt.Sprintf("/file%d.txt", i), nil, 0o600)
		}

		state, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     file,
			ParsingCompilationContext: createCompilationCtx(dir),
			ParentContextRequired:     true,
			ParentContext:             createEvaluationCtx(dir),
			Out:                       io.Discard,
			IgnoreHighRiskScore:       false, //<---
			PreinitFilesystem:         preinitFs,
		})

		if !assert.ErrorIs(t, err, inox_ns.ErrNoProvidedConfirmExecPrompt) {
			return
		}

		assert.Nil(t, state)
	})
}

func createCompilationCtx(dir string) *core.Context {
	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.CreateFsReadPerm(core.PathPattern(dir + "/...")),
		},
		Filesystem: fs_ns.GetOsFilesystem(),
	})
	core.NewGlobalState(compilationCtx)
	return compilationCtx
}

func randS3Host() core.Host {
	return core.Host("s3://bucket-" + strconv.Itoa(int(rand.Int31())))
}
