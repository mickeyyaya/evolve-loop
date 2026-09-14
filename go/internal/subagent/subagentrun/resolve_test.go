package subagentrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Test 22 — the profile error keeps the cause-less text and the signal
// carries the cause.
func TestResolve_ProfileErrorKeepsTextAndCarriesCause(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.Profile = func(p string) (Profile, error) {
		return Profile{}, &os.PathError{Op: "open", Path: p, Err: syscall.EACCES}
	}
	d, r := observed(t, deps)
	_, err := d.Dispatch(context.Background(), f.request())
	if err == nil || err.Error() != "subagent/run: profile not found: /p/scout.json" {
		t.Fatalf("the returned text drops the cause: %v", err)
	}
	e := r.only(t, CodeResolutionFailed)
	if e.Fields["step"] != "profile" || e.Fields["path"] != "/p/scout.json" || !strings.Contains(e.Reason, "permission denied") || !strings.HasPrefix(e.Reason, "profile not found: /p/scout.json: ") {
		t.Fatalf("the signal carries the cause: %+v", e)
	}
}

// Test 23 — the router's cli and tier win (the tier resolver is NOT called);
// a router error falls back to the profile with ONE LLM_RESOLVE_FALLBACK; a
// nil error with an empty cli falls back silently.
func TestResolve_LLMPrecedenceAndFallbackSignal(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	tierCalls := 0
	deps.ResolveTier = func(TierRequest) (string, error) { tierCalls++; return "haiku", nil }
	d, r := observed(t, deps)
	out, err := d.Dispatch(context.Background(), f.request())
	if err != nil || out.CLI != "claude" || out.Model != "sonnet" || tierCalls != 0 || len(r.events) != 0 {
		t.Fatalf("the router wins: %v %+v tier calls %d events %v", err, out, tierCalls, r.codes())
	}

	deps.ResolveLLM = func(string) (LLM, error) { return LLM{}, errors.New("no llm") }
	var env AdapterEnv
	deps.Adapter = AdapterFunc(func(ctx context.Context, e AdapterEnv) (int, error) { env = e; return soundAdapter(t).Exec(ctx, e) })
	d, r = observed(t, deps)
	out, err = d.Dispatch(context.Background(), f.request())
	if err != nil || out.CLI != "claude" || out.Model != "haiku" || env.CLIResolutionSource != "profile" {
		t.Fatalf("the profile fallback: %v %+v source %q", err, out, env.CLIResolutionSource)
	}
	e := r.only(t, CodeLLMResolveFallback)
	if e.Fields["step"] != "cli" || e.Fields["source"] != "profile" || e.Fields["cli"] != "claude" || e.Reason != "llm resolver failed for scout; cli taken from the profile: no llm" {
		t.Fatalf("the fallback signal: %+v", e)
	}

	deps.ResolveLLM = func(string) (LLM, error) { return LLM{}, nil }
	d, r = observed(t, deps)
	if out, err := d.Dispatch(context.Background(), f.request()); err != nil || out.CLI != "claude" || len(r.events) != 0 {
		t.Fatalf("a nil error with an empty cli is the designed fallback, silent: %v %+v %v", err, out, r.codes())
	}
}

// Test 24 — antigravity → agy on both the router and the profile path.
func TestResolve_CanonicalisesBothPaths(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.ResolveLLM = func(string) (LLM, error) { return LLM{CLI: "antigravity", ModelTier: "sonnet", Source: "profile"}, nil }
	d, _ := observed(t, deps)
	if out, err := d.Dispatch(context.Background(), f.request()); err != nil || out.CLI != "agy" {
		t.Fatalf("router path: %v %+v", err, out)
	}
	deps.ResolveLLM = func(string) (LLM, error) { return LLM{}, nil }
	deps.Profile = func(string) (Profile, error) {
		p := scoutProfile
		p.CLI = "antigravity"
		return p, nil
	}
	d, _ = observed(t, deps)
	if out, err := d.Dispatch(context.Background(), f.request()); err != nil || out.CLI != "agy" {
		t.Fatalf("profile path: %v %+v", err, out)
	}
}

