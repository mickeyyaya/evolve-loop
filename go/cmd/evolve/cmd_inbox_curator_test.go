package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
)

const curatedItem = `{"id":"rolling-pool","kind":"feature","priority_class":"performance","weight":0.6,"origin":"console",` +
	`"title":"A free lane takes the next ready task","files":["go/internal/fleet/pool.go"],"summary":"s","fix":"f",` +
	`"acceptance":["TestPool_AFreeSlotTakesTheNextTodo passes"]}`

type curatorFixture struct {
	repo *gittest.Repo
	root string
}

func newCuratorFixture(t *testing.T, stage string) curatorFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".evolve/policy.json", `{"inbox": {"curator": {"stage": "`+stage+`"}}}`)
	repo.Write(".evolve/inbox/2026-10-01T00-00-00Z-rolling-pool.json", curatedItem)
	repo.Write("go/internal/fleet/pool.go", "package fleet\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	return curatorFixture{repo: repo, root: repo.Dir}
}

func (f curatorFixture) land(t *testing.T, msg string) string {
	t.Helper()
	f.repo.Write("go/internal/fleet/pool.go", "package fleet\n\nfunc Backfill() {}\n")
	f.repo.Write("go/internal/fleet/pool_test.go", "package fleet\n\nfunc TestPool_AFreeSlotTakesTheNextTodo(t *testing.T) {}\n")
	f.repo.Git("add", "go")
	f.repo.Git("commit", "-q", "-m", msg)
	return f.repo.Git("rev-parse", "HEAD")
}

func (f curatorFixture) curate(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runInbox(append([]string{"curate", "--project-root", f.root}, args...), strings.NewReader(stdin), &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func (f curatorFixture) pending(t *testing.T) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(f.root, ".evolve", "inbox", "2026-10-01T00-00-00Z-rolling-pool.json"))
	return err == nil
}

func TestCmd_InboxCurate_TheBoundaryConsumesAnItemWhoseAcceptanceMergedInEnforce(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	if rc, _, stderr := f.curate(t, "", "--apply"); rc != 0 {
		t.Fatalf("the first boundary: rc = %d stderr = %q", rc, stderr)
	}
	f.land(t, "feat: backfill the pool")

	rc, stdout, stderr := f.curate(t, "", "--apply")

	if rc != 0 || f.pending(t) || !strings.Contains(stdout, "consume rolling-pool (landing, applied)") {
		t.Fatalf("rc = %d stdout = %q stderr = %q, want rolling-pool consumed by the landing proposal", rc, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".evolve", "inbox", "consumed", "2026-10-01T00-00-00Z-rolling-pool.json")); err != nil {
		t.Errorf("the item is not in consumed/: %v", err)
	}
}

func TestCmd_InboxCurate_TheShadowStageProposesAndAppliesNothing(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "shadow")
	f.curate(t, "", "--apply")
	f.land(t, "feat: backfill the pool")

	rc, stdout, _ := f.curate(t, "", "--apply", "--json")

	var report curateReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || rc != 0 {
		t.Fatalf("rc = %d stdout = %q (%v)", rc, stdout, err)
	}
	if !f.pending(t) || len(report.Proposals) != 1 || !report.Proposals[0].Deterministic || report.Proposals[0].Applied || report.Stage != "shadow" {
		t.Errorf("report = %+v, want one deterministic proposal that shadow does not apply", report)
	}
}

func TestCmd_InboxCurate_ApplyIsRefusedWhileALaneIsLive(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	f.curate(t, "", "--apply")
	f.land(t, "feat: backfill the pool")
	writeLiveRunLease(t, f.root)

	rc, _, stderr := f.curate(t, "", "--apply")

	if rc != 1 || !f.pending(t) || !strings.Contains(stderr, "a loop lane is live") {
		t.Errorf("rc = %d stderr = %q, want 1 with the item still pending", rc, stderr)
	}
}

func TestCmd_InboxCurate_AShipLandedEventProposesAndNeverApplies(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	sha := f.land(t, "feat: backfill the pool")
	record := `{"source":"signal","signal":{"kind":"ship.landed","module":"ship","fields":{"commit":"` + sha + `"}}}`

	rc, stdout, stderr := f.curate(t, record, "--event")

	if rc != 0 || !f.pending(t) || !strings.Contains(stdout, "consume rolling-pool (landing, deterministic)") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want a deterministic proposal left for the boundary", rc, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".evolve", curatorCursorFile)); !os.IsNotExist(err) {
		t.Errorf("the event moved the cursor: %v", err)
	}
}

