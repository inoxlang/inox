package internal

import (
	"errors"
	"fmt"

	"github.com/inox-project/inox/internal/utils"
)

var (
	ANY_EVENT = utils.Must(NewEvent(ANY))
)

// EventSource represents a symbolic EventSource.
type EventSource struct {
	UnassignablePropsMixin
	_ int
}

func NewEventSource() *EventSource {
	return &EventSource{}
}

func (s *EventSource) Test(v SymbolicValue) bool {
	_, ok := v.(*EventSource)
	return ok
}

func (s *EventSource) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "close":
		return &GoFunction{fn: s.Close}, true
	}
	return &GoFunction{}, false
}

func (s *EventSource) Prop(name string) SymbolicValue {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (s *EventSource) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (s *EventSource) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(s))
}

func (*EventSource) PropertyNames() []string {
	return []string{"close"}
}

func (s *EventSource) Close() {
}

func (s *EventSource) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *EventSource) IsWidenable() bool {
	return false
}

func (s *EventSource) String() string {
	return "event-source"
}

func (s *EventSource) IteratorElementKey() SymbolicValue {
	return &Int{}
}

func (s *EventSource) IteratorElementValue() SymbolicValue {
	return ANY_EVENT
}

func (s *EventSource) WidestOfType() SymbolicValue {
	return &EventSource{}
}

type Event struct {
	UnassignablePropsMixin
	value SymbolicValue
}

func NewEvent(value SymbolicValue) (*Event, error) {
	if !isAny(value) && value.IsMutable() {
		return nil, fmt.Errorf("failed to create event: value should be immutable: %T", value)
	}
	return &Event{value: value}, nil
}

func (r *Event) Test(v SymbolicValue) bool {
	_, ok := v.(*Event)
	return ok
}

func (e *Event) PropertyNames() []string {
	return []string{"time", "value"}
}

func (e *Event) Prop(name string) SymbolicValue {
	switch name {
	case "time":
		return &Date{}
	case "value":
		return e.value
	default:
		panic(FormatErrPropertyDoesNotExist(name, e))
	}
}

func (r *Event) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *Event) IsWidenable() bool {
	return false
}

func (r *Event) String() string {
	return "event"
}

func (r *Event) WidestOfType() SymbolicValue {
	return &Event{value: ANY}
}
