package crypto

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// ServerTLSConfig builds the daemon-side TLS config: it presents identity's
// certificate and requires the peer to present one too (RequireAnyClientCert
// with no configured ClientCAs makes Go's stdlib accept any client
// certificate without chain validation — trust is decided afterwards, per
// RPC, by the auth interceptor checking the peer's fingerprint against the
// pinned trusted-clients store).
func ServerTLSConfig(identity *Identity) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{identity.TLS},
		ClientAuth:   tls.RequireAnyClientCert,
		MinVersion:   tls.VersionTLS13,
	}
}

// PeerVerifier builds client-side TLS configs and captures whatever
// certificate the remote party presents, so callers can show its
// fingerprint for TOFU confirmation. When PinnedFingerprint is non-empty,
// the handshake is rejected unless the presented certificate matches it.
type PeerVerifier struct {
	PinnedFingerprint string

	mu       sync.Mutex
	lastCert *x509.Certificate
}

// TLSConfig returns a client-side tls.Config that presents identity's
// certificate (for mTLS) and defers server certificate trust decisions to
// the verifier instead of the standard CA chain (there is no CA).
func (v *PeerVerifier) TLSConfig(identity *Identity) *tls.Config {
	return &tls.Config{
		Certificates:          []tls.Certificate{identity.TLS},
		InsecureSkipVerify:    true, //nolint:gosec // trust is enforced in VerifyPeerCertificate below, not the default chain
		MinVersion:            tls.VersionTLS13,
		VerifyPeerCertificate: v.verify,
	}
}

func (v *PeerVerifier) verify(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return errors.New("no certificate presented by peer")
	}
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("parse peer certificate: %w", err)
	}

	v.mu.Lock()
	v.lastCert = cert
	v.mu.Unlock()

	if v.PinnedFingerprint != "" {
		if fp := Fingerprint(cert); fp != v.PinnedFingerprint {
			return fmt.Errorf("peer fingerprint mismatch: pinned %s, presented %s", v.PinnedFingerprint, fp)
		}
	}
	return nil
}

// LastCertificate returns the most recently observed peer certificate, or
// nil if no handshake has completed yet.
func (v *PeerVerifier) LastCertificate() *x509.Certificate {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.lastCert
}

// PeerCertificateFromContext extracts the leaf TLS certificate presented by
// the caller of a gRPC handler, for use by the daemon's auth interceptor.
func PeerCertificateFromContext(ctx context.Context) (*x509.Certificate, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("no peer information in context")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, errors.New("connection is not TLS-secured")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return nil, errors.New("peer did not present a certificate")
	}
	return tlsInfo.State.PeerCertificates[0], nil
}
