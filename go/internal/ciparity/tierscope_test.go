package ciparity

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

func TestTierScope_SplitsEnvExclusiveAndWholeModule(t *testing.T) {
	t.Parallel()
	if scoped, ex, whole := TierScope([]string{"./internal/bridge/..."}); len(scoped) != 0 || len(ex) != 1 || whole {
		t.Errorf("bridge-only: scoped=%v ex=%v whole=%v", scoped, ex, whole)
	}
	if scoped, ex, whole := TierScope([]string{"./internal/bridge/...", "./internal/prompts/...", "./acs/regression/...", "./internal/skilloverlay/..."}); strings.Join(scoped, " ") != "./internal/prompts/... ./internal/skilloverlay/..." || len(ex) != 1 || whole {
		t.Errorf("mixed: scoped=%v ex=%v whole=%v", scoped, ex, whole)
	}
	if _, _, whole := TierScope([]string{"./internal/bridge/...", "./..."}); !whole {
		t.Error("./... anywhere → the whole module")
	}
	if scoped, ex, _ := TierScope([]string{"./internal/bridge", "./internal/core"}); strings.Join(scoped, " ") != "./internal/core" || strings.Join(ex, " ") != "./internal/bridge" {
		t.Errorf("the floor's bare package form: scoped=%v ex=%v", scoped, ex)
	}
}

func TestTierLockPath_IsTheLockOfTheAuditRetake(t *testing.T) {
	t.Parallel()
	if got, want := TierLockPath("/proj"), filepath.Join("/proj", ".evolve", "locks", "integration-tier.lock"); got != want {
		t.Errorf("TierLockPath = %q, want %q", got, want)
	}
	if TierLockWait <= 0 {
		t.Errorf("TierLockWait = %v, want a positive bound", TierLockWait)
	}
}

func TestEnvExclusivePkg_BridgeIsExcluded(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"./internal/bridge/...", "internal/bridge", "github.com/mickeyyaya/evolve-loop/go/internal/bridge"} {
		if !EnvExclusivePkg(p) {
			t.Fatalf("%q must be env-exclusive: its requireTmux tests false-RED under a live wave (cycle-1539/1543/1546 class)", p)
		}
	}
}

func TestEnvExclusivePkg_OnlyBridge(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"internal/core", "cmd/evolve", "internal/phases/ship",
		"internal/acssuite", "internal/prompts", "internal/inboxmover", "internal/bridgeling", "xinternal/bridge", "x/internal/bridge/y"} {
		if EnvExclusivePkg(p) {
			t.Fatalf("%q must NOT be env-exclusive — the list is a scalpel (bridge only); contention is the serialized retake's job, and the last fossil here cost 2.5 days of red main (cycle-1594)", p)
		}
	}
}

func TestEnvExclusiveBackstopNote_IsHonestForBridge(t *testing.T) {
	t.Parallel()
	note := EnvExclusiveBackstopNote([]string{"./internal/bridge/...", "internal/bridge"})
	for _, falseBelief := range []string{"NOT covered by CI", "no tmux on runners", "loop-boot preflight"} {
		if strings.Contains(note, falseBelief) {
			t.Fatalf("the note must not state the false belief %q: the ubuntu and macOS CI jobs run the requireTmux tier; got %q", falseBelief, note)
		}
	}
	if !strings.Contains(note, EnvExclusiveCIMarker) || !strings.Contains(note, "ubuntu") || !strings.Contains(note, "macOS") {
		t.Fatalf("the note must say which CI jobs run bridge's requireTmux tier; got %q", note)
	}
	if !strings.Contains(note, "requireTmux tests boot real tmux sessions") {
		t.Fatalf("the note must carry the entry's evidence (why), not only the backstop; got %q", note)
	}
	if strings.Count(note, "requireTmux tests boot") != 1 {
		t.Fatalf("one record rendered once for two spellings of the same package; got %q", note)
	}
}

func TestEnvExclusive_EntriesNameTheCIJobsThatRunThem(t *testing.T) {
	t.Parallel()
	if len(tierEnvExclusive) == 0 {
		t.Fatal("the record table is empty — if the last exclusion was removed on purpose, retire this guard deliberately, not by vacuity")
	}
	for _, e := range tierEnvExclusive {
		if e.pkg == "" || e.why == "" || e.backstop == "" {
			t.Fatalf("entry %+v must carry pkg, evidence, and backstop — an unexplained exclusion is the fossil class", e)
		}
		if !strings.Contains(e.backstop, EnvExclusiveCIMarker) {
			t.Fatalf("%s: a lane that skips an env-exclusive package leaves CI as the only run, so the backstop must name the CI jobs that run it; backstop: %q", e.pkg, e.backstop)
		}
	}
}

func workflowJob(t *testing.T, workflow, job string) string {
	t.Helper()
	_, body, found := strings.Cut(workflow, "\n  "+job+":\n")
	if !found {
		t.Fatalf("go.yml has no job %q", job)
	}
	for _, line := range strings.SplitAfter(body, "\n") {
		if len(line) > 2 && line[:2] == "  " && line[2] != ' ' {
			return body[:strings.Index(body, line)]
		}
	}
	return body
}

func TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "go.yml"))
	if err != nil {
		t.Fatal(err)
	}
	darwinSet, err := os.ReadFile(filepath.Join("..", "..", ".darwin-set"))
	if err != nil {
		t.Fatal(err)
	}
	requires := testhost.RequiredToolsEnv + ": tmux"
	jobs := map[string][]string{
		"build-test":         {"runs-on: ubuntu-latest", "make test-integration\n", requires},
		"build-test-current": {"runs-on: ubuntu-latest", "make test-integration\n", requires},
		"build-test-darwin":  {"runs-on: macos-latest", "brew install tmux", "make test-integration-darwin", requires},
	}
	for job, wants := range jobs {
		block := workflowJob(t, string(raw), job)
		for _, want := range wants {
			if !strings.Contains(block, want) {
				t.Errorf("the bridge backstop names CI job %s as a run of the requireTmux tier, but that job lacks %q", job, want)
			}
		}
		for _, narrowing := range []string{"\n    if:", "INTEGRATION_SET"} {
			if strings.Contains("\n"+block, narrowing) {
				t.Errorf("CI job %s carries %q, which can stop it from running the requireTmux tier that the bridge backstop names", job, strings.TrimSpace(narrowing))
			}
		}
	}
	if !slices.Contains(strings.Fields(string(darwinSet)), "./internal/bridge") {
		t.Error("the bridge backstop names the macOS job, but internal/bridge is not in go/.darwin-set")
	}
}

func TestIntegrationTierScope_CoversCoreCmdShip_Cycle1594(t *testing.T) {
	t.Parallel()
	changed := []string{"./internal/core/...", "./cmd/evolve/...", "./internal/phases/ship/..."}
	scoped, _, _ := TierScope(changed)
	if strings.Join(scoped, " ") != strings.Join(changed, " ") {
		t.Fatalf("CI-backstopped packages must not be skipped as env-exclusive (cycle-1594 class): got %v", scoped)
	}
}
