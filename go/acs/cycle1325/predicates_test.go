//go:build acs

// Package cycle1325 materializes the cycle-1325 acceptance criteria for the
// TWO fleet-scoped tasks this lane owns that are NOT the boundary-refresh
// wiring gap (that one — auto-refresh-binary-at-boundary — is covered by a
// plain package-main test, cmd/evolve/cmd_loop_wave_boundaryrefresh_wiring_test.go,
// per R9.3: predicates bind only to this lane's top_n tasks, and that gap
// lives entirely inside package main, which an external acs package cannot
// import).
//
// Task B — wire-scout-carryforward-filter (scout-report.md Task B).
// Task C — mint-profile-driver-suffix (scout-report.md Task C).
//
// ARCHITECTURE CORRECTION (Task C, cycle-644 reachability-probe obligation):
// scout-report.md's targetFiles named go/internal/core/phase_advisor.go:1009
// as the fix site (`Dispatch: phaseconfig.Dispatch{CLI: e.Mint.CLI, ...}`).
// Compiler-probed here (temp import of internal/bridge from a
// zzz_import_probe_test.go dropped into internal/core, `go vet
// ./internal/core/...`): internal/bridge already imports internal/core
// (driver_claudep.go, engine.go, and ~a dozen *_test.go files) — so
// internal/core importing internal/bridge is a PROVEN import cycle
// ("import cycle not allowed in test"), not merely a plausible one. Freezing
// a predicate that pins bridge.DriverFor inside phase_advisor.go would
// reproduce cycle-644 exactly (an unsatisfiable AC baked into a frozen
// test). The fix instead lands at go/internal/phaseregistrar/registrar.go's
// Register — the actual mint choke point every minted phase (including
// advisor mints) already funnels through for its dispatch.cli validation
// (the existing "phase has no dispatch.cli; refusing to mint a driverless
// profile stub" guard, right next to where this task's guard belongs).
// phaseregistrar already imports internal/core (for core.Bridge/
// core.PhaseRunner) and compiler-probed clean for an ADDITIONAL import of
// internal/bridge (no cycle: bridge does not import phaseregistrar).
//
// Predicate strategy (cycle-85 degenerate-predicate ban: every predicate
// below exercises the real system under test, never a source-grep):
//
//	C1325_001 (Task C, positive)  — Register on a bare-CLI ("claude") config
//	  persists a profile whose cli field is the RESOLVED driver name
//	  ("claude-tmux"), proving the mint-time projection actually ran (not
//	  merely that dispatch tolerates the bare name elsewhere).
//	C1325_002 (Task C, negative)  — Register on an UNRESOLVABLE CLI name
//	  (neither a registered driver nor a known bare alias) is REJECTED
//	  loudly (non-nil error, nothing persisted) — never silently passed
//	  through to disk to fail hours later at the next loop launch's
//	  preflight halt (the batch-15 incident this task exists to close).
//	C1325_003 (Task C, edge/no-regression) — an ALREADY-resolved driver name
//	  ("claude-tmux") persists unchanged; resolution is idempotent, not a
//	  second, incompatible clamp on top of the existing base-name
//	  AllowedCLIs clamp (TestRegister_CLIWithDriverSuffix_ClampedByBase,
//	  registrar_test.go, pre-existing GREEN, left untouched by this task).
//	C1325_004 (Task B, positive+negative, the cycle-826 repro) — a real git
//	  fixture with three orphan cycle-* branches (superseded/ancestor,
//	  genuine 3-way conflict, clean/landable) proves
//	  triage.CarryforwardCandidatesSection surfaces ONLY the landable one —
//	  reproducing the exact cd409fed(ancestor)/b270ee11(orphan-but-here-
//	  conflicting) mis-selection the inbox item roots.
//	C1325_005 (Task B, edge) — no local cycle-* branches at all renders "".
//	C1325_006 (Task B, WIRING PROOF) — triage.ComposePrompt's real
//	  production body actually calls CarryforwardCandidatesSection (AST
//	  caller-proof, cycle-968 precedent) so the filter is reachable from the
//	  real triage prompt, not a second inert oracle sitting beside it.
//
// Adversarial diversity: negative (002, unresolvable-CLI rejection; 004's
// conflict+superseded exclusions), edge (003 idempotence; 005 empty-repo),
// semantic (004 distinguishes THREE distinct verdicts, not one restated).
package cycle1325

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseregistrar"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// -----------------------------------------------------------------------
// Task C — mint-profile-driver-suffix
// -----------------------------------------------------------------------

// fakeBridge is the minimal core.Bridge stub Register needs (it never
// dispatches during Register itself — only specrunner.New wraps it).
type fakeBridge struct{}

