# internal/reviewcheck

> Related: the plan `docs/plans/review-routing-2026-11.md` (component S2, decision D4) and its research `docs/research/review-routing-2026-11.md` (F2.14, F2.15). Both are on the branch `docs/review-routing-2026-11` and are not on main yet. The persona strip rule is in [internal-prompts.md](internal-prompts.md).

## Purpose

`internal/reviewcheck` holds three deterministic checks of the review routing plan. Each check returns a list of `Refusal` values. The checks replace an LLM review seat for the change classes C2, C8 and C9 of the plan.

| Check | Pure function | Git source | Verb | Class |
|---|---|---|---|---|
| `CheckInboxIntegrity` | `InboxIntegrity` | `InboxIntegrityAt` | `evolve inbox integrity --base B [--json]` | C2 |
| `CheckPersonaStrip` | `PersonaStrip` | none (the verb reads `agents/`) | `evolve persona strip-check [--json]` | C8 |
| `CheckPermissionWidening` | `PermissionWidening` | `PermissionWideningAt` | `evolve permissions trace --base B [--json]` | C9 |

## Design

- **Two layers.** Each pure function reads only the values that it gets, so a test states a case as data. Each `*At` function reads git over a `Range` (a base and a head), then calls the pure function. The verbs in `go/cmd/evolve/cmd_reviewcheck.go` give the head `HEAD`. The commit gate of plan S4 can give another head.
- **Inbox integrity.** A pending item is a `*.json` file directly in `inboxstamps.Pathspec`. The check refuses a pending item of the base that is gone at the head. Each of these facts accounts for the item:
  - a path at the head has the same file name (a consume, a claim, a quarantine or a retry keeps the name);
  - git reports a rename of the item anywhere in the tree;
  - a commit of the range has a `Closes-Inbox` line that names the item id or the file stem. The parser is `inboxmover.ClosesInboxIDs`, and the item id is `inboxbatch.ItemID`.
- **Persona strip.** The check calls `prompts.StrippedDirectives` for each persona. A diff reviewer persona (`agents/evolve-diff-*.md`) also gets a verdict check. A line is a verdict line when `reportdoc.Verdict` reads a verdict from that line alone. The check blanks the other verdict lines and refuses the line when `reportdoc.Verdict` then reads nothing. Thus the fence rules of the verdict parser are the only fence rules.
- **Permission widening trace.** Each surface decodes a file through the type that owns it. A profile in `.evolve/profiles/` decodes into `profiles.Profile`, and a manifest in `go/internal/bridge/manifests/` decodes into `bridge.Manifest`. The struct tags are the one home of the decoding. The labels in `grantFields` are the field words of the trailer and of the refusal.
- **A widening is a new entry.** The `Grants` value holds `allowed_tools`, `sandbox.write_subpaths` and `env_passthrough`. An entry in a head list that is not in the base list is a widening. A new file widens each of its entries.
- **A widening names its consumer in a trailer.** The trailer is `Widening-consumer: <file> <field> <consumer path> <entry>`. Spaces or tabs separate the first three words. The entry is the rest of the line, so an entry can hold a space. The consumer path must be in the tree at the head.
- **The base and the head resolve first.** `git rev-parse --verify --end-of-options <rev>^{commit}` gives each commit that the later queries use. A value that starts with `-` is therefore never a git option. Each query sets `core.quotePath=false`, so a path with a non-ASCII name matches.
- **A rename is a move.** The inbox check reads the renames of the whole tree, so a move out of the inbox under a new name is not a deletion. The permission trace reads a renamed file at its old path at the base.

## Invariants

- **The stage is shadow.** Each verb prints its refusals and exits 0. The JSON output has `"stage": "shadow"` and a `refusals` list that is never null. Plan component S4 adds the policy key `review_routing.stage`, and the commit gate reads the refusals at `enforce`.
- **A read fault is an error, not a refusal.** A bad base or an unparseable JSON file gives exit 2. A refusal is a fact about the change, and a fault is a fact about the tool.
- **A deleted file never widens.** A narrowed list never widens. An empty file has no list.
- **Two limits.** The trace does not expand an `$include_policy:` entry, so a wider policy file is not traced. Git quotes a path that holds a `"` or a backslash, so the trace skips such a file.
- Pinned by `inbox_test.go`, `persona_test.go`, `widening_test.go`, `gitsource_test.go` and `go/cmd/evolve/cmd_reviewcheck_test.go`.

## Findings

- **2026-11-03 inbox deletion (F2.15).** A code-reviewer seat passed an inbox commit that deleted a pending item with no consumed copy. `TestInboxIntegrity_RefusesThe20261103DeletionWithNoConsumedCopy` replays the case. Over the 238 commits of a cloud clone on 2026-11-03, the verb removed 27 pending items and gave 0 refusals.
- **Hidden verdict lines.** On 2026-11-03 `evolve persona strip-check` refuses 4 personas: `evolve-diff-architecture-reviewer.md:268`, `evolve-diff-code-reviewer.md:71`, `evolve-diff-docs-review.md:27` and `evolve-diff-go-reviewer.md:56`. In each, the verdict parser cannot see the `Verdict:` line, because the line is inside a code fence. The refusal names the line and the parser. The persona change is class C8, so it is not part of this package.
- **The first design had its own fence parser.** The architecture review of 2026-11-03 found that it disagreed with `reportdoc` on an indented fence. The check now asks `reportdoc.Verdict`, because `reportdoc` is protected and keeps its scanner private.
