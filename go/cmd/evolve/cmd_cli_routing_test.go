package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const routingFixtureProfiles = `{
 "auditor": {"name":"auditor","cli":"claude-tmux","cli_fallback":["claude-p"],"allowed_clis":["claude"],"model_tier_default":"deep","cross_family_with":"builder"},
 "builder": {"name":"builder","cli":"codex-tmux","cli_fallback":["claude-tmux"],"allowed_clis":["claude","codex","agy"],"model_tier_default":"balanced","cross_family_with":"auditor"},
 "scout":   {"name":"scout","cli":"codex-tmux","cli_fallback":["claude-tmux"],"model_tier_default":"balanced"},
 "intent":  {"name":"intent","cli":"claude-tmux","model_tier_default":"deep","model_tier_envelope":{"min":"deep","max":"deep"}}
}`

func routingFixture(t *testing.T, policyJSON string) string {
	t.Helper()
	root, evolveDir := wiringRoot(t, policyJSON)
	var docs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(routingFixtureProfiles), &docs); err != nil {
		t.Fatal(err)
	}
	for name, doc := range docs {
		if err := os.WriteFile(filepath.Join(evolveDir, "profiles", name+".json"), doc, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const operatorTable = `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"],"tiers":{"deep":["claude"],"top":["claude"]}}}`

func runRoutingVerb(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runCLIRouting(args, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestCLIRouting_UsageErrorsExitTwo(t *testing.T) {
	root := routingFixture(t, "")
	for _, args := range [][]string{nil, {"bogus"}, {"show", "extra", "--project-root", root}, {"explain", "--project-root", root}, {"check", "--nope"}, {"check", "--project-root", root, "x"}} {
		if rc, _, _ := runRoutingVerb(t, args...); rc != exitRoutingUsage {
			t.Errorf("args %v: rc=%d, want 2", args, rc)
		}
	}
	if rc, out, _ := runRoutingVerb(t, "--help"); rc != 0 || !strings.Contains(out, "show") {
		t.Fatalf("help: rc=%d out=%q", rc, out)
	}
}

func TestCLIRoutingCheck_ExitsZeroWhenItCompilesAndOneOnAnErrorFinding(t *testing.T) {
	if rc, out, _ := runRoutingVerb(t, "check", "--project-root", routingFixture(t, "")); rc != 0 || !strings.Contains(out, "OK") {
		t.Fatalf("a legacy tree checks clean: rc=%d %q", rc, out)
	}
	collapsed := `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"],"agents":{"builder":["claude"]}}}`
	rc, out, _ := runRoutingVerb(t, "check", "--project-root", routingFixture(t, collapsed))
	if rc != exitRoutingFinding || !strings.Contains(out, "error cross_family_with.auditor+builder") || !strings.Contains(out, "REFUSED") {
		t.Fatalf("builder and auditor on one family is an error finding: rc=%d %q", rc, out)
	}
	if rc, out, _ := runRoutingVerb(t, "check", "--project-root", routingFixture(t, operatorTable)); rc != 0 || !strings.Contains(out, "warn cross_family_with.auditor+builder") {
		t.Fatalf("the operator's table with a builder that may use agy compiles with the accepted warning: rc=%d %q", rc, out)
	}
}

func TestCLIRoutingShow_GroupsAgentsByChainAndNotesTheFloorAndTheCeiling(t *testing.T) {
	table := `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"],"agents":{"builder":["agy","claude"]},"tiers":{"deep":["claude"],"top":["claude"]}}}`
	rc, out, errOut := runRoutingVerb(t, "show", "--static", "--project-root", routingFixture(t, table))
	if rc != 0 {
		t.Fatalf("show: rc=%d stderr=%q", rc, errOut)
	}
	for _, want := range []string{"declared cli_routing table", "chain agy-tmux → claude-tmux", "scout", "auditor", "FLOOR", "rule agents:builder", "ceiling: deep drops [agy-tmux]", "deep: claude-tmux"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output lacks %q:\n%s", want, out)
		}
	}
}

func TestCLIRoutingShow_JSONCarriesEveryAgentRoute(t *testing.T) {
	rc, out, _ := runRoutingVerb(t, "show", "--static", "--json", "--project-root", routingFixture(t, ""))
	var report routingReport
	if err := json.Unmarshal([]byte(out), &report); rc != 0 || err != nil || len(report.Agents) != 4 || report.Mode == "" {
		t.Fatalf("rc=%d err=%v report=%+v", rc, err, report)
	}
}

func TestCLIRoutingShow_ARefusedTableExitsOne(t *testing.T) {
	if rc, _, errOut := runRoutingVerb(t, "show", "--project-root", routingFixture(t, `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)); rc != exitRoutingFinding || !strings.Contains(errOut, "cli_routing.clis") {
		t.Fatalf("rc=%d stderr=%q", rc, errOut)
	}
}

func TestCLIRoutingExplain_NamesTheRuleChainAndAllowedSet(t *testing.T) {
	root := routingFixture(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"],"agents":{"builder":["agy"]}}}`)
	rc, out, _ := runRoutingVerb(t, "explain", "auditor", "--static", "--project-root", root)
	if rc != 0 || !strings.Contains(out, "rule:    default") || !strings.Contains(out, "allowed: claude") || !strings.Contains(out, "chain:   claude-tmux") {
		t.Fatalf("explain auditor: rc=%d\n%s", rc, out)
	}
	rc, out, _ = runRoutingVerb(t, "explain", "scout", "--static", "--project-root", root)
	if rc != 0 || !strings.Contains(out, "allowed: agy, claude") || !strings.Contains(out, "chain:   agy-tmux → claude-tmux") {
		t.Fatalf("explain scout: rc=%d\n%s", rc, out)
	}
	if rc, out, _ := runRoutingVerb(t, "explain", "scout", "--static", "--project-root", routingFixture(t, "")); rc != 0 || !strings.Contains(out, "every family") {
		t.Fatalf("a legacy profile with no allowed_clis allows every family: rc=%d\n%s", rc, out)
	}
	t.Setenv("EVOLVE_CLI", "codex-tmux")
	if rc, out, _ := runRoutingVerb(t, "explain", "scout", "--static", "--project-root", root); rc != exitRoutingFinding || !strings.Contains(out, "refused") {
		t.Fatalf("a refused route explains itself and exits 1: rc=%d %q", rc, out)
	}
}

func TestFindingNamesAgent_MatchesKeysNeverMessages(t *testing.T) {
	cases := map[string]bool{"agent.scout": true, "agent.scout.tier_ceiling": true, "profiles.scout": true, "cross_family_with.auditor+scout": true, "agent.scout-scan": false, "cli_routing.default": false}
	for key, want := range cases {
		if got := findingNamesAgent(key, "scout"); got != want {
			t.Errorf("findingNamesAgent(%q, scout) = %v, want %v", key, got, want)
		}
	}
}
