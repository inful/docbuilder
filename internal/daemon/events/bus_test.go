package events

import (
	"context"
	"testing"
	"time"

	ferrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"github.com/stretchr/testify/require"
)

type testEvent struct {
	Value int
}

type testEventer interface {
	EventValue() int
}

func (e testEvent) EventValue() int { return e.Value }

func TestBus_PublishSubscribe(t *testing.T) {
	b := NewBus()
	defer b.Close()

	ch, unsubscribe := Subscribe[testEvent](b, 1)
	defer unsubscribe()

	require.NoError(t, b.Publish(context.Background(), testEvent{Value: 123}))

	select {
	case got := <-ch:
		require.Equal(t, 123, got.Value)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for event")
	}
}

func TestBus_InterfaceSubscriptionReceivesConcreteEvents(t *testing.T) {
	b := NewBus()
	defer b.Close()

	ch, unsubscribe := Subscribe[testEventer](b, 1)
	defer unsubscribe()

	require.NoError(t, b.Publish(context.Background(), testEvent{Value: 7}))

	select {
	case got := <-ch:
		require.Equal(t, 7, got.EventValue())
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for event")
	}
}

func TestBus_PublishBackpressure(t *testing.T) {
	b := NewBus()
	defer b.Close()

	_, unsubscribe := Subscribe[testEvent](b, 0) // unbuffered; no receiver => blocks
	defer unsubscribe()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := b.Publish(ctx, testEvent{Value: 1})
	require.Error(t, err)

	classified, ok := ferrors.AsClassified(err)
	require.True(t, ok)
	require.Equal(t, ferrors.CategoryRuntime, classified.Category())
}

func TestBus_Close(t *testing.T) {
	b := NewBus()

	ch, _ := Subscribe[testEvent](b, 1)
	b.Close()

	// Channel must be closed on bus close.
	_, ok := <-ch
	require.False(t, ok)

	err := b.Publish(context.Background(), testEvent{Value: 1})
	require.Error(t, err)
}
