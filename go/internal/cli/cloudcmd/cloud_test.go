package cloudcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

func session(id, ts string) *cloudlaunch.Session {
	return &cloudlaunch.Session{
		Schema: cloudlaunch.Schema, Outcome: cloudlaunch.OutcomeLaunched, ID: id, URL: "https://claude.ai/code/" + id,
		Title: "Fix " + id, Repo: "acme/widgets", Branch: "main", CLI: "claude-cloud", TS: ts,
	}
}

func projectWithRecords(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	records := []launchrecord.Record{
		{RunID: "agent-builder-2", Profile: "builder", CLI: "claude-cloud", TS: "2026-11-03T11:00:00Z", Cloud: session("session_02", "2026-11-03T11:00:00Z")},
		{RunID: "agent-review-1", Profile: "go-reviewer", CLI: "claude-tmux", TS: "2026-11-03T10:30:00Z"},
		{RunID: "agent-builder-1", Profile: "builder", CLI: "claude-cloud", TS: "2026-11-03T10:00:00Z", Cloud: session("session_01", "2026-11-03T10:00:00Z")},
	}
	for _, r := range records {
		if _, err := launchrecord.Write(evolveDir, r); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func run(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRun_ListPrintsEachCloudSessionInTimeOrder(t *testing.T) {
	t.Parallel()
	root := projectWithRecords(t)
	code, out, errOut := run("list", "--project-root", root)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errOut)
	}
	want := "session_01\thttps://claude.ai/code/session_01\tFix session_01\t2026-11-03T10:00:00Z\n" +
		"session_02\thttps://claude.ai/code/session_02\tFix session_02\t2026-11-03T11:00:00Z\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestRun_ListJSONIsOneArrayOfSessions(t *testing.T) {
	t.Parallel()
	root := projectWithRecords(t)
	code, out, _ := run("list", "--json", "--project-root", root)
	var got []cloudlaunch.Session
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 {
		t.Fatalf("exit %d, stdout is not a JSON array: %v\n%s", code, err, out)
	}
	if len(got) != 2 || got[0] != *session("session_01", "2026-11-03T10:00:00Z") || got[1].ID != "session_02" {
		t.Errorf("sessions = %+v", got)
	}
	_, empty, _ := run("list", "--json", "--project-root", t.TempDir())
	if strings.TrimSpace(empty) != "[]" {
		t.Errorf("no records = %q, want []", empty)
	}
}

func TestRun_ABadRecordFailsTheList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", launchrecord.Subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent-x-1.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run("list", "--project-root", root)
	if code != exitFailed || !strings.Contains(errOut, "agent-x-1") {
		t.Errorf("exit %d stderr %q, want %d and the bad record", code, errOut, exitFailed)
	}
}

func TestRun_UsageErrorsAndHelp(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"launch"}, {"list", "--bogus"}, {"list", "extra"}} {
		if code, _, errOut := run(args...); code != exitUsage || errOut == "" {
			t.Errorf("Run(%q) = %d, %q; want exit %d and a reason", args, code, errOut, exitUsage)
		}
	}
	for _, help := range []string{"help", "-h", "--help"} {
		if code, out, _ := run(help); code != 0 || !strings.Contains(out, "evolve cloud list") {
			t.Errorf("Run(%s) = %d, %q; want the usage", help, code, out)
		}
	}
}

func TestRun_ListWithNoRootFlagReadsTheProjectRootOfTheShell(t *testing.T) {
	root := projectWithRecords(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if code, out, errOut := run("list"); code != 0 || !strings.Contains(out, "session_02") {
		t.Errorf("Run(list) = %d, %q, %q; want the sessions of the root", code, out, errOut)
	}
}
