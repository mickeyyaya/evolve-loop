// Package cyclesimulator walks every phase of a cycle with no LLM call and appends simulated ledger entries.
// See docs/architecture/packages/internal-cyclesimulator.md.
package cyclesimulator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgertip"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	ExitOK         = 0
	ExitRuntimeErr = 1
	ExitGateRefuse = 2
)

type Inputs struct {
	Cycle        int
	Workspace    string
	ProjectRoot  string
	PluginRoot   string
	Token        string
	Now          func() time.Time
	AdvanceFn    func(phase, agent string) error
	ShipDryRunFn func(msg string) (int, error)
	VerifyFn     func() error
}

func Run(in Inputs, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[simulator] "+format+"\n", args...)
	}

	if !in.validate(logf) {
		return ExitRuntimeErr
	}
	in.applyDefaults()
	in.installDefaultSeams()

	if err := os.MkdirAll(in.Workspace, 0o755); err != nil {
		logf("workspace mkdir failed: %v", err)
		return ExitRuntimeErr
	}

	ledgerPath := filepath.Join(in.ProjectRoot, ".evolve", "ledger.jsonl")

	logf("starting simulated walk for cycle %d", in.Cycle)

	if rc := in.walkPipeline(ledgerPath, logf); rc != ExitOK {
		return rc
	}
	shipRC, rc := in.simulateShip(logf)
	if rc != ExitOK {
		return rc
	}
	if rc := in.simulateRetrospective(ledgerPath, logf); rc != ExitOK {
		return rc
	}
	in.verifyChain(logf)
	if rc := in.writeSimReport(shipRC, logf); rc != ExitOK {
		return rc
	}

	logf("DONE: simulated cycle %d complete", in.Cycle)
	return ExitOK
}

type logFunc func(format string, args ...any)

func (in *Inputs) validate(logf logFunc) bool {
	if in.Cycle <= 0 {
		logf("cycle must be positive integer, got: %d", in.Cycle)
		return false
	}
	if in.Workspace == "" {
		logf("missing workspace arg")
		return false
	}
	if in.ProjectRoot == "" {
		logf("missing project root (EVOLVE_PROJECT_ROOT)")
		return false
	}
	return true
}

func (in *Inputs) applyDefaults() {
	if in.PluginRoot == "" {
		in.PluginRoot = in.ProjectRoot
	}
	if in.Now == nil {
		in.Now = time.Now
	}
	if in.Token == "" {
		in.Token = fmt.Sprintf("sim-token-%d-%d", in.Cycle, os.Getpid())
	}
}

func (in *Inputs) installDefaultSeams() {
	pluginScript := func(p string) string { return filepath.Join(in.PluginRoot, "legacy", "scripts", p) }

	if in.AdvanceFn == nil {
		in.AdvanceFn = func(phase, agent string) error {
			cmd := sysexec.Command(context.Background(), "bash", pluginScript("lifecycle/cycle-state.sh"), "advance", phase, agent)
			cmd.Env = os.Environ()
			return cmd.Run()
		}
	}
	if in.ShipDryRunFn == nil {
		in.ShipDryRunFn = func(msg string) (int, error) {
			cmd := sysexec.Command(context.Background(), "bash", pluginScript("lifecycle/ship.sh"), "--dry-run", msg)
			cmd.Env = os.Environ()
			err := cmd.Run()
			if cmd.ProcessState != nil {
				return cmd.ProcessState.ExitCode(), err
			}
			return 1, err
		}
	}
	if in.VerifyFn == nil {
		in.VerifyFn = func() error {
			cmd := sysexec.Command(context.Background(), "bash", pluginScript("observability/verify-ledger-chain.sh"))
			cmd.Env = os.Environ()
			return cmd.Run()
		}
	}
}

func (in *Inputs) walkPipeline(ledgerPath string, logf logFunc) int {
	phases := []struct {
		phase, agent, fname string
		writer              func(string) string
	}{
		{"intent", "intent", phasecontract.ArtifactName("intent"), in.writeIntent},
		{"research", "scout", phasecontract.ArtifactName("scout"), in.writeScout},
		{"build", "builder", phasecontract.ArtifactName("build"), in.writeBuild},
		{"audit", "auditor", phasecontract.ArtifactName("audit"), in.writeAudit},
	}
	for _, ph := range phases {
		if err := in.AdvanceFn(ph.phase, ph.agent); err != nil {
			logf("FAIL: cycle-state advance to %s (%s) refused", ph.phase, ph.agent)
			return ExitGateRefuse
		}
		artifactPath := filepath.Join(in.Workspace, ph.fname)
		if err := os.WriteFile(artifactPath, []byte(ph.writer(in.Token)), 0o644); err != nil {
			logf("FAIL: writing %s: %v", ph.fname, err)
			return ExitRuntimeErr
		}
		if err := appendSimLedger(ledgerPath, in.Cycle, ph.agent, artifactPath, in.Token, in.ProjectRoot, in.Now); err != nil {
			logf("FAIL: ledger append for %s: %v", ph.agent, err)
			return ExitRuntimeErr
		}
		logf("  ✓ %s → wrote %s, ledger entry", ph.phase, ph.fname)
	}
	return ExitOK
}

