package fs_ns

import (
	"errors"
	"fmt"
)

var (
	ErrCannotOpenDir                 = errors.New("cannot open directory")
	ErrClosedFilesystem              = errors.New("closed filesystem")
	ErrNoRemainingSpaceUsableByFS    = errors.New("no remaining space usable by filesystem")
	ErrNoRemainingSpaceToApplyChange = errors.New("no remaining space to apply change")
	ErrMaxUsableSpaceTooSmall        = errors.New("the given usable space value is too small")
)

func fmtDirContainFiles(path string) string {
	return fmt.Sprintf("dir: %s contains files", path)
}
