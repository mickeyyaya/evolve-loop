package proctree

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Cwd struct {
	Pid  int    `json:"pid"`
	Ppid int    `json:"ppid"`
	Cwd  string `json:"cwd"`
}

type CwdLister func(ctx context.Context) ([]Cwd, error)

func ExecCwdLister() CwdLister {
	return func(ctx context.Context) ([]Cwd, error) { return ListCwds(ctx, sysexec.DefaultRunner, "") }
}

func ListCwds(ctx context.Context, run sysexec.RunFunc, dir string) ([]Cwd, error) {
	var out strings.Builder
	code, err := run(ctx, "lsof", dir, []string{"-a", "-d", "cwd", "-u", strconv.Itoa(os.Getuid()), "-R", "-F", "pRn"}, nil, nil, &out, nil)
	if err != nil {
		return nil, fmt.Errorf("list process cwds: %w", err)
	}
	if code != 0 && out.Len() == 0 {
		return nil, fmt.Errorf("list process cwds: lsof exit %d with no output", code)
	}
	return ParseCwdListing(out.String()), nil
}

func ParseCwdListing(s string) []Cwd {
	var out []Cwd
	var cur Cwd
	flush := func() {
		if cur.Pid > 0 && cur.Cwd != "" {
			out = append(out, cur)
		}
		cur = Cwd{}
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			flush()
			cur.Pid, _ = strconv.Atoi(line[1:])
		case 'R':
			cur.Ppid, _ = strconv.Atoi(line[1:])
		case 'n':
			cur.Cwd = line[1:]
		}
	}
	flush()
	return out
}

func OrphanUnder(base string, since time.Time, cwds []Cwd) Proof {
	clean := filepath.Clean(base)
	if !filepath.IsAbs(clean) || filepath.Dir(clean) == clean {
		return func(Process) bool { return false }
	}
	under := make(map[int]bool, len(cwds))
	for _, c := range cwds {
		if inside(clean, filepath.Clean(c.Cwd)) {
			under[c.Pid] = true
		}
	}
	return func(p Process) bool {
		return p.Ppid == 1 && under[p.Pid] && !p.Started.Before(since) && !SharedHelper(p)
	}
}

func Under(dir, path string) bool {
	return within(baseForms(dir), path)
}

func baseForms(dir string) []string {
	if dir == "" {
		return nil
	}
	return []string{filepath.Clean(dir), resolved(dir)}
}

func within(bases []string, path string) bool {
	if path == "" {
		return false
	}
	for _, base := range bases {
		for _, p := range []string{filepath.Clean(path), resolved(path)} {
			if inside(base, p) {
				return true
			}
		}
	}
	return false
}

func inside(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}
