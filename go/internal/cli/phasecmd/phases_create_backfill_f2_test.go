package phasecmd

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestParseCreateFlags_RefusesEveryMalformedInvocationWithExitTenF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"--bogus"}, "flag provided but not defined"},
		{"neither spec nor mint", nil, "usage: evolve phases create --spec"},
		{"both spec and mint", []string{"--spec", "a.json", "--mint", "m.json"}, "usage: evolve phases create --spec"},
		{"spec and persona both from stdin", []string{"--spec", "-", "--persona", "-"}, "at most one of --spec/--persona/--mint may read stdin"},
		{"mint and persona both from stdin", []string{"--mint", "-", "--persona", "-"}, "at most one of --spec/--persona/--mint may read stdin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var errb bytes.Buffer

			flags, code := parseCreateFlags(tc.args, &errb)

			if code != 10 || flags != (createFlags{}) || !strings.Contains(errb.String(), tc.want) {
				t.Errorf("flags=%+v code=%d err=%q, want a zero value, exit 10 and %q", flags, code, errb.String(), tc.want)
			}
		})
	}
}

func TestParseCreateFlags_AcceptsOneStdinConsumerAndKeepsEveryValueF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want createFlags
	}{
		{"spec from stdin with a persona file", []string{"--spec", "-", "--persona", "p.md", "--root", "r"}, createFlags{spec: "-", persona: "p.md", root: "r"}},
		{"persona from stdin with a spec file", []string{"--spec", "s.json", "--persona", "-"}, createFlags{spec: "s.json", persona: "-"}},
		{"mint alone", []string{"--mint", "m.json"}, createFlags{mint: "m.json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var errb bytes.Buffer

			flags, code := parseCreateFlags(tc.args, &errb)

			if code != 0 || flags != tc.want || errb.Len() != 0 {
				t.Errorf("flags=%+v code=%d err=%q, want %+v", flags, code, errb.String(), tc.want)
			}
		})
	}
}

func TestPhasesCreate_AnUnreadableOrMalformedSpecIsReportedWithoutWritingF2(t *testing.T) {
	root := createFixtureProject(t)
	missing := filepath.Join(root, "no-such-spec.json")

	code, out, errb := runCreate(t, "", "--spec", missing)

	if code != 1 || out != "" || !strings.Contains(errb, "read --spec:") {
		t.Errorf("missing spec: code=%d out=%q err=%q, want exit 1 and the read error", code, out, errb)
	}

	code, out, errb = runCreate(t, "{not json", "--spec", "-")

	env := parseEnvelope(t, out)
	if code != 2 || env.OK || len(env.Errors) != 1 || !strings.HasPrefix(env.Errors[0], "spec JSON malformed:") || env.Hint == "" {
		t.Errorf("malformed spec: code=%d env=%+v err=%q, want exit 2 with one malformed-JSON error and a hint", code, env, errb)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "phases")); !os.IsNotExist(err) {
		t.Errorf("a refused create wrote the phase root (%v)", err)
	}
}

func TestCheckPersonaPath_RefusesOutsideAgentsAndOverwritesAndWarnsOnAMissingPersonaF2(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	agents := filepath.Join(project, "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(agents, "evolve-old.md")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		path        string
		agent       string
		hasBody     bool
		wantRefusal string
		wantWarning string
	}{
		{"outside agents", filepath.Join(project, "elsewhere", "evolve-x.md"), "evolve-x", true, `agent "evolve-x" resolves outside agents/ — refusing`, ""},
		{"a sibling dir sharing the prefix", filepath.Join(project, "agents-extra", "x.md"), "x", false, `agent "x" resolves outside agents/ — refusing`, ""},
		{"traversal out of agents", filepath.Clean(filepath.Join(agents, "..", "x.md")), "../x", true, `agent "../x" resolves outside agents/ — refusing`, ""},
		{"body would overwrite", existing, "evolve-old", true, "persona agents/evolve-old.md already exists — refusing to overwrite", ""},
		{"body for a new persona", filepath.Join(agents, "evolve-new.md"), "evolve-new", true, "", ""},
		{"no body and no file", filepath.Join(agents, "evolve-none.md"), "evolve-none", false, "", "no persona supplied and agents/evolve-none.md does not exist"},
		{"no body but a file exists", existing, "evolve-old", false, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refusal, warning := checkPersonaPath(project, tc.path, tc.agent, tc.hasBody)

			if refusal != tc.wantRefusal || (tc.wantWarning == "") != (warning == "") || !strings.Contains(warning, tc.wantWarning) {
				t.Errorf("checkPersonaPath = (%q, %q), want (%q, containing %q)", refusal, warning, tc.wantRefusal, tc.wantWarning)
			}
		})
	}
}

