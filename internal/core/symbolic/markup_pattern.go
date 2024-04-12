package symbolic

import pprint "github.com/inoxlang/inox/internal/prettyprint"

var (
	ANY_MARKUP_PATTERN = &MarkupPattern{}
)

type MarkupPattern struct {
	NotCallablePatternMixin
	SerializableMixin
}

func (p *MarkupPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*MarkupPattern)
	return ok
}

func (p *MarkupPattern) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	return false
}

func (p *MarkupPattern) IteratorElementKey() Value {
	return ANY_INT
}

func (p *MarkupPattern) IteratorElementValue() Value {
	return ANY
}

func (p *MarkupPattern) SymbolicValue() Value {
	return ANY_MARKUP_NODE
}

func (p *MarkupPattern) HasUnderlyingPattern() bool {
	return true
}

func (n *MarkupPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("markup-pattern")
}

func (p *MarkupPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (n *MarkupPattern) WidestOfType() Value {
	return ANY_MARKUP_PATTERN
}
