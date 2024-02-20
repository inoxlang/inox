package internal

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/learn"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	_ "github.com/inoxlang/inox/internal/globals"
)

const DEFAULT_TUTORIAL_TIMEOUT_DURATION = 25 * time.Second

func TestTutorials(t *testing.T) {
	const fpath = "/main.tut.ix"

	t.Run("BytecodeEval", func(t *testing.T) {
		for _, series := range learn.TUTORIAL_SERIES {
			for _, tut := range series.Tutorials {
				testTutorial(t, series, tut, fpath, true)
			}
		}
	})

	t.Run("TreeWalkEval", func(t *testing.T) {
		for _, series := range learn.TUTORIAL_SERIES {
			for _, tut := range series.Tutorials {
				testTutorial(t, series, tut, fpath, false)
			}
		}
	})
}

func testTutorial(t *testing.T, series learn.TutorialSeries, tut learn.Tutorial, fpath string, useBytecode bool) {
	t.Run(series.Name+"--"+tut.Name, func(t *testing.T) {

		hasHttpServer := bytes.Contains([]byte(tut.Program), []byte("http.Server"))

		var stdlibCtx context.Context
		var cancel context.CancelFunc

		//parallelize all tutorials that don't start an HTTP server
		if hasHttpServer {
			//cancel after 3 seconds.
			stdlibCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			//Start the dev server.

			devServerCtx := core.NewContextWithEmptyState(core.ContextConfig{
				Filesystem: fs_ns.NewMemFilesystem(1_000_000),
			}, nil)

			err := http_ns.StartDevServer(devServerCtx, http_ns.DevServerConfig{
				DevServersDir: "/",
				Port:          inoxconsts.DEV_PORT_0,
			})

			if !assert.NoError(t, err) {
				return
			}

			defer devServerCtx.CancelGracefully()
		} else {
			testconfig.AllowParallelization(t)
			stdlibCtx, cancel = context.WithCancel(context.Background())
			defer cancel()
		}

		done := make(chan int)
		timeout := DEFAULT_TUTORIAL_TIMEOUT_DURATION

		go func() {
			defer func() {
				if v := recover(); v != nil {
					panic(fmt.Errorf("(example %s) %s", fpath, v))
				}
			}()
			defer func() {
				done <- 0
				close(done)
			}()

			//create filesystem
			fls := fs_ns.NewMemFilesystem(1_000_000)
			util.WriteFile(fls, fpath, []byte(tut.Program), 0500)
			for filePath, content := range tut.OtherFiles {
				err := util.WriteFile(fls, filePath, []byte(content), 0500)
				if !assert.NoError(t, err) {
					return
				}
			}

			//
			parsingCompilationContext := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
					core.HttpPermission{Kind_: permkind.Read, AnyEntity: true},
				},
				Filesystem: fls,
			})
			core.NewGlobalState(parsingCompilationContext)
			defer parsingCompilationContext.CancelGracefully()

			outputBuff := bytes.NewBuffer(nil)
			logOutputBuff := bytes.NewBuffer(nil)

			_, _, _, _, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
				Fpath:                     fpath,
				PassedArgs:                core.NewEmptyModuleArgs(),
				ParsingCompilationContext: parsingCompilationContext,
				StdlibCtx:                 stdlibCtx,
				PreinitFilesystem:         fls,
				ScriptContextFileSystem:   fls,

				UseBytecode:         useBytecode,
				OptimizeBytecode:    useBytecode,
				Out:                 outputBuff,
				LogOut:              logOutputBuff,
				AllowMissingEnvVars: true,
				IgnoreHighRiskScore: true,

				Project: project.NewDummyProject("proj", fls),
				OnPrepared: func(state *core.GlobalState) error {
					if hasHttpServer {
						//Add a dev session key entry in order to allow the creation of a virtual HTTP server.
						state.Ctx.PutUserData(http_ns.CTX_DATA_KEY_FOR_DEV_SESSION_KEY, core.String(http_ns.RandomDevSessionKey()))
					}

					return nil
				},
			})

			if hasHttpServer {
				if !assert.ErrorIs(t, err, context.DeadlineExceeded) {
					return
				}
			} else if !assert.NoError(t, err) {
				return
			}

			output := strings.Split(outputBuff.String(), "\n")
			output = utils.FilterMapSlice(output, func(e string) (string, bool) {
				if e == "" {
					return "", false
				}
				return utils.StripANSISequences(e), true
			})

			if output == nil {
				output = []string{}
			}

			if tut.ExpectedOutput != nil {
				assert.Equal(t, tut.ExpectedOutput, output)
			}

			//TODO: make the writer for log output thread safe

			// logOutput := strings.Split(logOutputBuff.String(), "\n")
			// logOutput = utils.FilterSlice(logOutput, func(e string) bool {
			// 	return e != ""
			// })
			// if tut.ExpectedLogOutput != nil {
			// 	assert.Equal(t, tut.ExpectedLogOutput, logOutput)
			// }
		}()

		select {
		case <-done:
		case <-time.After(timeout):
			assert.Fail(t, "timeout")
			cancel()
		}
	})
}
