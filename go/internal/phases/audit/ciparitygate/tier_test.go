package ciparitygate

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

// Test 20 (moved intent: audit/ciparity_unit_test.go:421) — every attempt runs
// under the scrubbed allowlist env, captured ONCE at wrap time and identical on
// both attempts: PATH and GOFLAGS survive in allowlist order, a lane-leaked
// EVOLVE_* canary does not, and nil (inherit) is never passed.
func TestTierAttempts_ScrubbedEnvIsCapturedOnceAndIdenticalOnBothAttempts(t *testing.T) {
	t.Setenv("EVOLVE_LEAK_CANARY", "1")
	t.Setenv("GOFLAGS", "-mod=mod")
	root, _ := goWorktree(t)
	fn, calls, envs := seqRunFunc(t, []step{{1, "--- FAIL: TestFlaky (0.00s)\n"}, {0, "ok\n"}})
	g := New(fn, fixedSet("./internal/widget/..."))
	if _, err := g.IntegrationTier(tierRequest(root, t.TempDir())); err == nil || *calls != 2 {
		t.Fatalf("red-then-green: err=%v calls=%d", err, *calls)
	}
	first, second := (*envs)[0], (*envs)[1]
	if first == nil || strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Fatalf("both attempts carry the SAME explicit scrubbed env:\n%v\n%v", first, second)
	}
	joined := strings.Join(first, "\n")
	if !strings.Contains(joined, "PATH=") || !strings.Contains(joined, "GOFLAGS=-mod=mod") || strings.Contains(joined, "EVOLVE_LEAK_CANARY") {
		t.Errorf("scrubbed env: %v", first)
	}
	rank := map[string]int{}
	for i, k := range tierEnvAllowlist {
		rank[k] = i
	}
	last := -1
	for _, kv := range first {
		k, _, _ := strings.Cut(kv, "=")
		if r, ok := rank[k]; !ok || r < last {
			t.Fatalf("env %v is not in allowlist order / carries a foreign key %q", first, k)
		} else {
			last = r
		}
	}
	// The clean env is captured when the runner is wrapped, not per call: a
	// change between the attempts (the first attempt mutates GOFLAGS) never
	// reaches the retake.
	var seen [][]string
	mutating := func(_ context.Context, _, _ string, _, env []string, _ io.Reader, so, _ io.Writer) (int, error) {
		seen = append(seen, env)
		os.Setenv("GOFLAGS", "-mod=vendor")
		if len(seen) == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestFlaky (0.00s)\n")
			return 1, nil
		}
		return 0, nil
	}
	if _, err := New(mutating, fixedSet("./internal/widget/...")).IntegrationTier(tierRequest(root, "")); err == nil || len(seen) != 2 {
		t.Fatalf("red-then-green: %v", err)
	}
	if strings.Join(seen[1], "\n") != strings.Join(seen[0], "\n") || !strings.Contains(strings.Join(seen[1], "\n"), "GOFLAGS=-mod=mod") {
		t.Errorf("the retake must carry the env captured at wrap time, not a re-read: %v", seen[1])
	}
}

