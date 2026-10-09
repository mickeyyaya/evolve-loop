// Package wake blocks a channel reader until the kernel posts a change, a process exit or an output hangup.
// Dirs wake on entry-set changes and Files on content changes; a spurious wake is allowed and a Hangup is terminal.
// The caller reads again after every Arm and every wake, and arms again after a Changed wake on a file a rename can replace.
package wake
