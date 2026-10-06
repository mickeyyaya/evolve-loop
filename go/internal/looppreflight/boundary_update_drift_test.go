package looppreflight

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

func recordBoundaryUpdates(t *testing.T, evolveDir string, results ...cliupdate.Result) {
	t.Helper()
	rep := cliupdate.Report{Results: results}
	if err := cliupdate.Remember(evolveDir, rep, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func updated(family, from, to string) cliupdate.Result {
	return cliupdate.Result{Family: family, Status: cliupdate.StatusUpdated, OldVersion: from, NewVersion: to}
}

func TestVersionDrift_AChangeTheBoundaryUpdaterRecordedIsExpectedNotDrift(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.285"})
	recordBoundaryUpdates(t, opts.EvolveDir, updated("claude", "2.1.285", "2.1.286"))
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.286"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelPass {
		t.Fatalf("a recorded boundary update is expected, not drift; got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Message, "boundary updater") || !strings.Contains(c.Detail, "claude 2.1.285 → 2.1.286") {
		t.Errorf("the pass must say the boundary updater made the change: message=%q detail=%q", c.Message, c.Detail)
	}
}

func TestVersionDrift_AChangeTheBoundaryUpdaterDidNotRecordStillWarns(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.285", "agy": "1.0.20"})
	recordBoundaryUpdates(t, opts.EvolveDir, updated("claude", "2.1.285", "2.1.286"))
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.287", "agy": "1.0.21"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelWarn {
		t.Fatalf("a change no boundary update recorded is a self-update; it must WARN; got %s (%s)", c.Level, c.Detail)
	}
	for _, want := range []string{"claude changed: 2.1.285 → 2.1.287", "agy changed: 1.0.20 → 1.0.21", "not recorded by the boundary updater"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("drift detail missing %q: %q", want, c.Detail)
		}
	}
}

func TestVersionDrift_ExpectedAndUnrecordedChangesAreReportedApart(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.285", "agy": "1.0.20"})
	recordBoundaryUpdates(t, opts.EvolveDir, updated("claude", "2.1.285", "2.1.286"))
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.286", "agy": "1.0.21"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelWarn || !strings.HasPrefix(c.Message, "1 CLI(s) changed version") {
		t.Fatalf("only agy drifted; want a one-CLI WARN, got %s %q", c.Level, c.Message)
	}
	if !strings.Contains(c.Detail, "agy changed: 1.0.20 → 1.0.21") || !strings.Contains(c.Detail, "expected (boundary update): claude 2.1.285 → 2.1.286") {
		t.Errorf("detail must separate the drift from the expected update: %q", c.Detail)
	}
}

func TestVersionDrift_AnUnreadableUpdateRecordLeavesEveryChangeAsDrift(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.285"})
	if err := os.WriteFile(cliupdate.RecordsPath(opts.EvolveDir), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.286"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelWarn || !strings.Contains(c.Detail, "boundary update record unreadable") {
		t.Fatalf("an unreadable record explains nothing and says so; got %s (%s)", c.Level, c.Detail)
	}
}

func TestRun_VersionFreeze_ABoundaryUpdateRecordDoesNotUnfreezeASelfUpdater(t *testing.T) {
	opts := freezeOptions(t, map[string]string{"codex": "~/.codex/version.json present (updater state)"}, nil, nil)
	recordBoundaryUpdates(t, opts.EvolveDir, updated("codex", "0.153.4", "0.154.0"))

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCheck(t, r, "cli-version-freeze"); c.Level != LevelHalt {
		t.Fatalf("the freeze guards against a mid-wave self-update; a boundary record must not relax it; got %s (%s)", c.Level, c.Detail)
	}
}

func TestCLIVersion_ReadsTheFirstVersionToken(t *testing.T) {
	orig := execVersion
	t.Cleanup(func() { execVersion = orig })
	execVersion = func(bin string) (string, error) {
		switch bin {
		case "claude":
			return "2.1.286 (Claude Code)", nil
		case "silent":
			return "no version here", nil
		}
		return "", errors.New("exec: not found")
	}

	if v, err := CLIVersion("claude"); err != nil || v != "2.1.286" {
		t.Errorf("CLIVersion(claude) = %q, %v; want 2.1.286", v, err)
	}
	if _, err := CLIVersion("silent"); err == nil || !strings.Contains(err.Error(), "no version token") {
		t.Errorf("a probe without a version token is an error, got %v", err)
	}
	if _, err := CLIVersion("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("a failed probe is an error, got %v", err)
	}
}

func TestVersionDrift_ASelfUpdateTheBoundarySmokeBootedIsReportedAsSelfUpdatedSmokeOK(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"agy": "1.2.16", "claude": "2.1.285"})
	recordBoundaryUpdates(t, opts.EvolveDir,
		cliupdate.Result{Family: "agy", Status: cliupdate.StatusSelfUpdated, SelfUpdatedFrom: "1.2.16", OldVersion: "1.2.17", NewVersion: "1.2.17"},
		updated("claude", "2.1.285", "2.1.286"))
	opts.VersionInventory = func() map[string]string { return map[string]string{"agy": "1.2.17", "claude": "2.1.286"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelWarn || !strings.Contains(c.Message, "self-updated outside a boundary") {
		t.Fatalf("a self-update means the freeze leaked; it warns, named as absorbed; got %s %q", c.Level, c.Message)
	}
	for _, want := range []string{"agy self-updated, smoke OK: 1.2.16 → 1.2.17", "expected (boundary update): claude 2.1.285 → 2.1.286"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail missing %q: %q", want, c.Detail)
		}
	}
	if strings.Contains(c.Detail, unrecordedDriftNote) {
		t.Errorf("a smoke-booted self-update is not unrecorded drift: %q", c.Detail)
	}
}

func TestVersionDrift_ARecordOlderThanTheBaselineExplainsNothing(t *testing.T) {
	opts := goodPipelineOptions(t)
	if err := cliupdate.Remember(opts.EvolveDir, cliupdate.Report{Results: []cliupdate.Result{updated("claude", "2.1.285", "2.1.286")}}, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.285"})
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.286"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelWarn || !strings.Contains(c.Detail, "claude changed: 2.1.285 → 2.1.286") {
		t.Fatalf("the baseline (2.1.285, after a rollback) is newer than the 2.1.285 → 2.1.286 record, so that record cannot explain today's change; got %s (%s)", c.Level, c.Detail)
	}
}
