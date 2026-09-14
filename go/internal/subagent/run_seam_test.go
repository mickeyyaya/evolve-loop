package subagent

// run_seam_test.go — ADR-0103 unit 16 step 3: the seam between the host's
// RunOptions bag and the subagentrun dispatcher — one construction, one
// projection each way, the Center forwarded, every facade projecting the leaf.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

// Test 52 — the dispatcher is constructed in exactly ONE non-test file.
func TestSubagentRun_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/subagent/run.go"
	if offenders := nonTestSourcesMentioning(t, "subagentrun.New(", onlySite); len(offenders) > 0 {
		t.Errorf("subagentrun.New( belongs to ONE non-test file (%s); these non-test files use it too: %v", onlySite, offenders)
	}
}

// nonTestSourcesMentioning lists the module's non-test Go files outside the
// subagentrun leaf and the one allowed site whose source contains needle
// (core/carryover_lifecycle_test.go idiom).
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
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/subagent/subagentrun/") {
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

// Test 53 — one Run drives each of the twelve live seams exactly once
// (WriteFile is dead); the request projection carries every field, PluginRoot
// excluded; the result projection carries the nine fields with Stderr "".
func TestRun_FacadeProjectsEverySeamByName(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	calls := map[string]int{}
	hit := func(name string) { calls[name]++ }
	clock := fixedClock(t, "2026-05-23T17:00:00Z")
	opts := RunOptions{
		ReadProfile: func(string) (string, error) {
			hit("ReadProfile")
			return `{"role":"scout","cli":"claude","output_artifact":".evolve/runs/cycle-{cycle}/scout.md"}`, nil
		},
		ResolveLLM: func(string) (resolvellm.Result, error) {
			hit("ResolveLLM")
			return resolvellm.Result{CLI: "claude", Source: "profile"}, nil
		},
		InspectCapability: func(string, string) (capability.Inspection, error) {
			hit("InspectCapability")
			return capability.Inspection{Manifest: capability.Manifest{BudgetNative: true}}, nil
		},
		ResolveModelTier: func(ResolveModelTierRequest, ResolveModelTierOptions) (string, error) {
			hit("ResolveModelTier")
			return "haiku", nil
		},
		AdapterExists: func(string) bool { hit("AdapterExists"); return true },
		ExecAdapter: func(_ context.Context, adapter string, env map[string]string) (int, error) {
			hit("ExecAdapter")
			if adapter != "/a/claude.sh" || env["VALIDATE_ONLY"] != "0" {
				t.Errorf("the func seam receives the vestigial path and the rendered env: %q %v", adapter, env)
			}
			unit16Artifact(t, env["ARTIFACT_PATH"], env["CHALLENGE_TOKEN"], clock())
			return 0, nil
		},
		WriteFile: func(string, []byte, os.FileMode) error { hit("WriteFile"); return nil },
		GitState:  func(context.Context, string) (string, string, error) { hit("GitState"); return "h", "t", nil },
		StatMTime: func(p string) (time.Time, error) { hit("StatMTime"); return defaultStatMTime(p) },
		ReadFile:  func(p string) ([]byte, error) { hit("ReadFile"); return os.ReadFile(p) },
		HashFile:  func(p string) (string, error) { hit("HashFile"); return defaultHashFile(p) },
		Now:       func() time.Time { hit("Now"); return clock() },
		Rand: func(b []byte) (int, error) {
			hit("Rand")
			for i := range b {
				b[i] = 0xaa
			}
			return len(b), nil
		},
	}
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProfilesDir: "/p", AdaptersDir: "/a", ProjectRoot: root, WorktreePath: worktree, LedgerPath: filepath.Join(root, "ledger.jsonl"), PromptReader: strings.NewReader("hi\n")}, opts)
	if err != nil || res.Verdict != VerdictPASS || res.Model != "haiku" {
		t.Fatalf("%v %+v", err, res)
	}
	want := map[string]int{"ReadProfile": 1, "ResolveLLM": 1, "InspectCapability": 1, "ResolveModelTier": 1, "AdapterExists": 1, "ExecAdapter": 1, "GitState": 1, "StatMTime": 1, "ReadFile": 1, "HashFile": 1, "Now": 4, "Rand": 1}
	for k, v := range want {
		if calls[k] != v {
			t.Errorf("%s driven %d times, want %d", k, calls[k], v)
		}
	}
	if calls["WriteFile"] != 0 {
		t.Errorf("WriteFile is dead: %d calls", calls["WriteFile"])
	}

	req := RunRequest{"a", 1, "ws", "p", "a", "c", "root", "plugin", "wt", "l", strings.NewReader("x"), "hint", "ovr", true, true, false, 2, "tok"}
	got := requestOf(req)
	want2 := subagentrun.Request{Agent: "a", Cycle: 1, WorkspacePath: "ws", ProfilesDir: "p", AdaptersDir: "a", CapabilityDir: "c", ProjectRoot: "root", WorktreePath: "wt", LedgerPath: "l",
		Prompt: req.PromptReader, ModelTierHint: "hint", AuditorTierOverride: "ovr", DiffComplexityDisabled: true, AdversarialAudit: true, LegacyAgentDispatch: false, DispatchDepth: 2, ChallengeTokenOverride: "tok"}
	if got != want2 {
		t.Fatalf("requestOf:\n got %+v\nwant %+v", got, want2)
	}
	out := subagentrun.Outcome{Verdict: "FAIL", CLI: "c", Model: "m", ArtifactPath: "/a", ArtifactSHA256: "s", ChallengeToken: "t", ExitCode: 3, DurationMS: 9, Warns: []string{"w"}, Integrity: subagentrun.IntegrityStale}
	r := resultOf(out)
	if r.Verdict != "FAIL" || r.CLI != "c" || r.Model != "m" || r.ArtifactPath != "/a" || r.ArtifactSHA256 != "s" || r.ChallengeToken != "t" || r.ExitCode != 3 || r.DurationMS != 9 || len(r.Warns) != 1 || r.Warns[0] != "w" || r.Stderr != "" {
		t.Fatalf("resultOf: %+v", r)
	}
}

