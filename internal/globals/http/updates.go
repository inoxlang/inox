package internal

import (
	"context"
	"errors"
	"net/http"
	"time"

	core "github.com/inox-project/inox/internal/core"
	_dom "github.com/inox-project/inox/internal/globals/dom"
	_html "github.com/inox-project/inox/internal/globals/html"
)

const (
	VIEW_UPDATE_WATCHING_TIMEOUT        = 10 * time.Millisecond
	VIEW_UPDATE_SSE_STREAM_STOP_TIMEOUT = 10 * time.Millisecond
)

func pushViewUpdates(view *_dom.View, h handlingArguments) error {

	h.logger.Println("publish view updates for", h.req.Path)

	streamId := string(h.req.Session.Id) + string(h.req.Path)

	sseStream, sseServer, err := h.server.getOrCreateStream(streamId)
	if err != nil {
		return err
	}

	//TODO: implement a single subscription stream type to reduce memory and CPU usage
	ctx := h.state.Ctx

	go func() {
		defer func() {
			sseStream.Stop()
			ctx.Cancel()
		}()

		w := view.Watcher(h.state.Ctx, core.WatcherConfiguration{Filter: core.MUTATION_PATTERN})

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, err := w.WaitNext(ctx, nil, VIEW_UPDATE_WATCHING_TIMEOUT)
			if errors.Is(err, core.ErrStoppedWatcher) || errors.Is(err, context.Canceled) {
				sseStream.GracefulStop(ctx, VIEW_UPDATE_SSE_STREAM_STOP_TIMEOUT)
				return
			}

			if errors.Is(err, core.ErrWatchTimeout) {
				continue
			}

			if err == nil {
				node := view.Node()
				bytes := _html.Render(ctx, node)

				sseStream.PublishAsync(&ServerSentEvent{
					timestamp: time.Now(),
					Data:      bytes.Bytes,
				}) //TODO: make sure call never blocks
			}
		}
	}()

	http.NewResponseController(h.rw.rw).SetWriteDeadline(time.Now().Add(SSE_STREAM_WRITE_TIMEOUT))

	sseServer.PushSubscriptionEvents(eventPushConfig{
		ctx:     ctx,
		stream:  sseStream,
		writer:  h.rw,
		request: h.req,
		logger:  h.logger,
	})

	return nil

}
