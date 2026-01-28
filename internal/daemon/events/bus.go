package events

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"

	ferrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// Bus is a small, typed, in-process event bus intended for daemon orchestration.
//
// Design goals:
//   - Typed subscriptions (via generics)
//   - Bounded buffering/backpressure (Publish blocks until delivered or ctx canceled)
//   - Clean shutdown (Close closes all subscription channels)
//
// This is intentionally not durable and is not a replacement for internal/eventstore.
// It is used for control-flow events inside the single daemon process.
type Bus struct {
	mu        sync.RWMutex
	subs      map[reflect.Type]map[uint64]*subscriber
	nextID    atomic.Uint64
	isClosed  atomic.Bool
	closeOnce sync.Once
}

type subscriber struct {
	send  func(ctx context.Context, evt any) error
	close func()
}

func NewBus() *Bus {
	return &Bus{
		subs: make(map[reflect.Type]map[uint64]*subscriber),
	}
}

// Subscribe registers a subscription for events of type T.
//
// If T is an interface, published events whose concrete type implements T will be delivered.
// For concrete T, events are delivered only when the concrete type matches exactly.
func Subscribe[T any](b *Bus, buffer int) (<-chan T, func()) {
	eventType := reflect.TypeFor[T]()
	ch := make(chan T, buffer)

	if b.isClosed.Load() {
		close(ch)
		return ch, func() {}
	}

	id := b.nextID.Add(1)

	var closeOnce sync.Once
	closeChannel := func() {
		closeOnce.Do(func() {
			close(ch)
		})
	}

	var unsubOnce sync.Once
	unsubscribe := func() {
		unsubOnce.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if typeSubs, ok := b.subs[eventType]; ok {
				delete(typeSubs, id)
				if len(typeSubs) == 0 {
					delete(b.subs, eventType)
				}
			}

			closeChannel()
		})
	}

	sub := &subscriber{
		send: func(ctx context.Context, evt any) error {
			v, ok := evt.(T)
			if !ok {
				return ferrors.InternalError("event type mismatch").
					WithContext("expected", eventType.String()).
					WithContext("actual", reflect.TypeOf(evt).String()).
					Build()
			}

			select {
			case ch <- v:
				return nil
			case <-ctx.Done():
				return ferrors.WrapError(ctx.Err(), ferrors.CategoryRuntime, "event publish canceled").
					WithContext("event_type", eventType.String()).
					Build()
			}
		},
		close: func() {
			closeChannel()
		},
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isClosed.Load() {
		closeChannel()
		return ch, func() {}
	}

	if b.subs[eventType] == nil {
		b.subs[eventType] = make(map[uint64]*subscriber)
	}
	b.subs[eventType][id] = sub

	return ch, unsubscribe
}

// SubscriberCount returns the number of active subscribers for events of type T.
//
// This is primarily intended for tests and diagnostics.
func SubscriberCount[T any](b *Bus) int {
	if b == nil {
		return 0
	}

	eventType := reflect.TypeFor[T]()

	b.mu.RLock()
	defer b.mu.RUnlock()

	if typeSubs, ok := b.subs[eventType]; ok {
		return len(typeSubs)
	}
	return 0
}

// Publish delivers an event to all matching subscribers.
//
// Backpressure: Publish blocks until each subscriber has accepted the event, or the
// provided context is canceled.
func (b *Bus) Publish(ctx context.Context, evt any) error {
	if evt == nil {
		return ferrors.ValidationError("event cannot be nil").Build()
	}
	if ctx == nil {
		return ferrors.ValidationError("context cannot be nil").Build()
	}
	if b.isClosed.Load() {
		return ferrors.DaemonError("event bus is closed").Build()
	}

	evtType := reflect.TypeOf(evt)

	b.mu.RLock()
	var targets []*subscriber
	for subType, typeSubs := range b.subs {
		match := subType == evtType
		if !match && subType.Kind() == reflect.Interface {
			match = evtType.Implements(subType)
		}
		if !match {
			continue
		}
		for _, s := range typeSubs {
			targets = append(targets, s)
		}
	}
	b.mu.RUnlock()

	for _, s := range targets {
		if err := s.send(ctx, evt); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the bus and all subscription channels.
func (b *Bus) Close() {
	b.closeOnce.Do(func() {
		b.isClosed.Store(true)

		b.mu.Lock()
		estimated := 0
		for _, typeSubs := range b.subs {
			estimated += len(typeSubs)
		}

		toClose := make([]*subscriber, 0, estimated)
		for _, typeSubs := range b.subs {
			for _, s := range typeSubs {
				toClose = append(toClose, s)
			}
		}
		b.subs = make(map[reflect.Type]map[uint64]*subscriber)
		b.mu.Unlock()

		for _, s := range toClose {
			s.close()
		}
	})
}
