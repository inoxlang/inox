package dom_ns

import (
	"sync"
	"time"

	core "github.com/inoxlang/inox/internal/core"
)

const EVENT_CHAN_SIZE = 100

type DomEventSource struct {
	core.EventSourceHandlerManagement
	core.NotClonableMixin
	core.NoReprMixin

	events   chan string
	lock     sync.RWMutex
	isClosed bool
}

func (evs *DomEventSource) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "close":
		return core.WrapGoMethod(evs.Close), true
	}
	return nil, false
}

func (evs *DomEventSource) Prop(ctx *core.Context, name string) core.Value {
	method, ok := evs.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, evs))
	}
	return method
}

func (*DomEventSource) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*DomEventSource) PropertyNames(ctx *core.Context) []string {
	return []string{"close"}
}

func (evs *DomEventSource) Close() {
	evs.lock.Lock()
	defer evs.lock.Unlock()
	evs.isClosed = true
	close(evs.events) // should be closed by sender
}

func (evs *DomEventSource) IsClosed() bool {
	evs.lock.RLock()
	defer evs.lock.RUnlock()
	return evs.isClosed
}

func (evs *DomEventSource) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEventSourceIterator(evs, config)
}

func NewDomEventSource(ctx *core.Context, resourceNameOrPattern core.Value) (*DomEventSource, error) {
	eventSource := &DomEventSource{
		events: make(chan string, EVENT_CHAN_SIZE),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				eventSource.Close()
				return

			case e := <-eventSource.events:
				value := core.NewRecordFromMap(core.ValMap{
					"type": core.Str(e),
				})

				domEvent := core.NewEvent(value, core.Date(time.Now()))

				for _, handler := range eventSource.GetHandlers() {
					func() {
						defer func() { recover() }()
						handler(domEvent)
					}()
				}
			}
		}
	}()

	return eventSource, nil
}
