# Builder Notes — Cycle 30

## Task: add-changelog-and-bump-v690

### File Fragility
- CHANGELOG.md: append-only, low fragility — prepended new block, no existing lines modified
- .claude-plugin/plugin.json: single version field changed; tightly coupled to marketplace.json — both must stay in sync
- .claude-plugin/marketplace.json: single version field changed; coupled to plugin.json above

### Approach Surprises
- None. The task was exactly as scoped: 3 targeted edits across 3 files.

### Recommendations for Scout
- plugin.json and marketplace.json must always be bumped together — consider flagging them as a pair in future version-bump tasks.
- CHANGELOG.md format is stable; future entries follow the same header + ### Added pattern.

---

# Builder Notes — Cycle 29

## Task: update-readme-docs-section

### File Fragility
- README.md: low fragility — additive changes only, no existing lines modified

### Approach Surprises
- None. The task was exactly as scoped: 3 doc entries in Project Structure + 2 feature bullets.

### Recommendations for Scout
- README docs table and Features list are now in sync with docs/ directory. Future doc tasks should update both sections together.
- Feature bullets are growing long (49 items). Consider grouping into subsections in a future cycle.

---

# Builder Notes — Cycle 28 (add-token-optimization-doc)

## Task: add-token-optimization-doc

### File Fragility
- docs/token-optimization.md: new file, zero coupling — no blast radius

### Approach Surprises
- All 8 mechanisms were fully documented in SKILL.md; no gaps requiring inference
- File came in at 95 lines — slightly over the 60-80 target but all content was required by task spec

### Recommendations for Scout
- Summary table at top provides a quick-reference anchor; future updates to any mechanism should keep the table in sync
- If token budget defaults change in state.json initialization block, this doc needs a matching update

---

# Builder Notes — Cycle 28 (add-memory-hierarchy-doc)

## Task: add-memory-hierarchy-doc

### File Fragility
- `docs/memory-hierarchy.md`: new file, low blast radius; the agent access matrix table will need updating if a new agent is added
- `memory-protocol.md`: used as source of truth — any schema changes there will make the hierarchy doc stale

### Approach Surprises
- Eval grader 3 uses lowercase grep (`episodic\|semantic\|procedural`) but content naturally uses Title Case in headers and table cells. Required adding one lowercase prose sentence to reach the minimum line count of 3. For future doc tasks: always check grep case sensitivity against actual content before writing.

### Recommendations for Scout
- The 70-90 line target was too tight for a doc with 6 layers + access matrix + mailbox example. Future architecture guide tasks should specify ~100-150 lines.
- `docs/memory-hierarchy.md` and `docs/self-learning.md` are complementary; a follow-up cross-reference pass could link them (e.g., self-learning.md references Layer 4 instinct graduation pathway).
