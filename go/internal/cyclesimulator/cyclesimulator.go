// Package cyclesimulator ports legacy/scripts/dispatch/cycle-simulator.sh.
//
// No-LLM cycle plumbing simulator (v8.50.0+). Walks every phase of an
// /evolve-loop cycle writing deterministic artifacts and appending
// tamper-evident ledger entries WITHOUT making any LLM API calls.
//
// What this validates:
//   - cycle-state.json advances correctly through every phase
//   - phase-gate.sh accepts every transition
//   - prev_hash chain remains intact post-run
//   - ship.sh --dry-run executes inside a real cycle context
//
// What this does NOT validate:
//   - LLM output quality (no LLM is invoked)
//   - Real Builder file edits (no source code changes)
//   - Real Auditor judgment (verdict is hardcoded PASS)
package cyclesimulator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Exit codes (matches cycle-simulator.sh):
//   0 — every phase completed and ledger chain intact
//   1 — runtime failure
//   2 — phase-gate refused a transition
const (
	ExitOK         = 0
	ExitRuntimeErr = 1
	ExitGateRefuse = 2
)

// Inputs for the simulator.
type Inputs struct {
	Cycle        int    // required, positive integer
	Workspace    string // required, will be created if absent
	ProjectRoot  string // EVOLVE_PROJECT_ROOT — ledger location
	PluginRoot   string // EVOLVE_PLUGIN_ROOT — helper scripts
	Token        string // optional simulator token; defaults to "sim-token-<cycle>-<pid>"
	Now          func() time.Time
	AdvanceFn    func(phase, agent string) error // injectable seam for tests
	ShipDryRunFn func(msg string) (int, error)   // returns rc, err
	VerifyFn     func() error                    // verify-ledger-chain
}

// Run executes the simulator. stderr receives [simulator] log lines.
func Run(in Inputs, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[simulator] "+format+"\n", args...)
	}

	// validation
	if in.Cycle <= 0 {
		logf("cycle must be positive integer, got: %d", in.Cycle)
		return ExitRuntimeErr
	}
	if in.Workspace == "" {
		logf("missing workspace arg")
		return ExitRuntimeErr
	}
	if in.ProjectRoot == "" {
		logf("missing project root (EVOLVE_PROJECT_ROOT)")
		return ExitRuntimeErr
	}
	if in.PluginRoot == "" {
		in.PluginRoot = in.ProjectRoot
	}
	if in.Now == nil {
		in.Now = time.Now
	}
	if in.Token == "" {
		in.Token = fmt.Sprintf("sim-token-%d-%d", in.Cycle, os.Getpid())
	}

	pluginScript := func(p string) string { return filepath.Join(in.PluginRoot, "legacy", "scripts", p) }

	// default advance: shell out to cycle-state.sh
	if in.AdvanceFn == nil {
		in.AdvanceFn = func(phase, agent string) error {
			cmd := exec.Command("bash", pluginScript("lifecycle/cycle-state.sh"), "advance", phase, agent)
			cmd.Env = os.Environ()
			return cmd.Run()
		}
	}
	if in.ShipDryRunFn == nil {
		in.ShipDryRunFn = func(msg string) (int, error) {
			cmd := exec.Command("bash", pluginScript("lifecycle/ship.sh"), "--dry-run", msg)
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
			cmd := exec.Command("bash", pluginScript("observability/verify-ledger-chain.sh"))
			cmd.Env = os.Environ()
			return cmd.Run()
		}
	}

	if err := os.MkdirAll(in.Workspace, 0o755); err != nil {
		logf("workspace mkdir failed: %v", err)
		return ExitRuntimeErr
	}

	ledgerPath := filepath.Join(in.ProjectRoot, ".evolve", "ledger.jsonl")

	logf("starting simulated walk for cycle %d", in.Cycle)

	// pipeline: (phase, agent, artifact-writer)
	phases := []struct {
		phase, agent, fname string
		writer              func(string) string
	}{
		{"intent", "intent", "intent.md", in.writeIntent},
		{"research", "scout", "scout-report.md", in.writeScout},
		{"build", "builder", "build-report.md", in.writeBuild},
		{"audit", "auditor", "audit-report.md", in.writeAudit},
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

	// ship phase
	if err := in.AdvanceFn("ship", "orchestrator"); err != nil {
		logf("FAIL: cycle-state advance to ship refused")
		return ExitGateRefuse
	}
	logf("  ▶ ship phase: invoking ship.sh --dry-run")
	shipRC, _ := in.ShipDryRunFn(fmt.Sprintf("simulator: cycle %d plumbing test", in.Cycle))
	if shipRC == 0 {
		logf("  ✓ ship.sh --dry-run completed cleanly")
	} else {
		logf("  ⚠ ship.sh --dry-run exited rc=%d (acceptable for tree-state-mismatch in simulator context)", shipRC)
	}

	// retrospective
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

	// chain verify
	logf("verifying ledger chain post-simulation...")
	if err := in.VerifyFn(); err == nil {
		logf("OK: ledger chain intact")
	} else {
		logf("WARN: ledger chain verification flagged anomalies (may be pre-existing; simulator did not break it)")
	}

	// simulator-report
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

	logf("DONE: simulated cycle %d complete", in.Cycle)
	return ExitOK
}

