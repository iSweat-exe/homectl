# homectl

A lightweight tool for centrally managing Linux servers on a local network
from a single client machine, with automatic mDNS discovery and no manual
configuration. See [docs/OBJECTIF.md](docs/OBJECTIF.md) for the full spec.

Every daemon↔client connection is mTLS, pinned by certificate fingerprint
(TOFU) — pairing is always a manual, operator-initiated step, never
automatic.

## Requirements

- Go 1.27+ (build machine — any OS; the daemon itself only runs on Linux)
- Node.js 24+ / npm 11+ (to build the web frontend)
- Docker (optional — only needed for the Docker-based end-to-end test)

## Building

```bash
./scripts/build.sh
```

This builds everything into `dist/`:

- `dist/homectl-daemon` — the daemon, cross-compiled for `linux/amd64`
- `dist/homectl` (or `homectl.exe` on Windows) — the native client, with the
  built frontend embedded via `go:embed`

Building by hand instead of the script:

```bash
# Daemon (must target Linux — it's the only supported server platform)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/homectl-daemon ./cmd/daemon

# Frontend (must run before building the client — go:embed needs web/dist to exist)
cd web && npm ci && npm run build && cd ..

# Client (native host platform, embeds web/dist)
go build -o dist/homectl ./cmd/client
```

## Installing the daemon on a server

Copy `dist/homectl-daemon` to the target Linux server, then, as root:

```bash
sudo ./scripts/install-daemon.sh /path/to/homectl-daemon
```

This installs the binary to `/usr/local/bin/homectl-daemon`, registers the
[systemd unit](deploy/systemd/homectl-daemon.service), and starts it. State
(identity key/certificate, trusted-clients list) lives in `/var/lib/homectl`;
the runtime pidfile lives in `/run/homectl` — both created and owned by
systemd via `StateDirectory=`/`RuntimeDirectory=`.

Check it came up and note its fingerprint:

```bash
journalctl -u homectl-daemon -f
```

## Running the client

```bash
./dist/homectl
```

This starts the local HTTP server (default `localhost:8421`) and begins
browsing mDNS for daemons on the network. Open `http://localhost:8421` in a
browser. Client identity and the list of known/paired servers are stored
under the OS config directory (e.g. `%AppData%\homectl` on Windows,
`~/.config/homectl` on Linux); override with `--config-dir`. Use `--http-port`
to change the listen port.

## Pairing a new server

Pairing is mutual: the daemon only accepts a new client certificate while its
pairing window is open, and the client only trusts the daemon's certificate
once you've visually confirmed the fingerprint. Neither side auto-trusts the
other.

1. In the client web UI, the server should appear under "Discovered on this
   network" (via mDNS) — or enter its `address:port` manually if discovery
   doesn't reach it (e.g. across a Docker/VPN boundary). Click **Pair**.
2. On the server, open the pairing window (valid for a short time only):
   ```bash
   sudo homectl-daemon pair
   ```
   (equivalent to sending it `SIGUSR1` directly, e.g. `sudo kill -SIGUSR1 $(cat /run/homectl/daemon.pid)`)
3. Back in the client, verify the displayed fingerprint matches what the
   daemon reports (check `journalctl -u homectl-daemon` for
   `daemon identity fingerprint: ...`), then confirm. This step is the only
   thing standing between you and a spoofed daemon — don't skip it.
4. Once confirmed, the server appears under "Paired servers" with system
   info, a command console, file transfer, and systemd service management
   (start/stop/restart/enable/disable, plus live `journalctl -f` log tailing).

## Releasing

Pushing a semver tag (`vX.Y.Z`) triggers
[.github/workflows/release.yml](.github/workflows/release.yml), which builds
the frontend once, cross-compiles the daemon and client for `linux/amd64` and
`linux/arm64` (both binaries embed the tag as their version via
`-ldflags -X homectl/internal/shared/version.Version=...`, printed by
`homectl-daemon version` / `homectl --version`), and publishes a GitHub
release for the tag with these assets:

- `homectl-daemon-linux-amd64`, `homectl-daemon-linux-arm64`
- `homectl-linux-amd64`, `homectl-linux-arm64`
- `manifest.json` — version, per-artifact `sha256`/size/download URL (see
  [scripts/generate-manifest.sh](scripts/generate-manifest.sh)), fetched by
  `homectl update` / `homectl-daemon update` (below) to find and verify the
  binary matching their own component/OS/CPU architecture.

```bash
git tag v1.1.0 && git push origin v1.1.0
```

## Updating

```bash
homectl update            # or: homectl-daemon update
homectl update --check    # report an available update without installing it
homectl update --repo someone/fork   # fetch the manifest from a different GitHub repo
```

Each command downloads `manifest.json` from the target repo's latest GitHub
release, picks the artifact matching its own component (`client`/`daemon`)
and the machine's OS/CPU architecture (`runtime.GOOS`/`runtime.GOARCH` —
there's no build for anything but `linux/amd64` and `linux/arm64` yet, so it
refuses to update on other platforms), verifies the download's `sha256`
against the manifest, and replaces the running executable in place. Nothing
restarts itself automatically — restart the daemon
(`sudo systemctl restart homectl-daemon`) or the client yourself once it's
done.

## Testing

```bash
go build ./... && go vet ./... && go test ./...   # unit + mDNS loopback discovery test
cd web && npm run build && npm run lint            # frontend typecheck + build + lint
```

The full pairing/RPC flow (SIGUSR1 window, TOFU pairing, SystemInfo, Exec
streaming, Upload/Download, and rejection of an unpaired client) is covered
by a Docker-based end-to-end test, run separately since it needs Docker and
takes longer:

```bash
go test -tags e2e ./e2e/... -v -timeout 5m
```

Not covered by the above: `ListServices`/`ServiceAction`/`TailLogs` (systemd
service management and log tailing) have no automated end-to-end test. The
E2E container is a minimal `debian:bookworm-slim` image with the daemon
binary as `ENTRYPOINT` directly — no systemd/init system, and `systemctl`/
`journalctl` aren't even installed — so it can't exercise these RPCs, the
same limitation mDNS discovery hits crossing the Docker Desktop network
boundary. `internal/daemon/systemd` has unit tests for its JSON parsing and
validation logic via an injectable exec seam, but the actual `systemctl`/
`journalctl` behavior should be verified on a real server after deploying:
pair with it, then list/start/stop/enable a test service and tail its logs
from the UI.
