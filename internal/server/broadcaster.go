package server

import (
	"sync"
	"time"
)

const debounceDuration = 200 * time.Millisecond

// broadcaster is a thread-safe pub/sub hub that coalesces rapid publish signals
// within a debounce window before fanning out to all subscribers.
type broadcaster struct {
	mu          sync.Mutex
	subscribers map[chan struct{}]struct{}
	publishCh   chan struct{}
	done        chan struct{}
	wg          sync.WaitGroup
}

// newBroadcaster creates and starts a new broadcaster.
func newBroadcaster() *broadcaster {
	b := &broadcaster{
		subscribers: make(map[chan struct{}]struct{}),
		publishCh:   make(chan struct{}, 1),
		done:        make(chan struct{}),
	}
	b.wg.Add(1)
	go b.run()
	return b
}

// subscribe creates a buffered channel, registers it, and returns it to the caller.
func (b *broadcaster) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes and closes the given subscriber channel.
func (b *broadcaster) unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

// publish signals that a mutation occurred. Non-blocking; rapid calls are coalesced.
func (b *broadcaster) publish() {
	select {
	case b.publishCh <- struct{}{}:
	default:
	}
}

// stop terminates the background goroutine and waits for it to exit.
func (b *broadcaster) stop() {
	close(b.done)
	b.wg.Wait()
}

// run is the background goroutine that debounces publish signals and fans out.
func (b *broadcaster) run() {
	defer b.wg.Done()
	var timer *time.Timer
	var timerCh <-chan time.Time

	for {
		select {
		case <-b.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case <-b.publishCh:
			// Start or reset the debounce timer.
			if timer == nil {
				timer = time.NewTimer(debounceDuration)
				timerCh = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(debounceDuration)
			}
		case <-timerCh:
			timer = nil
			timerCh = nil
			b.fanOut()
		}
	}
}

// fanOut delivers a signal to all subscribers in a non-blocking manner.
// The mutex is held throughout so that unsubscribe cannot close a channel
// while fanOut is sending to it.
func (b *broadcaster) fanOut() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
