package http_ns

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/inoxlang/inox/internal/core"
)

const (
	STOPPING_SUB_WAIT_TIME = time.Millisecond
)

type multiSubscriptionSSEStream struct {
	lock                sync.Mutex
	id                  string
	eventlog            eventLog
	events              chan *ServerSentEvent
	stopGoroutine       chan struct{}
	lastEventHandledAck chan struct{}
	stopping            atomic.Bool
	subscriptions       map[*Subscription]struct{}
	subCount            int
}

func newStream(id string, buffSize int) *multiSubscriptionSSEStream {
	return &multiSubscriptionSSEStream{
		id:                  id,
		subscriptions:       map[*Subscription]struct{}{},
		eventlog:            make(eventLog, 0),
		events:              make(chan *ServerSentEvent, buffSize),
		stopGoroutine:       make(chan struct{}, 1),
		lastEventHandledAck: make(chan struct{}, 1),
	}
}

func (s *multiSubscriptionSSEStream) startGoroutine() {
	go func(s *multiSubscriptionSSEStream) {
		defer func() {
			close(s.lastEventHandledAck)
			close(s.stopGoroutine)
			close(s.events)
		}()

		for {
			select {
			case event := <-s.events:
				s.sendEventToSubscriptions(event)

				if s.stopping.Load() && len(s.events) == 0 {
					s.lastEventHandledAck <- struct{}{}
				}
			case <-s.stopGoroutine:
				return
			}
		}
	}(s)
}

func (s *multiSubscriptionSSEStream) sendEventToSubscriptions(event *ServerSentEvent) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.sendEventToSubscriptionsNoLock(event)
}

func (s *multiSubscriptionSSEStream) sendEventToSubscriptionsNoLock(event *ServerSentEvent) {
	for sub := range s.subscriptions {
		if sub.IsStopped() {
			delete(s.subscriptions, sub)
			s.subCount--
			continue
		}
		sub.events <- event //TODO: prevent blocking
	}
}

// Stop stops the stream, it does not wait for events to be sent.
func (s *multiSubscriptionSSEStream) Stop() {
	if s.stopping.CompareAndSwap(false, true) {
		s.stopGoroutine <- struct{}{}
		s.removeAllSubscriptions()
	}
}

// GracefulStop stops the stream, it waits for events to be sent.
func (s *multiSubscriptionSSEStream) GracefulStop(ctx *core.Context, timeout time.Duration) {
	if s.stopping.CompareAndSwap(false, true) {

		defer func() {
			s.removeAllSubscriptions()
			s.stopGoroutine <- struct{}{}
		}()

		start := time.Now()

		select {
		case <-ctx.Done():
			return
		case <-time.After(timeout - time.Since(start)):
			return
		case <-s.lastEventHandledAck:
			break
		}

		s.lock.Lock()
		defer s.lock.Unlock()

		// stop asynchronously the subscriptions
		for sub := range s.subscriptions {
			sub.StopAsync()
		}

		//wait for the subscriptions to be stopped
	wait_loop:
		for {
			for sub := range s.subscriptions {
				if sub.IsStopped() {
					continue
				}
				time.Sleep(STOPPING_SUB_WAIT_TIME)
				if time.Since(start) > timeout || ctx.IsDone() {
					return
				}
				continue wait_loop
			}
			return
		}

	}
}

func (s *multiSubscriptionSSEStream) QueuedEventCount() int {
	return len(s.events)
}

func (s *multiSubscriptionSSEStream) PublishAsync(e *ServerSentEvent) {
	if s.stopping.Load() {
		panic(errors.New("failed to publish SSE: SSE stream is stopping"))
	}

	s.events <- e
}

// addSubscription creates & add a new subscription to a stream
func (s *multiSubscriptionSSEStream) addSubscription(requestContext *core.Context, lastEventId int, url core.URL) *Subscription {
	s.lock.Lock()
	defer s.lock.Unlock()

	sub := &Subscription{
		lastEventId: lastEventId,
		events:      make(chan *ServerSentEvent, DEFAULT_SUBSCRIPTION_CHAN_SIZE),
		url:         url,
	}

	s.subscriptions[sub] = struct{}{}
	return sub
}

func (s *multiSubscriptionSSEStream) removeSubscription(sub *Subscription) (newCount int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.subscriptions[sub]; ok {
		s.subCount--
		delete(s.subscriptions, sub)
	}

	return s.subCount
}

func (str *multiSubscriptionSSEStream) removeAllSubscriptions() {
	str.lock.Lock()
	defer str.lock.Unlock()

	for sub := range str.subscriptions {
		if !sub.IsStopped() {
			sub.StopAsync()
		}
	}

	str.subCount = 0
	str.subscriptions = map[*Subscription]struct{}{}
}

func (str *multiSubscriptionSSEStream) SubscriptionCount() int {
	str.lock.Lock()
	defer str.lock.Unlock()

	return str.subCount
}
