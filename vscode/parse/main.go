//go:build js

package main

import (
	"encoding/json"
	"errors"
	"math"
	"syscall/js"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/oklog/ulid/v2"
)

const (
	CHUNK_LIST_CAPACITY = 100
)

var lastParsedChunks = make([]parsedChunk, 0, CHUNK_LIST_CAPACITY)

type parsedChunk struct {
	*parse.ParsedChunkSource
	id string
}

func main() {
	lastParsedChunks = lastParsedChunks[0:0:len(lastParsedChunks)]
	exports := js.Global().Get("exports")

	exports.Set("get_span_line_column", js.FuncOf(func(this js.Value, args []js.Value) any {
		chunkId := args[0].String()
		start := args[1].Int()
		end := args[2].Int()

		if start > math.MaxInt32 {
			return makeResult(nil, errors.New("start index is too big"))
		}
		if end > math.MaxInt32 {
			return makeResult(nil, errors.New("end index is too big"))
		}

		for _, chunk := range lastParsedChunks {
			if chunk.id == chunkId {
				line, col := chunk.GetSpanLineColumn(parse.NodeSpan{
					Start: int32(start),
					End:   int32(end),
				})

				return makeResult([]any{line, col}, nil)
			}
		}

		return makeResult(nil, errors.New("invalid start or end index"))
	}))

	exports.Set("get_span_end_line_column", js.FuncOf(func(this js.Value, args []js.Value) any {
		chunkId := args[0].String()
		start := args[1].Int()
		end := args[2].Int()

		if start > math.MaxInt32 {
			return makeResult(nil, errors.New("start index is too big"))
		}
		if end > math.MaxInt32 {
			return makeResult(nil, errors.New("end index is too big"))
		}

		for _, chunk := range lastParsedChunks {
			if chunk.id == chunkId {
				line, col := chunk.GetEndSpanLineColumn(parse.NodeSpan{
					Start: int32(start),
					End:   int32(end),
				})

				return makeResult([]any{line, col}, nil)
			}
		}

		return makeResult(nil, errors.New("invalid start or end index"))
	}))

	exports.Set("parse_chunk", js.FuncOf(func(this js.Value, args []js.Value) any {
		fpath := args[0].String()

		//we avoid computing the directory's path on purpose.
		dirpath := args[1].String()
		content := args[2].String()

		if fpath == "" {
			return makeResult(nil, errors.New("fpath should not be empty"))
		}

		chunk, err := parse.ParseChunk(content, fpath, parse.ParserOptions{})

		var result struct {
			CompleteErrorMessage string                      `json:"completeErrorMessage"`
			Errors               []*parse.ParsingError       `json:"errors,omitempty"`
			ErrorPositions       []parse.SourcePositionRange `json:"errorPositions,omitempty"`
			Chunk                *parse.Chunk                `json:"chunk,omitempty"`
			ChunkId              string                      `json:"chunkId,omitempty"`
		}

		if chunk != nil {
			result.ChunkId = ulid.Make().String()
			result.Chunk = chunk

			if len(lastParsedChunks) == CHUNK_LIST_CAPACITY {
				//shift to remove the oldest chunk
				copy(lastParsedChunks[0:CHUNK_LIST_CAPACITY], lastParsedChunks[1:])
				lastParsedChunks = lastParsedChunks[:len(lastParsedChunks)-1]
			}

			lastParsedChunks = append(lastParsedChunks, parsedChunk{
				id: result.ChunkId,
				ParsedChunkSource: parse.NewParsedChunkSource(chunk, parse.SourceFile{
					NameString:             fpath,
					UserFriendlyNameString: fpath,
					Resource:               fpath,
					IsResourceURL:          false,
					CodeString:             content,
					ResourceDir:            dirpath,
				}),
			})
		}

		var aggregationError *parse.ParsingErrorAggregation
		if errors.As(err, &aggregationError) {
			result.Errors = aggregationError.Errors
			result.ErrorPositions = aggregationError.ErrorPositions
			result.CompleteErrorMessage = aggregationError.Message
		} else if err != nil {
			result.CompleteErrorMessage = err.Error()
		}

		marshalled, err := json.Marshal(result)
		if err != nil {
			return makeResult(nil, err)
		}
		return makeResult(string(marshalled), nil)
	}))

	channel := make(chan struct{})
	<-channel
}

func makeResult(value any, error error) any {
	if error == nil {
		return js.ValueOf([]any{value, nil})
	}
	return js.ValueOf([]any{nil, error})
}