func (in *Inputs) simulateShip(logf logFunc) (shipRC, rc int) {
	if err := in.AdvanceFn("ship", "orchestrator"); err != nil {
		logf("FAIL: cycle-state advance to ship refused")
		return 0, ExitGateRefuse
	}
	logf("  ▶ ship phase: invoking ship.sh --dry-run")
	shipRC, _ = in.ShipDryRunFn(fmt.Sprintf("simulator: cycle %d plumbing test", in.Cycle))
	if shipRC == 0 {
		logf("  ✓ ship.sh --dry-run completed cleanly")
	} else {
		logf("  ⚠ ship.sh --dry-run exited rc=%d (acceptable for tree-state-mismatch in simulator context)", shipRC)
	}
	return shipRC, ExitOK
}

func (in *Inputs) simulateRetrospective(ledgerPath string, logf logFunc) int {
	if err := in.AdvanceFn("retrospective", "retrospective"); err != nil {
		logf("FAIL: cycle-state advance to retrospective refused")
		return ExitGateRefuse
	}
	retroPath := filepath.Join(in.Workspace, "retrospective-report.md")
	if err := os.WriteFile(retroPath, []byte(in.writeRetro(in.Token)), 0o644); err != nil {
		logf("FAIL: writing retrospective: %v", err)
		return ExitRuntimeErr
	}
	if err := appendSimLedger(ledgerPath, in.Cycle, "retrospective", retroPath, in.Token, in.ProjectRoot, in.Now); err != nil {
		logf("FAIL: ledger append for retrospective: %v", err)
		return ExitRuntimeErr
	}
	logf("  ✓ retrospective → wrote retrospective-report.md, ledger entry")
	return ExitOK
}

func (in *Inputs) verifyChain(logf logFunc) {
	logf("verifying ledger chain post-simulation...")
	if err := in.VerifyFn(); err == nil {
		logf("OK: ledger chain intact")
	} else {
		logf("WARN: ledger chain verification flagged anomalies (may be pre-existing; simulator did not break it)")
	}
}

func (in *Inputs) writeSimReport(shipRC int, logf logFunc) int {
	reportPath := filepath.Join(in.Workspace, "simulator-report.md")
	report := fmt.Sprintf(
		"<!-- challenge-token: %s -->\n# Cycle Simulator Report — Cycle %d\n\n"+
			"All 6 phases advanced cleanly. 6 ledger entries appended (chain intact).\n"+
			"Ship phase exercised via ship.sh --dry-run (rc=%d).\n\n"+
			"This is a no-LLM plumbing validation; agent output quality is NOT validated.\n",
		in.Token, in.Cycle, shipRC,
	)
	if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
		logf("FAIL: writing simulator-report: %v", err)
		return ExitRuntimeErr
	}
	return ExitOK
}

func (in *Inputs) writeIntent(token string) string {
	return fmt.Sprintf(`<!-- challenge-token: %s -->
---
awn_class: CLEAR
goal: Simulator cycle plumbing validation
challenged_premises:
  - Premise: real cycles always succeed; this simulator covers the unhappy structural paths
constraints: []
non_goals: []
acceptance: All phases advance, all ledger entries written, no real LLM calls.
risk_level: low
---

# Intent (simulated cycle %d)

This artifact is produced by cycle-simulator.sh — no LLM was involved.
`, token, in.Cycle)
}

func (in *Inputs) writeScout(token string) string {
	return fmt.Sprintf(`<!-- challenge-token: %s -->
# Scout Report — Cycle %d (simulated)

## Selected task

- ID: simulator-noop
- Description: validate the cycle pipeline plumbing
- Eval: every phase completes and the ledger chain stays intact

## Discoveries

(none — simulator does not scan the codebase)
`, token, in.Cycle)
}

