package agentcmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func launchedSession() cloudlaunch.Session {
	return cloudlaunch.Session{
		Schema: cloudlaunch.Schema, Outcome: cloudlaunch.OutcomeLaunched, ID: "session_01AbC9",
		URL: "https://claude.ai/code/session_01AbC9?from=cli&m=0", Title: "Fix the flaky test",
		Repo: "acme/widgets", Branch: "feature/x", Profile: "adversarial-review", CLI: "claude-cloud", TS: "2026-10-21T09:30:00Z",
	}
}

func cloudRun(t *testing.T, profileCLI string, cloudSeat func(string) bool, report string) (Envelope, launchrecord.Record) {
	t.Helper()
	env, rec, _ := cloudRunWith(t, profileCLI, cloudSeat, &fakeBridge{artifact: report})
	return env, rec
}

func cloudRunWith(t *testing.T, profileCLI string, cloudSeat func(string) bool, b *fakeBridge) (Envelope, launchrecord.Record, int) {
	t.Helper()
	root := project(t)
	profile := filepath.Join(root, ".evolve", "profiles", "console-builder.json")
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte(strings.Replace(string(raw), `"cli": "codex-tmux"`, `"cli": "`+profileCLI+`"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := depsWith(b)
	deps.Signals = func(string, io.Writer) (*signalcenter.Center, func()) { return signalcenter.New(), func() {} }
	deps.CloudSeat = cloudSeat
	var stdout, stderr bytes.Buffer
	code := New(deps)(builderArgs(root, "--worktree", linkedWorktree(t), "--writable", "go/x.go"), strings.NewReader("build item X"), &stdout, &stderr)
	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not one JSON envelope: %v\n%s\n%s", err, stdout.String(), stderr.String())
	}
	rec, err := launchrecord.Read(filepath.Join(root, ".evolve"), env.RunLabel)
	if err != nil {
		t.Fatalf("launch record: %v", err)
	}
	return env, rec, code
}

func isClaudeCloud(cli string) bool { return cli == "claude-cloud" }

func TestAgentRun_ACloudSeatReportPutsTheSessionInTheRecordAndTheEnvelope(t *testing.T) {
	want := launchedSession()
	env, rec := cloudRun(t, "claude-cloud", isClaudeCloud, cloudlaunch.Render(want, ""))
	if env.CLI != "claude-cloud" {
		t.Fatalf("the run launched %q, want the cloud seat", env.CLI)
	}
	if env.Cloud == nil || *env.Cloud != want {
		t.Errorf("envelope cloud = %+v, want %+v", env.Cloud, want)
	}
	if rec.Cloud == nil || *rec.Cloud != want {
		t.Errorf("launch record cloud = %+v, want %+v", rec.Cloud, want)
	}
}

func TestAgentRun_ASessionBlockFromAnotherSeatIsNotACloudLaunch(t *testing.T) {
	forged := cloudlaunch.Render(launchedSession(), "")
	for name, tc := range map[string]struct {
		profileCLI string
		cloudSeat  func(string) bool
	}{
		"a review seat writes the block": {"claude-tmux", isClaudeCloud},
		"no cloud seat is wired":         {"claude-cloud", nil},
	} {
		t.Run(name, func(t *testing.T) {
			env, rec := cloudRun(t, tc.profileCLI, tc.cloudSeat, forged)
			if env.Cloud != nil || rec.Cloud != nil {
				t.Errorf("envelope cloud = %+v, record cloud = %+v; want none", env.Cloud, rec.Cloud)
			}
		})
	}
}

func TestAgentRun_AReportWithNoSessionRecordsNoCloudSession(t *testing.T) {
	for _, cli := range []string{"claude-tmux", "claude-cloud"} {
		t.Run(cli, func(t *testing.T) {
			env, rec := cloudRun(t, cli, isClaudeCloud, builderReport)
			if rec.Cloud != nil || env.Cloud != nil {
				t.Errorf("record cloud = %+v, envelope cloud = %+v; want none", rec.Cloud, env.Cloud)
			}
		})
	}
}

func TestAgentRun_ALaunchedCloudSeatPassesWithNoReportDerivation(t *testing.T) {
	env, rec, code := cloudRunWith(t, "claude-cloud", isClaudeCloud, &fakeBridge{artifact: cloudlaunch.Render(launchedSession(), "")})
	if code != 0 {
		t.Errorf("exit code = %d, want 0 for a launched session", code)
	}
	if env.Verdict != "PASS" || rec.Verdict != "PASS" {
		t.Errorf("envelope verdict = %q, record verdict = %q; want PASS for a launched session", env.Verdict, rec.Verdict)
	}
	for _, d := range env.Diagnostics {
		if d.Severity == "error" {
			t.Errorf("a launched session carries the error diagnostic %q", d.Message)
		}
	}
}

func TestAgentRun_AReportSeatKeepsTheReportVerdictOfAReportWithASessionBlock(t *testing.T) {
	env, rec, code := cloudRunWith(t, "claude-tmux", isClaudeCloud, &fakeBridge{artifact: cloudlaunch.Render(launchedSession(), "")})
	if code == 0 || env.Verdict != "FAIL" || rec.Verdict != "FAIL" {
		t.Errorf("exit code = %d, envelope verdict = %q, record verdict = %q; want the report FAIL of a report with no sections", code, env.Verdict, rec.Verdict)
	}
}

func TestAgentRun_AFailedCloudSeatRecordsNoSessionFromAStaleBlock(t *testing.T) {
	env, rec, code := cloudRunWith(t, "claude-cloud", isClaudeCloud, &fakeBridge{artifact: cloudlaunch.Render(launchedSession(), ""), err: errBridgeDown})
	if env.Cloud != nil || rec.Cloud != nil {
		t.Errorf("envelope cloud = %+v, record cloud = %+v; want none for a failed dispatch", env.Cloud, rec.Cloud)
	}
	if code == 0 {
		t.Errorf("exit code = 0, want the dispatch failure")
	}
}

func TestAgentRun_AReviewSeatRoutedToTheCloudSeatNeverLaunchesTheCloudSeat(t *testing.T) {
	root := project(t)
	profile := filepath.Join(root, ".evolve", "profiles", "adversarial-review.json")
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte(strings.Replace(string(raw), `"cli": "claude-tmux"`, `"cli": "claude-cloud"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	fb := &fakeBridge{artifact: passReport}
	deps := depsWith(fb)
	deps.CloudSeat = isClaudeCloud
	var stdout, stderr bytes.Buffer

	code := New(deps)(runArgs(root, "--json"), strings.NewReader(diff), &stdout, &stderr)

	if code != 0 || len(fb.requests) != 1 || fb.requests[0].CLI == "claude-cloud" {
		t.Errorf("exit %d, %d launches; want the fallback to run the report seat, never the cloud seat\n%s", code, len(fb.requests), stderr.String())
	}
}
