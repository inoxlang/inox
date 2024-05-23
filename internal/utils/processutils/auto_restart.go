package processutils

import (
	"context"
	"fmt"
	"os/exec"
	"runtime/debug"
	"sync/atomic"
	"time"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/rs/zerolog"
)

const (
	DEFAULT_MAX_TRY_COUNT                   = 3
	DEFAULT_POST_START_BURST_PAUSE_DURATION = 5 * time.Minute

	NEW_PROCESS_PID_LOG_FIELD_NAME = "newProcessPID"
)

type AutoRestartArgs struct {
	GoCtx       context.Context
	MakeCommand func(goCtx context.Context) (*exec.Cmd, error)

	Logger            zerolog.Logger
	ProcessNameInLogs string

	//defaults to DEFAULT_MAX_TRY_COUNT
	MaxTryCount int

	//optional, an item is written to this channel each time a created process exits.
	ExitEventChan chan struct{}

	//optional, each time a process is started its PID is written to the channel.
	StartEventChan chan int32

	//duration of the pause following a burst of failed starts, defaults to DEFAULT_POST_START_BURST_PAUSE_DURATION.
	PostStartBurstPauseDuration time.Duration

	//optional, the value is set to true when the autorestart is paused
	PostStartBurstPause *atomic.Bool
}

// AutoRestart starts a restart loop in the calling goroutine. The Args.MakeCommand function is called to obtain the command before each execution.
// The loop stops when the args.GoCtx is done or if Args.MakeCommand returns an error.
func AutoRestart(args AutoRestartArgs) error {
	if args.MaxTryCount <= 0 {
		args.MaxTryCount = DEFAULT_MAX_TRY_COUNT
	}

	if args.PostStartBurstPauseDuration <= 0 {
		args.PostStartBurstPauseDuration = DEFAULT_POST_START_BURST_PAUSE_DURATION
	}

	if args.PostStartBurstPause == nil {
		args.PostStartBurstPause = &atomic.Bool{}
	}

	if args.StartEventChan != nil {
		defer close(args.StartEventChan)
	}

	if args.ExitEventChan != nil {
		defer close(args.ExitEventChan)
	}

	for {
		err := func() error {
			defer func() {
				e := recover()
				if e != nil {
					err := utils.ConvertPanicValueToError(e)
					err = fmt.Errorf("%w: %s", err, debug.Stack())
					args.Logger.Err(err).Send()
				}
			}()
			return autoRestart(args)
		}()

		if err != nil {
			return err
		}
	}
}

func autoRestart(args AutoRestartArgs) (ctxError error) {
	logger := args.Logger
	maxTryCount := args.MaxTryCount
	processName := args.ProcessNameInLogs

	tryCount := 0
	var lastLaunchTime time.Time

	for !utils.IsContextDone(args.GoCtx) {

		if tryCount >= maxTryCount {

			logger.Error().Msgf(processName+" process exited unexpectedly %d or more times in a short timeframe; wait %s\n", maxTryCount, args.PostStartBurstPauseDuration)
			args.PostStartBurstPause.Store(true)

			select {
			case <-time.After(args.PostStartBurstPauseDuration):
			case <-args.GoCtx.Done():
				return args.GoCtx.Err()
			}

			args.PostStartBurstPause.Store(false)
			tryCount = 0
		}

		tryCount++
		lastLaunchTime = time.Now()

		cmd, err := args.MakeCommand(args.GoCtx)

		if err != nil {
			err := fmt.Errorf("failed to created command for %s: %w", processName, err)
			logger.Err(err).Send()
			return err
		}

		logger.Info().Msg("create a new process (" + processName + ")")

		err = cmd.Start()
		if err == nil {
			logger.Info().Int(NEW_PROCESS_PID_LOG_FIELD_NAME, cmd.Process.Pid).Send()
			if args.StartEventChan != nil {
				select {
				case args.StartEventChan <- int32(cmd.Process.Pid):
				default:
					//drop event
				}
			}
			err = cmd.Wait()
		}

		if err == nil {
			logger.Error().Msg(processName + " process exited with an unexpected status of 0")
		} else {
			logger.Error().Err(err).Msg(processName + " process exited")
		}

		if args.ExitEventChan != nil {
			select {
			case args.ExitEventChan <- struct{}{}:
			default:
				//drop event
			}
		}

		if time.Since(lastLaunchTime) < 10*time.Second {
			tryCount++
		} else {
			tryCount = 1
		}
	}

	//context is done

	return args.GoCtx.Err()
}