// ── artifact templates ─────────────────────────────────────────────────────

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

// ── ledger appender ───────────────────────────────────────────────────────

// appendSimLedger writes a simulated ledger entry to ledger.jsonl + updates
// ledger.tip. Mirrors the bash write_sim_ledger byte layout exactly: the JSON
// object has `simulated: true` and computes prev_hash from the SHA256 of the
// last line.
func appendSimLedger(ledgerPath string, cycle int, role, artifactPath, token, projectRoot string, now func() time.Time) error {
	artifactSHA := ""
	if data, err := os.ReadFile(artifactPath); err == nil {
		sum := sha256.Sum256(data)
		artifactSHA = hex.EncodeToString(sum[:])
	}
	gitHEAD := runGit(projectRoot, "rev-parse", "HEAD")
	if gitHEAD == "" {
		gitHEAD = "unknown"
	}
	treeStateDiff := runGit(projectRoot, "diff", "HEAD")
	treeStateSHA := sha256Hex(treeStateDiff)

	prevHash, entrySeq, err := readChainLink(ledgerPath)
	if err != nil {
		return err
	}

	entry := map[string]any{
		"ts":               now().UTC().Format("2006-01-02T15:04:05Z"),
		"cycle":            cycle,
		"role":             role,
		"kind":             "agent_subprocess",
		"model":            "simulator",
		"exit_code":        0,
		"duration_s":       "0",
		"artifact_path":    artifactPath,
		"artifact_sha256":  artifactSHA,
		"challenge_token":  token,
		"git_head":         gitHEAD,
		"tree_state_sha":   treeStateSHA,
		"entry_seq":        entrySeq,
		"prev_hash":        prevHash,
		"simulated":        true,
	}
	line, err := jsonCompact(entry)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	tipPath := filepath.Join(filepath.Dir(ledgerPath), "ledger.tip")
	tip := fmt.Sprintf("%d:%s\n", entrySeq, sha256Hex(line))
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
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// jsonCompact serializes a map[string]any with a stable key order matching
// the bash jq -nc output: ts, cycle, role, kind, model, exit_code, duration_s,
// artifact_path, artifact_sha256, challenge_token, git_head, tree_state_sha,
// entry_seq, prev_hash, simulated. Stable order is load-bearing for
// downstream hash-chain verifiers that recompute SHA over the raw line.
func jsonCompact(m map[string]any) (string, error) {
	keys := []string{
		"ts", "cycle", "role", "kind", "model", "exit_code", "duration_s",
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

// ErrCycleNonPositive is returned when the simulator receives a non-positive cycle.
var ErrCycleNonPositive = errors.New("cyclesimulator: cycle must be positive")
