# Clean Code — the rules that survived measurement

> Load when: writing or reviewing production code. 2025-26 evidence retired some classics and hardened others; this file keeps only what held up (GitClear 2026 duplication data, OpenAI's internal diff-budget policy, comment-density studies of AI code).

## Duplication — the #1 measured AI pathology (RIGID)

AI-assisted codebases show **+81% duplicated blocks since 2023** with refactored lines collapsing to 3.8%. Countermeasures, in force here:
1. **Search-before-write** (Iron Law 3): grep for the function/type/constant before creating it; extending an existing helper beats a parallel one.
2. Copy-adapt is allowed exactly **twice**; the third occurrence pays for the extraction (rule-of-three). Below three, prefer visible repetition over a premature abstraction — but leave a note at the second copy.
3. Single-source with projection: when the same fact must exist in two artifacts (constant + doc, schema + example), one is generated from the other or a drift test pins them together. Two hand-maintained copies WILL diverge.

## Reviewability is the metric (RIGID)

The reader's cost, not the writer's taste, decides:
- **Diff budgets**: ~500 lines for complex changes, ~800 for mechanical ones; past that, split the PR. One concern per change — a fix, a refactor, and a rename are three changes.
- **Scope contract in the description**: goal + explicit non-goals + blast radius. The reviewer verifies against it; anything outside it is scope creep to remove.
- **Convention matching**: the codebase's naming, error style, file layout, and comment density win over your preferences — always. Style changes are their own commits, or nothing.
- Functions stay small enough to hold in one read (the *small* finding survived scrutiny; exact line-count caps did not). Nesting ≤4 remains a good forcing function for extraction.

## Comments (RIGID)

Comments answer **why** — a constraint the code cannot express: an invariant, an external contract, a deliberate deviation, a tooling requirement ("explicitly typed so the identifier appears in the test AST"). Never *what* the next line does, never narration of your editing process ("added to fix review"), never restating the diff. AI-generated narration slop is a known review burden — strip it before shipping. Delete stale comments as part of the change that stales them; a wrong comment is worse than none.

## Errors (RIGID)

- Handle or propagate — never swallow. `_ = err` needs a comment proving why ignoring is correct.
- Wrap with context on the way up: `fmt.Errorf("seal cycle %d: %w", id, err)` — the reader at the top must be able to locate the failure without a debugger.
- Fail-open only with a WARN and a justification comment; silent fail-open is a dormant outage (a gate that no-ops on missing input will no-op forever — make missing input loud).
- Validate at boundaries (input parsing, config load, IPC); trust internally past the boundary rather than re-validating everywhere.

## State & data (FLEXIBLE — strong defaults)

- Immutability first: return new values instead of mutating arguments; mutation is an optimization taken knowingly, locally, and documented at the seam.
- Constructors/composition-root wire dependencies (DI); package-level mutable state needs an extraordinary reason.
- Name by role and domain, not type (`ownerLive`, not `boolFlag`); `is/has` for booleans; exported names read as API, so they get the naming care.

## The AI-specific hygiene list (RIGID)

- No unnecessary guard clauses: every nil-check/bounds-check corresponds to a caller that can produce it or a test that pins the boundary. (~2× over-guarding measured in agent commits — it obscures real invariants.)
- No dead parameters, no speculative config knobs, no "for future use" fields — YAGNI is enforced, see `minimalism`.
- No new env flags; behavior differences ride config/policy/DI (project standing rule).
- Generated/vendored/binary artifacts never enter a source commit (staging guard territory; check `git status` before finishing).
