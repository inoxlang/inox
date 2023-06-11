package fs_ns

import (
	"errors"
	"io/fs"
	"time"

	"github.com/djherbis/times"
	"github.com/inoxlang/inox/internal/core"
)

var (
	ErrTimeInfoNotAvailable = errors.New("time information of file is not available")
)

// GetCreationAndModifTime returns the creation time & content modification time.
func GetCreationAndModifTime(i fs.FileInfo) (time.Time, time.Time, error) {
	if i.Sys() == nil {
		switch info := i.(type) {
		case core.ExtendedFileInfo:
			creationTime, ok := info.CreationTime()

			if !ok {
				return time.Time{}, time.Time{}, ErrTimeInfoNotAvailable
			}

			return creationTime, info.ModTime(), nil
		default:
			return time.Time{}, time.Time{}, ErrTimeInfoNotAvailable
		}
	}
	spec := times.Get(i)
	return spec.BirthTime(), spec.ModTime(), nil
}
