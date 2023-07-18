package http_ns

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"github.com/inoxlang/inox/internal/utils"
	"gopkg.in/cenkalti/backoff.v1"
)

const (
	INITIAL_SSE_CLIENT_BUFFER_SIZE = 4096
	MAX_SSE_CLIENT_BUFFER_SIZE     = 1 << 16
)

var (
	SSE_ID_HEADER    = []byte("id:")
	SSE_DATA_HEADER  = []byte("data:")
	SSE_EVENT_HEADER = []byte("event:")
	SSE_RETRY_HEADER = []byte("retry:")

	_ = []core.EventSource{&ServerSentEventSource{}}
)

func init() {
	core.RegisterEventSourceFactory(core.Scheme("https"), func(ctx *core.Context, resourceNameOrPattern core.Value) (core.EventSource, error) {
		return NewEventSource(ctx, resourceNameOrPattern)
	})
}

type ServerSentEventSource struct {
	core.EventSourceHandlerManagement
	isClosed bool

	//retry             time.Time
	reconnectStrategy        backoff.BackOff
	additionalRequestHeaders map[string]string
	reconnectNotify          backoff.Notify
	httpClient               *http.Client
	url                      core.URL
	lastEventId              string
	maxBufferSize            int
	lock                     sync.RWMutex
	connected                bool
	context                  *core.Context
}

func NewEventSource(ctx *core.Context, resourceNameOrPattern core.Value) (*ServerSentEventSource, error) {
	url := resourceNameOrPattern.(core.URL)

	client, err := ctx.GetProtolClient(url)
	var httpClient *http.Client
	if err != nil {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	} else {
		httpClient = client.(*HttpClient).client
	}

	evs := &ServerSentEventSource{
		url:                      url,
		httpClient:               httpClient,
		additionalRequestHeaders: make(map[string]string),
		maxBufferSize:            MAX_SSE_CLIENT_BUFFER_SIZE,
		reconnectStrategy:        backoff.NewExponentialBackOff(),
		context:                  ctx.BoundChild(),
	}

	handleEvent := func(sse *ServerSentEvent) {
		event := sse.ToEvent()

		for _, handler := range evs.GetHandlers() {
			handler(event)
		}
	}

	fn := func() error {
		resp, err := evs.sendRequest()

		if err != nil {
			return err
		}
		// TODO: add additional validations ?
		if resp.StatusCode != 200 {

			resp.Body.Close()
			return fmt.Errorf("failed to connect to the event stream: %s", http.StatusText(resp.StatusCode))
		}

		defer resp.Body.Close()

		eventChan := make(chan *ServerSentEvent)
		errorChan := make(chan error)

		go evs.readEvents(resp, eventChan, errorChan)

		for {
			select {
			case <-evs.context.Done():
				return evs.context.Err()
			case err, ok := <-errorChan:
				if !ok {
					return nil
				}
				return err
			case msg, ok := <-eventChan:
				if !ok {
					return nil
				}
				handleEvent(msg)
			}
		}
	}

	go func() {
		_ = backoff.RetryNotify(fn, evs.reconnectStrategy, nil)
		//TODO: post mortem log (note: context is potentially already done)
	}()

	return evs, nil
}

func (evs *ServerSentEventSource) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "close":
		return core.WrapGoMethod(evs.Close), true
	}
	return nil, false
}

func (evs *ServerSentEventSource) Prop(ctx *core.Context, name string) core.Value {
	method, ok := evs.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, evs))
	}
	return method
}

func (*ServerSentEventSource) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*ServerSentEventSource) PropertyNames(ctx *core.Context) []string {
	return http_ns_symb.SSE_SOURCE_PROPNAMES
}

func (evs *ServerSentEventSource) Close() {
	evs.lock.Lock()
	defer evs.lock.Unlock()

	if evs.isClosed {
		return
	}

	evs.isClosed = true
	evs.context.Cancel()
}

func (evs *ServerSentEventSource) IsClosed() bool {
	evs.lock.RLock()
	defer evs.lock.RUnlock()
	return evs.isClosed
}

func (evs *ServerSentEventSource) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEventSourceIterator(evs, config)
}

