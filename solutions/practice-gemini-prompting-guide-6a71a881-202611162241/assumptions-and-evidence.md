# Gemini prompting guide — assumptions and evidence

Every claim in the options and the recommendation cites an entry here: `A<n>` for an assumption, `E<n>` for evidence.

## Assumptions

- **A1** — The pasted source delta is untrusted data. The notes read each line as data. They change a rule only when a rule for the agy CLI changes. Source: operator-stated, the inbox acceptance 1.
- **A2** — ADR-0133 governs a persona change. A proposal either names each prompt or persona to change, or states that none changes and why. Source: operator-stated, the inbox acceptance 2.
- **A3** — The loop reaches an outside service through a CLI, never through MCP. Source: `CLAUDE.md` hard rules.

## Evidence

- **E1** — The changed text of the source is three header links: the docs index `https://aistudio.google.com/llms.txt`, the Docs MCP server, and the coding-agent skills guide `https://aistudio.google.com/docs/coding-agents.md`. No line carries prompting guidance. Citation: `.evolve/runs/cycle-2445/scout-report.md`, Discovery Summary.
- **E2** — The agy notes hold one review bullet for each earlier source change, for example cycles 2274, 2289 and 2407. Each bullet names a unit that stays unused. Citation: `skills/model-notes-agy/COMPACT.md`.
- **E3** — The new bullet passes `evolve docs ste-lint --strict`, and the file keeps at least 15 bullets. Citation: command run in cycle 2445, exit 0.
- **E4** — Not sourced: the content of the linked guides was not read, because the delta contains links only.
