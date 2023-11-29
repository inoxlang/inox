package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_COLOR = &Color{}
)

// A Color represents a symbolic Color.
type Color struct {
	UnassignablePropsMixin
	_ int
}

func (c *Color) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *Color:
		return true
	default:
		return false
	}
}

func (c *Color) WidestOfType() Value {
	return ANY_COLOR
}

func (c *Color) Prop(name string) Value {
	switch name {
	}
	return nil
}

func (*Color) PropertyNames() []string {
	return []string{}
}

func (c *Color) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("color")
	return
}