func (evs *ServerSentEventSource) readEvents(resp *http.Response, events chan *ServerSentEvent, errorChan chan error) {
	// create event scanner
	scanner := bufio.NewScanner(resp.Body)
	initialBuffer := make([]byte, INITIAL_SSE_CLIENT_BUFFER_SIZE)
	scanner.Buffer(initialBuffer, evs.maxBufferSize)

	var splitByDoubleLines bufio.SplitFunc = func(data []byte, atEOF bool) (advance int, eventBytes []byte, err error) {

		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// we split at the doube line sequence
		if i, length := utils.FindDoubleLineSequence(data); i >= 0 {
			advance = i + length // we move after the event message and the double line sequence
			eventBytes = data[0:i]

			return
		}

		if atEOF {
			advance = len(data)
			eventBytes = data
			return
		}

		// we need to wait for more data
		advance = 0
		return
	}

	scanner.Split(splitByDoubleLines)

	defer func() {
		close(events)
		close(errorChan)
	}()

	for {
		select {
		case <-evs.context.Done():
			return
		default:
		}

		var eventMessage []byte
		if scanner.Scan() {
			eventMessage = scanner.Bytes()

		} else if err := scanner.Err(); err != nil {
			if errors.Is(err, context.Canceled) || err == io.EOF {
				errorChan <- nil
				return
			}
			evs.connected = false
			errorChan <- err
			return
		}

		evs.connected = true

		if msg, err := evs.parseEvent(eventMessage); err == nil { // ignore errors

			if len(msg.ID) > 0 {
				evs.lastEventId = string(msg.ID)
			} else {
				msg.ID = []byte(evs.lastEventId)
			}

			if msg.HasContent() {
				events <- msg
			}
		}
	}
}

func (evs *ServerSentEventSource) sendRequest() (*http.Response, error) {
	req, err := http.NewRequest("GET", string(evs.url), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(evs.context)

	headers := req.Header
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Accept", core.EVENT_STREAM_CTYPE)
	headers.Set("Connection", "keep-alive")

	if evs.lastEventId != "" {
		headers.Set("Last-*ServerSentEvent-ID", evs.lastEventId)
	}

	// for k, v := range evs.Headerh {
	// 	req.Header.Set(k, v)
	// }

	return evs.httpClient.Do(req)
}

func (c *ServerSentEventSource) parseEvent(eventMessage []byte) (event *ServerSentEvent, err error) {
	var e = &ServerSentEvent{}

	if len(eventMessage) < 1 {
		return nil, errors.New("event message is empty")
	}

	var line []byte
	var lastLineEnd int = 0

	// split the line by "\n" or "\r", per the spec
	for i, b := range eventMessage {
		switch b {
		case '\n', '\r':
			line = eventMessage[lastLineEnd:i]
			lastLineEnd = i + 1
		default:
			if i < len(eventMessage)-1 {
				continue
			}
			line = eventMessage[lastLineEnd:i]
		}

		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case 'i':
			if bytes.HasPrefix(line, SSE_ID_HEADER) {
				e.ID = utils.CopySlice(trimEventStreamHeader(len(SSE_ID_HEADER), line))
			}
		case 'd':
			if len(line) == 4 && bytes.Equal(line, []byte("data")) {
				// a line that simply contains the string "data" should be treated as a data field with an empty body
				e.Data = append(e.Data, byte('\n'))
			} else if bytes.HasPrefix(line, SSE_DATA_HEADER) {
				// the spec allows for multiple data fields per event, each followed with "\n".
				e.Data = append(e.Data[:], append(trimEventStreamHeader(len(SSE_DATA_HEADER), line), byte('\n'))...)
			}
		case 'e':
			if bytes.HasPrefix(line, SSE_EVENT_HEADER) {
				e.Event = utils.CopySlice(trimEventStreamHeader(len(SSE_EVENT_HEADER), line))
			}
		case 'r':
			if bytes.HasPrefix(line, SSE_RETRY_HEADER) {
				e.Retry = utils.CopySlice(trimEventStreamHeader(len(SSE_RETRY_HEADER), line))
			}
		default:
			// ignore
		}
	}

	// trim the last "\n" per the spec.
	e.Data = bytes.TrimSuffix(e.Data, []byte("\n"))
	e.timestamp = time.Now()

	return e, err
}

func trimEventStreamHeader(size int, data []byte) []byte {
	if data == nil || len(data) < size {
		return data
	}

	data = data[size:]
	// remove optional leading whitespace
	if len(data) > 0 && data[0] == 32 {
		data = data[1:]
	}

	// remove trailing new line
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return data
}
