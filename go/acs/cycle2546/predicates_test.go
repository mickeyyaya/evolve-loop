//go:build acs

package cycle2546

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleNumber          = 7
	realFile             = "go/internal/real/file.go"
	staleRepeatedCode    = "INBOX_STALE_DROP_REPEATED"
	evidenceVerifiedKey  = "evidence_verified"
	scoutReportFileName  = "scout-report.md"
	scoutLinesNamingItem = "3,9,15"
)

type laneFixture struct {
	root, workspace, inbox string
	merged, side           string
}

func newLane(t *testing.T) laneFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(realFile, "package real\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "merged work")
	merged := repo.Git("rev-parse", "--short", "HEAD")
	repo.Git("checkout", "-q", "-b", "side")
	repo.Write("go/internal/side/only.go", "package side\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "unmerged work")
	side := repo.Git("rev-parse", "--short", "HEAD")
	repo.Git("checkout", "-q", "main")
	f := laneFixture{root: repo.Dir, merged: merged, side: side}
	f.inbox = filepath.Join(f.root, ".evolve", "inbox")
	f.workspace = filepath.Join(f.root, ".evolve", "runs", "cycle-7")
	for _, dir := range []string{f.inbox, f.workspace} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f laneFixture) writeItem(t *testing.T, id string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"id": id, "title": "fixture " + id, "kind": "bug"})
	if err := os.WriteFile(filepath.Join(f.inbox, "2026-09-26T00-00-00Z-"+id+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f laneFixture) answer(t *testing.T, bucket, id, reason string) {
	t.Helper()
	pin, _ := json.Marshal(map[string]any{"todo_ids": []string{id}, "goal_hash": "g"})
	decision, _ := json.Marshal(map[string]any{"top_n": []any{}, bucket: []map[string]string{{"id": id, "reason": reason}}})
	for name, body := range map[string][]byte{"lane-scope.json": pin, "triage-decision.json": decision} {
		if err := os.WriteFile(filepath.Join(f.workspace, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func (f laneFixture) closeOut(t *testing.T, cycle int, c *signalcenter.Center) {
	t.Helper()
	in := cycleoutcome.NoWorkInputs{ProjectRoot: f.root, Workspace: f.workspace, Cycle: cycle, Stderr: io.Discard}
	if _, err := cycleoutcome.ApplyNoWork(in.WithSignals(c)); err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
}

func (f laneFixture) record(t *testing.T, id string) map[string]any {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(f.inbox, id)
	if err != nil {
		t.Fatalf("%s not in %s: %v", id, f.inbox, err)
	}
	body, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func staleRepeated(events []signalcenter.Event, id string) int {
	n := 0
	for _, e := range events {
		if string(e.Code) == staleRepeatedCode && e.Fields["task_id"] == id {
			n++
		}
	}
	return n
}

func TestC2546_001_ConsoleVerificationResetsTheDriftWindow(t *testing.T) {
	names := []string{
		"TestPremiseDriftSection_ResetsTheDriftWindowAtTheConsoleVerification",
		"TestPremiseDriftSection_AVerificationOlderThanTheFilingChangesNothing",
		"TestPremiseDriftSection_AnUnreadableVerificationStampFallsBackToTheFiling",
		"TestPremiseDriftSection_SilentWhenTheVerificationPostdatesEveryChange",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/phases/triage",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2546_002_StaleDropEvidenceIsCheckedOnTheHost(t *testing.T) {
	f := newLane(t)
	cases := []struct {
		name, token string
		want        bool
	}{
		{"a cited sha on main", f.merged, true},
		{"a cited sha that is not an ancestor of HEAD", f.side, false},
		{"a cited sha that names no object", "deadbeefdeadbeef", false},
		{"a cited file:line whose file is at HEAD", realFile + ":12", true},
		{"a cited file:line whose file is absent at HEAD", "go/internal/gone/missing.go:3", false},
		{"free text that cites nothing checkable", "closed", false},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := "stale-case-" + string(rune('a'+i))
			f.writeItem(t, id)
			f.answer(t, "dropped", id, "stale: "+tc.token+" already handled")
			f.closeOut(t, cycleNumber, nil)
			rec := f.record(t, id)
			if rec["route"] != "console-manual" {
				t.Fatalf("the stale drop is still handed to the console: %v", rec)
			}
			got, present := rec[evidenceVerifiedKey].(bool)
			if !present || got != tc.want {
				t.Errorf("%s: %s = %v (present=%v), want %v", tc.name, evidenceVerifiedKey, rec[evidenceVerifiedKey], present, tc.want)
			}
		})
	}
}

func TestC2546_003_OnlyAStaleDropCarriesEvidenceVerified(t *testing.T) {
	f := newLane(t)
	f.writeItem(t, "plain-duplicate")
	f.answer(t, "dropped", "plain-duplicate", "duplicate: merged into another item")
	f.closeOut(t, cycleNumber, nil)
	rec := f.record(t, "plain-duplicate")
	if rec["route"] != "console-manual" {
		t.Fatalf("the drop is handed to the console: %v", rec)
	}
	if _, present := rec[evidenceVerifiedKey]; present {
		t.Errorf("a drop that cites no premise evidence must not claim a verification verdict: %v", rec)
	}
}

func TestC2546_004_SecondStaleDropOfTheSameIdEscalates(t *testing.T) {
	f := newLane(t)
	id := "bouncing-item"
	f.writeItem(t, id)
	opts := inboxmover.Options{ProjectRoot: f.root, Stderr: io.Discard}
	c := signalcenter.New()
	events := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *events = append(*events, e) })

	f.answer(t, "dropped", id, "stale: "+f.merged+" already landed")
	f.closeOut(t, cycleNumber, c)
	if n := staleRepeated(*events, id); n != 0 {
		t.Fatalf("the first stale drop must not escalate (got %d %s events)", n, staleRepeatedCode)
	}
	if _, err := inboxmover.RouteLane(opts, id, "console re-verified the premise and re-queued it"); err != nil {
		t.Fatalf("console re-queue: %v", err)
	}

	f.answer(t, "dropped", id, "stale: "+f.merged+" already landed")
	f.closeOut(t, cycleNumber+1, c)
	if n := staleRepeated(*events, id); n != 1 {
		t.Fatalf("the second stale drop of the same id emits exactly one %s (got %d)", staleRepeatedCode, n)
	}
	if rec := f.record(t, id); rec["route"] != "console-manual" {
		t.Errorf("the escalated item stays operator-owned, never back on a lane: %v", rec)
	}
	if _, err := inboxmover.Claim(opts, id, "9"); err == nil {
		t.Error("a lane claim of the escalated item must be refused")
	}
}

func TestC2546_005_EscalationNeedsTheSameIdStaleDroppedTwice(t *testing.T) {
	f := newLane(t)
	opts := inboxmover.Options{ProjectRoot: f.root, Stderr: io.Discard}
	c := signalcenter.New()
	events := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *events = append(*events, e) })

	for i, id := range []string{"first-item", "second-item"} {
		f.writeItem(t, id)
		f.answer(t, "dropped", id, "stale: "+f.merged+" already landed")
		f.closeOut(t, cycleNumber+i, c)
	}
	for _, e := range *events {
		if string(e.Code) == staleRepeatedCode {
			t.Errorf("two different ids each dropped once are not a repeat: %+v", e)
		}
	}

	f.writeItem(t, "duplicate-item")
	for i := 0; i < 2; i++ {
		f.answer(t, "dropped", "duplicate-item", "duplicate: merged elsewhere")
		f.closeOut(t, cycleNumber+10+i, c)
		if _, err := inboxmover.RouteLane(opts, "duplicate-item", "re-queued"); err != nil {
			t.Fatalf("re-queue: %v", err)
		}
	}
	if n := staleRepeated(*events, "duplicate-item"); n != 0 {
		t.Errorf("only stale drops count toward the repeat (got %d %s events)", n, staleRepeatedCode)
	}
}

func TestC2546_006_AbsentDropMustCiteTheScoutLinesThatNameTheId(t *testing.T) {
	f := newLane(t)
	scout := []string{"# Scout report", "", "selected: evidence-id", "", "", "", "", "", "see evidence-id again", "", "", "", "", "", "evidence-id last mention", ""}
	if err := os.WriteFile(filepath.Join(f.workspace, scoutReportFileName), []byte(strings.Join(scout, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, id, reason string
		want             bool
	}{
		{"zero citation while the id occurs in the report", "evidence-id", "absent: the assigned id is not in the scout report", false},
		{"one of three occurrence lines cited", "evidence-id", "invalid: scout-report.md:3 consulted", false},
		{"every occurrence line cited", "evidence-id", "absent: scout-report.md:" + scoutLinesNamingItem + " consulted, none selects it", true},
		{"occurrences covered by a cited range", "evidence-id", "absent: scout-report.md:3-9,15 consulted", true},
		{"an id the report never names", "ghost-id", "absent: scout-report.md has no line naming the id", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.writeItem(t, tc.id)
			f.answer(t, "dropped", tc.id, tc.reason)
			f.closeOut(t, cycleNumber, nil)
			rec := f.record(t, tc.id)
			got, present := rec[evidenceVerifiedKey].(bool)
			if !present || got != tc.want {
				t.Errorf("%s: %s = %v (present=%v), want %v", tc.name, evidenceVerifiedKey, rec[evidenceVerifiedKey], present, tc.want)
			}
		})
	}
}
