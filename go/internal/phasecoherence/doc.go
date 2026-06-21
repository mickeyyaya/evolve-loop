// Package phasecoherence cross-checks the paired hand-authored phase
// definitions for internal consistency — the persona markdown and the profile
// declarations that together define a phase, plus the artifacts they name, must
// agree with one another.
//
// HOW: [Check] runs the coherence checks over the provided [Options] (the agents
// and profiles filesystems); [CheckArtifactNames] verifies that each persona's
// declared output artifact (its first .md token) matches the output_artifact
// named in the paired profile. Each disagreement surfaces as a [Violation] —
// report-only; the package never edits the surfaces it inspects.
//
// WHY: a phase's behavior is configured across two hand-edited surfaces
// (persona + profile) that can silently drift apart, and a drift means an agent
// writes an artifact the pipeline never reads (or reads one the agent never
// writes). Centralizing the cross-check in one leaf lets a guard/test catch the
// drift mechanically up front instead of surfacing it as a mid-cycle failure.
//
// Key exported symbols:
//   - [Check] — run the coherence checks, returning []Violation
//   - [CheckArtifactNames] — persona-artifact ↔ profile output_artifact agreement
//   - [Violation], [Options] — the report record and the check inputs
//
// Depends on: internal/profiles, internal/prompts.
package phasecoherence
