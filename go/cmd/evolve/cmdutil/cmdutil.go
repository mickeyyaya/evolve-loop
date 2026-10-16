// Package cmdutil holds tiny helpers shared by the `evolve` CLI's
// per-subcommand handlers.
package cmdutil

import (
	"flag"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func HasHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func EnvOrCwd(env string) string {
	if v := os.Getenv(env); v != "" {
		return paths.AbsoluteRoot(env, v, nil)
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func ReorderArgs(args []string) []string {
	flags := make([]string, 0, len(args))
	pos := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

func FilterEvolveEnv(environ []string) map[string]string {
	out := map[string]string{}
	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		k := kv[:eq]
		if strings.HasPrefix(k, "EVOLVE_") || strings.HasPrefix(k, "BRIDGE_") {
			out[k] = kv[eq+1:]
		}
	}
	return out
}

func NewPromptsLoader(projectRoot string) *prompts.Loader {
	if d := os.Getenv("EVOLVE_PROMPTS_DIR"); d != "" {
		return prompts.NewFromDir(d)
	}
	return prompts.NewFromDir(projectRoot)
}

func ParseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if endsOnTerminator(fs, args[:len(args)-len(rest)]) {
			return append(positional, rest...), nil
		}
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

func endsOnTerminator(fs *flag.FlagSet, parsed []string) bool {
	for i := 0; i < len(parsed); i++ {
		if parsed[i] == "--" {
			return true
		}
		if takesSeparateValue(fs, parsed[i]) {
			i++
		}
	}
	return false
}

func takesSeparateValue(fs *flag.FlagSet, token string) bool {
	f := fs.Lookup(strings.TrimLeft(token, "-"))
	if f == nil {
		return false
	}
	boolean, isBoolean := f.Value.(interface{ IsBoolFlag() bool })
	return !isBoolean || !boolean.IsBoolFlag()
}
