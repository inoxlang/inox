package internal

import (
	"sync/atomic"

	core "github.com/inox-project/inox/internal/core"
)

const DEFAULT_SUBSCRIPTION_CHAN_SIZE = 50

var (
	endEvent = &ServerSentEvent{}
)

type Subscription struct {
	// a event equal to stopEvent will cause the server to stop the subscription,
	// in this way we avoid creating a channel just for stopping the subscription.
	events chan *ServerSentEvent

	lastEventId int
	url         core.URL

	stopped atomic.Bool
}

// StopAsync causes the server to stop the event stream after all other events have been sent.
func (s *Subscription) StopAsync() {
	if s.stopped.Load() {
		return
	}
	s.events <- endEvent
}

func (s *Subscription) IsStopped() bool {
	return s.stopped.Load()
}
