package processutils

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/process"
)

func KillHiearachy(pid int, logger zerolog.Logger) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		logger.Debug().Int("pid", pid).Err(err).Send()
		return
	}

	killHiearachy(p, logger)
}

func killHiearachy(proc *process.Process, logger zerolog.Logger) {
	defer proc.Kill()

	children, err := proc.Children()
	if err != nil {
		logger.Debug().Int32("pid", proc.Pid).Err(err).Msg("failed to get children of process")
		return
	}

	for _, child := range children {
		killHiearachy(child, logger)
	}

	time.Sleep(100 * time.Millisecond)

	//kill all remaining children

	children, err = proc.Children()
	if err == nil {
		for _, child := range children {
			killHiearachy(child, logger)
		}
	}
}
