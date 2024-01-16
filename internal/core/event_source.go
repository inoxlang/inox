package core

import (
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	HARD_MINIMUM_LAST_EVENT_AGE              = 25 * time.Millisecond
	MAX_MINIMUM_LAST_EVENT_AGE               = 10 * time.Second
	IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL = 25 * time.Millisecond
)

var (
	eventSourceFactories     = map[Scheme]EventSourceFactory{}
	eventSourceFactoriesLock sync.RWMutex

	idleEventSourceManagerSpawned           atomic.Bool
	eventSourcesWithEnabledIdleHandling     = map[*EventSourceBase]struct{}{}
	eventSourcesWithEnabledIdleHandlingLock sync.Mutex

	ErrNonUniqueEventSourceFactoryRegistration = errors.New("non unique event source factory registration")
	ErrHandlerAlreadyAdded                     = errors.New("handler already added to event source")
	ErrFileWatchingNotSupported                = errors.New("file watching is not supported")
)

// RegisterEventSourceFactory registers an event source factory for a given scheme (e.g. file).
func RegisterEventSourceFactory(scheme Scheme, factory EventSourceFactory) {
	eventSourceFactoriesLock.Lock()
	defer eventSourceFactoriesLock.Unlock()

	_, ok := eventSourceFactories[scheme]
	if ok {
		panic(ErrNonUniqueEventSourceFactoryRegistration)
	}
	eventSourceFactories[scheme] = factory
}

// GetEventSourceFactory returns the event source factory function for $scheme
// or (nil, false) if no factory is registered for $scheme.
func GetEventSourceFactory(scheme Scheme) (EventSourceFactory, bool) {
	eventSourceFactoriesLock.RLock()
	defer eventSourceFactoriesLock.RUnlock()

	factory, ok := eventSourceFactories[scheme]
	return factory, ok
}

type EventSourceFactory func(ctx *Context, resourceNameOrPattern Value) (EventSource, error)

// TODO: rework
// An EventSource is a source of events created by a scheme-specific (e.g. file) factory.
// Implementations should embed EventSourceBase.
type EventSource interface {
	GoValue
	Iterable
	OnEvent(microtask EventHandler) error
	Close()
	IsClosed() bool
}

// NewEventSource creates an EventSource by calling the factory registered for
// the scheme of $resourceNameOrPattern. For example for a path or path pattern
// the event source factory of the 'file' scheme is called.
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

type EventHandler func(event *Event)

// EventSourceBase provides handler registration for regular events and 'idle' handler registration.
// EventSource implementations should embed EventSourceBase and sould retrieve handlers by calling GetHandlers.
type EventSourceBase struct {
	lock          sync.RWMutex
	eventHandlers []EventHandler

	idleHandlers            []IdleEventSourceHandler
	lastEvents              *memds.TSArrayQueue[*Event]
	lastEventsQueueCreation time.Time
	isEventAdderRegistered  bool
}

// OnEvent registers an handler for regular (non-idle) events. The execution of $handler should take less than a few milliseconds.
func (evs *EventSourceBase) OnEvent(handler EventHandler) error {
	evs.lock.Lock()
	defer evs.lock.Unlock()

	return evs.onEventNoLock(handler)
}

