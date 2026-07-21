package api

import (
	"sync"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

// liveScan buffers every progress event a running scan has emitted so far
// and fans it out to any number of SSE subscribers, including ones that
// connect after the scan already produced some events (the frontend POSTs
// /scan, gets a scanId back, and only *then* opens the EventSource — by
// which point a fast local scan may already be half done).
type liveScan struct {
	mu        sync.Mutex
	events    []clusterdoctor.ScanProgressEvent
	listeners map[chan clusterdoctor.ScanProgressEvent]bool
	done      bool
}

func newLiveScan() *liveScan {
	return &liveScan{listeners: map[chan clusterdoctor.ScanProgressEvent]bool{}}
}

// broadcast records ev and forwards it to every currently-subscribed
// listener. A listener whose buffer is full is skipped rather than blocking
// the scan — SSE replay covers it if it's just slow, not disconnected.
func (l *liveScan) broadcast(ev clusterdoctor.ScanProgressEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, ev)

	for ch := range l.listeners {
		select {
		case ch <- ev:
		default:
		}
	}
}

// finish marks the scan complete and closes every listener channel so their
// SSE handlers can return.
func (l *liveScan) finish() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.done = true

	for ch := range l.listeners {
		close(ch)
	}

	l.listeners = map[chan clusterdoctor.ScanProgressEvent]bool{}
}

// subscribe returns a channel of future events plus a replay of everything
// already broadcast. If the scan already finished, ch is nil and done is
// true — the caller should just send the replay and close the response.
func (l *liveScan) subscribe() (ch chan clusterdoctor.ScanProgressEvent, replay []clusterdoctor.ScanProgressEvent, done bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	replay = append([]clusterdoctor.ScanProgressEvent{}, l.events...)

	if l.done {
		return nil, replay, true
	}

	ch = make(chan clusterdoctor.ScanProgressEvent, 16) //nolint:mnd
	l.listeners[ch] = true

	return ch, replay, false
}

func (l *liveScan) unsubscribe(ch chan clusterdoctor.ScanProgressEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.listeners, ch)
}
