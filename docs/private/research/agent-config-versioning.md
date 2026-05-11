# Agent Config Versioning

> Reference document for tracking and managing agent configuration versions.
> Use these patterns to ensure every agent output is traceable to the exact
> configuration that produced it, enable rollback to known-good states, and
> prevent configuration drift across evolve-loop cycles.

## Table of Contents

1. [What to Version](#what-to-version)
2. [Versioning Strategies](#versioning-strategies)
3. [Traceability](#traceability)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Implementation Patterns](#implementation-patterns)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## What to Version

Track every configuration element that affects agent behavior or output quality.

| Config Element | Description | Change Frequency | Impact of Unversioned Change |
|---|---|---|---|
| **System prompts** | Instructions injected before user input (Scout, Builder, Auditor roles) | Low | Agents silently change behavior; outputs become non-reproducible |
| **Tool definitions** | Available tools, parameter schemas, permission scopes | Low | Agents gain/lose capabilities without audit trail |
| **Model selection** | Which model powers each agent (Haiku, Sonnet, Opus) | Medium | Cost and quality shift with no traceability |
| **Hyperparameters** | Temperature, max tokens, top-p, thinking budget | Medium | Output variability changes; reasoning depth shifts |
| **Instincts** | Learned patterns with confidence scores (`instincts/*.yaml`) | High | Agent avoidance/preference rules change silently |
| **Genes** | Reusable fix templates with selectors and validation | High | Automated fix behavior changes without record |
| **Eval definitions** | Grading criteria, pass/fail thresholds, scoring rubrics | Medium | Quality bar shifts without detection |
| **Phase-gate rules** | Transition conditions between Scout, Build, Audit, Ship, Learn | Low | Pipeline integrity changes silently |

---

## Versioning Strategies

Select a strategy based on change frequency and rollback requirements.

| Strategy | Mechanism | Best For | Pros | Cons |
|---|---|---|---|---|
| **Content-addressable** | SHA-256 hash of config content | Instincts, genes, eval definitions | Tamper-proof; identical content = identical hash | No ordering; hard to compare "newer vs older" |
| **Semantic versioning** | `MAJOR.MINOR.PATCH` bumps | Plugin releases, system prompts | Human-readable; conveys breaking vs non-breaking | Requires manual discipline to bump correctly |
| **Timestamp-based** | ISO 8601 timestamp per snapshot | Cycle-level snapshots | Automatic; no manual step needed | No semantic meaning; clock drift risk |
| **Git-based** | Commit SHA of config files | All configs checked into repo | Free with existing git workflow; full diff history | Requires configs to live in version control |

### Recommended Combination

| Config Element | Primary Strategy | Secondary Strategy |
|---|---|---|
| System prompts | Git-based (commit SHA) | Semantic version in file header |
| Instincts / genes | Content-addressable (SHA-256) | Timestamp per modification |
| Eval definitions | Content-addressable (SHA-256) | Git-based |
| Plugin manifest | Semantic versioning (`plugin.json`) | Git-based |
| Hyperparameters | Git-based | Timestamp-based snapshots |

---

## Traceability

Link every agent output back to the exact configuration that produced it.

### Required Metadata per Build Report

| Field | Source | Example |
|---|---|---|
| `config_version` | SHA-256 of combined config snapshot | `a3f8c1...` |
| `model_id` | Model identifier used for this cycle | `claude-sonnet-4-6` |
| `instinct_hash` | Hash of active instincts at cycle start | `b7d2e4...` |
| `gene_hash` | Hash of active gene library at cycle start | `c9f1a3...` |
| `eval_hash` | Hash of eval definitions used for grading | `d4e6b8...` |
| `plugin_version` | Semantic version from `plugin.json` | `8.0.0` |
| `git_commit` | Commit SHA of the config files | `9a5506f` |
| `cycle_number` | Evolve-loop cycle that used this config | `142` |

### Traceability Chain

```
build-report.md
  → references config_version (SHA-256)
    → maps to specific instincts, genes, evals, prompts
      → each element has its own version/hash
        → git history provides full diff trail
```

---

## Mapping to Evolve-Loop

Apply versioning to evolve-loop's existing structures.

| Evolve-Loop Concept | Versioning Approach | Implementation |
|---|---|---|
| **state.json** | Add a `config_version` field | Store SHA-256 of all active config files at cycle start |
| **Instinct confidence** | Treat confidence changes as version increments | Log confidence deltas per cycle; hash instinct file after LEARN phase |
| **Gene versioning** | Hash `selector` + `action` + `validation` fields | Increment `successCount`/`failCount` without changing the version; re-hash when steps change |
| **Eval checksums** | Hash eval criteria before each Auditor run | Compare current hash to previous cycle; flag drift |
| **plugin.json version** | Bump on meaningful config changes | `MAJOR` = breaking agent behavior change; `MINOR` = new capability; `PATCH` = tuning |
| **Scout prompts** | Version via git commit SHA | Tag prompt changes in commit messages |
| **Builder prompts** | Version via git commit SHA | Tag prompt changes in commit messages |
| **Auditor prompts** | Version via git commit SHA | Tag prompt changes in commit messages |

### Config Snapshot per Cycle

Generate a snapshot at the start of each cycle in the SCOUT phase:

```
.evolve/snapshots/cycle-{N}/
  config-snapshot.json    # Combined hash + individual hashes
  instincts-active.yaml   # Copy of active instincts
  genes-active.yaml       # Copy of active genes
  eval-criteria.yaml      # Copy of eval definitions
```

---

## Implementation Patterns

### Pattern 1: Config Snapshot per Cycle

| Step | Action | Timing |
|---|---|---|
| 1 | Collect all active config files | Start of SCOUT phase |
| 2 | Compute SHA-256 of each file | Before any agent execution |
| 3 | Compute combined SHA-256 of all hashes | Single version identifier |
| 4 | Write `config-snapshot.json` to cycle workspace | Alongside scout-report.md |
| 5 | Reference `config_version` in build-report and audit-report | During BUILD and AUDIT phases |

### Pattern 2: Diff Between Versions

| Step | Action | Command |
|---|---|---|
| 1 | Identify two cycle snapshots to compare | `ls .evolve/snapshots/` |
| 2 | Diff instinct files | `diff cycle-{A}/instincts-active.yaml cycle-{B}/instincts-active.yaml` |
| 3 | Diff gene files | `diff cycle-{A}/genes-active.yaml cycle-{B}/genes-active.yaml` |
| 4 | Diff eval criteria | `diff cycle-{A}/eval-criteria.yaml cycle-{B}/eval-criteria.yaml` |
| 5 | Summarize changes in learn-report | Include in LEARN phase output |

### Pattern 3: Rollback to Known-Good Config

| Step | Action | When |
|---|---|---|
| 1 | Detect quality regression via Auditor scores | Audit score drops below threshold |
| 2 | Identify last known-good config snapshot | Query snapshots with passing audit scores |
| 3 | Restore instincts, genes, and evals from snapshot | Copy snapshot files to active locations |
| 4 | Re-run cycle with restored config | Verify quality recovery |
| 5 | Investigate which config change caused regression | Diff current vs restored snapshot |

### Pattern 4: A/B Config Comparison

| Step | Action | Purpose |
|---|---|---|
| 1 | Create two config branches (A and B) | Isolate the variable under test |
| 2 | Run identical tasks with config A | Baseline measurement |
| 3 | Run identical tasks with config B | Variant measurement |
| 4 | Compare Auditor scores and build quality | Quantify impact of config change |
| 5 | Promote winning config; archive losing config | Data-driven config evolution |

---

## Prior Art

| Tool / Framework | Versioning Approach | Relevance to Evolve-Loop |
|---|---|---|
| **MLflow** | Experiment tracking with parameter logging and artifact versioning | Model for logging hyperparameters and linking runs to configs |
| **Weights & Biases** | Config snapshots per run; automatic diff between runs | Inspiration for per-cycle config snapshots and comparison UI |
| **DVC (Data Version Control)** | Git-like versioning for data and model files | Pattern for versioning large config artifacts alongside code |
| **Git + conventional commits** | Commit messages encode change type and scope | Already used; extend to tag config-affecting commits explicitly |
| **Terraform state** | Declarative config with plan/apply and state locking | Model for "planned config" vs "applied config" distinction |
| **Kubernetes ConfigMaps** | Versioned config objects with rollback via `kubectl rollout undo` | Pattern for declarative config with built-in rollback |
| **Nix/NixOS** | Content-addressable, reproducible system configurations | Gold standard for config reproducibility; hash-based versioning |

---

## Anti-Patterns

Avoid these common mistakes when managing agent configuration versions.

| Anti-Pattern | Problem | Fix |
|---|---|---|
| **Unversioned configs** | No way to reproduce past agent behavior; debugging is guesswork | Hash or tag every config before use |
| **Config drift** | Active config diverges from checked-in config over time | Snapshot at cycle start; compare to git HEAD |
| **No rollback path** | Bad config change has no recovery mechanism | Maintain snapshots; automate restore from last-known-good |
| **Version without diff** | Version numbers exist but no way to see what changed | Store full snapshots or use git-based diffing |
| **Manual version bumps** | Humans forget to bump; version falls out of sync | Automate version computation (hash-based or CI-enforced) |
| **Shared mutable config** | Multiple agents read/write the same config file concurrently | Use immutable snapshots; write new version, never mutate in place |
| **Ignoring instinct churn** | Instinct confidence changes every cycle but is never tracked | Log confidence deltas; treat high-churn instincts as unstable |
| **Eval criteria drift** | Grading standards shift without record; Auditor scores become incomparable | Hash eval criteria; flag changes between cycles |
