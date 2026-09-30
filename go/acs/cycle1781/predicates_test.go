//go:build acs

package cycle1781

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	claimCycle  = "7"
	failedCycle = 41
	nextCycle   = 42
	stampedID   = "task-p"
)

type inboxFixture struct {
	root  string
	inbox string
	opts  inboxmover.Options
}

func newInboxFixture(t *testing.T) inboxFixture {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	mkdirAll(t, inbox)
	return inboxFixture{root: root, inbox: inbox, opts: inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}}
}

func (f inboxFixture) fileItem(t *testing.T, rel, id string, deps ...string) string {
	t.Helper()
	doc := map[string]any{"id": id, "title": id}
	if len(deps) > 0 {
		doc["deps"] = deps
	}
	return f.fileDoc(t, rel, id, doc)
}

func (f inboxFixture) fileDoc(t *testing.T, rel, id string, doc map[string]any) string {
	t.Helper()
	dir := filepath.Join(f.inbox, rel)
	mkdirAll(t, dir)
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "2026-09-30T00-00-00Z-"+id+".json")
	writeFile(t, p, body)
	return p
}

func (f inboxFixture) assertPendingAtRoot(t *testing.T, src, id string) {
	t.Helper()
	if _, err := os.Stat(src); err != nil {
		t.Errorf("RED: %s left the inbox root (%v) — a refused claim must leave the item pending", id, err)
	}
	loc, err := inboxmover.Locate(f.inbox, id)
	if err != nil || loc.Cycle != 0 {
		t.Errorf("RED: %s is located at %+v (err=%v), want the inbox root (cycle 0)", id, loc, err)
	}
}

func (f inboxFixture) assertClaimedInto(t *testing.T, id string, cycle int) {
	t.Helper()
	loc, err := inboxmover.Locate(f.inbox, id)
	if err != nil || loc.Cycle != cycle {
		t.Errorf("%s is located at %+v (err=%v), want claimed into processing/cycle-%d", id, loc, err, cycle)
	}
}

type waitingRow struct {
	name    string
	setup   func(t *testing.T, f inboxFixture)
	deps    []string
	blocker string
}

func waitingRows() []waitingRow {
	return []waitingRow{
		{"dependency still pending at the root", func(t *testing.T, f inboxFixture) { f.fileItem(t, ".", "dep-a") }, []string{"dep-a"}, "dep-a"},
		{"dependency claimed by another lane", func(t *testing.T, f inboxFixture) { f.fileItem(t, "processing/cycle-9", "dep-a") }, []string{"dep-a"}, "dep-a"},
		{"dependency parked for retry", func(t *testing.T, f inboxFixture) { f.fileItem(t, "retry", "dep-a") }, []string{"dep-a"}, "dep-a"},
		{"first unmet of several, after a landed one", func(t *testing.T, f inboxFixture) {
			f.fileItem(t, "processed/cycle-9", "dep-landed")
			f.fileItem(t, "retry", "dep-a")
		}, []string{"dep-landed", "dep-a"}, "dep-a"},
	}
}

func hostClaimerOpts(f inboxFixture, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		InboxDir:        f.inbox,
		Stderr:          stderr,
		Ledger:          ledger.New(filepath.Join(f.root, ".evolve")),
		IsProtectedPath: func(string) bool { return false },
	}
}

func TestC1781_001_HostClaimerRefusesItemWaitingOnUnlandedDependencyNamingBlocker(t *testing.T) {
	for _, row := range waitingRows() {
		t.Run(row.name, func(t *testing.T) {
			f := newInboxFixture(t)
			row.setup(t, f)
			src := f.fileItem(t, ".", "waiting-b", row.deps...)
			f.fileItem(t, ".", "ready-c")
			if d := inboxmover.PendingDispatchability(f.opts, row.deps); d.Dispatchable {
				t.Fatalf("fixture: PendingDispatchability(%v) is dispatchable, so the row is not a waiting item", row.deps)
			}

			var stderr bytes.Buffer
			err := inboxmover.ClaimPending(hostClaimerOpts(f, &stderr), 7, []string{"waiting-b", "ready-c"})
			report := stderr.String()
			if err != nil {
				report += "\n" + err.Error()
			}

			f.assertPendingAtRoot(t, src, "waiting-b")
			f.assertClaimedInto(t, "ready-c", 7)
			if !strings.Contains(report, "waiting-b") || !strings.Contains(report, row.blocker) {
				t.Errorf("RED: the host claim's refusal of waiting-b does not name the item and its blocking dependency %q (stderr + returned error):\n%s", row.blocker, report)
			}
		})
	}
}

