//go:build integration

package cyclesimulator

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallDefaultSeams_ShipWithoutBashIsExitOneAndTheLaunchError(t *testing.T) {
	in := &Inputs{PluginRoot: t.TempDir()}
	in.installDefaultSeams()
	t.Setenv("PATH", t.TempDir())

	rc, err := in.ShipDryRunFn("msg")

	if rc != 1 || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("ShipDryRunFn = (%d, %v), want (1, exec.ErrNotFound)", rc, err)
	}
}

func TestInstallDefaultSeams_ShellOutsCarryTheirArgumentsAndExitCodes(t *testing.T) {
	pluginRoot := t.TempDir()
	record := filepath.Join(t.TempDir(), "argv")
	body := "#!/bin/bash\nprintf '%s|' \"$(basename \"$0\")\" \"$@\" >> '" + record + "'\necho >> '" + record + "'\nexit 4\n"
	for _, rel := range []string{"lifecycle/cycle-state.sh", "lifecycle/ship.sh", "observability/verify-ledger-chain.sh"} {
		writeScript(t, pluginRoot, rel, body)
	}
	in := &Inputs{PluginRoot: pluginRoot}
	in.installDefaultSeams()

	advErr := in.AdvanceFn("build", "builder")
	rc, shipErr := in.ShipDryRunFn("dry run message")
	verErr := in.VerifyFn()

	var exitErr *exec.ExitError
	if !errors.As(advErr, &exitErr) || exitErr.ExitCode() != 4 {
		t.Fatalf("AdvanceFn err = %v, want exit 4", advErr)
	}
	if rc != 4 || !errors.As(shipErr, &exitErr) {
		t.Fatalf("ShipDryRunFn = (%d, %v), want (4, the exit error)", rc, shipErr)
	}
	if !errors.As(verErr, &exitErr) || exitErr.ExitCode() != 4 {
		t.Fatalf("VerifyFn err = %v, want exit 4", verErr)
	}
	got, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	want := "cycle-state.sh|advance|build|builder|\nship.sh|--dry-run|dry run message|\nverify-ledger-chain.sh|\n"
	if string(got) != want {
		t.Fatalf("recorded argv = %q, want %q", got, want)
	}
}

func TestInstallDefaultSeams_KeepsInjectedSeams(t *testing.T) {
	var calls []string
	in := &Inputs{
		PluginRoot:   t.TempDir(),
		AdvanceFn:    func(p, a string) error { calls = append(calls, "advance"); return nil },
		ShipDryRunFn: func(string) (int, error) { calls = append(calls, "ship"); return 9, nil },
		VerifyFn:     func() error { calls = append(calls, "verify"); return nil },
	}
	t.Setenv("PATH", t.TempDir())

	in.installDefaultSeams()
	_ = in.AdvanceFn("p", "a")
	rc, _ := in.ShipDryRunFn("m")
	_ = in.VerifyFn()

	if rc != 9 || len(calls) != 3 || calls[0] != "advance" || calls[1] != "ship" || calls[2] != "verify" {
		t.Fatalf("calls = %v rc = %d, want the injected seams untouched", calls, rc)
	}
}
