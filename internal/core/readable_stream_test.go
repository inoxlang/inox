package core

import (
	"runtime"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestWrappedWatcherStream(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 10)
	}

	t.Run("WaitNext", func(t *testing.T) {

		t.Run("stream stopped after delay", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
			stream := watcher.Stream(ctx, &ReadableStreamConfiguration{}).(*wrappedWatcherStream)

			watcher.values <- Str("a")

			go func() {
				time.Sleep(10 * time.Millisecond)
				watcher.Stop()
			}()

			next, err := stream.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, Str("a"), next)

			next, err = stream.WaitNext(ctx, nil, time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}

			assert.Nil(t, next)
		})

		t.Run("stream already stopped", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
			stream := watcher.Stream(ctx, &ReadableStreamConfiguration{}).(*wrappedWatcherStream)

			watcher.values <- Str("a")
			watcher.Stop()

			next, err := stream.WaitNext(ctx, nil, time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}
			assert.Nil(t, next)
		})

	})

	t.Run("WaitNextChunk", func(t *testing.T) {

		t.Run("configured chunk size = 2..3, stream stopped after delay", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
			stream := watcher.Stream(ctx, &ReadableStreamConfiguration{}).(*wrappedWatcherStream)

			watcher.values <- Str("a")
			watcher.values <- Str("b")

			go func() {
				time.Sleep(10 * time.Millisecond)
				watcher.Stop()
			}()

			next, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, NewWrappedValueList(Str("a"), Str("b")), next.data)

			time.Sleep(20 * time.Millisecond)
			next, err = stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}
			assert.Nil(t, next)
		})

		t.Run("configured chunk size = 2..3, stream already stopped", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()

			watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
			stream := watcher.Stream(ctx, &ReadableStreamConfiguration{}).(*wrappedWatcherStream)

			watcher.values <- Str("a")
			watcher.values <- Str("b")
			watcher.Stop()

			next, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}
			assert.Nil(t, next)
		})

	})

	//TODO: add more tests
}

func TestElementsStream(t *testing.T) {

	//TODO: add more tests
	t.Run("WaitNextChunk", func(t *testing.T) {
		t.Run("configured chunk size = 2..3, 1 element", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()
			stream := &ElementsStream{
				filter:   ANYVAL_PATTERN,
				elements: []Value{Int(1)},
			}

			chunk1, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}

			assert.Equal(t, NewWrappedValueList(Int(1)), chunk1.data)

			chunk2, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}

			assert.Nil(t, chunk2)
		})

		t.Run("configured chunk size = 2..3, 2 elements", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()
			stream := &ElementsStream{
				filter:   ANYVAL_PATTERN,
				elements: []Value{Int(1), Int(2)},
			}

			chunk1, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}

			assert.Equal(t, NewWrappedValueList(Int(1), Int(2)), chunk1.data)

			chunk2, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}

			assert.Nil(t, chunk2)
		})

		t.Run("configured chunk size = 2..3, 3 elements", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()
			stream := &ElementsStream{
				filter:   ANYVAL_PATTERN,
				elements: []Value{Int(1), Int(2), Int(3)},
			}

			chunk1, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewWrappedValueList(Int(1), Int(2)), chunk1.data)

			chunk2, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}

			assert.Equal(t, NewWrappedValueList(Int(3)), chunk2.data)
		})

		t.Run("configured chunk size = 2..3, 4 elements", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			defer ctx.CancelGracefully()
			stream := &ElementsStream{
				filter:   ANYVAL_PATTERN,
				elements: []Value{Int(1), Int(2), Int(3), Int(4)},
			}

			chunk1, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewWrappedValueList(Int(1), Int(2)), chunk1.data)

			chunk2, err := stream.WaitNextChunk(ctx, nil, NewIncludedEndIntRange(2, 3), time.Second)
			if !assert.ErrorIs(t, err, ErrEndOfStream) {
				return
			}

			assert.Equal(t, NewWrappedValueList(Int(3), Int(4)), chunk2.data)
		})

	})
}