// Test 25 — an unresolved cli and a missing driver keep their texts; the
// driver's vestigial .sh path rides the signal.
func TestResolve_UnresolvedCLIAndDriverMissingKeepTheirTexts(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.ResolveLLM = func(string) (LLM, error) { return LLM{}, nil }
	deps.Profile = func(string) (Profile, error) { return Profile{OutputArtifact: "x.md"}, nil }
	d, r := observed(t, deps)
	if _, err := d.Dispatch(context.Background(), f.request()); err == nil || err.Error() != "subagent/run: cli unresolved for agent scout" {
		t.Fatalf("unresolved: %v", err)
	}
	if e := r.only(t, CodeResolutionFailed); e.Fields["step"] != "cli" {
		t.Fatalf("step=cli: %+v", e)
	}

	deps = happyDeps(t)
	deps.AdapterExists = func(string) bool { return false }
	d, r = observed(t, deps)
	if _, err := d.Dispatch(context.Background(), f.request()); err == nil || err.Error() != "subagent/run: adapter not executable: /a/claude.sh" {
		t.Fatalf("driver: %v", err)
	}
	if e := r.only(t, CodeResolutionFailed); e.Fields["step"] != "driver" || e.Fields["path"] != "/a/claude.sh" || e.Fields["cli"] != "claude" {
		t.Fatalf("step=driver with the .sh path: %+v", e)
	}
}

// Test 26 — the tier request carries exactly the seven fields; an error is
// `resolve tier: <err>` with step=tier.
func TestResolve_TierRequestProjectionAndError(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	deps.ResolveLLM = func(string) (LLM, error) { return LLM{CLI: "claude", Source: "profile"}, nil }
	var got TierRequest
	deps.ResolveTier = func(tr TierRequest) (string, error) { got = tr; return "haiku", nil }
	d, _ := observed(t, deps)
	req := f.request()
	req.ModelTierHint, req.AuditorTierOverride, req.DiffComplexityDisabled = "opus", "sonnet", true
	if out, err := d.Dispatch(context.Background(), req); err != nil || out.Model != "haiku" {
		t.Fatalf("%v %+v", err, out)
	}
	if want := (TierRequest{ProfilePath: "/p/scout.json", Cycle: 5, ProjectRoot: f.root, WorktreePath: f.worktree, ModelTierHint: "opus", AuditorTierOverride: "sonnet", DiffComplexityDisabled: true}); got != want {
		t.Fatalf("tier request:\n got %+v\nwant %+v", got, want)
	}
	deps.ResolveTier = func(TierRequest) (string, error) { return "", errors.New("tier resolution failed") }
	d, r := observed(t, deps)
	if _, err := d.Dispatch(context.Background(), req); err == nil || err.Error() != "subagent/run: resolve tier: tier resolution failed" {
		t.Fatalf("%v", err)
	}
	if e := r.only(t, CodeResolutionFailed); e.Fields["step"] != "tier" || e.Reason != "resolve tier: tier resolution failed" {
		t.Fatalf("%+v", e)
	}
}

// Test 27 — CapabilityDir defaults to AdaptersDir; an inspect error is
// `capability inspect: <err>` with step=capability.
func TestResolve_CapabilityDirDefaultsAndInspectError(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	var dir string
	deps.Inspect = func(d, _ string) (Capability, error) { dir = d; return Capability{}, nil }
	d, _ := observed(t, deps)
	req := f.request()
	if _, err := d.Dispatch(context.Background(), req); err != nil || dir != "/a" {
		t.Fatalf("empty CapabilityDir ⇒ AdaptersDir: %v %q", err, dir)
	}
	req.CapabilityDir = "/c"
	if _, err := d.Dispatch(context.Background(), req); err != nil || dir != "/c" {
		t.Fatalf("CapabilityDir wins when set: %v %q", err, dir)
	}
	deps.Inspect = func(string, string) (Capability, error) { return Capability{}, errors.New("manifest parse error") }
	d, r := observed(t, deps)
	if _, err := d.Dispatch(context.Background(), req); err == nil || err.Error() != "subagent/run: capability inspect: manifest parse error" {
		t.Fatalf("%v", err)
	}
	if e := r.only(t, CodeResolutionFailed); e.Fields["step"] != "capability" || e.Fields["path"] != "/c" {
		t.Fatalf("%+v", e)
	}
}

