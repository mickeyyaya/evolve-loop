// Package opscmd holds the `evolve` CLI handlers for release and operational
// subcommands — doctor (probe/boot/live), changelog-gen, version-bump,
// marketplace-poll, release-preflight, release-consistency, release(-pipeline),
// and rollback.
//
// HOW: each exported Run* function has the standard subcommand signature
// (args []string, stdin io.Reader, stdout, stderr io.Writer) int and is wired
// into cmd/evolve/registry.go. Handlers parse their own flags and delegate the
// real work to the internal/* packages (doctor, changeloggen, versionbump,
// marketplacepoll, releasepreflight, releaseconsistency, releasepipeline,
// rollback); shared CLI helpers come from cmd/evolve/cmdutil. No business logic
// lives here.
//
// WHY: cmd/evolve was a 77-handler package main; grouping the release/ops
// handlers into their own importable package (SRP) shrinks the composition root
// and makes them directly testable. cmd is a fan-in-0 sink, so the extraction
// carries no import-cycle risk; registry.go references opscmd.Run* instead of
// package-main-private funcs.
//
// Key exported symbols:
//   - [RunDoctor] — environment probe / boot-smoke / live-smoke
//   - [RunChangelogGen], [RunVersionBump] — changelog + atomic version bump
//   - [RunReleasePreflight], [RunReleaseConsistency], [RunReleasePipeline], [RunRollback] — release pipeline
//   - [RunMarketplacePoll] — marketplace propagation check
//
// Depends on: cmd/evolve/cmdutil + the internal/* release/ops implementations.
package opscmd
