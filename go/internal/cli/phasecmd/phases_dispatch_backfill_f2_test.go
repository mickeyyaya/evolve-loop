package phasecmd

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecoherence"
)

var wantSubcommandNamesF2 = []string{"list", "validate", "check-coherence", "check-artifact-coherence", "check-provenance", "add", "create"}

func TestPhasesSubcommandNames_ListsEverySubcommandInTableOrderF2(t *testing.T) {
	t.Parallel()

	got := phasesSubcommandNames()

	if !slices.Equal(got, wantSubcommandNamesF2) {
		t.Errorf("phasesSubcommandNames = %v, want %v", got, wantSubcommandNamesF2)
	}
	usage := phasesUsage()
	for _, name := range got {
		if !strings.Contains(usage, name) {
			t.Errorf("usage %q does not mention subcommand %q", usage, name)
		}
	}
}

func TestLookupPhasesSubcommand_FindsExactNamesOnlyF2(t *testing.T) {
	t.Parallel()
	for _, name := range wantSubcommandNamesF2 {
		sub, ok := lookupPhasesSubcommand(name)

		if !ok || sub.name != name || sub.run == nil || sub.usage == "" {
			t.Errorf("lookupPhasesSubcommand(%q) = (%+v, %v), want its own entry", name, sub, ok)
		}
	}
	for _, name := range []string{"", "LIST", " list", "list ", "lis", "listing", "check", "delete", "--help"} {
		sub, ok := lookupPhasesSubcommand(name)

		if ok || sub.name != "" || sub.run != nil {
			t.Errorf("lookupPhasesSubcommand(%q) = (%+v, %v), want not found and a zero value", name, sub, ok)
		}
	}
}

func TestRunPhases_RefusesUsageErrorsWithExitTenAndTheRightMessageF2(t *testing.T) {
	newCoherenceProject(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown subcommand", []string{"bogus"}, `unknown subcommand "bogus" (want ` + strings.Join(wantSubcommandNamesF2, "|") + `)`},
		{"no subcommand", nil, "usage: evolve phases [--profile-dir <dir>] [--persona-override <path>:<name>] <" + phasesUsage() + ">"},
		{"persona override without a name", []string{"--persona-override", "/tmp/p.md", "list"}, `--persona-override "/tmp/p.md": want <path>:<name>`},
		{"persona override without a path", []string{"--persona-override", ":widget", "list"}, "want <path>:<name>"},
		{"unknown flag", []string{"--bogus", "list"}, "flag provided but not defined"},
		{"flag after the subcommand is the subcommands", []string{"bogus", "--profile-dir", "x"}, `unknown subcommand "bogus"`},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer

		rc := RunPhases(tc.args, nil, &out, &errb)

		if rc != 10 || out.Len() != 0 || !strings.Contains(errb.String(), tc.want) {
			t.Errorf("%s: rc=%d out=%q err=%q, want exit 10 containing %q", tc.name, rc, out.String(), errb.String(), tc.want)
		}
	}
}

func TestPhasesValidate_NamingAMissingUserPhaseIsAUsageErrorUnlessProvenanceAlreadyFailedF2(t *testing.T) {
	root := newCoherenceProject(t)
	writeCoherenceProfile(t, root, "naked", `{"name":"naked","role":"naked","cli":"claude-tmux","model_tier_default":"sonnet"}`)
	cases := []struct {
		name   string
		args   []string
		wantRC int
	}{
		{"missing phase", []string{"no-such-phase"}, 10},
		{"missing phase and strict provenance fails", []string{"no-such-phase", "--strict-provenance"}, 2},
		{"flag order does not matter", []string{"--strict-provenance", "no-such-phase"}, 2},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer

		rc := phasesValidate(root, "", tc.args, &out, &errb)

		if rc != tc.wantRC || !strings.Contains(errb.String(), `no user phase named "no-such-phase" in any phase root`) {
			t.Errorf("%s: rc=%d err=%q, want exit %d naming the phase", tc.name, rc, errb.String(), tc.wantRC)
		}
		if !strings.Contains(out.String(), "WARN: profile naked missing generated_from") {
			t.Errorf("%s: stdout %q lacks the provenance warning", tc.name, out.String())
		}
	}
}

