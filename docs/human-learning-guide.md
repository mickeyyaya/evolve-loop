# Human Learning Guide

The evolve-loop is transparent by design. Everything it learns is stored in files you can read. This guide explains how to read those files and intervene when needed.

Challenge token: 41ba1555b6b85ad2

---

## 1. How to Read Instincts (`.evolve/instincts/`)

Instincts are lessons the loop extracted from past cycles — things that worked, things that failed, and patterns worth repeating.

Each instinct is a YAML file with these key fields:

```
id: inst-007
title: "Use inline task pattern for S-complexity"
category: procedural          # episodic | semantic | procedural
confidence: 0.91              # 0.0 (weak) to 1.0 (strong)
observation: "what was observed"
recommendation: "what to do next time"
source: cycle 12, task add-foo
```

**Confidence scores** reflect how often the pattern held up across cycles:
- `0.9+` — High confidence. The loop treats these as policy. The orchestrator applies them automatically.
- `0.7–0.89` — Medium confidence. Cited as guidance but not enforced.
- `< 0.7` — Low confidence. Experimental. May be discarded in consolidation.

**Categories:**
- `episodic` — A specific event worth remembering ("cycle 5 broke X because of Y")
- `semantic` — General knowledge ("eval graders with `^` anchors fail on macOS")
- `procedural` — Step-by-step patterns ("always read the file before editing")

Every 3 cycles, consolidation merges related instincts and archives superseded ones. The `archived/` subfolder holds retired instincts — useful for debugging regressions.

**What to look for:** If the loop keeps making the same mistake, check whether an instinct exists for it. If not, that's a gap. If one does exist but isn't being applied, check `instinctsApplied` in the build report.

---

## 2. How to Read the Scout Decision Trace

Open `.evolve/workspace/scout-report.md` and scroll to the `decisionTrace` section (usually near the bottom of the task list).

Each entry looks like:

```
- task: add-foo
  finalDecision: selected
  signals:
    - "processRewards.codeQuality below 0.7 for 2 cycles"
    - "taskArms.feature avgReward 0.85, pulls 7 — boosted"

- task: fix-bar
  finalDecision: rejected
  signals:
    - "similar to failed approach in cycle 11"
    - "dependsOn: add-baz not yet shipped"
```

**Reading the signals:** Each signal is a reason the Scout included or excluded a task. Common signals:
- `avgReward >= 0.8` — the loop has done this type of task well before
- `same-file churn` — the file was recently edited and is risky to touch again
- `source: introspection` — the loop identified this gap itself from metrics
- `source: crossover` — a hybrid task combining two previously successful types
- `deferred` — scheduled for a future cycle (see `pendingImprovements` in `state.json`)

**What to look for:** If a task you expected to run was rejected, its signals tell you why. If a task you didn't want ran anyway, its signals tell you what drove that decision.

---

## 3. How to Follow the Ledger (`.evolve/ledger.jsonl`)

The ledger is an append-only log — one JSON entry per agent invocation, every cycle. It never gets rewritten, only extended.

Open it with any text editor or `cat .evolve/ledger.jsonl | python3 -m json.tool` (one entry per line).

**Entry types:**

| type | role | what it means |
|------|------|---------------|
| `scout` | scout | Discovery phase complete — tasks selected |
| `build` | builder | A task was built (check `status: PASS\|FAIL`) |
| `audit` | auditor | Audit result — `selfVerify` and `status` fields |
| `ship` | orchestrator | Code committed and pushed |
| `learn` | orchestrator | Instincts extracted this cycle |
| `health` | operator | Loop health check — fitness score, stagnation flags |
| `tool-synthesis` | builder | A new reusable tool was created |

**Key fields to watch:**
- `data.status` — `PASS` or `FAIL` for build/audit entries
- `data.filesChanged` — how many files a build touched (high numbers = higher risk)
- `data.instinctsApplied` — which instincts shaped this build
- `data.selfVerify` — did the builder's own checks pass?

**Reading a sequence:** Find all entries for one cycle by filtering on `"cycle": N`. Reading them in order gives you the full story: what the Scout chose, what the Builder did, whether the Auditor approved, and what the loop learned.

---

## 4. How to Read Cycle Summaries (`docs/notes.md`)

