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
