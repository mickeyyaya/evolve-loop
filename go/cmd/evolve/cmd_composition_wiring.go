// Package main is the evolve command: the composition root and CLI of the loop.
// See docs/architecture/packages/cmd-evolve.md.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func compositionOptions() []core.Option {
	return []core.Option{
		core.WithCompositionVerdictWriter(writeCompositionVerdict),
		core.WithCompositionGateRunner(composedGatesTo(os.Stderr)),
		core.WithCompositionSnapshot(readCompositionSnapshot),
	}
}

func writeCompositionVerdict(ledgerPath string, in core.CompositionVerdictInput) error {
	return ledger.WriteCompositionVerdict(ledgerPath, ledger.CompositionVerdictInput{
		Cycle:          in.Cycle,
		Method:         in.Method,
		LaneAuditRef:   in.LaneAuditRef,
		PatchID:        in.PatchID,
		AuditedBase:    in.AuditedBase,
		GitHead:        in.GitHead,
		TreeStateSHA:   in.TreeStateSHA,
		AuditedTreeSHA: in.AuditedTreeSHA,
		GateResults:    in.GateResults,
		AuditedDiff:    in.AuditedDiff,
		ComposedDiff:   in.ComposedDiff,
	})
}

var composedGateTargets = map[string]string{
	"compile":  "build",
	"test":     "test",
	"acs":      "test-acs-durable",
	"apicover": "apicover-enforce",
}

var composedGateFlags = map[string][]string{
	"test":     {"-race"},
	"apicover": {"-tags", "integration"},
}

type composedGateScope struct {
	variable string
	packages func(scope changedpkgs.Scope, worktree string) ([]string, bool)
}

var composedGateScopes = map[string]composedGateScope{
	"test":     {variable: "FAST_PKGS", packages: scopedTestPackages},
	"apicover": {variable: "APICOVER_PKGS", packages: scopedEnforcedPackages},
}

func scopedTestPackages(scope changedpkgs.Scope, _ string) ([]string, bool) {
	return scope.Packages, true
}

func scopedEnforcedPackages(scope changedpkgs.Scope, worktree string) ([]string, bool) {
	raw, err := os.ReadFile(filepath.Join(worktree, "go", ".apicover-enforce"))
	if err != nil {
		return nil, false
	}
	var entries []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			entries = append(entries, line)
		}
	}
	return scope.Enforced(entries), true
}

func gateOverride(gate string, scope changedpkgs.Scope, worktree string) (override string, skip bool) {
	spec, scoped := composedGateScopes[gate]
	if !scoped || scope.Full {
		return "", false
	}
	pkgs, ok := spec.packages(scope, worktree)
	if !ok {
		return "", false
	}
	if len(pkgs) == 0 {
		return "", true
	}
	return spec.variable + "=" + strings.Join(pkgs, " "), false
}

func composedGatesTo(log io.Writer) func(ctx context.Context, worktree string) map[string]ciparity.GateOutcome {
	return func(ctx context.Context, worktree string) map[string]ciparity.GateOutcome {
		run := composedGateRun{log: log, worktree: worktree}
		scope := changedpkgs.GateScope(worktree, "main")
		results := make(map[string]ciparity.GateOutcome, len(ciparity.RequiredComposedGates))
		for _, gate := range ciparity.RequiredComposedGates {
			if target, ok := composedGateTargets[gate]; ok {
				results[gate] = run.scoped(ctx, gate, target, scope)
			}
		}
		recordScopedShadow(ctx, log, worktree, results)
		return results
	}
}

type composedGateRun struct {
	log      io.Writer
	worktree string
}

func (r composedGateRun) scoped(ctx context.Context, gate, target string, scope changedpkgs.Scope) ciparity.GateOutcome {
	override, skip := gateOverride(gate, scope, r.worktree)
	if skip {
		fmt.Fprintf(r.log, "[orchestrator] composed-tree gate %s skipped: no package of the change needs it\n", gate)
		return ciparity.GateOutcome{Status: "pass"}
	}
	return r.gate(ctx, gate, target, override)
}