func TestC1781_002_HostClaimerStillTakesDispatchableItemsAndLeavesConsoleItems(t *testing.T) {
	rows := []struct {
		name  string
		setup func(t *testing.T, f inboxFixture)
		deps  []string
	}{
		{"no declared dependency", func(*testing.T, inboxFixture) {}, nil},
		{"dependency processed", func(t *testing.T, f inboxFixture) { f.fileItem(t, "processed/cycle-9", "dep-a") }, []string{"dep-a"}},
		{"dependency consumed", func(t *testing.T, f inboxFixture) { f.fileItem(t, "consumed", "dep-a") }, []string{"dep-a"}},
		{"dependency rejected", func(t *testing.T, f inboxFixture) { f.fileItem(t, "rejected/cycle-9", "dep-a") }, []string{"dep-a"}},
		{"dependency quarantined", func(t *testing.T, f inboxFixture) { f.fileItem(t, "quarantine", "dep-a") }, []string{"dep-a"}},
		{"dependency never filed", func(*testing.T, inboxFixture) {}, []string{"dep-never-filed"}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			f := newInboxFixture(t)
			row.setup(t, f)
			f.fileItem(t, ".", "ready-b", row.deps...)
			if d := inboxmover.PendingDispatchability(f.opts, row.deps); !d.Dispatchable {
				t.Fatalf("fixture: PendingDispatchability(%v) = %+v, want dispatchable", row.deps, d)
			}
			if err := inboxmover.ClaimPending(hostClaimerOpts(f, io.Discard), 7, []string{"ready-b"}); err != nil {
				t.Errorf("ClaimPending returned %v for a dispatchable item", err)
			}
			f.assertClaimedInto(t, "ready-b", 7)
		})
	}

	t.Run("console-routed item stays operator-owned", func(t *testing.T) {
		f := newInboxFixture(t)
		src := f.fileDoc(t, ".", "console-x", map[string]any{"id": "console-x", "route": "console-manual"})
		if err := inboxmover.ClaimPending(hostClaimerOpts(f, io.Discard), 7, []string{"console-x"}); err != nil {
			t.Errorf("ClaimPending returned %v for a console-routed item, which it leaves for the effects gate", err)
		}
		f.assertPendingAtRoot(t, src, "console-x")
	})
}

func TestC1781_003_EvolveInboxMoverClaimRefusesWaitingItemAndNamesBlocker(t *testing.T) {
	bin := buildEvolve(t)
	f := newInboxFixture(t)
	f.fileItem(t, ".", "dep-a")
	src := f.fileItem(t, ".", "waiting-b", "dep-a")
	f.fileItem(t, ".", "ready-c")

	out, code := runEvolve(t, bin, f.root, "inbox-mover", "claim", "ready-c", claimCycle)
	if code != 0 {
		t.Fatalf("control: `evolve inbox-mover claim ready-c %s` exited %d, so the CLI fixture is broken:\n%s", claimCycle, code, out)
	}
	f.assertClaimedInto(t, "ready-c", 7)

	out, code = runEvolve(t, bin, f.root, "inbox-mover", "claim", "waiting-b", claimCycle)
	if code == 0 {
		t.Errorf("RED: `evolve inbox-mover claim waiting-b %s` exited 0 although dep-a is still pending:\n%s", claimCycle, out)
	}
	if !strings.Contains(out, "dep-a") {
		t.Errorf("RED: the CLI refusal does not name the blocking dependency dep-a:\n%s", out)
	}
	f.assertPendingAtRoot(t, src, "waiting-b")
}

type stampFixture struct {
	home     string
	root     string
	ws       string
	findings []byte
	stamp    continuation.Continuation
	opts     inboxmover.Options
}

func newStampFixture(t *testing.T, worktree func(home string) string) stampFixture {
	t.Helper()
	home := realDir(t, t.TempDir())
	t.Setenv("HOME", home)
	root := filepath.Join(home, "proj")
	ws := filepath.Join(root, ".evolve", "runs", "cycle-41")
	mkdirAll(t, ws)
	wt := filepath.Join(home, "wt", "cycle-41")
	if worktree != nil {
		wt = worktree(home)
	}
	fx := stampFixture{
		home:     home,
		root:     root,
		ws:       ws,
		findings: []byte(`{"phase":"audit","reasons":["export Y is named by no test"]}`),
		stamp: continuation.Continuation{
			Worktree:     wt,
			Branch:       "evolve/cycle-41",
			SnapshotSHA:  strings.Repeat("a", 40),
			BaseSHA:      strings.Repeat("b", 40),
			FindingsPath: filepath.Join(ws, "audit-fail-reason.json"),
			Cycle:        failedCycle,
		},
		opts: inboxmover.Options{ProjectRoot: root, Stderr: io.Discard},
	}
	writeFile(t, fx.stamp.FindingsPath, fx.findings)
	if err := continuation.WriteManifest(ws, fx.stamp); err != nil {
		t.Fatal(err)
	}
	inbox := inboxFixture{root: root, inbox: filepath.Join(root, ".evolve", "inbox")}
	inbox.fileItem(t, "processing/cycle-41", stampedID)
	return fx
}

