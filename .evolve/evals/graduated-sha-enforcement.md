---
score_cap:
  - criterion: "SELF_SHA mismatch with intact attestation chain → warn + repin, not integrity block"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestShip_SelfSHA_WarnRepin ./internal/phases/ship/"
  - criterion: "SELF_SHA mismatch WITHOUT attestation (genuine tampering) still → hard block (regression guard)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestShip_SelfSHA_HardBlock_NoAttestation ./internal/phases/ship/"
---

# Eval: Graduated SELF_SHA enforcement (operator-rebuild tier)

Cycle 249 failed at ship with `SELF_SHA_TAMPERED` after a legitimate operator binary rebuild
(`9b54c0a chore(build): rebuild tracked go/evolve`). The attestation chain was intact — the
commit-gate ran and produced a valid attestation — but the ship phase hard-blocked because the
binary's SHA didn't match the pinned value. This is a false positive for the operator-rebuild class.

The fix: when ship encounters a SELF_SHA mismatch, check whether the current cycle's
commit-gate attestation is present and valid. If yes → emit a WARN, repin the expected SHA
in state.json, and continue. If no attestation → retain the hard block (genuine tampering indicator).

## Graders

### [code] SELF_SHA mismatch + valid attestation → warn + repin (happy operator-rebuild path)

```bash
cd go && go test -count=1 -run TestShip_SelfSHA_WarnRepin ./internal/phases/ship/ 2>&1 | tail -3
# expected: PASS
```

### [code] SELF_SHA mismatch + no attestation → hard block (genuine tamper case)

```bash
cd go && go test -count=1 -run TestShip_SelfSHA_HardBlock_NoAttestation ./internal/phases/ship/ 2>&1 | tail -3
# expected: PASS
```

### [code] SELF_SHA mismatch + expired/invalid attestation → hard block

```bash
cd go && go test -count=1 -run TestShip_SelfSHA_HardBlock_InvalidAttestation ./internal/phases/ship/ 2>&1 | tail -3
# expected: PASS
```

### [code] After repin, subsequent ship call with new SHA passes without warn

```bash
cd go && go test -count=1 -run TestShip_SelfSHA_RepinnedPassesNextRun ./internal/phases/ship/ 2>&1 | tail -3
# expected: PASS
```

### [code] Full ship package tests still green (no regression)

```bash
cd go && go test -count=1 ./internal/phases/ship/... 2>&1 | grep -E "^(ok|FAIL)"
# expected: ok
```

### [model] Incident traceability: warn log line contains the old SHA, new SHA, and attestation reference

Verify that the warn path emits a structured log line containing:
- `old_sha` (expected value that no longer matches)
- `actual_sha` (the rebuilt binary's SHA)
- `attestation_ref` (the cycle/commit that produced the valid attestation)
- `action: "repin"` signal
