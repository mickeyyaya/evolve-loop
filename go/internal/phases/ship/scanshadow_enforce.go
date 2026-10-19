package ship

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var packageListFlags = map[string]bool{"-timeout": true, "-run": true, "-tags": true, "-p": true, "-parallel": true}

func splitPackages(args []string) (flagArgs, patterns []string) {
	flagArgs = []string{args[0]}
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch {
		case packageListFlags[a] && i+1 < len(args):
			flagArgs = append(flagArgs, a, args[i+1])
			i++
		case strings.HasPrefix(a, "-"):
			flagArgs = append(flagArgs, a)
		default:
			patterns = append(patterns, a)
		}
	}
	return flagArgs, patterns
}

func (s *scanShadow) prune(ctx context.Context, moduleDir string, args []string) (run, skipped []string) {
	run = args
	if s == nil || !s.enforce || s.treeID == "" || s.goVersion == "" {
		return run, nil
	}
	defer s.guard("prune")
	flags, tags := splitTestArgs(args)
	flagArgs, patterns := splitPackages(args)
	if len(tags) > 0 || len(patterns) == 0 {
		return run, nil
	}
	listed, err := sysexec.Output(ctx, s.run, moduleDir, "go", append([]string{"list"}, patterns...)...)
	if err != nil {
		s.degrade("go list for the skip: " + err.Error())
		return run, nil
	}
	var kept, hits, fulls []string
	for _, pkg := range strings.Fields(listed) {
		if full, ok := s.covered(pkgResult{Package: pkg, Passed: true, Flags: flags}); ok {
			hits, fulls = append(hits, pkg), append(fulls, full)
			continue
		}
		kept = append(kept, pkg)
	}
	if len(hits) == 0 {
		return run, nil
	}
	s.commitSkips(hits, fulls)
	if len(kept) == 0 {
		return nil, hits
	}
	return append(flagArgs, kept...), hits
}

func (s *scanShadow) covered(r pkgResult) (string, bool) {
	if !cacheable(r) {
		return "", false
	}
	key, err := s.keyOf(r)
	if err != nil {
		s.degrade("key for the skip: " + err.Error())
		return "", false
	}
	_, hit := s.store.Lookup(key.File, nil)
	return key.Full, hit
}

func (s *scanShadow) commitSkips(hits, fulls []string) {
	s.packages += len(hits)
	s.skipped += len(hits)
	s.hits = append(s.hits, hits...)
	for _, full := range fulls {
		s.settled[full] = true
	}
}

func (s *scanShadow) note(w io.Writer) {
	if s == nil {
		return
	}
	line := fmt.Sprintf("[%s] verdict store: recorded %d green package(s) under gate %s; %d uncacheable, %d unkeyed, %d store error(s)",
		s.gate, s.recorded, s.gate, s.uncacheable, s.unkeyed, s.storeErrors)
	if s.degraded != "" {
		line += "; degraded: " + s.degraded
	}
	fmt.Fprintln(w, line)
}
