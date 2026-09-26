package commentaudit

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Git is the repository view verify needs; Show returns fs.ErrNotExist for a
// path absent at base.
type Git interface {
	ChangedFiles(base string) ([]string, error)
	Show(base, path string) ([]byte, error)
	Root() (string, error)
}

const usage = "usage: commentaudit rank [-n N] [dir] | commentaudit verify|check -base <ref> [dir ...]"

// Main runs the CLI and returns its exit code: 0 ok, 1 violations, 2 usage.
func Main(args []string, stdout, stderr io.Writer, git Git) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "rank":
		return rank(args[1:], stdout, stderr)
	case "check":
		return check(args[1:], stdout, stderr, git)
	case "verify":
		return verify(args[1:], stdout, stderr, git)
	}
	fmt.Fprintln(stderr, usage)
	return 2
}

func rank(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("rank", flag.ContinueOnError)
	fl.SetOutput(stderr)
	top := fl.Int("n", 40, "packages to list")
	if fl.Parse(args) != nil {
		return 2
	}
	root := "."
	if fl.NArg() > 0 {
		root = fl.Arg(0)
	}
	ranked, err := Rank(os.DirFS(root))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "narrative\tcomment\tcode\tfiles\tdir")
	for i, p := range ranked {
		if i == *top {
			break
		}
		fmt.Fprintf(stdout, "%d\t%d\t%d\t%d\t%s\n", p.Stats.Narrative, p.Stats.Comment, p.Stats.Code, p.Files, p.Dir)
	}
	return 0
}

// diff is a change set: the Go files changed since base and how to read each
// side of them.
type diff struct {
	files         []string
	before, after func(string) ([]byte, error)
}

func loadDiff(name string, args []string, stderr io.Writer, git Git) (diff, int) {
	fl := flag.NewFlagSet(name, flag.ContinueOnError)
	fl.SetOutput(stderr)
	base := fl.String("base", "", "git ref the change started from")
	if fl.Parse(args) != nil || *base == "" {
		fmt.Fprintln(stderr, usage)
		return diff{}, 2
	}
	changed, err := git.ChangedFiles(*base)
	var root string
	if err == nil {
		root, err = git.Root()
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return diff{}, 1
	}
	scope, err := scopeDirs(root, fl.Args())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return diff{}, 1
	}
	var goFiles []string
	for _, f := range changed {
		if strings.HasSuffix(f, ".go") && under(f, scope) {
			goFiles = append(goFiles, f)
		}
	}
	for i, dir := range scope {
		if !anyUnder(goFiles, dir) {
			fmt.Fprintf(stderr, "no changed Go files under %s\n", fl.Arg(i))
			return diff{}, 1
		}
	}
	return diff{
		files:  goFiles,
		before: func(p string) ([]byte, error) { return git.Show(*base, p) },
		after:  func(p string) ([]byte, error) { return os.ReadFile(filepath.Join(root, p)) },
	}, 0
}

func verify(args []string, stdout, stderr io.Writer, git Git) int {
	d, code := loadDiff("verify", args, stderr, git)
	if code != 0 {
		return code
	}
	if len(d.files) == 0 {
		fmt.Fprintln(stderr, "no changed Go files to verify")
		return 1
	}
	violations, err := VerifyChanges(d.files, d.before, d.after)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, v := range violations {
		fmt.Fprintln(stdout, v)
	}
	if len(violations) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "comment-only: %d changed Go file(s) verified\n", len(d.files))
	return 0
}

func check(args []string, stdout, stderr io.Writer, git Git) int {
	d, code := loadDiff("check", args, stderr, git)
	if code != 0 {
		return code
	}
	found := 0
	for _, f := range d.files {
		b, a, err := readBoth(d.before, d.after, f)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, line := range AddedNarrative(b, a) {
			fmt.Fprintf(stdout, "%s: %s\n", f, line)
			found++
		}
	}
	if found > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "no narrative comments added in %d changed Go file(s)\n", len(d.files))
	return 0
}

// under reports whether path lies in one of dirs; no dirs means everywhere.
func under(path string, dirs []string) bool {
	if len(dirs) == 0 {
		return true
	}
	for _, d := range dirs {
		d = strings.TrimSuffix(filepath.ToSlash(d), "/")
		if path == d || strings.HasPrefix(path, d+"/") {
			return true
		}
	}
	return false
}

// scopeDirs turns each dir argument into a repo-root-relative path: relative
// to the working directory when that exists, else relative to root.
func scopeDirs(root string, dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	var scoped []string
	for _, d := range dirs {
		abs, err := filepath.Abs(d)
		if err != nil {
			return nil, err
		}
		if resolved, statErr := filepath.EvalSymlinks(abs); statErr == nil {
			abs = resolved
		} else {
			abs = filepath.Join(realRoot, d)
		}
		rel, err := filepath.Rel(realRoot, abs)
		if err != nil {
			return nil, err
		}
		if rel == "." {
			return nil, nil
		}
		scoped = append(scoped, filepath.ToSlash(rel))
	}
	return scoped, nil
}

func anyUnder(files []string, dir string) bool {
	for _, f := range files {
		if under(f, []string{dir}) {
			return true
		}
	}
	return false
}
