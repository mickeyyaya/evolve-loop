// Package shipjournal is the one home of the format of .evolve/ship-journal.jsonl: the entry, the file name, the loop landing class and the time stamp.
// The ship phase writes through it and the self-healing loop reads through it, so the writer and the readers cannot drift.
// See docs/architecture/packages/internal-shipjournal.md.
package shipjournal
