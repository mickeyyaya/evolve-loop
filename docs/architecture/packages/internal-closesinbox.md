# go/internal/closesinbox

Plan: [architecture-decisions-2026-11](../../plans/architecture-decisions-2026-11.md), component AD2-1.

## Purpose

The package reads the inbox trailers of a build report or a commit message. It is one home for the grammar, so a reader of a trailer does not parse the line again.

## The grammar

A marker line starts at column zero, after one optional markdown bullet (`-`, `*` or `+`). The marker can be in any case. A line inside a code fence does not count.

| Line | Meaning | Reader |
|---|---|---|
| `Closes-Inbox: <id>[, <id>]` | Closes the items. | `IDs` |
| `Closes-Inbox: none` | An explicit empty set. `none` is never an id, in any case, alone or in a list. An id that starts with `none` stays an id. | `IDs`, `Declares` |
| `Refs-Inbox: <id>[, <id>]` | Links partial work, and closes nothing. | `RefIDs` |

## Declares

`Declares` is true when a marker line holds at least one valid token, an id or `none`. A blank line, a placeholder such as `<id>` and prose do not count. So a later check can tell "declared nothing" from "forgot the trailer".

No caller reads `RefIDs` or `Declares` yet. Component AD2-3 adds the first.
