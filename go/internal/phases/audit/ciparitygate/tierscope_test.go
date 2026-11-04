package ciparitygate

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func TestIntegrationTierScope_EnvExclusiveEventsWithoutStderr(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")

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
	if e.Fields["scope"] != "all" || e.Fields["pkgs"] != "./internal/bridge/..." || e.Fields["remainder"] != "0" || !strings.Contains(e.Fields["backstop"], ciparity.EnvExclusiveCIMarker) || e.Reason != err.Error() {
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

func TestIntegrationTierScope_ErrorCarriesTheHonestBackstop(t *testing.T) {
	root, _ := goWorktree(t)
	_, err := New(fakeRunFunc(0, "", "", nil), fixedSet("./internal/bridge/...")).IntegrationTier(tierRequest(root, ""))
	if err == nil {
		t.Fatalf("bridge-only scope must return the env-exclusive WARN error")
	}
	msg := err.Error()
	if !strings.Contains(msg, ciparity.EnvExclusiveCIMarker) || !strings.Contains(msg, "macOS") {
		t.Fatalf("the emitted skip must carry bridge's real backstop; got %q", msg)
	}
	if strings.Contains(msg, "NOT covered by CI") || strings.Contains(msg, "quiet-host") {
		t.Fatalf("the emitted skip must not deny the CI coverage; got %q", msg)
	}
}

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
