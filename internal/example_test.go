package internal

import (
	"bytes"
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

	_ "embed"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_TEST_TIMEOUT_DURATION = 25 * time.Second
	CHROME_EXAMPLE_FOLDER         = "chrome/"
	PROJECT_EXAMPLES_FOLDER       = "projects/"
	LOCALDB_EXAMPLES_FOLDER       = "local_database/"
	EXAMPLES_DIR                  = "./examples"
)

var (
	TIMING_OUT_EXAMPLES        = []string{"events.ix"}
	CANCELLED_TOP_CTX_EXAMPLES = []string{"execution-time.ix", "rollback-on-cancellation.ix"}
	SKIPPED_EXAMPLES           = []string{"get-resource.ix", "websocket.ix", "shared-patterns.ix", "add.ix", "models.ix", "fs-events.ix"}

	RUN_BROWSER_AUTOMATION_EXAMPLES = os.Getenv("RUN_BROWSER_AUTOMATION_EXAMPLES") == "true"
)

type filesystemWithRootWD struct {
	afs.Filesystem
}

func (fls filesystemWithRootWD) Absolute(path string) (string, error) {
	if strings.Contains(path, "/") {
		_ = 1
	}
	if len(path) > 1 && path[0] == '.' && path[1] == '/' {
		return path[1:], nil
	}
	if path[0] != '/' && !strings.HasPrefix(path, "../") {
		return "/" + path, nil
	}
	return path, nil
}

// TestExamples tests the scripts located in ./examples/ .
func TestExamples(t *testing.T) {
	if !chrome_ns.IsBrowserAutomationAllowed() {
		defer chrome_ns.DisallowBrowserAutomation()
	}

	chrome_ns.AllowBrowserAutomation()
	inProjectRoot := false

	dir, _ := os.Getwd()

	for _, entry := range utils.Must(os.ReadDir(dir)) {
		if entry.Name() == "examples" {
			inProjectRoot = true
			break
		}
	}
	exampleDir := "./examples"

	if !inProjectRoot {
		exampleDir = "." + EXAMPLES_DIR
	}

	//copy the examples folder inside a memory filesystem

	memFS := fs_ns.NewMemFilesystem(10_000_000)
	osFs := fs_ns.GetOsFilesystem()

	var exampleFilePaths []string
	const exampleDirInMemFS = "/examples/"

	core.WalkDir(osFs, core.Path(exampleDir)+"/", func(path core.Path, d fs.DirEntry, err error) error {
		if strings.HasPrefix(path.UnderlyingString(), "./../") {
			path = path[2:]
		}

		newPath := filepath.Clean(strings.ReplaceAll(string(path), exampleDir, exampleDirInMemFS))

		if d.IsDir() {
			return memFS.MkdirAll(newPath, d.Type())
		} else {
			content, err := util.ReadFile(osFs, path.UnderlyingString())
			if err != nil {
				return err
			}

			if strings.HasSuffix(newPath, inoxconsts.INOXLANG_FILE_EXTENSION) {
				exampleFilePaths = append(exampleFilePaths, newPath)
				content = bytes.ReplaceAll(content, []byte(EXAMPLES_DIR), []byte(exampleDirInMemFS))
			}

			return util.WriteFile(memFS, newPath, content, d.Type())
		}
	})

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
		testExamples(testExamplesArgs{
			t:                t,
			memFS:            memFS,
			exampleFilePaths: exampleFilePaths,
		})
	})

	if !testing.Short() {

		t.Run("bytecode", func(t *testing.T) {
			testExamples(testExamplesArgs{
				t:                t,
				memFS:            memFS,
				exampleFilePaths: exampleFilePaths,
				useBytecode:      true,
				optimizeBytecode: true,
			})
		})
	}

	t.Run("optimized bytecode", func(t *testing.T) {
		testExamples(testExamplesArgs{
			t:                t,
			memFS:            memFS,
			exampleFilePaths: exampleFilePaths,
			useBytecode:      true,
			optimizeBytecode: false,
		})
	})
}

