package project

import "github.com/inoxlang/inox/internal/core"

var (
	_ = core.Image((*Image)(nil))
)

type Image struct {
	filesystem core.FilesystemSnapshot
}

func (img *Image) FilesystemSnapshot() core.FilesystemSnapshot {
	return img.filesystem
}
