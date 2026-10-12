package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
)

func e3StubCollect(t *testing.T, snap *dashboard.Snapshot) *[]string {
	t.Helper()
	var roots []string
	prev := statusCollect
	t.Cleanup(func() { statusCollect = prev })
	statusCollect = func(root string, _ time.Time) *dashboard.Snapshot {
		roots = append(roots, root)
		return snap
	}
	return &roots
}

func e3NoGH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func e3StatusRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	e3Mkdir(t, filepath.Join(root, ".evolve"))
	return root
}

func TestBuildStatusReport_MapsTheSnapshotAndNeverEmitsNullCycles(t *testing.T) {
	e3NoGH(t)
	zero := &dashboard.ZeroShipRun{FirstCycle: 4, LastCycle: 6, Length: 3}
	roots := e3StubCollect(t, &dashboard.Snapshot{
		Loop:  dashboard.LoopStatus{Running: true, CycleID: 9, Phase: "build"},
		Trend: dashboard.Trend{ShipStreak: 2, LastZeroShipRun: zero, Closed: 10, Shipped: 7},
	})

	r := buildStatusReport("/proj", time.Now())

	if (*roots)[0] != "/proj" {
		t.Errorf("collected %q, want /proj", (*roots)[0])
	}
	if r.Cycles == nil || len(r.Cycles) != 0 {
		t.Errorf("cycles = %#v, want a non-nil empty slice", r.Cycles)
	}
	if r.Loop.CycleID != 9 || r.Streak != (statusStreak{ConsecutiveShipped: 2, LastZeroShipRun: zero, Closed: 10, Shipped: 7}) {
		t.Errorf("report = %+v", r)
	}
	if r.PRs.Available || !strings.HasPrefix(r.PRs.Error, "gh pr list: ") || r.CI.Available || !strings.HasPrefix(r.CI.Error, "gh run list --branch main: ") {
		t.Errorf("without gh both remotes must be unavailable with a reason: prs %+v ci %+v", r.PRs, r.CI)
	}
	out, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(out), `"cycles":[]`) {
		t.Errorf("json = %s (err %v), want cycles:[]", out, err)
	}
}

func TestBuildStatusReport_KeepsCollectedCycles(t *testing.T) {
	e3NoGH(t)
	e3StubCollect(t, &dashboard.Snapshot{Cycles: []dashboard.CycleSummary{{ID: 3, StateName: "shipped"}}})

	r := buildStatusReport("/proj", time.Now())

	if len(r.Cycles) != 1 || r.Cycles[0].ID != 3 {
		t.Errorf("cycles = %+v", r.Cycles)
	}
}

