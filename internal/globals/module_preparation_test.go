package internal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project/cloudflareprovider"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"

	//_ "github.com/inoxlang/inox/internal/obsdb"

	"github.com/inoxlang/inox/internal/globals/fs_ns"

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

func TestPrepareLocalModule(t *testing.T) {

	t.Run("specified log level", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		outBuf := bytes.NewBuffer(nil)
		logLevel := zerolog.WarnLevel

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       outBuf,
			LogLevels:                 core.NewLogLevels(core.LogLevelsInitialization{DefaultLevel: logLevel}),
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.NoError(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		infoLevelMsg := "this message should not be logged"
		warnLevelMsg := "this message should  be logged"

		state.Logger.Info().Msg(infoLevelMsg)
		state.Logger.Warn().Msg(warnLevelMsg)
		output := outBuf.String()

		assert.NotContains(t, output, infoLevelMsg)
		assert.Contains(t, output, warnLevelMsg)
	})

	t.Run("preinit block defines a pattern used in the manifest", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			preinit {
				pattern patt = %/...
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
		defer ctx.CancelGracefully()

		state, mod, _, err2 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.NoError(t, err2) {
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

	t.Run("the manifest contains a URL expression using a global constant", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			const (
				HOST = https://localhost
			)
			manifest {
				permissions: {
					read: $HOST/
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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
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
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		preinitFs := fs_ns.NewMemFilesystem(100)
		util.WriteFile(preinitFs, "/file.txt", nil, 0o600)

		state, mod, manifest, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       preinitFs,
			ScriptContextFileSystem: fs_ns.GetOsFilesystem(),
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
			Parsed:  core.String(""),
			RequiredPermission: core.FilesystemPermission{
				Kind_:  permbase.Read,
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

	t.Run("manifest checks", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			preinit {
				pattern patt = %/...
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
		defer ctx.CancelGracefully()

		var errInvalidManifest = errors.New("invalid manifest")

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			BeforeContextCreation: func(m *core.Manifest) ([]core.Limit, error) {
				return nil, errInvalidManifest
			},
			ScriptContextFileSystem: fs_ns.GetOsFilesystem(),
		})

		if !assert.ErrorIs(t, err, errInvalidManifest) {
			return
		}

		// the module should not be present
		if !assert.Nil(t, mod) {
			return
		}

		// the state should not be present
		assert.Nil(t, state)
	})

	t.Run("manifest eval & symbolic eval should be ignored when there is a preinit check error: data extraction mode", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DataExtractionMode:        true,
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

	t.Run("project", func(t *testing.T) {
		if CLOUDFLARE_ACCOUNT_ID == "" {
			t.Skip()
			return
		}

		//create project with a secret
		var proj *project.Project
		projectName := "test-mod-prep"
		{
			tempCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer tempCtx.CancelGracefully()
			registry, err := project.OpenRegistry(t.TempDir(), tempCtx)
			if !assert.NoError(t, err) {
				return
			}

			id, _, err := registry.CreateProject(tempCtx, project.CreateProjectParams{
				Name: projectName,
			})

			if !assert.NoError(t, err) {
				return
			}

			p, err := registry.OpenProject(tempCtx, project.OpenProjectParams{
				Id: id,
				DevSideConfig: project.DevSideProjectConfig{
					Cloudflare: &cloudflareprovider.DevSideConfig{
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
			return `+globalnames.PROJECT_SECRETS+`.my-secret
		`), 0o600)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
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

		projectSecrets, ok := state.Globals.CheckedGet(globalnames.PROJECT_SECRETS)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, []string{"my-secret"}, projectSecrets.(*core.Record).Keys())
	})

	t.Run("manifest & symbolic eval should be ignored when there is a preinit check error: regular mode", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DataExtractionMode:        false,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should not be present as we are not in data extraction mode
		assert.Nil(t, state)
	})

	t.Run("manifest & symbolic eval should be ignored when there is a manifest check error: data extraction mode", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DataExtractionMode:        true,
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
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			DataExtractionMode:        false,
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should not be present as we are not in data extraction mode
		assert.Nil(t, state)
	})

	t.Run(".spec.ix modules should be granted wide implicit permissions if testing is enabled", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "file.spec.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
			}
		
		`), 0o600)

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),

			EnableTesting: true,
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

		perms := state.Ctx.GetGrantedPermissions()
		assert.Contains(t, perms, core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")})
		assert.Contains(t, perms, core.FilesystemPermission{Kind_: permbase.Write, Entity: core.PathPattern("/...")})
		assert.Contains(t, perms, core.LThreadPermission{Kind_: permbase.Create})
	})

	t.Run("program testing should be allowed in project mode", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(100_000)

		compilationCtx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.CreateFsReadPerm(core.PathPattern("/...")),
			},
			Filesystem: fls,
		}, nil)
		defer compilationCtx.CancelGracefully()

		util.WriteFile(fls, "/main.spec.ix", []byte(`
			manifest {

			}

			testsuite({
				program: /main.ix
			}){

				testcase {

				}
			}
		
		`), 0o600)

		util.WriteFile(fls, "/main.ix", []byte(`
			manifest {

			}
			
			testsuite()
		
		`), 0o600)

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     "/main.spec.ix",
			ParsingCompilationContext: compilationCtx,
			Out:                       io.Discard,
			Project:                   project.NewDummyProject("project", fls),
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),

			EnableTesting: true,
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
	})

	t.Run("program testing should not be allowed when not in project mode", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(100_000)

		compilationCtx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.CreateFsReadPerm(core.PathPattern("/...")),
			},
			Filesystem: fls,
		}, nil)
		defer compilationCtx.CancelGracefully()

		util.WriteFile(fls, "/main.spec.ix", []byte(`
			manifest {

			}

			testsuite({
				program: /main.ix
			}){

				testcase {

				}
			}
		
		`), 0o600)

		util.WriteFile(fls, "/main.ix", []byte(`
			manifest {

			}
			
			testsuite()
		
		`), 0o600)

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     "/main.spec.ix",
			ParsingCompilationContext: compilationCtx,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),

			EnableTesting: true,
		})

		if !assert.ErrorContains(t, err, symbolic.PROGRAM_TESTING_ONLY_SUPPORTED_IN_PROJECTS) {
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
	})

	t.Run(".spec.ix modules should not be granted wide implicit permissions if testing is disabled", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "file.spec.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
			}
		
		`), 0o600)

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),

			EnableTesting: false,
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

		perms := state.Ctx.GetGrantedPermissions()
		assert.NotContains(t, perms, core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")})
	})

	t.Run("if the OS filesystem used the IWD should be the current working directory", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: IWD
				}
			}
		`), 0o600)

		parsingCompilationCtx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})

		core.NewGlobalState(parsingCompilationCtx)
		defer parsingCompilationCtx.CancelGracefully()

		state, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             parsingCompilationCtx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.NoError(t, err) {
			return
		}

		wd := utils.Must(os.Getwd())

		IWD := state.Globals.Get(globalnames.INITIAL_WORKING_DIR_VARNAME)
		IWD_PREFIX_PATTERN := state.Globals.Get(globalnames.INITIAL_WORKING_DIR_PREFIX_VARNAME)

		if !assert.EqualValues(t, wd+"/", IWD) {
			return
		}
		assert.EqualValues(t, wd+"/...", IWD_PREFIX_PATTERN)

		perms := state.Ctx.GetGrantedPermissions()
		assert.Contains(t, perms, core.FilesystemPermission{Kind_: permbase.Read, Entity: IWD.(core.Path)})
	})

	t.Run("function requiring an early declaration", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {}
		
			return f()

			fn f(){
				return 1
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		outBuf := bytes.NewBuffer(nil)
		logLevel := zerolog.WarnLevel

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       outBuf,
			LogLevels:                 core.NewLogLevels(core.LogLevelsInitialization{DefaultLevel: logLevel}),
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.NoError(t, err) {
			return
		}

		// The state should be present.
		if !assert.NotNil(t, state) {
			return
		}

		assert.Same(t, state.Module, mod)

		earlyDeclarationPosition, ok := state.StaticCheckData.GetEarlyFunctionDeclarationsPosition(mod.MainChunk.Node)
		if !assert.True(t, ok) {
			return
		}

		returnStmt := mod.MainChunk.Node.Statements[0].(*parse.ReturnStatement)

		assert.Equal(t, returnStmt.Span.Start, earlyDeclarationPosition)
	})
}

