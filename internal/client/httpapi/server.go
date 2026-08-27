// Package httpapi is the client's local HTTP backend: it serves the JSON
// API the embedded frontend talks to, translating each request into an
// on-demand mTLS gRPC call to the relevant paired daemon. Connections are
// not pooled — every request dials fresh, pinned to the server's stored
// fingerprint, which keeps the implementation simple at the cost of a
// reconnect per call (acceptable for a lightweight LAN tool).
package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"homectl/internal/client/discovery"
	"homectl/internal/client/store"
	"homectl/internal/shared/crypto"
)

// Server implements http.Handler for every /api/... route.
type Server struct {
	identity *crypto.Identity
	store    *store.Store
	browser  *discovery.Browser

	mux *http.ServeMux
}

// New builds the HTTP API, wired to identity (the client's own mTLS
// identity), st (the known-servers store) and browser (the live mDNS scan).
func New(identity *crypto.Identity, st *store.Store, browser *discovery.Browser) *Server {
	s := &Server{identity: identity, store: st, browser: browser, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/discovery", s.handleDiscovery)

	s.mux.HandleFunc("GET /api/servers", s.handleListServers)
	s.mux.HandleFunc("DELETE /api/servers/{id}", s.handleDeleteServer)
	s.mux.HandleFunc("GET /api/servers/{id}/system-info", s.handleSystemInfo)
	s.mux.HandleFunc("GET /api/servers/{id}/exec", s.handleExecWS)
	s.mux.HandleFunc("POST /api/servers/{id}/upload", s.handleUpload)
	s.mux.HandleFunc("GET /api/servers/{id}/download", s.handleDownload)
	s.mux.HandleFunc("GET /api/servers/{id}/services", s.handleListServices)
	s.mux.HandleFunc("POST /api/servers/{id}/services/{unit}/action", s.handleServiceAction)
	s.mux.HandleFunc("GET /api/servers/{id}/services/{unit}/logs", s.handleServiceLogsWS)

	s.mux.HandleFunc("POST /api/pairing/propose", s.handlePairingPropose)
	s.mux.HandleFunc("POST /api/pairing/confirm", s.handlePairingConfirm)
}

// serverByID resolves the {id} path value to a stored server, writing a 404
// JSON error itself when unknown.
func (s *Server) serverByID(w http.ResponseWriter, r *http.Request) (store.Server, bool) {
	id := r.PathValue("id")
	srv, ok := s.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown server id %q", id))
		return store.Server{}, false
	}
	return srv, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON body: %w", err))
		return false
	}
	return true
}