func TestWriteStatusText_RendersEverySection(t *testing.T) {
	t.Parallel()
	r := statusReport{
		Loop:   dashboard.LoopStatus{Running: true, BrakeEngaged: false, CycleID: 12, Phase: "audit"},
		Cycles: []dashboard.CycleSummary{{ID: 11, StateName: "shipped"}, {ID: 12, StateName: "auditing"}},
		Streak: statusStreak{ConsecutiveShipped: 3, Shipped: 8, Closed: 10, LastZeroShipRun: &dashboard.ZeroShipRun{FirstCycle: 5, LastCycle: 6, Length: 2}},
		PRs: statusRemote{Available: true, Items: []statusPR{
			{Number: 851, Title: "test: backfill", HeadRefName: "test/x", Checks: "failing", FailingChecks: []string{"lint", "unit"}},
			{Number: 852, Title: "docs: ünïcode", HeadRefName: "docs/y", Checks: "passing"},
		}},
		CI: statusRemote{Available: true, Items: []statusCIRun{{Workflow: "go.yml", Branch: "main", Status: "completed", Conclusion: "failure", URL: "https://u/1", FailingJobs: []string{"test (ubuntu)", "lint"}}}},
	}
	var out bytes.Buffer

	writeStatusText(&out, r)

	want := "loop:    running=true brake=false cycle=12 phase=audit\n" +
		"cycles:  2 rendered\n" +
		"  cycle 11  shipped\n" +
		"  cycle 12  auditing\n" +
		"streak:  3 consecutive shipped (shipped 8 of 10 closed)\n" +
		"         last zero-ship run: cycles 5-6 (2)\n" +
		"prs:     2 open\n" +
		"  #851 test: backfill [test/x] checks failing: lint, unit\n" +
		"  #852 docs: ünïcode [docs/y] checks passing\n" +
		"ci:      go.yml on main: completed failure https://u/1\n" +
		"         failing jobs: test (ubuntu), lint\n"
	if out.String() != want {
		t.Errorf("text =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestWriteStatusText_UnavailableRemotesAndNoZeroShipRun(t *testing.T) {
	t.Parallel()
	r := statusReport{
		PRs: statusRemote{Error: "gh pr list: not logged in"},
		CI:  statusRemote{Error: "gh run list --branch main: exit 4"},
	}
	var out bytes.Buffer

	writeStatusText(&out, r)

	want := "loop:    running=false brake=false cycle=0 phase=\n" +
		"cycles:  0 rendered\n" +
		"streak:  0 consecutive shipped (shipped 0 of 0 closed)\n" +
		"         last zero-ship run: none\n" +
		"prs:     unavailable (gh pr list: not logged in)\n" +
		"ci:      unavailable (gh run list --branch main: exit 4)\n"
	if out.String() != want {
		t.Errorf("text =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestWriteStatusPRs_ASingleFailingCheckIsNamed(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	writeStatusPRs(&out, statusRemote{Available: true, Items: []statusPR{{Number: 9, Title: "t", HeadRefName: "h", Checks: "failing", FailingChecks: []string{"lint"}}}})

	if out.String() != "prs:     1 open\n  #9 t [h] checks failing: lint\n" {
		t.Errorf("got %q", out.String())
	}
}

func TestWriteStatusPRs_AvailableWithNoPRs(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	writeStatusPRs(&out, statusRemote{Available: true, Items: []statusPR{}})

	if out.String() != "prs:     0 open\n" {
		t.Errorf("got %q", out.String())
	}
}

func TestWriteStatusCI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		r    statusRemote
		want string
	}{
		{"available but no run is unavailable", statusRemote{Available: true, Items: []statusCIRun{}}, "ci:      unavailable ()\n"},
		{"items of the wrong type", statusRemote{Available: true, Items: "garbage"}, "ci:      unavailable ()\n"},
		{"green run has no failing line", statusRemote{Available: true, Items: []statusCIRun{{Workflow: "go.yml", Branch: "main", Status: "completed", Conclusion: "success", URL: "u"}}},
			"ci:      go.yml on main: completed success u\n"},
		{"queued run", statusRemote{Available: true, Items: []statusCIRun{{Workflow: "go.yml", Branch: "main", Status: "queued"}}},
			"ci:      go.yml on main: queued  \n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			writeStatusCI(&out, tc.r)
			if out.String() != tc.want {
				t.Errorf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestRunStatus_RefusesBadInvocations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		root     func(t *testing.T) string
		wantCode int
		want     string
	}{
		{"unknown flag", []string{"--nope"}, func(*testing.T) string { return "" }, 1, "flag provided but not defined"},
		{"positional", []string{"extra"}, func(*testing.T) string { return "" }, 1, `evolve status: unexpected argument "extra"`},
		{"no .evolve", nil, func(t *testing.T) string { return t.TempDir() }, 2, "evolve status: cannot read the snapshot: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if root := tc.root(t); root != "" {
				args = append(args, "--project-root", root)
			}
			var out, errb bytes.Buffer

			code := runStatus(args, nil, &out, &errb)

			if code != tc.wantCode || out.Len() != 0 || !strings.Contains(errb.String(), tc.want) {
				t.Fatalf("code %d stdout %q stderr %q, want %d with %q", code, out.String(), errb.String(), tc.wantCode, tc.want)
			}
		})
	}
}

func TestRunStatus_AnUnresolvableWorkingDirectoryExitsOne(t *testing.T) {
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd: no such file or directory") }
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var out, errb bytes.Buffer

	code := runStatus(nil, nil, &out, &errb)

	if code != 1 || errb.String() != "evolve status: cwd: getwd: no such file or directory\n" {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}

func TestRunStatus_TextAndJSONOfAnIdleProjectWithoutGH(t *testing.T) {
	e3NoGH(t)
	root := e3StatusRoot(t)
	var text, js, errb bytes.Buffer

	textCode := runStatus([]string{"--project-root", root}, nil, &text, &errb)
	jsonCode := runStatus([]string{"--json", "--project-root", root}, nil, &js, &errb)

	if textCode != 0 || jsonCode != 0 {
		t.Fatalf("codes %d/%d stderr %q", textCode, jsonCode, errb.String())
	}
	if !strings.HasPrefix(text.String(), "loop:    running=false") || !strings.Contains(text.String(), "prs:     unavailable (gh pr list: ") || !strings.Contains(text.String(), "ci:      unavailable (gh run list --branch main: ") {
		t.Errorf("text = %q", text.String())
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(js.Bytes(), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, js.String())
	}
	for _, key := range []string{"loop", "cycles", "streak", "prs", "ci"} {
		if _, ok := got[key]; !ok {
			t.Errorf("json lacks %q: %s", key, js.String())
		}
	}
	if string(got["cycles"]) != "[]" {
		t.Errorf("cycles = %s, want []", got["cycles"])
	}
}

func TestRunStatus_AFailedJSONWriteExitsOne(t *testing.T) {
	e3NoGH(t)
	root := e3StatusRoot(t)
	var errb bytes.Buffer

	code := runStatus([]string{"--json", "--project-root", root}, nil, e3FailingWriter{}, &errb)

	if code != 1 || errb.String() != "evolve status: encode: stdout closed\n" {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}

func TestStatusMainCI_WithoutGHIsUnavailable(t *testing.T) {
	e3NoGH(t)
	if got := statusMainCI(context.Background(), t.TempDir()); got.Available || got.Items != nil || !strings.Contains(got.Error, "gh run list --branch main") {
		t.Errorf("statusMainCI without gh = %+v", got)
	}
}
