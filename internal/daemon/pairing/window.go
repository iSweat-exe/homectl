package pairing

import (
	"sync"
	"time"
)

// Window is the daemon's pairing window: a short, single-use period during
// which PairingService.Pair is allowed to pin a new client certificate.
// Closed by default; only opened when the operator explicitly signals the
// daemon (SIGUSR1), never automatically.
type Window struct {
	mu        sync.Mutex
	openUntil time.Time
}

// Open opens the pairing window for d.
func (w *Window) Open(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.openUntil = time.Now().Add(d)
}

// Close closes the pairing window immediately.
func (w *Window) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.openUntil = time.Time{}
}

// IsOpen reports whether the pairing window is currently open.
func (w *Window) IsOpen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Now().Before(w.openUntil)
}
