package covchanged

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var DefaultTags = []string{"integration"}

type Options struct {
	Base     string
	Profile  string
	Tags     []string
	Packages PackageSet
}

type Workspace struct {
	ModuleDir string
	Run       sysexec.RunFunc
}

func (w Workspace) Check(ctx context.Context, opt Options) (Verdict, error) {
	git := gitexec.Git{Dir: w.ModuleDir, Exec: w.Run}
	if _, err := git.Output(ctx, "rev-parse", "--verify", "--quiet", opt.Base+"^{commit}"); err != nil {
		return Verdict{}, fmt.Errorf("covchanged: base %q is not a commit: %w", opt.Base, err)
	}
	forkPoint, err := git.Output(ctx, "merge-base", opt.Base, "HEAD")
	if err != nil {
		return Verdict{}, fmt.Errorf("covchanged: no merge base of %q and HEAD: %w", opt.Base, err)
	}
	src, err := w.sources(ctx, git, forkPoint, opt)
	if err != nil {
		return Verdict{}, err
	}
	report, err := Measure(src)
	if err != nil {
		return Verdict{}, err
	}
	return Judge(report), nil
}

func (w Workspace) sources(ctx context.Context, git gitexec.Git, base string, opt Options) (Sources, error) {
	module, err := ModulePath(w.ModuleDir)
	if err != nil {
		return Sources{}, err
	}
	changed, err := changedLines(ctx, git, base)
	if err != nil {
		return Sources{}, err
	}
	untracked, err := w.untrackedLines(ctx, git)
	if err != nil {
		return Sources{}, err
	}
	for rel, lines := range untracked {
		changed.Added[rel] = lines
	}
	info, err := os.Stat(opt.Profile)
	if err != nil {
		return Sources{}, fmt.Errorf("covchanged: read profile: %w", err)
	}
	data, err := os.ReadFile(opt.Profile)
	if err != nil {
		return Sources{}, fmt.Errorf("covchanged: read profile: %w", err)
	}
	profile, err := ParseProfile(data, module)
	if err != nil {
		return Sources{}, err
	}
	ctxt := build.Default
	ctxt.BuildTags = opt.Tags
	read := w.freshReader(opt.Profile, info.ModTime())
	return Sources{Changed: changed, Profile: profile, ReadFile: read, Build: ctxt, Packages: opt.Packages}, nil
}

func ModulePath(moduleDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("covchanged: read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", fmt.Errorf("covchanged: %s/go.mod declares no module", moduleDir)
}

func changedLines(ctx context.Context, git gitexec.Git, base string) (Diff, error) {
	out, stderr, code, err := git.Capture(ctx, "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", "--relative", base, "--", ".")
	if err != nil || code != 0 {
		return Diff{}, fmt.Errorf("covchanged: git diff %s: exit %d: %v %s", base, code, err, strings.TrimSpace(stderr))
	}
	return ParseDiff([]byte(out))
}

func (w Workspace) untrackedLines(ctx context.Context, git gitexec.Git) (AddedLines, error) {
	out, stderr, code, err := git.Capture(ctx, "ls-files", "-z", "--others", "--exclude-standard", "--", ".")
	if err != nil || code != 0 {
		return nil, fmt.Errorf("covchanged: git ls-files: exit %d: %v %s", code, err, strings.TrimSpace(stderr))
	}
	added := AddedLines{}
	for _, rel := range strings.Split(out, "\x00") {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(w.ModuleDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("covchanged: read untracked %s: %w", rel, err)
		}
		for n := 1; n <= strings.Count(string(data), "\n")+1; n++ {
			added[rel] = append(added[rel], n)
		}
	}
	return added, nil
}

func (w Workspace) freshReader(profile string, measured time.Time) func(string) ([]byte, error) {
	return func(rel string) ([]byte, error) {
		path := filepath.Join(w.ModuleDir, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.ModTime().After(measured) {
			return nil, fmt.Errorf("the profile %s is older than %s; run `make -C go test-integration` again, then run this check again", profile, rel)
		}
		return os.ReadFile(path)
	}
}
