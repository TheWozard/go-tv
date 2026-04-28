package ui

import "sync"

// Broadcaster fans out pre-rendered HTML strings to all connected SSE clients.
// Call Broadcast after rendering a state fragment; each registered channel
// receives the string and the SSE handler writes it to its client.
type Broadcaster struct {
	mu   sync.Mutex
	subs map[uint64]chan string
	next uint64
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[uint64]chan string)}
}

func (b *Broadcaster) Subscribe() (uint64, <-chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan string, 4)
	b.subs[id] = ch
	return id, ch
}

func (b *Broadcaster) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		close(ch)
		delete(b.subs, id)
	}
}

// Broadcast sends html to all subscribers, dropping slow clients.
func (b *Broadcaster) Broadcast(html string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- html:
		default:
		}
	}
}
