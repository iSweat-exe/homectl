// Package version holds the build-time version string reported by both
// binaries (`homectl --version` / `homectl-daemon version`) and, in the
// future, compared against the release manifest by the planned
// `homectl update` / `homectl-daemon update` commands.
package version

// Version is overridden at release build time via
// -ldflags "-X homectl/internal/shared/version.Version=vX.Y.Z" (see
// .github/workflows/release.yml). It stays "dev" for local builds.
var Version = "dev"
