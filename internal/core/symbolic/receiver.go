package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_MSG_RECEIVER     = &AnyMessageReceiver{}
	ANY_MSG              = &Message{}
	ANY_SYNC_MSG_HANDLER = &SynchronousMessageHandler{}

	MSG_PROPNAMES = []string{"data"}

	_ = []MessageReceiver{(*Object)(nil)}
)

// An MessageReceiver represents a symbolic MessageReceiver.
type MessageReceiver interface {
	Value
	ReceiveMessage(Value) error
}

// An Message represents a symbolic Message.
type Message struct {
	UnassignablePropsMixin
	_ int
}

func (m *Message) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(MessageReceiver)

	return ok
}

func (m *Message) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%message")))
	return
}

func (m *Message) WidestOfType() Value {
	return ANY_MSG
}

func (m *Message) ReceiveMessage(Value) error {
	return nil
}

func (m *Message) Prop(name string) Value {
	switch name {
	case "data":
		return ANY
	}
	panic(FormatErrPropertyDoesNotExist(name, m))
}

func (m *Message) PropertyNames() []string {
	return MSG_PROPNAMES
}

// An AnyMessageReceiver represents a symbolic MessageReceiver we do not know the concrete type.
type AnyMessageReceiver struct {
	_ int
}

func (r *AnyMessageReceiver) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(MessageReceiver)

	return ok
}

func (r *AnyMessageReceiver) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%message-receiver")))
	return
}

func (r *AnyMessageReceiver) WidestOfType() Value {
	return ANY_MSG_RECEIVER
}

func (r *AnyMessageReceiver) ReceiveMessage(Value) error {
	return nil
}

// A SynchronousMessageHandler represents a symbolic SynchronousMessageHandler.
type SynchronousMessageHandler struct {
	UnassignablePropsMixin
	SerializableMixin
}

func NewMessageHandler() *SynchronousMessageHandler {
	return &SynchronousMessageHandler{}
}

func (l *SynchronousMessageHandler) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*SynchronousMessageHandler)

	return ok
}

func (l *SynchronousMessageHandler) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%reception-handler")))
	return
}

func (l *SynchronousMessageHandler) WidestOfType() Value {
	return ANY_SYNC_MSG_HANDLER
}

func (l *SynchronousMessageHandler) ReceiveMessage(Value) error {
	return nil
}

func (l *SynchronousMessageHandler) Prop(name string) Value {
	panic(FormatErrPropertyDoesNotExist(name, l))
}

func (m *SynchronousMessageHandler) PropertyNames() []string {
	return nil
}

func (m *SynchronousMessageHandler) WatcherElement() Value {
	return ANY
}

//

func (*Object) ReceiveMessage(Value) error {
	return nil
}
