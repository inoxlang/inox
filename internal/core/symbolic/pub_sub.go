package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_SUBSCRIBER   = &AnySubscriber{}
	ANY_PUBLICATION  = &Publication{}
	ANY_SUBSCRIPTION = &Subscription{}
	_                = []Subscriber{&Object{}}
)

// An Subscriber represents a symbolic Subscriber.
type Subscriber interface {
	SymbolicValue
	ReceivePublication(*Publication) error
}

// An Publication represents a symbolic Publication.
type Publication struct {
	_ int
}

// add parameters
func NewPublication() *Publication {
	return &Publication{}
}

func (r *Publication) Test(v SymbolicValue) bool {
	_, ok := v.(Subscriber)

	return ok
}

func (r *Publication) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Publication) IsWidenable() bool {
	return false
}

func (r *Publication) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%publication")))
	return
}

func (r *Publication) WidestOfType() SymbolicValue {
	return ANY_PUBLICATION
}

func (r *Publication) ReceivePublication(SymbolicValue) error {
	return nil
}

// An Subscription represents a symbolic Subscription.
type Subscription struct {
	_ int
}

// add parameters
func NewSubscription() *Subscription {
	return &Subscription{}
}

func (r *Subscription) Test(v SymbolicValue) bool {
	_, ok := v.(Subscriber)

	return ok
}

func (r *Subscription) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Subscription) IsWidenable() bool {
	return false
}

func (r *Subscription) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%subscription")))
	return
}

func (r *Subscription) WidestOfType() SymbolicValue {
	return ANY_SUBSCRIPTION
}

// An AnySubscriber represents a symbolic Subscriber we do not know the concrete type.
type AnySubscriber struct {
	_ int
}

func (r *AnySubscriber) Test(v SymbolicValue) bool {
	_, ok := v.(Subscriber)

	return ok
}

func (r *AnySubscriber) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnySubscriber) IsWidenable() bool {
	return false
}

func (r *AnySubscriber) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%subscriber")))
	return
}

func (r *AnySubscriber) WidestOfType() SymbolicValue {
	return &AnySubscriber{}
}

func (r *AnySubscriber) ReceivePublication(SymbolicValue) error {
	return nil
}

func (obj *Object) ReceivePublication(pub *Publication) error {
	return nil
}
