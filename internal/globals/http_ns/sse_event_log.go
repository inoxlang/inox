package http_ns

import (
	"strconv"
	"time"
)

// eventLog contains the log of published events in a stream
type eventLog []*ServerSentEvent

func (e *eventLog) Add(ev *ServerSentEvent) {
	if !ev.HasContent() {
		return
	}

	ev.ID = []byte(e.currentindex())
	ev.timestamp = time.Now()
	*e = append(*e, ev)
}

// Clear events from eventlog
func (e *eventLog) Clear() {
	*e = nil
}

// Replay events to a subscriber
func (e *eventLog) Replay(s *Subscription) {
	for i := 0; i < len(*e); i++ {
		id, _ := strconv.Atoi(string((*e)[i].ID))
		if id >= s.lastEventId {
			s.events <- (*e)[i]
		}
	}
}

func (e *eventLog) currentindex() string {
	return strconv.Itoa(len(*e))
}
