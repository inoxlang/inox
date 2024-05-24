package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_SUBSCRIBER   = &AnySubscriber{}
	ANY_PUBLICATION  = &Publication{}
	ANY_SUBSCRIPTION = &Subscription{}
	_                = []Subscriber{(*Object)(nil)}
)

// An Subscriber represents a symbolic Subscriber.
type Subscriber interface {
	Value
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

func (r *Publication) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Subscriber)

	return ok
}

func (r *Publication) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("publication")
	return
}

func (r *Publication) WidestOfType() Value {
	return ANY_PUBLICATION
}

func (r *Publication) ReceivePublication(Value) error {
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

func (r *Subscription) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Subscriber)

	return ok
}

func (r *Subscription) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("subscription")
	return
}

func (r *Subscription) WidestOfType() Value {
	return ANY_SUBSCRIPTION
}

// An AnySubscriber represents a symbolic Subscriber we do not know the concrete type.
type AnySubscriber struct {
	_ int
}

func (r *AnySubscriber) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Subscriber)

	return ok
}

func (r *AnySubscriber) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("subscriber")
	return
}

func (r *AnySubscriber) WidestOfType() Value {
	return ANY_SUBSCRIBER
}

func (r *AnySubscriber) ReceivePublication(Value) error {
	return nil
}

func (obj *Object) ReceivePublication(pub *Publication) error {
	return nil
}