func TestCmd_InboxCurate_AnEventThatIsNotALandingIsANoOp(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")

	rc, stdout, _ := f.curate(t, `{"source":"signal","signal":{"kind":"cycle.sealed"}}`, "--event")

	if rc != 0 || !strings.Contains(stdout, "0 proposals") {
		t.Errorf("rc = %d stdout = %q, want no proposal", rc, stdout)
	}
}

func TestCmd_InboxCurate_ThePlainVerbIsReadOnly(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	f.land(t, "feat: backfill the pool")

	rc, stdout, _ := f.curate(t, "")

	if rc != 0 || !f.pending(t) || !strings.Contains(stdout, "consume rolling-pool (sweep, judgment)") {
		t.Errorf("rc = %d stdout = %q, want a sweep proposal and the item untouched", rc, stdout)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".evolve", curatorCursorFile)); !os.IsNotExist(err) {
		t.Errorf("the plain verb wrote the cursor: %v", err)
	}
}

func TestCmd_InboxCurate_RefusesBadArguments(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	for name, args := range map[string][]string{
		"apply with event":  {"--apply", "--event"},
		"event with follow": {"--event", "--follow"},
		"a bad since":       {"--follow", "--since", "yesterday"},
		"an unknown flag":   {"--bogus"},
		"an operand":        {"x"},
	} {
		if rc, _, _ := f.curate(t, "", args...); rc != 10 {
			t.Errorf("%s: rc = %d, want 10", name, rc)
		}
	}
}

func TestCmd_InboxCurate_ALostCursorIsWarnedAndRestartsAtHEAD(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{"an unknown commit": `{"head":"0123456789abcdef0123456789abcdef01234567"}`, "a torn file": `{"head":`, "an option": `{"head":"--output=x"}`, "a symbolic name": `{"head":"HEAD"}`} {
		f := newCuratorFixture(t, "enforce")
		if err := os.WriteFile(filepath.Join(f.root, ".evolve", curatorCursorFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}

		rc, stdout, stderr := f.curate(t, "", "--apply")

		after, _ := os.ReadFile(filepath.Join(f.root, ".evolve", curatorCursorFile))
		if rc != 0 || !strings.Contains(stderr, "the range is lost") || !strings.Contains(stdout, "inbox curate: stage enforce") || !strings.Contains(string(after), f.repo.Git("rev-parse", "HEAD")) {
			t.Errorf("%s: rc = %d stdout = %q stderr = %q cursor = %s, want a WARN and the cursor at HEAD", name, rc, stdout, stderr, after)
		}
	}
}

func TestCmd_InboxCurate_AGitFaultOnTheRangeKeepsTheCursor(t *testing.T) {
	f := newCuratorFixture(t, "enforce")
	base := f.repo.Git("rev-parse", "HEAD")
	cursor := `{"head":"` + base + `"}`
	if err := os.WriteFile(filepath.Join(f.root, ".evolve", curatorCursorFile), []byte(cursor), 0o644); err != nil {
		t.Fatal(err)
	}
	f.land(t, "feat: backfill the pool")
	gittest.FaultOn(t, "log")

	rc, _, stderr := f.curate(t, "", "--apply")

	after, _ := os.ReadFile(filepath.Join(f.root, ".evolve", curatorCursorFile))
	if rc != 0 || !strings.Contains(stderr, "the cursor stays") || string(after) != cursor {
		t.Errorf("rc = %d stderr = %q cursor = %s, want the cursor kept after a git fault", rc, stderr, after)
	}
}

func TestCmd_InboxCurate_TheBoundaryCommitsWhatItApplied(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	f.curate(t, "", "--apply")
	f.land(t, "feat: backfill the pool")

	if rc, _, stderr := f.curate(t, "", "--apply"); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}

	status := f.repo.Git("status", "--porcelain", "--", ".evolve/inbox")
	if status != "" || f.repo.Git("log", "-1", "--format=%s") != curatorCommitMessage {
		t.Errorf("status = %q, last commit = %q; want a clean inbox and the curator's commit, so the next sync-main is not refused", status, f.repo.Git("log", "-1", "--format=%s"))
	}
}

func TestCmd_InboxCurate_AFailedCommitOfTheAppliedProposalsExitsIO(t *testing.T) {
	f := newCuratorFixture(t, "enforce")
	f.curate(t, "", "--apply")
	f.land(t, "feat: backfill the pool")
	gittest.FaultOn(t, "commit")

	if rc, _, stderr := f.curate(t, "", "--apply"); rc != 2 || !strings.Contains(stderr, "commit the applied proposals") {
		t.Errorf("rc = %d stderr = %q, want 2 naming the commit", rc, stderr)
	}
}

