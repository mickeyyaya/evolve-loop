# Comment history: `acs/cycle639`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle639/predicates_test.go:3` — above `package cycle639`

```text
// Package cycle639 materialises the acceptance criteria for the single
// triage-committed top_n task of cycle 639, self-sha-fail-early-boot-gate
// (weight 0.95, inbox 2026-07-08T03-05-30Z-self-sha-fail-early-boot-gate.json;
// carryover, named in cycles 629/630/632 lessons, never built).
//
// Defect: a WITHIN-VERSION ship-binary SHA mismatch (state.json:
// expected_ship_version == the current plugin version, but expected_ship_sha !=
// the on-disk go/bin/evolve) is boot-time-knowable and cycle-fatal — it is
// exactly the SELF_SHA_TAMPERED integrity failure the terminal ship gate raises
// (internal/phases/ship/verify.go:119-127). Yet boot only WARNed and proceeded,
// so 8 consecutive cycles (625-634) each burned a full ~32-40 min lane before
// dying at the terminal ship gate on a ship doomed from boot.
//
// Fix: classify the mismatch at boot the SAME way verifySelfSHA does — a
// within-version mismatch HALTs pre-scout with the operator-unblock recipe
// (`make -C go build` → `evolve reset-sha -operator` → relaunch) and does NOT
// auto-repin; an across-version / legacy-unversioned mismatch stays on the
// existing cycle-514 boot auto-repin path unchanged; a matching SHA boots into
// scout.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…637 precedent).
// Each predicate shells `go test -run` over the RED regression tests authored
// this cycle in cmd/evolve/cmd_loop_boot_selfsha_gate_test.go. Every one
// EXERCISES the system under test — the real defaultBootRecovery (driven with a
// real on-disk state.json + go/bin/evolve + .claude-plugin/plugin.json fixture)
// and the real runLoop boot path — and asserts on the resulting behaviour
// (HaltSelfSHA set + pin untouched + recipe emitted / auto-repin still fires for
// across-version / no action on a matched SHA / runLoop returns 2 pre-preflight).
// None is a source-grep. RED now: the cmd/evolve test package fails to build
// because bootRecoveryResult has no HaltSelfSHA field. GREEN once Builder adds
// the field, the within-version classification + halt-recipe in
// defaultBootRecovery, and the pre-scout return in runLoop.
//
// The fourth Acceptance line ("go vet ./..., -race, apicover green") is
// dispositioned manual+checklist in test-report.md — a repo-wide toolchain gate
// the cycle audit already runs, not predicated here.
```

### `go/acs/cycle639/predicates_test.go:78` — above `func TestC639_002_AcrossVersionStillAutoRepins(t *testing.T) {`

```text
// TestC639_002_AcrossVersionStillAutoRepins — AC2 (regression twin): an
// across-version mismatch (a legitimate plugin/version bump) stays on the
// existing cycle-514 boot auto-repin path — it heals and re-pins the SHA and
// does NOT trip the new within-version halt. Guards the fix against regressing
// the established auto-repin behavior.
```
