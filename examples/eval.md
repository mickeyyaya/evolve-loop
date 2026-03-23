# Eval: add-dark-mode

Example eval definition created by the Planner in Phase 2. The Eval Runner executes these checks in Phase 5.5 as a hard gate before deploy.

## Code Graders (bash commands that must exit 0)
- `npm test -- --grep "dark mode"`
- `npm test -- --grep "theme"`
- `npx tsc --noEmit`

## Regression Evals (full test suite)
- `npm test`
- `npx playwright test`

## Acceptance Checks (manual verification commands)
- `grep -r "dark\|theme" src/styles/ | head -5`
- `grep -r "useTheme\|ThemeProvider" src/ | head -5`
- `npm run build`

## Thresholds
- Code graders: pass@1 = 1.0 (all must pass)
- Regression: pass@1 = 1.0 (all must pass)
- Acceptance: pass@1 = 1.0 (all must pass)