func (r composedGateRun) gate(ctx context.Context, gate, target, override string) ciparity.GateOutcome {
	tail := newTailWriter(64 * 1024)
	args := []string{"-C", "go", target}
	if override != "" {
		args = append(args, override)
	}
	cmd := sysexec.Command(ctx, "make", args...)
	cmd.Dir = r.worktree
	cmd.Env = lanebudget.Env(ciparity.CIEnv(os.Environ()))
	cmd.Stdout, cmd.Stderr = tail, tail
	err := cmd.Run()
	if err == nil {
		return ciparity.GateOutcome{Status: "pass"}
	}
	fmt.Fprintf(r.log, "[orchestrator] composed-tree gate %s (make %s) failed in %s: %v; its output tail rides in the %s event\n", gate, target, r.worktree, err, core.CodeComposedGateDeclined)
	reds := baseattr.ParseGoTest(tail.String())
	reds.Flags = composedGateFlags[gate]
	return ciparity.GateOutcome{Status: "fail", Tail: tail.lastLines(20), Reds: reds}
}

type tailWriter struct {
	buf []byte
	max int
}

func newTailWriter(max int) *tailWriter { return &tailWriter{max: max} }

func (t *tailWriter) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailWriter) String() string { return string(t.buf) }

func (t *tailWriter) lastLines(n int) string {
	all := strings.Split(strings.TrimRight(string(t.buf), "\n"), "\n")
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return strings.Join(all, "\n")
}

func readCompositionSnapshot(ctx context.Context, worktree, runID string) (core.CompositionAuditSnapshot, error) {
	ledgerPath := filepath.Join(worktree, ".evolve", "ledger.jsonl")
	entry, err := latestAuditEntry(ledgerPath, runID)
	if err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	if err := requireReusableAudit(entry); err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	diff, err := gitDiffCapture(ctx, worktree, "main..."+entry.GitHEAD)
	if err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	patchID, err := ledger.PatchID(diff)
	if err != nil {
		return core.CompositionAuditSnapshot{}, err
	}
	return core.CompositionAuditSnapshot{
		LaneAuditRef: entry.ArtifactSHA256,
		AuditedBase:  entry.GitHEAD,
		Diff:         diff,
		PatchID:      patchID,
	}, nil
}

func latestAuditEntry(ledgerPath, runID string) (auditledger.Entry, error) {
	rows, err := auditledger.AuditorRows(ledgerPath)
	if err != nil {
		return auditledger.Entry{}, fmt.Errorf("composition snapshot: %w", err)
	}
	bound := make([]auditledger.Entry, 0, len(rows))
	for _, e := range rows {
		if e.GitHEAD != "" {
			bound = append(bound, e)
		}
	}
	entry, err := auditledger.BindRun(bound, runID)
	if err != nil {
		return auditledger.Entry{}, fmt.Errorf("composition snapshot: %w in %s (foreign-run entries refused)", err, ledgerPath)
	}
	return entry, nil
}

func requireReusableAudit(entry auditledger.Entry) error {
	if entry.ArtifactPath == "" {
		return fmt.Errorf("composition snapshot: bound auditor entry has no artifact_path — cannot confirm its verdict")
	}
	body, err := os.ReadFile(entry.ArtifactPath)
	if err != nil {
		return fmt.Errorf("composition snapshot: read audit artifact %s: %w", entry.ArtifactPath, err)
	}
	sentinel, ok := phasecontract.ParseVerdictSentinelFull(string(body))
	if !ok {
		return fmt.Errorf("composition snapshot: audit artifact %s declares no parseable verdict sentinel", entry.ArtifactPath)
	}
	if sentinel.Phase != string(core.PhaseAudit) {
		return fmt.Errorf("composition snapshot: audit artifact %s carries a %q-phase verdict sentinel, not audit",
			entry.ArtifactPath, sentinel.Phase)
	}
	if !verdictcache.Reusable(sentinel.Verdict) {
		return fmt.Errorf("composition snapshot: refusing to carry forward a %s audit (%s) — only PASS/WARN are reusable",
			sentinel.Verdict, entry.ArtifactPath)
	}
	return nil
}

func gitDiffCapture(ctx context.Context, worktree, spec string) ([]byte, error) {
	cmd := sysexec.Command(ctx, "git", "diff", spec)
	cmd.Dir = worktree
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("composition snapshot: git diff %s: %w", spec, err)
	}
	return out.Bytes(), nil
}