func TestCmd_InboxCurate_AFailedStageOfTheAppliedProposalsExitsIO(t *testing.T) {
	f := newCuratorFixture(t, "enforce")
	f.curate(t, "", "--apply")
	f.land(t, "feat: backfill the pool")
	gittest.FaultOn(t, "add")

	if rc, _, stderr := f.curate(t, "", "--apply"); rc != 2 || !strings.Contains(stderr, "stage the applied proposals") {
		t.Errorf("rc = %d stderr = %q, want 2 naming the stage", rc, stderr)
	}
}

func TestCmd_InboxCurate_AFailedConsumeKeepsTheCursor(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	f.curate(t, "", "--apply")
	before, _ := os.ReadFile(filepath.Join(f.root, ".evolve", curatorCursorFile))
	f.land(t, "feat: backfill the pool")
	if err := os.WriteFile(filepath.Join(f.root, ".evolve", "inbox", "consumed"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, _, stderr := f.curate(t, "", "--apply")

	after, _ := os.ReadFile(filepath.Join(f.root, ".evolve", curatorCursorFile))
	if rc != 0 || !f.pending(t) || string(after) != string(before) || !strings.Contains(stderr, "the cursor stays") {
		t.Errorf("rc = %d stderr = %q, want the item pending and the cursor kept for a retry", rc, stderr)
	}
}

func plainCuratorRoot(t *testing.T, policy string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".evolve/inbox", "go"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func curateAt(root, stdin string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	rc := runInbox(append([]string{"curate", "--project-root", root}, args...), strings.NewReader(stdin), &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestCmd_InboxCurate_AGateFileItemOnTheLaneRouteIsReroutedToTheConsoleAtTheBoundary(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	gate := `{"id":"gate-item","kind":"feature","priority_class":"stability","weight":0.5,"origin":"console","route":"lane",` +
		`"title":"Tighten the ship guard","files":["go/internal/guards/ship.go"],"summary":"s","fix":"f","acceptance":["a"]}`
	f.repo.Write(".evolve/inbox/2026-10-02T00-00-00Z-gate-item.json", gate)
	f.repo.Git("add", ".evolve")
	f.repo.Git("commit", "-q", "-m", "gate item")

	rc, stdout, stderr := f.curate(t, "", "--apply")

	raw, err := os.ReadFile(filepath.Join(f.root, ".evolve", "inbox", "2026-10-02T00-00-00Z-gate-item.json"))
	if rc != 0 || err != nil || !strings.Contains(string(raw), `"route":"console-manual"`) || !strings.Contains(stdout, "reroute gate-item -> console (routing, applied)") {
		t.Errorf("rc = %d stdout = %q stderr = %q item = %s, want the item rerouted to the console", rc, stdout, stderr, raw)
	}
}

func TestCmd_InboxCurate_AnInputThatCannotBeReadExitsIO(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		prep  func(root string)
		args  []string
		stdin string
		want  string
	}{
		"a malformed policy": {func(root string) {
			_ = os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"inbox":{"curator":{"stage":"off"}}}`), 0o644)
		}, nil, "", "stage"},
		"an inbox that is a file": {func(root string) {
			_ = os.RemoveAll(filepath.Join(root, ".evolve", "inbox"))
			_ = os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("x"), 0o644)
		}, nil, "", "inbox"},
		"a torn wire record":     {func(string) {}, []string{"--event"}, "{", "wire record"},
		"a cursor that is a dir": {func(root string) { _ = os.MkdirAll(filepath.Join(root, ".evolve", curatorCursorFile), 0o755) }, nil, "", curatorCursorFile},
		"apply with no git":      {func(string) {}, []string{"--apply"}, "", "HEAD"},
	} {
		root := plainCuratorRoot(t, `{}`)
		tc.prep(root)

		if rc, _, stderr := curateAt(root, tc.stdin, tc.args...); rc != 2 || !strings.Contains(stderr, tc.want) {
			t.Errorf("%s: rc = %d stderr = %q, want 2 naming %q", name, rc, stderr, tc.want)
		}
	}
}

func TestCuratorRun_AFailedApplyIsNotApplied(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	var stderr bytes.Buffer
	run := curatorRun{req: curateRequest{root: root}, stderr: &stderr}

	consume := run.applyProposal(inboxcurator.Proposal{Kind: inboxcurator.ProposeConsume, ID: "gone", Path: "gone.json"}, nil)
	reroute := run.applyProposal(inboxcurator.Proposal{Kind: inboxcurator.ProposeReroute, ID: "gone", Target: inboxcurator.RouteConsoleTarget}, nil)
	toLane := run.applyProposal(inboxcurator.Proposal{Kind: inboxcurator.ProposeReroute, ID: "gone", Target: inboxcurator.RouteLaneTarget, Deterministic: true}, nil)
	merge := run.applyProposal(inboxcurator.Proposal{Kind: inboxcurator.ProposeMerge, ID: "gone"}, nil)

	if consume || reroute || toLane || merge || !strings.Contains(stderr.String(), "reroute gone") {
		t.Errorf("applied = %v %v %v %v stderr = %q, want nothing applied and the reroute error named", consume, reroute, toLane, merge, stderr.String())
	}
}

func TestCmd_InboxCurate_TheRootComesFromTheEnvironmentWhenNotNamed(t *testing.T) {
	root := plainCuratorRoot(t, `{}`)
	onlyHere := `{"id":"only-here","kind":"feature","route":"console-manual","files":["go/a.go"],"title":"t","acceptance":["a"]}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "a-only-here.json"), []byte(onlyHere), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", curatorCursorFile), []byte(`{"head":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"curate"}, strings.NewReader(""), &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "reroute only-here -> lane") || strings.Contains(stderr.String(), "WARN inbox") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want the item of the env root and no cursor warning", rc, stdout.String(), stderr.String())
	}
}

func TestCmd_InboxCurate_AnItemConsumedOnceIsNotReroutedInTheSameRun(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	gate := `{"id":"gate-fix","kind":"feature","priority_class":"stability","weight":0.5,"origin":"console","route":"lane",` +
		`"title":"Tighten the ship guard","files":["go/internal/guards/ship.go"],"summary":"s","fix":"f","acceptance":["TestShipGuard_Tight passes"]}`
	f.curate(t, "", "--apply")
	f.repo.Write(".evolve/inbox/2026-10-02T00-00-00Z-gate-fix.json", gate)
	f.repo.Write("go/internal/guards/ship.go", "package guards\n")
	f.repo.Write("go/internal/guards/ship_test.go", "package guards\n\nfunc TestShipGuard_Tight(t *testing.T) {}\n")
	f.repo.Git("add", ".")
	f.repo.Git("commit", "-q", "-m", "tighten the guard")

	rc, stdout, stderr := f.curate(t, "", "--apply", "--json")

	var report curateReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || rc != 0 {
		t.Fatalf("rc = %d stdout = %q stderr = %q (%v): the JSON report must stay parseable after an applied consume", rc, stdout, stderr, err)
	}
	kinds := map[string]bool{}
	for _, p := range report.Proposals {
		if p.ID == "gate-fix" {
			kinds[string(p.Kind)+"="+strconv.FormatBool(p.Applied)] = true
		}
	}
	if !kinds["consume=true"] || !kinds["reroute=false"] || strings.Contains(stderr, "reroute gate-fix") {
		t.Errorf("proposals of gate-fix = %v stderr = %q, want the consume applied and the reroute skipped", kinds, stderr)
	}
}

func TestCmd_InboxCurate_AFileThatDoesNotLoadIsWarned(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "torn.json"), []byte(`{"id":`), 0o644); err != nil {
		t.Fatal(err)
	}

	if rc, _, stderr := curateAt(root, ""); rc != 0 || !strings.Contains(stderr, "WARN") || !strings.Contains(stderr, "torn.json") {
		t.Errorf("rc = %d stderr = %q, want a WARN naming torn.json", rc, stderr)
	}
}

func TestCmd_InboxCurate_AnUntrackedItemIsConsumedAndItsCopyCommitted(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	f.curate(t, "", "--apply")
	f.repo.Git("rm", "-q", "--cached", ".evolve/inbox/2026-10-01T00-00-00Z-rolling-pool.json")
	f.repo.Git("commit", "-q", "-m", "untrack the item")
	f.land(t, "feat: backfill the pool")

	rc, _, stderr := f.curate(t, "", "--apply")

	if rc != 0 || f.pending(t) || f.repo.Git("log", "-1", "--format=%s") != curatorCommitMessage {
		t.Errorf("rc = %d stderr = %q, want the untracked item consumed and its consumed copy committed", rc, stderr)
	}
}

func TestCmd_InboxCurate_AProjectWithNoGoTreeStillCurates(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	if err := os.RemoveAll(filepath.Join(root, "go")); err != nil {
		t.Fatal(err)
	}

	if rc, stdout, stderr := curateAt(root, ""); rc != 0 || !strings.Contains(stdout, "0 proposals") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want an empty curation", rc, stdout, stderr)
	}
}

func TestCmd_InboxCurate_AnUnreadableTestFileExitsIO(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	file := filepath.Join(root, "go", "a_test.go")
	if err := os.WriteFile(file, []byte("package a\n"), 0o000); err != nil {
		t.Fatal(err)
	}

	if rc, _, stderr := curateAt(root, ""); rc != 2 || !strings.Contains(stderr, "index the tests") {
		t.Errorf("rc = %d stderr = %q, want 2 naming the test index", rc, stderr)
	}
}
