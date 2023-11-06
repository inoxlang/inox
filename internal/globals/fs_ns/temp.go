package fs_ns

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
)

const (
	PROCESS_TEMP_DIR_PREFIX = "inoxlang"
)

var (
	processTempDir     string
	processTempDirLock sync.Mutex
)

// CreateTempdir creates a directory with permissions o700 in the process's temporary directory in the OS's filesystem.
// The process's temporary directory is created if necessary.
func CreateDirInProcessTempDir(namePrefix string) core.Path {
	fls := GetOsFilesystem()

	func() {
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
	}()

	path := core.Path(fmt.Sprintf("%s/%s-%d-%s", processTempDir, namePrefix, rand.Int(), time.Now().Format(time.RFC3339Nano)))

	if err := fls.MkdirAll(string(path), 0o700); err != nil {
		panic(err)
	}

	return path
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
