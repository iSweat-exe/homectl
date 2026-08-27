//go:build e2e

// Package e2e is a Docker-based end-to-end test exercising the full
// homectl pairing + RPC surface against a real daemon binary running in a
// container, reached directly by IP:port (no mDNS — mDNS does not cross the
// Docker Desktop network boundary, so it is covered separately by the
// loopback test in internal/client/discovery). Requires a working Docker
// daemon on PATH. Run with:
//
//	go test -tags e2e ./e2e/... -v -timeout 5m
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"homectl/internal/client/grpcclient"
	clientpairing "homectl/internal/client/pairing"
	"homectl/internal/shared/crypto"
	"homectl/internal/shared/pb"
)

const (
	hostPort      = 47331
	containerName = "homectl-e2e-test"
	imageTag      = "homectl-e2e-test:latest"
)

func TestDockerEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available on PATH")
	}

	// Best-effort cleanup of a leftover container from a previous crashed run.
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	buildDir := t.TempDir()
	buildDaemonBinary(t, filepath.Join(buildDir, "homectl-daemon"))
	writeDockerfile(t, buildDir)
	buildImage(t, buildDir)
	t.Cleanup(func() { runDocker(t, "rmi", "-f", imageTag) })

	startContainer(t)
	t.Cleanup(func() { runDocker(t, "rm", "-f", containerName) })

	addr := fmt.Sprintf("127.0.0.1:%d", hostPort)
	waitForDaemon(t, addr)

	clientDir := t.TempDir()
	identity, err := crypto.LoadOrCreateIdentity(clientDir, "e2e-client")
	if err != nil {
		t.Fatalf("create client identity: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	proposed, err := clientpairing.Propose(ctx, identity, addr)
	if err != nil {
		t.Fatalf("propose (learn fingerprint): %v", err)
	}
	if proposed.Fingerprint == "" {
		t.Fatal("propose returned empty fingerprint")
	}

	// The daemon's pairing window is closed until the operator opens it;
	// pairing must be rejected before that happens (proves there is no
	// auto-trust in either direction).
	if err := clientpairing.Confirm(ctx, identity, addr, proposed.Fingerprint, "e2e-test-client"); err == nil {
		t.Fatal("expected pairing to be rejected before SIGUSR1, but it succeeded")
	}

	// `docker kill --signal` delivers straight to the container's PID 1,
	// so no `kill` binary needs to exist inside the (debian-slim) image.
	runDocker(t, "kill", "--signal=USR1", containerName)
	time.Sleep(300 * time.Millisecond)

	if err := clientpairing.Confirm(ctx, identity, addr, proposed.Fingerprint, "e2e-test-client"); err != nil {
		t.Fatalf("expected pairing to succeed after SIGUSR1: %v", err)
	}

	client, err := grpcclient.Dial(ctx, addr, identity, proposed.Fingerprint)
	if err != nil {
		t.Fatalf("dial paired: %v", err)
	}
	defer client.Close()

	t.Run("SystemInfo", func(t *testing.T) {
		info, err := client.Homectl.SystemInfo(ctx, &pb.SystemInfoRequest{})
		if err != nil {
			t.Fatalf("system info: %v", err)
		}
		if info.GetHostname() == "" {
			t.Fatal("system info returned empty hostname")
		}
		t.Logf("container system info: hostname=%s os=%s cores=%d", info.GetHostname(), info.GetOs(), info.GetCpuCores())
	})

	t.Run("Exec", func(t *testing.T) {
		// command is a full "sh -c" string; args (if any) are exposed to it
		// as positional parameters, not appended to the command itself.
		res := runExec(t, ctx, client, "echo hello-e2e", nil)
		if !bytes.Contains(res.stdout, []byte("hello-e2e")) {
			t.Fatalf("expected stdout to contain hello-e2e, got %q", res.stdout)
		}
		if res.exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d (stderr=%q)", res.exitCode, res.stderr)
		}
	})

	t.Run("UploadDownload", func(t *testing.T) {
		const remotePath = "/tmp/homectl-e2e-roundtrip.txt"
		payload := []byte("homectl e2e round-trip payload\n")

		uploadStream, err := client.Homectl.Upload(ctx)
		if err != nil {
			t.Fatalf("open upload stream: %v", err)
		}
		if err := uploadStream.Send(&pb.UploadChunk{Payload: &pb.UploadChunk_DestinationPath{DestinationPath: remotePath}}); err != nil {
			t.Fatalf("send destination path: %v", err)
		}
		if err := uploadStream.Send(&pb.UploadChunk{Payload: &pb.UploadChunk_Data{Data: payload}}); err != nil {
			t.Fatalf("send upload data: %v", err)
		}
		summary, err := uploadStream.CloseAndRecv()
		if err != nil {
			t.Fatalf("close upload stream: %v", err)
		}
		if summary.GetBytesWritten() != uint64(len(payload)) {
			t.Fatalf("expected %d bytes written, got %d", len(payload), summary.GetBytesWritten())
		}

		downloadStream, err := client.Homectl.Download(ctx, &pb.DownloadRequest{Path: remotePath})
		if err != nil {
			t.Fatalf("open download stream: %v", err)
		}
		var got bytes.Buffer
		for {
			chunk, err := downloadStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("recv download chunk: %v", err)
			}
			got.Write(chunk.GetData())
		}
		if got.String() != string(payload) {
			t.Fatalf("round-trip mismatch: sent %q, got %q", payload, got.String())
		}
	})

	// A brand new identity that never went through Pair must be rejected,
	// even though it knows the daemon's (correct) fingerprint — proves
	// trust is keyed on the pinned *client* certificate, not just TLS.
	t.Run("UnpairedRejected", func(t *testing.T) {
		strangerDir := t.TempDir()
		stranger, err := crypto.LoadOrCreateIdentity(strangerDir, "e2e-stranger")
		if err != nil {
			t.Fatalf("create stranger identity: %v", err)
		}
		strangerClient, err := grpcclient.Dial(ctx, addr, stranger, proposed.Fingerprint)
		if err != nil {
			t.Fatalf("dial as stranger: %v", err)
		}
		defer strangerClient.Close()

		if _, err := strangerClient.Homectl.SystemInfo(ctx, &pb.SystemInfoRequest{}); err == nil {
			t.Fatal("expected an unpaired client to be rejected, but the call succeeded")
		}
	})
}

type execResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int32
}

func runExec(t *testing.T, ctx context.Context, client *grpcclient.Client, command string, args []string) execResult {
	t.Helper()

	stream, err := client.Homectl.Exec(ctx)
	if err != nil {
		t.Fatalf("open exec stream: %v", err)
	}
	if err := stream.Send(&pb.ExecInput{Payload: &pb.ExecInput_Start{Start: &pb.StartCommand{Command: command, Args: args}}}); err != nil {
		t.Fatalf("send start command: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close exec send side: %v", err)
	}

	var res execResult
	for {
		out, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv exec output: %v", err)
		}
		switch p := out.GetPayload().(type) {
		case *pb.ExecOutput_Stdout:
			res.stdout = append(res.stdout, p.Stdout...)
		case *pb.ExecOutput_Stderr:
			res.stderr = append(res.stderr, p.Stderr...)
		case *pb.ExecOutput_Exit:
			res.exitCode = p.Exit.GetCode()
		}
	}
	return res
}

func buildDaemonBinary(t *testing.T, out string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", out, "./cmd/daemon")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	cmd.Dir = moduleRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build daemon binary: %v\n%s", err, output)
	}
}

func writeDockerfile(t *testing.T, dir string) {
	t.Helper()
	content := "FROM debian:bookworm-slim\nCOPY homectl-daemon /homectl-daemon\nENTRYPOINT [\"/homectl-daemon\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(content), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
}

func buildImage(t *testing.T, dir string) {
	t.Helper()
	runDocker(t, "build", "-t", imageTag, dir)
}

func startContainer(t *testing.T) {
	t.Helper()
	runDocker(t, "run", "-d", "--name", containerName,
		"-p", fmt.Sprintf("%d:%d", hostPort, hostPort), imageTag)
}

func waitForDaemon(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			identity, err := crypto.LoadOrCreateIdentity(t.TempDir(), "e2e-probe")
			if err != nil {
				lastErr = err
				return
			}
			client, err := grpcclient.Dial(ctx, addr, identity, "")
			if err != nil {
				lastErr = err
				return
			}
			defer client.Close()
			_, lastErr = client.Pairing.GetFingerprint(ctx, &pb.FingerprintRequest{})
		}()
		if lastErr == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("daemon did not become ready on %s: %v", addr, lastErr)
}

func runDocker(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("docker", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker %v: %v\n%s", args, err, output)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd) // e2e/ is one level below the module root
}