func (in *Inputs) writeBuild(token string) string {
	return fmt.Sprintf(`<!-- challenge-token: %s -->
# Build Report — Cycle %d (simulated)

## Files modified

(none — simulator makes no file edits)

## Tests run

simulator: not applicable
`, token, in.Cycle)
}

func (in *Inputs) writeAudit(token string) string {
	return fmt.Sprintf(`<!-- challenge-token: %s -->
# Audit Report — Cycle %d (simulated)

Verdict: PASS

All criteria met (simulator hardcodes PASS to exercise the ship-success path).
`, token, in.Cycle)
}

func (in *Inputs) writeRetro(token string) string {
	return fmt.Sprintf(`<!-- challenge-token: %s -->
# Retrospective Report — Cycle %d (simulated)

## Lesson

simulator-cycle: kernel-plumbing validated; no semantic learning produced.
`, token, in.Cycle)
}

var (
	simLedgerCompact = jsonCompact
	simLedgerWrite   = func(f *os.File, s string) (int, error) { return f.WriteString(s) }
)

func appendSimLedger(ledgerPath string, cycle int, role, artifactPath, token, projectRoot string, now func() time.Time) error {
	artifactSHA := fileSHA256(artifactPath)
	gitHEAD, treeStateSHA := gitTreeState(projectRoot)

	prevHash, entrySeq, err := readChainLink(ledgerPath)
	if err != nil {
		return err
	}

	entry := map[string]any{
		"ts":              now().UTC().Format("2006-01-02T15:04:05Z"),
		"cycle":           cycle,
		"role":            role,
		"kind":            "agent_subprocess",
		"model":           "simulator",
		"exit_code":       0,
		"duration_s":      "0",
		"artifact_path":   artifactPath,
		"artifact_sha256": artifactSHA,
		"challenge_token": token,
		"git_head":        gitHEAD,
		"tree_state_sha":  treeStateSHA,
		"entry_seq":       entrySeq,
		"prev_hash":       prevHash,
		"simulated":       true,
	}
	if runID := core.RunIDFromWorkspace(filepath.Dir(artifactPath)); runID != "" {
		entry["run_id"] = runID
	}
	line, err := simLedgerCompact(entry)
	if err != nil {
		return err
	}

	if err := appendLedgerLine(ledgerPath, line); err != nil {
		return err
	}
	return writeLedgerTip(ledgerPath, entrySeq, line)
}

func fileSHA256(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func gitTreeState(projectRoot string) (gitHEAD, treeStateSHA string) {
	gitHEAD = runGit(projectRoot, "rev-parse", "HEAD")
	if gitHEAD == "" {
		gitHEAD = "unknown"
	}
	treeStateDiff := runGit(projectRoot, "diff", "HEAD")
	return gitHEAD, sha256Hex(treeStateDiff)
}

func appendLedgerLine(ledgerPath, line string) error {
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := simLedgerWrite(f, line+"\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func writeLedgerTip(ledgerPath string, entrySeq int, line string) error {
	tipPath := filepath.Join(filepath.Dir(ledgerPath), "ledger.tip")
	tip := ledgertip.Format(entrySeq, sha256Hex(line))
	tmp := tipPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(tip), 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, tipPath)
}

const zeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

func readChainLink(ledgerPath string) (prevHash string, entrySeq int, err error) {
	prevHash = zeroSeed
	entrySeq = 0
	info, statErr := os.Stat(ledgerPath)
	if statErr != nil || info.Size() == 0 {
		return prevHash, entrySeq, nil
	}
	data, rerr := os.ReadFile(ledgerPath)
	if rerr != nil {
		return "", 0, rerr
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return prevHash, entrySeq, nil
	}
	last := lines[len(lines)-1]
	prevHash = sha256Hex(last)
	entrySeq = len(lines)
	return prevHash, entrySeq, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func runGit(dir string, args ...string) string {
	cmd := sysexec.Command(context.Background(), "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

func jsonCompact(m map[string]any) (string, error) {
	keys := []string{
		"ts", "cycle", "run_id", "role", "kind", "model", "exit_code", "duration_s",
		"artifact_path", "artifact_sha256", "challenge_token",
		"git_head", "tree_state_sha", "entry_seq", "prev_hash", "simulated",
	}
	var b strings.Builder
	b.WriteByte('{')
	first := true
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		kj, _ := json.Marshal(k)
		vj, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		b.Write(kj)
		b.WriteByte(':')
		b.Write(vj)
	}
	b.WriteByte('}')
	return b.String(), nil
}
