package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"homectl/internal/client/grpcclient"
	"homectl/internal/shared/pb"
)

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
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

	resp, err := client.Homectl.ListServices(ctx, &pb.ListServicesRequest{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, resp.GetServices())
}

// serviceActions maps the action name sent by the frontend to the proto enum.
var serviceActions = map[string]pb.ServiceActionType{
	"start":   pb.ServiceActionType_SERVICE_ACTION_START,
	"stop":    pb.ServiceActionType_SERVICE_ACTION_STOP,
	"restart": pb.ServiceActionType_SERVICE_ACTION_RESTART,
	"enable":  pb.ServiceActionType_SERVICE_ACTION_ENABLE,
	"disable": pb.ServiceActionType_SERVICE_ACTION_DISABLE,
}

type serviceActionRequest struct {
	Action string `json:"action"`
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	srv, ok := s.serverByID(w, r)
	if !ok {
		return
	}
	unit := r.PathValue("unit")

	var body serviceActionRequest
	if !readJSON(w, r, &body) {
		return
	}
	action, ok := serviceActions[body.Action]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown action %q", body.Action))
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

	resp, err := client.Homectl.ServiceAction(ctx, &pb.ServiceActionRequest{Unit: unit, Action: action})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// wsLogMessage is a message sent to the frontend over the log-tail
// WebSocket: either a "line" of journalctl output or a terminal "error".
type wsLogMessage struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

// handleServiceLogsWS bridges a browser WebSocket to the daemon's
// server-streaming TailLogs RPC. The browser never sends meaningful data;
// its only inbound messages are read purely to detect the socket closing,
// which cancels ctx and kills journalctl on the daemon side.
func (s *Server) handleServiceLogsWS(w http.ResponseWriter, r *http.Request) {
	srv, ok := s.serverByID(w, r)
	if !ok {
		return
	}
	unit := r.PathValue("unit")

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote an HTTP error response.
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	client, err := grpcclient.Dial(ctx, srv.Address, s.identity, srv.Fingerprint)
	if err != nil {
		_ = conn.WriteJSON(wsLogMessage{Type: "error", Error: err.Error()})
		return
	}
	defer client.Close()

	stream, err := client.Homectl.TailLogs(ctx, &pb.TailLogsRequest{Unit: unit})
	if err != nil {
		_ = conn.WriteJSON(wsLogMessage{Type: "error", Error: err.Error()})
		return
	}

	// Drain inbound messages solely to notice the browser closing the
	// socket, so ctx gets cancelled and journalctl is killed daemon-side.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	for {
		line, err := stream.Recv()
		if err != nil {
			return
		}
		if err := conn.WriteJSON(wsLogMessage{Type: "line", Text: line.GetText()}); err != nil {
			return
		}
	}
}
