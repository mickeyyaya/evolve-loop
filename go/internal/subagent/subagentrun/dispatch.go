package subagentrun

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

// Dispatch is the `evolve subagent run` execution path in named steps: admit
// → resolve → prepare → the effective worktree → stage the prompt → exec →
// record. Statements move, values and their order do not — every error text,
// the prompt, the adapter env, the Warns channel, the verdict wiring and the
// ledger line are byte-identical to the host's former Run; the run id is
// resolved once, as admission's last act, instead of at the ledger step, and
// the identity is complete from here on. The prompt temp file lives exactly
// as long as the exec.
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

// worktreeFor is step 12's first half: the Warns channel (the capability
// warns, then the fallback sentence) and the effective worktree. The
// ProjectRoot fallback is deliberate — a non-worktree dispatch has no lane
// worktree and must still run — but it is never what a FLEET lane wants: an
// orchestrator that forgot to propagate WorktreePath silently points the
// agent at the main repo tree, the shape the tree-diff guard kills a lane
// for. The sentence stays on the Warns channel callers already print, and the
// same fallback is BRIDGE_SUBAGENT_WORKTREE_FALLBACK on the stream.
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

// stage materialises the prompt through the stager; a failure is ONE
// BRIDGE_SUBAGENT_PREPARE_FAILED whose op names the syscall when the stager
// says.
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

// execute is step 12's second half: the two clock reads bracket exactly the
// adapter call.
func (d *Dispatcher) execute(ctx context.Context, env AdapterEnv) execution {
	start := d.now()
	exitCode, execErr := d.deps.Adapter.Exec(ctx, env)
	return execution{exitCode: exitCode, execErr: execErr, durationMS: d.now().Sub(start).Milliseconds()}
}

// record is steps 13-14: the verification ladder (its diagnostics and rung
// carried on the Outcome instead of dropped), the artifact hash, the ONE
// outcome signal of a non-PASS run, then the ledger line — written whenever a
// ledger path is set, even on an adapter error, whose error masks the exec
// error exactly as before. The outcome signal precedes the ledger append so a
// ledger failure cannot hide an adapter failure on the stream (signals are not
// the ledger); the error-return order is unchanged.
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

// hashArtifact is step 13's second half: the artifact's sha256, "" when it
// cannot be read. A hash failure is BRIDGE_SUBAGENT_ARTIFACT_HASH_FAILED only
// when the artifact stood the ladder (PASS or FAIL) — on INTEGRITY_FAIL the
// rung already names the artifact fault.
func (d *Dispatcher) hashArtifact(req Request, id identity, artifact, verdict string) string {
	sha, err := d.hash(artifact)
	if err == nil {
		return sha
	}
	if verdict != VerdictIntegrityFail {
		d.warn(id, req.Cycle, CodeArtifactHashFailed, `artifact hash failed; ledger stamps artifact_sha256="": `+err.Error(),
			map[string]string{"step": "hash", "artifact": artifact})
	}
	return ""
}

// signalOutcome emits the ONE outcome signal of a non-PASS run: an adapter
// error is BRIDGE_SUBAGENT_ADAPTER_EXEC_FAILED carrying what the ladder found;
// otherwise an INTEGRITY_FAIL is BRIDGE_SUBAGENT_ARTIFACT_INTEGRITY_FAIL with
// its rung and the ladder's diagnostic, and a FAIL over a sound artifact is
// BRIDGE_SUBAGENT_VERDICT_FAIL with the exit code. A PASS emits nothing.
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

// entryOf is the ledger record of one run: the FULL agent name as the role,
// the integer-truncated duration as a string, the git state and the quality
// tier, and the run id resolved after the gate.
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