func (fx stampFixture) itemPath(rel string) string {
	return filepath.Join(fx.root, ".evolve", "inbox", rel, "2026-09-30T00-00-00Z-"+stampedID+".json")
}

func (fx stampFixture) redactedStamp() continuation.Continuation {
	want := fx.stamp
	want.Worktree = "~/wt/cycle-41"
	want.FindingsPath = "~/proj/.evolve/runs/cycle-41/audit-fail-reason.json"
	return want
}

type drain struct {
	name string
	run  func(opts inboxmover.Options) error
}

func failedLaneDrains() []drain {
	return []drain{
		{"failed-cycle outcome", func(opts inboxmover.Options) error {
			_, err := inboxmover.ApplyCycleOutcome(opts, inboxmover.CycleOutcome{
				Cycle: failedCycle, CommittedIDs: []string{stampedID}, Ceiling: 3, Reason: "cycle-failure-release",
			})
			return err
		}},
		{"cycle processing release", func(opts inboxmover.Options) error {
			_, err := inboxmover.ReleaseCycleProcessingWithReason(opts, failedCycle, "cycle-failure-release")
			return err
		}},
	}
}

func TestC1781_004_FailedLaneReleaseWritesNoHomePathIntoTrackedItem(t *testing.T) {
	for _, d := range failedLaneDrains() {
		t.Run(d.name, func(t *testing.T) {
			fx := newStampFixture(t, nil)
			if err := d.run(fx.opts); err != nil {
				t.Fatalf("drain: %v", err)
			}
			raw := readFile(t, fx.itemPath("."))
			if bytes.Contains(raw, []byte(fx.home)) {
				t.Errorf("RED: the released inbox item carries the absolute home path %s:\n%s", fx.home, raw)
			}
			if got, want := stampOf(t, raw), fx.redactedStamp(); got != want {
				t.Errorf("RED: released stamp = %+v\nwant worktree and findings_path in ~ form, the salvage identity untouched: %+v", got, want)
			}
		})
	}
}

func TestC1781_005_StampPathsOutsideHomePassThroughVerbatim(t *testing.T) {
	rows := []struct {
		name     string
		worktree func(home string) string
	}{
		{"outside the home directory", func(string) string { return "/opt/evolve-wt/cycle-41" }},
		{"sibling directory sharing the home prefix", func(home string) string { return home + "-sibling/wt/cycle-41" }},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			fx := newStampFixture(t, row.worktree)
			if _, err := inboxmover.ReleaseCycleProcessingWithReason(fx.opts, failedCycle, "cycle-failure-release"); err != nil {
				t.Fatalf("release: %v", err)
			}
			got := stampOf(t, readFile(t, fx.itemPath(".")))
			if got.Worktree != fx.stamp.Worktree {
				t.Errorf("worktree %q was rewritten to %q; only paths inside the home directory collapse to ~", fx.stamp.Worktree, got.Worktree)
			}
			if want := fx.redactedStamp().FindingsPath; got.FindingsPath != want {
				t.Errorf("RED: findings_path = %q, want %q", got.FindingsPath, want)
			}
		})
	}
}

func TestC1781_006_ContinuationRegistryKeepsAbsolutePaths(t *testing.T) {
	fx := newStampFixture(t, nil)
	if err := continuation.WriteRegistryEntry(fx.root, stampedID, fx.stamp); err != nil {
		t.Fatalf("WriteRegistryEntry: %v", err)
	}
	got, ok, err := continuation.ReadRegistryEntry(fx.root, stampedID)
	if err != nil || !ok || got != fx.stamp {
		t.Errorf("registry entry = %+v ok=%v err=%v, want the absolute %+v", got, ok, err, fx.stamp)
	}
	if !bytes.Contains(readFile(t, continuation.RegistryPath(fx.root)), []byte(fx.stamp.FindingsPath)) {
		t.Errorf("the registry file no longer holds the absolute findings path %s", fx.stamp.FindingsPath)
	}
}

