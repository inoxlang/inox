package fs_ns

import core "github.com/inoxlang/inox/internal/core"

func (f *File) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherFile, ok := other.(*File)
	return ok && f == otherFile
}

func (evs *FilesystemEventSource) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherEvs, ok := other.(*FilesystemEventSource)
	return ok && evs == otherEvs
}

func (fls *FilesystemIL) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherFls, ok := other.(*FilesystemIL)
	return ok && fls == otherFls
}
