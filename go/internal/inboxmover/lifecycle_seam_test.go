package inboxmover

// lifecycle_seam_test.go — ADR-0103 unit 06 step 3: the host seam. Options is a
// value copied per call, so the ONE projection onto the leaf is (Options).mover
// and the ONE construction site is pinned by a source scan; every seam is
// threaded; every production and ACS spelling is kept; the wired-root golden
// renders the fifteen replaced lines through the root's StderrSink.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// nonTestSourcesMentioning lists the module's non-test Go files outside the
// lifecycle leaf and the one allowed site whose source contains needle (the
// core/carryover_lifecycle_test.go idiom).
func nonTestSourcesMentioning(t *testing.T, needle, allowed string) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/inboxmover/lifecycle/") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), needle) && rel != allowed {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	return offenders
}

// Test 46 — lifecycle.New( is spelled in exactly one non-test file: the seam.
func TestOptionsMover_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/inboxmover/inboxmover.go"
	if offenders := nonTestSourcesMentioning(t, "lifecycle.New(", onlySite); len(offenders) > 0 {
		t.Errorf("lifecycle.New( belongs to ONE non-test file (%s); these non-test files construct a Mover too: %v", onlySite, offenders)
	}
	if offenders := nonTestSourcesMentioning(t, "lifecycle.New(", "nowhere"); len(offenders) != 1 || offenders[0] != onlySite {
		t.Errorf("the construction lives in %s, found in %v", onlySite, offenders)
	}
}

type recordingCenter struct {
	c      *signalcenter.Center
	events []signalcenter.Event
}

func newRecordingCenter() *recordingCenter {
	r := &recordingCenter{c: signalcenter.New()}
	r.c.Subscribe(func(e signalcenter.Event) { r.events = append(r.events, e) })
	return r
}

// Test 47 — every Options seam reaches the leaf: the clock (the ledger TS),
// stderr, the active-cycle reader, the landing probe (called with the sha), the
// protected-path predicate (nil stays nil: a protected-files item is CLAIMABLE
// through ClaimLaneScope-shaped Options; non-nil refuses) and the Center.
func TestOptionsMover_ThreadsEverySeam(t *testing.T) {
	repo := makeRepo(t)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	writeItemAt(t, filepath.Join(inbox, "p.json"), `{"id":"p","files":["go/internal/guards/role.go (fix)"]}`)
	writeItemAt(t, filepath.Join(inbox, "q.json"), `{"id":"q","files":["go/internal/guards/role.go (fix)"]}`)
	dropProcessingFile(t, repo, "9", "s.json", "s")
	var stderr strings.Builder
	rec := &recordingAppender{}
	rc := newRecordingCenter()
	activeCalls, landedWith := 0, ""
	opts := Options{
		ProjectRoot:   repo,
		Stderr:        &stderr,
		Now:           func() time.Time { return fixedLifecycleClock },
		Ledger:        rec,
		ActiveCycleFn: func() (string, error) { activeCalls++; return "99", nil },
		IsLandedFn:    func(sha string) (bool, error) { landedWith = sha; return true, nil },
		Signals:       rc.c,
	}
	if _, err := Claim(opts, "p", "3"); err != nil {
		t.Errorf("a nil IsProtectedPath leaves the files-derived rule OFF: %v", err)
	}
	refusing := opts
	refusing.IsProtectedPath = func(string) bool { return true }
	if _, err := Claim(refusing, "q", "3"); !errors.Is(err, ErrConsoleRouted) {
		t.Errorf("a wired predicate refuses: %v", err)
	}
	if _, err := Promote(opts, "s", "processed", PromoteOpts{Cycle: "9", CommitSHA: "cafef00d00"}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverOrphans(opts); err != nil {
		t.Fatal(err)
	}
	if landedWith != "cafef00d00" || activeCalls != 1 {
		t.Errorf("IsLandedFn called with %q, ActiveCycleFn called %d times", landedWith, activeCalls)
	}
	if len(rec.records) < 2 || rec.records[0].TS != "2026-05-24T12:00:00Z" {
		t.Errorf("Now threads into the ledger TS: %+v", rec.records)
	}
	if !strings.Contains(stderr.String(), "[inbox-mover] claimed: p.json → processing/cycle-3/\n") {
		t.Errorf("Stderr threads: %q", stderr.String())
	}
	if len(rc.events) != 1 || rc.events[0].Code != lifecycle.CodeClaimRefused || rc.events[0].Origin != "Mover.Claim" {
		t.Errorf("Signals threads (the refusal is the only fault): %+v", rc.events)
	}
	if !opts.mover().SignalsWired() || (Options{ProjectRoot: repo}).mover().SignalsWired() {
		t.Error("SignalsWired follows Options.Signals")
	}
}

// Test 48 — the retire hook is WIRED (a promote releases the registry binding
// and preserves the pointer) and the drain reads the manifest from
// <root>/.evolve/runs/cycle-N (the duplicated belief, pinned until 06-F4).
func TestOptionsMover_RetireHookAndRunWorkspace(t *testing.T) {
	root := t.TempDir()
	seedRootItem(t, root, "task-r")
	seedRegistryOnly(t, root, "task-r", 1507)
	res, err := Promote(Options{ProjectRoot: root}, "task-r", "rejected", PromoteOpts{Cycle: "1507"})
	if err != nil || res.NoOp {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(root, "task-r"); ok {
		t.Error("the retire hook releases the binding")
	}
	if raw, _ := os.ReadFile(res.DestPath); !strings.Contains(string(raw), `"released_continuations"`) {
		t.Errorf("the pointer is preserved onto the retired item: %s", raw)
	}
	writeItemAt(t, filepath.Join(root, ".evolve", "runs", "cycle-91", "continuation-manifest.json"), `{"snapshot_sha":"abc123","cycle":91}`)
	writeItemAt(t, filepath.Join(root, ".evolve", "inbox", "processing", "cycle-91", "task-a.json"), `{"id":"task-a"}`)
	if _, err := ReleaseCycleProcessing(Options{ProjectRoot: root}, 91); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, ".evolve", "inbox", "task-a.json")); !strings.Contains(string(raw), `"snapshot_sha":"abc123"`) {
		t.Errorf("the drain stamps from <root>/.evolve/runs/cycle-N: %s", raw)
	}
}

