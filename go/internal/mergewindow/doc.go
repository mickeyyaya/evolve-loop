// Package mergewindow decides whether a PR can merge in a window while a wave runs, or must wait for a boundary. It reads the path classes from the routing table of .github/workflows/required.yml, so CI routing and the merge window share one rule. See docs/architecture/packages/internal-mergewindow.md.
package mergewindow
