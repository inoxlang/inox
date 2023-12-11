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
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/learn"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	_ "github.com/inoxlang/inox/internal/globals"
)

const DEFAULT_TUTORIAL_TIMEMOUT_DURATION = 10 * time.Second

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

		//parallelize all tutorials that don't start an HTTP server
		if !bytes.Contains([]byte(tut.Program), []byte("http.Server")) {
			t.Parallel()
		}

		stdlibCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan int)
		timeout := DEFAULT_TUTORIAL_TIMEMOUT_DURATION

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
			fls := fs_ns.NewMemFilesystem(10_000)
			util.WriteFile(fls, fpath, []byte(tut.Program), 0500)
			for filePath, content := range tut.OtherFiles {
				util.WriteFile(fls, filePath, []byte(content), 0500)
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

			_, _, _, _, err := mod.RunLocalScript(mod.RunScriptArgs{
				Fpath:                     fpath,
				PassedArgs:                core.NewEmptyStruct(),
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
			})

			if assert.NoError(t, err) {
				output := strings.Split(outputBuff.String(), "\n")
				output = utils.FilterSlice(output, func(e string) bool {
					return e != ""
				})
				if tut.ExpectedLogOutput != nil {
					assert.Equal(t, tut.ExpectedOutput, output)
				}
			}
		}()

		select {
		case <-done:
		case <-time.After(timeout):
			assert.Fail(t, "timeout")
			cancel()
		}
	})
}