// Test 49 — every production and ACS spelling compiles unchanged and the
// sentinels are the leaf's own pointers (errors.Is across the boundary).
func TestFacades_KeepEverySpelling(t *testing.T) {
	var (
		_ func(Options, string, string) (ClaimResult, error)                   = Claim
		_ func(Options, string, string, PromoteOpts) (PromoteResult, error)    = Promote
		_ func(Options, string) (PromoteResult, error)                         = ReleaseFromQuarantine
		_ func(Options) (RecoverResult, error)                                 = RecoverOrphans
		_ func(Options, int) (RecoverResult, error)                            = ReleaseCycleProcessing
		_ func(Options, int, string) (RecoverResult, error)                    = ReleaseCycleProcessingWithReason
		_ func(Options, string) (int, bool)                                    = ReadFailureCount
		_ func(string, string) (string, error)                                 = FindFileByTaskID
		_ func(int, int, bool) bool                                            = ShouldQuarantine
		_ func([]byte) []string                                                = SupersededInboxIDs
		_ func(Options, []string, string, PromoteOpts) ([]string, error)       = ReconcileSuperseded
		_ func(string, string) (Location, error)                               = Locate
		_ func(Options, CycleOutcome) (OutcomeResult, error)                   = ApplyCycleOutcome
		_ func(Options, int, []string) ([]string, error)                       = ClaimLaneScope
		_ func(Options, string, int, string, int) (int, bool, error)           = RecordRootTaskFailure
		_ func(Options, string) DispatchState                                  = ResolveDispatchState
		_ func(string, string) (int, error)                                    = bumpFailureCount
		_ func(string, func(map[string]json.RawMessage)) error                 = updateItemJSON
		_ func(Options, int, string, *quarantinePolicy) (RecoverResult, error) = releaseCycleProcessing
		// The drain policy is ONE struct with ONE contract: the host's spelling is
		// an alias of the leaf's Policy, so a field added to either is the same
		// field at outcome.go's literal — never a second struct projected by hand.
		_ *lifecycle.Policy                                   = (*quarantinePolicy)(nil)
		_ lifecycle.ClaimResult                               = ClaimResult{}
		_ lifecycle.PromoteOpts                               = PromoteOpts{}
		_ lifecycle.PromoteResult                             = PromoteResult{}
		_ lifecycle.RecoverResult                             = RecoverResult{}
		_ lifecycle.Location                                  = Location{}
		_ LedgerAppender                                      = (*ledger.FileLedger)(nil)
		_ lifecycle.LedgerAppender                            = LedgerAppender(nil)
		_ func(context.Context, ledger.LifecycleRecord) error = (*ledger.FileLedger)(nil).AppendLifecycle
		_ *signalcenter.Center                                = Options{}.Signals
	)
	for name, pair := range map[string][2]error{
		"ErrNotFound": {ErrNotFound, lifecycle.ErrNotFound}, "ErrMvFailed": {ErrMvFailed, lifecycle.ErrMvFailed},
		"ErrBadArgs": {ErrBadArgs, lifecycle.ErrBadArgs}, "ErrBadState": {ErrBadState, lifecycle.ErrBadState},
		"ErrConsoleRouted": {ErrConsoleRouted, lifecycle.ErrConsoleRouted},
	} {
		if !errors.Is(fmt.Errorf("wrapped: %w", pair[0]), pair[1]) || pair[0].Error() != pair[1].Error() {
			t.Errorf("%s: the host must re-export the leaf's pointer", name)
		}
	}
	if !strings.HasPrefix(ErrConsoleRouted.Error(), "inboxmover: ") {
		t.Error("the sentinel texts keep the inboxmover: prefix (the cmd layer prints the error verbatim)")
	}
}

