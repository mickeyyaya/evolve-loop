// Package fixability sorts the findings of a failed audit into lane findings and hand-off findings, and finds a hand-off finding that blocks, so a cycle stops before a repair round that cannot converge (ADR-0158 decisions 1 to 6).
// See docs/architecture/packages/internal-fixability.md.
package fixability
