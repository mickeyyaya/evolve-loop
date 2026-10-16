// Package detectnested reports whether evolve-loop runs nested inside Claude Code, from its environment beacons.
// See docs/architecture/packages/internal-detectnested.md.
package detectnested

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

type Options struct {
	Env func(name string) string
}