func TestByteStream(t *testing.T) {

	//TODO: add more tests
	t.Run("RingBuffer source", func(t *testing.T) {
		const TIMEOUT = time.Second / 4
		bufferSize := ByteCount(20)

		t.Run("WaitNext", func(t *testing.T) {
			t.Run("1 byte in buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer := NewRingBuffer(ctx, bufferSize)
				buffer.Write([]byte("a"))
				stream := buffer.Stream(ctx, nil).(*ReadableByteStream)

				byte, err := stream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, Byte('a'), byte)

				byte, err = stream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamElemWaitTimeout) {
					return
				}

				assert.Nil(t, byte)
			})

			t.Run("2 bytes in buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer := NewRingBuffer(ctx, bufferSize)
				buffer.Write([]byte("ab"))
				stream := buffer.Stream(ctx, nil).(*ReadableByteStream)

				byte, err := stream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, Byte('a'), byte)

				byte, err = stream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, Byte('b'), byte)

				byte, err = stream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamElemWaitTimeout) {
					return
				}

				assert.Nil(t, byte)
			})
		})

		t.Run("WaitNextChunk", func(t *testing.T) {

			chunkSizeRange := NewIncludedEndIntRange(5, 10)

			t.Run("configured chunk size = 5..10, 4 bytes in buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer := NewRingBuffer(ctx, bufferSize)
				buffer.Write([]byte("abcd"))
				stream := buffer.Stream(ctx, nil).(*ReadableByteStream)

				chunk1, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcd"), true, ""), chunk1.data)

				chunk2, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamChunkWaitTimeout) {
					return
				}

				assert.Nil(t, chunk2)
			})

			t.Run("configured chunk size = 5..10, 5 bytes in buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer := NewRingBuffer(ctx, bufferSize)
				buffer.Write([]byte("abcde"))
				stream := buffer.Stream(ctx, nil).(*ReadableByteStream)

				chunk1, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcde"), true, ""), chunk1.data)

				chunk2, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamChunkWaitTimeout) {
					return
				}

				assert.Nil(t, chunk2)
			})

			t.Run("configured chunk size = 5..10, 10 bytes in buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer := NewRingBuffer(ctx, bufferSize)
				buffer.Write([]byte("abcdefghij"))
				stream := buffer.Stream(ctx, nil).(*ReadableByteStream)

				chunk1, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcdefghij"), true, ""), chunk1.data)

				chunk2, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamChunkWaitTimeout) {
					return
				}

				assert.Nil(t, chunk2)
			})

			t.Run("configured chunk size = 5..10, 11 bytes in buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer := NewRingBuffer(ctx, bufferSize)
				buffer.Write([]byte("abcdefghijk"))
				stream := buffer.Stream(ctx, nil).(*ReadableByteStream)

				chunk1, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcdefghij"), true, ""), chunk1.data)

				chunk2, err := stream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("k"), true, ""), chunk2.data)
			})
		})

	})
}

