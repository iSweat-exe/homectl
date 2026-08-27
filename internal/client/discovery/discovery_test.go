package discovery_test

import (
	"context"
	"testing"
	"time"

	clientdiscovery "homectl/internal/client/discovery"
	daemondiscovery "homectl/internal/daemon/discovery"
)

// TestLoopbackDiscovery announces a daemon over mDNS and confirms the
// client's browser sees it on the same machine. It exercises the real
// announce/browse path (Couche 1's discovery.Announce against Couche 2's
// discovery.Start) rather than mocking either side; it does not cross a
// Docker network boundary, so it only proves mDNS works on a single host —
// real LAN discovery across physical machines still needs manual
// verification.
func TestLoopbackDiscovery(t *testing.T) {
	const grpcPort = 47332 // distinct from config.DefaultGRPCPort to avoid clashing with a running daemon

	server, err := daemondiscovery.Announce(grpcPort)
	if err != nil {
		t.Skipf("announce mDNS (environment may lack multicast support): %v", err)
	}
	defer server.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	browser, err := clientdiscovery.Start(ctx)
	if err != nil {
		t.Skipf("start mDNS browser (environment may lack multicast support): %v", err)
	}
	defer browser.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, found := range browser.Snapshot() {
			if found.Port == grpcPort {
				return // found it, test passes
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("did not observe announced daemon (port %d) via mDNS browse within timeout; snapshot: %+v", grpcPort, browser.Snapshot())
}
