// Package component holds component-tier tests: several real internal/
// adapters wired together against temp-dir filesystem state via the
// fixtures harness, with no CLI subagent and no git subprocess. They run in
// the fast default suite (no build tag). See go/docs/testing.md for the
// two-axis (cost × granularity) tier model.
package component
