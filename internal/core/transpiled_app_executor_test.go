package core

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils/processutils"
)

const (
	TEMP_DIR_PREFIX = "inox-mod-executor-"
)

// TestingAppExecutor is a TranspiledAppExecutor tailored for testing in the core package, it is not used by
// Inox's testing engine.
type TestingAppExecutor struct {
	lock       sync.Mutex
	app        *TranspiledApp
	instances  map[*TestingAppInstance]struct{}
	tempDir    string
	binaryPath string
	ctx        context.Context
}

// NewTestingGoExecutor creates a TestingGoExecutor that compiles the Go code of a transpiled mode
// inside a temporary directory, and creates a process.
func NewTestingGoExecutor(t *testing.T, ctx *Context, app *TranspiledApp) (*TestingAppExecutor, error) {

	//Create a temporary directory.

	tempDir := t.TempDir()
	mainPkgPath := filepath.Join(tempDir, inoxconsts.RELATIVE_MAIN_INOX_MOD_PKG_PATH)
	binaryPath := filepath.Join(tempDir, "mod")

	//Write the soure code inside the temporary directory.

	err := app.WriteTo(ctx, tempDir)

	if err != nil {
		return nil, err
	}

	//Compile

	compilationCtx, cancelCompilation := context.WithTimeout(ctx, 5*time.Second)
	defer cancelCompilation()

	cmd := exec.CommandContext(compilationCtx, "go", "build", mainPkgPath, "-o="+binaryPath)

	err = cmd.Run()

	if err != nil {
		return nil, err
	}

	return &TestingAppExecutor{
		app:        app,
		instances:  make(map[*TestingAppInstance]struct{}),
		tempDir:    tempDir,
		binaryPath: binaryPath,
		ctx:        ctx,
	}, nil
}

func (e *TestingAppExecutor) App() *TranspiledApp {
	return e.app
}

func (e *TestingAppExecutor) CreateInstance(t *testing.T, ctx *Context) (TranspiledAppInstance, error) {
	//Create a process

	t.Cleanup(func() {
		ctx.CancelGracefully()
	})

	appInstance := &TestingAppInstance{
		startEvents: make(chan int32, 1),
		exitEvents:  make(chan struct{}, 1),
		ctx:         ctx,
	}

	earlyErrChan := make(chan error, 1)

	go func() {
		earlyErrChan <- processutils.AutoRestart(processutils.AutoRestartArgs{
			GoCtx: ctx,
			MakeCommand: func(goCtx context.Context) (*exec.Cmd, error) {
				return exec.CommandContext(goCtx, e.binaryPath), nil
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
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	go func() {
		//Loop that updates $appInstance.currentPID
		for {
			select {
			case pid := <-appInstance.startEvents:
				appInstance.currentPID.Store(pid)
			case <-appInstance.exitEvents:
				appInstance.currentPID.Store(-1)
			case <-ctx.Done():
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
	ctx         *Context
}

func (i *TestingAppInstance) IsRunning() bool {
	return i.currentPID.Load() != 0
}

func (i *TestingAppInstance) Context() *Context {
	return i.ctx
}
