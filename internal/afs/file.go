package afs

import (
	"errors"
	"os"
)

var (
	ErrNotStatCapable = errors.New("not stat capable")
)

type StatCapable interface {
	Stat() (os.FileInfo, error)
}

type SyncCapable interface {
	Sync() error
}
