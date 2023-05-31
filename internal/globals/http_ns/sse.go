package http_ns

import (
	core "github.com/inoxlang/inox/internal/core"

	"time"
)

type ServerSentEvent struct {
	ID      []byte
	Data    []byte
	Event   []byte
	Retry   []byte
	Comment []byte

	timestamp time.Time
}

func (e *ServerSentEvent) ToEvent() *core.Event {
	valMap := core.ValMap{
		"id":      core.Str(""),
		"data":    core.Str(""),
		"event":   core.Str(""),
		"retry":   core.Str(""),
		"comment": core.Str(""),
	}

	if len(e.ID) != 0 {
		valMap["id"] = core.Str(e.ID)
	}
	if len(e.Data) != 0 {
		valMap["data"] = core.Str(e.Data)
	}
	if len(e.Event) != 0 {
		valMap["event"] = core.Str(e.Event)
	}
	if len(e.Retry) != 0 {
		valMap["retry"] = core.Str(e.Retry)
	}
	if len(e.Comment) != 0 {
		valMap["comment"] = core.Str(e.Comment)
	}

	return core.NewEvent(core.NewRecordFromMap(valMap), core.Date(e.timestamp))
}

func (e *ServerSentEvent) HasContent() bool {
	return len(e.ID) > 0 || len(e.Data) > 0 || len(e.Event) > 0 || len(e.Retry) > 0
}
