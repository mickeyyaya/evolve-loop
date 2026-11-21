package bridge

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var cellBinaries = gotooltest.NewCache(gotooltest.ExecRunner)

func init() { AddTestMainCleanup(func() { _ = cellBinaries.Close() }) }

var (
	cellSelfCheck    = regexp.MustCompile(`<self-check>(.+?)</self-check>`)
	cellCapabilities = regexp.MustCompile(`Run (evolve [^.]+?) first`)
)

func correctionCommandsForCell(t *testing.T, workspace string) []string {
	t.Helper()
	triage, ok := phasecontract.For("triage")
	if !ok {
		t.Fatal("no triage contract")
	}
	tail := phasecontract.RenderContractTail(triage, filepath.Join(workspace, "triage-report.md"), workspace)
	var commands []string
	for _, src := range []struct {
		re   *regexp.Regexp
		text string
	}{{cellSelfCheck, tail}, {cellCapabilities, agentclass.Brief(workspace)}} {
		m := src.re.FindStringSubmatch(src.text)
		if m == nil {
			t.Fatalf("no command found by %s", src.re)
		}
		commands = append(commands, strings.ReplaceAll(m[1], "<your workspace dir>", workspace))
	}
	return commands
}

func runInCell(t *testing.T, envFile, command string) (string, int) {
	t.Helper()
	cmd := exec.Command("/usr/bin/env", "-i", "/bin/sh", envFile)
	cmd.Stdin = strings.NewReader(command + "\n")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return out.String(), ee.ExitCode()
	}
	if err != nil {
		t.Fatalf("run %q: %v", command, err)
	}
	return out.String(), 0
}

func TestCUJ_EachCorrectionCommandRunsInCell(t *testing.T) {
	moduleDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binary, err := cellBinaries.Build(gotooltest.Spec{Dir: moduleDir, Pkg: "./cmd/evolve", Name: "evolve"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	for _, stage := range []string{"enforce", "shadow", "off"} {
		t.Run(stage, func(t *testing.T) {
			ws := t.TempDir()
			deps := Deps{EnvStage: stage, Stderr: new(bytes.Buffer), HostGitConfig: fakeHostGit(nil), CellBinRoot: fixedRoot(filepath.Join(t.TempDir(), "cell-bin")), ExecutablePath: func() (string, error) { return binary, nil }}
			env, _, err := deps.cellEnv(&Config{CLI: "claude-tmux", Agent: "triage", Workspace: ws})
			if err != nil {
				t.Fatal(err)
			}
			wantEntry := filepath.Join(firstPathEntry(env), "evolve")
			for _, command := range correctionCommandsForCell(t, ws) {
				envFile, err := writeCellEnvFile(env)
				if err != nil {
					t.Fatal(err)
				}
				out, code := runInCell(t, envFile, "command -v evolve; "+command)
				if code == 127 || strings.Contains(out, "command not found") {
					t.Errorf("%q in the %s cell: exit %d, output %q", command, stage, code, out)
				}
				if first, _, _ := strings.Cut(out, "\n"); first != wantEntry {
					t.Errorf("%q in the %s cell: command -v evolve = %q, want %q", command, stage, first, wantEntry)
				}
				if strings.TrimSpace(strings.TrimPrefix(out, wantEntry)) == "" {
					t.Errorf("%q in the %s cell printed no verifier output", command, stage)
				}
			}
		})
	}
}
