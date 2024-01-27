package http_ns

import (
	"time"

	"github.com/inoxlang/inox/internal/core"
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
		"id":      core.String(""),
		"data":    core.String(""),
		"event":   core.String(""),
		"retry":   core.String(""),
		"comment": core.String(""),
	}

	if len(e.ID) != 0 {
		valMap["id"] = core.String(e.ID)
	}
	if len(e.Data) != 0 {
		valMap["data"] = core.String(e.Data)
	}
	if len(e.Event) != 0 {
		valMap["event"] = core.String(e.Event)
	}
	if len(e.Retry) != 0 {
		valMap["retry"] = core.String(e.Retry)
	}
	if len(e.Comment) != 0 {
		valMap["comment"] = core.String(e.Comment)
	}

	return core.NewEvent(nil, core.NewRecordFromMap(valMap), core.DateTime(e.timestamp))
}

func (e *ServerSentEvent) HasContent() bool {
	return len(e.ID) > 0 || len(e.Data) > 0 || len(e.Event) > 0 || len(e.Retry) > 0
}
