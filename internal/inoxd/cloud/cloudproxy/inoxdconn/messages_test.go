package inoxdconn

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestEncodingDecoding(t *testing.T) {
	RegisterTypesInGob()

	msg := Message{
		ULID:  ulid.Make(),
		Inner: Ack{},
	}
	encoded, err := EncodeMessage(msg)
	if !assert.NoError(t, err) {
		return
	}

	var decoded Message
	err = DecodeMessage(encoded, &decoded)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, msg, decoded)
}