// Test 50 — the FAIL drain through Options.Signals: the quarantine failure
// renders in the Center (origin Mover.Release, the cycle), the fallback line
// does not print, the INFO lines still do, and the FAIL closeout's lane-scope
// pass leaves an already-claimed id alone — no INBOX_CLAIM_NOT_FOUND, no
// console duplicate (the false not-found of the 2026-09-14 poison-loop
// incident; 06-F7/06-F11 retired with it).
func TestApplyCycleOutcome_FailDrain_EmitsInboxCodesThroughOptionsSignals(t *testing.T) {
	repo := makeRepo(t)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	writeItemAt(t, filepath.Join(inbox, "processing", "cycle-11", "t7.json"), `{"id":"t7"}`)
	writeItemAt(t, filepath.Join(inbox, "quarantine"), "x")
	var stderr strings.Builder
	rc := newRecordingCenter()
	res, err := ApplyCycleOutcome(Options{ProjectRoot: repo, Stderr: &stderr, Signals: rc.c},
		CycleOutcome{Cycle: 11, CommittedIDs: []string{"t7"}, Ceiling: 1})
	if err != nil || len(res.Released) != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	var codes []signalcenter.Code
	for _, e := range rc.events {
		codes = append(codes, e.Code)
	}
	want := []signalcenter.Code{lifecycle.CodePromoteMoveFailed, lifecycle.CodeQuarantineFailed}
	if fmt.Sprint(codes) != fmt.Sprint(want) {
		t.Fatalf("codes = %v, want %v", codes, want)
	}
	if rc.events[1].Origin != "Mover.Release" || rc.events[1].Cycle != 11 || rc.events[1].Fields["outcome"] != "error" {
		t.Errorf("the quarantine failure: %+v", rc.events[1])
	}
	out := stderr.String()
	if strings.Contains(out, "[inbox-mover] ERROR: ") || strings.Contains(out, "[inbox-mover] WARN: claim: ") {
		t.Errorf("a wired root prints no fallback line for a coded fault: %q", out)
	}
	if strings.Contains(out, "claim-lane-scope: 't7' not claimed") || !strings.Contains(out, "[inbox-mover] released: t7.json ← processing/cycle-11/\n") {
		t.Errorf("no lane-scope line for an already-claimed id; the INFO lines stay on stderr: %q", out)
	}
}

