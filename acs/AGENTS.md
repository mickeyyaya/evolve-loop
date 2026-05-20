# acs/ — Acceptance Criteria (ACS) Predicate Guide

> **Directory purpose**: Acceptance criteria shell predicates that gate the
> Build → Audit transition. Each predicate encodes one acceptance criterion and
> is run by the TDD-Engineer (RED phase) and Builder (GREEN verification).
> Passing predicates are promoted to `acs/regression-suite/cycle-N/` after the
> cycle ships.

## Directory layout

```
acs/
  cycle-N/                    # predicates authored by TDD-Engineer for cycle N
    001-<slug>.sh             # predicate 001
    002-<slug>.sh             # predicate 002
    …
  regression-suite/           # promoted predicates (permanent regression set)
    cycle-N/                  # copies from acs/cycle-N/ after GREEN ship
    rhds-end-to-end/          # cross-cutting end-to-end regression slices
  AGENTS.md                   # this file
```

## Predicate quality requirements

### Rule: behavioral, not grep-only

Every predicate MUST invoke the system under test as a subprocess. Predicates
that only `grep` source files or check for text presence are FORBIDDEN.

| Category | Pattern | Verdict |
|---|---|---|
| Behavioral | Runs the function/script/process and asserts on exit code, stdout, or side effect | REQUIRED |
| Mixed | Runs the system AND greps source for sanity strings | ACCEPTABLE |
| Grep-only | Only contains `grep`, `test`, `[`, `[[` — no subprocess invocation | FORBIDDEN |
| Waived config-check | Config-presence check with `# acs-predicate: config-check` comment | ALLOWED with waiver |

### File-existence dual-check (cycle-93+)

File-existence predicates MUST combine two checks, not just one:

```bash
# Disk presence
[ -f "$path" ] || { echo "RED: $path missing on disk"; exit 1; }

# Git tracking — catches gitignored worktree files (cycle-92 defect mode)
git ls-files --error-unmatch "$path" >/dev/null 2>&1 \
  || { echo "RED: $path untracked by git"; exit 1; }
```

A file that passes `[ -f ]` but fails `git ls-files --error-unmatch` is
gitignored. It exists in the worktree but will be silently dropped at ship.
This is the cycle-92 failure mode (`.evolve/profiles/AGENTS.md` gitignored
by `.gitignore:29`). Bare `[ -f ]` cannot detect this; the dual-check can.

## WORKTREE_PATH resolution

Predicates run from both the main tree and from per-cycle git worktrees.
Always resolve the repo root dynamically:

```bash
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
cd "$REPO_ROOT" || { echo "RED: not in a git tree"; exit 1; }
```

Never hardcode absolute paths. Use paths relative to `REPO_ROOT`.

## Regression-suite promotion policy

After a cycle ships GREEN:

1. Copy `acs/cycle-N/*.sh` to `acs/regression-suite/cycle-N/`.
2. The regression suite is the permanent accumulating baseline.
3. `scripts/lifecycle/run-regression-suite-slice.sh` runs a reachability-filtered
   subset of the regression suite before Builder writes `build-report.md`.
4. A RED result in the regression slice BLOCKS the build report. Fix the
   regression before writing the report.

Promotion is done in a post-ship hygiene step, not by Builder during the cycle.

## Bash 3.2 compliance

All predicate scripts MUST be bash 3.2 compatible:

- No `declare -A` (bash 4+ associative arrays)
- No `mapfile` / `readarray`
- No `${var^^}` / `${var,,}` case conversion
- No GNU `sed -i ''` (use `tmp.$$` + `mv` for atomic rewrites)
- Avoid `grep -q … | cmd` on large streams under `set -o pipefail` (SIGPIPE risk)
- Use `printf '%s\n' "$TUPLES" | while IFS=: read -r a b; do …; done` for
  iteration over structured data (avoids bash 4+ process substitution quirks)

## Exit code convention

```
0 = GREEN (predicate satisfied)
1 = RED   (predicate violated)
```

Always emit a diagnostic `RED: <reason>` to stderr before exiting 1.
Always emit a `GREEN: <what was verified>` to stdout on exit 0.

## Naming convention

```
NNN-<slug>.sh
```

Where `NNN` is a zero-padded three-digit sequence number and `<slug>` is a
kebab-case description matching the acceptance criterion (e.g.,
`001-gitignore-profiles-md-exception.sh`).

## Related files

- `AGENTS.md` (repo root) — Core Agent Rule 9: Write meaningful tests
- `agents/evolve-tdd-engineer.md` — full predicate authoring workflow
- `scripts/lifecycle/run-regression-suite-slice.sh` — pre-handoff slice runner
- `scripts/verification/validate-predicate.sh` — tautology detection (EGPS v10.0+)
