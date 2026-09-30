//go:build acs

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

type fakeBridge struct{}

func (fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}
func (fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

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

func TestRegister_BareCLI_PersistsResolvedDriverName(t *testing.T) {
	r := newTestRegistrar(t)
	res, err := r.Register(mintCfg("claude"))
	if err != nil {
		t.Fatalf("Register(bare claude): %v", err)
	}
	want := bridge.DriverFor("claude")
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

	runGit(t, dir, "checkout", "-q", "-b", "cycle-100")
	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("base\ncycle100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-aq", "-m", "c100")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "-q", "cycle-100", "-m", "merge c100")

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

	runGit(t, dir, "checkout", "-q", "-b", "cycle-300", "main")
	if err := os.WriteFile(filepath.Join(dir, "cycle300.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "cycle300.txt")
	runGit(t, dir, "commit", "-aq", "-m", "c300 feature")
	runGit(t, dir, "checkout", "-q", "main")
	return dir
}

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

// acs-predicate: config-check — caller-existence is an inherent source-
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
