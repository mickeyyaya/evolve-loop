package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const declaredProjectPolicy = `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"], "agents": {"scout": ["claude"]}}}`

func declaredSetupProject(t *testing.T) (policyPath string, before []byte) {
	t.Helper()
	project := t.TempDir()
	evolveDir := filepath.Join(project, ".evolve")
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	setupWrite(t, filepath.Join(evolveDir, "profiles", "scout.json"), `{"name":"scout","cli":"codex-tmux","model_tier_default":"balanced","allowed_clis":["all"]}`)
	policyPath = filepath.Join(evolveDir, "policy.json")
	setupWrite(t, policyPath, declaredProjectPolicy)
	before, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	return policyPath, before
}

func TestRunSetupApply_ADeclaredProjectIsRefusedAndKeepsTheOperatorsRules(t *testing.T) {
	policyPath, before := declaredSetupProject(t)
	var out, errb bytes.Buffer

	rc := runSetup([]string{"apply", "--preset", "recommended"}, nil, &out, &errb)

	after, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if rc != 1 || !bytes.Equal(before, after) || strings.Contains(out.String(), "wrote") {
		t.Fatalf("rc=%d stdout=%q policy=%s: a declared table owns every route, so a preset writes nothing and the agents.scout rule stays", rc, out.String(), after)
	}
	if !strings.Contains(errb.String(), "evolve cli-routing set agents.<role>") || !strings.Contains(errb.String(), "evolve cli-routing show") {
		t.Fatalf("stderr=%q: the refusal names the verbs that do route a declared project", errb.String())
	}
}

func TestRunSetupRecommend_ADeclaredProjectOffersNoPresets(t *testing.T) {
	for _, args := range [][]string{{"recommend", "--json"}, {"recommend"}} {
		declaredSetupProject(t)
		var out, errb bytes.Buffer

		rc := runSetup(args, nil, &out, &errb)

		if rc != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "setup presets do not apply") {
			t.Fatalf("%v: rc=%d stdout=%q stderr=%q: a declared project gets the pointer to cli-routing, never presets apply would refuse, in JSON and human mode alike", args, rc, out.String(), errb.String())
		}
	}
}

func TestRunSetupDetect_ReportsADeclaredTable(t *testing.T) {
	declaredSetupProject(t)
	var out, errb bytes.Buffer

	rc := runSetup([]string{"detect", "--json"}, nil, &out, &errb)

	if rc != 0 || !strings.Contains(out.String(), `"routing_table_declared": true`) {
		t.Fatalf("rc=%d stdout=%s: detect tells the skill the table owns the routes", rc, out.String())
	}
}

func TestRunSetupDetect_TheHumanReportNamesTheDeclaredTable(t *testing.T) {
	declaredSetupProject(t)
	var out, errb bytes.Buffer

	rc := runSetup([]string{"detect"}, nil, &out, &errb)

	if rc != 0 || !strings.Contains(out.String(), "declares a cli_routing table") || !strings.Contains(out.String(), "evolve cli-routing show") {
		t.Fatalf("rc=%d stdout:\n%s\nthe human report says the table owns every route and how to read it", rc, out.String())
	}
}

func TestRunSetupDetect_TheHumanReportOfAPinsProjectNamesNoTable(t *testing.T) {
	project := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	setupWrite(t, filepath.Join(project, ".evolve", "policy.json"), `{"pins": {}}`)
	var out, errb bytes.Buffer

	rc := runSetup([]string{"detect"}, nil, &out, &errb)

	if rc != 0 || strings.Contains(out.String(), "cli_routing") {
		t.Fatalf("rc=%d stdout:\n%s\na project without a table hears nothing about one", rc, out.String())
	}
}
