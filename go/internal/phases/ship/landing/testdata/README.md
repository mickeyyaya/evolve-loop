# Unit 07 goldens (ADR-0103 — the ship landing)

Captured on `8e8f080f` (2026-09-14) by a throwaway full-argv recorder BEFORE any
code moved, then deleted; the JSON goldens carry `captured_at` in their header
and `ship-binding.golden.json` is the writer's exact bytes. Both the host pins
(`internal/phases/ship/landing_pins_test.go`) and the leaf's own tests replay
these files. Never regenerate silently: a missing golden fails the pin.

- `landing_integrate.golden.json` — reset ok/fail × merge ok/diverged × fleet on/off
- `push_direct|worktree|pushonly.golden.json` — the three push sites × the repair matrix
- `ship-binding.golden.json` — the binding writer's bytes for a fixed body
