package opscmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

var absPath = filepath.Abs

func runDoctorPlane(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve doctor plane", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var repair bool
	fs.BoolVar(&repair, "repair", false, "repair broken worktree links")
	if err := fs.Parse(cmdutil.ReorderArgs(args)); err != nil || fs.NArg() > 1 {
		fmt.Fprintln(stderr, "evolve doctor plane: usage: evolve doctor plane [root] [--repair]")
		return exitUsage
	}
	root := "."
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}
	abs, err := absPath(root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor plane: %v\n", err)
		return exitFail
	}
	info, err := plane.Classify(abs)
	if err != nil {
		fmt.Fprintf(stderr, "evolve doctor plane: %v\n", err)
		return exitFail
	}
	if repair {
		return repairDoctorPlane(abs, info, stdout, stderr)
	}
	printDoctorPlaneInfo(stdout, info)
	if _, serr := os.Stat(filepath.Join(abs, ".evolve", "state.json")); serr == nil && !info.IsLinkedWorktree {
		fmt.Fprintln(stderr, "WARN: loop state lives in the PRIMARY checkout — operator activity here can kill lanes; migrate the loop to a dedicated runtime worktree (ADR-0080)")
		return exitRuntime
	}
	_, store := findHubStore(abs)
	if store == "" {
		return exitOK
	}
	broken := brokenHubWorktrees(filepath.Dir(store), store)
	for _, wt := range broken {
		fmt.Fprintf(stdout, "broken: %s\n", wt)
	}
	if len(broken) > 0 {
		return exitFail
	}
	return exitOK
}

func printDoctorPlaneInfo(stdout io.Writer, info plane.Info) {
	kind := "PRIMARY checkout (console plane)"
	if info.IsLinkedWorktree {
		kind = "linked worktree (runtime plane)"
	}
	branch := info.Branch
	if branch == "" {
		branch = "(detached)"
	}
	fmt.Fprintf(stdout, "plane: %s\nbranch: %s\ngitdir: %s\n", kind, branch, info.GitDir)
}

func repairDoctorPlane(abs string, info plane.Info, stdout, stderr io.Writer) int {
	hubRoot, store := findHubStore(abs)
	if store == "" {
		printDoctorPlaneInfo(stdout, info)
		return exitOK
	}
	broken := brokenHubWorktrees(hubRoot, store)
	if len(broken) > 0 {
		g := gitexec.Default(hubRoot)
		argv := append([]string{"--git-dir=" + store, "worktree", "repair"}, broken...)
		_, gerr, code, err := g.Capture(context.Background(), argv...)
		if err != nil || code != 0 {
			fmt.Fprintf(stderr, "evolve doctor plane: %s\n", strings.TrimSpace(gerr))
			return exitRuntime
		}
		if refreshed, rerr := plane.Classify(abs); rerr == nil {
			info = refreshed
		}
	}
	printDoctorPlaneInfo(stdout, info)
	for _, wt := range broken {
		fmt.Fprintf(stdout, "repaired: %s\n", wt)
	}
	return exitOK
}

func findHubStore(abs string) (string, string) {
	dir := abs
	for {
		candidate := filepath.Join(dir, ".repo.git")
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return dir, candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
		dir = parent
	}
}

func brokenHubWorktrees(hubRoot, store string) []string {
	entries, _ := os.ReadDir(filepath.Join(store, "worktrees"))
	var broken []string
	for _, entry := range entries {
		name := entry.Name()
		recordDir := filepath.Join(store, "worktrees", name)
		raw, err := os.ReadFile(filepath.Join(recordDir, "gitdir"))
		if err != nil {
			continue
		}
		recDotGit := strings.TrimSpace(string(raw))
		wtPath, gitDir := resolveStoreWorktree(hubRoot, store, name, filepath.Dir(recDotGit))
		if wtPath == "" {
			continue
		}
		if sameCleanPath(gitDir) != sameCleanPath(recordDir) ||
			sameCleanPath(recDotGit) != sameCleanPath(filepath.Join(wtPath, ".git")) {
			broken = append(broken, wtPath)
		}
	}
	sort.Strings(broken)
	return broken
}

func resolveStoreWorktree(hubRoot, store, name, recWt string) (string, string) {
	if gitDir := worktreeRecordGitDir(recWt, store, name); gitDir != "" {
		return recWt, gitDir
	}
	for rel := recWt; ; {
		idx := strings.IndexByte(rel, filepath.Separator)
		if idx < 0 {
			break
		}
		rel = rel[idx+1:]
		cand := filepath.Join(hubRoot, rel)
		if gitDir := worktreeRecordGitDir(cand, store, name); gitDir != "" {
			return cand, gitDir
		}
	}
	parent := filepath.Dir(recWt)
	siblings, _ := os.ReadDir(parent)
	for _, sib := range siblings {
		cand := filepath.Join(parent, sib.Name())
		if gitDir := worktreeRecordGitDir(cand, store, name); gitDir != "" {
			return cand, gitDir
		}
	}
	return "", ""
}

func worktreeRecordGitDir(wtPath, store, name string) string {
	info, err := plane.Classify(wtPath)
	if err != nil || !info.IsLinkedWorktree {
		return ""
	}
	worktreesDir := filepath.Dir(info.GitDir)
	storeDir := filepath.Dir(worktreesDir)
	if filepath.Base(info.GitDir) != name ||
		filepath.Base(worktreesDir) != "worktrees" ||
		filepath.Base(storeDir) != filepath.Base(store) {
		return ""
	}
	return info.GitDir
}

func sameCleanPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(p)
}
