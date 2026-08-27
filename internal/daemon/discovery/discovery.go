// Package discovery announces the daemon's presence over mDNS so clients
// can find it on the local network without any manual configuration.
package discovery

import (
	"fmt"
	"os"

	"github.com/grandcat/zeroconf"

	"homectl/internal/shared/config"
)

const protocolVersion = "1"

// Announce registers the daemon under config.MDNSServiceType, advertising
// grpcPort and the protocol version via a TXT record. Call Shutdown on the
// returned server when the daemon stops.
func Announce(grpcPort int) (*zeroconf.Server, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "homectl-daemon"
	}

	server, err := zeroconf.Register(
		hostname,
		config.MDNSServiceType,
		config.MDNSDomain,
		grpcPort,
		[]string{"version=" + protocolVersion},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("announce mDNS service: %w", err)
	}
	return server, nil
}
