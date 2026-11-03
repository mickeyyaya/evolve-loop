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
	root := project(t)
	profile := filepath.Join(root, ".evolve", "profiles", "adversarial-review.json")
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte(strings.Replace(string(raw), `"cli": "claude-tmux"`, `"cli": "`+profileCLI+`"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := depsWith(&fakeBridge{artifact: report})
	deps.Signals = func(string, io.Writer) (*signalcenter.Center, func()) { return signalcenter.New(), func() {} }
	deps.CloudSeat = cloudSeat
	var stdout, stderr bytes.Buffer
	New(deps)(runArgs(root, "--json"), strings.NewReader(diff), &stdout, &stderr)
	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not one JSON envelope: %v\n%s\n%s", err, stdout.String(), stderr.String())
	}
	rec, err := launchrecord.Read(filepath.Join(root, ".evolve"), env.RunLabel)
	if err != nil {
		t.Fatalf("launch record: %v", err)
	}
	return env, rec
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
			env, rec := cloudRun(t, cli, isClaudeCloud, passReport)
			if rec.Cloud != nil || env.Cloud != nil {
				t.Errorf("record cloud = %+v, envelope cloud = %+v; want none", rec.Cloud, env.Cloud)
			}
		})
	}
}
