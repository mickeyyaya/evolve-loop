# Comment history: `internal/skillcheck`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/skillcheck/commands.go:1` — above `package skillcheck`

```text
// commands.go is the second half of ADR-0040's projection: it mirrors every
// skill under skills/ into a thin commands/<name>.md slash-command stub so each
// skill surfaces in the Claude Code `/` menu as /evo:<name>. The stub filename is
// BARE (loop.md, not evo-loop.md): Claude Code derives the slash command from the
// plugin name + the file basename, so the `evo` plugin's loop.md becomes /evo:loop
// natively — matching .evolve/naming.json canonical.commandPrefix "/evo:" and the
// Skill tool id evo:loop. (An evo- filename prefix would double up to /evo:evo-loop;
// the prefix once dodged the built-in /loop on pre-namespace Claude Code, but the
// /evo: namespace makes that collision impossible, so the workaround is now dead.)
// SKILL.md stays the single source — the command carries only the skill's
// description/argument-hint (for the menu) and a body that delegates back to the
// skill. Generated stubs carry a marker so the projection can detect drift and
// reap orphans (including the legacy evo-*.md stubs) without ever touching a
// hand-authored command.
```

### `go/internal/skillcheck/skillcheck.go:1` — above `package skillcheck`

```text
// Package skillcheck is the reusable projection half of ADR-0040: it renders
// the marker-delimited GENERATED:phase-facts region of each phase SKILL.md from
// its SSOTs (phase registry, phasecontract headings, dispatch profiles) and
// either writes it (generate) or reports drift (check).
//
// It was extracted from cmd/evolve so BOTH the `evolve skills` CLI AND the
// autonomous cycle's audit phase can run the SAME drift check in-process —
// without the audit (an internal package) importing package main. Run preserves
// the exact CLI behavior; Check is the pure, print-free gate the audit calls.
```

### `go/internal/skillcheck/skillcheck.go:61` — above `type skillFacts struct {`

```text
// skillFacts is the template payload — every field traces to exactly one SSOT
// (see the table in ADR-0040 §2).
```

### `go/internal/skillcheck/skillcheck.go:181` — above `cmdDiffs, cmdErr := commandDiffs(projectRoot)`

```text
// Command-stub projection (ADR-0040 second surface): mirror every skill into
// commands/<name>.md so /evo:<name> appears in the Claude Code `/` menu.
```

### `go/internal/skillcheck/skillcheck.go:306` — above `func nameMismatches(projectRoot string) []string {`

```text
// nameMismatches enforces the ADR-0040 naming rule: every skill dir's SKILL.md
// frontmatter `name` must equal the directory name. Returns one human message
// per violation (empty when all match) — pure, so both Run and Check decide
// what to do with them.
```
