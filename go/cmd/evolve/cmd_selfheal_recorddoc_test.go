package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	recordDocIncident = "inc-2026-10-26-0a1b2c3d"
	recordDocPath     = "docs/incidents/2026-10-26-flaky-rerun.md"
)

func recordDocProject(t *testing.T, doc string) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := paths.EvolveDirOf(root)
	selfhealFile(t, filepath.Join(evolveDir, paths.PolicyFile), `{}`)
	inc := selfheal.Incident{
		Schema: selfheal.RecordSchema, ID: recordDocIncident, Fingerprint: "flaky-test:other", Class: selfheal.ClassFlakyTest, State: selfheal.StateOpen,
		Sources: []string{}, OpenedAt: selfhealNow.Add(-20 * time.Hour), LastSeenAt: selfhealNow.Add(-15 * time.Hour),
		ReproTest: "go/x/x_test.go:TestRepro", RecordDoc: doc,
	}
	if err := selfheal.WriteIncident(selfheal.Dir(evolveDir), inc); err != nil {
		t.Fatal(err)
	}
	selfhealFile(t, filepath.Join(root, "go/x/x_test.go"), "package x\n\nfunc TestRepro(t *testing.T) {}\n")
	selfhealFile(t, filepath.Join(root, "docs/retrospectives/2026-10-26-flaky.md"), "Recovered "+recordDocIncident+".\n")
	ended := selfhealNow.Add(-5 * time.Hour)
	if err := wave.NewStore(evolveDir).Save(wave.Record{Number: 1, StartedAt: selfhealNow.Add(-10 * time.Hour), EndedAt: &ended}); err != nil {
		t.Fatal(err)
	}
	return root
}

func greenMainGH(context.Context, string, ...string) (string, error) {
	return `[{"databaseId":9,"headBranch":"main","status":"completed","conclusion":"success","attempt":1,"createdAt":"2026-10-27T00:00:00Z","updatedAt":"2026-10-27T00:20:00Z"}]`, nil
}

func incidentState(t *testing.T, root string) selfheal.State {
	t.Helper()
	records, err := selfheal.LoadIncidents(selfheal.Dir(paths.EvolveDirOf(root)))
	if err != nil || len(records) != 1 {
		t.Fatalf("records: %v %+v", err, records)
	}
	return records[0].State
}

func TestSelfhealIncidents_ClosesAnIncidentOnlyWhenItsRecordDocIsInTheTree(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, doc string
		writeDoc  bool
		want      selfheal.State
	}{
		{"the document exists", recordDocPath, true, selfheal.StateClosed},
		{"the document was never merged", recordDocPath, false, selfheal.StateOpen},
		{"no document was set", "", false, selfheal.StateOpen},
		{"the path leaves the tree", "../outside.md", false, selfheal.StateOpen},
	}
	for _, c := range cases {
		root := recordDocProject(t, c.doc)
		if c.writeDoc {
			selfhealFile(t, filepath.Join(root, filepath.FromSlash(c.doc)), "# record\n")
		}
		rc, e, stderr := runSelfhealJSON(t, selfhealTestEnv(greenMainGH), root)
		if rc != 0 || len(e.Warnings) != 0 || incidentState(t, root) != c.want {
			t.Errorf("%s: rc=%d warnings=%v state=%s want %s stderr=%s", c.name, rc, e.Warnings, incidentState(t, root), c.want, stderr)
		}
	}
}

func runRecordDoc(root string, operands ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	args := append([]string{"record-doc"}, operands...)
	rc := runSelfhealWith(selfhealTestEnv(greenMainGH), append(args, "--project-root", root), &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestSelfhealRecordDoc_StoresThePathOnTheIncidentFile(t *testing.T) {
	t.Parallel()
	root := recordDocProject(t, "")
	rc, stdout, stderr := runRecordDoc(root, recordDocIncident, recordDocPath)
	if rc != 0 || !strings.Contains(stdout, recordDocIncident) {
		t.Fatalf("rc=%d stdout=%s stderr=%s", rc, stdout, stderr)
	}
	raw, err := os.ReadFile(filepath.Join(selfheal.Dir(paths.EvolveDirOf(root)), recordDocIncident+".json"))
	if err != nil || !strings.Contains(string(raw), `"record_doc": "`+recordDocPath+`"`) {
		t.Fatalf("the file on disk must carry the path: %v\n%s", err, raw)
	}
}

func TestSelfhealRecordDoc_RefusesWhatItCannotStore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		operands []string
		want     int
	}{
		{"no operands", nil, exitUsage},
		{"one operand", []string{recordDocIncident}, exitUsage},
		{"three operands", []string{recordDocIncident, recordDocPath, "x"}, exitUsage},
		{"an absolute path", []string{recordDocIncident, "/etc/passwd"}, selfhealExitIn},
		{"a path that escapes", []string{recordDocIncident, "docs/incidents/../../go.mod"}, selfhealExitIn},
		{"outside the incident directory", []string{recordDocIncident, "README.md"}, selfhealExitIn},
		{"not a markdown file", []string{recordDocIncident, "docs/incidents/2026-10-26-x.txt"}, selfhealExitIn},
		{"a file name that is no record id", []string{recordDocIncident, "docs/incidents/Notes.md"}, selfhealExitIn},
		{"a session log is no record", []string{recordDocIncident, "docs/incidents/2026-11-20-session-log.md"}, selfhealExitIn},
		{"a nested directory", []string{recordDocIncident, "docs/incidents/sub/2026-10-26-x.md"}, selfhealExitIn},
		{"an unknown incident", []string{"inc-2026-10-26-ffffffff", recordDocPath}, selfhealExitIn},
	}
	for _, c := range cases {
		root := recordDocProject(t, "")
		rc, _, stderr := runRecordDoc(root, c.operands...)
		records, _ := selfheal.LoadIncidents(selfheal.Dir(paths.EvolveDirOf(root)))
		if rc != c.want || stderr == "" || records[0].RecordDoc != "" {
			t.Errorf("%s: rc=%d want %d stderr=%q recordDoc=%q", c.name, rc, c.want, stderr, records[0].RecordDoc)
		}
	}
}

func TestSelfhealRecordDoc_AnUnreadableIncidentStoreIsAnIOFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	selfhealFile(t, filepath.Join(paths.EvolveDirOf(root), paths.PolicyFile), `{}`)
	selfhealFile(t, filepath.Join(selfheal.Dir(paths.EvolveDirOf(root)), "inc-x.json"), "{")
	rc, _, stderr := runRecordDoc(root, recordDocIncident, recordDocPath)
	if rc != exitIO || stderr == "" {
		t.Fatalf("rc=%d stderr=%s", rc, stderr)
	}
}
