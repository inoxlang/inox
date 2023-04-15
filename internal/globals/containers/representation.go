package internal

import (
	"io"

	core "github.com/inoxlang/inox/internal/core"
)

func (*Set) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Set) WriteRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Set) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Set) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Stack) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Stack) WriteRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Stack) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Stack) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Queue) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Queue) WriteRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Queue) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Queue) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Thread) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Thread) WriteRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Thread) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Thread) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Map) HasRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Map) WriteRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*Map) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (*Map) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}
