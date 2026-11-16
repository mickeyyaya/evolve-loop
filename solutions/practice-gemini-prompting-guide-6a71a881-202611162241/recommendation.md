# Recommendation — Gemini prompting guide

Citations: [assumptions-and-evidence.md](assumptions-and-evidence.md).

## Options Compared

| Criterion | 1 — review note | 2 — persona rewrite | 3 — Docs MCP server |
|---|---|---|---|
| Reads the source as data (A1) | yes | no, it acts on guidance that the delta lacks (E1) | no |
| States the ADR-0133 verdict (A2) | yes, none changes | no, it changes many | n/a |
| Keeps the CLI rule (A3) | yes | yes | no |
| Cost | one file (E3) | several files and cycles | several cycles |

## Recommendation

**Winner: Option 1**, built in this cycle. The delta is three header links and no prompting guidance (E1). So no persona changes under ADR-0133, and the note records the two CLI-reachable URLs and keeps MCP unused (A2, A3, E2).

**Runner-up: Option 2.** It applies only when a later source change carries a real prompting rule.

**Evidence that can flip the choice:** a source line that changes prompt format, length or structure for Gemini. No source for the linked guides exists here (E4).