// Test 28 — the run id is resolved ONCE after the gate (on a resolution
// failure and on a full run, ledger or not) and stamped on every later signal
// and on the ledger line; "" omits the key.
func TestResolve_RunIDResolvedOnceAfterTheGateAndStampedOnEverySignal(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	calls := 0
	deps.RunID = func(ws string) string {
		calls++
		if ws != f.ws {
			t.Fatalf("resolved from the run workspace, got %q", ws)
		}
		return "run-7"
	}
	deps.AdapterExists = func(string) bool { return false }
	d, r := observed(t, deps)
	if _, err := d.Dispatch(context.Background(), f.request()); err == nil || calls != 1 || r.only(t, CodeResolutionFailed).RunID != "run-7" {
		t.Fatalf("one read on a resolution failure, stamped: %v calls %d", err, calls)
	}

	deps = happyDeps(t)
	calls = 0
	deps.RunID = func(string) string { calls++; return "run-7" }
	deps.GitState = func(context.Context, string) (string, string, error) { return "", "", errors.New("no git") }
	deps.Adapter = AdapterFunc(func(context.Context, AdapterEnv) (int, error) { return 0, nil })
	d, r = observed(t, deps)
	req := f.request()
	req.WorktreePath = ""
	if _, err := d.Dispatch(context.Background(), req); err != nil || calls != 1 {
		t.Fatalf("one read on a full run without a ledger: %v calls %d", err, calls)
	}
	for _, e := range r.events {
		if e.RunID != "run-7" {
			t.Fatalf("every later signal carries the run id: %+v", e)
		}
	}
	if len(r.events) != 3 {
		t.Fatalf("git, worktree and integrity signals: %v", r.codes())
	}

	calls = 0
	req.LedgerPath = filepath.Join(f.root, "ledger.jsonl")
	if _, err := d.Dispatch(context.Background(), req); err != nil || calls != 1 {
		t.Fatalf("one read with a ledger: %v calls %d", err, calls)
	}
	line, _ := os.ReadFile(req.LedgerPath)
	if !strings.Contains(string(line), `"cycle":5,"run_id":"run-7","role":"scout"`) {
		t.Fatalf("the line carries the run id after cycle: %s", line)
	}
	deps.RunID = func(string) string { return "" }
	d, _ = observed(t, deps)
	req.LedgerPath = filepath.Join(f.root, "ledger2.jsonl")
	if _, err := d.Dispatch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	line, _ = os.ReadFile(req.LedgerPath)
	if strings.Contains(string(line), "run_id") {
		t.Fatalf("an empty run id omits the key: %s", line)
	}
}

// Test 64 (review fold, architecture M2) — the driver check receives the
// resolved cli, never the vestigial path: the host binds the port to driver
// presence without decoding a file name, so the compiler sees the round trip.
// The `.sh` path stays the error text's and the signal's `path` (test 25) —
// kills `pass the path to the port`.
func TestResolve_DriverCheckReceivesTheCLI(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	var got []string
	deps.AdapterExists = func(cli string) bool { got = append(got, cli); return true }
	d, _ := observed(t, deps)
	if _, err := d.Dispatch(context.Background(), f.request()); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "claude" {
		t.Fatalf("the port receives the cli once: %q", got)
	}
}
