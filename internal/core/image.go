package core

type Image interface {
	FilesystemSnapshot() FilesystemSnapshot
}
