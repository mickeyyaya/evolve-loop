package ciparitygate

// tierscope_test.go — the integration tier's scope (tierscope.go): the pure
// split, the env-exclusive record table and the whole-suite fallback.

import (
	"context"
	"io"
	"strings"
	"testing"
)

// Test 17 (moved: audit/ciparity_unit_test.go:133-157 + the mixed branch) —
// tierScope is PURE: bridge-only → the golden env-exclusive error and ONE
// TIER_ENV_EXCLUSIVE_SKIPPED{scope=all}; mixed → only the runnable remainder,
// in order, ONE {scope=mixed} and ZERO bytes on stderr; /acs/ dropped; ./... →
// the whole module.
func TestTierScope_PureSplitAndEnvExclusiveEventsWithoutStderr(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	if scoped, ex, whole := tierScope([]string{"./internal/bridge/..."}); len(scoped) != 0 || len(ex) != 1 || whole {
		t.Errorf("bridge-only: scoped=%v ex=%v whole=%v", scoped, ex, whole)
	}
	if scoped, ex, whole := tierScope([]string{"./internal/bridge/...", "./internal/prompts/...", "./acs/regression/...", "./internal/skilloverlay/..."}); strings.Join(scoped, " ") != "./internal/prompts/... ./internal/skilloverlay/..." || len(ex) != 1 || whole {
		t.Errorf("mixed: scoped=%v ex=%v whole=%v", scoped, ex, whole)
	}
	if _, _, whole := tierScope([]string{"./internal/bridge/...", "./..."}); !whole {
		t.Error("./... anywhere → the whole module")
	}

	// An acs-only scope skips the tier silently: nothing runs, nothing is emitted.
	acsOnly, acsEvents := observed(t, fakeRunFunc(1, "", "must not run", nil), fixedSet("./acs/regression/..."))
	if off, err := acsOnly.IntegrationTier(tierRequest(func() string { r, _ := goWorktree(t); return r }(), "")); off != nil || err != nil || len(*acsEvents) != 0 {
		t.Errorf("acs-only scope: (%v, %v) events=%v, want a silent skip", off, err, codesOf(*acsEvents))
	}

	root, _ := goWorktree(t)
	req := tierRequest(root, "")
	g, events := observed(t, fakeRunFunc(0, "", "", nil), fixedSet("./internal/bridge/..."))
	off, err := g.IntegrationTier(req)
	if off != nil || err == nil || err.Error() != g1["tier.env_exclusive.all"] {
		t.Fatalf("bridge-only scope: (%v, %v), want (nil, the golden env-exclusive WARN)", off, err)
	}
	e := only(t, *events, CodeTierEnvExclusiveSkipped)
	if e.Fields["scope"] != "all" || e.Fields["pkgs"] != "./internal/bridge/..." || e.Fields["remainder"] != "0" || !strings.Contains(e.Fields["backstop"], envExclusiveNoCIMarker) || e.Reason != err.Error() {
		t.Errorf("scope=all event: %+v", e)
	}

	var seen []string
	g, events = observed(t, func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		seen = append(seen, strings.Join(args, " "))
		return 0, nil
	}, fixedSet("./internal/bridge/...", "./internal/prompts/..."))
	out := captureStderr(t, func() {
		if off, err := g.IntegrationTier(req); off != nil || err != nil {
			t.Errorf("mixed scope must run the remainder: (%v, %v)", off, err)
		}
	})
	if out != "" {
		t.Errorf("the mixed skip is an event, never stderr: %q", out)
	}
	if len(seen) != 1 || !strings.HasSuffix(seen[0], " ./internal/prompts/...") || strings.Contains(seen[0], "bridge") {
		t.Errorf("only the remainder ran: %v", seen)
	}
	e = only(t, *events, CodeTierEnvExclusiveSkipped)
	if e.Fields["scope"] != "mixed" || e.Fields["remainder"] != "1" || e.Fields["pkgs"] != "./internal/bridge/..." || !strings.HasPrefix(e.Reason, "skipping env-exclusive package(s) under a live loop: ./internal/bridge/.... Backstop: ") {
		t.Errorf("scope=mixed event: %+v", e)
	}
}

// Test 18 — moved verbatim from audit/envexclusive_bridge_test.go (the
// TestIntegrationTierScope_* pair re-spelled onto tierScope / IntegrationTier).
func TestEnvExclusive_BridgeIsExcluded(t *testing.T) {
	for _, p := range []string{"./internal/bridge/...", "internal/bridge", "github.com/mickeyyaya/evolve-loop/go/internal/bridge"} {
		if !envExclusivePkg(p) {
			t.Fatalf("%q must be env-exclusive: its requireTmux tests false-RED under a live wave (cycle-1539/1543/1546 class)", p)
		}
	}
}

func TestEnvExclusive_OnlyBridge(t *testing.T) {
	for _, p := range []string{"internal/core", "cmd/evolve", "internal/phases/ship",
		"internal/acssuite", "internal/prompts", "internal/inboxmover", "internal/bridgeling", "xinternal/bridge", "x/internal/bridge/y"} {
		if envExclusivePkg(p) {
			t.Fatalf("%q must NOT be env-exclusive — the list is a scalpel (bridge only); contention is the serialized retake's job, and the last fossil here cost 2.5 days of red main (cycle-1594)", p)
		}
	}
}

