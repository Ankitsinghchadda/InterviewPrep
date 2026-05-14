package submissions

import (
	"sync"
)

// Event is a single message published for a submission's processing pipeline.
// Kind values: "transcript" | "review_token" | "review_done" | "error".
type Event struct {
	Kind string `json:"kind"`
	Data string `json:"data"`
}

// Broker is an in-memory pub/sub keyed by submission ID. It powers SSE
// streaming for review feedback. Publish is non-blocking: events drop if a
// subscriber isn't reading fast enough (review tokens are best-effort UX).
//
// Single-process only. For multi-instance deployment swap this for Redis pub/sub.
type Broker struct {
	mu   sync.RWMutex
	subs map[string][]chan Event
}

func NewBroker() *Broker {
	return &Broker{subs: map[string][]chan Event{}}
}

// Subscribe returns a buffered channel of events for the given submission ID
// plus an unsubscribe function the caller MUST invoke (typically via defer).
func (b *Broker) Subscribe(submissionID string) (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs[submissionID] = append(b.subs[submissionID], ch)
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.subs[submissionID]
		for i, c := range list {
			if c == ch {
				b.subs[submissionID] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(b.subs[submissionID]) == 0 {
			delete(b.subs, submissionID)
		}
		// Drain + close so the consumer's range exits.
		safeClose(ch)
	}
	return ch, unsubscribe
}

// Publish sends an event to all current subscribers. Slow consumers drop
// events instead of blocking the producer goroutine.
func (b *Broker) Publish(submissionID string, ev Event) {
	b.mu.RLock()
	subs := append([]chan Event{}, b.subs[submissionID]...)
	b.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// channel full — drop this event for this subscriber.
		}
	}
}

// Close fans out a final event then closes every subscriber channel for a
// submission. The handler treats this as "no more updates coming". Late
// subscribers should fall back to reading the DB directly.
func (b *Broker) Close(submissionID string) {
	b.mu.Lock()
	subs := b.subs[submissionID]
	delete(b.subs, submissionID)
	b.mu.Unlock()
	for _, ch := range subs {
		safeClose(ch)
	}
}

// safeClose recovers from "close of closed channel" — Subscribe's unsubscribe
// and Close race on the same channel and either may win.
func safeClose(ch chan Event) {
	defer func() { _ = recover() }()
	close(ch)
}
