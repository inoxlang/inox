package symbolic

import (
	"errors"
	"fmt"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_EVENT       = utils.Must(NewEvent(ANY))
	EVENT_PROPNAMES = []string{"time", "value"}
)

// EventSource represents a symbolic EventSource.
type EventSource struct {
	UnassignablePropsMixin
	_ int
}

func NewEventSource() *EventSource {
	return &EventSource{}
}

func (s *EventSource) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*EventSource)
	return ok
}

func (s *EventSource) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "close":
		return WrapGoMethod(s.Close), true
	}
	return nil, false
}

func (s *EventSource) Prop(name string) Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (s *EventSource) SetProp(name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (s *EventSource) WithExistingPropReplaced(name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (*EventSource) PropertyNames() []string {
	return []string{"close"}
}

func (s *EventSource) Close() {
}

func (s *EventSource) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("event-source")
}

func (s *EventSource) IteratorElementKey() Value {
	return ANY_INT
}

func (s *EventSource) IteratorElementValue() Value {
	return ANY_EVENT
}

func (s *EventSource) WidestOfType() Value {
	return &EventSource{}
}

type Event struct {
	UnassignablePropsMixin
	value Value
}

func NewEvent(value Value) (*Event, error) {
	if !IsAny(value) && value.IsMutable() {
		return nil, fmt.Errorf("failed to create event: value should be immutable: %T", value)
	}
	return &Event{value: value}, nil
}

func (r *Event) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Event)
	return ok
}

func (e *Event) PropertyNames() []string {
	return EVENT_PROPNAMES
}

func (e *Event) Prop(name string) Value {
	switch name {
	case "time":
		return &DateTime{}
	case "value":
		return e.value
	default:
		panic(FormatErrPropertyDoesNotExist(name, e))
	}
}

func (r *Event) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("event")
}

func (r *Event) WidestOfType() Value {
	return &Event{value: ANY}
}