func TestConfluenceStream(t *testing.T) {
	const TIMEOUT = time.Second / 4

	//TODO: test more combinations

	t.Run("byte streams created from ring buffers", func(t *testing.T) {
		bufferSize := ByteCount(20)

		t.Run("WaitNext", func(t *testing.T) {
			t.Run("1 byte in first buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				buffer1 := NewRingBuffer(ctx, bufferSize)
				buffer1.Write([]byte("a"))
				stream1 := buffer1.Stream(ctx, nil).(*ReadableByteStream)

				buffer2 := NewRingBuffer(ctx, bufferSize)
				stream2 := buffer2.Stream(ctx, nil).(*ReadableByteStream)

				confluenceStream, err := NewConfluenceStream(ctx, []ReadableStream{stream1, stream2})
				if !assert.NoError(t, err) {
					return
				}

				byte, err := confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, Byte('a'), byte)

				byte, err = confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamElemWaitTimeout) {
					return
				}

				assert.Nil(t, byte)
			})

			t.Run("2 bytes in first buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				buffer1 := NewRingBuffer(ctx, bufferSize)
				buffer1.Write([]byte("ab"))
				stream1 := buffer1.Stream(ctx, nil).(*ReadableByteStream)

				buffer2 := NewRingBuffer(ctx, bufferSize)
				stream2 := buffer2.Stream(ctx, nil).(*ReadableByteStream)

				confluenceStream, err := NewConfluenceStream(ctx, []ReadableStream{stream1, stream2})
				if !assert.NoError(t, err) {
					return
				}

				byte, err := confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, Byte('a'), byte)

				byte, err = confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, Byte('b'), byte)

				byte, err = confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamElemWaitTimeout) {
					return
				}

				assert.Nil(t, byte)
			})

			t.Run("1 byte in both buffers", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()

				buffer1 := NewRingBuffer(ctx, bufferSize)
				buffer1.Write([]byte("a"))
				stream1 := buffer1.Stream(ctx, nil).(*ReadableByteStream)

				buffer2 := NewRingBuffer(ctx, bufferSize)
				buffer2.Write([]byte("b"))
				stream2 := buffer2.Stream(ctx, nil).(*ReadableByteStream)

				confluenceStream, err := NewConfluenceStream(ctx, []ReadableStream{stream1, stream2})
				if !assert.NoError(t, err) {
					return
				}

				byte, err := confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, Byte('a'), byte)

				byte, err = confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, Byte('b'), byte)

				byte, err = confluenceStream.WaitNext(ctx, nil, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamElemWaitTimeout) {
					return
				}

				assert.Nil(t, byte)
			})
		})

		t.Run("WaitNextChunk", func(t *testing.T) {

			chunkSizeRange := NewIncludedEndIntRange(5, 10)

			t.Run("configured chunk size = 5..10, 4 bytes in first buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer1 := NewRingBuffer(ctx, bufferSize)
				buffer1.Write([]byte("abcd"))
				stream1 := buffer1.Stream(ctx, nil).(*ReadableByteStream)

				buffer2 := NewRingBuffer(ctx, bufferSize)
				stream2 := buffer2.Stream(ctx, nil).(*ReadableByteStream)

				confluenceStream, err := NewConfluenceStream(ctx, []ReadableStream{stream1, stream2})
				if !assert.NoError(t, err) {
					return
				}

				chunk1, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcd"), true, ""), chunk1.data)

				chunk2, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamChunkWaitTimeout) {
					return
				}

				assert.Nil(t, chunk2)
			})

			t.Run("configured chunk size = 5..10, 5 bytes in buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer1 := NewRingBuffer(ctx, bufferSize)
				buffer1.Write([]byte("abcde"))
				stream1 := buffer1.Stream(ctx, nil).(*ReadableByteStream)

				buffer2 := NewRingBuffer(ctx, bufferSize)
				stream2 := buffer2.Stream(ctx, nil).(*ReadableByteStream)

				confluenceStream, err := NewConfluenceStream(ctx, []ReadableStream{stream1, stream2})
				if !assert.NoError(t, err) {
					return
				}

				chunk1, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcde"), true, ""), chunk1.data)

				chunk2, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamChunkWaitTimeout) {
					return
				}

				assert.Nil(t, chunk2)
			})

			t.Run("configured chunk size = 5..10, 10 bytes in first buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer1 := NewRingBuffer(ctx, bufferSize)
				buffer1.Write([]byte("abcdefghij"))
				stream1 := buffer1.Stream(ctx, nil).(*ReadableByteStream)

				buffer2 := NewRingBuffer(ctx, bufferSize)
				stream2 := buffer2.Stream(ctx, nil).(*ReadableByteStream)

				confluenceStream, err := NewConfluenceStream(ctx, []ReadableStream{stream1, stream2})
				if !assert.NoError(t, err) {
					return
				}

				chunk1, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcdefghij"), true, ""), chunk1.data)

				chunk2, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.ErrorIs(t, err, ErrStreamChunkWaitTimeout) {
					return
				}

				assert.Nil(t, chunk2)
			})

			t.Run("configured chunk size = 5..10, 11 bytes in first buffer", func(t *testing.T) {
				ctx := NewContext(ContextConfig{})
				defer ctx.CancelGracefully()
				buffer1 := NewRingBuffer(ctx, bufferSize)
				buffer1.Write([]byte("abcdefghijk"))
				stream1 := buffer1.Stream(ctx, nil).(*ReadableByteStream)

				buffer2 := NewRingBuffer(ctx, bufferSize)
				stream2 := buffer2.Stream(ctx, nil).(*ReadableByteStream)

				confluenceStream, err := NewConfluenceStream(ctx, []ReadableStream{stream1, stream2})
				if !assert.NoError(t, err) {
					return
				}

				chunk1, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("abcdefghij"), true, ""), chunk1.data)

				chunk2, err := confluenceStream.WaitNextChunk(ctx, nil, chunkSizeRange, TIMEOUT)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, NewByteSlice([]byte("k"), true, ""), chunk2.data)
			})
		})
	})
}
