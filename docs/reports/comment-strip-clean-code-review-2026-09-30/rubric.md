# Clean Code review of stripped files — rubric

You review Go files AFTER every comment was removed by `commentaudit strip` (only tool directives and one short package doc per package remain). The tree is the git worktree
/private/tmp/claude-501/-Users-danleemh-ai-claude-evolve-loop/51fe9c6f-9ce9-42f3-a9aa-7c2a399a1d0d/scratchpad/strip-trial
(paths below are relative to it). Read each file as it is now, and read what it lost with
`git -C <worktree> diff -- <path>` (the removed lines are the comments). Judge by Robert C. Martin, *Clean Code*:

- **Names (ch. 2; smells N1–N7):** intention-revealing, unambiguous, no encodings or mental mapping, length fits scope, a name says every side effect.
- **Functions (ch. 3; F1–F4):** small and doing one thing, one level of abstraction, at most 3 parameters, no flag or output arguments, no hidden side effects, command–query separation.
- **General smells (ch. 17):** G5 duplication, G16 obscured intent, G19 explanatory variables missing, G20 a name that does not say what the function does, G25 magic numbers (named constants), G28 conditionals not encapsulated, G29 negative conditionals, G31 hidden temporal coupling, G34 mixed abstraction levels.
- **Errors (ch. 7):** errors carry context, nothing swallowed, no silent fail-open.
- **What each removed comment carried (ch. 4, "explain yourself in code").** Classify every removed comment group:
  - `REDUNDANT` — the code already says it; nothing lost.
  - `RECOVERABLE` — intent the code does not show; give the exact refactor (rename X→Y, extract `func z()`, named constant `c = …`, explanatory variable).
  - `INVARIANT` — a *why* or warning of consequences only a test can keep; name the test to write (`TestXxx_…`).
  - `DESIGN` — design knowledge that belongs in `docs/architecture/packages/<pkg>.md`.
  - `HISTORY` — incident, cycle or date narrative; kept by the history archive, loss acceptable.

## Output (write it to the file named in your task, in exactly this shape)

```
## <path>
verdict: CLEAN | MINOR | NEEDS-REFACTOR
readability: <1-5>   (5 = reads fully without its comments)
removed: REDUNDANT=<n> RECOVERABLE=<n> INVARIANT=<n> DESIGN=<n> HISTORY=<n>
| line | smell | finding | refactor |
|---|---|---|---|
| <line in the stripped file> | <N1/F3/G25/ch4-RECOVERABLE/…> | <what is unclear now> | <the concrete change> |
```

List only findings that matter to a reader; no style nits gofmt already settles. Be concrete: every refactor names the new identifier or test. READ-ONLY: never run git stash, checkout, reset, add, commit, or anything that edits files; do not run test suites.