func (fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}
func (fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// mintCfg mirrors registrar_test.go's validCfg() shape (a minimal in-envelope
// minted phase), parameterized on the dispatch CLI this predicate varies.
func mintCfg(cli string) phaseconfig.PhaseConfig {
	return phaseconfig.PhaseConfig{
		PhaseSpec: phasespec.PhaseSpec{Name: "minted-driver-probe", Optional: true},
		Dispatch: phaseconfig.Dispatch{
			CLI:              cli,
			ModelTierDefault: "balanced",
		},
		Prompt: "You are a probe phase.",
	}
}

func newTestRegistrar(t *testing.T) phaseregistrar.Registrar {
	t.Helper()
	return phaseregistrar.Registrar{
		Bridge:      fakeBridge{},
		Prompts:     prompts.NewFromFS(fstest.MapFS{}),
		ProfilesDir: filepath.Join(t.TempDir(), "profiles"),
		PhasesDir:   filepath.Join(t.TempDir(), "phases"),
	}
}

// persistedProfileCLI reads back the profile Register persisted and returns
// its cli field — reading the REAL emitted artifact, not the in-memory
// Result, so a bug that resolves in Result but forgets to persist the
// resolved value cannot fake a pass.
func persistedProfileCLI(t *testing.T, profilesDir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(profilesDir, name+".json"))
	if err != nil {
		t.Fatalf("read persisted profile: %v", err)
	}
	var prof profiles.Profile
	if err := json.Unmarshal(raw, &prof); err != nil {
		t.Fatalf("unmarshal persisted profile: %v", err)
	}
	return prof.CLI
}

// C1325_001: a bare family name ("claude") persists as its resolved driver
// name ("claude-tmux") — the exact projection bridge.DriverFor already
// performs for every OTHER dispatch call site (subagent, consensusdispatch),
// now applied at the mint choke point too.
func TestRegister_BareCLI_PersistsResolvedDriverName(t *testing.T) {
	r := newTestRegistrar(t)
	res, err := r.Register(mintCfg("claude"))
	if err != nil {
		t.Fatalf("Register(bare claude): %v", err)
	}
	want := bridge.DriverFor("claude") // claude-tmux, per bareDriverMap — asserted
	// against the real resolver so a future remap doesn't hardcode past it.
	if want != "claude-tmux" {
		t.Fatalf("test assumption broken: bridge.DriverFor(\"claude\")=%q, want claude-tmux", want)
	}
	if res.Spec.Name == "" {
		t.Fatal("Register returned an empty spec")
	}
	got := persistedProfileCLI(t, r.ProfilesDir, "minted-driver-probe")
	if got != want {
		t.Errorf("persisted profile cli=%q, want %q (bridge.DriverFor projection) — a bare family name reached disk unresolved", got, want)
	}
}

// C1325_002 (negative): a CLI name that is neither a registered driver nor a
// known bare alias (bareDriverMap) must be REJECTED at mint time — never
// silently persisted to fail hours later at the next loop launch's
// preflight halt (the batch-15 incident).
func TestRegister_UnresolvableCLI_RejectedNotPersisted(t *testing.T) {
	r := newTestRegistrar(t)
	const bogus = "not-a-real-cli-family"
	if _, ok := bridge.LookupDriver(bridge.DriverFor(bogus)); ok {
		t.Fatalf("test assumption broken: %q unexpectedly resolves to a registered driver", bogus)
	}

	_, err := r.Register(mintCfg(bogus))
	if err == nil {
		t.Fatal("Register(unresolvable CLI) returned nil error — an unresolvable family name must fail mint loudly, not silently persist")
	}
	if _, statErr := os.Stat(filepath.Join(r.ProfilesDir, "minted-driver-probe.json")); statErr == nil {
		t.Error("an unresolvable-CLI mint persisted a profile despite the rejection")
	}
	if _, statErr := os.Stat(filepath.Join(r.PhasesDir, "minted-driver-probe", "phase.json")); statErr == nil {
		t.Error("an unresolvable-CLI mint persisted a phase spec despite the rejection")
	}
}

// C1325_003 (edge, no-regression): an ALREADY-resolved driver name passes
// through unchanged — resolution must be idempotent, and must not disturb
// the pre-existing base-name AllowedCLIs clamp
// (TestRegister_CLIWithDriverSuffix_ClampedByBase in registrar_test.go).
func TestRegister_AlreadyResolvedCLI_PersistsUnchanged(t *testing.T) {
	r := newTestRegistrar(t)
	if _, err := r.Register(mintCfg("claude-tmux")); err != nil {
		t.Fatalf("Register(claude-tmux): %v", err)
	}
	got := persistedProfileCLI(t, r.ProfilesDir, "minted-driver-probe")
	if got != "claude-tmux" {
		t.Errorf("persisted profile cli=%q, want claude-tmux unchanged (resolution must be idempotent)", got)
	}
}

// -----------------------------------------------------------------------
// Task B — wire-scout-carryforward-filter
// -----------------------------------------------------------------------

