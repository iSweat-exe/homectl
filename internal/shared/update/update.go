// Package update implements the logic behind the `homectl update` and
// `homectl-daemon update` subcommands: fetch the release manifest published
// by .github/workflows/release.yml (see scripts/generate-manifest.sh),
// find the artifact matching this binary's component and the machine's own
// OS/CPU architecture, and — after verifying its sha256 — replace the
// currently running executable with it.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	manifestFetchTimeout = 15 * time.Second
	downloadTimeout      = 5 * time.Minute
)

// Artifact describes one release binary, as produced by
// scripts/generate-manifest.sh.
type Artifact struct {
	Component string `json:"component"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	URL       string `json:"url"`
}

// Manifest is the release manifest published alongside the binaries.
type Manifest struct {
	Version    string     `json:"version"`
	ReleasedAt string     `json:"released_at"`
	Artifacts  []Artifact `json:"artifacts"`
}

// FetchManifest downloads and parses the manifest.json at url.
func FetchManifest(ctx context.Context, url string) (*Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, manifestFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %s", url, resp.Status)
	}

	var m Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &m, nil
}

// Find returns the artifact in m for component that matches the current
// binary's OS and CPU architecture (runtime.GOOS / runtime.GOARCH), or
// false if this platform has no published build.
func (m *Manifest) Find(component string) (Artifact, bool) {
	for _, a := range m.Artifacts {
		if a.Component == component && a.OS == runtime.GOOS && a.Arch == runtime.GOARCH {
			return a, true
		}
	}
	return Artifact{}, false
}

// Apply downloads artifact and installs it at execPath, verifying the
// download's sha256 against the manifest first. The download is written to
// a temp file in execPath's directory and installed via rename, so a
// failed or interrupted download never leaves execPath truncated or
// missing.
func Apply(ctx context.Context, a Artifact, execPath string) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", a.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", a.URL, resp.Status)
	}

	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, ".homectl-update-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	hasher := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(resp.Body, hasher)); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}

	if sum := hex.EncodeToString(hasher.Sum(nil)); sum != a.SHA256 {
		return fmt.Errorf("checksum mismatch for %s: downloaded file does not match manifest (got %s, want %s)", a.Filename, sum, a.SHA256)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("install to %s: %w", execPath, err)
	}
	return nil
}

// RunCLI implements the `update` subcommand shared by homectl and
// homectl-daemon: report (and, unless -check is set, install) the release
// build matching component ("client" or "daemon") and the machine's own
// OS/CPU architecture. currentVersion is the running binary's own
// version.Version.
func RunCLI(component, currentVersion, defaultRepo string, args []string) error {
	fs := flag.NewFlagSet(component+" update", flag.ExitOnError)
	repo := fs.String("repo", defaultRepo, `GitHub "owner/repo" to fetch the release manifest from`)
	checkOnly := fs.Bool("check", false, "check for an available update without installing it")
	if err := fs.Parse(args); err != nil {
		return err
	}

	manifestURL := fmt.Sprintf("https://github.com/%s/releases/latest/download/manifest.json", *repo)
	fmt.Printf("checking %s for updates...\n", manifestURL)

	manifest, err := FetchManifest(context.Background(), manifestURL)
	if err != nil {
		return fmt.Errorf("fetch release manifest: %w", err)
	}

	artifact, ok := manifest.Find(component)
	if !ok {
		return fmt.Errorf("no %s build published for %s/%s in %s (release %s)", component, runtime.GOOS, runtime.GOARCH, *repo, manifest.Version)
	}

	if currentVersion != "dev" && currentVersion == manifest.Version {
		fmt.Printf("%s is up to date (%s, %s/%s)\n", component, currentVersion, runtime.GOOS, runtime.GOARCH)
		return nil
	}

	fmt.Printf("update available for %s/%s: %s -> %s\n", runtime.GOOS, runtime.GOARCH, currentVersion, manifest.Version)
	if *checkOnly {
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate running executable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", execPath, err)
	}

	fmt.Printf("downloading %s...\n", artifact.URL)
	if err := Apply(context.Background(), artifact, execPath); err != nil {
		return fmt.Errorf("install update: %w", err)
	}

	fmt.Printf("updated %s to %s (%s)\n", component, manifest.Version, execPath)
	if component == "daemon" {
		fmt.Println("restart it to apply: sudo systemctl restart homectl-daemon")
	} else {
		fmt.Println("restart it to apply the update")
	}
	return nil
}
