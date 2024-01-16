package core

import (
	"fmt"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

// An Event represents a generic event, Event implements Value.
type Event struct {
	time              DateTime
	affectedResources []ResourceName //can be empty
	value             Value          //data
	sourceValue       any            //Golang value
}

func NewEvent(srcValue any, value Value, time DateTime, affectedResources ...ResourceName) *Event {
	if value.IsMutable() {
		panic(fmt.Errorf("failed to create event: value should be immutable: %T", value))
	}
	return &Event{
		value:             value,
		sourceValue:       srcValue,
		time:              time,
		affectedResources: affectedResources,
	}
}

func (e *Event) Value() Value {
	return e.value
}

// SourceValue() returns the Golang value that was used to create the event, it can be nil.
func (e *Event) SourceValue() any {
	return e.sourceValue
}

// Age returns the ellapsed time since the event happened.
func (e *Event) Age() time.Duration {
	return time.Since(time.Time(e.time))
}

// AgeWithCurrentTime returns the ellapsed time since the event happened but using $now as the current time.
func (e *Event) AgeWithCurrentTime(now time.Time) time.Duration {
	return now.Sub(time.Time(e.time))
}

func (e *Event) PropertyNames(ctx *Context) []string {
	return symbolic.EVENT_PROPNAMES
}

func (e *Event) Prop(ctx *Context, name string) Value {
	switch name {
	case "time":
		return e.time
	case "value":
		return e.value
	default:
		panic(FormatErrPropertyDoesNotExist(name, e))
	}
}

func (e *Event) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}
