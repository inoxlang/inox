package internal

import core "github.com/inox-project/inox/internal/core"

func (f *File) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherFile, ok := other.(*File)
	return ok && f == otherFile
}

func (evs *FilesystemEventSource) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherEvs, ok := other.(*FilesystemEventSource)
	return ok && evs == otherEvs
}
