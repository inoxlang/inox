package internal

import (
	"io"

	core "github.com/inox-project/inox/internal/core"
)

func (*HttpServer) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (serv *HttpServer) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*HttpRequest) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (req *HttpRequest) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*HttpResponseWriter) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (resp *HttpResponseWriter) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}

func (*HttpResponse) HasJSONRepresentation(encountered map[uintptr]int, config *core.ReprConfig) bool {
	return false
}

func (resp *HttpResponse) WriteJSONRepresentation(ctx *core.Context, w io.Writer, encountered map[uintptr]int, config *core.ReprConfig) error {
	return core.ErrNoRepresentation
}
