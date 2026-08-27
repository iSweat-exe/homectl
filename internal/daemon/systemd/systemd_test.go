package systemd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateUnitName(t *testing.T) {
	valid := []string{"nginx.service", "docker.service", "my-app@1.service", "cron.service"}
	invalid := []string{"", "nginx", "nginx.socket", "-x.service", "../etc/passwd.service", ".service"}

	for _, unit := range valid {
		if err := ValidateUnitName(unit); err != nil {
			t.Errorf("ValidateUnitName(%q): expected nil, got %v", unit, err)
		}
	}
	for _, unit := range invalid {
		if err := ValidateUnitName(unit); err == nil {
			t.Errorf("ValidateUnitName(%q): expected an error, got nil", unit)
		}
	}
}

func TestList(t *testing.T) {
	origOutput := runOutput
	defer func() { runOutput = origOutput }()

	const unitsJSON = `[
		{"unit":"nginx.service","load":"loaded","active":"active","sub":"running","description":"A high performance web server"},
		{"unit":"cron.service","load":"loaded","active":"inactive","sub":"dead","description":"Regular background program processing daemon"}
	]`
	const unitFilesJSON = `[
		{"unit_file":"nginx.service","state":"enabled"},
		{"unit_file":"cron.service","state":"disabled"}
	]`

	runOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "systemctl" {
			t.Fatalf("unexpected command %q", name)
		}
		switch args[0] {
		case "list-units":
			return []byte(unitsJSON), nil
		case "list-unit-files":
			return []byte(unitFilesJSON), nil
		default:
			t.Fatalf("unexpected systemctl subcommand %q", args[0])
			return nil, nil
		}
	}

	units, err := List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}

	byName := make(map[string]Unit, len(units))
	for _, u := range units {
		byName[u.Name] = u
	}

	nginx, ok := byName["nginx.service"]
	if !ok {
		t.Fatal("missing nginx.service in result")
	}
	if nginx.ActiveState != "active" || nginx.SubState != "running" || nginx.UnitFileState != "enabled" {
		t.Errorf("nginx.service: unexpected merged state %+v", nginx)
	}

	cron, ok := byName["cron.service"]
	if !ok {
		t.Fatal("missing cron.service in result")
	}
	if cron.ActiveState != "inactive" || cron.UnitFileState != "disabled" {
		t.Errorf("cron.service: unexpected merged state %+v", cron)
	}
}

func TestList_UnitsCommandError(t *testing.T) {
	origOutput := runOutput
	defer func() { runOutput = origOutput }()

	runOutput = func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("systemctl not found")
	}

	if _, err := List(context.Background()); err == nil {
		t.Fatal("expected an error when systemctl list-units fails")
	}
}

func TestAction_RejectsInvalidUnitName(t *testing.T) {
	origCombined := runCombined
	defer func() { runCombined = origCombined }()

	runCombined = func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runCombined should not be called for an invalid unit name")
		return nil, nil
	}

	if _, err := Start(context.Background(), "not-a-service"); err == nil {
		t.Fatal("expected Start to reject an invalid unit name")
	}
}

func TestAction_Success(t *testing.T) {
	origCombined := runCombined
	defer func() { runCombined = origCombined }()

	var gotArgs []string
	runCombined = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "systemctl" {
			t.Fatalf("unexpected command %q", name)
		}
		gotArgs = args
		return []byte("ok\n"), nil
	}

	out, err := Restart(context.Background(), "nginx.service")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if out != "ok\n" {
		t.Errorf("expected output %q, got %q", "ok\n", out)
	}
	if want := []string{"restart", "nginx.service"}; len(gotArgs) != 2 || gotArgs[0] != want[0] || gotArgs[1] != want[1] {
		t.Errorf("expected args %v, got %v", want, gotArgs)
	}
}

func TestAction_Failure(t *testing.T) {
	origCombined := runCombined
	defer func() { runCombined = origCombined }()

	runCombined = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("Unit foo.service not found.\n"), errors.New("exit status 5")
	}

	out, err := Stop(context.Background(), "foo.service")
	if err == nil {
		t.Fatal("expected an error to be propagated")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected combined output to be returned even on failure, got %q", out)
	}
}

func TestTailLogs_RejectsInvalidUnitName(t *testing.T) {
	if _, err := TailLogs(context.Background(), "nginx"); err == nil {
		t.Fatal("expected TailLogs to reject an invalid unit name")
	}
}
