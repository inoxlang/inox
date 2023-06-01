package fs_ns

import (
	"errors"
	"io/fs"
	"time"

	"github.com/djherbis/times"
)

var (
	ErrTimeInfoNotAvailable = errors.New("time information of file is not available")
)

// GetCreationAndModifTime returns the creation time & content modification time.
func GetCreationAndModifTime(i fs.FileInfo) (time.Time, time.Time, error) {
	if i.Sys() == nil {
		switch info := i.(type) {
		case *memFileInfo:
			return info.creationTime, info.modificationTime, nil
		default:
			return time.Time{}, time.Time{}, ErrTimeInfoNotAvailable
		}
	}
	spec := times.Get(i)
	return spec.BirthTime(), spec.ModTime(), nil
}
