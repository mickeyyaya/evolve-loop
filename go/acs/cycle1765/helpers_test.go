//go:build acs

package cycle1765

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type goRun struct {
	out  string
	code int
	err  error
}

func execGo(dir string, args ...string) goRun {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return classifyRun(out, err, "go "+strings.Join(args, " "))
}

func classifyRun(out []byte, err error, what string) goRun {
	if err == nil {
		return goRun{out: string(out)}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return goRun{out: string(out), err: fmt.Errorf("%s: %w", what, err)}
	}
	return goRun{out: string(out), code: exitErr.ExitCode()}
}

const (
	evidenceChildEnv = "EVOLVE_ACS_C1765_EVIDENCE_CHILD"
	evidenceBudget   = 4 * time.Minute
)

func runEvidence(dir, script string) goRun {
	ctx, cancel := context.WithTimeout(context.Background(), evidenceBudget)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), evidenceChildEnv+"=1")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return classifyRun(out, err, "sh -c "+strconv.Quote(script))
}

type scoreCap struct {
	Criterion    string `yaml:"criterion"`
	MaxIfMissing int    `yaml:"max_if_missing"`
	Evidence     string `yaml:"evidence"`
}

func loadScoreCaps(path string) ([]scoreCap, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") {
		return nil, fmt.Errorf("%s does not open with a --- YAML frontmatter block", path)
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil, fmt.Errorf("%s has an unterminated YAML frontmatter block", path)
	}
	var fm struct {
		ScoreCap []scoreCap `yaml:"score_cap"`
	}
	if err := yaml.Unmarshal([]byte(text[4:4+end]), &fm); err != nil {
		return nil, fmt.Errorf("%s frontmatter: %w", path, err)
	}
	return fm.ScoreCap, nil
}

func readExplanationDoc(root string, cycle int) (string, string, error) {
	pattern := filepath.Join(root, "docs", "explain", "builds", fmt.Sprintf("cycle-%d-*.md", cycle))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", "", err
	}
	if len(matches) != 1 {
		return "", "", fmt.Errorf("want exactly one build explanation matching %s, found %d: %v", pattern, len(matches), matches)
	}
	raw, err := os.ReadFile(matches[0])
	return matches[0], string(raw), err
}

func markdownSection(doc, heading string) string {
	var b strings.Builder
	in := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(line) == heading
			continue
		}
		if in {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

var backtickSpanRE = regexp.MustCompile("`([^`]+)`")

func backtickSpans(text string) []string {
	var spans []string
	for _, m := range backtickSpanRE.FindAllStringSubmatch(text, -1) {
		spans = append(spans, m[1])
	}
	return spans
}

var goFlagsWithValue = map[string]bool{"-run": true, "-timeout": true, "-p": true, "-count": true, "-coverprofile": true, "-skip": true}

func goCommandPackages(span string) (tags, patterns []string, isGoCommand bool) {
	cmd := strings.TrimSpace(strings.Trim(strings.TrimSpace(span), "()"))
	cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "cd go &&"))
	fields := strings.Fields(cmd)
	if len(fields) < 2 || fields[0] != "go" || (fields[1] != "test" && fields[1] != "vet") {
		return nil, nil, false
	}
	for i := 2; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "-tags" && i+1 < len(fields):
			tags = append(tags, "-tags", fields[i+1])
			i++
		case strings.HasPrefix(f, "-tags="):
			tags = append(tags, f)
		case goFlagsWithValue[f]:
			i++
		case strings.HasPrefix(f, "-"):
		default:
			patterns = append(patterns, f)
		}
	}
	return tags, patterns, true
}

func fileImports(path string) (map[string]bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := map[string]bool{}
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		imports[p] = true
	}
	return imports, nil
}

type worktreeGit struct{ root string }

func (g worktreeGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g worktreeGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g worktreeGit) Show(base, path string) ([]byte, error) {
	if _, err := g.run("cat-file", "-e", base+":"+path); err != nil {
		return nil, fs.ErrNotExist
	}
	return g.run("show", base+":"+path)
}

func (g worktreeGit) Root() (string, error) { return g.root, nil }

func importsPath(imports map[string]bool, suffix string) bool {
	for p := range imports {
		if p == suffix || strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}
