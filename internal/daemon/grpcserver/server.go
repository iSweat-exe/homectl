// Package grpcserver wires the daemon's PairingService and HomectlService
// implementations onto a gRPC server secured with mTLS, gating every
// HomectlService call on the caller having already been paired.
package grpcserver

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"homectl/internal/daemon/pairing"
	"homectl/internal/shared/crypto"
	"homectl/internal/shared/pb"
)

// pairingServiceName is exempt from the pinned-client-certificate check:
// PairingService.Pair enforces the pairing window itself, and
// GetFingerprint returns nothing more sensitive than any TLS handshake
// with the daemon already reveals.
const pairingServiceName = "/homectl.v1.PairingService/"

// New builds the daemon's gRPC server with both services registered and an
// auth interceptor that rejects any HomectlService call from a client
// certificate that has not been through PairingService.Pair.
func New(identity *crypto.Identity, store *pairing.Store, window *pairing.Window) *grpc.Server {
	srv := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(crypto.ServerTLSConfig(identity))),
		grpc.UnaryInterceptor(authUnaryInterceptor(store)),
		grpc.StreamInterceptor(authStreamInterceptor(store)),
	)

	pb.RegisterPairingServiceServer(srv, &pairingServer{identity: identity, store: store, window: window})
	pb.RegisterHomectlServiceServer(srv, &homectlServer{})

	return srv
}

func authUnaryInterceptor(store *pairing.Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, pairingServiceName) {
			return handler(ctx, req)
		}
		if err := requireTrustedPeer(ctx, store); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func authStreamInterceptor(store *pairing.Store) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if strings.HasPrefix(info.FullMethod, pairingServiceName) {
			return handler(srv, ss)
		}
		if err := requireTrustedPeer(ss.Context(), store); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func requireTrustedPeer(ctx context.Context, store *pairing.Store) error {
	cert, err := crypto.PeerCertificateFromContext(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "no client certificate: %v", err)
	}
	if !store.IsTrusted(crypto.Fingerprint(cert)) {
		return status.Error(codes.Unauthenticated, "client certificate is not paired with this daemon")
	}
	return nil
}
