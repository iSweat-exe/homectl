// Package systemd shells out to systemctl and journalctl to list, control,
// and tail the logs of systemd service units, for the daemon's
// ListServices/ServiceAction/TailLogs RPCs.
package systemd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
)

var unitNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:_.@-]*\.service$`)

// ValidateUnitName rejects anything that isn't a plausible systemd service
// unit name. exec.Command (no shell involved) already rules out shell
// injection, but this still guards against a unit name being misread as a
// systemctl/journalctl flag (e.g. one starting with "-"), and keeps the
// surface scoped to .service units.
func ValidateUnitName(unit string) error {
	if !unitNamePattern.MatchString(unit) {
		return fmt.Errorf("invalid service unit name %q", unit)
	}
	return nil
}

// Unit is one systemd service unit's merged state, from list-units
// (load/active/sub state, description) and list-unit-files (enabled state).
type Unit struct {
	Name          string
	Description   string
	LoadState     string
	ActiveState   string
	SubState      string
	UnitFileState string
}

// runOutput and runCombined indirect over os/exec so tests can inject
// fixture output without a real systemd instance.
var runOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

var runCombined = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type listUnitsRow struct {
	Unit        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
}

type listUnitFilesRow struct {
	UnitFile string `json:"unit_file"`
	State    string `json:"state"`
}

// List returns every service unit systemd knows about (loaded or not),
// merging systemctl's "currently loaded units" view with its "unit files on
// disk" view so the enabled/disabled state is available even for inactive
// services.
func List(ctx context.Context) ([]Unit, error) {
	unitsOut, err := runOutput(ctx, "systemctl", "list-units", "--all", "--type=service", "--output=json", "--no-pager")
	if err != nil {
		return nil, fmt.Errorf("systemctl list-units: %w", err)
	}
	var rows []listUnitsRow
	if err := json.Unmarshal(unitsOut, &rows); err != nil {
		return nil, fmt.Errorf("parse list-units json: %w", err)
	}

	filesOut, err := runOutput(ctx, "systemctl", "list-unit-files", "--type=service", "--output=json", "--no-pager")
	if err != nil {
		return nil, fmt.Errorf("systemctl list-unit-files: %w", err)
	}
	var fileRows []listUnitFilesRow
	if err := json.Unmarshal(filesOut, &fileRows); err != nil {
		return nil, fmt.Errorf("parse list-unit-files json: %w", err)
	}
	fileState := make(map[string]string, len(fileRows))
	for _, r := range fileRows {
		fileState[r.UnitFile] = r.State
	}

	units := make([]Unit, 0, len(rows))
	for _, r := range rows {
		units = append(units, Unit{
			Name:          r.Unit,
			Description:   r.Description,
			LoadState:     r.Load,
			ActiveState:   r.Active,
			SubState:      r.Sub,
			UnitFileState: fileState[r.Unit],
		})
	}
	return units, nil
}

// Start starts unit via "systemctl start", returning its combined output
// (useful for diagnosing a failure) alongside any error.
func Start(ctx context.Context, unit string) (string, error) { return action(ctx, "start", unit) }

// Stop stops unit via "systemctl stop".
func Stop(ctx context.Context, unit string) (string, error) { return action(ctx, "stop", unit) }

// Restart restarts unit via "systemctl restart".
func Restart(ctx context.Context, unit string) (string, error) { return action(ctx, "restart", unit) }

// Enable enables unit (boot activation) via "systemctl enable".
func Enable(ctx context.Context, unit string) (string, error) { return action(ctx, "enable", unit) }

// Disable disables unit (boot activation) via "systemctl disable".
func Disable(ctx context.Context, unit string) (string, error) { return action(ctx, "disable", unit) }

func action(ctx context.Context, verb, unit string) (string, error) {
	if err := ValidateUnitName(unit); err != nil {
		return "", err
	}
	out, err := runCombined(ctx, "systemctl", verb, unit)
	return string(out), err
}

// TailLogs starts "journalctl -u unit -f", sending each output line on the
// returned channel until ctx is cancelled or the process exits (the channel
// is then closed). The process is killed on ctx cancellation, mirroring how
// internal/daemon/exec kills its "sh -c" command.
func TailLogs(ctx context.Context, unit string) (<-chan string, error) {
	if err := ValidateUnitName(unit); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "journalctl", "-u", unit, "-f", "-n", "50", "--no-pager")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start journalctl: %w", err)
	}

	lines := make(chan string)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		_ = scanner.Err() // ctx cancellation kills the process mid-read; nothing to report
		_ = cmd.Wait()
	}()
	return lines, nil
}
