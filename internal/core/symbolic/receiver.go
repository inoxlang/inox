package internal

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

	_ = []MessageReceiver{&Object{}}
)

// An MessageReceiver represents a symbolic MessageReceiver.
type MessageReceiver interface {
	SymbolicValue
	ReceiveMessage(SymbolicValue) error
}

// An Message represents a symbolic Message.
type Message struct {
	UnassignablePropsMixin
	_ int
}

func (m *Message) Test(v SymbolicValue) bool {
	_, ok := v.(MessageReceiver)

	return ok
}

func (m *Message) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (m *Message) IsWidenable() bool {
	return false
}

func (m *Message) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%message")))
	return
}

func (m *Message) WidestOfType() SymbolicValue {
	return ANY_MSG
}

func (m *Message) ReceiveMessage(SymbolicValue) error {
	return nil
}

func (m *Message) Prop(name string) SymbolicValue {
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

func (r *AnyMessageReceiver) Test(v SymbolicValue) bool {
	_, ok := v.(MessageReceiver)

	return ok
}

func (r *AnyMessageReceiver) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *AnyMessageReceiver) IsWidenable() bool {
	return false
}

func (r *AnyMessageReceiver) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%message-receiver")))
	return
}

func (r *AnyMessageReceiver) WidestOfType() SymbolicValue {
	return ANY_MSG_RECEIVER
}

func (r *AnyMessageReceiver) ReceiveMessage(SymbolicValue) error {
	return nil
}

// A SynchronousMessageHandler represents a symbolic SynchronousMessageHandler.
type SynchronousMessageHandler struct {
	UnassignablePropsMixin
	_ int
}

func NewMessageHandler() *SynchronousMessageHandler {
	return &SynchronousMessageHandler{}
}

func (l *SynchronousMessageHandler) Test(v SymbolicValue) bool {
	_, ok := v.(*SynchronousMessageHandler)

	return ok
}

func (l *SynchronousMessageHandler) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (l *SynchronousMessageHandler) IsWidenable() bool {
	return false
}

func (l *SynchronousMessageHandler) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%reception-handler")))
	return
}

func (l *SynchronousMessageHandler) WidestOfType() SymbolicValue {
	return ANY_SYNC_MSG_HANDLER
}

func (l *SynchronousMessageHandler) ReceiveMessage(SymbolicValue) error {
	return nil
}

func (l *SynchronousMessageHandler) Prop(name string) SymbolicValue {
	panic(FormatErrPropertyDoesNotExist(name, l))
}

func (m *SynchronousMessageHandler) PropertyNames() []string {
	return nil
}

//

func (*Object) ReceiveMessage(SymbolicValue) error {
	return nil
}