type testExamplesArgs struct {
	t                             *testing.T
	memFS                         *fs_ns.MemFilesystem
	exampleFilePaths              []string
	useBytecode, optimizeBytecode bool
}

func testExamples(args testExamplesArgs) {
	t := args.t
	memFS := args.memFS

	fsSnapshot, err := memFS.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
		InclusionFilters: []core.PathPattern{"/..."},
		GetContent: func(ChecksumSHA256 [32]byte) core.AddressableContent {
			return nil
		},
	})

	if !assert.NoError(args.t, err) {
		return
	}

	for _, fpath := range args.exampleFilePaths {
		fpath := fpath
		testName := strings.ReplaceAll(fpath, "/", "--")

		// check if the test should be executed

		if strings.Contains(fpath, PROJECT_EXAMPLES_FOLDER) && !strings.HasSuffix(fpath, "/main.ix") {
			continue
		}

		if utils.SliceContains(SKIPPED_EXAMPLES, filepath.Base(fpath)) {
			continue
		}

		content := utils.Must(util.ReadFile(memFS, fpath))

		t.Run(testName, func(t *testing.T) {
			//parallelize all tests that don't start an HTTP server
			if !bytes.Contains(content, []byte("http.Server")) {
				t.Parallel()
			}

			fls := filesystemWithRootWD{
				Filesystem: utils.Must(fsSnapshot.NewAdaptedFilesystem(1_000_000)),
			}

			testExample(t, exampleTestConfig{
				fpath:            fpath,
				useBytecode:      args.useBytecode,
				optimizeBytecode: args.optimizeBytecode,
				testTimeout:      DEFAULT_TEST_TIMEOUT_DURATION,
				fls:              fls,
			})
		})
	}

	testing.CoverMode()
}

type exampleTestConfig struct {
	fpath                         string
	useBytecode, optimizeBytecode bool
	testTimeout                   time.Duration
	fls                           afs.Filesystem
}

func testExample(t *testing.T, config exampleTestConfig) {

	fpath := config.fpath
	useBytecode := config.useBytecode
	optimizeBytecode := config.optimizeBytecode
	testTimeout := config.testTimeout
	filename := filepath.Base(fpath)

	if strings.Contains(fpath, CHROME_EXAMPLE_FOLDER) && !RUN_BROWSER_AUTOMATION_EXAMPLES {
		t.Skip()
		return
	}

	var scriptContextFileSystem afs.Filesystem
	var proj *project.Project

	if strings.Contains(fpath, PROJECT_EXAMPLES_FOLDER) {
		//create the project's filesystem

		fileDirNoTrailingSlash := filepath.Dir(fpath)
		fileDir := core.Path(fileDirNoTrailingSlash) + "/"
		scriptContextFileSystem = fs_ns.NewMemFilesystem(1_000_000)

		core.WalkDir(config.fls, fileDir, func(path core.Path, d fs.DirEntry, err error) error {
			pathInSnapshot := strings.TrimPrefix(string(path), fileDirNoTrailingSlash)

			if d.IsDir() {
				return scriptContextFileSystem.MkdirAll(pathInSnapshot, d.Type())
			} else {
				content, err := util.ReadFile(config.fls, path.UnderlyingString())
				if err != nil {
					return err
				}
				return util.WriteFile(scriptContextFileSystem, pathInSnapshot, content, d.Type())
			}
		})

		proj = project.NewDummyProject("project", scriptContextFileSystem.(core.SnapshotableFilesystem))
	} else {
		scriptContextFileSystem = config.fls
	}

	done := make(chan int)

	//execute the example in a goroutine.
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
		defer parsingCompilationContext.CancelGracefully()

		actualFpath := fpath
		if proj != nil {
			actualFpath = "/main.ix"
		}

		_, _, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
			Fpath:                     actualFpath,
			PassedArgs:                core.NewEmptyModuleArgs(),
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

	//wait for the example to finish.
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