func TestPhasesValidate_SaysSoWhenThereIsNothingToValidateF2(t *testing.T) {
	root := newCoherenceProject(t)
	var out, errb bytes.Buffer

	rc := phasesValidate(root, "", nil, &out, &errb)

	if rc != 0 || out.String() != "no user phases to validate\n" {
		t.Errorf("rc=%d out=%q err=%q, want exit 0 and the nothing-to-validate line", rc, out.String(), errb.String())
	}
	writeCoherenceProfile(t, root, "stamped", `{"name":"stamped","role":"stamped","cli":"claude-tmux","model_tier_default":"sonnet","generated_from":"hand-authored"}`)
	out.Reset()

	rc = phasesValidate(root, "", nil, &out, &errb)

	if rc != 0 || strings.Contains(out.String(), "no user phases") {
		t.Errorf("with a profile present: rc=%d out=%q, want no nothing-to-validate line", rc, out.String())
	}
}

func TestPhasesCoherence_ForwardsOptionsAndMapsOutcomeToExitCodeF2(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "marker.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agents-marker.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	violation := phasecoherence.Violation{Severity: "WARN", Persona: "evolve-x", Message: "drift"}
	cases := []struct {
		name       string
		args       []string
		check      func(phasecoherence.Options) ([]phasecoherence.Violation, error)
		wantRC     int
		wantStdout string
		wantStderr string
	}{
		{"clean", nil, func(phasecoherence.Options) ([]phasecoherence.Violation, error) { return nil, nil }, 0, "", ""},
		{"violation without strict", nil, func(phasecoherence.Options) ([]phasecoherence.Violation, error) {
			return []phasecoherence.Violation{violation}, nil
		}, 0, "WARN: evolve-x: drift\n", ""},
		{"violation with strict", []string{"--strict"}, func(phasecoherence.Options) ([]phasecoherence.Violation, error) {
			return []phasecoherence.Violation{violation}, nil
		}, 2, "WARN: evolve-x: drift\n", ""},
		{"strict but clean", []string{"--strict"}, func(phasecoherence.Options) ([]phasecoherence.Violation, error) { return nil, nil }, 0, "", ""},
		{"checker fails", []string{"--strict"}, func(phasecoherence.Options) ([]phasecoherence.Violation, error) {
			return nil, errors.New("check boom")
		}, 1, "", "check-coherence: check boom\n"},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer
		var seen phasecoherence.Options
		call := phasesCall{project: root, args: tc.args, overrides: map[string]string{"widget": "/p.md"}, stdout: &out, stderr: &errb}

		rc := phasesCoherence(call, "check-coherence", func(o phasecoherence.Options) ([]phasecoherence.Violation, error) {
			seen = o
			return tc.check(o)
		})

		if rc != tc.wantRC || out.String() != tc.wantStdout || errb.String() != tc.wantStderr {
			t.Errorf("%s: rc=%d out=%q err=%q, want %d %q %q", tc.name, rc, out.String(), errb.String(), tc.wantRC, tc.wantStdout, tc.wantStderr)
		}
		if _, err := fs.Stat(seen.ProfilesFS, "marker.json"); err != nil {
			t.Errorf("%s: ProfilesFS is not the project's default profile dir: %v", tc.name, err)
		}
		if _, err := fs.Stat(seen.AgentsFS, "agents-marker.txt"); err != nil || seen.Overrides["widget"] != "/p.md" {
			t.Errorf("%s: AgentsFS/Overrides not forwarded (%v, %v)", tc.name, err, seen.Overrides)
		}
	}
}

func TestPhasesCoherence_AProfileDirFlagReplacesTheDefaultF2(t *testing.T) {
	root := t.TempDir()
	alt := t.TempDir()
	if err := os.WriteFile(filepath.Join(alt, "alt.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var seen phasecoherence.Options

	rc := phasesCoherence(phasesCall{project: root, profileDir: alt, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}, "x", func(o phasecoherence.Options) ([]phasecoherence.Violation, error) {
		seen = o
		return nil, nil
	})

	if _, err := fs.Stat(seen.ProfilesFS, "alt.json"); rc != 0 || err != nil {
		t.Errorf("rc=%d stat=%v, want the explicit profile dir used", rc, err)
	}
}
