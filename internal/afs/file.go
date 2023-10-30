package afs

import (
	"errors"
	"os"

	"github.com/go-git/go-billy/v5"
)

var (
	ErrNotStatCapable = errors.New("not stat capable")
)

type StatCapable interface {
	billy.File
	Stat() (os.FileInfo, error)
}

type SyncCapable interface {
	billy.File
	Sync() error
}
