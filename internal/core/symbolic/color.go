package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_COLOR = &Color{}
)

// A Color represents a symbolic Color.
type Color struct {
	UnassignablePropsMixin
	_ int
}

func (c *Color) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *Color:
		return true
	default:
		return false
	}
}

func (c *Color) WidestOfType() SymbolicValue {
	return ANY_COLOR
}

func (c *Color) Prop(name string) SymbolicValue {
	switch name {
	}
	return nil
}

func (*Color) PropertyNames() []string {
	return []string{}
}

func (c *Color) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%color")))
	return
}
