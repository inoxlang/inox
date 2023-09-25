package fs_ns

import (
	"errors"
	"fmt"
)

var (
	ErrCannotOpenDir    = errors.New("cannot open directory")
	ErrClosedFilesystem = errors.New("closed filesystem")
)

func fmtDirContainFiles(path string) string {
	return fmt.Sprintf("dir: %s contains files", path)
}
