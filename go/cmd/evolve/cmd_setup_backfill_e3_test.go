package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

func TestRunSetupSubcommands_RefuseAnUnknownFlag(t *testing.T) {
	t.Parallel()
	for _, sub := range []string{"detect", "recommend", "apply"} {
		t.Run(sub, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := runSetup([]string{sub, "--nope"}, nil, &out, &errb); code != 10 || out.Len() != 0 || !strings.Contains(errb.String(), "flag provided but not defined: -nope") {
				t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
			}
		})
	}
}

func TestRunSetupJSON_AMarshalFailureExitsOne(t *testing.T) {
	prev := setupJSONIndent
	t.Cleanup(func() { setupJSONIndent = prev })
	setupJSONIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("unsupported value") }
	for _, sub := range []string{"detect", "recommend"} {
		t.Run(sub, func(t *testing.T) {
			root := t.TempDir()
			var out, errb bytes.Buffer

			code := runSetup([]string{sub, "--json", "--project-root", root}, nil, &out, &errb)

			if code != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "evolve setup "+sub+": unsupported value\n") {
				t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
			}
		})
	}
}

func TestRunSetup_AMalformedPresetOverrideRefusesRecommendAndApply(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"recommend"}, {"apply", "--preset", "recommended"}} {
		t.Run(args[0], func(t *testing.T) {
			root := t.TempDir()
			setupWrite(t, filepath.Join(root, ".evolve", "setup-presets.json"), "{not json")
			var out, errb bytes.Buffer

			code := runSetup(append(args, "--project-root", root), nil, &out, &errb)

			if code != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "evolve setup "+args[0]+": setup presets: ") || !strings.Contains(errb.String(), "malformed preset JSON") {
				t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
			}
		})
	}
}

func TestPrintDetectHuman_RendersEveryOptionalSection(t *testing.T) {
	t.Parallel()
	rep := setup.DetectReport{
		CLIs: []setup.CLIStatus{
			{CLI: "claude", BinaryPresent: true, AuthConfigured: true, AuthMode: "oauth", CapabilityTier: "top", Verdict: "ready",
				TierModels: map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"}},
			{CLI: "codex", AuthMode: "none", CapabilityTier: "n/a", Verdict: "blocked"},
		},
		Phases: []setup.PhaseStatus{
			{Role: "builder", CurrentCLI: "agy", CurrentTier: "balanced", Source: "pin", Envelope: setup.Envelope{Min: "fast", Max: "deep"}, PinViolation: "tier top above envelope max deep"},
			{Role: "auditor", CurrentCLI: "claude", CurrentTier: "deep", Source: "profile", Envelope: setup.Envelope{Min: "deep", Max: "top"}},
		},
		PolicyError:      "unexpected end of JSON input",
		SetupCompletedAt: "2026-10-12T09:00:00Z",
		SetupVersion:     3,
	}
	var out bytes.Buffer

	printDetectHuman(&out, rep)

	got := out.String()
	for _, want := range []string{
		"  claude  ✓ binary  ✓ oauth",
		"          models: fast=haiku  balanced=sonnet  deep=opus\n",
		"  codex   ✗ binary  none",
		"\n⚠ policy.json malformed (pins ignored): unexpected end of JSON input\n",
		"      ✗ pin violation: tier top above envelope max deep\n",
		"\nSetup: completed at 2026-10-12T09:00:00Z (v3).\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "pin violation") != 1 || strings.Count(got, "models:") != 1 {
		t.Errorf("a clean phase or a CLI without models printed an optional line:\n%s", got)
	}
	for _, absent := range []string{"NOT yet completed", "cli_routing table"} {
		if strings.Contains(got, absent) {
			t.Errorf("report printed %q for a completed pins project:\n%s", absent, got)
		}
	}
}

func TestPrintDetectHuman_AnEmptyReportStillNamesTheSetupState(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	printDetectHuman(&out, setup.DetectReport{})

	want := "Detected LLM CLIs:\n\nPer-phase routing (current):\n\nSetup: NOT yet completed (run /setup).\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}
