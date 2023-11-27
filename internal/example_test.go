package internal

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	_ "github.com/inoxlang/inox/internal/globals"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/project"

	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"

	"github.com/inoxlang/inox/internal/permkind"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_TEST_TIMEMOUT_DURATION = 10 * time.Second
	CHROME_EXAMPLE_FOLDER          = "chrome/"
	PROJECT_EXAMPLES_FOLDER        = "projects/"
)

var (
	TIMING_OUT_EXAMPLES        = []string{"events.ix"}
	CANCELLED_TOP_CTX_EXAMPLES = []string{"execution-time.ix", "rollback-on-cancellation.ix"}
	SKIPPED_EXAMPLES           = []string{"get-resource.ix", "websocket.ix", "shared-patterns.ix", "add.ix", "models.ix"}

	RUN_BROWSER_AUTOMATION_EXAMPLES = os.Getenv("RUN_BROWSER_AUTOMATION_EXAMPLES") == "true"
)

// TestExamples tests the scripts located in ./examples/ .
func TestExamples(t *testing.T) {
	if !chrome_ns.IsBrowserAutomationAllowed() {
		defer chrome_ns.DisallowBrowserAutomation()
	}

	chrome_ns.AllowBrowserAutomation()

	//we set the working directory to the project's root
	dir, _ := os.Getwd()

	defer os.Chdir(dir)
	os.Chdir("..")
	core.SetInitialWorkingDir(os.Getwd)

	//uncomment the following lines to test a given script
	// testExample(t, exampleTestConfig{
	// fpath:            "./ide/ide.ix",
	// useBytecode:      true,
	// optimizeBytecode: true,
	// testTimeout:      1000 * time.Second,
	// runInTempDir:     false,
	// })
	//return

	t.Run("tree-walk", func(t *testing.T) {
		testExamples(t, false, false)
	})

	if !testing.Short() {
		t.Run("bytecode", func(t *testing.T) {
			testExamples(t, true, true)
		})
	}

	t.Run("optimized bytecode", func(t *testing.T) {
		testExamples(t, true, true)
	})
}

func testExamples(t *testing.T, useBytecode, optimizeBytecode bool) {

	const exampleFolder = "./examples"
	var exampleFilePaths []string

	filepath.Walk(exampleFolder, func(path string, info fs.FileInfo, err error) error {
		if info.Mode().IsRegular() && strings.HasSuffix(path, inoxconsts.INOXLANG_FILE_EXTENSION) {
			exampleFilePaths = append(exampleFilePaths, path)
		}
		return nil
	})

	for _, fpath := range exampleFilePaths {
		testName := strings.ReplaceAll(fpath, "/", "--")

		if strings.Contains(fpath, PROJECT_EXAMPLES_FOLDER) && !strings.HasSuffix(fpath, "/main.ix") {
			continue
		}

		if utils.SliceContains(SKIPPED_EXAMPLES, filepath.Base(fpath)) {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			testExample(t, exampleTestConfig{
				fpath:            fpath,
				useBytecode:      useBytecode,
				optimizeBytecode: optimizeBytecode,
				testTimeout:      DEFAULT_TEST_TIMEMOUT_DURATION,
				runInTempDir:     true,
			})

		})
	}

}

type exampleTestConfig struct {
	fpath                         string
	useBytecode, optimizeBytecode bool
	testTimeout                   time.Duration
	runInTempDir                  bool
}

func testExample(t *testing.T, config exampleTestConfig) {

	fpath := config.fpath
	useBytecode := config.useBytecode
	optimizeBytecode := config.optimizeBytecode
	testTimeout := config.testTimeout
	runInTempDir := config.runInTempDir

	filename := filepath.Base(fpath)

	if strings.Contains(fpath, CHROME_EXAMPLE_FOLDER) && !RUN_BROWSER_AUTOMATION_EXAMPLES {
		t.Skip()
		return
	}

	osFilesystem := fs_ns.GetOsFilesystem()
	var scriptContextFileSystem afs.Filesystem
	var proj *project.Project

	if strings.Contains(fpath, PROJECT_EXAMPLES_FOLDER) {
		//create the project's filesystem

		fileDirNoTrailingSlash := filepath.Dir(fpath)
		fileDir := core.Path(fileDirNoTrailingSlash) + "/"
		scriptContextFileSystem = fs_ns.NewMemFilesystem(1_000_000)

		core.WalkDir(osFilesystem, fileDir, func(path core.Path, d fs.DirEntry, err error) error {
			pathInSnapshot := strings.TrimPrefix(string(path), fileDirNoTrailingSlash)

			if d.IsDir() {
				return scriptContextFileSystem.MkdirAll(pathInSnapshot, d.Type())
			} else {
				content, err := util.ReadFile(osFilesystem, path.UnderlyingString())
				if err != nil {
					return err
				}
				return util.WriteFile(scriptContextFileSystem, pathInSnapshot, content, d.Type())
			}
		})

		proj = project.NewDummyProject("project", scriptContextFileSystem.(core.SnapshotableFilesystem))
	} else if runInTempDir {
		scriptContextFileSystem = osFilesystem
		tempDir := t.TempDir()

		//we copy the examples in test directory and we set the WD to this directory

		err := fs_ns.Copy(core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: core.PathPattern("/...")},
			},
			Limits:     core.GetDefaultScriptLimits(),
			Filesystem: scriptContextFileSystem,
		}), core.Path("./examples/"), core.Path(filepath.Join(tempDir, "./examples/")+"/"))

		if !assert.NoError(t, err) {
			return
		}

		dir, _ := os.Getwd()

		defer os.Chdir(dir)
		os.Chdir(tempDir)
		core.SetInitialWorkingDir(os.Getwd)
	}

	done := make(chan int)

	go func() {
		defer func() {
			if v := recover(); v != nil {
				panic(fmt.Errorf("(example %s) %s", fpath, v))
			}
		}()

		parsingCompilationContext := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{core.CreateFsReadPerm(core.PathPattern("/..."))},
			Filesystem:  scriptContextFileSystem,
		})

		core.NewGlobalState(parsingCompilationContext)

		actualFpath := fpath
		if proj != nil {
			actualFpath = "/main.ix"
		}

		_, _, _, _, err := mod.RunLocalScript(mod.RunScriptArgs{
			Fpath:                     actualFpath,
			PassedArgs:                core.NewEmptyStruct(),
			UseBytecode:               useBytecode,
			OptimizeBytecode:          optimizeBytecode,
			ParsingCompilationContext: parsingCompilationContext,

			ScriptContextFileSystem: scriptContextFileSystem,
			Project:                 proj,

			Out:                 io.Discard,
			AllowMissingEnvVars: true,
			IgnoreHighRiskScore: true,
			//Out:              os.Stdout, // &utils.TestWriter{T: t},

			EnableTesting: strings.HasSuffix(fpath, inoxconsts.INOXLANG_SPEC_FILE_SUFFIX),
			TestFilters: core.TestFilters{
				PositiveTestFilters: []core.TestFilter{{NameRegex: ".*"}},
			},
		})

		if utils.SliceContains(CANCELLED_TOP_CTX_EXAMPLES, filename) {
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
		} else {
			assert.NoError(t, err)
		}
		done <- 0
		close(done)
	}()

	select {
	case <-done:
		if utils.SliceContains(TIMING_OUT_EXAMPLES, filename) {
			assert.FailNow(t, "example should have timed out")
		}
	case <-time.After(testTimeout):
		if !utils.SliceContains(TIMING_OUT_EXAMPLES, filename) {
			assert.FailNow(t, "timeout")
		}
	}
}