// Test 21 — integration-tier.log is byte-identical to the goldens for the
// red-then-green and red-then-red runs; append-only, 0o644.
func TestTierLog_BytesMatchTheGoldenForBothOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script []step
		golden string
	}{
		{"red-green", []step{{1, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n"}, {0, "ok\n"}}, "tier-log-red-green.golden.txt"},
		{"red-red", []step{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, {1, "--- FAIL: TestGenuine (0.01s)\nFAIL\tpkg\t2.0s\n"}}, "tier-log-red-red.golden.txt"},
	} {
		root, _ := goWorktree(t)
		ws := t.TempDir()
		logPath := filepath.Join(ws, "integration-tier.log")
		if err := os.WriteFile(logPath, []byte("PRIOR\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fn, _, _ := seqRunFunc(t, tc.script)
		_, _ = New(fn, fixedSet("./internal/widget/...")).IntegrationTier(tierRequest(root, ws))
		got, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("testdata", tc.golden))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "PRIOR\n"+string(want) {
			t.Errorf("%s: log drifted (append-only) from the golden:\n got %q\nwant %q", tc.name, got, "PRIOR\n"+string(want))
		}
		if fi, err := os.Stat(logPath); err != nil || fi.Mode().Perm() != 0o644 {
			t.Errorf("%s: mode %v", tc.name, fi.Mode())
		}
	}
}

// Test 22 — log failures are coded and never fatal: a regular file as the
// Workspace fails both appends (ONE TIER_LOG_WRITE_FAILED per attempt, the
// verdict still returned, `where` == "integration-tier.log unavailable");
// Workspace == "" is a declared no-op (no event, no file); a log replaced by
// a directory between the attempts fails ONLY attempt 2 and the earlier
// pointer stands in the offenders.
func TestTierLog_FailuresAreCodedAndTheEarlierPointerStands(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, _ := goWorktree(t)
	wsFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(wsFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fn, _, _ := seqRunFunc(t, []step{{1, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n"}, {0, "ok\n"}})
	g, events := observed(t, fn, fixedSet("./internal/widget/..."))
	if _, err := g.IntegrationTier(Request{3, root, root, wsFile}); err == nil || err.Error() != g1["tier.flake.no_log"] {
		t.Fatalf("unwritable workspace: %v", err)
	}
	var attempts []string
	for _, e := range *events {
		if e.Code == CodeTierLogWriteFailed {
			attempts = append(attempts, e.Fields["attempt"])
			if e.Fields["path"] != filepath.Join(wsFile, "integration-tier.log") || e.Fields["err"] == "" || !strings.Contains(e.Reason, "integration-tier.log") {
				t.Errorf("TIER_LOG_WRITE_FAILED fields: %+v", e)
			}
		}
	}
	if strings.Join(attempts, ",") != "1,2" {
		t.Errorf("one event per failed append, got attempts %v in %v", attempts, codesOf(*events))
	}

	fn, _, _ = seqRunFunc(t, []step{{1, "--- FAIL: TestFlaky (0.00s)\n"}, {0, "ok\n"}})
	g, events = observed(t, fn, fixedSet("./internal/widget/..."))
	if _, err := g.IntegrationTier(tierRequest(root, "")); err == nil || err.Error() != g1["tier.flake.no_log"] {
		t.Fatalf("empty workspace: %v", err)
	}
	for _, e := range *events {
		if e.Code == CodeTierLogWriteFailed {
			t.Errorf("Workspace == \"\" is a declared no-op, not a fault: %+v", e)
		}
	}

	ws := t.TempDir()
	logPath := filepath.Join(ws, "integration-tier.log")
	calls := 0
	g, events = observed(t, func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		calls++
		if calls == 2 { // between the attempts: attempt 1 is on disk; replace the log by a directory
			if err := os.Remove(logPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(logPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		_, _ = io.WriteString(so, "--- FAIL: TestGenuine (0.01s)\n")
		return 1, nil
	}, fixedSet("./internal/widget/..."))
	off, err := g.IntegrationTier(tierRequest(root, ws))
	if err != nil || len(off) == 0 || off[len(off)-1] != "full output: "+logPath {
		t.Fatalf("the earlier pointer stands: (%v, %v)", off, err)
	}
	e := only(t, *events, CodeTierLogWriteFailed)
	if e.Fields["attempt"] != "2" {
		t.Errorf("only attempt 2's append failed: %+v", e)
	}
}

// Test 23 (moved: audit/ciparity_unit_test.go:513-539 and the pre-move pin
// TestAcquireTierLock_PrefersProjectRootOverWorktree, now over the injected
// clock) — the lock table: a clean acquire, release re-acquires; a held lock
// times out after TierLockWait with the 2 s poll observed; a regular file as
// ProjectRoot fails flock's mkdir; no root at all; the lock lives under
// ProjectRoot, never the worktree; the real flock path serializes and releases.
func TestAcquireLock_TableOverAnInjectedClock(t *testing.T) {
	shared, lane := t.TempDir(), t.TempDir()
	g, events := observed(t, nil, nil)
	release, note := g.acquireLock(gateTier, Request{1, shared, lane, ""})
	if note != "" || len(*events) != 0 {
		t.Fatalf("first acquire must succeed cleanly: note=%q events=%v", note, codesOf(*events))
	}
	if _, err := os.Stat(filepath.Join(shared, ".evolve", "locks", "integration-tier.lock")); err != nil {
		t.Errorf("the lock lives under ProjectRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lane, ".evolve", "locks", "integration-tier.lock")); !os.IsNotExist(err) {
		t.Errorf("the lock must NOT live under the per-lane Worktree (err=%v)", err)
	}

	// Held → poll → timeout on the injected clock.
	tick := 0
	base := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	var slept []time.Duration
	held, hevents := observed(t, nil, nil, WithClock(func() time.Time { tick++; return base.Add(time.Duration(tick-1) * 3 * time.Minute) }, func(d time.Duration) { slept = append(slept, d) }))
	rel2, note2 := held.acquireLock(gateTier, Request{1, shared, "", ""})
	rel2()
	if note2 != ", lock wait timed out (retake unserialized)" || len(slept) != 1 || slept[0] != 2*time.Second {
		t.Fatalf("held lock: note=%q slept=%v (deadline now+5m, clock steps 3m: one poll then timeout)", note2, slept)
	}
	e := only(t, *hevents, CodeTierLockUnavailable)
	if e.Fields["reason"] != "timeout" || e.Fields["wait"] != "5m0s" || e.Fields["path"] != filepath.Join(shared, ".evolve", "locks", "integration-tier.lock") || e.Reason != "lock wait timed out (retake unserialized)" {
		t.Errorf("timeout event: %+v", e)
	}
	release()
	rel3, note3 := g.acquireLock(gateTier, Request{1, shared, "", ""})
	if note3 != "" {
		t.Fatalf("post-release acquire must succeed, note=%q", note3)
	}
	rel3()

	// A regular file as the root: flock's mkdir fails → reason=error.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	*events = nil
	rel4, note4 := g.acquireLock(gateTier, Request{1, file, "", ""})
	rel4()
	if !strings.HasPrefix(note4, ", lock unavailable: flock mkdir: ") {
		t.Fatalf("unusable root: note=%q", note4)
	}
	e = only(t, *events, CodeTierLockUnavailable)
	if e.Fields["reason"] != "error" || e.Fields["err"] == "" || e.Reason != strings.TrimPrefix(note4, ", ") {
		t.Errorf("error event: %+v", e)
	}

	*events = nil
	rel5, note5 := g.acquireLock(gateTier, Request{1, "", "", ""})
	rel5()
	if note5 != ", lock unavailable: no project root" || only(t, *events, CodeTierLockUnavailable).Fields["reason"] != "no_root" {
		t.Errorf("no root: note=%q events=%v", note5, *events)
	}

	// The real flock path: the leaf's lock and a second process-local holder exclude each other.
	rel6, note6 := g.acquireLock(gateTier, Request{1, shared, "", ""})
	if note6 != "" {
		t.Fatal(note6)
	}
	if _, isHeld, err := flock.TryLock(filepath.Join(shared, ".evolve", "locks", "integration-tier.lock")); err != nil || !isHeld {
		t.Errorf("the leaf's lock is a real flock: held=%v err=%v", isHeld, err)
	}
	rel6()
}

// Test 24 — decideTier is the PURE four-way table over two attempts: a retake
// that could not start → attempt 1's offenders + pointer; red-then-green → the
// golden flake text (both `where` variants); a deadline kill with markers →
// the retake's offenders; without markers → the golden budget text; red-red →
// the retake's offenders.
func TestDecideTier_FourWayTable(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	first := attempt{out: "--- FAIL: TestFirstAttempt (0.00s)\nFAIL\tpkg\t1.0s\n", code: 1}
	const logPath = "/ws/integration-tier.log"
	d := decideTier(first, attempt{code: -1, err: errors.New("fork failed")}, logPath, time.Minute)
	if d.code != CodeTierRetakeExecFailed || d.cause != "retake_exec_failed" || d.err != nil || strings.Join(d.offenders, "\n") != strings.ReplaceAll(g1["tier.retake_exec_failed.offenders"], "{WS}", "/ws") {
		t.Errorf("retake exec failure: %+v", d)
	}
	d = decideTier(first, attempt{out: "ok\n"}, logPath, time.Minute)
	if d.code != CodeTierFlakeAbsorbed || d.offenders != nil || d.err == nil || d.err.Error() != strings.ReplaceAll(g1["tier.flake.with_log"], "{WS}", "/ws") {
		t.Errorf("flake with log: %+v", d)
	}
	if d = decideTier(first, attempt{out: "ok\n"}, "", time.Minute); d.err == nil || d.err.Error() != g1["tier.flake.no_log"] {
		t.Errorf("flake without log: %+v", d)
	}
	judged := attempt{out: "--- FAIL: TestTruncatedButJudged (0.02s)\nFAIL\tpkg\t3.0s\nsignal: killed\n", code: -1, deadlineHit: true}
	d = decideTier(first, judged, logPath, time.Nanosecond)
	if d.code != "" || d.cause != "deadline_with_markers" || strings.Join(d.offenders, "\n") != strings.ReplaceAll(g1["tier.deadline.with_markers.offenders"], "{WS}", "/ws") {
		t.Errorf("deadline with markers: %+v", d)
	}
	silent := attempt{out: "partial toolchain chatter, no verdict lines\nsignal: killed\n", code: -1, deadlineHit: true}
	d = decideTier(first, silent, logPath, time.Nanosecond)
	if d.code != CodeTierDeadlineNoVerdict || d.offenders != nil || d.err == nil || d.err.Error() != strings.ReplaceAll(g1["tier.deadline.no_verdict.with_log"], "{WS}", "/ws") {
		t.Errorf("deadline without markers: %+v", d)
	}
	if d = decideTier(first, silent, "", time.Nanosecond); d.err == nil || d.err.Error() != g1["tier.deadline.no_verdict.no_log"] {
		t.Errorf("deadline without markers, no log: %+v", d)
	}
	red := attempt{out: "--- FAIL: TestGenuine (0.01s)\nFAIL\tpkg\t2.0s\n", code: 1}
	d = decideTier(first, red, logPath, time.Minute)
	if d.code != "" || d.cause != "retake_red" || strings.Join(d.offenders, "\n") != strings.ReplaceAll(g1["tier.redred.offenders"], "{WS}", "/ws") {
		t.Errorf("red-red: %+v", d)
	}
	// deadlineHit on attempt 1 is recorded but never consulted (the original never checked attempt 1's ctx).
	if d = decideTier(attempt{code: 1, deadlineHit: true}, red, "", time.Minute); d.cause != "retake_red" {
		t.Errorf("attempt 1's deadline flag is inert: %+v", d)
	}
}

// Test 25 (moved intent: audit/ciparity_unit_test.go:421/:454/:485 and the
// pre-move pins TestIntegrationTier_RetakeExecFailure_* / _RetakeUsesAFreshBudget)
// — end to end through fakes: green → one run, no event; red-then-green → two
// runs, the golden flake WARN, ONE FLAKE_ABSORBED; red-then-red → the retake's
// offenders + pointer, ONE GATE_FAILED{retake_red}; a retake start error →
// ONE TIER_RETAKE_EXEC_FAILED then ONE GATE_FAILED{retake_exec_failed} on
// attempt 1's offenders, the lock released; the retake carries a FRESH
// deadline; a 1 ns budget → the deadline arms with the golden texts.
func TestIntegrationTier_EndToEndThroughFakes(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, _ := goWorktree(t)
	ws := t.TempDir()
	req := tierRequest(root, ws)
	logPath := filepath.Join(ws, "integration-tier.log")

	fn, calls, _ := seqRunFunc(t, []step{{0, "ok"}})
	g, events := observed(t, fn, fixedSet("./internal/widget/..."))
	if off, err := g.IntegrationTier(req); off != nil || err != nil || *calls != 1 || len(*events) != 0 {
		t.Fatalf("green: (%v, %v) calls=%d events=%v", off, err, *calls, codesOf(*events))
	}

	fn, calls, _ = seqRunFunc(t, []step{{1, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n"}, {0, "ok\n"}})
	g, events = observed(t, fn, fixedSet("./internal/widget/..."))
	off, err := g.IntegrationTier(req)
	if off != nil || err == nil || err.Error() != fill(g1["tier.flake.with_log"], root, ws) || *calls != 2 {
		t.Fatalf("red-then-green: (%v, %v) calls=%d", off, err, *calls)
	}
	e := only(t, *events, CodeTierFlakeAbsorbed)
	if e.Fields["log"] != logPath || e.Fields["attempt1_exit"] != "1" || e.Fields["lock_note"] != "" || e.Reason != err.Error() || len(*events) != 1 {
		t.Errorf("FLAKE_ABSORBED: %+v (events %v)", e, codesOf(*events))
	}

	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	fn, calls, _ = seqRunFunc(t, []step{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, {1, "--- FAIL: TestGenuine (0.01s)\nFAIL\tpkg\t2.0s\n"}})
	g, events = observed(t, fn, fixedSet("./internal/widget/..."))
	off, err = g.IntegrationTier(req)
	if err != nil || strings.Join(off, "\n") != fill(g1["tier.redred.offenders"], root, ws) || *calls != 2 {
		t.Fatalf("red-then-red: (%v, %v) calls=%d", off, err, *calls)
	}
	e = only(t, *events, CodeGateFailed)
	if e.Fields["cause"] != "retake_red" || e.Fields["offenders"] != "3" || e.Fields["first"] != off[0] || e.Fields["log"] != logPath || len(*events) != 1 {
		t.Errorf("GATE_FAILED: %+v (events %v)", e, codesOf(*events))
	}

	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	n := 0
	var deadlines []time.Time
	g, events = observed(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		n++
		d, _ := ctx.Deadline()
		deadlines = append(deadlines, d)
		time.Sleep(2 * time.Millisecond)
		if n == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestFirstAttempt (0.00s)\nFAIL\tpkg\t1.0s\n")
			return 1, nil
		}
		return 0, errors.New("exec: fork/exec go: resource temporarily unavailable")
	}, fixedSet("./internal/widget/..."))
	off, err = g.IntegrationTier(req)
	if err != nil || strings.Join(off, "\n") != fill(g1["tier.retake_exec_failed.offenders"], root, ws) {
		t.Fatalf("retake exec failure: (%v, %v)", off, err)
	}
	if got := codesOf(*events); strings.Join(got, ",") != string(CodeTierRetakeExecFailed)+","+string(CodeGateFailed) {
		t.Errorf("two facts, two events in order: %v", got)
	}
	if e = only(t, *events, CodeTierRetakeExecFailed); e.Fields["err"] != "exec: fork/exec go: resource temporarily unavailable" || e.Fields["log"] != logPath {
		t.Errorf("TIER_RETAKE_EXEC_FAILED: %+v", e)
	}
	if e = only(t, *events, CodeGateFailed); e.Fields["cause"] != "retake_exec_failed" {
		t.Errorf("GATE_FAILED after the retake failure: %+v", e)
	}
	if logB, _ := os.ReadFile(logPath); !strings.Contains(string(logB), "# attempt 1") || strings.Contains(string(logB), "# attempt 2") {
		t.Errorf("the log holds attempt 1 only when the retake never ran:\n%s", logB)
	}
	if len(deadlines) != 2 || !deadlines[1].After(deadlines[0]) {
		t.Errorf("the retake carries a fresh, later deadline: %v", deadlines)
	}
	rel, isHeld, lerr := flock.TryLock(filepath.Join(root, ".evolve", "locks", "integration-tier.lock"))
	if lerr != nil || isHeld {
		t.Fatalf("the lock is released by the time the gate returns (held=%v err=%v)", isHeld, lerr)
	}
	rel()

	budget := DefaultTimeouts()
	budget.TierAttempt = time.Nanosecond
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	fn, _, _ = seqRunFunc(t, []step{{1, "--- FAIL: TestSlowRed (0.00s)\nFAIL\tpkg\t1.0s\n"}, {-1, "partial toolchain chatter, no verdict lines\nsignal: killed\n"}})
	g, events = observed(t, killedAtDeadline(fn), fixedSet("./internal/widget/..."), WithTimeouts(budget))
	off, err = g.IntegrationTier(req)
	if off != nil || err == nil || err.Error() != fill(g1["tier.deadline.no_verdict.with_log"], root, ws) {
		t.Fatalf("deadline, no verdict: (%v, %v)", off, err)
	}
	if e = only(t, *events, CodeTierDeadlineNoVerdict); e.Fields["budget"] != "1ns" || e.Fields["log"] != logPath || e.Reason != err.Error() {
		t.Errorf("TIER_DEADLINE_NO_VERDICT: %+v", e)
	}
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	fn, _, _ = seqRunFunc(t, []step{{1, "--- FAIL: TestSlowRed (0.00s)\n"}, {-1, "--- FAIL: TestTruncatedButJudged (0.02s)\nFAIL\tpkg\t3.0s\nsignal: killed\n"}})
	g, events = observed(t, killedAtDeadline(fn), fixedSet("./internal/widget/..."), WithTimeouts(budget))
	off, err = g.IntegrationTier(req)
	if err != nil || strings.Join(off, "\n") != fill(g1["tier.deadline.with_markers.offenders"], root, ws) {
		t.Fatalf("deadline with markers: (%v, %v)", off, err)
	}
	if e = only(t, *events, CodeGateFailed); e.Fields["cause"] != "deadline_with_markers" {
		t.Errorf("GATE_FAILED: %+v", e)
	}

	// Attempt 1 cannot start → GATE_STEP_FAILED{step=tier_attempt}.
	g, events = observed(t, fakeRunFunc(-1, "", "", errors.New("executable file not found")), fixedSet("./internal/widget/..."))
	if off, err := g.IntegrationTier(req); off != nil || err == nil || err.Error() != g1["tier.exec_failed"] {
		t.Fatalf("attempt 1 start failure: (%v, %v)", off, err)
	}
	if e = only(t, *events, CodeGateStepFailed); e.Fields["step"] != "tier_attempt" || !strings.HasPrefix(e.Fields["cmd"], "go test -race -count=1 -p 4 -parallel 4 -tags integration") {
		t.Errorf("GATE_STEP_FAILED: %+v", e)
	}
}
