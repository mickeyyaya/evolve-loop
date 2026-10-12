package dashboard

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func TestListArtifacts_ListsOnlyRegularAllowedFilesSorted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 7)
	writeFile(t, filepath.Join(ws, "zeta.md"), "z")
	writeFile(t, filepath.Join(ws, "alpha.json"), "{}")
	writeFile(t, filepath.Join(ws, "binary.exe"), "x")
	writeFile(t, filepath.Join(ws, ".hidden.md"), "x")
	if err := os.MkdirAll(filepath.Join(ws, "sub.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(ws, "zeta.md"), filepath.Join(ws, "link.md")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	got, err := ListArtifacts(root, 7)

	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, a := range got {
		names = append(names, a.Name)
	}
	if strings.Join(names, ",") != "alpha.json,zeta.md" || got[0].Size != 2 || got[1].Size != 1 {
		t.Fatalf("artifacts = %+v", got)
	}
}

func TestListArtifacts_ReportsAMissingWorkspaceAsNotExist(t *testing.T) {
	t.Parallel()

	got, err := ListArtifacts(t.TempDir(), 7)

	if !errors.Is(err, os.ErrNotExist) || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

type fakeArtifactF3 struct {
	io.Reader
	info    os.FileInfo
	statErr error
	closed  *bool
}

func (f fakeArtifactF3) Stat() (os.FileInfo, error) { return f.info, f.statErr }
func (f fakeArtifactF3) Close() error {
	*f.closed = true
	return nil
}

func TestReadArtifactVia_EveryDefenceAfterTheLstatRefusesAndClosesTheFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 5)
	writeFile(t, filepath.Join(ws, "report.md"), "real")
	writeFile(t, filepath.Join(ws, "other.md"), "other")
	otherInfo, err := os.Lstat(filepath.Join(ws, "other.md"))
	if err != nil {
		t.Fatal(err)
	}
	openErr := errors.New("open denied")
	statErr := errors.New("stat failed")

	cases := []struct {
		name       string
		open       func(closed *bool) func(string) (artifactFile, error)
		wantErr    error
		wantClosed bool
	}{
		{name: "open fails", open: func(*bool) func(string) (artifactFile, error) {
			return func(string) (artifactFile, error) { return nil, openErr }
		}, wantErr: openErr},
		{name: "stat fails", open: func(closed *bool) func(string) (artifactFile, error) {
			return func(string) (artifactFile, error) {
				return fakeArtifactF3{Reader: strings.NewReader("x"), statErr: statErr, closed: closed}, nil
			}
		}, wantErr: statErr, wantClosed: true},
		{name: "the opened file is not the file that was checked", open: func(closed *bool) func(string) (artifactFile, error) {
			return func(string) (artifactFile, error) {
				return fakeArtifactF3{Reader: strings.NewReader("swapped"), info: otherInfo, closed: closed}, nil
			}
		}, wantErr: ErrArtifactNotAllowed, wantClosed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			closed := false

			got, err := readArtifactVia(root, 5, "report.md", tc.open(&closed))

			if !errors.Is(err, tc.wantErr) || got != nil || closed != tc.wantClosed {
				t.Fatalf("got=%q err=%v closed=%v", got, err, closed)
			}
		})
	}
}

func TestOpenArtifactNoFollow_ANilInterfaceOnErrorAndAnOpenFileOnSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "body")

	missing, err := openArtifactNoFollow(filepath.Join(dir, "absent.md"))
	if err == nil || missing != nil {
		t.Fatalf("a failed open must return a nil interface, got %v %v", missing, err)
	}
	f, err := openArtifactNoFollow(filepath.Join(dir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if body, _ := io.ReadAll(f); string(body) != "body" {
		t.Fatalf("body = %q", body)
	}
}

func TestReadArtifact_AnUnreadableRegularFileSurfacesTheOpenError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root opens a mode-000 file")
	}
	root := t.TempDir()
	path := filepath.Join(core.RunWorkspacePath(root, 5), "locked.md")
	writeFile(t, path, "secret")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	got, err := ReadArtifact(root, 5, "locked.md")

	if !errors.Is(err, os.ErrPermission) || got != nil {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestCollect_AnUnreadableRunsDirectoryIsAWarningNotAFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "runs"), "a file where the directory belongs")

	snap := Collect(root, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	runsWarnings := 0
	for _, w := range snap.Warnings {
		if strings.HasPrefix(w, "runs ") {
			runsWarnings++
		}
	}
	if runsWarnings != 2 || snap.Cycles == nil || len(snap.Cycles) != 0 {
		t.Fatalf("one runs warning from the status read and one from the cycle selection: warnings=%v cycles=%v", snap.Warnings, snap.Cycles)
	}
}