// Test 54 — Run over runHappyOpts is field for field the RunResult captured
// on the pre-extraction code.
func TestRun_OverRunHappyOptsIsFieldForFieldTheSameAsBefore(t *testing.T) {
	root, ws, _ := unit16Dirs(t)
	res, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 5, WorkspacePath: ws, ProfilesDir: "/p", AdaptersDir: "/a", ProjectRoot: root, PluginRoot: root, PromptReader: strings.NewReader("Do the thing.\n"), AdversarialAudit: true}, runHappyOpts(t))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.MarshalIndent(res, "", "  ")
	want, err := os.ReadFile(filepath.Join("testdata", "run-result.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(got), root, "{ROOT}")+"\n" != string(want) {
		t.Fatalf("RunResult drifted:\n got %s\nwant %s", got, want)
	}
}

// Test 55 — the exec seam carries the Center: execAdapterDepsWith sets it on
// the one-arg execAdapterDeps (whose spelling the tokenresolver wiring pins),
// nil is today's Deps, adapterOf picks the bridge adapter for a nil
// ExecAdapter and the func seam otherwise, and execAdapter builds its engine
// from execAdapterDepsWith (the wiring proof).
func TestExecAdapterDepsWith_CarriesTheCenter(t *testing.T) {
	c := signalcenter.New()
	env := map[string]string{"HOME": t.TempDir()}
	with := execAdapterDepsWith(env, c)
	without := execAdapterDepsWith(env, nil)
	plain := execAdapterDeps(env)
	if with.Signals != c || without.Signals != nil || plain.Signals != nil || with.TokenResolver == nil || without.TokenResolver == nil || with.Env["HOME"] != env["HOME"] {
		t.Fatalf("with %+v without %+v", with.Signals, without.Signals)
	}
	if a, ok := adapterOf(RunOptions{Signals: c}).(bridgeAdapter); !ok || a.signals != c {
		t.Fatalf("a nil ExecAdapter is the bridge adapter carrying the Center: %T", adapterOf(RunOptions{Signals: c}))
	}
	var gotPath string
	var gotEnv map[string]string
	a := adapterOf(RunOptions{ExecAdapter: func(_ context.Context, p string, e map[string]string) (int, error) {
		gotPath, gotEnv = p, e
		return 7, nil
	}})
	if _, ok := a.(subagentrun.AdapterFunc); !ok {
		t.Fatalf("a supplied ExecAdapter is the func seam: %T", a)
	}
	if code, err := a.Exec(context.Background(), subagentrun.AdapterEnv{AdapterPath: "/a/x.sh", ResolvedCLI: "x", ProjectRoot: "/r"}); err != nil || code != 7 || gotPath != "/a/x.sh" || gotEnv["RESOLVED_CLI"] != "x" || gotEnv["EVOLVE_PROJECT_ROOT"] != "/r" || len(gotEnv) != 17 {
		t.Fatalf("the func seam receives AdapterPath and Map(): %d %v %q %v", code, err, gotPath, gotEnv)
	}
	src, err := os.ReadFile("bridgeadapter.go")
	if err != nil {
		t.Fatal(err)
	}
	body := regexp.MustCompile(`(?s)func execAdapter\(ctx context\.Context, env map\[string\]string, signals \*signalcenter\.Center\) \(int, error\) \{.*?\n\}`).FindString(string(src))
	if !strings.Contains(body, "gobridge.NewEngine(execAdapterDepsWith(env, signals))") {
		t.Fatalf("execAdapter must build its engine from execAdapterDepsWith(env, signals):\n%s", body)
	}
	if code, err := execAdapter(context.Background(), map[string]string{"PROMPT_FILE": filepath.Join(t.TempDir(), "missing")}, c); code != -1 || err == nil {
		t.Fatalf("an unreadable prompt file is the -1 infra error: %d %v", code, err)
	}
}

