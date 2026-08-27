package httpapi

import (
	"net"
	"net/http"
	"strconv"

	"homectl/internal/client/store"
)

type discoveredServer struct {
	InstanceName string `json:"instance_name"`
	Address      string `json:"address"`
	Paired       bool   `json:"paired"`
	ServerID     string `json:"server_id,omitempty"`
}

func (s *Server) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	found := s.browser.Snapshot()
	paired := s.store.List()

	pairedByAddr := make(map[string]store.Server, len(paired))
	for _, p := range paired {
		pairedByAddr[p.Address] = p
	}

	out := make([]discoveredServer, 0, len(found))
	for _, f := range found {
		addr := net.JoinHostPort(f.Host, strconv.Itoa(f.Port))
		d := discoveredServer{InstanceName: f.InstanceName, Address: addr}
		if p, ok := pairedByAddr[addr]; ok {
			d.Paired = true
			d.ServerID = p.ID
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}
