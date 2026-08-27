// Command homectl runs the client: it browses for daemons over mDNS, serves
// the local JSON API the embedded frontend talks to, and serves that
// frontend itself, all on a single localhost port.
package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"homectl/internal/client/discovery"
	"homectl/internal/client/httpapi"
	"homectl/internal/client/store"
	"homectl/internal/shared/config"
	"homectl/internal/shared/crypto"
	frontendpkg "homectl/web"
)

func main() {
	log.SetFlags(0)

	flagSet := flag.NewFlagSet("homectl", flag.ExitOnError)
	httpPort := flagSet.Int("http-port", config.DefaultHTTPPort, "local HTTP listen port")
	configDir := flagSet.String("config-dir", "", "directory for client identity + known-servers state (default: OS config dir)")
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	dir := *configDir
	if dir == "" {
		d, err := config.ClientConfigDir()
		if err != nil {
			log.Fatalf("resolve config dir: %v", err)
		}
		dir = d
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "homectl-client"
	}

	identity, err := crypto.LoadOrCreateIdentity(dir, "homectl-client@"+hostname)
	if err != nil {
		log.Fatalf("load identity: %v", err)
	}
	log.Printf("client identity fingerprint: %s", identity.Fingerprint())

	st, err := store.NewStore(dir)
	if err != nil {
		log.Fatalf("load known servers store: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	browser, err := discovery.Start(ctx)
	if err != nil {
		log.Fatalf("start mdns browser: %v", err)
	}
	defer browser.Stop()

	frontend, err := fs.Sub(frontendpkg.Dist, "dist")
	if err != nil {
		log.Fatalf("load embedded frontend: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", httpapi.New(identity, st, browser))
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	addr := net.JoinHostPort("localhost", strconv.Itoa(*httpPort))
	srv := &http.Server{Addr: addr, Handler: mux}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("shutting down")
		_ = srv.Close()
	}()

	log.Printf("homectl listening on http://%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}
