package events

import "sync"

const subscriberBuffer = 64

type Bus struct {
	mu   sync.Mutex
	subs map[int]chan Event
	next int
}

func NewBus() *Bus {
	return &Bus{subs: make(map[int]chan Event)}
}

// Publish never blocks: a subscriber whose buffer is full drops the event
// rather than stalling the server, since the live log must never throttle I/O.
//
//nolint:gocritic // value receipt is the published contract: callers pass an Event literal and the bus never aliases caller memory.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a receive channel and a cancel func; cancel must be called
// to release the subscription and drain the channel.
func (b *Bus) Subscribe() (stream <-chan Event, cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, subscriberBuffer)
	b.subs[id] = ch
	cancel = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if existing, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(existing)
		}
	}
	return ch, cancel
}
