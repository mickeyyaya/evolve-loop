package core

import "path/filepath"

// perCycleGOCACHE returns the absolute Go build-cache directory for a cycle's
// run workspace, and whether the caller should apply it.
//
// ADR-0049 N12: concurrent fleet cycles must NOT share a GOCACHE — concurrent
// `go build`/`go test` invocations racing on one cache can corrupt it
// (golang/go#43052). Pinning each cycle to <workspace>/.gocache isolates them;
// the cost is a cold cache per cycle, negligible against a multi-hour LLM cycle.
//
// go build REQUIRES an absolute GOCACHE ("GOCACHE is not an absolute path"), but
// the default --project-root "." makes WorkspacePath relative, so a relative dir
// is resolved against the process CWD. An empty workspace (worktree-less test
// cycles) or an unresolvable path returns ok=false: leave the inherited GOCACHE
// untouched rather than break the build with a relative one.
func perCycleGOCACHE(workspacePath string) (dir string, ok bool) {
	if workspacePath == "" {
		return "", false
	}
	dir = filepath.Join(workspacePath, ".gocache")
	if filepath.IsAbs(dir) {
		return dir, true
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	return abs, true
}
