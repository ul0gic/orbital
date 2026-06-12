package events

import (
	"sync"
	"testing"
	"time"
)

func recv(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}

func TestPublishDeliversToSubscriber(t *testing.T) {
	b := NewBus()
	stream, cancel := b.Subscribe()
	defer cancel()

	b.Publish(Event{Type: Ready, File: "url"})
	got := recv(t, stream)
	if got.Type != Ready || got.File != "url" {
		t.Errorf("got %+v", got)
	}
}

func TestPublishFanOutToAllSubscribers(t *testing.T) {
	b := NewBus()
	s1, c1 := b.Subscribe()
	s2, c2 := b.Subscribe()
	defer c1()
	defer c2()

	b.Publish(Event{Type: Visitor})
	if recv(t, s1).Type != Visitor {
		t.Error("subscriber 1 missed event")
	}
	if recv(t, s2).Type != Visitor {
		t.Error("subscriber 2 missed event")
	}
}

func TestCancelStopsDeliveryAndClosesChannel(t *testing.T) {
	b := NewBus()
	stream, cancel := b.Subscribe()
	cancel()

	if _, ok := <-stream; ok {
		t.Error("channel should be closed after cancel")
	}
	// Publishing after cancel must not panic on a closed channel.
	b.Publish(Event{Type: Error})
}

func TestCancelIsIdempotent(t *testing.T) {
	b := NewBus()
	_, cancel := b.Subscribe()
	cancel()
	cancel()
}

func TestCancelOneSubscriberLeavesOthers(t *testing.T) {
	b := NewBus()
	s1, c1 := b.Subscribe()
	s2, c2 := b.Subscribe()
	defer c2()

	c1()
	b.Publish(Event{Type: DownloadStart})
	if recv(t, s2).Type != DownloadStart {
		t.Error("surviving subscriber missed event after sibling cancel")
	}
	if _, ok := <-s1; ok {
		t.Error("cancelled subscriber channel still open")
	}
}

// A stalled subscriber (never reading) must not block Publish; events overflow
// the buffer and are dropped rather than deadlocking the publisher.
func TestPublishNeverBlocksOnStalledSubscriber(t *testing.T) {
	b := NewBus()
	stalled, cancelStalled := b.Subscribe()
	defer cancelStalled()
	active, cancelActive := b.Subscribe()
	defer cancelActive()
	_ = stalled

	done := make(chan struct{})
	go func() {
		for i := range subscriberBuffer * 4 {
			b.Publish(Event{Type: UploadStart, Size: int64(i)})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish deadlocked on a stalled subscriber")
	}

	// The active subscriber still receives at least the buffered prefix.
	if recv(t, active).Type != UploadStart {
		t.Error("active subscriber received nothing")
	}
}

func TestConcurrentPublishAndSubscribe(t *testing.T) {
	b := NewBus()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, cancel := b.Subscribe()
			defer cancel()
			b.Publish(Event{Type: Visitor})
			select {
			case <-s:
			case <-time.After(time.Second):
			}
		}()
	}
	wg.Wait()
}
