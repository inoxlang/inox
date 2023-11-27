package processutils

import (
	"context"
	"fmt"
	"os/exec"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	DEFAULT_MAX_TRY_COUNT                   = 3
	DEFAULT_POST_START_BURST_PAUSE_DURATION = 5 * time.Minute

	NEW_PROCESS_PID_LOG_FIELD_NAME = "newProcessPID"
)

type AutoRestartArgs struct {
	GoCtx       context.Context
	MakeCommand func() *exec.Cmd

	Logger            zerolog.Logger
	ProcessNameInLogs string

	//defaults to DEFAULT_MAX_TRY_COUNT
	MaxTryCount int

	//an item is written to this channel each time a created process exits.
	ExitEventChan chan struct{}

	//duration of the pause following a burst of failed starts, defaults to DEFAULT_POST_START_BURST_PAUSE_DURATION.
	PostStartBurstPauseDuration time.Duration

	//optional
	PostStartBurstPause *atomic.Bool
}

func AutoRestart(args AutoRestartArgs) {
	if args.MaxTryCount <= 0 {
		args.MaxTryCount = DEFAULT_MAX_TRY_COUNT
	}

	if args.PostStartBurstPauseDuration <= 0 {
		args.PostStartBurstPauseDuration = DEFAULT_POST_START_BURST_PAUSE_DURATION
	}

	if args.PostStartBurstPause == nil {
		args.PostStartBurstPause = &atomic.Bool{}
	}

	for {
		func() {
			defer func() {
				e := recover()
				if e != nil {
					err := utils.ConvertPanicValueToError(e)
					err = fmt.Errorf("%w: %s", err, debug.Stack())
					args.Logger.Err(err).Send()
				}
			}()
			autoRestart(args)
		}()
	}
}

func autoRestart(args AutoRestartArgs) {
	logger := args.Logger
	maxTryCount := args.MaxTryCount
	processName := args.ProcessNameInLogs

	tryCount := 0
	var lastLaunchTime time.Time

	for !utils.IsContextDone(args.GoCtx) {

		if tryCount >= maxTryCount {

			logger.Error().Msgf(processName+" process exited unexpectedly %d or more times in a short timeframe; wait %s\n", maxTryCount, args.PostStartBurstPauseDuration)
			args.PostStartBurstPause.Store(true)

			time.Sleep(args.PostStartBurstPauseDuration)
			args.PostStartBurstPause.Store(false)
			tryCount = 0
		}

		tryCount++
		lastLaunchTime = time.Now()

		cmd := args.MakeCommand()

		logger.Info().Msg("create a new process (" + processName + ")")

		err := cmd.Start()
		if err == nil {
			logger.Info().Int(NEW_PROCESS_PID_LOG_FIELD_NAME, cmd.Process.Pid).Send()
			err = cmd.Wait()
		}

		if err == nil {
			logger.Error().Msg(processName + "proxy process returned with au unexpected status of 0")
		} else {
			logger.Error().Err(err).Msg(processName + "process returned")
		}

		args.ExitEventChan <- struct{}{}

		if time.Since(lastLaunchTime) < 10*time.Second {
			tryCount++
		} else {
			tryCount = 1
		}
	}
}
