package fs_ns

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/rs/zerolog"
	processutils "github.com/shirou/gopsutil/v3/process"
)

const (
	PROCESS_TEMP_DIR_PREFIX = "inoxlangprocess"
)

var (
	processTempDir     string
	processTempDirLock sync.Mutex
)

// CreateTempdir creates a directory with permissions o700 in the process's temporary directory in the OS's filesystem.
// The process's temporary directory is created if necessary.
func CreateDirInProcessTempDir(namePrefix string) core.Path {
	fls := GetOsFilesystem()

	tempDir := GetCreateProcessTempDir()
	path := core.Path(fmt.Sprintf("%s/%s-%d-%s", tempDir, namePrefix, rand.Int(), time.Now().Format(time.RFC3339Nano)))

	if err := fls.MkdirAll(string(path), 0o700); err != nil {
		panic(err)
	}

	return path
}

func DeleteDirInProcessTempDir(path core.Path) error {
	pth := path.UnderlyingString()

	if path[0] != '/' {
		return nil
	}

	parts := strings.Split(pth[1:], "/")
	if len(parts) != 3 {
		return nil
	}

	if parts[0] != "tmp" || !strings.HasPrefix(parts[1], PROCESS_TEMP_DIR_PREFIX) {
		return nil
	}

	fls := GetOsFilesystem()
	return fls.RemoveAll(pth)
}

func GetCreateProcessTempDir() core.Path {
	fls := GetOsFilesystem()

	processTempDirLock.Lock()
	defer processTempDirLock.Unlock()

	//create the process's temporary directory if it does not already exist.
	if processTempDir == "" {
		dir := fmt.Sprintf("/tmp/%s-%d-%s", PROCESS_TEMP_DIR_PREFIX, os.Getpid(), time.Now().Format(time.RFC3339Nano))
		err := fls.MkdirAll(dir, 0700)
		if err != nil {
			panic(err)
		}
		processTempDir = dir
	}
	return core.Path(processTempDir)
}

func DeleteProcessTempDir() {
	fls := GetOsFilesystem()

	processTempDirLock.Lock()
	defer processTempDirLock.Unlock()

	if processTempDir != "" {
		fls.RemoveAll(processTempDir)
		processTempDir = ""
	}
}

func DeleteDeadProcessTempDirs(logger zerolog.Logger, maxDuration time.Duration) {
	fls := GetOsFilesystem()

	entries, err := fls.ReadDir("/tmp")
	if err != nil {
		panic(err)
	}

	deadline := time.Now().Add(maxDuration)

	for _, entry := range entries {

		path := filepath.Join("/tmp", entry.Name())

		if !strings.HasPrefix(entry.Name(), PROCESS_TEMP_DIR_PREFIX) {
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
			logger.Info().Msgf("remove temp dir %s", path)
			err := fls.RemoveAll(path)
			if err != nil {
				logger.Err(err).Msgf("failed to remove dead process's temporary directory (%s): %s", path, err.Error())
			}
		}
	}
}
