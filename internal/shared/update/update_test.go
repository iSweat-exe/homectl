package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestManifestFind(t *testing.T) {
	m := &Manifest{
		Version: "v1.2.3",
		Artifacts: []Artifact{
			{Component: "daemon", OS: runtime.GOOS, Arch: runtime.GOARCH, Filename: "homectl-daemon"},
			// Only published for a platform this test never runs on, to
			// exercise the "no build for this OS/arch" path.
			{Component: "client", OS: "plan9", Arch: "amd64", Filename: "homectl-client-plan9"},
		},
	}

	got, ok := m.Find("daemon")
	if !ok {
		t.Fatalf("Find(daemon): expected a match for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if got.Filename != "homectl-daemon" {
		t.Fatalf("Find(daemon) = %+v, want filename homectl-daemon", got)
	}

	if _, ok := m.Find("client"); ok {
		t.Fatalf("Find(client): expected no match since it's only published for plan9/amd64")
	}
	if _, ok := m.Find("nonexistent-component"); ok {
		t.Fatalf("Find(nonexistent-component): expected no match, got one")
	}
}

func TestApplyVerifiesChecksumAndInstalls(t *testing.T) {
	const content = "pretend this is a binary\n"
	sum := sha256.Sum256([]byte(content))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer srv.Close()

	dir := t.TempDir()
	execPath := filepath.Join(dir, "homectl")
	if err := os.WriteFile(execPath, []byte("old binary"), 0o755); err != nil {
		t.Fatalf("seed execPath: %v", err)
	}

	artifact := Artifact{
		Filename: "homectl",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
	}

	if err := Apply(context.Background(), artifact, execPath); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read execPath after Apply: %v", err)
	}
	if string(got) != content {
		t.Fatalf("execPath content = %q, want %q", got, content)
	}

	// No leftover temp file in dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only execPath left in %s, found %d entries", dir, len(entries))
	}
}

func TestApplyRejectsChecksumMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("actual content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	execPath := filepath.Join(dir, "homectl")
	original := []byte("original binary")
	if err := os.WriteFile(execPath, original, 0o755); err != nil {
		t.Fatalf("seed execPath: %v", err)
	}

	artifact := Artifact{
		Filename: "homectl",
		URL:      srv.URL,
		SHA256:   "0000000000000000000000000000000000000000000000000000000000000",
	}

	if err := Apply(context.Background(), artifact, execPath); err == nil {
		t.Fatal("Apply: expected checksum mismatch error, got nil")
	}

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read execPath after failed Apply: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("execPath was modified despite checksum mismatch: got %q", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected temp file to be cleaned up, found %d entries in %s", len(entries), dir)
	}
}
