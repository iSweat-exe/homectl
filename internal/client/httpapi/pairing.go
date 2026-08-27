package httpapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"homectl/internal/client/pairing"
	"homectl/internal/client/store"
)

type proposeRequest struct {
	Address string `json:"address"`
}

type proposeResponse struct {
	Address     string `json:"address"`
	Fingerprint string `json:"fingerprint"`
}

// handlePairingPropose dials a not-yet-paired daemon and reports the
// certificate fingerprint it presents, for the user to visually confirm
// before handlePairingConfirm is called. It does not open the daemon's
// pairing window itself — that's a manual, local step on the daemon
// (SIGUSR1 or `homectl-daemon pair`).
func (s *Server) handlePairingPropose(w http.ResponseWriter, r *http.Request) {
	var req proposeRequest
	if !readJSON(w, r, &req) {
		return
	}
	if req.Address == "" {
		writeError(w, http.StatusBadRequest, errors.New("address is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	proposed, err := pairing.Propose(ctx, s.identity, req.Address)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, proposeResponse{Address: proposed.Address, Fingerprint: proposed.Fingerprint})
}

type confirmRequest struct {
	Address     string `json:"address"`
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name"`
}

// handlePairingConfirm completes pairing with a daemon whose fingerprint
// the user has already confirmed out of band (via handlePairingPropose's
// response). It re-dials pinned to that exact fingerprint, so the
// connection used to send Pair is provably the one the user saw.
func (s *Server) handlePairingConfirm(w http.ResponseWriter, r *http.Request) {
	var req confirmRequest
	if !readJSON(w, r, &req) {
		return
	}
	if req.Address == "" || req.Fingerprint == "" {
		writeError(w, http.StatusBadRequest, errors.New("address and fingerprint are required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "homectl-client"
	}

	if err := pairing.Confirm(ctx, s.identity, req.Address, req.Fingerprint, hostname); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	label := req.Name
	if label == "" {
		label = req.Address
	}

	srv := store.Server{
		ID:          store.IDFromFingerprint(req.Fingerprint),
		Name:        label,
		Address:     req.Address,
		Fingerprint: req.Fingerprint,
		PairedAt:    time.Now().UTC(),
	}
	if err := s.store.Add(srv); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, srv)
}
