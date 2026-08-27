package httpapi

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"

	"homectl/internal/client/grpcclient"
	"homectl/internal/shared/pb"
)

// wsUpgrader deliberately keeps gorilla's default CheckOrigin (same-origin
// only): the frontend is always served from this same process/origin, so
// there is no legitimate cross-origin caller of this endpoint.
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// wsClientMessage is a message the frontend sends over the exec WebSocket.
// The first message of a connection must have Type "start"; afterwards
// "stdin" and "close_stdin" may be sent any number of times.
type wsClientMessage struct {
	Type       string   `json:"type"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	WorkingDir string   `json:"working_dir,omitempty"`
	Data       []byte   `json:"data,omitempty"`
}

// wsServerMessage is a message sent to the frontend: "stdout"/"stderr" data
// chunks, a final "exit" with the command's result, or an "error".
type wsServerMessage struct {
	Type      string `json:"type"`
	Data      []byte `json:"data,omitempty"`
	Code      int32  `json:"code,omitempty"`
	ExitError string `json:"exit_error,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleExecWS bridges a browser WebSocket to the daemon's bidirectional
// Exec RPC: frontend "start"/"stdin"/"close_stdin" messages become
// ExecInput messages, and ExecOutput messages become "stdout"/"stderr"/
// "exit" WebSocket messages.
func (s *Server) handleExecWS(w http.ResponseWriter, r *http.Request) {
	srv, ok := s.serverByID(w, r)
	if !ok {
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote an HTTP error response.
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	client, err := grpcclient.Dial(ctx, srv.Address, s.identity, srv.Fingerprint)
	if err != nil {
		_ = conn.WriteJSON(wsServerMessage{Type: "error", Error: err.Error()})
		return
	}
	defer client.Close()

	stream, err := client.Homectl.Exec(ctx)
	if err != nil {
		_ = conn.WriteJSON(wsServerMessage{Type: "error", Error: err.Error()})
		return
	}

	var start wsClientMessage
	if err := conn.ReadJSON(&start); err != nil || start.Type != "start" {
		_ = conn.WriteJSON(wsServerMessage{Type: "error", Error: "first message must have type=start"})
		return
	}
	if err := stream.Send(&pb.ExecInput{Payload: &pb.ExecInput_Start{Start: &pb.StartCommand{
		Command:    start.Command,
		Args:       start.Args,
		WorkingDir: start.WorkingDir,
	}}}); err != nil {
		_ = conn.WriteJSON(wsServerMessage{Type: "error", Error: err.Error()})
		return
	}

	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		for {
			out, err := stream.Recv()
			if err != nil {
				return
			}
			switch p := out.GetPayload().(type) {
			case *pb.ExecOutput_Stdout:
				_ = conn.WriteJSON(wsServerMessage{Type: "stdout", Data: p.Stdout})
			case *pb.ExecOutput_Stderr:
				_ = conn.WriteJSON(wsServerMessage{Type: "stderr", Data: p.Stderr})
			case *pb.ExecOutput_Exit:
				_ = conn.WriteJSON(wsServerMessage{Type: "exit", Code: p.Exit.GetCode(), ExitError: p.Exit.GetError()})
				return
			}
		}
	}()

	for {
		var msg wsClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			_ = stream.CloseSend()
			break
		}
		switch msg.Type {
		case "stdin":
			if err := stream.Send(&pb.ExecInput{Payload: &pb.ExecInput_Stdin{Stdin: msg.Data}}); err != nil {
				_ = stream.CloseSend()
			}
		case "close_stdin":
			if err := stream.Send(&pb.ExecInput{Payload: &pb.ExecInput_CloseStdin{CloseStdin: true}}); err != nil {
				_ = stream.CloseSend()
			}
		}
	}

	<-recvDone
}
