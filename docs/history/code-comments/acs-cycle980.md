# Comment history: `acs/cycle980`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle980/predicates_test.go:3` — above `package cycle980`

```text
// Package cycle980 materializes the cycle-980 acceptance criteria for this
// fleet lane's single scoped id `scan-phase-fast-tier-envelopes`.
//
// Goal (scout-report.md). Give judgment-light scan-phase profiles an explicit
// `model_tier_envelope` of {min:"fast", max:"balanced"} — overriding the
// universal {balanced,deep} floor — so their handoff-summary work can run at the
// fast tier while capping escalation at balanced. Twelve scan/checklist profiles
// are in scope; `secret-leak-scan.json` and `flake-rerun-scan.json` are OUT of
// scope (owned by `mechanical-scans-to-native`, which converts them to native
// Go) and MUST be left untouched.
//
// PREDICATE STYLE (cycle-85 rule): go/internal/{policy,profiles} are importable
// from go/acs, so the load-bearing predicates EXERCISE the live resolver —
// `policy.ValidatePin` — against the SHIPPED profiles (loaded from the on-disk
// worktree via acsassert.RepoRoot). C980_001 asserts the envelope *shape* and
// C980_002 asserts the envelope actually *clamps* by feeding ValidatePin a
// "deep" pin and requiring it to error. A magic-string source/config edit that
// does not produce a {fast,balanced} envelope the resolver honours cannot
// satisfy them.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE → C980_002 "reject deep" is the anti-no-op signal: a deep pin
//	           (rank 3) must be REJECTED for each of the 12 profiles once the
//	           {fast,balanced} envelope exists. RED today (nil envelope ⇒
//	           ValidatePin returns nil ⇒ no rejection).
//	EDGE/SCOPE→ C980_003 pins the OUT-OF-SCOPE boundary: the two excluded
//	           profiles must NOT gain a fast envelope and must STILL accept a
//	           deep pin. Green today; fails only if the Builder over-reaches.
//	SEMANTIC → C980_001 (shape: Min=="fast"/Max=="balanced"), C980_002
//	           (enforcement: clamp deep, admit fast), C980_004 (report-size
//	           budget stays in the 1–2K band) are distinct outcomes.
//
// RED before Builder: C980_001 fails (ModelTierEnvelope is nil on all 12) and
// C980_002 fails (ValidatePin admits a deep pin because there is no envelope to
// clamp against). C980_003 and C980_004 are regression guards — green today and
// MUST stay green after the change.
//
// AC map (1:1 with test-report.md AC-Materialization table):
//
//	AC1 12 profiles carry {min:fast,max:balanced}  → C980_001 + C980_002 (predicate)
//	AC2 excluded profiles left untouched            → C980_003 (predicate)
//	AC4 report-size budget resolves within 1–2K     → C980_004 (predicate, config-check)
```
