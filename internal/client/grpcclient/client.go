// Package grpcclient wraps a single mTLS gRPC connection to one homectl
// daemon, with trust pinned by certificate fingerprint (TOFU) rather than a
// certificate authority.
package grpcclient

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"homectl/internal/shared/crypto"
	"homectl/internal/shared/pb"
)

// Client is one connection to a daemon's gRPC/mTLS endpoint.
type Client struct {
	conn     *grpc.ClientConn
	verifier *crypto.PeerVerifier

	Pairing pb.PairingServiceClient
	Homectl pb.HomectlServiceClient
}

// Dial connects to addr ("host:port"), presenting identity's certificate.
// pinnedFingerprint may be empty to accept whatever certificate the server
// presents — used only to learn an unpaired server's fingerprint so it can
// be shown to the user for TOFU confirmation. Once a server is paired,
// callers must always pass its pinned fingerprint.
func Dial(_ context.Context, addr string, identity *crypto.Identity, pinnedFingerprint string) (*Client, error) {
	verifier := &crypto.PeerVerifier{PinnedFingerprint: pinnedFingerprint}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(verifier.TLSConfig(identity))),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	return &Client{
		conn:     conn,
		verifier: verifier,
		Pairing:  pb.NewPairingServiceClient(conn),
		Homectl:  pb.NewHomectlServiceClient(conn),
	}, nil
}

// ObservedFingerprint returns the fingerprint of the certificate the server
// actually presented during the TLS handshake (which only happens once the
// first RPC is made — grpc.NewClient connects lazily). This is the
// authoritative value to show the user for TOFU confirmation, not anything
// self-reported by an RPC response body.
func (c *Client) ObservedFingerprint() (string, error) {
	cert := c.verifier.LastCertificate()
	if cert == nil {
		return "", errors.New("no handshake completed yet")
	}
	return crypto.Fingerprint(cert), nil
}

// Close releases the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
