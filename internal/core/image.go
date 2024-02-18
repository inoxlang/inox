package core

import "io"

// An image represents a project image.
type Image interface {
	ProjectID() ProjectID
	FilesystemSnapshot() FilesystemSnapshot
	Zip(ctx *Context, w io.Writer) error
}
