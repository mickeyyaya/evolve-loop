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
- **Convention matching**: the codebase's naming, error style and file layout win over your preferences — always. Style changes are their own commits, or nothing.
- **Size caps (project rule, RIGID)**: a function ≤ 50 lines, a file ≤ 800 lines, nesting ≤ 4. A new or changed function over a cap is a defect; a touched function already over one shrinks in the same change or the change names why not (the Boy Scout rule).

## Functions (RIGID — *Clean Code* ch. 3)

- **Do one thing, at one level of abstraction.** A function that parses, decides, mutates and logs is four functions; read top-down, each call descends one level (the stepdown rule).
- **Arguments**: three at most; more is a parameter object. **No flag arguments**: a bool that selects between two behaviors is two functions, or a Strategy.
- **Command-query separation**: a function either changes state or answers a question. A query that deletes, refreshes or caches as a side effect is a defect; so is a name that hides what the function does.
- **Law of Demeter at boundaries**: talk to collaborators, not their internals (`a.B().C().D()` across packages is a missing method).
- **No dead code**: an unexported symbol nothing calls, a parameter nothing reads and an unreachable branch are deleted in the change that orphans them.

## Comments (RIGID)

New and changed code carries no comments; the rule and its few machine-read exceptions live in [docs/conventions/code-comments.md](../../../docs/conventions/code-comments.md). The fix for a comment is always a better name, a type, an extracted function, a test or a line in the package's design notes — never a reworded comment.

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