// The backstop claim must be per-package-honest: bridge's note must say the
// requireTmux subset runs only on a quiet host, and must NOT say CI covers it.
func TestEnvExclusive_BackstopNoteIsHonestForBridge(t *testing.T) {
	note := envExclusiveBackstopNote([]string{"./internal/bridge/...", "internal/bridge"})
	if strings.Contains(note, "CI's isolated integration-tier step covers internal/bridge") {
		t.Fatalf("the note must never claim CI covers bridge — requireTmux skips there; got %q", note)
	}
	if !strings.Contains(note, envExclusiveNoCIMarker) || !strings.Contains(note, "quiet-host") {
		t.Fatalf("the note must deny CI coverage and name the quiet-host backstop; got %q", note)
	}
	if !strings.Contains(note, "requireTmux tests boot real tmux sessions") {
		t.Fatalf("the note must carry the entry's evidence (why), not only the backstop; got %q", note)
	}
	if strings.Count(note, "requireTmux tests boot") != 1 {
		t.Fatalf("one record rendered once for two spellings of the same package; got %q", note)
	}
}

// TestEnvExclusive_EntriesDeclareNoCIBackstop is the RULE, expressed over the
// record table so it needs no package names and cannot fossilize: a package may
// be env-exclusive ONLY when CI provides no backstop.
func TestEnvExclusive_EntriesDeclareNoCIBackstop(t *testing.T) {
	if len(tierEnvExclusive) == 0 {
		t.Fatal("the record table is empty — if the last exclusion was removed on purpose, retire this guard deliberately, not by vacuity")
	}
	for _, e := range tierEnvExclusive {
		if e.pkg == "" || e.why == "" || e.backstop == "" {
			t.Fatalf("entry %+v must carry pkg, evidence, and backstop — an unexplained exclusion is the fossil class", e)
		}
		if !strings.Contains(e.backstop, envExclusiveNoCIMarker) {
			t.Fatalf("%s: an env-exclusive entry must declare CI does NOT back it — a CI-covered package belongs in the lane tier (cycle-1594); backstop: %q", e.pkg, e.backstop)
		}
		if strings.Contains(e.backstop, "CI's isolated integration-tier step covers") {
			t.Fatalf("%s: backstop claims CI coverage — contradiction with the selection criterion; backstop: %q", e.pkg, e.backstop)
		}
	}
}

// THE WIRING: the note must reach the EMITTED error — the string an operator
// and the WARN diagnostic actually see — not merely exist as a correct helper.
func TestIntegrationTierScope_ErrorCarriesTheHonestBackstop(t *testing.T) {
	root, _ := goWorktree(t)
	_, err := New(fakeRunFunc(0, "", "", nil), fixedSet("./internal/bridge/...")).IntegrationTier(tierRequest(root, ""))
	if err == nil {
		t.Fatalf("bridge-only scope must return the env-exclusive WARN error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "quiet-host") || !strings.Contains(msg, "NOT covered by CI") {
		t.Fatalf("the emitted skip must carry bridge's real backstop; got %q", msg)
	}
	if strings.Contains(msg, "CI's isolated integration-tier step covers internal/bridge") {
		t.Fatalf("the emitted skip must not claim CI covers bridge; got %q", msg)
	}
}

// TestIntegrationTierScope_CoversCoreCmdShip_Cycle1594 is the INSTANCE
// regression for the incident: the three packages whose fossilized skip cost
// 2.5 days of red main must stay in the lane tier scope.
func TestIntegrationTierScope_CoversCoreCmdShip_Cycle1594(t *testing.T) {
	changed := []string{"./internal/core/...", "./cmd/evolve/...", "./internal/phases/ship/..."}
	scoped, _, _ := tierScope(changed)
	if strings.Join(scoped, " ") != strings.Join(changed, " ") {
		t.Fatalf("CI-backstopped packages must not be skipped as env-exclusive (cycle-1594 class): got %v", scoped)
	}
}

// Test 19 — the whole-suite fallback lists the module, drops /acs/ and the
// env-exclusive full import paths, and fails OPEN with the golden text + ONE
// GATE_STEP_FAILED{step=tier_list} when `go list` fails.
func TestWholeSuite_ListsFiltersAndFailsOpen(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, _ := goWorktree(t)
	req := tierRequest(root, "")
	var testArgs []string
	g, events := observed(t, func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		if args[0] == "list" {
			_, _ = io.WriteString(so, "ciparitytest/cmd/foo\nciparitytest/acs/regression\nciparitytest/internal/other\nciparitytest/internal/bridge\n")
			return 0, nil
		}
		testArgs = args
		return 0, nil
	}, fixedSet("./..."))
	if off, err := g.IntegrationTier(req); off != nil || err != nil || len(*events) != 0 {
		t.Fatalf("whole suite clean: (%v, %v) events=%v", off, err, codesOf(*events))
	}
	if joined := strings.Join(testArgs, " "); !strings.HasSuffix(joined, " ciparitytest/cmd/foo ciparitytest/internal/other") {
		t.Errorf("/acs/ and the env-exclusive import path must be filtered: %v", testArgs)
	}

	g, events = observed(t, func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, _, se io.Writer) (int, error) {
		_, _ = io.WriteString(se, "boom")
		return 1, nil
	}, fixedSet("./..."))
	off, err := g.IntegrationTier(req)
	if off != nil || err == nil || err.Error() != g1["tier.list_failed"] {
		t.Fatalf("go list failure: (%v, %v), want (nil, the golden text)", off, err)
	}
	e := only(t, *events, CodeGateStepFailed)
	if e.Fields["step"] != "tier_list" || e.Fields["cmd"] != "go list ./..." || e.Reason != err.Error() {
		t.Errorf("GATE_STEP_FAILED fields: %+v", e)
	}
}
