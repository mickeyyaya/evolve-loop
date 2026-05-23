// Package detectnested ports legacy/scripts/dispatch/detect-nested-claude.sh.
//
// Probes whether evolve-loop is running inside Claude Code. Used by the
// dispatcher to auto-enable EVOLVE_SANDBOX_FALLBACK_ON_EPERM and
// EVOLVE_SKIP_WORKTREE when the parent process is itself sandboxed (the
// canonical macOS Darwin 25.4+ nested-sandbox EPERM case).
package detectnested

// Detect returns "nested" if any Claude Code env-var beacon is present,
// "standalone" otherwise. Signals (any one match → nested):
//   CLAUDECODE
//   CLAUDE_CODE_ENTRYPOINT
//   CLAUDE_CODE_EXECPATH
//
// The env reader can be overridden via opts.Env for tests; defaults to
// the real os.Getenv.
func Detect(opts Options) string {
	getEnv := opts.Env
	if getEnv == nil {
		getEnv = osGetenv
	}
	for _, name := range []string{
		"CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
		"CLAUDE_CODE_EXECPATH",
	} {
		if getEnv(name) != "" {
			return "nested"
		}
	}
	return "standalone"
}

// Options exposes the Env reader for test injection. Zero-value uses
// os.Getenv.
type Options struct {
	Env func(name string) string
}
