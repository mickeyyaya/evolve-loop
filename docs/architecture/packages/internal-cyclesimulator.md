# internal/cyclesimulator

## Purpose

`internal/cyclesimulator` is the Go port of the legacy `cycle-simulator.sh`. It walks every phase of a cycle with no LLM call. It writes deterministic artifacts and appends tamper-evident ledger entries, so the plumbing of a cycle can be tested at no model cost.

## Design

- **What a run proves.** `cycle-state.json` advances through every phase. The phase gate accepts every transition. The `prev_hash` chain stays intact after the run. `ship.sh --dry-run` runs inside a real cycle context.
- **What a run does not prove.** It does not prove the quality of LLM output, because no LLM runs. It does not prove real Builder file edits, because the source does not change. It does not prove real Auditor judgment, because the verdict is always PASS.
- **Exit codes.** They match `cycle-simulator.sh`. Code 0 means that every phase completed and the ledger chain is intact. Code 1 is a runtime failure. Code 2 means that the phase gate refused a transition. A ship dry run with a non-zero code is tolerated, because a tree-state mismatch is acceptable in a simulation.
- **Artifact names come from `phasecontract`.** The simulator exists to prove that the real plumbing accepts what the simulator writes. It cannot prove that from its own copy of the names. The phase key `research` is the legacy vocabulary of the gate. Its artifact is the one of scout, so the lookup uses `scout`.
- **Ledger layout.** `appendSimLedger` follows the byte layout of the bash `write_sim_ledger`. The entry has `simulated: true`, and `prev_hash` is the SHA256 of the last line. `ledger.tip` is written through a temporary file, in the `ledgertip.Format` encoding with no newline ([internal-ledgertip.md](internal-ledgertip.md)).
- **Run identity.** The simulator writes into the same ledger as the real loop, so its entries carry `run_id` too. The lookup that is scoped to a run refuses an entry with no run identity (cycle 1571, H1). The value is resolved from the run workspace of the artifact. It is set only when it can be resolved, in line with `json:"run_id,omitempty"` in `core`.

## Invariants

- **`jsonCompact` has a fixed key order.** The order matches the output of `jq -nc` in bash: ts, cycle, run_id, role, kind, model, exit_code, duration_s, artifact_path, artifact_sha256, challenge_token, git_head, tree_state_sha, entry_seq, prev_hash, simulated. `run_id` follows `cycle`, as in the other writers of `agent_subprocess`. Verifiers of the hash chain recompute the SHA over the raw line, so the order is load-bearing.
- **`jsonCompact` is an allowlist.** A key that is not in the list is dropped without a signal, however the caller filled the map. That is how the first try to stamp `run_id` looked right and emitted nothing (a go-review CRITICAL finding). Keep the slice and `runid_stamp_test.go` in step.
- **Only a behavioral test sees a dropped key.** A scan of the source sees that `RunIDFromWorkspace` is called. It cannot see that the value reaches the bytes. Pinned by `runid_stamp_test.go`, a test over the real writer.

## Findings

- `extra_coverage_test.go` builds a raw git repo with `git init` in one test (`TestRunGit_SuccessInRealRepo`). It skips when git is unavailable.
