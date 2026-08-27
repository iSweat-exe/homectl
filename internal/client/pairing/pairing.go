// Package pairing implements the client side of the TOFU pairing flow:
// dial a newly discovered daemon to learn its certificate fingerprint
// (Propose), then, once the user has visually confirmed it, dial again
// pinned to that exact fingerprint and complete the pairing handshake
// (Confirm). Re-dialing between the two steps ensures the connection used
// to send Pair is provably the same one whose fingerprint the user saw.
package pairing

import (
	"context"
	"fmt"

	"homectl/internal/client/grpcclient"
	"homectl/internal/shared/crypto"
	"homectl/internal/shared/pb"
)

// Proposed is a daemon reachable at Address, whose certificate fingerprint
// is Fingerprint — awaiting the user's manual confirmation before Confirm
// is called.
type Proposed struct {
	Address     string
	Fingerprint string
}

// Propose connects to addr without pinning any fingerprint and returns
// whatever certificate fingerprint the daemon presents.
func Propose(ctx context.Context, identity *crypto.Identity, addr string) (*Proposed, error) {
	client, err := grpcclient.Dial(ctx, addr, identity, "")
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if _, err := client.Pairing.GetFingerprint(ctx, &pb.FingerprintRequest{}); err != nil {
		return nil, fmt.Errorf("get fingerprint: %w", err)
	}

	fp, err := client.ObservedFingerprint()
	if err != nil {
		return nil, err
	}
	return &Proposed{Address: addr, Fingerprint: fp}, nil
}

// Confirm re-dials addr pinned to expectedFingerprint and calls Pair. The
// daemon's pairing window must already be open (SIGUSR1, or `homectl-daemon
// pair`) on its side, or this fails with an error from the daemon.
func Confirm(ctx context.Context, identity *crypto.Identity, addr, expectedFingerprint, clientName string) error {
	client, err := grpcclient.Dial(ctx, addr, identity, expectedFingerprint)
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.Pairing.Pair(ctx, &pb.PairRequest{ClientName: clientName})
	if err != nil {
		return fmt.Errorf("pair: %w", err)
	}
	if !resp.GetAccepted() {
		return fmt.Errorf("pairing rejected: %s", resp.GetMessage())
	}
	return nil
}