func TestPrepareLocalModuleWithInvalidInputs(t *testing.T) {

	t.Run(".ParentContext & .StdlibCtx should not be both set", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		stdlibCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		assert.PanicsWithError(t, core.ErrBothParentCtxArgsProvided.Error(), func() {
			core.PrepareLocalModule(core.ModulePreparationArgs{
				Fpath:                     file,
				ParsingCompilationContext: compilationCtx,
				ParentContext:             ctx,
				StdlibCtx:                 stdlibCtx,
				ParentContextRequired:     true,
				Out:                       io.Discard,
				ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
			})
		})

	})

	t.Run("recoverable parsing error not supported by symbolic evaluation", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
			a = <
			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present because we can still perform static check
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

	t.Run("recoverable parsing error supported by symbolic evaluation", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, mod) {
			return
		}

		// the state should be present because we can still perform static check
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

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("invalid CLI arguments: missing positional argument", func(t *testing.T) {

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
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
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
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
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
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
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			Args:                      core.NewModuleArgs(map[string]core.Value{}),
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
		defer compilationCtx.CancelGracefully()

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

		res, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath: file,
			Args: core.NewModuleArgs(map[string]core.Value{
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
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath: file,
			Args: core.NewModuleArgs(map[string]core.Value{
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
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath: file,
			Args: core.NewModuleArgs(map[string]core.Value{
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
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath: file,
			Args: core.NewModuleArgs(map[string]core.Value{
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
		defer compilationCtx.CancelGracefully()

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
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath: file,
			Args: core.NewModuleArgs(map[string]core.Value{
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
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				parameters: {}
			}
		
		`), 0o600)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath: file,
			Args: core.NewModuleArgs(map[string]core.Value{
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

func TestPrepareLocalModuleWithDatabases(t *testing.T) {
	projectsDir := t.TempDir()

	registryCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	defer registryCtx.CancelGracefully()
	projectRegistry, err := project.OpenRegistry(projectsDir, registryCtx)

	if !assert.NoError(t, err) {
		return
	}

	t.Run("local database", func(t *testing.T) {
		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				databases: {
					local: {
						resource: ldb://main
						resolution-data: nil
					}
				}
			}
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(100_000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
			Filesystem:  fs,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,

			Project:         project,
			MemberAuthToken: string(memberId),
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

		//The database opening configuration should be available.

		fn, config, availabe := db.OpeningConfiguration()
		if !assert.True(t, availabe) {
			return
		}

		assert.NotNil(t, fn)
		assert.NotZero(t, config)
	})

	t.Run("if the current schema of a database does not match the expected schema, only an error should be returned", func(t *testing.T) {
		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			preinit {
				pattern expected-schema = %{
					user: {name: "foo"}
				}
			}
			manifest {
				databases: {
					local: {
						resource: ldb://main
						resolution-data: nil
						assert-schema: %expected-schema
					}
				}
			}
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(100_000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
			Filesystem:  fs,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
			Project:                 project,
			MemberAuthToken:         string(memberId),
		})

		if !assert.ErrorIs(t, err, core.ErrCurrentSchemaNotEqualToExpectedSchema) {
			return
		}

		// the module should not be present
		if !assert.Nil(t, mod) {
			return
		}

		// the state should not be present
		if !assert.Nil(t, state) {
			return
		}
	})

	t.Run("in data extraction mode if the current schema of a database does not match the expected schema, the state and an error should be returned", func(t *testing.T) {
		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			preinit {
				pattern expected-schema = %{
					user: {name: "foo"}
				}
			}
			manifest {
				databases: {
					local: {
						resource: ldb://main
						resolution-data: nil
						assert-schema: %expected-schema
					}
				}
			}
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(100_000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
			Filesystem:  fs,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,

			DataExtractionMode: true,
			Project:            project,
			MemberAuthToken:    string(memberId),
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

		schema := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
			{
				Name: "user",
				Pattern: core.NewInexactObjectPattern([]core.ObjectPatternEntry{
					{
						Name:    "name",
						Pattern: core.NewExactStringPattern("foo"),
					},
				}),
			},
		})

		assert.Equal(t, schema, state.Databases["local"].Prop(ctx, "schema"))
		assert.Equal(t, core.Host("ldb://main"), state.Databases["local"].Resource())
	})

	t.Run("local database + assert-schema matching the current schema", func(t *testing.T) {
		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			preinit {
				pattern expected-schema = %{}
			}
			manifest {
				databases: {
					local: {
						resource: ldb://main
						resolution-data: nil
						assert-schema: %expected-schema
					}
				}
			}
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(100_000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
			Filesystem:  fs,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
			Project:                 project,
			MemberAuthToken:         string(memberId),
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

	t.Run("local database + expected schema update: the entities should not be loaded yet", func(t *testing.T) {

		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				databases: {
					local: {
						resource: ldb://main
						resolution-data: nil
						expected-schema-update: true
					}
				}
			}

			dbs.local.update_schema(%{})
		`), 0o600)

		fs := fs_ns.NewMemFilesystem(100_000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: core.GetDefaultGlobalVarPermissions(),
			Filesystem:  fs,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
			Project:                 project,
			MemberAuthToken:         string(memberId),
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

		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		fls := fs_ns.NewMemFilesystem(10_000)

		util.WriteFile(fls, "/main.ix", []byte(`
			manifest {
				databases: {
					local: {
						resource: ldb://main
						resolution-data: nil
					}
				}
				permissions: {
					read: %/...
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
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			),
			Filesystem: fls,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		mainState, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     "/main.ix",
			ParsingCompilationContext: ctx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:     fls,
			FullAccessToDatabases: true,
			Project:               project,
			MemberAuthToken:       string(memberId),
		})

		if !assert.NoError(t, err) {
			return
		}

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
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

	t.Run("object storage database", func(t *testing.T) {
		if OS_DB_TEST_ACCESS_KEY == "" {
			t.SkipNow()
			return
		}

		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		s3Host := randS3Host()
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {}
				host-definitions: :{
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

		fs := fs_ns.NewMemFilesystem(100_000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			),
			Filesystem: fs,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			Project:                   project,
			MemberAuthToken:           string(memberId),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,
			AdditionalGlobalsTestOnly: map[string]core.Value{
				"access-key": core.String(OS_DB_TEST_ACCESS_KEY),
				"secret-key": utils.Must(core.SECRET_STRING_PATTERN.NewSecret(
					core.NewContextWithEmptyState(core.ContextConfig{}, nil),
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

		//Create a project.

		id, memberId, err := projectRegistry.CreateProject(registryCtx, project.CreateProjectParams{
			Name: "test-project",
		})

		if !assert.NoError(t, err) {
			return
		}

		project, err := projectRegistry.OpenProject(registryCtx, project.OpenProjectParams{Id: id})
		if !assert.NoError(t, err) {
			return
		}

		//

		s3Host := randS3Host()
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {}
				host-definitions: :{
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

		fs := fs_ns.NewMemFilesystem(100_000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			),
			Filesystem: fs,
		})
		core.NewGlobalState(ctx)
		defer ctx.CancelGracefully()

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			Project:                   project,
			MemberAuthToken:           string(memberId),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,

			AdditionalGlobalsTestOnly: map[string]core.Value{
				"access-key": core.String(OS_DB_TEST_ACCESS_KEY),
				"secret-key": utils.Must(core.SECRET_STRING_PATTERN.NewSecret(
					core.NewContextWithEmptyState(core.ContextConfig{}, nil),
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

	t.Run("object storage database with credentials provided by the project", func(t *testing.T) {
		if CLOUDFLARE_ACCOUNT_ID == "" {
			t.Skip()
			return
		}

		//create project
		var proj *project.Project
		var memberId string
		projectName := "test-mod-prep-creds-from-project"
		{
			tempCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer tempCtx.CancelGracefully()
			registry, err := project.OpenRegistry(t.TempDir(), tempCtx)
			if !assert.NoError(t, err) {
				return
			}

			id, membID, err := registry.CreateProject(tempCtx, project.CreateProjectParams{
				Name: projectName,
			})

			if !assert.NoError(t, err) {
				return
			}

			p, err := registry.OpenProject(tempCtx, project.OpenProjectParams{
				Id: id,
				DevSideConfig: project.DevSideProjectConfig{
					Cloudflare: &cloudflareprovider.DevSideConfig{
						AdditionalTokensApiToken: CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN,
						AccountID:                CLOUDFLARE_ACCOUNT_ID,
					},
				},
			})

			memberId = string(membID)

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
				host-definitions: :{
					`+string(s3Host)+` : {
						bucket: "test"
						provider: "cloudflare"
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

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     "/script.ix",
			ParsingCompilationContext: ctx,
			ParentContext:             ctx,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			PreinitFilesystem:       fs,
			ScriptContextFileSystem: fs,
			FullAccessToDatabases:   true,

			Project:         proj,
			MemberAuthToken: memberId,
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

}

func TestPrepareLocalModuleWithCache(t *testing.T) {

	t.Run("module only: manual cache entry creation", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
		
			a = 1
			b = c 		  	# static check error
		`), 0o600)

		//First preparation: we create the cache.

		ctx1 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx1)
		defer ctx1.CancelGracefully()
		defer ctx1.CancelGracefully()

		state1, module, _, err1 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx1,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err1) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, module) {
			return
		}

		cacheKey := state1.EffectivePreparationParameters.PreparationCacheKey
		cacheTime := time.Now()

		cache := core.NewPreparationCacheEntry(cacheKey, core.PreparationCacheEntryUpdate{
			Module: module,
			Time:   cacheTime,
		})

		//Second preparation but with the cache.

		ctx2 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx2)
		defer ctx2.CancelGracefully()
		defer ctx2.CancelGracefully()

		state2, module2, _, err2 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx2,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			CacheEntry:              cache,
			ScriptContextFileSystem: fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err2) {
			return
		}

		if !assert.Equal(t, err1.Error(), err2.Error()) {
			return
		}

		// the module should be present
		if !assert.Same(t, module, module2) {
			return
		}

		// the state should be present because we can still perform static check
		if !assert.NotNil(t, state2) {
			return
		}

		// the state should not the previous state
		if !assert.NotSame(t, state1, state2) {
			return
		}

		if !assert.True(t, state2.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state2.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state2.SymbolicData.IsEmpty())

		assert.Equal(t, cacheTime, cache.LastUpdateTime())
	})

	t.Run("module only: automatic cache update", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
		
			a = 1
			b = c 		  	# static check error
		`), 0o600)

		//First preparation

		fls := fs_ns.GetOsFilesystem()

		ctx1 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fls,
		})
		core.NewGlobalState(ctx1)
		defer ctx1.CancelGracefully()
		defer ctx1.CancelGracefully()

		cache := core.NewPreparationCache()

		state1, module, _, err1 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx1,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fls,

			Cache: cache,
		})

		if !assert.Error(t, err1) {
			return
		}

		// The module should be present
		if !assert.NotNil(t, module) {
			return
		}

		//The cache should be populated.

		entryKey := state1.EffectivePreparationParameters.PreparationCacheKey
		entry, ok := cache.Get(entryKey)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, module.Name(), entry.ModuleName())
		assert.Equal(t, entryKey, entry.Key())
		assert.True(t, entry.CheckValidity(fls))

		//Second preparation.

		ctx2 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx2)
		defer ctx2.CancelGracefully()
		defer ctx2.CancelGracefully()

		state2, module2, _, err2 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx2,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			Cache:                   cache,
			ScriptContextFileSystem: fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err2) {
			return
		}

		if !assert.Equal(t, err1.Error(), err2.Error()) {
			return
		}

		// the module should be present
		if !assert.Same(t, module, module2) {
			return
		}

		// the state should be present because we can still perform static check
		if !assert.NotNil(t, state2) {
			return
		}

		// the state should not the previous state
		if !assert.NotSame(t, state1, state2) {
			return
		}

		if !assert.True(t, state2.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state2.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state2.SymbolicData.IsEmpty())
	})

	t.Run("module and static check data", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
		
			a = 1
			b = c 		  	# static check error
		`), 0o600)

		//First preparation: we create the cache.

		ctx1 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx1)
		defer ctx1.CancelGracefully()
		defer ctx1.CancelGracefully()

		state1, module, _, err1 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx1,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err1) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, module) {
			return
		}

		cacheKey := state1.EffectivePreparationParameters.PreparationCacheKey

		cache := core.NewPreparationCacheEntry(cacheKey, core.PreparationCacheEntryUpdate{
			Module:          module,
			Time:            time.Now(),
			StaticCheckData: state1.StaticCheckData,
		})

		//Second preparation but with the cache.

		ctx2 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx2)
		defer ctx2.CancelGracefully()
		defer ctx2.CancelGracefully()

		state2, module2, _, err2 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx2,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			CacheEntry:              cache,
			ScriptContextFileSystem: fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err2) {
			return
		}

		if !assert.Equal(t, err1.Error(), err2.Error()) {
			return
		}

		// the module should be present
		if !assert.Same(t, module, module2) {
			return
		}

		// the state should be present because we can still perform static check
		if !assert.NotNil(t, state2) {
			return
		}

		// the state should not the previous state
		if !assert.NotSame(t, state1, state2) {
			return
		}

		if !assert.True(t, state2.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		// static check data should have been re-used
		if !assert.NotEmpty(t, state2.StaticCheckData.Errors()) {
			return
		}

		// symbolic check data should have been re-used
		assert.False(t, state2.SymbolicData.IsEmpty())
	})

	t.Run("module and static check data", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
		
			a = 1
			b = c 		  	# static check error
		`), 0o600)

		//First preparation: we create the cache.

		ctx1 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx1)
		defer ctx1.CancelGracefully()
		defer ctx1.CancelGracefully()

		state1, module, _, err1 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx1,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err1) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, module) {
			return
		}

		cacheKey := state1.EffectivePreparationParameters.PreparationCacheKey

		cache := core.NewPreparationCacheEntry(cacheKey, core.PreparationCacheEntryUpdate{
			Module:                module,
			Time:                  time.Now(),
			StaticCheckData:       state1.StaticCheckData,
			SymbolicData:          state1.SymbolicData.Data,
			FinalSymbolicCheckErr: state1.FinalSymbolicCheckError,
		})

		//Second preparation but with the cache.

		ctx2 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx2)
		defer ctx2.CancelGracefully()
		defer ctx2.CancelGracefully()

		state2, module2, _, err2 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx2,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			CacheEntry:              cache,
			ScriptContextFileSystem: fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err2) {
			return
		}

		if !assert.Equal(t, err1.Error(), err2.Error()) {
			return
		}

		// the module should be present
		if !assert.Same(t, module, module2) {
			return
		}

		// the state should be present because we can still perform static check
		if !assert.NotNil(t, state2) {
			return
		}

		// the state should not the previous state
		if !assert.NotSame(t, state1, state2) {
			return
		}

		if !assert.True(t, state2.Ctx.HasPermission(core.CreateFsReadPerm(core.PathPattern("/...")))) {
			return
		}

		// static check data should have been re-used
		if !assert.NotEmpty(t, state2.StaticCheckData.Errors()) {
			return
		}

		// symbolic check data should have been re-used
		assert.False(t, state2.SymbolicData.IsEmpty())
	})

	t.Run("cache has an unexpected path", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.ix")
		otherFile := filepath.Join(dir, "script2.ix")

		compilationCtx := createCompilationCtx(dir)
		defer compilationCtx.CancelGracefully()

		os.WriteFile(file, []byte(`
			manifest {
				permissions: {
					read: %/...
				}
			}
		
			a = 1
			b = c 		  	# static check error
		`), 0o600)

		ctx1 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx1)
		defer ctx1.CancelGracefully()
		defer ctx1.CancelGracefully()

		state1, module, _, err1 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     file,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx1,
			ParentContextRequired:     true,
			Out:                       io.Discard,
			ScriptContextFileSystem:   fs_ns.GetOsFilesystem(),
		})

		if !assert.Error(t, err1) {
			return
		}

		// the module should be present
		if !assert.NotNil(t, module) {
			return
		}

		//second parsing but with the cache

		ctx2 := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(ctx2)
		defer ctx2.CancelGracefully()
		defer ctx2.CancelGracefully()

		cacheKey := state1.EffectivePreparationParameters.PreparationCacheKey

		state2, module2, _, err2 := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     otherFile, //path is different
			ParsingCompilationContext: compilationCtx,
			ParentContext:             ctx2,
			ParentContextRequired:     true,
			Out:                       io.Discard,

			CacheEntry: core.NewPreparationCacheEntry(cacheKey, core.PreparationCacheEntryUpdate{
				Module: module,
				Time:   time.Now(),
			}),
		})

		if !assert.ErrorIs(t, err2, core.ErrNonMatchingCachedEntryModulePath) {
			return
		}

		// the module should not be present
		if !assert.Nil(t, module2) {
			return
		}

		// the state should not be present
		assert.Nil(t, state2)
	})
}

func TestPrepareDevModeIncludableChunkFile(t *testing.T) {

	t.Run("recoverable parsing error not supported by symbolic evaluation", func(t *testing.T) {
		fs := fs_ns.NewMemFilesystem(10000)

		util.WriteFile(fs, "/included.ix", []byte(`
			includable-file

			a = <
			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		
		`), 0o600)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, _, _, err := core.PrepareExtractionModeIncludableFile(core.IncludableFilePreparationArgs{
			Fpath:                          "/included.ix",
			ParsingContext:                 ctx,
			LogOut:                         io.Discard,
			Out:                            io.Discard,
			IncludedChunkContextFileSystem: fs,
		})

		if !assert.Error(t, err) {
			return
		}

		// the state should be present because we can still perform static check
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.NotEmpty(t, state.Module.Errors) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should not have been performed
		assert.True(t, state.SymbolicData.IsEmpty())
	})

	t.Run("recoverable parsing error supported by symbolic evaluation", func(t *testing.T) {
		fs := fs_ns.NewMemFilesystem(10000)

		util.WriteFile(fs, "/included.ix", []byte(`
			includable-file

			a = ;
			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		
		`), 0o600)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, _, _, err := core.PrepareExtractionModeIncludableFile(core.IncludableFilePreparationArgs{
			Fpath:                          "/included.ix",
			ParsingContext:                 ctx,
			LogOut:                         io.Discard,
			Out:                            io.Discard,
			IncludedChunkContextFileSystem: fs,
		})

		if !assert.Error(t, err) {
			return
		}

		// the state should be present because we can still perform static check
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.NotEmpty(t, state.Module.Errors) {
			return
		}

		// static check should have been performed
		if !assert.NotEmpty(t, state.StaticCheckData.Errors()) {
			return
		}

		// symbolic check should have been performed
		assert.False(t, state.SymbolicData.IsEmpty())
	})

	t.Run("static check error", func(t *testing.T) {
		fs := fs_ns.NewMemFilesystem(10000)

		util.WriteFile(fs, "/included.ix", []byte(`
			includable-file

			b = 1
			c = d 		  	# static check error
			(b + "string") 	# symbolic check error
		`), 0o600)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.CreateFsReadPerm(core.PathPattern("/..."))),
			Filesystem:  fs,
		}, nil)

		state, _, _, err := core.PrepareExtractionModeIncludableFile(core.IncludableFilePreparationArgs{
			Fpath:                          "/included.ix",
			ParsingContext:                 ctx,
			LogOut:                         io.Discard,
			Out:                            io.Discard,
			IncludedChunkContextFileSystem: fs,
		})

		if !assert.Error(t, err) {
			return
		}

		// the state should be present because we can still perform static check
		if !assert.NotNil(t, state) {
			return
		}

		if !assert.Empty(t, state.Module.Errors) {
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
