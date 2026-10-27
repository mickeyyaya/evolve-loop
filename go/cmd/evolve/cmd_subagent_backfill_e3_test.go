package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseValidateProfileFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantLog  string
		wantRest []string
		wantOK   bool
		wantErr  string
	}{
		{"no flags", []string{"scout"}, "", []string{"scout"}, true, ""},
		{"empty", nil, "", nil, true, ""},
		{"spaced value", []string{"--dispatch-plan-log", "/p.json", "scout"}, "/p.json", []string{"scout"}, true, ""},
		{"inline value", []string{"--dispatch-plan-log=/q.json", "scout"}, "/q.json", []string{"scout"}, true, ""},
		{"inline empty value", []string{"--dispatch-plan-log=", "scout"}, "", []string{"scout"}, true, ""},
		{"last flag wins", []string{"--dispatch-plan-log", "/a", "--dispatch-plan-log=/b", "x"}, "/b", []string{"x"}, true, ""},
		{"flags stop at the first operand", []string{"scout", "--dispatch-plan-log", "/p"}, "", []string{"scout", "--dispatch-plan-log", "/p"}, true, ""},
		{"flag without a value is unknown", []string{"--dispatch-plan-log"}, "", nil, false, "evolve subagent validate-profile: unknown flag: --dispatch-plan-log\n"},
		{"unknown flag", []string{"--verbose", "scout"}, "", nil, false, "evolve subagent validate-profile: unknown flag: --verbose\n"},
		{"single dash is an operand", []string{"-v"}, "", []string{"-v"}, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errb bytes.Buffer

			log, rest, ok := parseValidateProfileFlags(tc.args, &errb)

			if log != tc.wantLog || !reflect.DeepEqual(rest, tc.wantRest) || ok != tc.wantOK || errb.String() != tc.wantErr {
				t.Errorf("got %q %q %v stderr %q, want %q %q %v %q", log, rest, ok, errb.String(), tc.wantLog, tc.wantRest, tc.wantOK, tc.wantErr)
			}
		})
	}
}

func TestRunSubagentValidateProfile_AnUnknownFlagExitsTwo(t *testing.T) {
	t.Parallel()
	_, errOut, rc := runCLI("validate-profile", "--bogus", "scout")
	if rc != 2 || errOut != "evolve subagent validate-profile: unknown flag: --bogus\n" {
		t.Fatalf("rc=%d stderr=%q", rc, errOut)
	}
}

func e3SubagentProject(t *testing.T, profile string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if profile != "" {
		writeFileForPublishTest(t, filepath.Join(root, ".evolve", "profiles", "scout.json"), profile)
	}
	for rel, body := range extra {
		writeFileForPublishTest(t, filepath.Join(root, rel), body)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("EVOLVE_PLUGIN_ROOT", root)
	t.Setenv("WORKTREE_PATH", "")
	return root
}

const e3ScoutProfile = `{"name":"scout","cli":"codex-tmux","model_tier_default":"deep","output_artifact":"e3-out.md"}`

const e3RefusedTable = `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`

func TestRunSubagentValidateProfile_ValidProfileReportsTheResolutionAndCapabilityWarnings(t *testing.T) {
	root := e3SubagentProject(t, e3ScoutProfile, map[string]string{
		"adapters/codex-tmux.capabilities.json": `{"supports":{"budget_cap_native":false,"permission_scoping":false}}`,
	})
	planLog := filepath.Join(root, "plan.json")

	_, errOut, rc := runCLI("validate-profile", "--dispatch-plan-log", planLog, "scout")

	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, errOut)
	}
	want := "[dispatch-resolve] cli=codex-tmux source=profile model=deep\n" +
		"[subagent-run] cli_resolution: source=profile target_cli=codex-tmux\n" +
		"[adapter-cap] WARN cli=codex-tmux missing=budget_cap_native substitute=wall_clock_timeout\n" +
		"[adapter-cap] WARN cli=codex-tmux missing=permission_scoping substitute=kernel_role_gate_only\n" +
		"[subagent-run] profile valid: scout\n"
	if !strings.HasSuffix(errOut, want) {
		t.Errorf("stderr =\n%s\nwant suffix\n%s", errOut, want)
	}
	plan, err := os.ReadFile(planLog)
	if err != nil || !strings.Contains(string(plan), `"cli":"codex-tmux"`) || !strings.Contains(string(plan), `"cap_budget_native":false`) {
		t.Errorf("dispatch plan log = %s (err %v)", plan, err)
	}
}

func TestRunSubagentValidateProfile_WithoutWarningsPrintsNoCapabilityLine(t *testing.T) {
	e3SubagentProject(t, e3ScoutProfile, nil)

	_, errOut, rc := runCLI("validate-profile", "scout")

	if rc != 0 || strings.Contains(errOut, "[adapter-cap]") || !strings.HasSuffix(errOut, "[subagent-run] profile valid: scout\n") {
		t.Fatalf("rc=%d stderr=%q", rc, errOut)
	}
}

func TestRunSubagent_ARefusedRoutingTableExitsThreeBeforeDispatch(t *testing.T) {
	root := e3SubagentProject(t, e3ScoutProfile, map[string]string{".evolve/policy.json": e3RefusedTable})
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(root, "prompt.txt")
	writeFileForPublishTest(t, prompt, "hi\n")
	t.Setenv("PROMPT_FILE_OVERRIDE", prompt)
	cases := []struct {
		name string
		args []string
		verb string
	}{
		{"validate-profile", []string{"validate-profile", "scout"}, "evolve subagent validate-profile: "},
		{"run", []string{"run", "scout", "7", ws}, "evolve subagent run: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errOut, rc := runCLI(tc.args...)
			if rc != exitRoutingRefused || !strings.Contains(errOut, tc.verb) {
				t.Fatalf("rc=%d stderr=%q", rc, errOut)
			}
			if strings.Contains(errOut, "profile valid") || strings.Contains(errOut, "verdict=") {
				t.Errorf("a refused table still dispatched: %q", errOut)
			}
		})
	}
}

func TestPrintSubagentRunHelp(t *testing.T) {
	t.Parallel()
	for _, h := range []string{"-h", "--help"} {
		out, errOut, rc := runCLI("run", h)
		if rc != 0 || errOut != "" {
			t.Fatalf("%s: rc=%d stderr=%q", h, rc, errOut)
		}
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 6 || lines[0] != "Usage: evolve subagent run <agent> <cycle> <workspace_path>" || !strings.Contains(out, "PROMPT_FILE_OVERRIDE") || !strings.Contains(out, "LEGACY_AGENT_DISPATCH is retired") {
			t.Errorf("%s: help = %q", h, out)
		}
	}
}

func TestRunSubagentRun_RefusesBadOperands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"none", nil, "evolve subagent run: expected <agent> <cycle> <workspace>\n"},
		{"two", []string{"scout", "7"}, "evolve subagent run: expected <agent> <cycle> <workspace>\n"},
		{"four", []string{"scout", "7", "/ws", "x"}, "evolve subagent run: expected <agent> <cycle> <workspace>\n"},
		{"cycle not a number", []string{"scout", "seven", "/ws"}, "evolve subagent run: cycle must be integer: "},
		{"cycle with a space", []string{"scout", " 7", "/ws"}, "evolve subagent run: cycle must be integer: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if rc := runSubagentRun(tc.args, &out, &errb); rc != 2 || out.Len() != 0 || !strings.HasPrefix(errb.String(), tc.want) {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want prefix %q", rc, out.String(), errb.String(), tc.want)
			}
		})
	}
}
