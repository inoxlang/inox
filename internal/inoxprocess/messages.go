package inoxprocess

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

var (
	areMsgTypesRegistered atomic.Bool

	ErrAlreadyExecuting = errors.New("already executing")
)

type Message struct {
	ULID  ulid.ULID
	Inner any
}

func RegisterTypesInGob() {
	if !areMsgTypesRegistered.CompareAndSwap(false, true) {
		return
	}
	gob.Register(Message{})
	gob.Register(ulid.ULID{})

	gob.Register(LaunchApplicationRequest{})
	gob.Register(LaunchAppResponse{})

	gob.Register(StopAllRequest{})
	gob.Register(StopAllResponse{})
	gob.Register(AllStoppedEvent{})

	gob.Register(AckMsg{})
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

type LaunchApplicationRequest struct {
	AppDir string
}

type LaunchAppResponse struct {
	Request ulid.ULID
	Error   error //can be empty
}

type StopAllRequest struct {
}

type StopAllResponse struct {
	AlreadyStopped bool
}

type AllStoppedEvent struct {
}

type AckMsg struct {
	AcknowledgedMessage ulid.ULID
}
