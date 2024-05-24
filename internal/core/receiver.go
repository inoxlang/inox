package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

var (
	_                     = []MessageReceiver{(*Object)(nil)}
	ErrMutableMessageData = errors.New("impossible to create a Message with mutable data")

	MSG_PROPNAMES = []string{"data"}
)

func init() {
	RegisterSymbolicGoFunction(SendVal, func(ctx *symbolic.Context, v symbolic.Value, r symbolic.MessageReceiver, s symbolic.Value) *symbolic.Error {
		return nil
	})
}

type MessageReceiver interface {
	SystemGraphNodeValue
	ReceiveMessage(ctx *Context, msg Message) error
}

func SendVal(ctx *Context, value Value, r MessageReceiver, sender Value) error {
	r.AddSystemGraphEvent(ctx, "reception of a message")
	return r.ReceiveMessage(ctx, Message{data: value, sender: sender})
}

// A Message is an immutable package around an immutable piece of data sent by a sender to a MessageReceiver, Message implements Value.
type Message struct {
	data         Value // immutable value
	sender       Value
	sendindgDate DateTime
}

func (m Message) Data() Value {
	return m.data
}

func NewMessage(data Value, sender Value) Message {
	if data.IsMutable() {
		panic(ErrMutableMessageData)
	}
	return Message{data: data, sender: sender, sendindgDate: DateTime(time.Now())}
}

func (m Message) Prop(ctx *Context, name string) Value {
	switch name {
	case "data":
		return m.data
	}
	panic(FormatErrPropertyDoesNotExist(name, m))
}

func (Message) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (Message) PropertyNames(ctx *Context) []string {
	return MSG_PROPNAMES
}

type SynchronousMessageHandler struct {
	pattern Pattern
	handler *InoxFunction

	mutationFieldsLock sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	watchers           *ValueWatchers
	mutationCallbacks  *MutationCallbacks
	watchingDepth      WatchingDepth
}

func NewSynchronousMessageHandler(ctx *Context, fn *InoxFunction, pattern Pattern) *SynchronousMessageHandler {
	if ok, expl := fn.IsSharable(fn.originState); !ok {
		panic(fmt.Errorf("only sharable functions are allowed: %s", expl))
	}
	fn.Share(ctx.MustGetClosestState())

	return &SynchronousMessageHandler{
		handler: fn,
		pattern: pattern,
	}
}

func (h *SynchronousMessageHandler) Pattern() Pattern {
	return h.pattern
}

func (h *SynchronousMessageHandler) Prop(ctx *Context, name string) Value {
	switch name {
	}
	panic(FormatErrPropertyDoesNotExist(name, h))
}

func (*SynchronousMessageHandler) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*SynchronousMessageHandler) PropertyNames(ctx *Context) []string {
	return nil
}

type SynchronousMessageHandlers struct {
	list []*SynchronousMessageHandler
}

func NewSynchronousMessageHandlers(handlers ...*SynchronousMessageHandler) *SynchronousMessageHandlers {
	return &SynchronousMessageHandlers{list: handlers}
}

func (handlers *SynchronousMessageHandlers) CallHandlers(ctx *Context, msg Message, self Value) error {
	if handlers == nil {
		return nil
	}
	for _, h := range handlers.list {
		if h.Pattern().Test(ctx, msg.Data()) {
			_, err := h.handler.Call(ctx.MustGetClosestState(), self, []Value{msg.Data()}, nil)
			if err != nil {
				return fmt.Errorf("one of the message handler returned an error: %w", err)
			}
		}
	}
	return nil
}

// receivers

func (obj *Object) ReceiveMessage(ctx *Context, msg Message) error {
	state := ctx.MustGetClosestState()
	obj._lock(state)
	defer obj._unlock(state)

	if !obj.hasAdditionalFields() {
		return nil
	}

	if err := obj.messageHandlers.CallHandlers(ctx, msg, obj); err != nil {
		return err
	}

	obj.watchers.InformAboutAsync(ctx, msg, ShallowWatching, false)
	return nil
}
