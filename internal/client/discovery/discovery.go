// Package discovery continuously browses for homectl daemons on the local
// network via mDNS, so the client's server list can show both paired and
// not-yet-paired daemons without any manual configuration.
package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/grandcat/zeroconf"

	"homectl/internal/shared/config"
)

// Found is one daemon instance currently visible on the network.
type Found struct {
	InstanceName string
	Host         string
	Port         int
}

// Browser watches for _homectl._tcp instances in the background and keeps a
// snapshot of what's currently seen. Entries are never actively expired
// (acceptable for a small, mostly-static LAN of servers); Stop releases the
// underlying resolver.
type Browser struct {
	mu      sync.RWMutex
	entries map[string]Found

	cancel context.CancelFunc
}

// Start begins browsing in the background, deriving its lifetime from ctx.
// Call Stop when done to release the resolver.
func Start(ctx context.Context) (*Browser, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("create mdns resolver: %w", err)
	}

	browseCtx, cancel := context.WithCancel(ctx)
	b := &Browser{entries: map[string]Found{}, cancel: cancel}

	results := make(chan *zeroconf.ServiceEntry)
	go b.consume(results)

	if err := resolver.Browse(browseCtx, config.MDNSServiceType, config.MDNSDomain, results); err != nil {
		cancel()
		return nil, fmt.Errorf("browse mdns: %w", err)
	}
	return b, nil
}

func (b *Browser) consume(results <-chan *zeroconf.ServiceEntry) {
	for e := range results {
		host := ""
		switch {
		case len(e.AddrIPv4) > 0:
			host = e.AddrIPv4[0].String()
		case len(e.AddrIPv6) > 0:
			host = e.AddrIPv6[0].String()
		default:
			continue
		}

		b.mu.Lock()
		b.entries[e.Instance] = Found{InstanceName: e.Instance, Host: host, Port: e.Port}
		b.mu.Unlock()
	}
}

// Snapshot returns every daemon instance currently known.
func (b *Browser) Snapshot() []Found {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]Found, 0, len(b.entries))
	for _, f := range b.entries {
		out = append(out, f)
	}
	return out
}

// Stop stops the background browse.
func (b *Browser) Stop() {
	b.cancel()
}
