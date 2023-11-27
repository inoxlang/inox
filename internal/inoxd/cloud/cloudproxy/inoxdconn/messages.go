package inoxdconn

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

var areTypesRegistered atomic.Bool

type Message struct {
	ULID  ulid.ULID
	Inner any
}

func RegisterTypesInGob() {
	if !areTypesRegistered.CompareAndSwap(false, true) {
		return
	}
	gob.Register(Message{})
	gob.Register(ulid.ULID{})
	gob.Register(Hello{})
	gob.Register(Ack{})
}

func EncodeMessage(msg Message) ([]byte, error) {
	if msg.ULID == (ulid.ULID{}) {
		return nil, errors.New("missing ULID")
	}

	buf := bytes.NewBuffer(nil)
	var encoder = gob.NewEncoder(buf)
	err := encoder.Encode(msg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func MustEncodeMessage(msg Message) []byte {
	return utils.Must(EncodeMessage(msg))
}

func DecodeMessage(p []byte, ptr any) error {
	var decoder = gob.NewDecoder(bytes.NewReader(p))
	return decoder.Decode(ptr)
}

type Hello struct {
}

type Ack struct {
	AcknowledgedMessage ulid.ULID
}
