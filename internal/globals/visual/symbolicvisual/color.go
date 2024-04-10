package symbolicvisual

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_COLOR            = &Color{}
	COLOR_PROPERTY_NAMES = []string{}
)

// A Color represents a symbolic Color.
type Color struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (c *Color) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *Color:
		return true
	default:
		return false
	}
}

func (c Color) IsMutable() bool {
	return false
}

func (c *Color) WidestOfType() symbolic.Value {
	return ANY_COLOR
}

func (c *Color) Prop(name string) symbolic.Value {
	switch name {
	}
	return nil
}

func (*Color) PropertyNames() []string {
	return COLOR_PROPERTY_NAMES
}

func (c *Color) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("color")
}
