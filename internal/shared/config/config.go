// Package config holds the small set of constants and path helpers shared
// between the daemon and the client, so the two never drift apart on
// network defaults or on-disk layout.
package config

import (
	"os"
	"path/filepath"
)

const (
	// MDNSServiceType is the mDNS service type the daemon announces itself
	// under and the client browses for.
	MDNSServiceType = "_homectl._tcp"
	MDNSDomain      = "local."

	DefaultGRPCPort = 47331
	DefaultHTTPPort = 8421

	// TransferChunkSize is the size of each Upload/Download chunk.
	TransferChunkSize = 64 * 1024

	// PairingWindowDuration is how long the daemon accepts a new client
	// certificate after an operator opens the pairing window (SIGUSR1).
	PairingWindowDuration = "2m0s"

	// ReleaseRepo is the GitHub "owner/repo" the `update` subcommand fetches
	// its release manifest from by default (see internal/shared/update).
	ReleaseRepo = "iSweat-exe/homectl"

	daemonStateDirEnv = "HOMECTL_STATE_DIR"
	daemonRunDirEnv   = "HOMECTL_RUN_DIR"

	defaultDaemonStateDir = "/var/lib/homectl"
	defaultDaemonRunDir   = "/run/homectl"
)

// DaemonStateDir returns the directory holding the daemon's persistent
// state (identity key/certificate, trusted-clients store). Overridable via
// HOMECTL_STATE_DIR, mainly for local development/testing without root.
func DaemonStateDir() string {
	if v := os.Getenv(daemonStateDirEnv); v != "" {
		return v
	}
	return defaultDaemonStateDir
}

// DaemonRunDir returns the directory holding the daemon's runtime state
// (pidfile). Overridable via HOMECTL_RUN_DIR.
func DaemonRunDir() string {
	if v := os.Getenv(daemonRunDirEnv); v != "" {
		return v
	}
	return defaultDaemonRunDir
}

// DaemonPIDFile returns the path of the daemon's pidfile, used by the
// `homectl-daemon pair` convenience subcommand to signal the running
// daemon process.
func DaemonPIDFile() string {
	return filepath.Join(DaemonRunDir(), "daemon.pid")
}

// ClientConfigDir returns the per-user directory holding the client's own
// identity and its known-servers store, using the OS's conventional config
// location (%AppData% on Windows, ~/.config on Linux, ...).
func ClientConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "homectl"), nil
}
