package opscmd

// console_lease.go — ADR-0080 S4 writer: `evolve console-lease <path>...
// [--ttl 30m] [--reason text] [--project-root P]` records a time-bounded,
// exact-path operator lease in the git COMMON dir (the hub — outside every
// worktree, so no lane phase can author or shadow it; review BLOCK).
// `--clear` removes it. The reader (internal/core) adopts the lease once at
// cycle start and treats absent/expired/malformed as an armed guard.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

// leaseFile mirrors internal/core's consoleLease shape (schema owned there;
// duplicated as a wire struct to keep opscmd free of a core import).
type leaseFile struct {
	Paths     []string `json:"paths"`
	ExpiresAt string   `json:"expires_at"`
	Reason    string   `json:"reason,omitempty"`
}

// hubLeasePath resolves the hub-resident lease location for a project root —
// the SAME resolution the core reader applies (plane.Classify → CommonGitDir).
func hubLeasePath(projectRoot string) (string, error) {
	info, err := plane.Classify(projectRoot)
	if err != nil {
		return "", err
	}
	common, err := plane.CommonGitDir(info)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, plane.ConsoleLeaseFileName), nil
}

// normalizeLeasePaths cleans operator input into the exact repo-relative
// slash form the guard's porcelain paths use (review MEDIUM: a `./` prefix or
// absolute path silently waived nothing while claiming success).
func normalizeLeasePaths(projectRoot string, raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if filepath.IsAbs(p) {
			return nil, fmt.Errorf("path %q is absolute — leases take exact repo-relative paths", p)
		}
		clean := filepath.ToSlash(filepath.Clean(p))
		clean = strings.TrimPrefix(clean, "./")
		if clean == "" || clean == "." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("path %q does not name a file inside the repo", p)
		}
		if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(clean))); err != nil {
			return nil, fmt.Errorf("path %q not found under %s — create the file first (a lease names the exact path that will leak; adoption happens at the NEXT cycle start)", clean, projectRoot)
		}
		out = append(out, clean)
	}
	return out, nil
}

// RunConsoleLease implements `evolve console-lease`.
func RunConsoleLease(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve console-lease", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ttl time.Duration
	var reason, projectRoot string
	var clear bool
	fs.DurationVar(&ttl, "ttl", 30*time.Minute, "lease lifetime (hard expiry; guard re-arms automatically)")
	fs.StringVar(&reason, "reason", "", "why the operator is touching the runtime tree")
	fs.StringVar(&projectRoot, "project-root", ".", "runtime plane root the lease applies to")
	fs.BoolVar(&clear, "clear", false, "remove the active lease")
	if err := fs.Parse(cmdutil.ReorderArgs(args)); err != nil {
		return 10
	}
	leasePath, err := hubLeasePath(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return 1
	}
	if clear {
		if err := os.Remove(leasePath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "evolve console-lease: clear: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "[console-lease] cleared — tree-diff guard fully armed")
		return 0
	}
	paths, err := normalizeLeasePaths(projectRoot, fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return 10
	}
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "evolve console-lease: at least one exact repo-relative path is required (or --clear)")
		return 10
	}
	expires := time.Now().Add(ttl).UTC()
	b, err := json.MarshalIndent(leaseFile{
		Paths:     paths,
		ExpiresAt: expires.Format(time.RFC3339),
		Reason:    reason,
	}, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return 1
	}
	tmp, err := os.CreateTemp(filepath.Dir(leasePath), "console-lease-*.tmp")
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return 1
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return 1
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return 1
	}
	if err := os.Rename(tmp.Name(), leasePath); err != nil {
		_ = os.Remove(tmp.Name())
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[console-lease] %d path(s) leased until %s at %s — every waived leak still WARNs in the cycle log; adoption is at the NEXT cycle start\n", len(paths), expires.Format(time.RFC3339), leasePath)
	return 0
}