// Test 56 — every host facade projects the leaf on one input (the consumer
// pins; keeps ./internal/subagent's apicover row green).
func TestHostFacades_ProjectTheLeaf(t *testing.T) {
	if VerdictPASS != subagentrun.VerdictPASS || VerdictFAIL != subagentrun.VerdictFAIL || VerdictIntegrityFail != subagentrun.VerdictIntegrityFail ||
		ArtifactMaxAge != subagentrun.ArtifactMaxAge || ChallengeTokenBytes != subagentrun.ChallengeTokenBytes || ErrInProcessDispatchBanned != subagentrun.ErrInProcessDispatchBanned || ledgerZeroSeed != subagentrun.LedgerZeroSeed {
		t.Fatal("the vocabulary projections")
	}
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	var in VerifyInput = VerifyInput{StatErr: errors.New("missing"), Now: now, MaxAge: ArtifactMaxAge, Token: "tok", ArtifactPath: "/nope"}
	var res VerifyResult = Verify(in)
	if leaf := subagentrun.Verify(in); res.Verdict != VerdictIntegrityFail || res.Reason != subagentrun.IntegrityMissing || res.Verdict != leaf.Verdict || len(res.Diagnostics) != len(leaf.Diagnostics) {
		t.Fatalf("Verify projects: %+v", res)
	}
	stat := func(string) (time.Time, error) { return time.Time{}, errors.New("missing") }
	if got := VerifyArtifact(stat, os.ReadFile, time.Now, "/nope", "tok", 0, nil); got.Verdict != VerdictIntegrityFail {
		t.Fatalf("VerifyArtifact projects: %+v", got)
	}
	if capabilityTier(capability.Manifest{BudgetNative: true, PermissionScoping: false}) != subagentrun.QualityTier(true, false) || capabilityTier(capability.Manifest{}) != "degraded" {
		t.Fatal("capabilityTier projects")
	}
	if tok, err := generateRunToken(nil); err != nil || len(tok) != 16 {
		t.Fatalf("generateRunToken(nil) mints through crypto/rand: %q %v", tok, err)
	}
	if tok, err := generateRunToken(func(b []byte) (int, error) { return 1, nil }); err == nil || err.Error() != "rand returned 1 bytes, want 8" || tok != "" {
		t.Fatalf("generateRunToken projects the short read: %q %v", tok, err)
	}
	if capBoolEnv(true) != subagentrun.BoolEnv(true) || capBoolEnv(false) != "false" {
		t.Fatal("capBoolEnv projects")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(p, []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev, seq, err := readChainLink(p)
	if err != nil || prev != subagentrun.SHA256Hex("second") || seq != 2 {
		t.Fatalf("readChainLink projects: %q %d %v", prev, seq, err)
	}
	if sha256Hex("x") != subagentrun.SHA256Hex("x") || jsonStringEscape(`a"b`) != subagentrun.JSONStringEscape(`a"b`) {
		t.Fatal("sha256Hex / jsonStringEscape project")
	}
	if resolveArtifactPath("x/{cycle}.md", 3, "/r") != subagentrun.ResolveArtifactPath("x/{cycle}.md", 3, "/r") || resolveArtifactPath("", 3, "/r") != "" {
		t.Fatal("resolveArtifactPath projects")
	}
	f := filepath.Join(dir, "a.md")
	if err := os.WriteFile(f, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	mt, err := defaultStatMTime(f)
	want, _ := subagentrun.StatMTime(f)
	if err != nil || !mt.Equal(want) {
		t.Fatalf("defaultStatMTime projects: %v %v", mt, err)
	}
	if sha, err := defaultHashFile(f); err != nil || sha != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("defaultHashFile projects: %q %v", sha, err)
	}
}

// Test 57 — RunOptions.Signals reaches the leaf; a nil one is safe.
func TestRun_SignalsFieldReachesTheLeafAndNilIsSafe(t *testing.T) {
	_, ws, worktree := unit16Dirs(t)
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	opts := runHappyOpts(t)
	opts.AdapterExists = func(string) bool { return false }
	opts.Signals = c
	req := RunRequest{Agent: "scout", Cycle: 7, WorkspacePath: ws, AdaptersDir: "/a", WorktreePath: worktree, PromptReader: strings.NewReader("hi\n")}
	if _, err := Run(context.Background(), req, opts); err == nil {
		t.Fatal("expected the driver rejection")
	}
	if len(got) != 1 || got[0].Code != subagentrun.CodeResolutionFailed || got[0].Cycle != 7 || got[0].Phase != "scout" || got[0].Fields["step"] != "driver" {
		t.Fatalf("the Center receives the leaf's signal: %+v", got)
	}
	opts.Signals = nil
	if _, err := Run(context.Background(), req, opts); err == nil {
		t.Fatal("nil Center: the same rejection, no panic")
	}
}

// Test 65 (review fold, architecture M2) — RunOptions.AdapterExists receives
// the cli (its func type is unchanged, so every by-name binder compiles and,
// ignoring its argument, behaves as before) and its production default is
// driver presence BY CLI: a path is not a cli, nothing decodes a file name on
// the run path — kills `bind the path-decoding validate default to the run
// path`, `compose the .sh path for the seam`.
func TestRun_AdapterExistsSeamReceivesTheCLIAndDefaultsToDriverPresence(t *testing.T) {
	root, ws, worktree := unit16Dirs(t)
	opts := runHappyOpts(t)
	var got []string
	opts.AdapterExists = func(cli string) bool { got = append(got, cli); return true }
	if _, err := Run(context.Background(), RunRequest{Agent: "scout", Cycle: 7, WorkspacePath: ws, ProfilesDir: "/p", AdaptersDir: "/a", ProjectRoot: root, WorktreePath: worktree, PromptReader: strings.NewReader("hi\n")}, opts); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "claude" {
		t.Fatalf("the seam receives the cli once: %q", got)
	}
	var def RunOptions
	fillRunDefaults(&def)
	for cli, want := range map[string]bool{"claude": true, "codex": true, "claude-tmux": true, "nope": false, "/a/claude.sh": false} {
		if got := def.AdapterExists(cli); got != want {
			t.Errorf("the default AdapterExists(%q)=%v, want %v", cli, got, want)
		}
	}
}

// Test 66 (review fold, architecture M1) — the adapter-env contract has ONE
// typed home, subagentrun.AdapterEnv.Map(); ValidateProfile's literal map is
// its named twin until follow-up 16-9 folds it onto an AdapterEnv with a
// ValidateOnly axis. Until then this pin keeps the two key sets from
// drifting: the validate-only env is exactly Map()'s keys minus the run-only
// CHALLENGE_TOKEN (and never EVOLVE_PROJECT_ROOT — the twin's two gaps 16-9
// decides), VALIDATE_ONLY the one axis ("1" vs "0"). A key added to either
// side goes red here — kills `a key added to Map()`, `VALIDATE_ONLY dropped
// from the twin`.
func TestValidateProfile_EnvIsTheLeafsWireMinusTheRunOnlyKeys(t *testing.T) {
	opts := happyOpts(`{"cli":"claude","model_tier_default":"sonnet","output_artifact":"x.md"}`, "claude")
	var got map[string]string
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) { got = env; return 0, nil }
	if _, err := ValidateProfile(context.Background(), ValidateProfileRequest{Agent: "x", ProfilesDir: "/p", AdaptersDir: "/a", ProjectRoot: "/r"}, opts); err != nil {
		t.Fatal(err)
	}
	want := subagentrun.AdapterEnv{}.Map()
	delete(want, "CHALLENGE_TOKEN")
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("the validate env lacks the wire's key %s", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("the validate env carries %s, which the wire does not", k)
		}
	}
	if got["VALIDATE_ONLY"] != "1" || want["VALIDATE_ONLY"] != "0" {
		t.Errorf("VALIDATE_ONLY is the one axis: validate %q, run %q", got["VALIDATE_ONLY"], want["VALIDATE_ONLY"])
	}
	if _, ok := got["EVOLVE_PROJECT_ROOT"]; ok {
		t.Error("the validate twin exports no EVOLVE_PROJECT_ROOT even with a project root (its second gap, 16-9)")
	}
}
