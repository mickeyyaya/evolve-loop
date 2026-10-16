package opscmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

type leaseFile struct {
	Paths     []string `json:"paths"`
	ExpiresAt string   `json:"expires_at"`
	Reason    string   `json:"reason,omitempty"`
}

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

func parseConsoleLeaseArgs(args []string, stderr io.Writer) (ttl time.Duration, reason, projectRoot string, clear bool, rawPaths []string, code int, done bool) {
	fs := flag.NewFlagSet("evolve console-lease", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.DurationVar(&ttl, "ttl", 30*time.Minute, "lease lifetime (hard expiry; guard re-arms automatically)")
	fs.StringVar(&reason, "reason", "", "why the operator is touching the runtime tree")
	fs.StringVar(&projectRoot, "project-root", ".", "runtime plane root the lease applies to")
	fs.BoolVar(&clear, "clear", false, "remove the active lease")
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return ttl, reason, projectRoot, clear, nil, 10, true
		}
		if fs.NArg() == 0 {
			break
		}
		rawPaths = append(rawPaths, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	return ttl, reason, projectRoot, clear, rawPaths, 0, false
}

func clearConsoleLease(leasePath string, rawPaths []string, stdout, stderr io.Writer) int {
	if len(rawPaths) > 0 {
		fmt.Fprintln(stderr, "evolve console-lease: --clear takes no paths (it removes the whole lease)")
		return exitUsage
	}
	if err := os.Remove(leasePath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "evolve console-lease: clear: %v\n", err)
		return exitFail
	}
	fmt.Fprintln(stdout, "[console-lease] cleared — tree-diff guard fully armed")
	return exitOK
}

var (
	leaseMarshalIndent = json.MarshalIndent
	leaseCloseTemp     = (*os.File).Close
)

func writeConsoleLease(leasePath string, paths []string, reason string, ttl time.Duration, stdout, stderr io.Writer) int {
	expires := time.Now().Add(ttl).UTC()
	b, err := leaseMarshalIndent(leaseFile{
		Paths:     paths,
		ExpiresAt: expires.Format(time.RFC3339),
		Reason:    reason,
	}, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return exitFail
	}
	tmp, err := os.CreateTemp(filepath.Dir(leasePath), "console-lease-*.tmp")
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return exitFail
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return exitFail
	}
	if err := leaseCloseTemp(tmp); err != nil {
		_ = os.Remove(tmp.Name())
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return exitFail
	}
	if err := os.Rename(tmp.Name(), leasePath); err != nil {
		_ = os.Remove(tmp.Name())
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return exitFail
	}
	fmt.Fprintf(stdout, "[console-lease] %d path(s) leased until %s at %s — every waived leak still WARNs in the cycle log; adoption is at the NEXT cycle start\n", len(paths), expires.Format(time.RFC3339), leasePath)
	return exitOK
}

func RunConsoleLease(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	ttl, reason, projectRoot, clear, rawPaths, code, done := parseConsoleLeaseArgs(args, stderr)
	if done {
		return code
	}
	leasePath, err := hubLeasePath(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return exitFail
	}
	if clear {
		return clearConsoleLease(leasePath, rawPaths, stdout, stderr)
	}
	paths, err := normalizeLeasePaths(projectRoot, rawPaths)
	if err != nil {
		fmt.Fprintf(stderr, "evolve console-lease: %v\n", err)
		return exitUsage
	}
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "evolve console-lease: at least one exact repo-relative path is required (or --clear)")
		return exitUsage
	}
	return writeConsoleLease(leasePath, paths, reason, ttl, stdout, stderr)
}
