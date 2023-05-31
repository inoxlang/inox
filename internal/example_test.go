package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	core "github.com/inoxlang/inox/internal/core"

	_ "github.com/inoxlang/inox/internal/globals"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"

	"github.com/inoxlang/inox/internal/permkind"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_TEST_TIMEMOUT_DURATION = 10 * time.Second
	CHROME_EXAMPLE_FOLDER          = "chrome/"
)

var (
	TIMING_OUT_EXAMPLES        = []string{"events.ix"}
	CANCELLED_TOP_CTX_EXAMPLES = []string{"execution-time.ix", "rollback-on-cancellation.ix"}
	FALSE_ASSERTION_EXAMPLES   = []string{"simple-testsuite.ix"}
	SKIPPED_EXAMPLES           = []string{"get-resource.ix", "websocket.ix", "shared-patterns.ix", "add.ix"}

	RUN_BROWSER_AUTOMATION_EXAMPLES = os.Getenv("RUN_BROWSER_AUTOMATION_EXAMPLES") == "true"
)

// TestExamples tests the scripts located in ./examples/ .
func TestExamples(t *testing.T) {

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
		if info.Mode().IsRegular() && strings.HasSuffix(path, ".ix") {
			exampleFilePaths = append(exampleFilePaths, path)
		}
		return nil
	})

	for _, fpath := range exampleFilePaths {
		testName := strings.ReplaceAll(fpath, "/", "--")

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

	if strings.Contains(fpath, CHROME_EXAMPLE_FOLDER) && !RUN_BROWSER_AUTOMATION_EXAMPLES ||
		utils.SliceContains(SKIPPED_EXAMPLES, filename) {
		t.Skip()
	}

	if runInTempDir {
		tempDir := t.TempDir()

		//we copy the examples in test directory and we set the WD to this directory

		err := fs_ns.Copy(core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern("/...")},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: core.PathPattern("/...")},
			},
			Limitations: []core.Limitation{
				{
					Name:  fs_ns.FS_READ_LIMIT_NAME,
					Kind:  core.ByteRateLimitation,
					Value: 100_000_000,
				},
				{
					Name:  fs_ns.FS_WRITE_LIMIT_NAME,
					Kind:  core.ByteRateLimitation,
					Value: 100_000_000,
				},
			},
			Filesystem: fs_ns.GetOsFilesystem(),
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
			Filesystem:  fs_ns.GetOsFilesystem(),
		})
		core.NewGlobalState(parsingCompilationContext)

		_, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     fpath,
			UseBytecode:               useBytecode,
			OptimizeBytecode:          optimizeBytecode,
			ParsingCompilationContext: parsingCompilationContext,
			Out:                       io.Discard,
			AllowMissingEnvVars:       true,
			IgnoreHighRiskScore:       true,
			//Out:              os.Stdout, // &utils.TestWriter{T: t},
		})

		if utils.SliceContains(CANCELLED_TOP_CTX_EXAMPLES, filename) {
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
		} else if utils.SliceContains(FALSE_ASSERTION_EXAMPLES, filename) {
			assert.Error(t, err)

			e := errors.Unwrap(err)
			for {
				unwrapped := errors.Unwrap(e)
				if unwrapped == nil {
					break
				}
				e = unwrapped
			}

			assert.IsType(t, &core.AssertionError{}, e)
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
