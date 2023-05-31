package fs_ns

import (
	"io"

	core "github.com/inoxlang/inox/internal/core"
)

func (*File) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*File) WriteRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*File) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*File) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

//

func (*FilesystemEventSource) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*FilesystemEventSource) WriteRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*FilesystemEventSource) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*FilesystemEventSource) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}
