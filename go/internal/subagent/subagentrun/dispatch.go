package subagentrun

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

func (d *Dispatcher) Dispatch(ctx context.Context, req Request) (Outcome, error) {
	id, err := d.admit(req)
	if err != nil {
		return Outcome{}, err
	}
	p, err := d.resolve(req, id)
	if err != nil {
		return Outcome{}, err
	}
	prov, err := d.prepare(ctx, req, id, p)
	if err != nil {
		return Outcome{}, err
	}
	worktree, warns := d.worktreeFor(req, id, p.cap.Warns)
	promptPath, cleanup, err := d.stage(req, id, prov)
	if err != nil {
		return Outcome{}, err
	}
	defer cleanup()
	x := d.execute(ctx, adapterEnv(req, p, prov, promptPath, worktree))
	return d.record(req, id, p, prov, x, warns)
}

func (d *Dispatcher) worktreeFor(req Request, id identity, capWarns []string) (string, []string) {
	warns := append([]string(nil), capWarns...)
	worktree := req.WorktreePath
	if worktree == "" {
		worktree = req.ProjectRoot
		sentence := fmt.Sprintf("WorktreePath not propagated — WORKTREE_PATH falls back to the project root %s; this agent will run against the main tree", worktree)
		warns = append(warns, fmt.Sprintf("[subagent-run] WARN agent=%s cycle=%d: %s", req.Agent, req.Cycle, sentence))
		d.warn(id, req.Cycle, CodeWorktreeFallback, sentence, map[string]string{"step": "env", "worktree": worktree, "project_root": req.ProjectRoot})
	}
	return worktree, warns
}

func (d *Dispatcher) stage(req Request, id identity, prov provenance) (string, func(), error) {
	path, cleanup, err := d.stager.Stage(prov.prompt)
	if err != nil {
		fields := map[string]string{"step": "stage", "artifact": prov.artifactPath}
		var fault *stageFault
		if errors.As(err, &fault) {
			fields["op"] = fault.op
		}
		d.warn(id, req.Cycle, CodePrepareFailed, err.Error(), fields)
		return "", nil, err
	}
	return path, cleanup, nil
}

func (d *Dispatcher) execute(ctx context.Context, env AdapterEnv) execution {
	start := d.now()
	exitCode, execErr := d.deps.Adapter.Exec(ctx, env)
	return execution{exitCode: exitCode, execErr: execErr, durationMS: d.now().Sub(start).Milliseconds()}
}

func (d *Dispatcher) record(req Request, id identity, p plan, prov provenance, x execution, warns []string) (Outcome, error) {
	out := Outcome{CLI: p.cli, Model: p.model, ArtifactPath: prov.artifactPath, ChallengeToken: prov.token,
		ExitCode: x.exitCode, DurationMS: x.durationMS, Warns: warns}
	v := VerifyArtifact(d.stat, d.read, d.now, prov.artifactPath, prov.token, x.exitCode, x.execErr)
	out.Verdict, out.Diagnostics, out.Integrity = v.Verdict, v.Diagnostics, v.Reason
	out.ArtifactSHA256 = d.hashArtifact(req, id, prov.artifactPath, v.Verdict)
	d.signalOutcome(req, id, p, prov, x, out)
	if req.LedgerPath != "" {
		if op, err := d.appendLedger(req.LedgerPath, entryOf(req, id, p, prov, x, out)); err != nil {
			d.warn(id, req.Cycle, CodeLedgerWriteFailed, "ledger write failed at "+op+": "+err.Error(),
				map[string]string{"step": "ledger", "op": op, "path": req.LedgerPath})
			return out, fmt.Errorf("subagent/run: ledger write: %w", err)
		}
	}
	if x.execErr != nil {
		return out, fmt.Errorf("subagent/run: adapter exec: %w", x.execErr)
	}
	return out, nil
}

func (d *Dispatcher) hashArtifact(req Request, id identity, artifact, verdict string) string {
	sha, err := d.hash(artifact)
	if err == nil {
		return sha
	}
	passedIntegrityLadder := verdict != VerdictIntegrityFail
	if passedIntegrityLadder {
		d.warn(id, req.Cycle, CodeArtifactHashFailed, `artifact hash failed; ledger stamps artifact_sha256="": `+err.Error(),
			map[string]string{"step": "hash", "artifact": artifact})
	}
	return ""
}

func (d *Dispatcher) signalOutcome(req Request, id identity, p plan, prov provenance, x execution, out Outcome) {
	fields := map[string]string{"exit_code": strconv.Itoa(x.exitCode), "cli": p.cli, "artifact": prov.artifactPath}
	switch {
	case x.execErr != nil:
		fields["step"], fields["verdict"] = "exec", out.Verdict
		if out.Integrity != "" {
			fields["integrity"] = string(out.Integrity)
		}
		d.warn(id, req.Cycle, CodeAdapterExecFailed, fmt.Sprintf("adapter exec failed (exit=%d): %v", x.exitCode, x.execErr), fields)
	case out.Verdict == VerdictIntegrityFail:
		fields["step"], fields["rung"] = "verify", string(out.Integrity)
		d.warn(id, req.Cycle, CodeArtifactIntegrityFail, out.Diagnostics[len(out.Diagnostics)-1].Message, fields)
	case out.Verdict == VerdictFAIL:
		fields["step"] = "verify"
		d.warn(id, req.Cycle, CodeVerdictFail, fmt.Sprintf("adapter exited %d over a sound artifact", x.exitCode), fields)
	}
}

func entryOf(req Request, id identity, p plan, prov provenance, x execution, out Outcome) ledgerEntry {
	return ledgerEntry{
		Cycle:          req.Cycle,
		Role:           req.Agent,
		Model:          p.model,
		ExitCode:       x.exitCode,
		DurationS:      strconv.FormatInt(x.durationMS/1000, 10),
		ArtifactPath:   prov.artifactPath,
		ArtifactSHA256: out.ArtifactSHA256,
		ChallengeToken: prov.token,
		GitHEAD:        prov.gitHead,
		TreeStateSHA:   prov.treeDiff,
		QualityTier:    QualityTier(p.cap.BudgetNative, p.cap.PermissionScoping),
		RunID:          id.runID,
	}
}