// Test 51 — the wired-root golden: the same script as test 5 through
// Options{Signals: a Center with the root's WARN-filtered StderrSink onto the
// same stderr} renders sequence.stderr.wired.golden; the diff against the
// Center-less golden is exactly the fifteen replaced lines (the §5 table).
func TestGolden_LifecycleSequence_WiredRootStderr(t *testing.T) {
	repo := makeRepo(t)
	var stderr strings.Builder
	center := signalcenter.New()
	center.Subscribe(signalcenter.Filter(signalcenter.StderrSink(&stderr), signalcenter.SeverityWarn))
	lifecycleSequence(t, repo, func() Options {
		opts := goldenOptions(repo, &stderr)
		opts.Signals = center
		return opts
	})
	assertGolden(t, "sequence.stderr.wired.golden", stderr.String(), repo)
	assertGolden(t, "sequence.ledger.golden", readLedger(t, repo), repo)
	plain, wired := readGolden(t, "sequence.stderr.golden"), readGolden(t, "sequence.stderr.wired.golden")
	replaced, added := 0, 0
	for _, line := range strings.Split(strings.TrimSuffix(plain, "\n"), "\n") {
		if strings.HasPrefix(line, "[inbox-mover] WARN: ") || strings.HasPrefix(line, "[inbox-mover] ERROR: ") {
			if !strings.Contains(wired, line+"\n") {
				replaced++
			}
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(wired, "\n"), "\n") {
		if strings.HasPrefix(line, "[inbox] inbox.warning WARN ") {
			added++
		}
	}
	if replaced != added || replaced != 18 {
		t.Errorf("the wired golden replaces every coded fallback line one-for-one: %d replaced, %d rendered (the script provokes the 15 replaced sites; :201 fires twice since the 2026-09-14 breaker stopped re-claiming already-claimed ids, :336 and :349 twice)", replaced, added)
	}
	if strings.Count(plain, "\n")-replaced != strings.Count(wired, "\n")-added {
		t.Error("every other line is byte-identical between the two goldens")
	}
}

// gitRepoWithMain builds a repo whose main holds one commit and whose side
// branch holds a second; returns the root and the two shas.
func gitRepoWithMain(t *testing.T) (root, onMain, onSide string) {
	t.Helper()
	root = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	onMain = git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "side")
	git("commit", "-q", "--allow-empty", "-m", "side")
	onSide = git("rev-parse", "HEAD")
	return root, onMain, onSide
}

