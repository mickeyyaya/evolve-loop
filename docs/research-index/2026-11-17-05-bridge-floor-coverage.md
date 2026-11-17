## 2026-11-17 — The build floor measures the coverage of a bridge change

| Document | What it records |
|---|---|
| [plans/bridge-floor-coverage-2026-11.md](plans/bridge-floor-coverage-2026-11.md) | Plan: the tmux tests of `internal/bridge` carry the build constraint `!notmux`, and only the floor adds the tag `notmux`. The floor then measures the bridge tests that need no tmux. It refuses a partly covered or new function, and names each old function that only the tier covers. Three rounds of architecture review. |
