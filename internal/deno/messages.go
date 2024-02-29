package deno

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_RANDOM_ID_BYTE_COUNT = 16
	STOP_ALL_METHOD              = "stopAll"
	ALL_STOPPED_EVENT            = "allStopped"
	ALREADY_STOPPED_RESPONSE     = `"alreadyStopped"` //json encoded

	REQUEST_KIND  = "request"
	RESPONSE_KIND = "response"
	EVENT_KIND    = "response"
)

type message struct {
	Id      string          `json:"id,omitempty"` //not set for event.
	Kind    string          `json:"kind"`
	Method  string          `json:"method"` //method or event name. Not set for responses.
	Payload json.RawMessage `json:"payload,omitempty"`
}

func makeRequestMessage(method string, payload any) message {
	bytes := make([]byte, DEFAULT_RANDOM_ID_BYTE_COUNT)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}

	return message{
		Method:  method,
		Id:      hex.EncodeToString(bytes),
		Kind:    REQUEST_KIND,
		Payload: utils.Must(json.Marshal(payload)),
	}
}

func makeEventMessage(name string, payload any) message {
	return message{
		Method:  name,
		Kind:    EVENT_KIND,
		Payload: utils.Must(json.Marshal(payload)),
	}
}

func makeResponseMessage(id string, payload json.RawMessage) message {
	return message{
		Id:      id,
		Kind:    RESPONSE_KIND,
		Payload: payload,
	}
}

func encodeMessage(message message) []byte {
	return utils.Must(json.Marshal(message))
}
