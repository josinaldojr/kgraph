package server

import "sync"

// Broadcaster fans out a "graph-updated" notification to every connected
// SSE client (see events.go's /events handler). It carries no data itself —
// just wakes subscribers, who then re-fetch whatever view they currently
// have open through the JSON API.
type Broadcaster struct {
	mu   sync.Mutex
	subs map[chan struct{}]bool
}

// NewBroadcaster returns an empty Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[chan struct{}]bool)}
}

// Subscribe registers a new subscriber and returns its notification
// channel plus a cancel func the caller must call (typically via defer)
// when it stops listening, to avoid leaking the subscription.
func (b *Broadcaster) Subscribe() (ch chan struct{}, cancel func()) {
	ch = make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[ch] = true
	b.mu.Unlock()

	cancel = func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
	return ch, cancel
}

// Notify wakes every current subscriber. A subscriber with an
// already-pending notification is left alone (coalesced) rather than
// blocked on or queued twice — a client only ever needs to know "something
// changed, re-fetch," not how many times.
func (b *Broadcaster) Notify() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
