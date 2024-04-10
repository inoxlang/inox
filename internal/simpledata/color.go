package simpledata

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/muesli/termenv"
)

const (
	LAST_RESERVED_COLOR_ID = 10_000 // not definitive

	rgb24 colorEncodingId = iota + 1
	ansiColor
	ansiColor256
)

// A Color represents a color in a given encoding.
type Color struct {
	data       [6]byte
	encodingId colorEncodingId
}

type colorEncodingId uint16

func (id colorEncodingId) IsReserved() bool {
	return id <= LAST_RESERVED_COLOR_ID
}

func ColorFromAnsiColor(c termenv.ANSIColor) Color {
	return Color{
		data:       [6]byte{byte(c)},
		encodingId: ansiColor,
	}
}

func ColorFromAnsi256Color(c termenv.ANSI256Color) Color {
	return Color{
		data:       [6]byte{byte(c)},
		encodingId: ansiColor256,
	}
}

func ColorFromRGB24(r, g, b byte) Color {
	return Color{
		data:       [6]byte{r, g, b},
		encodingId: rgb24,
	}
}

func ColorFromTermenvColor(c termenv.Color, defaultColor ...Color) Color {
	switch color := c.(type) {
	case termenv.ANSIColor:
		return ColorFromAnsiColor(color)
	case termenv.ANSI256Color:
		return ColorFromAnsi256Color(color)
	case termenv.RGBColor:
		r, g, b := termenv.ConvertToRGB(color).RGB255()
		return ColorFromRGB24(r, g, b)
	case termenv.NoColor:
		if len(defaultColor) == 0 {
			panic(errors.New("missing default color"))
		}
		return defaultColor[0]
	default:
		panic(errors.New("unreachable"))
	}
}

func (c Color) Equal(other Color) bool {
	return c.data == other.data && c.encodingId == other.encodingId
}

func (c Color) ToTermColor() termenv.Color {
	switch c.encodingId {
	case ansiColor:
		return termenv.ANSIColor(c.data[0])
	case ansiColor256:
		return termenv.ANSI256Color(c.data[0])
	case rgb24:
		b := [7]byte{'#'}
		strconv.AppendUint(b[1:1:3], uint64(c.data[0]), 16)
		strconv.AppendUint(b[3:3:5], uint64(c.data[1]), 16)
		strconv.AppendUint(b[5:5:7], uint64(c.data[2]), 16)
		return termenv.RGBColor(string(b[:]))
	default:
		panic(fmt.Errorf("invalid or unsupported color id: %d", c.encodingId))
	}
}

func (c Color) GetAnsiEscapeSequence(background bool) []byte {
	return prettyprint.GetFullColorSequence(c.ToTermColor(), background)
}

func (c Color) IsDarkBackgroundColor() bool {
	termColor := c.ToTermColor()
	rgb := termenv.ConvertToRGB(termColor)

	_, _, l := rgb.Hsl()
	return l < 0.5
}
