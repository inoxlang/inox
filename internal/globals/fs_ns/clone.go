package fs_ns

import core "github.com/inoxlang/inox/internal/core"

func (f *File) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (evs *FilesystemEventSource) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}
