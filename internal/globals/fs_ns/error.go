package fs_ns

import (
	"errors"
	"fmt"
)

var (
	ErrCannotOpenDir = errors.New("cannot open directory")
)

func fmtDirContainFiles(path string) string {
	return fmt.Sprintf("dir: %s contains files", path)
}