// runGit runs a git subcommand in dir, failing the test loudly on error —
// fixture setup is not the behavior under test, so any setup failure must
// abort immediately rather than produce a misleading downstream assertion.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// carryforwardFixture builds a real repo reproducing the cycle-826 incident
// exactly: an ancestor-superseded orphan (cycle-100, mirrors cd409fed —
// already landed on main), a genuinely conflicting orphan (cycle-200,
// mirrors the auditor's real cherry-pick-dry-run conflicts a bare
// merge-tree-clean read would have missed), and a clean, not-yet-landed,
// landable orphan (cycle-300) that must be the ONLY one surfaced.
func carryforwardFixture(t *testing.T) (dir string) {
	t.Helper()
	dir = t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "acs@example.invalid")
	runGit(t, dir, "config", "user.name", "acs")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "shared.txt")
	runGit(t, dir, "commit", "-q", "-m", "base")

	// cycle-100: superseded — its change is merged straight into main, so it
	// becomes a pure ancestor (git merge-base --is-ancestor succeeds).
	runGit(t, dir, "checkout", "-q", "-b", "cycle-100")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("base\ncycle100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-aq", "-m", "c100")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "-q", "cycle-100", "-m", "merge c100")

	// cycle-200: forked BEFORE the c100 merge, diverges on the same line main
	// keeps editing — a genuine, unresolvable 3-way conflict.
	runGit(t, dir, "tag", "orig-base", "HEAD~1")
	runGit(t, dir, "checkout", "-q", "-b", "cycle-200", "orig-base")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("base\nCLASH-200\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-aq", "-m", "c200 clash")
	runGit(t, dir, "checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("base\ncycle100\nCLASH-main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-aq", "-m", "main clash edit")

	// cycle-300: clean, disjoint, not-yet-landed — the one true candidate.
	runGit(t, dir, "checkout", "-q", "-b", "cycle-300", "main")
	if err := os.WriteFile(filepath.Join(dir, "cycle300.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "cycle300.txt")
	runGit(t, dir, "commit", "-aq", "-m", "c300 feature")
	runGit(t, dir, "checkout", "-q", "main")
	return dir
}

// C1325_004: the cycle-826 repro — only cycle-300 (landable) is surfaced;
// cycle-100 (superseded/ancestor) and cycle-200 (real conflict) are BOTH
// excluded. A bare `git merge-tree`-clean-only read would have wrongly kept
// cycle-100 (merge-tree of an already-merged branch reads clean) — this
// predicate fails against that degenerate implementation and only passes
// against the real CarryforwardCandidateLandable-backed filter.
func TestCarryforwardCandidatesSection_ExcludesSupersededAndConflicting(t *testing.T) {
	dir := carryforwardFixture(t)

	section := triage.CarryforwardCandidatesSection(context.Background(), dir, "main")

	if !strings.Contains(section, "cycle-300") {
		t.Errorf("landable candidate cycle-300 missing from section:\n%s", section)
	}
	if strings.Contains(section, "cycle-100") {
		t.Errorf("superseded (already-landed ancestor) candidate cycle-100 must be EXCLUDED, found in section:\n%s", section)
	}
	if strings.Contains(section, "cycle-200") {
		t.Errorf("conflicting candidate cycle-200 must be EXCLUDED, found in section:\n%s", section)
	}
}

// C1325_005 (edge): no local cycle-* branches at all renders "" — the
// byte-identity pin every other triage prompt section (inboxBatchesSection)
// already honors, so an empty backlog never perturbs the cached prompt
// prefix.
func TestCarryforwardCandidatesSection_NoOrphansIsEmpty(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "acs@example.invalid")
	runGit(t, dir, "config", "user.name", "acs")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-q", "-m", "only commit")

	section := triage.CarryforwardCandidatesSection(context.Background(), dir, "main")
	if section != "" {
		t.Errorf("no orphan branches exist, want \"\", got %q", section)
	}
}

// C1325_006 (WIRING PROOF — kills a second inert oracle) — triage.go's real
// ComposePrompt body must call CarryforwardCandidatesSection, so the
// deterministic filter is actually consulted during the phase's own
// candidate selection, not merely built and left beside it (the exact
// "second, uninvoked oracle" gap the inbox item's [VERIFY 2026-07-23] note
// names). Structural (AST) caller-proof, paired with the behavioral
// predicates above — cycle-968 TestClassifyFleetRebaseCandidate_WiredIntoRecoverFromShipError
// precedent.
// acs-predicate: config-check — caller-existence is an inherent source-
// structure check; CarryforwardCandidatesSection's own filtering behavior is
// proven by C1325_004/005 above.
func TestCarryforwardCandidatesSection_WiredIntoComposePrompt(t *testing.T) {
	// acs-predicate: config-check
	src := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "phases", "triage", "triage.go")
	n, err := acsassert.CountInGoFunc(src, "ComposePrompt", "CarryforwardCandidatesSection")
	if err != nil {
		t.Fatalf("CountInGoFunc(ComposePrompt, CarryforwardCandidatesSection): %v", err)
	}
	if n < 1 {
		t.Errorf("triage.go's ComposePrompt does not call CarryforwardCandidatesSection (count=%d); the deterministic filter would remain a second, uninvoked oracle", n)
	}
}
