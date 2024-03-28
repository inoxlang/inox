package core_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox"
	"github.com/inoxlang/inox/internal/core"
	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils/processutils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

const (
	TEMP_DIR_PREFIX = "inox-mod-executor-"
)

func init() {
	core.InoxCodebaseFS = inox.CodebaseFS
}

func TestTestingAppExecutor(t *testing.T) {

	t.SkipNow()

	createExecCtx := func() *core.Context {

		timeoutCtx, _ := context.WithTimeout(context.Background(), 5*time.Second)

		return core.NewContextWithEmptyState(core.ContextConfig{
			Filesystem:          fs_ns.GetOsFilesystem(),
			ParentStdLibContext: timeoutCtx,
			Permissions: []core.Permission{
				core.FilesystemPermission{
					Kind_:  permkind.Read,
					Entity: core.ROOT_PREFIX_PATH_PATTERN,
				},
				core.FilesystemPermission{
					Kind_:  permkind.Write,
					Entity: core.ROOT_PREFIX_PATH_PATTERN,
				},
			},
		}, nil)
	}

	t.Run("empty main module", func(t *testing.T) {
		ctx, preparedModules := writeAndPrepareInoxFiles(t, map[string]string{
			"/main.ix": `manifest {}`,
		})

		defer ctx.CancelGracefully()

		app, err := core.TranspileApp(core.AppTranspilationParams{
			ParentContext:    ctx,
			MainModule:       core.Path("/main.ix"),
			ThreadSafeLogger: zerolog.Nop(),
			Config:           core.AppTranspilationConfig{},
			PreparedModules:  preparedModules,
		})

		if !assert.NoError(t, err) {
			return
		}

		execCtx := createExecCtx()
		defer execCtx.CancelGracefully()

		srcDir := t.TempDir()

		err = app.WriteToFilesystem(execCtx, srcDir)
		if !assert.NoError(t, err) {
			return
		}

		//Create the executor and an instance of the application.

		executor, err := NewTestingAppExecutor(t, execCtx, app)
		if !assert.NoError(t, err) {
			return
		}

		appCtx := execCtx.BoundChild()
		output := bytes.NewBuffer(nil)

		transpiledInstance, err := executor.CreateInstance(t, appCtx, output)

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, appCtx, transpiledInstance.Context())
	})
}

// TestingAppExecutor is a TranspiledAppExecutor tailored for testing in the core package, it is not used by
// Inox's testing engine.
type TestingAppExecutor struct {
	lock       sync.Mutex
	app        *core.TranspiledApp
	instances  map[*TestingAppInstance]struct{}
	tempDir    string
	binaryPath string
	ctx        context.Context
}

// NewTestingAppExecutor creates compiles the Go code of a transpiled mode inside a temporary directory,
// and returns a TestingGoExecutor able to create instances of the transpiled+compiled application.
func NewTestingAppExecutor(t *testing.T, ctx *core.Context, app *core.TranspiledApp) (*TestingAppExecutor, error) {

	//Create a temporary directory.

	srcDir := t.TempDir()

	//Write the soure code inside the temporary directory.

	err := app.WriteToFilesystem(ctx, srcDir)

	if err != nil {
		return nil, err
	}

	//Prepare the compilation command

	compilationCtx, cancelCompilation := context.WithTimeout(ctx, 30*time.Second)
	defer cancelCompilation()

	output := bytes.NewBuffer(nil)

	cmd := exec.CommandContext(compilationCtx,
		"go", "build",
		"-C="+srcDir, //Change working dir to $srcDir.
		"-o=./"+inoxconsts.TRANSPILED_APP_BINARY_NAME,
		"./"+inoxconsts.RELATIVE_MAIN_INOX_MOD_PKG_PATH,
	)

	cmd.Stderr = output
	cmd.Stdout = io.Discard

	//Compile

	err = cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("%w, ouput: %s", err, output.String())
	}

	binaryPath := filepath.Join(srcDir, inoxconsts.TRANSPILED_APP_BINARY_NAME)

	return &TestingAppExecutor{
		app:        app,
		instances:  make(map[*TestingAppInstance]struct{}),
		tempDir:    srcDir,
		binaryPath: binaryPath,
		ctx:        ctx,
	}, nil
}

func (e *TestingAppExecutor) App() *core.TranspiledApp {
	return e.app
}

func (e *TestingAppExecutor) CreateInstance(t *testing.T, appCtx *core.Context, output io.Writer) (core.TranspiledAppInstance, error) {
	//Create a process

	t.Cleanup(func() {
		appCtx.CancelGracefully()
	})

	appInstance := &TestingAppInstance{
		startEvents: make(chan int32, 1),
		exitEvents:  make(chan struct{}, 1),
		ctx:         appCtx,
	}

	earlyErrChan := make(chan error, 1)

	go func() {
		earlyErrChan <- processutils.AutoRestart(processutils.AutoRestartArgs{
			GoCtx: appCtx,
			MakeCommand: func(goCtx context.Context) (*exec.Cmd, error) {
				cmd := exec.CommandContext(goCtx, e.binaryPath)

				cmd.Stdout = output
				cmd.Stderr = output

				return cmd, nil
			},
			MaxTryCount:    2,
			StartEventChan: appInstance.startEvents,
			ExitEventChan:  appInstance.exitEvents,
		})
	}()

	select {
	case err := <-earlyErrChan:
		if err != nil {
			return nil, err
		}
	case <-appCtx.Done():
		return nil, appCtx.Err()
	}

	e.lock.Lock()
	e.instances[appInstance] = struct{}{}
	e.lock.Unlock()

	go func() {
		defer func() {
			//Remove instance.
			e.lock.Lock()
			delete(e.instances, appInstance)
			e.lock.Unlock()
		}()

		//Loop that updates $appInstance.currentPID
		for {
			select {
			case pid := <-appInstance.startEvents:
				appInstance.currentPID.Store(pid)
			case <-appInstance.exitEvents:
				appInstance.currentPID.Store(-1)
			case <-appCtx.Done():
				return
			}
		}
	}()

	return appInstance, nil
}

type TestingAppInstance struct {
	currentPID  atomic.Int32 //0 if not running
	startEvents chan int32
	exitEvents  chan struct{}
	ctx         *core.Context
}

func (i *TestingAppInstance) IsRunning() bool {
	return i.currentPID.Load() != 0
}

func (i *TestingAppInstance) Context() *core.Context {
	return i.ctx
}
