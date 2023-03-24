package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	fs_symbolic "github.com/inox-project/inox/internal/globals/fs/symbolic"
)

func (f *File) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &fs_symbolic.File{}, nil
}

func (evs *FilesystemEventSource) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewEventSource(), nil
}
