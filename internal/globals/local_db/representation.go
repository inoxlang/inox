package internal

import (
	"io"

	core "github.com/inoxlang/inox/internal/core"
)

func (*LocalDatabase) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*LocalDatabase) WriteRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*LocalDatabase) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*LocalDatabase) WriteJSONRepresentation(ctx *Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}
