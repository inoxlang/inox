package fs_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	fs_symbolic "github.com/inoxlang/inox/internal/globals/fs_ns/symbolic"
)

func (f *File) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &fs_symbolic.File{}, nil
}

func (evs *FilesystemEventSource) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewEventSource(), nil
}

func (evs *FilesystemIL) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return fs_symbolic.ANY_FILESYSTEM, nil
}
