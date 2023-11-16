package core

import (
	"errors"
	"fmt"
	"sync"
)

var (
	eventSourceFactories     = map[Scheme]EventSourceFactory{}
	eventSourceFactoriesLock sync.RWMutex

	ErrNonUniqueEventSourceFactoryRegistration = errors.New("non unique event source factory registration")
	ErrHandlerAlreadyAdded                     = errors.New("handler already added to event source")
	ErrFileWatchingNotSupported                = errors.New("file watching is not supported")
)

// RegisterEventSourceFactory registers an event source factory for a given scheme.
func RegisterEventSourceFactory(scheme Scheme, factory EventSourceFactory) {
	eventSourceFactoriesLock.Lock()
	defer eventSourceFactoriesLock.Unlock()

	_, ok := eventSourceFactories[scheme]
	if ok {
		panic(ErrNonUniqueEventSourceFactoryRegistration)
	}
	eventSourceFactories[scheme] = factory
}

func GetEventSourceFactory(scheme Scheme) (EventSourceFactory, bool) {
	eventSourceFactoriesLock.RLock()
	defer eventSourceFactoriesLock.RUnlock()

	factory, ok := eventSourceFactories[scheme]
	return factory, ok
}

// An Event represents a generic event, Event implements Value.
type Event struct {
	time              DateTime
	affectedResources []ResourceName //can be empty
	value             Value
}

func NewEvent(value Value, time DateTime, affectedResources ...ResourceName) *Event {
	if value.IsMutable() {
		panic(fmt.Errorf("failed to create event: value should be immutable: %T", value))
	}
	return &Event{
		value:             value,
		time:              time,
		affectedResources: affectedResources,
	}
}

func (e *Event) Value() Value {
	return e.value
}

func (e *Event) PropertyNames(ctx *Context) []string {
	return []string{"time", "value"}
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

type EventHandler func(event *Event)

type EventSourceFactory func(ctx *Context, resourceNameOrPattern Value) (EventSource, error)

// TODO: rework
type EventSource interface {
	GoValue
	Iterable
	OnEvent(handler EventHandler) error
	Close()
	IsClosed() bool
}

type EventSourceHandlerManagement struct {
	eventHandlers []EventHandler
	lock          sync.RWMutex
}

func (evs *EventSourceHandlerManagement) OnEvent(handler EventHandler) error {
	evs.lock.Lock()
	defer evs.lock.Unlock()

	for _, e := range evs.eventHandlers {
		//NOTE: function pointers are not necessarily unique in Golang
		if SamePointer(e, handler) {
			return ErrHandlerAlreadyAdded
		}
	}
	evs.eventHandlers = append(evs.eventHandlers, func(event *Event) {
		defer func() {
			recover()
		}()
		handler(event)
	})
	return nil
}
 
// GetHandlers returns all event listeners (handlers), they are safe to call without recovering.
func (evs *EventSourceHandlerManagement) GetHandlers() []EventHandler {
	evs.lock.RLock()
	defer evs.lock.RUnlock()
	eventHandlers := make([]EventHandler, len(evs.eventHandlers))
	copy(eventHandlers, evs.eventHandlers)
	return eventHandlers
}

func NewEventSource(ctx *Context, resourceNameOrPattern Value) (EventSource, error) {

	switch v := resourceNameOrPattern.(type) {
	case Path, PathPattern:
		factory, ok := GetEventSourceFactory(Scheme("file"))
		if !ok {
			return nil, ErrFileWatchingNotSupported
		}

		return factory(ctx, resourceNameOrPattern)
	case URL:
		scheme := v.Scheme()

		factory, ok := GetEventSourceFactory(scheme)
		if !ok {
			return nil, fmt.Errorf("watching with scheme %s is not supported", scheme)
		}
		return factory(ctx, resourceNameOrPattern)
	default:
		return nil, fmt.Errorf("cannot create event source with %#v %T", resourceNameOrPattern, resourceNameOrPattern)
	}

}
