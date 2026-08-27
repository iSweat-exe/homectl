package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"homectl/internal/client/grpcclient"
	"homectl/internal/shared/config"
	"homectl/internal/shared/pb"
)

// handleUpload streams a multipart-uploaded file to the daemon in
// TransferChunkSize chunks over the Upload RPC. The destination path on the
// server is given by the "path" query parameter.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	srv, ok := s.serverByID(w, r)
	if !ok {
		return
	}

	destPath := r.URL.Query().Get("path")
	if destPath == "" {
		writeError(w, http.StatusBadRequest, errors.New("missing path query parameter"))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("read uploaded file: %w", err))
		return
	}
	defer file.Close()

	ctx := r.Context()
	client, err := grpcclient.Dial(ctx, srv.Address, s.identity, srv.Fingerprint)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer client.Close()

	stream, err := client.Homectl.Upload(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	if err := stream.Send(&pb.UploadChunk{Payload: &pb.UploadChunk_DestinationPath{DestinationPath: destPath}}); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	buf := make([]byte, config.TransferChunkSize)
	for {
		n, rerr := file.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if err := stream.Send(&pb.UploadChunk{Payload: &pb.UploadChunk_Data{Data: chunk}}); err != nil {
				writeError(w, http.StatusBadGateway, err)
				return
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			writeError(w, http.StatusInternalServerError, rerr)
			return
		}
	}

	summary, err := stream.CloseAndRecv()
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// handleDownload streams a file from the daemon (via the Download RPC)
// straight through to the HTTP response as it arrives.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	srv, ok := s.serverByID(w, r)
	if !ok {
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, errors.New("missing path query parameter"))
		return
	}

	// No timeout on this context beyond the request's own: a download can
	// legitimately take a while for a large file.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	client, err := grpcclient.Dial(ctx, srv.Address, s.identity, srv.Fingerprint)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer client.Close()

	stream, err := client.Homectl.Download(ctx, &pb.DownloadRequest{Path: path})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(path)))

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			// Headers (and possibly some bytes) are likely already sent;
			// there is nothing more useful to do than stop writing.
			return
		}
		if _, err := w.Write(chunk.GetData()); err != nil {
			return
		}
	}
}
