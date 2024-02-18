//go:build js

package main

import (
	"encoding/json"
	"errors"
	"math"
	"syscall/js"

	"github.com/inoxlang/inox/internal/jsoniter"
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

var tokenSerializationBuf [5_000_000]byte

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

	exports.Set("parse_get_tokens", js.FuncOf(func(this js.Value, args []js.Value) any {
		fpath := args[0].String()

		//we avoid computing the directory's path on purpose.
		dirpath := args[1].String()
		content := args[2].String()

		chunk, _ := parse.ParseChunk(content, fpath, parse.ParserOptions{})

		if chunk == nil {
			return makeResult("[]", nil)
		}

		parsedChunkSource := parse.NewParsedChunkSource(chunk, parse.SourceFile{
			NameString:             fpath,
			UserFriendlyNameString: fpath,
			Resource:               fpath,
			IsResourceURL:          false,
			CodeString:             content,
			ResourceDir:            dirpath,
		})

		resultTokens := makeResultTokens(parsedChunkSource)
		return makeResult(string(marshalTokens(resultTokens, tokenSerializationBuf[:0])), nil)
	}))

	exports.Set("get_tokens_of_chunk", js.FuncOf(func(this js.Value, args []js.Value) any {
		chunkId := args[0].String()

		for _, chunk := range lastParsedChunks {
			if chunk.id != chunkId {
				continue
			}

			tokens := parse.GetTokens(chunk.Node, chunk.Node, false)

			resultTokens := make([]resultToken, len(tokens))

			for i, token := range tokens {
				resultToken := resultToken{Token: token}

				line, col := chunk.GetSpanLineColumn(token.Span)
				resultToken.StartLine = line
				resultToken.StartColumn = col

				endLine, endCol := chunk.GetEndSpanLineColumn(token.Span)
				resultToken.EndLine = endLine
				resultToken.EndColumn = endCol

				resultTokens[i] = resultToken
			}

			return makeResult(string(marshalTokens(resultTokens, tokenSerializationBuf[:0])), nil)
		}

		return makeResult(nil, errors.New("unknown chunk"))
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

type resultToken struct {
	parse.Token
	StartLine   int32 `json:"startLine"`
	EndLine     int32 `json:"endLine"`
	StartColumn int32 `json:"startColumn"`
	EndColumn   int32 `json:"endColumn"`
}

func makeResultTokens(chunk *parse.ParsedChunkSource) []resultToken {
	tokens := parse.GetTokens(chunk.Node, chunk.Node, false)
	resultTokens := make([]resultToken, len(tokens))

	for i, token := range tokens {
		resultToken := resultToken{Token: token}

		line, col := chunk.GetSpanLineColumn(token.Span)
		resultToken.StartLine = line
		resultToken.StartColumn = col

		endLine, endCol := chunk.GetEndSpanLineColumn(token.Span)
		resultToken.EndLine = endLine
		resultToken.EndColumn = endCol

		resultTokens[i] = resultToken
	}

	return resultTokens

}

func marshalTokens(tokens []resultToken, buffer []byte) string {
	w := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	w.SetBuffer(buffer)

	w.WriteArrayStart()
	for i, token := range tokens {

		if i != 0 {
			w.WriteMore()
		}
		w.WriteObjectStart()

		w.WriteObjectField("type")
		w.WriteUint64(uint64(token.Type))

		w.WriteMore()
		w.WriteObjectField("span")
		{
			w.WriteObjectStart()

			w.WriteObjectField("start")
			w.WriteInt32(token.Span.Start)

			w.WriteMore()
			w.WriteObjectField("end")
			w.WriteInt32(token.Span.End)

			w.WriteObjectEnd()
		}

		w.WriteMore()
		w.WriteObjectField("raw")
		w.WriteString(token.Raw)

		w.WriteMore()
		w.WriteObjectField("meta")
		w.WriteUint(uint(token.Meta))

		w.WriteMore()
		w.WriteObjectField("startLine")
		w.WriteInt32(token.StartLine)

		w.WriteMore()
		w.WriteObjectField("startColumn")
		w.WriteInt32(token.StartColumn)

		w.WriteMore()
		w.WriteObjectField("endLine")
		w.WriteInt32(token.EndLine)

		w.WriteMore()
		w.WriteObjectField("endColumn")
		w.WriteInt32(token.EndColumn)

		w.WriteObjectEnd()
	}
	w.WriteArrayEnd()

	return string(w.Buffer())
}
