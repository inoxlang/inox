package http_ns

import (
	"bytes"
	"net/http"
	"strconv"
	"sync"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const DEFAULT_EVENT_STREAM_BUFFER_SIZE = 200

var (
	NEWLINE_BYTES = []byte{'\n'}
)

type SseServer struct {
	lock           sync.RWMutex
	streams        map[string]*multiSubscriptionSSEStream
	eventTTL       time.Duration
	splitEventData bool //split data into multiple data: entries
}

func NewSseServer() *SseServer {
	return &SseServer{
		splitEventData: true,
		streams:        make(map[string]*multiSubscriptionSSEStream),
	}
}

// Close closes all the streams.
func (s *SseServer) Close() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for id := range s.streams {
		s.streams[id].Stop()
		delete(s.streams, id)
	}
}

func (s *SseServer) CreateStream(id string) *multiSubscriptionSSEStream {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.streams[id] != nil {
		return s.streams[id]
	}

	stream := newStream(id, DEFAULT_EVENT_STREAM_BUFFER_SIZE)
	stream.startGoroutine()
	s.streams[id] = stream

	return stream
}

func (s *SseServer) RemoveStream(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.streams[id] != nil {
		s.streams[id].Stop()
		delete(s.streams, id)
	}
}

func (s *SseServer) Publish(streamId string, event *ServerSentEvent) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	stream, ok := s.streams[streamId]
	if !ok {
		return
	}

	stream.PublishAsync(event)
}

func (s *SseServer) getStream(streamId string) *multiSubscriptionSSEStream {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.streams[streamId]
}

type eventPushConfig struct {
	ctx     *core.Context
	stream  *multiSubscriptionSSEStream
	writer  *HttpResponseWriter
	request *HttpRequest
	logger  zerolog.Logger
}

// PushSubscriptionEvents writes the headers required for event streaming & push events.
func (s *SseServer) PushSubscriptionEvents(config eventPushConfig) {
	w := config.writer.rw
	stream := config.stream

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}

	//send headers

	headerMap := w.Header()
	headerMap.Set("Connection", "keep-alive")
	headerMap.Set("Content-Type", mimeconsts.EVENT_STREAM_CTYPE)
	headerMap.Set("Cache-Control", "no-cache")

	//get last event id from headers & creates subscription

	lastEventId := 0
	if id := config.request.Request().Header.Get("Last-Event-ID"); id != "" {
		var err error
		lastEventId, err = strconv.Atoi(id)
		if err != nil {
			http.Error(w, "value of header Last-Event-ID should be a number", http.StatusBadRequest)
			return
		}
	}

	subscription := stream.addSubscription(config.ctx, lastEventId, config.request.URL)
	logger := config.logger.With().Str("sseStream", stream.id).Logger()

	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	defer func() {
		subscription.stopped.Store(true)

		newSubscriptionCount := stream.removeSubscription(subscription)

		if newSubscriptionCount == 0 {
			logger.Print("remove stream")
			s.RemoveStream(stream.id)
		}
	}()

	for event := range subscription.events { // at the end of the body we check if the subscription is stopping
		select {
		case <-config.ctx.Done():
			return
		default:
			if event == endEvent {
				close(subscription.events)
				return
			} else {
			}
		}

		// abort if the data buffer is empty
		if len(event.Data) == 0 && len(event.Comment) == 0 {
			break
		}

		// ignore event if expired
		if s.eventTTL != 0 && time.Now().After(event.timestamp.Add(s.eventTTL)) {
			continue
		}

		if len(event.Data) > 0 {
			w.Write(utils.StringAsBytes("id: "))
			w.Write(event.ID)
			w.Write(NEWLINE_BYTES)

			if s.splitEventData {
				lineStart := 0
				for i, b := range event.Data {
					if b == '\n' {
						//write line
						w.Write(utils.StringAsBytes("data: "))
						w.Write(event.Data[lineStart:i])
						w.Write(NEWLINE_BYTES)
						lineStart = i + 1
					}
				}

				if lineStart < len(event.Data) {
					//write last line
					w.Write(utils.StringAsBytes("data: "))
					w.Write(event.Data[lineStart:])
					w.Write(NEWLINE_BYTES)
				}

			} else {
				if bytes.HasPrefix(event.Data, []byte(":")) {
					w.Write(event.Data)
					w.Write(NEWLINE_BYTES)
				} else {
					w.Write(utils.StringAsBytes("data: "))
					w.Write(event.Data)
					w.Write(NEWLINE_BYTES)
				}
			}

			if len(event.Event) > 0 {
				w.Write(utils.StringAsBytes("event: "))
				w.Write(event.Event)
				w.Write(NEWLINE_BYTES)
			}

			if len(event.Retry) > 0 {
				w.Write(utils.StringAsBytes("retry: "))
				w.Write(event.Retry)
				w.Write(NEWLINE_BYTES)
			}
		}

		if len(event.Comment) > 0 {
			w.Write([]byte(": "))
			w.Write(event.Comment)
			w.Write(NEWLINE_BYTES)
		}

		w.Write(NEWLINE_BYTES)

		flusher.Flush()
	}
}
