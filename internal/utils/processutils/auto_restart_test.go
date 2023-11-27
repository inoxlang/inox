package processutils

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
)

func TestAutoRestart(t *testing.T) {

	t.Run("cancelling the context should stop the process", func(t *testing.T) {

		startEvents := make(chan int32, 10)

		goCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go AutoRestart(AutoRestartArgs{
			GoCtx: goCtx,
			MakeCommand: func(goCtx context.Context) *exec.Cmd {
				return exec.CommandContext(goCtx, "sleep", "10s")
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
}
