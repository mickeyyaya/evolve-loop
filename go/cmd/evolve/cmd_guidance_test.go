package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/guidance"
)

func guidanceRule(id, layer, extra string) string {
	return "---\nid: " + id + "\nstatement: Rule " + id + " holds.\nlayer: " + layer + "\naudience: [all]\nsource: test\nstatus: active\nadded: 2026-10-26\nevidence: a test\n" + extra + "---\nWhy: a test.\n"
}

func guidanceTree(t *testing.T, rules map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "guidance", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	agents := "# AGENTS\n" + guidance.L0Begin + " -->\n" + guidance.L0End + "\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	for id, text := range rules {
		if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runGuidanceCmd(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	rc := runGuidance(args, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestRunGuidance_BuildThenCheckPassesAndASecondBuildWritesNothing(t *testing.T) {
	t.Parallel()
	root := guidanceTree(t, map[string]string{"safe": guidanceRule("safe", "L0", "")})

	rc, out, errOut := runGuidanceCmd("build", "--project-root", root)
	if rc != 0 || !strings.Contains(out, "wrote AGENTS.md") || !strings.Contains(out, "3 file(s) written") {
		t.Fatalf("build = %d, %q, %q", rc, out, errOut)
	}
	rc, out, _ = runGuidanceCmd("build", "--project-root", root)
	if rc != 0 || !strings.Contains(out, "0 file(s) written") {
		t.Fatalf("second build = %d, %q", rc, out)
	}
	rc, out, errOut = runGuidanceCmd("check", "--project-root", root)
	if rc != 0 || out != "guidance check OK\n" {
		t.Fatalf("check = %d, %q, %q", rc, out, errOut)
	}
}

func TestRunGuidance_CheckExitsOneAndListsEachProblem(t *testing.T) {
	t.Parallel()
	root := guidanceTree(t, map[string]string{"safe": guidanceRule("safe", "L0", "enforced_by: [go/absent.go]\n")})

	rc, _, errOut := runGuidanceCmd("check", "--project-root", root)

	if rc != exitGuidanceFailed || !strings.Contains(errOut, "STALE: ") || !strings.Contains(errOut, "ENFORCED_BY: ") || !strings.Contains(errOut, "problem(s)") {
		t.Fatalf("check = %d, %q", rc, errOut)
	}
}

func TestRunGuidance_BuildAndCheckExitCodesOnBadInput(t *testing.T) {
	t.Parallel()
	bad := guidanceTree(t, map[string]string{"bad": "no frontmatter\n"})
	noRegion := guidanceTree(t, map[string]string{"safe": guidanceRule("safe", "L0", "")})
	if err := os.WriteFile(filepath.Join(noRegion, "AGENTS.md"), []byte("# AGENTS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brokenRegion := guidanceTree(t, nil)
	if err := os.WriteFile(filepath.Join(brokenRegion, "AGENTS.md"), []byte(guidance.L0Begin+" -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unwritable := guidanceTree(t, nil)
	if err := os.Mkdir(filepath.Join(unwritable, "guidance", "index.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"build bad registry", []string{"build", "--project-root", bad}, exitGuidanceInput},
		{"build no registry", []string{"build", "--project-root", t.TempDir()}, exitIO},
		{"build no region", []string{"build", "--project-root", noRegion}, exitGuidanceInput},
		{"build broken region", []string{"build", "--project-root", brokenRegion}, exitIO},
		{"build unwritable output", []string{"build", "--project-root", unwritable}, exitIO},
		{"check no registry", []string{"check", "--project-root", t.TempDir()}, exitIO},
		{"check stray word", []string{"check", "x"}, exitUsage},
		{"build index flag", []string{"build", "--json"}, exitUsage},
		{"no subcommand", nil, exitUsage},
		{"unknown subcommand", []string{"lint"}, exitUsage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if rc, _, errOut := runGuidanceCmd(c.args...); rc != c.want {
				t.Fatalf("%v = %d (%q), want %d", c.args, rc, errOut, c.want)
			}
		})
	}
}

func TestRunGuidance_HelpPrintsTheUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--help"}, {"index", "--help"}} {
		if rc, out, _ := runGuidanceCmd(args...); rc != 0 || !strings.HasPrefix(out, "usage: evolve guidance build") {
			t.Fatalf("%v = %d, %q", args, rc, out)
		}
	}
}

type guidanceTestEnvelope struct {
	Schema string `json:"schema"`
	OK     bool   `json:"ok"`
	Data   *struct {
		Filter  guidanceFilterOut   `json:"filter"`
		Entries []guidance.IndexRow `json:"entries"`
	} `json:"data"`
	Error    *guidanceError `json:"error"`
	Warnings []string       `json:"warnings"`
}

func decodeGuidanceEnvelope(t *testing.T, out string) guidanceTestEnvelope {
	t.Helper()
	var env guidanceTestEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("not one JSON envelope: %v\n%s", err, out)
	}
	return env
}

func TestRunGuidance_IndexFiltersByAudiencePhaseAndTheFamilyOfTheCLI(t *testing.T) {
	t.Parallel()
	root := guidanceTree(t, map[string]string{
		"safe":    guidanceRule("safe", "L0", ""),
		"claude":  strings.Replace(guidanceRule("claude", "L2", "families: [claude]\n"), "audience: [all]", "audience: [phase]", 1),
		"console": strings.Replace(guidanceRule("console", "L1", ""), "audience: [all]", "audience: [console]", 1),
	})

	rc, out, errOut := runGuidanceCmd("index", "--audience", "phase", "--phase", "build", "--cli", "codex", "--json", "--project-root", root)

	env := decodeGuidanceEnvelope(t, out)
	if rc != 0 || errOut != "" || env.Schema != guidanceIndexSchema || !env.OK || env.Error != nil || env.Warnings == nil {
		t.Fatalf("index = %d, %q, %+v", rc, errOut, env)
	}
	if env.Data.Filter.Family != "gpt" || len(env.Data.Entries) != 1 || env.Data.Entries[0].ID != "safe" || env.Data.Entries[0].Kind != guidance.EntryKindRule || env.Data.Entries[0].Bytes == 0 {
		t.Fatalf("index data = %+v, want only safe for the gpt family", env.Data)
	}
	rc, out, _ = runGuidanceCmd("index", "--cli", "claude-tmux", "--project-root", root)
	if rc != 0 || !strings.Contains(out, "[claude](guidance/rules/claude.md)") || !strings.Contains(out, "## L0 Invariants") {
		t.Fatalf("index markdown = %d, %q", rc, out)
	}
}

func TestRunGuidance_IndexPrintsTheEnvelopeOnEachFailure(t *testing.T) {
	t.Parallel()
	bad := guidanceTree(t, map[string]string{"bad": "no frontmatter\n"})
	cases := []struct {
		name string
		args []string
		code int
		kind string
	}{
		{"unknown audience", []string{"index", "--audience", "everyone", "--json"}, exitUsage, "usage"},
		{"unknown flag", []string{"index", "--colour", "--json"}, exitUsage, "usage"},
		{"unknown CLI", []string{"index", "--cli", "cobol", "--json", "--project-root", bad}, exitUsage, "usage"},
		{"bad registry", []string{"index", "--json", "--project-root", bad}, exitGuidanceInput, "input"},
		{"no registry", []string{"index", "--json", "--project-root", t.TempDir()}, exitIO, "io"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, out, errOut := runGuidanceCmd(c.args...)
			env := decodeGuidanceEnvelope(t, out)
			if rc != c.code || env.OK || env.Data != nil || env.Error == nil || env.Error.Code != c.code || env.Error.Kind != c.kind || env.Error.Message == "" || !strings.HasPrefix(errOut, guidancePrefix) {
				t.Fatalf("%v = %d, %+v, %q", c.args, rc, env, errOut)
			}
		})
	}
	if rc, out, _ := runGuidanceCmd("index", "--project-root", bad); rc != exitGuidanceInput || out != "" {
		t.Fatalf("index without --json on a bad registry = %d, %q; want 11 and no stdout", rc, out)
	}
}

func TestRunGuidance_ARepoRootFailureExitsTwo(t *testing.T) {
	restore := guidanceRepoRoot
	t.Cleanup(func() { guidanceRepoRoot = restore })
	guidanceRepoRoot = func(string) (string, error) { return "", errors.New("not in a git work tree") }

	for _, verb := range []string{"build", "index"} {
		if rc, _, errOut := runGuidanceCmd(verb); rc != exitIO || !strings.Contains(errOut, "not in a git work tree") {
			t.Fatalf("%s = %d, %q", verb, rc, errOut)
		}
	}
}

func TestGuidanceFamilies_NamesEachModelFamilyOnceAndFailsOnABadManifest(t *testing.T) {
	restoreNames, restoreLoad := guidanceManifestNames, guidanceLoadManifest
	t.Cleanup(func() { guidanceManifestNames, guidanceLoadManifest = restoreNames, restoreLoad })

	got, err := guidanceFamilies()
	if err != nil || strings.Join(got, ",") != "claude,gemini,gpt,local" {
		t.Fatalf("guidanceFamilies = %v, %v", got, err)
	}
	guidanceManifestNames = func() []string { return []string{"broken"} }
	guidanceLoadManifest = func(string) (bridge.Manifest, error) { return bridge.Manifest{}, errors.New("bad json") }
	if _, err := guidanceFamilies(); err == nil {
		t.Fatal("guidanceFamilies = nil error, want the manifest error")
	}
	root := guidanceTree(t, nil)
	if rc, _, errOut := runGuidanceCmd("check", "--project-root", root); rc != exitIO || !strings.Contains(errOut, "bad json") {
		t.Fatalf("check = %d, %q; want exit 2 with the manifest error", rc, errOut)
	}
	if rc, out, errOut := runGuidanceCmd("index", "--cli", "broken", "--json", "--project-root", root); rc != exitIO || !strings.Contains(errOut, "bad json") || !strings.Contains(out, `"kind": "io"`) {
		t.Fatalf("index --cli broken = %d, %q, %q; want exit 2 with the manifest error", rc, out, errOut)
	}
	guidanceLoadManifest = func(string) (bridge.Manifest, error) { return bridge.Manifest{}, nil }
	if rc, _, errOut := runGuidanceCmd("index", "--cli", "broken", "--project-root", root); rc != exitUsage || !strings.Contains(errOut, "no model family") {
		t.Fatalf("index --cli with no family = %d, %q; want exit 10", rc, errOut)
	}
}

type failingGuidanceWriter struct{}

func (failingGuidanceWriter) Write([]byte) (int, error) { return 0, errors.New("stdout closed") }

func TestRunGuidance_AnUnwritableEnvelopeExitsTwo(t *testing.T) {
	t.Parallel()
	root := guidanceTree(t, nil)
	var stderr bytes.Buffer

	rc := runGuidance([]string{"index", "--json", "--project-root", root}, nil, failingGuidanceWriter{}, &stderr)

	if rc != exitIO || !strings.Contains(stderr.String(), "stdout closed") {
		t.Fatalf("index --json to a failing writer = %d, %q; want exit 2", rc, stderr.String())
	}
}
