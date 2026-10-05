package acsrunner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestParseTestJSON_AStreamWithNoPredicateIsTheNoPredicatesHarnessRed(t *testing.T) {
	stream := `{"Action":"build-output","Package":"x/acs/cycle9","Output":"./p_test.go:1: syntax error\n"}` + "\n" +
		`{"Action":"fail","Package":"x/acs/cycle9"}` + "\n"
	v, err := ParseTestJSON(strings.NewReader(stream), 9)
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 1 || !reflect.DeepEqual(v.RedIDs, []string{"egps/no-predicates"}) || v.Total != 1 {
		t.Fatalf("red_count=%d red_ids=%v total=%d, want the one egps/no-predicates red: a FAIL with red_count 0 is the ambiguous shape ADR-0114 rejects", v.RedCount, v.RedIDs, v.Total)
	}
	if v.ShipEligible || v.Verdict != "FAIL" {
		t.Fatalf("ship_eligible=%v verdict=%q, want a FAIL that cannot ship", v.ShipEligible, v.Verdict)
	}
	if p := v.Predicates[0]; p.Verdict != "FAIL" || p.Output == "" {
		t.Errorf("harness red = %+v, want a FAIL predicate whose output says why", p)
	}
}

func TestNoPredicates_BothVerdictWritersNameTheSameHarnessRed(t *testing.T) {
	suite, err := acssuite.Run(acssuite.Options{Root: t.TempDir(), Cycle: 9})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := ParseTestJSON(strings.NewReader(""), 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.RedIDs) != 1 || !reflect.DeepEqual(runner.RedIDs, suite.RedIDs) {
		t.Fatalf("acsrunner red_ids=%v, acssuite red_ids=%v: the two writers of acs-verdict.json must name a run with no predicates the same way", runner.RedIDs, suite.RedIDs)
	}
}

func TestRun_AnExitNoPredicateExplainsIsARunnerErrorBesideTheHarnessRed(t *testing.T) {
	withCommander(func(context.Context, ...string) (io.ReadCloser, func() error, error) {
		return &closingReader{strings.NewReader("")}, func() error { return errors.New("exit status 1") }, nil
	}, func() {
		v, err := Run(context.Background(), 9, "./acs/cycle9")
		if err == nil {
			t.Fatal("Run = nil error: a go test that exited nonzero with no predicate result could not execute, so the CLI must exit 1")
		}
		if !reflect.DeepEqual(v.RedIDs, []string{"egps/no-predicates"}) {
			t.Errorf("red_ids=%v, want the no-predicates harness red in the verdict the CLI still writes", v.RedIDs)
		}
	})
}

func TestWriteVerdict_RefusesARelativeEvolveDir(t *testing.T) {
	const relative = "acsrunner-relative-evolve-dir-must-not-exist"
	t.Cleanup(func() { _ = os.RemoveAll(relative) })
	if _, err := WriteVerdict(relative, Verdict{SchemaVersion: "1.0", Cycle: 1}); err == nil {
		t.Fatal("WriteVerdict accepted a relative evolve dir; it would land wherever the process cwd happens to be")
	}
	if _, err := os.Stat(relative); !os.IsNotExist(err) {
		t.Errorf("a refused write must create nothing under the cwd (stat err=%v)", err)
	}
}

func TestExecCommand_ScrubsTheLaneIPCEnvironment(t *testing.T) {
	if _, err := exec.LookPath("env"); err != nil {
		t.Skip("env is not on PATH")
	}
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.CycleStateFileKey, "/plane/.evolve/runs/cycle-1700/cycle-state.json")
	stdout, wait, err := execCommand(context.Background(), "env")
	if err != nil {
		t.Fatal(err)
	}
	out, readErr := io.ReadAll(stdout)
	if err := wait(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, key := range []string{ipcenv.FleetKey, ipcenv.CycleStateFileKey} {
		if strings.Contains(string(out), key+"=") {
			t.Errorf("%s reached the predicate process; a lane's IPC state must not flip env-sensitive predicates", key)
		}
	}
	if !strings.Contains(string(out), "PATH=") {
		t.Error("the toolchain environment (PATH) must survive the scrub")
	}
}
