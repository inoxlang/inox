package core

// An image represents a project image.
type Image interface {
	FilesystemSnapshot() FilesystemSnapshot
}
