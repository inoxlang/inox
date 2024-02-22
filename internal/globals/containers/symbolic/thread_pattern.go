package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_THREAD_PATTERN = NewSetPatternWithElementPatternAndUniqueness(symbolic.ANY_OBJECT_PATTERN, nil)
)

type MessageThreadPattern struct {
	symbolic.UnassignablePropsMixin
	elementPattern *symbolic.ObjectPattern

	symbolic.NotCallablePatternMixin
	symbolic.SerializableMixin
}

func NewMessageThreadPattern(elementPattern *symbolic.ObjectPattern) *MessageThreadPattern {
	return &MessageThreadPattern{
		elementPattern: elementPattern,
	}
}

func (p *MessageThreadPattern) MigrationInitialValue() (symbolic.Serializable, bool) {
	return symbolic.EMPTY_LIST, true
}

func (p *MessageThreadPattern) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPattern, ok := v.(*MessageThreadPattern)
	return ok && p.elementPattern.Test(otherPattern.elementPattern, state)
}

func (p *MessageThreadPattern) TestValue(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	thread, ok := v.(*MessageThread)
	if !ok {
		return false
	}
	return p.elementPattern.Test(thread.elementPattern, state)
}

func (p *MessageThreadPattern) IsConcretizable() bool {
	return p.elementPattern.IsConcretizable()
}

func (p *MessageThreadPattern) Concretize(ctx symbolic.ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(symbolic.ErrNotConcretizable)
	}

	concreteElementPattern := utils.Must(symbolic.Concretize(p.elementPattern, ctx))
	return externalData.CreateConcreteThreadPattern(ctx, concreteElementPattern)
}

func (p *MessageThreadPattern) HasUnderlyingPattern() bool {
	return true
}

func (p *MessageThreadPattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (p *MessageThreadPattern) SymbolicValue() symbolic.Value {
	return NewThread(p.elementPattern)
}

func (p *MessageThreadPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("message-thread-pattern(")
	p.elementPattern.SymbolicValue().PrettyPrint(w, config)
	w.WriteByte(')')
}

func (*MessageThreadPattern) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (p *MessageThreadPattern) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (*MessageThreadPattern) WidestOfType() symbolic.Value {
	return ANY_THREAD_PATTERN
}
