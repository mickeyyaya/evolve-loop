//go:build acs

package cycle15

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC15_001_AllResumeFlagsAbsentFromRegistry(t *testing.T) {
	allFlags := []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS",
		"EVOLVE_RESUME",
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",
		"EVOLVE_RESUME_COMPLETED_PHASES",
		"EVOLVE_RESUME_MODE",
		"EVOLVE_RESUME_PHASE",
	}
	for _, name := range allFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-15 RESUME_* consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC15_004_NoResumeEnvReadsInProductionFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	resumeFile := filepath.Join(root, "go", "internal", "core", "resume.go")
	cmdLoopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	for _, flag := range []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS",
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",
		"EVOLVE_RESUME_COMPLETED_PHASES",
		"EVOLVE_RESUME_PHASE",
	} {
		envRead := `os.Getenv("` + flag + `")`
		if !acsassert.FileNotContains(t, resumeFile, envRead) {
			t.Errorf("resume.go reads %q via os.Getenv — must be absent.\n"+
				"File: %s", flag, resumeFile)
		}
		if !acsassert.FileNotContains(t, cmdLoopArgsFile, envRead) {
			t.Errorf("cmd_loop_args.go reads %q via os.Getenv — must be absent.\n"+
				"File: %s", flag, cmdLoopArgsFile)
		}
	}
}

func TestC15_005_ControlFlagsMdHasNoResumeRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	resumeFlags := []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS",
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",
		"EVOLVE_RESUME_COMPLETED_PHASES",
		"EVOLVE_RESUME_MODE",
		"EVOLVE_RESUME_PHASE",
		"`EVOLVE_RESUME`",
	}
	for _, flag := range resumeFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 6 RESUME_* rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

func TestC15_006_IPCSetPreservedInCmdLoopArgs(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdLoopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileContains(t, cmdLoopArgsFile, `"EVOLVE_RESUME"`) {
		t.Errorf("RED: cmd_loop_args.go no longer sets EVOLVE_RESUME — IPC handoff broken.\n"+
			"Builder must NOT remove the EVOLVE_RESUME= assignment in cmd_loop_args.go:273;\n"+
			"only the registry row in registry_table.go should be removed.\n"+
			"File: %s", cmdLoopArgsFile)
	}
}

func TestC15_007_BypassPolicyFlagInCycleRunHelp(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	cmd := exec.Command("go", "run", "./cmd/evolve", "cycle", "run", "--help")
	cmd.Dir = goDir
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "--bypass-policy") {
		t.Errorf("RED: 'evolve cycle run --help' does not show '--bypass-policy'.\n"+
			"Builder must add: fs.BoolVar(&bypassPolicy, \"bypass-policy\", false, ...) in runCycleRun.\n"+
			"Usage output:\n%s", out)
	}
}

func TestC15_008_BypassPolicyInLoopDryRunJSON(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	cmd := exec.Command("go", "run", "./cmd/evolve", "loop", "--dry-run", "--bypass-policy")
	cmd.Dir = goDir
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		t.Errorf("RED: 'evolve loop --dry-run --bypass-policy' failed — --bypass-policy not yet registered in parseLoopArgs.\n"+
			"Builder must add BoolVar for 'bypass-policy' in parseLoopArgs and BypassPolicy bool to loopConfig.\n"+
			"Error: %v\nStderr:\n%s", err, stderrBuf.String())
		return
	}
	combined := stdoutBuf.String() + stderrBuf.String()
	if !strings.Contains(combined, `"bypass_policy"`) {
		t.Errorf("RED: 'evolve loop --dry-run --bypass-policy' output missing 'bypass_policy' key.\n"+
			"Builder must add BypassPolicy bool with json:\"bypass_policy,omitempty\" tag to loopConfig.\n"+
			"Got output:\n%s", combined)
	}
}

func TestC15_009_PolicyBypassEnvBridgesAbsentFromCmdFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	files := []struct {
		name string
		path string
	}{
		{"cmd_cycle.go", filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")},
		{"cmd_loop.go", filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")},
	}
	for _, f := range files {
		if !acsassert.FileNotContains(t, f.path, `cycleEnv["EVOLVE_POLICY_BYPASS"]`) {
			t.Errorf("RED: %s still contains cycleEnv[\"EVOLVE_POLICY_BYPASS\"] bridge read.\n"+
				"Builder must replace the bridge read with the --bypass-policy CLI flag value.\n"+
				"File: %s", f.name, f.path)
		}
	}
}

func TestC15_010_PolicyBypassRowAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_POLICY_BYPASS"); ok {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_POLICY_BYPASS\") returned (flag, true) — row still registered.\n"+
			"Builder must delete the EVOLVE_POLICY_BYPASS row from registry_table.go (cycle-15 bypass-policy-flag).\n"+
			"Current entry: Status=%q Cluster=%q",
			f.Status, f.Cluster)
	}
}
