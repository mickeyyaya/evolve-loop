package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writeProtectedWriteRules(b *strings.Builder, cfg Config, denyPass bool) {
	if cfg.ProtectedWriteEnforce != denyPass {
		return
	}
	for _, filter := range []struct {
		name  string
		paths []string
	}{{"subpath", cfg.ProtectedWriteSubpaths}, {"prefix", cfg.ProtectedWritePrefixes}} {
		for _, p := range filter.paths {
			if p == "" {
				continue
			}
			if cfg.ProtectedWriteEnforce {
				fmt.Fprintf(b, "(deny file-write* (%s %q))\n", filter.name, p)
			} else {
				fmt.Fprintf(b, "(allow file-write* (%s %q) (with report))\n", filter.name, p)
			}
		}
	}
}

func bwrapProtectedWriteMasks(cfg Config) []string {
	if !cfg.ProtectedWriteEnforce {
		return nil
	}
	var out []string
	for _, p := range cfg.ProtectedWriteSubpaths {
		out = appendReadOnlyBind(out, p)
	}
	for _, prefix := range cfg.ProtectedWritePrefixes {
		for _, p := range filesWithPrefix(prefix) {
			out = appendReadOnlyBind(out, p)
		}
	}
	return out
}

func appendReadOnlyBind(out []string, p string) []string {
	if _, err := os.Lstat(p); err != nil {
		return out
	}
	return append(out, "--ro-bind", p, p)
}

func filesWithPrefix(prefix string) []string {
	if prefix == "" {
		return nil
	}
	dir, base := filepath.Split(prefix)
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}