func TestC1781_007_NextCycleResolverHandsBuilderAnOpenableFindingsPath(t *testing.T) {
	fx := newStampFixture(t, nil)
	if err := failedLaneDrains()[0].run(fx.opts); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if _, err := inboxmover.Claim(fx.opts, stampedID, "42"); err != nil {
		t.Fatalf("the next cycle could not claim the released item: %v", err)
	}
	claimed := readFile(t, fx.itemPath("processing/cycle-42"))
	if bytes.Contains(claimed, []byte(fx.home)) {
		t.Errorf("RED: the item the next cycle claimed carries the absolute home path %s:\n%s", fx.home, claimed)
	}

	c := inboxmover.ResolveContinuationForScope(fx.opts, nextCycle, []string{stampedID})
	if c == nil {
		t.Fatalf("the next cycle resolved no continuation from its claimed, stamped item")
	}
	if c.SnapshotSHA != fx.stamp.SnapshotSHA || c.BaseSHA != fx.stamp.BaseSHA || c.Cycle != failedCycle {
		t.Errorf("resolved continuation %+v does not carry the stamped snapshot %s / base %s from cycle %d", *c, fx.stamp.SnapshotSHA, fx.stamp.BaseSHA, failedCycle)
	}
	body, err := os.ReadFile(c.FindingsPath)
	if err != nil || !bytes.Equal(body, fx.findings) {
		t.Errorf("RED: the builder's findings reader opens the resolved findings_path %q as given and gets %q (err=%v), want the failed attempt's findings %q", c.FindingsPath, body, err, fx.findings)
	}
}

func TestC1781_008_TouchedEnrolledPackagesNameEveryExport(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	enrolled := string(readFile(t, filepath.Join(goDir, ".apicover-enforce")))
	ctx := context.Background()
	for _, pkg := range []string{"internal/inboxmover", "internal/inboxmover/lifecycle", "internal/continuation"} {
		t.Run(pkg, func(t *testing.T) {
			if !strings.Contains(enrolled, "./"+pkg+"\n") {
				t.Fatalf("%s is no longer enrolled in go/.apicover-enforce", pkg)
			}
			dir := filepath.Join(goDir, filepath.FromSlash(pkg))
			syms, err := apicover.Enumerate(ctx, dir)
			if err != nil {
				t.Fatalf("apicover.Enumerate: %v", err)
			}
			names, err := apicover.NamesReferencedInTests(ctx, dir)
			if err != nil {
				t.Fatalf("apicover.NamesReferencedInTests: %v", err)
			}
			for _, s := range apicover.Classify(syms, names, nil).Uncovered {
				t.Errorf("exported %s (%s:%d) is named by no _test.go in %s — make apicover-enforce fails the tree", s.Name, filepath.Base(s.File), s.Line, pkg)
			}
		})
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	bin := filepath.Join(t.TempDir(), "evolve")
	if out, code := runCmd(t, exec.CommandContext(testContext(t), "go", "build", "-o", bin, "./cmd/evolve"), goDir, os.Environ()); code != 0 {
		t.Fatalf("go build ./cmd/evolve exited %d:\n%s", code, out)
	}
	return bin
}

func runEvolve(t *testing.T, bin, projectRoot string, args ...string) (string, int) {
	t.Helper()
	env := append(withoutEnv(os.Environ(), "EVOLVE_"), "EVOLVE_PROJECT_ROOT="+projectRoot)
	return runCmd(t, exec.CommandContext(testContext(t), bin, args...), projectRoot, env)
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func runCmd(t *testing.T, cmd *exec.Cmd, dir string, env []string) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.WaitDelay = 10 * time.Second
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		return buf.String(), exitErr.ExitCode()
	case err != nil:
		t.Fatalf("run %v in %s: %v", cmd.Args, dir, err)
	}
	return buf.String(), 0
}

func withoutEnv(env []string, prefixes ...string) []string {
	var out []string
	for _, kv := range env {
		keep := true
		for _, p := range prefixes {
			if strings.HasPrefix(kv, p) {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}

func stampOf(t *testing.T, raw []byte) continuation.Continuation {
	t.Helper()
	var doc struct {
		Continuation *continuation.Continuation `json:"continuation"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Continuation == nil {
		t.Fatalf("item carries no continuation stamp (err=%v):\n%s", err, raw)
	}
	return *doc.Continuation
}

func realDir(t *testing.T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, body []byte) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
