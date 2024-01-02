package core

import (
	"errors"
	"fmt"
	"sync"
)

var (
	transientSubscriptions subscriptionStore

	ErrPublisherNotUniquelyIdentifiable  = errors.New("publisher not uniquely identifiable")
	ErrSubscriberNotUniquelyIdentifiable = errors.New("subscriber not uniquely identifiable")
)

// A Subscription holds metadata about the subscription of a Subscriber to a Publisher.
type Subscription struct {
	lock         sync.Mutex
	publisher    Value
	subscriber   Subscriber
	creationDate DateTime
	filter       Pattern
}

type Subscriptions struct {
	lock          sync.Mutex
	subscriptions []*Subscription
}

func (s *Subscriptions) ReceivePublications(ctx *Context, pub *Publication) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, sub := range s.subscriptions {
		if sub.filter.Test(ctx, pub) {
			sub.subscriber.ReceivePublication(ctx, pub)
		}
	}
}

type subscriptionStore struct {
	lock                      sync.Mutex
	publisherToSubscriptions  map[TransientID]*Subscriptions
	subscriberToSubscriptions map[TransientID]*Subscriptions
}

func (s *subscriptionStore) getSubscriberSubscriptions(ctx *Context, subscriber Subscriber) (*Subscriptions, bool, error) {
	subscriberFastId, ok := TransientIdOf(subscriber)
	if !ok {
		return nil, false, fmt.Errorf("failed to get subscriptions: %w", ErrPublisherNotUniquelyIdentifiable)
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	subs := s.subscriberToSubscriptions[subscriberFastId]
	if subs == nil {
		return nil, false, nil
	}

	return subs, true, nil
}

func (s *subscriptionStore) getPublisherSubscriptions(ctx *Context, publisher Value) (*Subscriptions, bool, error) {
	publisherFastId, ok := TransientIdOf(publisher)
	if !ok {
		return nil, false, fmt.Errorf("failed to get subscriptions: %w", ErrSubscriberNotUniquelyIdentifiable)
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	subs := s.publisherToSubscriptions[publisherFastId]
	if subs == nil {
		return nil, false, nil
	}

	return subs, true, nil
}

func (s *subscriptionStore) addSubscription(ctx *Context, sub *Subscription) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	publisherFastId, ok := TransientIdOf(sub.publisher)
	if !ok {
		return fmt.Errorf("failed to add subscription: %w", ErrPublisherNotUniquelyIdentifiable)
	}

	subscriberFastId, ok := TransientIdOf(sub.subscriber)
	if !ok {
		return fmt.Errorf("failed to add subscription: %w", ErrSubscriberNotUniquelyIdentifiable)
	}

	{
		subs := s.publisherToSubscriptions[publisherFastId]
		if subs == nil {
			subs = &Subscriptions{}
			s.publisherToSubscriptions[publisherFastId] = subs
		}
		subs.lock.Lock()
		subs.subscriptions = append(subs.subscriptions, sub)
		subs.lock.Unlock()
	}

	{
		subs := s.subscriberToSubscriptions[subscriberFastId]
		if subs == nil {
			subs = &Subscriptions{}
			s.subscriberToSubscriptions[subscriberFastId] = subs
		}
		subs.lock.Lock()
		subs.subscriptions = append(subs.subscriptions, sub)
		subs.lock.Unlock()
	}

	return nil
}