func TestReadAuditRounds_FailuresBecomeWarningsAndAMissingWorkspaceIsSilent(t *testing.T) {
	t.Parallel()
	t.Run("workspace is a file", func(t *testing.T) {
		t.Parallel()
		ws := filepath.Join(t.TempDir(), "cycle-3")
		writeFile(t, ws, "not a directory")

		rounds, warns := readAuditRounds(ws)

		if rounds != nil || len(warns) != 1 || !strings.HasPrefix(warns[0], "audit rounds cycle-3: ") {
			t.Fatalf("rounds=%v warns=%v", rounds, warns)
		}
	})
	t.Run("the live report is unreadable", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		if err := os.MkdirAll(filepath.Join(ws, auditReportName), 0o755); err != nil {
			t.Fatal(err)
		}

		rounds, warns := readAuditRounds(ws)

		if len(rounds) != 0 || len(warns) != 1 || !strings.HasPrefix(warns[0], auditReportName+": ") {
			t.Fatalf("rounds=%v warns=%v", rounds, warns)
		}
	})
	t.Run("missing workspace", func(t *testing.T) {
		t.Parallel()

		rounds, warns := readAuditRounds(filepath.Join(t.TempDir(), "absent"))

		if rounds != nil || warns != nil {
			t.Fatalf("rounds=%v warns=%v", rounds, warns)
		}
	})
}

func TestComputeFingerprints_SkipsOpenFailuresWithoutAFingerprintAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	ds := map[int]*dossier.Dossier{
		1: {Cycle: 1, FinalVerdict: dossier.VerdictFail},
		2: {Cycle: 2, FinalVerdict: dossier.VerdictFail, Failure: &dossier.FailureRecord{PreClass: "gate-block"}},
		3: {Cycle: 3, FinalVerdict: dossier.VerdictFail, Failure: &dossier.FailureRecord{Fingerprint: "fp-a", PreClass: "gate-block", Reasons: []string{"why"}}},
		4: {Cycle: 4, FinalVerdict: dossier.VerdictPass, CommitSHA: "abc"},
		5: {Cycle: 5, FinalVerdict: dossier.VerdictFail, Failure: &dossier.FailureRecord{Fingerprint: "fp-a", PreClass: "gate-block"}},
	}

	got := computeFingerprints([]int{1, 2, 3, 4, 5}, ds)

	if len(got) != 1 {
		t.Fatalf("a failure without a fingerprint is no fingerprint: %+v", got)
	}
	fp := got[0]
	if fp.Fingerprint != "fp-a" || fp.Count != 2 || fp.FirstCycle != 3 || fp.LastCycle != 5 || !fp.Regressed || fp.Reason != "why" {
		t.Fatalf("stat = %+v", fp)
	}
}

func TestComputeFingerprints_NothingFailedMeansNoStats(t *testing.T) {
	t.Parallel()

	if got := computeFingerprints(nil, nil); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestScanStream_AMissingStreamIsNoOutcomesAndNoWarning(t *testing.T) {
	t.Parallel()
	gates := map[string]bool{}

	outcomes, warn := scanStream(filepath.Join(t.TempDir(), "absent.ndjson"), gates)

	if outcomes != nil || warn != "" || len(gates) != 0 {
		t.Fatalf("outcomes=%v warn=%q gates=%v", outcomes, warn, gates)
	}
}

func TestReadQueue_AnUnreadableInboxIsOneWarningAndAnEmptySummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "inbox"), "a file where the inbox belongs")

	q, warns := readQueue(root, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	if len(q.Pending) != 0 || q.Consumed != 0 || len(warns) != 1 || !strings.HasPrefix(warns[0], "inbox: ") {
		t.Fatalf("q=%+v warns=%v", q, warns)
	}
}

func TestServerAllowHost_IgnoresAnEmptyAddressAndAdmitsANamedHostWithoutItsPort(t *testing.T) {
	t.Parallel()
	s := New(t.TempDir(), Options{})

	s.allowHost("")
	s.allowHost("Dash.Example:8080")

	if s.hostAllowed("") {
		t.Fatal("an empty host must stay refused")
	}
	if !s.hostAllowed("dash.example:1") || !s.hostAllowed("DASH.example") {
		t.Fatal("the bound host is allowed on any port and in any case")
	}
	if s.hostAllowed("evil.example") {
		t.Fatal("an unbound host stays refused")
	}
	s.hostMu.RLock()
	defer s.hostMu.RUnlock()
	if len(s.boundHosts) != 1 {
		t.Fatalf("boundHosts = %v", s.boundHosts)
	}
}

func TestHandleCycle_AnUnlistableWorkspaceIsAWarningAndAFailedCycleOpensOnItsAuditReport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDossier(t, root, failDossier(2, "audit|gate-block|aaaa"))
	writeDossier(t, root, passDossier(3))
	writeFile(t, core.RunWorkspacePath(root, 3), "a file where the workspace belongs")
	s := New(root, Options{})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, body := get(t, ts.URL+"/api/cycle/3")
	var passed cycleDetail
	if resp.StatusCode != 200 || json.Unmarshal([]byte(body), &passed) != nil {
		t.Fatalf("cycle 3: %d %s", resp.StatusCode, body)
	}
	found := false
	for _, w := range passed.Warnings {
		found = found || strings.HasPrefix(w, "artifacts: ")
	}
	if !found || passed.PrimaryReport != buildReportName {
		t.Fatalf("warnings=%v primary=%q", passed.Warnings, passed.PrimaryReport)
	}

	resp, body = get(t, ts.URL+"/api/cycle/2")
	var failed cycleDetail
	if resp.StatusCode != 200 || json.Unmarshal([]byte(body), &failed) != nil {
		t.Fatalf("cycle 2: %d %s", resp.StatusCode, body)
	}
	if failed.PrimaryReport != auditReportName || failed.Cycle.Failure == nil {
		t.Fatalf("a failed cycle opens on the audit report: %+v", failed)
	}
}
