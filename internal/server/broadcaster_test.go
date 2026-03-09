package server

import (
	"testing"
	"time"
)

const testTimeout = 2 * time.Second

func waitForSignal(t *testing.T, ch chan struct{}, timeout time.Duration) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TestBroadcaster_SingleSubscriberReceivesSignal(t *testing.T) {
	b := newBroadcaster()
	defer b.stop()

	ch := b.subscribe()
	b.publish()

	if !waitForSignal(t, ch, testTimeout) {
		t.Fatal("subscriber did not receive signal within timeout")
	}
}

func TestBroadcaster_MultipleSubscribersAllReceive(t *testing.T) {
	b := newBroadcaster()
	defer b.stop()

	ch1 := b.subscribe()
	ch2 := b.subscribe()
	ch3 := b.subscribe()

	b.publish()

	for i, ch := range []chan struct{}{ch1, ch2, ch3} {
		if !waitForSignal(t, ch, testTimeout) {
			t.Fatalf("subscriber %d did not receive signal within timeout", i+1)
		}
	}
}

func TestBroadcaster_UnsubscribedClientReceivesNothing(t *testing.T) {
	b := newBroadcaster()
	defer b.stop()

	ch := b.subscribe()
	b.unsubscribe(ch)

	b.publish()

	// Give time for any (erroneous) delivery to occur.
	time.Sleep(debounceDuration + 100*time.Millisecond)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("unsubscribed channel should not receive a signal")
		}
		// channel was closed — that's fine, it's just the close from unsubscribe
	default:
		// no signal, correct
	}
}

func TestBroadcaster_RapidPublishesCoalesced(t *testing.T) {
	b := newBroadcaster()
	defer b.stop()

	ch := b.subscribe()

	// Publish many times in rapid succession.
	for i := 0; i < 20; i++ {
		b.publish()
	}

	// We expect exactly one delivery.
	if !waitForSignal(t, ch, testTimeout) {
		t.Fatal("subscriber did not receive any signal")
	}

	// Wait well past the debounce window to ensure no second delivery.
	time.Sleep(debounceDuration + 100*time.Millisecond)

	select {
	case <-ch:
		t.Fatal("subscriber received more than one signal for coalesced publishes")
	default:
		// correct: only one delivery
	}
}

func TestBroadcaster_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	b := newBroadcaster()
	defer b.stop()

	fast := b.subscribe()
	slow := b.subscribe()

	// Pre-fill the slow subscriber's buffer so it can't receive.
	slow <- struct{}{}

	b.publish()

	// Fast subscriber should still receive despite slow being full.
	if !waitForSignal(t, fast, testTimeout) {
		t.Fatal("fast subscriber was blocked by slow subscriber")
	}
}

func TestBroadcaster_NoSubscribers_NoPanic(t *testing.T) {
	b := newBroadcaster()
	defer b.stop()

	// Should not panic or deadlock with no subscribers.
	b.publish()
	time.Sleep(debounceDuration + 100*time.Millisecond)
}

func TestBroadcaster_ConcurrentUnsubscribeNoPanic(t *testing.T) {
	// Regression test for the race where fanOut copies a channel under the lock
	// and then unsubscribe closes it before fanOut sends to it.
	for range 50 {
		b := newBroadcaster()
		ch := b.subscribe()

		b.publish()
		// Unsubscribe concurrently with fanOut delivery.
		go b.unsubscribe(ch)

		time.Sleep(debounceDuration + 50*time.Millisecond)
		b.stop()
	}
}

func TestBroadcaster_StopsCleanly(t *testing.T) {
	b := newBroadcaster()
	b.subscribe()
	b.publish()

	done := make(chan struct{})
	go func() {
		b.stop()
		close(done)
	}()

	select {
	case <-done:
		// stopped cleanly
	case <-time.After(testTimeout):
		t.Fatal("broadcaster did not stop within timeout")
	}
}
