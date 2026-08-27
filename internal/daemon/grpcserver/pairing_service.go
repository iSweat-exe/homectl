package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"homectl/internal/daemon/pairing"
	"homectl/internal/shared/crypto"
	"homectl/internal/shared/pb"
)

type pairingServer struct {
	pb.UnimplementedPairingServiceServer

	identity *crypto.Identity
	store    *pairing.Store
	window   *pairing.Window
}

func (s *pairingServer) GetFingerprint(_ context.Context, _ *pb.FingerprintRequest) (*pb.FingerprintResponse, error) {
	return &pb.FingerprintResponse{Fingerprint: s.identity.Fingerprint()}, nil
}

func (s *pairingServer) Pair(ctx context.Context, req *pb.PairRequest) (*pb.PairResponse, error) {
	if !s.window.IsOpen() {
		return nil, status.Error(codes.FailedPrecondition,
			"pairing window is closed; open it on the server (SIGUSR1, or `homectl-daemon pair`) and retry")
	}

	cert, err := crypto.PeerCertificateFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "no client certificate presented: %v", err)
	}

	fp := crypto.Fingerprint(cert)
	label := req.GetClientName()
	if label == "" {
		label = fp
	}
	if err := s.store.Trust(fp, label); err != nil {
		return nil, status.Errorf(codes.Internal, "persist trusted client: %v", err)
	}
	// Single-use: close the window as soon as one client has paired, to
	// minimize the exposure window on a shared network.
	s.window.Close()

	return &pb.PairResponse{
		Accepted:          true,
		DaemonFingerprint: s.identity.Fingerprint(),
		Message:           "paired successfully",
	}, nil
}
