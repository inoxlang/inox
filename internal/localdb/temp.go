package localdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/rs/zerolog"

	processutils "github.com/shirou/gopsutil/v3/process"
)

const (
	TEMP_DIRS_PARENT     = "/tmp"
	TEMP_DIR_NAME_PREFIX = "temp_local_inox_db"
)

func randTempDirPathInOsFs() string {
	return fmt.Sprintf("%s/%s-%d-%s", TEMP_DIRS_PARENT, TEMP_DIR_NAME_PREFIX, os.Getpid(), time.Now().Format(time.RFC3339Nano))
}

func DeleteTempDatabaseDirsOfDeadProcesses(logger zerolog.Logger, maxDuration time.Duration) {
	fls := fs_ns.GetOsFilesystem()

	entries, err := fls.ReadDir(TEMP_DIRS_PARENT)
	if err != nil {
		panic(err)
	}

	deadline := time.Now().Add(maxDuration)

	for _, entry := range entries {

		path := filepath.Join(TEMP_DIRS_PARENT, entry.Name())

		if !strings.HasPrefix(entry.Name(), TEMP_DIR_NAME_PREFIX) {
			continue
		}

		if time.Now().After(deadline) {
			return
		}

		parts := strings.Split(entry.Name(), "-")
		if len(parts) < 3 {
			continue
		}

		pidString := parts[1]

		pid, err := strconv.ParseInt(pidString, 10, 32)
		if err != nil {
			continue
		}

		if exists, err := processutils.PidExists(int32(pid)); err == nil && !exists {
			logger.Info().Msgf("remove temp local db dir %s", path)
			err := fls.RemoveAll(path)
			if err != nil {
				logger.Err(err).Msgf("failed to remove temporary local db directory (%s): %s", path, err.Error())
			}
		}
	}
}
