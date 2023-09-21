package http_ns

import (
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	DEFAULT_PUSHED_BYTESTREAM_CHUNK_SIZE_RANGE = core.NewIncludedEndIntRange(100, 1000)
	BYTESTREAM_SSE_STREAM_STOP_TIMEOUT         = 10 * time.Millisecond
	BYTESTREAM_CHUNK_WAIT_TIMEOUT              = 2 * time.Millisecond
)

func pushByteStream(byteStream core.ReadableStream, h handlingArguments) error {
	h.logger.Print("publish binary stream for", h.req.Path)

	streamId := string(h.req.Session.Id) + string(h.req.Path)

	sseStream, sseServer, err := h.server.getOrCreateStream(streamId)
	if err != nil {
		return err
	}

	//TODO: implement a single subscription stream type to reduce memory and CPU usage
	ctx := h.state.Ctx

	go func() {

		defer func() {
			defer utils.Recover()
			sseStream.Stop()
			ctx.CancelGracefully()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			chunk, err := byteStream.WaitNextChunk(ctx, nil, DEFAULT_PUSHED_BYTESTREAM_CHUNK_SIZE_RANGE, BYTESTREAM_CHUNK_WAIT_TIMEOUT)

			if err == nil || (errors.Is(err, core.ErrEndOfStream) && chunk != nil) {
				data, err := chunk.Data(ctx)
				if err != nil {
					h.logger.Print("error while getting data of binary chunk:", err)
					return
				}

				bytes := data.(*core.ByteSlice).Bytes

				if len(bytes) != 0 {
					b64 := make([]byte, base64.StdEncoding.EncodedLen(len(bytes)))
					base64.StdEncoding.Encode(b64, bytes)

					sseStream.PublishAsync(&ServerSentEvent{
						timestamp: time.Now(),
						Data:      b64,
					}) //TODO: make sure call never blocks
				}
			}

			if errors.Is(err, core.ErrEndOfStream) {
				sseStream.GracefulStop(ctx, BYTESTREAM_SSE_STREAM_STOP_TIMEOUT)
				return
			}

			if errors.Is(err, core.ErrStreamChunkWaitTimeout) {
				continue
			}

			if err != nil {
				h.logger.Print("unexpected error while reading bytestrem", err)
				return
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
