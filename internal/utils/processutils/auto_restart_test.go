package processutils

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
)

func TestAutoRestart(t *testing.T) {

	t.Run("cancelling the context should stop the process if the command was created using exec.CommandContext", func(t *testing.T) {

		startEvents := make(chan int32, 10)

		goCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go AutoRestart(AutoRestartArgs{
			GoCtx: goCtx,
			MakeCommand: func(goCtx context.Context) (*exec.Cmd, error) {
				return exec.CommandContext(goCtx, "sleep", "10s"), nil
			},
			Logger:            zerolog.Nop(),
			ProcessNameInLogs: "sleep",
			MaxTryCount:       3,
			StartEventChan:    startEvents,
		})

		var pid int32
		select {
		case <-time.After(time.Second):
			t.Fail()
			return
		case pid = <-startEvents:
		}

		exists, _ := process.PidExists(pid)
		if !assert.True(t, exists) {
			return
		}

		cancel()
		time.Sleep(100 * time.Millisecond)

		exists, _ = process.PidExists(pid)
		assert.False(t, exists)
	})

	t.Run("the loop should not start if the command factory returns an error", func(t *testing.T) {

		startEvents := make(chan int32, 10)

		goCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		commandCreationError := errors.New("failed to create command")

		err := AutoRestart(AutoRestartArgs{
			GoCtx: goCtx,
			MakeCommand: func(goCtx context.Context) (*exec.Cmd, error) {
				return nil, commandCreationError
			},
			Logger:            zerolog.Nop(),
			ProcessNameInLogs: "sleep",
			MaxTryCount:       3,
			StartEventChan:    startEvents,
		})

		assert.ErrorIs(t, err, commandCreationError)
	})

	t.Run("the loop should not start if the context is cancelled", func(t *testing.T) {

		startEvents := make(chan int32, 10)

		goCtx, cancel := context.WithCancel(context.Background())
		cancel()

		startTime := time.Now()
		err := AutoRestart(AutoRestartArgs{
			GoCtx: goCtx,
			MakeCommand: func(goCtx context.Context) (*exec.Cmd, error) {
				return exec.Command("sleep", "10s"), nil
			},
			Logger:            zerolog.Nop(),
			ProcessNameInLogs: "sleep",
			MaxTryCount:       3,
			StartEventChan:    startEvents,
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, time.Since(startTime), 10*time.Millisecond)
	})
}
