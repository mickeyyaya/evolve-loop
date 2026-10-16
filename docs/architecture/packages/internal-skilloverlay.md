# internal/skilloverlay

## Purpose

`internal/skilloverlay` turns policy-resolved skill overlays into a prompt prefix. The policy layer (`internal/policy`, `Policy.ResolveOverlays`, keyed on phase, CLI, model and tier) decides which skills a dispatch needs. This package reads the persona bodies of those skills from disk and formats them as one prepend block. A phase agent on any CLI then starts its turn with the configured operating discipline loaded.

## Design

- **Regular code, no LLM.** The decision "which skill for which phase agent" is configuration (`policy.json` overlays). The materialization is here.
- **Compact first.** For each skill, `readSkillBody` prefers the token-economy `COMPACT.md` projection when it is present and not empty. It falls back to the full `SKILL.md`. This is the fast-class projection that the fable skill documents.
- **Order.** `Materialize` keeps the order of the names it is given. The stable order of the policy resolver is authoritative.
- **Frontmatter.** `stripFrontmatter` removes a leading YAML block (`---` line to `---` line). Only the first closing delimiter after the opening is consumed, so a markdown horizontal rule in the body stays. Content without leading frontmatter passes through. If no closing delimiter exists, the text is a body and returns unchanged.

## Invariants

- **A configured overlay is never dropped silently.** A name that is unsafe, or that has neither a readable `COMPACT.md` nor a `SKILL.md` body, is skipped and reported in `missing`. The caller then warns loudly.
- **An empty `COMPACT.md` falls back.** `readSkillBody` returns `""` only when neither file yields a non-empty body. That one sentinel drives the `missing` report.
- **`safeName` stands alone.** On the wired overlay path, skill names come straight from the compiled defaults and the `policy.json` overlays, with no registry clamp. `safeName` is the only guard against a crafted name that reads a `SKILL.md` outside `skillsDir`. A name with a path separator or a traversal segment is unsafe.
- **No names means no change.** An empty names slice returns `("", nil)`. A phase without an overlay is byte for byte the same as before the feature.
