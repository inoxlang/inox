package afs

import "os"

type StatCapable interface {
	Stat() (os.FileInfo, error)
}

type SyncCapable interface {
	Sync() error
}
