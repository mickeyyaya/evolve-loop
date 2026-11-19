//go:build integration && darwin && !notmux

package bridge

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

var paneParityTargets = []parityTarget{
	{pkg: "./internal/swarm", named: []string{"TestExecTmuxKill_NeverKillsASessionWhoseNameExtendsTheTarget"}},
	{pkg: "./internal/bridge", tags: "integration", filtered: true,
		named: []string{"TestRealTmux_HappyPath", "TestRealTmux_AgentTmuxProbeCannotKillTheRunServer"}},
}

const (
	parityDoneMarker   = "PANE-PARITY-DONE"
	parityDispatchID   = "parity-dispatch"
	parityManifestKey  = "EVOLVE_PARITY_MANIFEST"
	paneParityDeadline = 10 * time.Minute
)

func TestPaneParity_RealTmuxPackagesPassInTheSandboxedPaneAsOnTheHost(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	probe := requireNativeSandbox(t)
	for _, target := range paneParityTargets {
		t.Run(target.pkg, func(t *testing.T) {
			host := runOnHost(t, target)
			pane := runInPane(t, probe, target, nil)
			if diffs := paneOnlyFailures(target.pkg, host.results, pane.report.results); len(diffs) > 0 {
				t.Fatalf("pane parity: these tests pass on the host and fail in the sandboxed pane:\n%s", strings.Join(diffs, "\n"))
			}
			if err := paneEnvContract(repoRootForParity(t), pane.env); err != nil {
				t.Fatalf("pane env contract: %v", err)
			}
		})
	}
}

func TestPaneParity_ReportsTheCycle1853TriggerWhenThePtyRulesAreRemoved(t *testing.T) {
	requireTmux(t)
	probe := requireNativeSandbox(t)
	swarm := paneParityTargets[0]
	host := runOnHost(t, swarm)
	pane := runInPane(t, probe, swarm, withoutOwnPtyRules)
	diffs := paneOnlyFailures(swarm.pkg, host.results, pane.report.results)
	want := "./internal/swarm (package): host=pass pane=fail"
	if !slices.Contains(diffs, want) {
		t.Fatalf("without the R1 pty rules the harness must report %q, got:\n%s", want, strings.Join(diffs, "\n"))
	}
}

func withoutOwnPtyRules(t *testing.T, profile string) {
	t.Helper()
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	lines := slices.DeleteFunc(strings.Split(string(raw), "\n"), func(l string) bool {
		return strings.Contains(l, "pseudo-tty") || strings.Contains(l, `"/dev/ptmx"`) || strings.Contains(l, "com.apple.sandbox.pty")
	})
	if err := os.WriteFile(profile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func paneOnlyFailures(pkg string, host, pane testResults) []string {
	var diffs []string
	for _, name := range slices.Sorted(maps.Keys(host)) {
		if host[name] == "pass" && pane[name] != "pass" {
			diffs = append(diffs, fmt.Sprintf("%s %s: host=pass pane=%s", pkg, name, orMissing(pane[name])))
		}
	}
	return diffs
}

func paneEnvContract(root string, env []string) error {
	for _, kv := range env {
		if strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TMUX_PANE=") {
			return fmt.Errorf("the pane still carries %s", kv)
		}
	}
	for _, want := range []string{"EVOLVE_PROJECT_ROOT=" + root, "EVOLVE_DISPATCH_ID=" + parityDispatchID, parityManifestKey + "=1"} {
		if !slices.Contains(env, want) {
			return fmt.Errorf("the pane misses the boot export %s", want)
		}
	}
	return nil
}

func runOnHost(t *testing.T, target parityTarget) goReport {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", target.goTestArgs()...)
	cmd.Dir = filepath.Join(repoRootForParity(t), "go")
	cmd.Env = gotooltest.Env(paneEnv(os.Environ()))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	rc := 0
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		rc = exit.ExitCode()
	case err != nil:
		t.Fatalf("start the host run of %s: %v", target.pkg, err)
	}
	report := newGoReport(string(out), rc)
	if err := report.validate(); err != nil {
		t.Fatalf("the host run of %s is not evidence: %v\nstderr:\n%s", target.pkg, err, stderr.String())
	}
	if err := target.hostEvidence(report); err != nil {
		t.Fatalf("the host run of %s proves nothing: %v", target.pkg, err)
	}
	return report
}

type paneRun struct {
	report goReport
	env    []string
}

func runInPane(t *testing.T, probe sandbox.ProbeResult, target parityTarget, mutate func(*testing.T, string)) paneRun {
	t.Helper()
	cfg := itConfig(t, "x")
	cfg.ProjectRoot, cfg.RequireSandbox, cfg.DispatchID = repoRootForParity(t), true, parityDispatchID
	cfg.Realization.Env = map[string]string{parityManifestKey: "1"}
	file := func(name string) string { return filepath.Join(cfg.Workspace, name) }
	mustWrite(t, file("parity.sh"), fmt.Sprintf("env > %s\ngo %s > %s 2> %s\necho $? > %s\nprintf '%%s\\n' %s\nexec sleep 600\n",
		shellQuotePOSIX(file("pane.env")), joinPrefixForTmux(target.goTestArgs()), shellQuotePOSIX(file("pane.json")),
		shellQuotePOSIX(file("pane.stderr")), shellQuotePOSIX(file("pane.rc")), parityDoneMarker))
	deps := itDeps(250 * time.Millisecond)
	deps.Env, deps.LookupEnv, deps.BootTimeoutS = map[string]string{}, mapLookup(nil), 2400
	wrap := defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
	deps.SandboxWrap = func(req SandboxWrapRequest) ([]string, WrapOutcome) {
		prefix, ok := wrap(req)
		if ok.Applied && mutate != nil {
			mutate(t, prefix[2])
		}
		return prefix, ok
	}
	session := itSession("parity")
	t.Cleanup(func() { _ = itTmuxCtl.KillSession(context.Background(), session) })
	_, code, err := bootTmuxREPL(itBoundedContext(t, paneParityDeadline), cfg, deps, itLaunch(session, "/bin/sh "+shellQuotePOSIX(file("parity.sh")), parityDoneMarker, 0, false),
		replPreparation{prefix: "[parity]", workingDir: filepath.Join(cfg.ProjectRoot, "go")}, nil)
	if err != nil || code != ExitOK {
		t.Fatalf("the pane run of %s did not finish: code=%d err=%v", target.pkg, code, err)
	}
	return paneRun{report: paneReport(t, target, file), env: strings.Split(readFile(t, file("pane.env")), "\n")}
}

func paneReport(t *testing.T, target parityTarget, file func(string) string) goReport {
	t.Helper()
	rc, err := strconv.Atoi(strings.TrimSpace(readFile(t, file("pane.rc"))))
	if err != nil {
		t.Fatalf("the pane run of %s wrote no exit status: %v", target.pkg, err)
	}
	report := newGoReport(readFile(t, file("pane.json")), rc)
	if err := report.validate(); err != nil {
		t.Fatalf("the pane run of %s is not evidence: %v\nstderr:\n%s", target.pkg, err, readFile(t, file("pane.stderr")))
	}
	return report
}

func repoRootForParity(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("pane parity: read the package directory: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