Each completed cycle appends a summary block to `docs/notes.md` (or `workspace/session-summary.md` for end-of-session). The narrative section is written in plain prose by the Operator.

A typical summary covers:
- What tasks were attempted and which shipped
- Fitness score and delta from last cycle
- Any stagnation flags or HALT warnings
- The Operator's assessment of loop health

**What to look for:** Trends across summaries. If fitness scores are declining for 3+ cycles, the loop is degrading. If the same file keeps appearing in failed tasks, that's a problem area. The Operator's `next-cycle-brief` at the end of each summary tells you what the loop plans to prioritize next.

---

## 5. How to Audit the LLM's Decisions

The loop makes claims in build reports. Here is how to verify them.

**Check eval graders directly:**
Every task has eval graders listed in `scout-report.md`. Run them yourself from the repo root:
```bash
test -f docs/some-file.md && echo PASS || echo FAIL
awk '/keyword/{found=1} END{exit !found}' docs/some-file.md
```

**Diff the actual commit:**
```bash
git show <commit-sha>
```
Compare the changed lines to the build report's "Changes" table. If the report says "MODIFY path/to/file — added logging" but the diff shows something else, that's a discrepancy.

**Check instinct citations:**
The build report lists `instinctsApplied`. Open those YAML files and verify the instinct's recommendation actually matches what was built. Mismatches indicate the loop cited instincts it didn't actually follow.

**Check tamper detection:**
The Auditor runs checksum verification on eval grader files. If `eval-checksums.json` has changed unexpectedly, the Auditor should have flagged it. If it didn't, check `auditor-report.md` for the tamper detection section.

---

## 6. How to Learn from the Loop

The loop surfaces patterns that apply beyond its own operation.

**Patterns worth extracting for human use:**

1. **Decision traces make selection auditable.** Logging why each option was chosen (not just what was chosen) makes debugging fast. Apply this to any decision-heavy system you build.

2. **Confidence-weighted memory beats binary memory.** Instead of "remember this" or "forget this", the loop uses a confidence score that degrades or strengthens with evidence. Useful for any system that updates beliefs over time.

3. **Small tasks compound faster than large ones.** The loop consistently ships more value with 3 small tasks than 1 large task, because failure is isolated and learning is faster. This applies to human sprint planning too.

4. **Intrinsic novelty reward prevents local optima.** The loop tracks which files it hasn't touched recently and nudges itself toward them. For human teams, this maps to deliberately rotating which parts of the codebase get attention.

5. **Instinct consolidation prevents memory bloat.** Merging related instincts every 3 cycles keeps the memory compact and actionable. For humans: periodic retros that merge similar lessons are more useful than long, unread retrospective documents.

---

## 7. How to Intervene

The loop is designed to run autonomously, but you can override it at several points.

**Before a cycle starts — set the strategy:**
Edit `state.json` and change `strategy` to one of: `balanced`, `innovate`, `harden`, `repair`. Each preset shifts what the Scout prioritizes and how strict the Auditor is.

**Block a specific task:**
Add the task slug to `state.json` under `failedApproaches` with a `humanBlocked: true` note. The Scout will treat it as a failed approach and avoid re-selecting it.

**Force a task:**
Add it to `state.json` under `pendingImprovements` with `priority: "high"`. The Scout will treat it as a high-priority candidate in the next introspection pass.

**Trigger a HALT manually:**
Set `state.json.haltRequested: true`. The Operator checks this flag and will stop the loop at the end of the current cycle, writing a `handoff.md` with context for resuming.

**Edit instincts directly:**
Instinct YAML files are plain text. You can edit confidence scores, add new instincts by hand (follow the existing schema), or move instincts to `archived/` to retire them. Changes take effect in the next cycle.

**Resume after interruption:**
Read `workspace/handoff.md`. It contains the last known state, current strategy, and the Operator's recommended next step. Pass its contents as context when starting a new session.

---

## Further Reading

- [architecture.md](architecture.md) — Pipeline structure and agent responsibilities
- [docs/instincts.md](instincts.md) — Instinct schema and graduation rules
- [docs/self-learning.md](self-learning.md) — All seven self-improvement mechanisms
- [docs/memory-hierarchy.md](memory-hierarchy.md) — Where each type of data lives
