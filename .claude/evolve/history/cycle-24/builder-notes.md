# Builder Notes — Cycle 24

## Task: add-builder-retrospective

### File Fragility
- agents/evolve-scout.md: Inputs list and incremental mode section are tightly coupled — changes to one often require matching changes to the other (added `builderNotes` to both).
- skills/evolve-loop/phases.md: Very long file (~660 lines). The Phase 1 bash pre-compute block and the Scout context JSON block are separated by ~20 lines; easy to update one and miss the other.

### Approach Surprises
- The eval grader checked for `builder-notes.md` (literal filename with extension) in the scout file, not just `builderNotes` (camelCase). Required adding the filename to the Inputs section in addition to the incremental mode bullet.

### Recommendations for Scout
- phases.md is a hotspot — 4 of the last 5 tasks have touched it. Future tasks targeting phases.md should be S-complexity to reduce blast radius.
- evolve-scout.md Inputs section and Mode-Based Discovery section are parallel mirrors — when adding a new context field, both locations need updating.