func (evs *EventSourceBase) onEventNoLock(handler EventHandler) error {
	for _, e := range evs.eventHandlers {
		//NOTE: function pointers are not necessarily unique in Golang.
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

// GetHandlers returns current event listeners (handlers), they are safe to call without recovering.
func (evs *EventSourceBase) GetHandlers() []EventHandler {
	evs.lock.RLock()
	defer evs.lock.RUnlock()
	eventHandlers := make([]EventHandler, len(evs.eventHandlers))
	copy(eventHandlers, evs.eventHandlers)
	return eventHandlers
}

func (evs *EventSourceBase) RemoveAllHandlers() {
	evs.lock.Lock()
	defer evs.lock.Unlock()
	evs.eventHandlers = evs.eventHandlers[:0]
	evs.idleHandlers = evs.idleHandlers[:0]
	evs.isEventAdderRegistered = false

	eventSourcesWithEnabledIdleHandlingLock.Lock()
	delete(eventSourcesWithEnabledIdleHandling, evs)
	eventSourcesWithEnabledIdleHandlingLock.Unlock()
}

type IdleEventSourceHandler struct {
	//Should be >= HARD_MINIMUM_LAST_EVENT_AGE and <= MAX_MINIMUM_LAST_EVENT_AGE
	MinimumLastEventAge time.Duration

	//If nil defaults to a function always returning false.
	IsIgnoredEvent func(*Event) Bool

	//If false the handler is called after the next IDLE phase.
	DontWaitForFirstEvent bool

	//Microtask to execute, the execution should take less than a millisecond.
	Microtask func()

	registrationTime                    time.Time //set by OnIDLE
	hasSeenAnEvent                      bool      //set once by the IDLE management goroutine
	hasBeenCalledDuringCurrentIdlePhase bool      //set by the IDLE management goroutine
	afterFirstTick                      bool      //set by the IDLE management goroutine
}

// OnIDLE registers the provided handler to be called when the age of the last non-ignored event is >= .MinimumLastEventAge.
func (evs *EventSourceBase) OnIDLE(handler IdleEventSourceHandler) {
	evs.lock.Lock()
	defer evs.lock.Unlock()

	//check arguments

	if handler.MinimumLastEventAge < HARD_MINIMUM_LAST_EVENT_AGE {
		panic(fmt.Errorf("provided minimum last event age is should be >= HARD_MINIMUM_LAST_EVENT_AGE (%s)",
			HARD_MINIMUM_LAST_EVENT_AGE.String(),
		))
	}

	if handler.MinimumLastEventAge >= MAX_MINIMUM_LAST_EVENT_AGE {
		panic(fmt.Errorf("provided minimum last event age is should be <= HARD_MINIMUM_LAST_EVENT_AGE (%s)",
			MAX_MINIMUM_LAST_EVENT_AGE.String(),
		))
	}

	if handler.IsIgnoredEvent == nil {
		handler.IsIgnoredEvent = func(e *Event) Bool {
			return false
		}
	}

	handler.registrationTime = time.Now()
	evs.idleHandlers = append(evs.idleHandlers, handler)

	if evs.lastEvents == nil {
		evs.lastEvents = memds.NewTSArrayQueueWithConfig(memds.TSArrayQueueConfig[*Event]{
			AutoRemoveCondition: func(ev *Event) bool {
				return ev.Age() > 2*MAX_MINIMUM_LAST_EVENT_AGE
			},
		})
		evs.lastEventsQueueCreation = time.Now()
	}

	if !evs.isEventAdderRegistered {
		evs.isEventAdderRegistered = true
		evs.onEventNoLock(func(event *Event) {
			evs.lastEvents.EnqueueAutoRemove(event)
		})
	}

	eventSourcesWithEnabledIdleHandlingLock.Lock()
	eventSourcesWithEnabledIdleHandling[evs] = struct{}{}
	eventSourcesWithEnabledIdleHandlingLock.Unlock()

	spawnIdleEventSourceManager()
}

// spawnIdleEventSourceManager spawns a goroutine that wakes up every IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL
// and calls 'idle' handlers registered with EventSourceBase.OnIDLE.
func spawnIdleEventSourceManager() {
	if !idleEventSourceManagerSpawned.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer utils.Recover()

		ticker := time.NewTicker(IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL)
		defer ticker.Stop()

		for t := range ticker.C {
			eventSourcesWithEnabledIdleHandlingLock.Lock()
			eventSources := maps.Clone(eventSourcesWithEnabledIdleHandling)
			eventSourcesWithEnabledIdleHandlingLock.Unlock()

			for evs := range eventSources {
				callIdleHandlers(evs, t)
			}
		}
	}()
}

func callIdleHandlers(evs *EventSourceBase, now time.Time) {
	defer utils.Recover()

	evs.lock.RLock()
	defer evs.lock.RUnlock()

	lastEvents := evs.lastEvents.Values()
	timeSinceQueueCreation := now.Sub(evs.lastEventsQueueCreation)
	queueNeverHadElements := evs.lastEvents.HasNeverHadElements()

	for handlerIndex := range evs.idleHandlers {
		handler := &evs.idleHandlers[handlerIndex]
		waitForFirstEvent := !handler.DontWaitForFirstEvent
		defer func() {
			handler.afterFirstTick = true
		}()

		//if the queue has just been created and there have not been any event, we don't call the handler.
		if (waitForFirstEvent || !handler.afterFirstTick) &&
			queueNeverHadElements &&
			timeSinceQueueCreation <= IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL {
			continue
		}

		recentEvent := false

		//we don't call the handler if at least one of the events is too recent AND is not ignored.
		for _, event := range lastEvents {
			age := event.AgeWithCurrentTime(now)

			if handler.IsIgnoredEvent(event) {
				continue
			}

			handler.hasSeenAnEvent = true

			if age < handler.MinimumLastEventAge {
				recentEvent = true
				break
			}
		}

		if recentEvent {
			handler.hasBeenCalledDuringCurrentIdlePhase = false
			continue
		}
		//IDLE

		if !handler.hasSeenAnEvent && waitForFirstEvent {
			continue
		}

		if handler.hasBeenCalledDuringCurrentIdlePhase {
			continue
		}

		handler.hasBeenCalledDuringCurrentIdlePhase = true

		//call handler
		func() {
			defer utils.Recover()
			handler.Microtask()
		}()
	}
}
