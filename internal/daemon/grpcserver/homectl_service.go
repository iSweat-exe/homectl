package grpcserver

import (
	"context"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execpkg "homectl/internal/daemon/exec"
	"homectl/internal/daemon/system"
	systemdpkg "homectl/internal/daemon/systemd"
	"homectl/internal/daemon/transfer"
	"homectl/internal/shared/pb"
)

type homectlServer struct {
	pb.UnimplementedHomectlServiceServer
}

func (s *homectlServer) SystemInfo(context.Context, *pb.SystemInfoRequest) (*pb.SystemInfoResponse, error) {
	info, err := system.Collect()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "collect system info: %v", err)
	}
	return info, nil
}

func (s *homectlServer) Exec(stream pb.HomectlService_ExecServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	start := first.GetStart()
	if start == nil {
		return status.Error(codes.InvalidArgument, "first message must be a StartCommand")
	}

	sess, err := execpkg.Start(stream.Context(), start.GetCommand(), start.GetArgs(), start.GetWorkingDir())
	if err != nil {
		return status.Errorf(codes.Internal, "start command: %v", err)
	}

	go pumpStdin(stream, sess)

	for chunk := range sess.Chunks {
		var out *pb.ExecOutput
		switch chunk.Kind {
		case execpkg.Stdout:
			out = &pb.ExecOutput{Payload: &pb.ExecOutput_Stdout{Stdout: chunk.Data}}
		case execpkg.Stderr:
			out = &pb.ExecOutput{Payload: &pb.ExecOutput_Stderr{Stderr: chunk.Data}}
		}
		if err := stream.Send(out); err != nil {
			return err
		}
	}

	result := <-sess.Done
	return stream.Send(&pb.ExecOutput{
		Payload: &pb.ExecOutput_Exit{Exit: &pb.ExitStatus{Code: result.ExitCode, Error: result.Err}},
	})
}

// pumpStdin relays further ExecInput messages (stdin chunks / an explicit
// close) from the client stream into the running command's stdin, until the
// stream ends or the command signals it no longer wants stdin.
func pumpStdin(stream pb.HomectlService_ExecServer, sess *execpkg.Session) {
	for {
		in, err := stream.Recv()
		if err != nil {
			sess.CloseStdin()
			return
		}
		switch p := in.GetPayload().(type) {
		case *pb.ExecInput_Stdin:
			if _, werr := sess.Write(p.Stdin); werr != nil {
				sess.CloseStdin()
				return
			}
		case *pb.ExecInput_CloseStdin:
			if p.CloseStdin {
				sess.CloseStdin()
				return
			}
		}
	}
}

func (s *homectlServer) Upload(stream pb.HomectlService_UploadServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	path := first.GetDestinationPath()
	if path == "" {
		return status.Error(codes.InvalidArgument, "first message must carry destination_path")
	}

	written, err := transfer.WriteChunks(path, func() ([]byte, error) {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil, io.EOF
		}
		if err != nil {
			return nil, err
		}
		return chunk.GetData(), nil
	})
	if err != nil {
		return status.Errorf(codes.Internal, "write upload: %v", err)
	}

	return stream.SendAndClose(&pb.UploadSummary{BytesWritten: written, Path: path})
}

func (s *homectlServer) Download(req *pb.DownloadRequest, stream pb.HomectlService_DownloadServer) error {
	if err := transfer.ReadChunks(req.GetPath(), func(data []byte) error {
		return stream.Send(&pb.DownloadChunk{Data: data})
	}); err != nil {
		return status.Errorf(codes.Internal, "read download: %v", err)
	}
	return nil
}

func (s *homectlServer) ListServices(ctx context.Context, _ *pb.ListServicesRequest) (*pb.ListServicesResponse, error) {
	units, err := systemdpkg.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list services: %v", err)
	}

	resp := &pb.ListServicesResponse{Services: make([]*pb.ServiceUnit, 0, len(units))}
	for _, u := range units {
		resp.Services = append(resp.Services, &pb.ServiceUnit{
			Name:          u.Name,
			Description:   u.Description,
			LoadState:     u.LoadState,
			ActiveState:   u.ActiveState,
			SubState:      u.SubState,
			UnitFileState: u.UnitFileState,
		})
	}
	return resp, nil
}

func (s *homectlServer) ServiceAction(ctx context.Context, req *pb.ServiceActionRequest) (*pb.ServiceActionResponse, error) {
	var (
		output string
		err    error
	)
	switch req.GetAction() {
	case pb.ServiceActionType_SERVICE_ACTION_START:
		output, err = systemdpkg.Start(ctx, req.GetUnit())
	case pb.ServiceActionType_SERVICE_ACTION_STOP:
		output, err = systemdpkg.Stop(ctx, req.GetUnit())
	case pb.ServiceActionType_SERVICE_ACTION_RESTART:
		output, err = systemdpkg.Restart(ctx, req.GetUnit())
	case pb.ServiceActionType_SERVICE_ACTION_ENABLE:
		output, err = systemdpkg.Enable(ctx, req.GetUnit())
	case pb.ServiceActionType_SERVICE_ACTION_DISABLE:
		output, err = systemdpkg.Disable(ctx, req.GetUnit())
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown service action %v", req.GetAction())
	}

	if err != nil {
		return &pb.ServiceActionResponse{Success: false, Message: output + err.Error()}, nil
	}
	return &pb.ServiceActionResponse{Success: true, Message: output}, nil
}

func (s *homectlServer) TailLogs(req *pb.TailLogsRequest, stream pb.HomectlService_TailLogsServer) error {
	lines, err := systemdpkg.TailLogs(stream.Context(), req.GetUnit())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "tail logs: %v", err)
	}

	for line := range lines {
		if err := stream.Send(&pb.LogLine{Text: line}); err != nil {
			return err
		}
	}
	return nil
}
