package httpapi

import (
	"context"
	"net/http"
	"time"

	"homectl/internal/client/grpcclient"
	"homectl/internal/shared/pb"
)

func (s *Server) handleListServers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.List())
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.Remove(id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	srv, ok := s.serverByID(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	client, err := grpcclient.Dial(ctx, srv.Address, s.identity, srv.Fingerprint)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer client.Close()

	info, err := client.Homectl.SystemInfo(ctx, &pb.SystemInfoRequest{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, info)
}
