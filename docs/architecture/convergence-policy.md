# Convergence policy: every iterative loop converges or escalates

> ADR-0126 (Proposed, 2026-10-07) · plan: [convergence-policy-2026-10.md](../plans/convergence-policy-2026-10.md) · the example: [incident, L2's seven review rounds](../incidents/2026-10-07-l2-seven-review-rounds.md) · research: [converging review feedback within limited rounds](../research/review-convergence-limited-rounds-2026-10.md) (CR1–CR8), [ship-rate sources](../research/ship-rate-harness-reliability-2026-09-02-sources.md) §1 item 10 and §3(6), [ADR-0096](adr/0096-repair-rounds-escalate-and-carry-findings.md).
> Written in the Issue / Gap / Solution shape.
>
> **Revision 2** (2026-10-07) folds in two things:
> - the doc review: C1, C2, H1–H5, M1, M2 and L1–L3, all applied;
> - the research refinements CR1–CR8, and the operator's answer on headroom (top = Opus 5.5 at xhigh effort).
>
> **Operator directives (2026-10-07):**
> - *"It just retry for too many times."*
> - *"Prioritize a convergence rule/solution using L2 with 7 rounds as the example issue that should converge earlier by escalating to top / deep model or other approaches to proceed with persuing the perfection with no output."* (verbatim)
> - *"The latest top model is opus 5.5 with xhigh effort, not fable 5.1"*
> - *"I would like you to research online to learn the best policy to coverage the review feedback with limited rounds"* (verbatim)
>
> **Terms used throughout:**
> - **Loop:** a repeat-until-accepted process that calls the policy: audit repair (ADR-0092/0093/0096), the code-review↔build loop (ADR-0124, landing 2), the explanation re-author route, and a console lane's review chain. The contract-correction ladder is a format loop and does **not** call the policy (§6).
> - **Judgment `J_r`:** one judge pass. `J_0` is the **full** first judgment. Round *r* is fix *r* plus its judgment `J_r`.
> - **`Decide(round = r)`** returns the rung for round *r+1*.
> - **Finding:** one judged defect, with these fields:
>   - `id`, stable across rounds;
>   - `severity`: CRITICAL, HIGH, MEDIUM, LOW or INFO;
>   - `kind`: `defect` or `capability`;
>   - `component` (§2.1);
>   - `location`;
>   - `status`: OPEN, FIXED, DEFERRED, DISPUTED or FILED;
>   - `late`: raised on code unchanged since `J_0`;
>   - `certificate`.
> - **Blocking bar:** the severity at or above which an OPEN finding blocks the loop's hand-off. Every finding carries an explicit `blocking` result (CR6).
> - **Mass:** the loop's unresolved-severity measure. The code-review loop supplies its own U(n), from design §5.3 (its finding mass, including LOW, plus its gap mass, with the baseline reset at each audit-repair re-entry). Other loops use the default: CRITICAL 8, HIGH 4, MEDIUM 2, LOW 1, INFO 0.
> - **Progress:** the mass fell, **measured at the current bar for both rounds**, so that raising the bar never fakes progress.
> - **Repair damage:** in round *r*, the previous FIXED findings that `J_r` reopens, plus the new findings inside fix *r*'s own hunks. **Repairs** are the findings fix *r* resolved.
> - **Concentration `C`:** the largest share of the window's countable findings (§2.1) that falls in one component.
> - **Headroom:** whether a raise changes the (model, effort) pair (§7).
> - **Follow-up:** an inbox item filed for a deferred finding, a split component, or a filed late finding.

## Issue

Loops grind instead of converging, and they end with no output.

- **Audit repair.** ADR-0096 measured ship probability by audit-round count over cycles 1560–1605: **100% → 50% → 17% → 0%**.
- **The console.** L2 needed **seven** review fix rounds on the deepest Claude model (incident). About 18 of the last 27 findings sat in one heuristic sub-feature that a review had added.
- **The research.** Two rounds capture 76–95% of the achievable gain. A fixed budget can *lower* quality, because repair damages correct work. LLM reviewers produce different findings on every run. More revisions do not converge a review unless they reduce the reviewer's uncertainty (research F1–F4).

## Gap

1. **No shared rule.** Each loop has its own stop. None escalates past a tier, and the console has nothing.
2. **The blocking bar never moves.**
3. **No stop on marginal gain, and no keep-best.** Later rounds can be worse, and the last round lands by default (research F2).
4. **Delta judgments hunt instead of verify.** A rerun of a nondeterministic reviewer always finds something new (research F3).
5. **Reviews can grow the change.** A capability finding is fixed inside the lane.
6. **Escalation has no headroom check.** No family has deep→top headroom in today's tier tables (§7).
7. **No output-first fallback.**

## Solution

```
J_0 full judgment ─▶ clean? ── yes ─▶ land
        │ no
        ▼
round 1  fix 1 ─▶ J_1   (audit repair: ADR-0096's tier raise stays here)
        │ strict OPEN remains
        ▼  marginal gain ≤ 0? ── yes ─▶ skip to round 3's strategy change (or rung 3 if already past it)
round 2  RUNG 1 — change the feedback: J_2 verify-only (fixed scope + falsification check);
                  effort raise to top for reasoning-class blockers (headroom permitting); fixer raised if not already
        │ strict OPEN remains
        ▼
round 3  RUNG 2 — change the strategy: fresh-context fixer (or re-plan); bar → HIGH; MEDIUM/LOW deferred + filed
        │ strict OPEN remains
        ▼
         RUNG 3 — change the scope: split (Stop + continuation / console unstage) │ accept fail-safe residue → audit adjudicates │ stop
         land the BEST qualifying round (keep-best); never round 4
```

### 1. Strategy (why this shape)

The ladder was chosen over a bare round cap and over best-of-N (plan §2). Rounds 1–2 are local refinement. Round 3 is allowed only after a strategy change, and there is never a round 4 (research CR1, F6).

### 2. One policy, one home

There is a pure package, `convergence`, with no I/O:

```go
type Input struct {
    Loop      string        // audit-repair | code-review | explanation-reauthor | console-lane
    Round     int           // completed rounds r (J_0 alone is round 0)
    Rounds    []Judgment    // J_0..J_r: findings (§2.1 schema), plus each fix's hunks
    Mass      MassFunc      // loop-supplied (code-review: U(n)); nil → default weights
    Fixer     TierEffort    // (tier, model, effort) of the last fix
    Judge     TierEffort    // (tier, model, effort) of the last judgment
    Headroom  HeadroomTable // per family: which raises change (model, effort) (§7)
    Config    Config        // workflow.convergence (§8)
}

type Decision struct {
    Rung          int        // 0..3, for round r+1
    Action        Action     // Continue | Land | Split | AcceptWithLimits | Adjudicate | Stop
    BlockingBar   string     // CRITICAL | HIGH | MEDIUM
    VerifyOnly    bool       // J_{r+1} is verify-only (code-review and console judges only, §4)
    FreshContext  bool       // fix r+1 starts from a distilled brief
    FixerRaise    TierEffort // zero value = no raise
    JudgeRaise    TierEffort // zero value = no raise
    Defer         []string   // finding ids → DEFERRED with follow-ups (never CRITICAL, never audit findings)
    File          []string   // late findings below HIGH, and capability findings below CRITICAL → FILED
    SplitComponent string
    LandRound     int        // keep-best: the round whose candidate lands
    Reasons       []string   // the evidence for every rung change (CR8)
}
```

- **The decision function** is `Decide(Input) Decision`, called at each loop's existing decision point.
- **The console** calls `evolve convergence decide --input <rounds.json> [--json]`, whose input is the same schema.

#### 2.1 Input schema and edge cases (one rule each)

1. **The `rounds.json` shape.**
   - It is `{loop, round, rounds:[{index, findings:[…], fix_hunks:[{file, from, to}]}]}`, and each finding carries the fields listed in Terms.
   - Ids are stable across rounds: a re-raised finding keeps its id.
2. **INFO** has weight 0. It never blocks and is never deferred.
3. **`component`** is optional and judge-declared, naming the sub-feature: for example `bridge:model-check`, which spans 7 packages. When absent, it falls back to the location's directory:
   - the Go package directory for `go/**`;
   - `go/acs/<cycle>` for ACS predicates;
   - the second path segment for `skills/<name>`, `docs/<area>`, `.evolve/<area>` and `agents/`;
   - the file itself for top-level files.
4. **A finding with no location** gets the component `unknown`. It counts toward mass, but not toward concentration. It never disables the policy.
5. **Concentration** counts findings at LOW or above (not INFO) in the window. It is defined only when the window holds **at least 5** countable findings. Ties go to the lexically smallest component.
6. **The window** is `concentration_window` rounds, default 2, ending at round *r*. The trigger needs two consecutive windows at or above the threshold (§3).
7. **Progress across a bar raise:** compare the mass of findings at or above the **current** bar in both rounds. For code-review, U(n) at the current bar is the finding mass of findings at or above the bar plus the gap mass.
8. **Rung positions follow `max_fix_rounds` = *N*:**
   - rung 1 is used for rounds 2 … *N*−1;
   - rung 2 is the final round *N*;
   - rung 3 follows round *N*.

   With the default *N* = 3, this matches the diagram.
9. **Nested loops.** The code-review round budget is **cycle-wide, across every audit-repair re-entry**, as in ADR-0124: `max_rounds` counts every review in the cycle. Only the **progress baseline** resets at a re-entry. The budget is never multiplied.
10. **Configuration decoding:** unknown keys are rejected (strict decode), and unknown enum words warn and resolve to the default.

### 3. The ladder

| Round | Rung | What changes | Blocking bar |
|---|---|---|---|
| `J_0` | — | a full judgment (probe and mutant quotas are allowed here only) | base (MEDIUM) |
| 1 | 0 | fix 1 with findings and certificates. For audit repair, ADR-0096's tier raise stays at this first repair. | base |
| 2 | 1, **change the feedback** | (a) `J_2` is **verify-only** for code-review and console judges (§4). (b) The judge's **effort** rises within its own family (deep high → top xhigh) **only for reasoning-class blockers** (correctness, concurrency, architecture), and only with headroom (§7). (c) The fixer is raised if it was not already. | base |
| 3 (= *N*) | 2, **change the strategy** | (a) A **fresh-context** fixer works from a distilled brief (the open strict findings, their certificates, and what each earlier fix changed), or it re-plans the approach. (b) The bar rises to **HIGH**. (c) Open MEDIUM and LOW findings become **DEFERRED** with filed follow-ups, for code-review and console only. | HIGH |
| after *N* | 3, **change the scope** | the exits in §3.1. **No round *N*+1.** | HIGH; CRITICAL never deferred |

**Triggers:**
- **Marginal gain ≤ 0 (CR2).** If round *r*'s repair damage is at least its repairs, or there is no progress at the current bar, the next round goes **straight to rung 2's strategy change**. If round *r* is already the final round, the loop goes to rung 3.
- **Concentration (CR5).** `C` ≥ `concentration_threshold` (default 0.6) on **two consecutive** windows, each with at least 5 findings, escalates **the hot component only**.
  - That component takes a rung-3 exit of its own: **split** if it is separable, or **accept with limits** if it is certified fail-safe.
  - The rest of the loop continues on the ladder.
  - If neither exit fits the component, the trigger is signalled and the ladder continues. Concentration alone never Stops a loop that is converging elsewhere.
  - **L2 replayed.** Its 2-round shares, with INFO excluded, were 0.39, 0.636 and 0.625, so the trigger reaches its threshold at `J_3`, after the final round. By then the bar is HIGH and no strict finding remains, so L2 **lands at round 3 through rung 2**. The trigger signals `CONVERGENCE_CONCENTRATION`, and the hot component's deferred MEDIUM/LOW findings carry a redesign follow-up. That is the structured-source item the console filed by hand.
- **Oscillation.** An id reopened after FIXED twice (as in ADR-0124) jumps to rung 3.

**Kind (C2):**
- A **capability** finding below CRITICAL never blocks: it is FILED at once, at any rung.
- A CRITICAL is always `kind: defect`.
- In the audit, `kind` is recorded, never used to unblock.

**Keep-best (CR3):** every round's candidate is checkpointed, and the loop lands the **best qualifying** round, never automatically the last. "Best" means the fewest open findings at the bar, with the earliest round winning a tie.

#### 3.1 Rung 3 exits (deterministic order; split and accept both require **no open CRITICAL**)

1. **Split.** This exit applies when the concentrated component holds every remaining strict finding. When the concentration trigger fires earlier, split applies only to that component, and the rest continues.
   - **Console lane:** the component is unstaged and filed as its own item. The split is allowed only when **the rest passes the floor without it**. A component wired into shared paths cannot be split; for example, L2's check is wired into `runTmuxREPL` and preflight.
   - **Cycle:** a split is a **Stop** plus an ADR-0076 continuation whose brief unwires the component (`CONVERGENCE_SPLIT`). A cycle never ships DEFERRED HIGH rows by splitting.
2. **Accept with limits.** This exit applies when every remaining strict finding is in a component the judge certifies as **failing safe**. The certificate is a test showing the failure direction falls back to a safer path.
   - The findings become DEFERRED with the certificate attached.
   - **For code-review, the audit adjudicates them, exactly as it adjudicates a DISPUTED row** (ADR-0124 §6.4). UPHOLD reopens the finding, and the cross-check then FAILs.
   - The landing records the limits plus a redesign follow-up (`CONVERGENCE_ACCEPTED_LIMITS`).
3. **Stop.** In every other case:
   - the work is preserved (an ADR-0076 continuation for a cycle, or a staged lane for the console);
   - every open finding is filed (`CONVERGENCE_STOP`);
   - the loop's existing ship rule decides the outcome. Audit repair FAILs, and code-review hands to the audit with `REVIEW_UNRESOLVED`.

#### 3.2 Adjudication (with headroom only)

Before a rung-3 exit, one `Adjudicate` dispatch at the judge family's **top** (model, effort) may rule each remaining HIGH finding `CONFIRM` or `DOWNGRADE`.
- This dispatch runs only when top differs from the judge's last (model, effort). Otherwise it is skipped, and `CONVERGENCE_NO_HEADROOM` is signalled.
- When the two judges are the same model at different efforts, their errors are correlated (research F7). A disagreement between them is settled by **deterministic evidence** (a certificate or test), never by a third LLM opinion.

### 4. Rules for judges

These bind the **code-review judge and console delta checks**. The audit keeps its full verdict rules (§11).

1. **`J_0` is a full judgment.** Every finding carries severity, kind, `blocking`, `component` (when the judge can name it), a certificate and a fix (CR6). Probe and mutant quotas belong to `J_0` only.
2. **`J_1` and later are verify-only, with a fixed scope (CR4).** The scope is the previous findings plus fix *r*'s own hunks:
   - each previous finding is FIXED (with a certificate) or not;
   - regressions are judged only within the fix's hunks.
3. **Late findings** are findings on code unchanged since `J_0`, and they are mostly judge noise (research F3).
   - A late finding below HIGH is FILED and never blocks.
   - A **late CRITICAL or HIGH blocks only when its certificate survives a falsification check** (refute-or-promote, seeing only the diff). This tightens ADR-0124 §5.3's `late` rule with the falsification check. Otherwise it is FILED.
4. **Every blocking finding passes the falsification check** before it blocks (research F3).
5. **A dispute never becomes another round.** It goes to the adjudicator: the audit for code-review (DISPUTED, as in ADR-0124), and the operator for console lanes (research F5).

### 5. Output first

- **From rung 2, the objective changes.** It becomes "ship the best qualifying round". Every exit files follow-ups for what it did not resolve, so perfection is pursued through the inbox, after output.
- **Metrics:**
  - rounds-to-land per loop;
  - the deferred-then-closed share;
  - the falsification-check drop rate. A high rate means a noisy judge, and noise lowers the judge's trust weight (phase B).

### 6. Where it applies

| Loop | Calls `Decide`? | Rungs it uses | Notes |
|---|---|---|---|
| Audit repair (ADR-0092/0093/0096) | yes. *N* = the envelope's attempt budget (default 2), so round 2 is its final round and combines rung 1's judge effort raise with rung 2(a)'s fresh-context fixer. | rung 1 (the judge effort raise; no verify-only, since the audit keeps full rules), rung 2(a) (the fresh-context fixer), Stop | **No deferrals and no split or accept**: no deferral overrides the audit's FAIL. ADR-0096's raise stays at the first repair. The retry envelope stays the budget's home. |
| Code-review ↔ build (ADR-0124, landing 2, Q3) | yes | all | `max_fix_rounds` 3 equals ADR-0124's `max_rounds` 4 (4 reviews, 3 fix builds), so the **budget is unchanged**. Mass is U(n). **Amends ADR-0124 §6.2:** a code-review row DEFERRED by the policy at rung 2 (MEDIUM/LOW, with a filed follow-up) no longer counts as strict OPEN, and accept-with-limits rows go to audit adjudication. |
| Explanation re-author | yes | as audit repair | It inherits the envelope. |
| Console lane | yes, through the verb | all, with split = unstage | The console follows the decision from the start. |
| Contract-correction ladder | **no** | — | It is a format loop, and its findings carry no severity or component. The existing mechanisms already play the rung roles: the second-block CLI escalation is rung 1, and the exhaustion degrade is rung 3 (the code-review design §7, its recovery-rung table; landing 1). |

### 7. Escalation headroom

Today's tier tables have **no** deep→top headroom in any family:

| Family | deep | top | deep→top headroom |
|---|---|---|---|
| claude-tmux | opus (effort from profile) | opus | **none in the manifest** |
| agy-claude | Claude Opus 5.5 (High) | Claude Opus 5.5 (High) | none |
| agy (Gemini) | Gemini 3.1 Pro (High) | Gemini 3.1 Pro (High) | none |
| codex | gpt-5.6-sol | gpt-5.6-sol | none |

- **The operator's directive** (2026-10-07): top is **Opus 5.5 at xhigh effort**. The claude CLI accepts `--effort` low, medium, high, xhigh and max (model catalog `efforts`).
- **Component V3b** gives Claude's top tier the xhigh effort through the one tier table. After that, deep (high) → top (xhigh) is real headroom, by **effort** within the same model.
- **Raises stay within the judge's own family.** "One tier above" is never computed across families.
- **What the headroom is good for (CR7).** Effort helps only **reasoning-class** failures. It does not help format, docs or hygiene findings, or a capability gap (research F7). Until V3b lands, rung 1(b) and §3.2 are **dormant** and signal `CONVERGENCE_NO_HEADROOM`. They never fake a raise.

### 8. Configuration

```json
"workflow": {
  "convergence": {
    "stage": "shadow",
    "max_fix_rounds": 3,
    "base_blocking_bar": "MEDIUM",
    "raised_blocking_bar": "HIGH",
    "concentration_threshold": 0.6,
    "concentration_window": 2,
    "concentration_min_findings": 5
  }
}
```

- **Defaults and decoding:** compiled defaults live in `internal/policy`. Unknown keys are rejected, and unknown enum words warn and resolve to the default (§2.1 rule 10).
- **`stage`:**
  - `shadow`: loops compute and signal the decision.
  - `enforce`: loops act on it.
- **The console follows the decision from the start.** There are no flags.

### 9. Signals (module `convergence`)

| Signal | Severity | Fields | When |
|---|---|---|---|
| `CONVERGENCE_RUNG` | INFO | `loop`, `round`, `rung`, `bar`, `reasons` | every decision |
| `CONVERGENCE_NO_PROGRESS` | WARN | `loop`, `round`, `mass_prev`, `mass`, `bar` | no progress at the current bar |
| `CONVERGENCE_REPAIR_DAMAGE` | WARN | `loop`, `round`, `damage`, `repairs` | damage ≥ repairs |
| `CONVERGENCE_CONCENTRATION` | WARN | `loop`, `component`, `share`, `window` | `C` at or above the threshold |
| `CONVERGENCE_DEFERRED` | INFO | `loop`, `count`, `followups` | rung 2 deferrals |
| `CONVERGENCE_FILED` | INFO | `loop`, `late`, `capability` | late and capability findings filed |
| `CONVERGENCE_SPLIT` | WARN | `loop`, `component`, `followup` | rung 3 split |
| `CONVERGENCE_ACCEPTED_LIMITS` | WARN | `loop`, `component`, `certificate` | rung 3 accept |
| `CONVERGENCE_STOP` | WARN | `loop`, `round`, `open`, `land_round` | rung 3 stop |
| `CONVERGENCE_NO_HEADROOM` | INFO | `loop`, `family`, `from`, `to` | a raise that changes no (model, effort) |

### 10. Recovery rungs (only logic blocks)

- **The decision cannot be computed** (malformed rounds). The loop keeps its current behaviour, signals the cause, and never blocks. A finding with no location is *not* malformed (§2.1 rule 4).
- **A follow-up cannot be filed.** The deferral or filing still happens, and the finding is written into the landing notes.

### 11. What this does not do

- **It never defers a CRITICAL, and never lets one land.** Split and accept both require no open CRITICAL.
- **It never turns an audit FAIL into a PASS.** The audit's verdict rules and the kernel cross-checks (ADR-0124/0125) are unchanged. The audit records `kind`, and verify-only does not bind it. The one changed input is which *code-review rows* the cross-check counts (§6), and accept-with-limits rows still face the audit's adjudication.
- **It does not replace any loop's budget home.**
- **It does not add a phase.**
