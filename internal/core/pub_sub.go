package core

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrMutablePublicationData = errors.New("impossible to create a publication with mutable data")
)

type Subscriber interface {
	Value
	ReceivePublication(ctx *Context, pub *Publication)
}

type WatchableSubscriber interface {
	Subscriber
	Watchable
	OnPublication(ctx *Context, microtask PublicationCallbackMicrotask, config PublicationCallbackConfiguration) CallbackHandle
}

type PublicationCallbackMicrotask func(ctx *Context, pub *Publication)

type PublicationCallbackConfiguration struct {
	//Subscribers []Subscriber
	//Filter      Pattern
}

func Subscribe(ctx *Context, subscriber Subscriber, publisher Value, filter Pattern) error {
	//TODO: update subscription if already existing ?

	sub := &Subscription{
		publisher:    publisher,
		subscriber:   subscriber,
		creationDate: Date(time.Now()),
		filter:       filter,
	}

	return transientSubscriptions.addSubscription(ctx, sub)
}

func Publish(ctx *Context, publisher Value, data Value) error {
	if data.IsMutable() {
		return ErrMutablePublicationData
	}

	subs, ok, err := transientSubscriptions.getPublisherSubscriptions(ctx, publisher)

	if err != nil {
		return fmt.Errorf("failed to publish: failed to get publisher's subscriptions: %w", err)
	}

	if !ok { // no subscribers
		return nil
	}

	pub := &Publication{
		data:            data,
		publisher:       publisher,
		publicationDate: Date(time.Now()),
	}

	subs.ReceivePublications(ctx, pub)

	return nil
}

// A Publication is an package around an immutable piece of data sent by a publisher to its subscribers, Publication implements Value.
type Publication struct {
	//internalId      InternalPublicationId
	data            Value // immutable value
	publisher       Value
	publicationDate Date

	NoReprMixin
	NotClonableMixin
}

type InternalPublicationId int64

func (p *Publication) Data() Value {
	return p.data
}

func (p *Publication) Publisher() Value {
	return p.publisher
}

func (p *Publication) PublicationDate() Date {
	return p.publicationDate
}