// Test 58 — the default landing probe answers the leaf's gate honestly: an
// ancestor of main is landed, a side-branch sha is not, and a probe git cannot
// answer (an unknown sha, no local main, a non-git root — exit 128 — or an exec
// fault) is fail-open AND returned as the error, so INBOX_LANDED_CHECK_FAILED
// reaches production instead of a silent (true, nil). Through a Center-less
// facade the fault prints the fallback line; through a wired one it is the
// event — the item lands in processed/ either way.
func TestShaLandedOnMain_ReportsWhatGitCannotAnswer_PromoteFailsOpenLoudly(t *testing.T) {
	repo, onMain, onSide := gitRepoWithMain(t)
	nonGit := t.TempDir()
	for _, tc := range []struct {
		name, root, sha string
		landed          bool
		wantErr         string
	}{
		{name: "ancestor of main", root: repo, sha: onMain, landed: true},
		{name: "side-branch sha", root: repo, sha: onSide, landed: false},
		{name: "unknown sha", root: repo, sha: "deadbeef00", landed: true, wantErr: "git merge-base --is-ancestor deadbeef00 main exit=128: fatal:"},
		{name: "non-git root", root: nonGit, sha: onMain, landed: true, wantErr: "exit=128: fatal: not a git repository"},
		{name: "absent root (exec fault)", root: filepath.Join(nonGit, "gone"), sha: onMain, landed: true, wantErr: "git merge-base --is-ancestor " + onMain + " main: chdir "},
	} {
		landed, err := shaLandedOnMain(tc.root, tc.sha)
		if landed != tc.landed || (tc.wantErr == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("%s: shaLandedOnMain = (%v, %v), want (%v, err containing %q)", tc.name, landed, err, tc.landed, tc.wantErr)
		}
	}
	root := makeRepo(t)
	writeItemAt(t, filepath.Join(root, ".evolve", "inbox", "lp.json"), `{"id":"lp"}`)
	var stderr strings.Builder
	res, err := Promote(Options{ProjectRoot: root, Stderr: &stderr}, "lp", "processed", PromoteOpts{Cycle: "4", CommitSHA: "deadbeef00"})
	if err != nil || res.NoOp || !strings.HasSuffix(res.DestPath, filepath.Join("processed", "cycle-4", "deadbeef-lp.json")) {
		t.Fatalf("fail-open promote: res = %+v, err = %v", res, err)
	}
	if out := stderr.String(); !strings.Contains(out, "[inbox-mover] WARN: promote: landed check for 'lp' failed (git merge-base --is-ancestor deadbeef00 main exit=128: fatal:") ||
		!strings.Contains(out, "— treating deadbeef00 as landed (fail-open)\n[inbox-mover] promoted: lp.json → processed/\n") {
		t.Errorf("the Center-less root prints the fallback line before the INFO line: %q", out)
	}
	writeItemAt(t, filepath.Join(root, ".evolve", "inbox", "lq.json"), `{"id":"lq"}`)
	rc := newRecordingCenter()
	stderr.Reset()
	if _, err := Promote(Options{ProjectRoot: root, Stderr: &stderr, Signals: rc.c}, "lq", "processed", PromoteOpts{Cycle: "4", CommitSHA: "deadbeef00"}); err != nil {
		t.Fatal(err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != lifecycle.CodeLandedCheckFailed || rc.events[0].Origin != "Mover.Promote" ||
		rc.events[0].Cycle != 4 || rc.events[0].Fields["step"] != "landing" || !strings.Contains(rc.events[0].Fields["err"], "exit=128") {
		t.Errorf("the wired root gets the event: %+v", rc.events)
	}
	if strings.Contains(stderr.String(), "WARN: ") {
		t.Errorf("no fallback line on the wired root: %q", stderr.String())
	}
}

// Test 59 — the console voice `[inbox-mover] ` is spelled ONCE across the host
// and the leaf (lifecycle.LegacyPrefix); the host's logf and the leaf's two
// links consume it, so 06-F5's fold of the sibling renderer is a deletion, not
// a hunt (the cmd layer's own copies are outside the package tree).
func TestInboxMoverPrefix_OneHome(t *testing.T) {
	const needle = `"[inbox-mover] ` // the opening quote pins string literals, not prose
	homes := map[string]int{}
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if n := strings.Count(string(body), needle); n > 0 {
			homes[filepath.ToSlash(path)] = n
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"lifecycle/lifecycle.go": 1}; fmt.Sprint(homes) != fmt.Sprint(want) {
		t.Errorf("%s homes = %v, want %v (one exported const, every renderer consumes it)", needle, homes, want)
	}
	if lifecycle.LegacyPrefix != "[inbox-mover] " {
		t.Errorf("LegacyPrefix = %q", lifecycle.LegacyPrefix)
	}
}

// Test 60 — the FAIL drain THROUGH ApplyCycleOutcome bumps only the committed
// ids even when a wave lane claimed the whole menu into processing/cycle-N/:
// the uncommitted item releases with no failure_count at all. This pins the
// third key of outcome.go's one Policy literal (Committed) at the host — the
// leaf pins its own Policy; the ACS 1180 negative half keeps its menu item at
// the root, where the drain never walks.
func TestApplyCycleOutcome_FailDrain_BumpsOnlyCommittedIDsAlreadyInProcessing(t *testing.T) {
	repo := makeRepo(t)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	writeItemAt(t, filepath.Join(inbox, "processing", "cycle-12", "c.json"), `{"id":"c"}`)
	writeItemAt(t, filepath.Join(inbox, "processing", "cycle-12", "u.json"), `{"id":"u"}`)
	res, err := ApplyCycleOutcome(Options{ProjectRoot: repo}, CycleOutcome{Cycle: 12, CommittedIDs: []string{"c"}, Ceiling: 5, Reason: "cycle-failure-release"})
	if err != nil || len(res.Released) != 2 || len(res.Quarantined) != 0 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	committed, _ := os.ReadFile(filepath.Join(inbox, "c.json"))
	uncommitted, _ := os.ReadFile(filepath.Join(inbox, "u.json"))
	if string(committed) != `{"failure_count":1,"id":"c","last_failure_reason":"cycle-failure-release"}` {
		t.Errorf("the committed id is bumped: %s", committed)
	}
	if string(uncommitted) != `{"id":"u"}` {
		t.Errorf("the uncommitted id releases untouched (no failure_count, no rewrite): %s", uncommitted)
	}
}
