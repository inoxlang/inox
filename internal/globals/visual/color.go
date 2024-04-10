package visual

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/visual/symbolicvisual"
	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/simpledata"
	"github.com/muesli/termenv"
)

type Color struct {
	simpledata.Color
}

func ColorFromTermenvColor(color termenv.Color) *Color {
	lower := simpledata.ColorFromTermenvColor(color)
	return &Color{
		Color: lower,
	}
}

func (*Color) IsMutable() bool {
	return false
}

func (c *Color) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherColor, ok := other.(*Color)
	if !ok {
		return false
	}

	//TODO: return true if equivalent colors ?

	return c.Color.Equal(otherColor.Color)
}

func (c *Color) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	return core.ErrNotImplementedYet
}

func (c *Color) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	core.InspectPrint(w, c)
}

func (c Color) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolicvisual.ANY_COLOR, nil
}
