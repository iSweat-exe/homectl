// Command homectl-daemon runs the homectl daemon: it announces itself over
// mDNS, serves PairingService and HomectlService over mTLS, and opens a
// pairing window only when the operator signals it (SIGUSR1, or via the
// `pair` subcommand from the same machine).
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"homectl/internal/daemon/discovery"
	"homectl/internal/daemon/grpcserver"
	"homectl/internal/daemon/pairing"
	"homectl/internal/shared/config"
	"homectl/internal/shared/crypto"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) > 1 && os.Args[1] == "pair" {
		pairCmd(os.Args[2:])
		return
	}
	runCmd(os.Args[1:])
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("homectl-daemon", flag.ExitOnError)
	grpcPort := fs.Int("grpc-port", config.DefaultGRPCPort, "gRPC/mTLS listen port")
	stateDir := fs.String("state-dir", config.DaemonStateDir(), "directory for identity + trusted-clients state")
	runDir := fs.String("run-dir", config.DaemonRunDir(), "directory for the daemon pidfile")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	windowDuration, err := time.ParseDuration(config.PairingWindowDuration)
	if err != nil {
		log.Fatalf("parse pairing window duration: %v", err)
	}

	if err := os.MkdirAll(*runDir, 0o700); err != nil {
		log.Fatalf("create run dir: %v", err)
	}
	pidPath := filepath.Join(*runDir, "daemon.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		log.Fatalf("write pidfile: %v", err)
	}
	defer os.Remove(pidPath)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "homectl-daemon"
	}

	identity, err := crypto.LoadOrCreateIdentity(*stateDir, "homectl-daemon@"+hostname)
	if err != nil {
		log.Fatalf("load identity: %v", err)
	}
	log.Printf("daemon identity fingerprint: %s", identity.Fingerprint())

	store, err := pairing.NewStore(*stateDir)
	if err != nil {
		log.Fatalf("load trusted clients store: %v", err)
	}

	window := &pairing.Window{}

	pairSig := make(chan os.Signal, 1)
	notifyPairingSignal(pairSig)
	go func() {
		for range pairSig {
			window.Open(windowDuration)
			log.Printf("pairing window open for %s", windowDuration)
		}
	}()

	mdnsServer, err := discovery.Announce(*grpcPort)
	if err != nil {
		log.Fatalf("announce mDNS: %v", err)
	}
	defer mdnsServer.Shutdown()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		log.Fatalf("listen on :%d: %v", *grpcPort, err)
	}

	srv := grpcserver.New(identity, store, window)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("shutting down")
		srv.GracefulStop()
	}()

	log.Printf("gRPC/mTLS listening on :%d", *grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func pairCmd(args []string) {
	fs := flag.NewFlagSet("homectl-daemon pair", flag.ExitOnError)
	runDir := fs.String("run-dir", config.DaemonRunDir(), "directory holding the daemon pidfile")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	pidPath := filepath.Join(*runDir, "daemon.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		log.Fatalf("read pidfile %s: %v", pidPath, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		log.Fatalf("parse pidfile %s: %v", pidPath, err)
	}

	if err := sendPairingSignal(pid); err != nil {
		log.Fatalf("%v", err)
	}
	log.Printf("pairing window opened on daemon pid %d for %s", pid, config.PairingWindowDuration)
}