func TestWritePhaseFiles_ReportsEachFailureAndRollsBackOnlyWhatItCreatedF2(t *testing.T) {
	t.Parallel()
	spec := phasespec.PhaseSpec{Name: "demo", Kind: "llm"}

	t.Run("spec that cannot be marshalled", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		bad := spec
		bad.Routing = &config.RoutingBlock{InsertWhen: []config.Condition{{Field: "f", Op: "eq", Value: math.NaN()}}}
		var errb bytes.Buffer

		path, code := writePhaseFiles(root, bad, filepath.Join(root, "agents", "p.md"), "body", &errb)

		entries, _ := os.ReadDir(root)
		if path != "" || code != 1 || !strings.Contains(errb.String(), "marshal spec:") || len(entries) != 0 {
			t.Errorf("path=%q code=%d err=%q files=%v, want exit 1 and nothing written", path, code, errb.String(), entries)
		}
	})
	t.Run("phase.json cannot be written", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		blocker := filepath.Join(root, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		var errb bytes.Buffer

		path, code := writePhaseFiles(blocker, spec, filepath.Join(root, "agents", "p.md"), "body", &errb)

		if path != "" || code != 1 || !strings.Contains(errb.String(), "write "+filepath.Join(blocker, "demo", "phase.json")) {
			t.Errorf("path=%q code=%d err=%q, want exit 1 naming phase.json", path, code, errb.String())
		}
	})
	t.Run("persona failure removes the directory this call created", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		agentsFile := filepath.Join(root, "agents")
		if err := os.WriteFile(agentsFile, []byte("a file where the agents dir should be"), 0o644); err != nil {
			t.Fatal(err)
		}
		var errb bytes.Buffer

		path, code := writePhaseFiles(filepath.Join(root, "phases"), spec, filepath.Join(agentsFile, "p.md"), "body", &errb)

		if _, err := os.Stat(filepath.Join(root, "phases", "demo")); path != "" || code != 1 || !os.IsNotExist(err) || !strings.Contains(errb.String(), "write "+filepath.Join(agentsFile, "p.md")) {
			t.Errorf("path=%q code=%d stat=%v err=%q, want exit 1 and the created dir removed", path, code, err, errb.String())
		}
	})
	t.Run("persona failure keeps a directory the operator already had", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		phaseDir := filepath.Join(root, "phases", "demo")
		if err := os.MkdirAll(phaseDir, 0o755); err != nil {
			t.Fatal(err)
		}
		notes := filepath.Join(phaseDir, "notes.txt")
		if err := os.WriteFile(notes, []byte("operator notes"), 0o644); err != nil {
			t.Fatal(err)
		}
		agentsFile := filepath.Join(root, "agents")
		if err := os.WriteFile(agentsFile, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		var errb bytes.Buffer

		_, code := writePhaseFiles(filepath.Join(root, "phases"), spec, filepath.Join(agentsFile, "p.md"), "body", &errb)

		if _, err := os.Stat(notes); code != 1 || err != nil {
			t.Errorf("code=%d notes stat=%v, want exit 1 and the operator file kept", code, err)
		}
		if _, err := os.Stat(filepath.Join(phaseDir, "phase.json")); !os.IsNotExist(err) {
			t.Errorf("phase.json was not rolled back (%v)", err)
		}
	})
	t.Run("success writes the spec and the persona", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		persona := filepath.Join(root, "agents", "p.md")
		var errb bytes.Buffer

		path, code := writePhaseFiles(filepath.Join(root, "phases"), spec, persona, "body\n", &errb)

		raw, err := os.ReadFile(path)
		if code != 0 || err != nil || !strings.Contains(string(raw), `"name": "demo"`) || errb.Len() != 0 {
			t.Fatalf("path=%q code=%d read=%v err=%q", path, code, err, errb.String())
		}
		if body, _ := os.ReadFile(persona); string(body) != "body\n" {
			t.Errorf("persona = %q", body)
		}
	})
}

func TestRebuildInventory_ReportsAFailedRebuildAsAWarningAndKeepsEarlierOnesF2(t *testing.T) {
	t.Parallel()

	t.Run("failure", func(t *testing.T) {
		t.Parallel()
		project := t.TempDir()
		if err := os.WriteFile(filepath.Join(project, ".evolve"), []byte("a file, not a dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		earlier := []string{"earlier warning"}

		rebuilt, warnings := rebuildInventory(project, []string{filepath.Join(project, "phases")}, earlier)

		if rebuilt || len(warnings) != 2 || warnings[0] != "earlier warning" ||
			!strings.HasPrefix(warnings[1], "inventory rebuild failed (") || !strings.HasSuffix(warnings[1], "run `evolve phase-inventory build --force`") {
			t.Errorf("rebuildInventory = (%v, %q), want false and the earlier warning plus the rebuild failure", rebuilt, warnings)
		}
	})
	t.Run("success rebuilds even when a fresh inventory exists", func(t *testing.T) {
		t.Parallel()
		project := t.TempDir()
		roots := []string{filepath.Join(project, ".evolve", "phases")}

		first, warnings := rebuildInventory(project, roots, nil)
		second, _ := rebuildInventory(project, roots, nil)

		if !first || !second || len(warnings) != 0 {
			t.Errorf("rebuilt first=%v second=%v warnings=%v, want both forced rebuilds and no warning", first, second, warnings)
		}
	})
}
