//go:build acs

package cycle1799

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type builtBinary struct {
	once      sync.Once
	pkg, name string
	dir, path string
	failure   string
}

var evolveBuild = &builtBinary{pkg: "./cmd/evolve", name: "evolve"}

func (b *builtBinary) get(t *testing.T) string {
	t.Helper()
	b.once.Do(func() { b.failure = b.build(filepath.Join(acsassert.RepoRoot(t), "go")) })
	if b.failure != "" {
		t.Fatalf("go build %s: %s", b.pkg, b.failure)
	}
	return b.path
}

func (b *builtBinary) build(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1799-"+b.name+"-")
	if err != nil {
		return err.Error()
	}
	b.dir, b.path = dir, filepath.Join(dir, b.name)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", b.path, b.pkg)
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func (r result) combined() string { return r.stdout + "\n" + r.stderr }

func runEvolve(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBuild.get(t), args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, env, 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve %s: %v", strings.Join(args, " "), err)
	}
	return r
}

func scrubbedEnviron() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if name == "PATH" || strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") ||
			strings.HasPrefix(name, "GH_") || strings.HasPrefix(name, "C1799_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func hermeticEnv(extra ...string) []string {
	env := append(scrubbedEnviron(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	return append(env, extra...)
}

func toolsDir(t *testing.T, withFakeGH bool) string {
	t.Helper()
	dir := t.TempDir()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("the fixtures need git: %v", err)
	}
	links := map[string]string{"git": gitBin}
	if withFakeGH {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		links["gh"] = self
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type mergedPR struct{ branch, headRefOid string }

func cliEnv(t *testing.T, withFakeGH bool, newestFirst ...mergedPR) (env []string, ghLog string) {
	t.Helper()
	ghLog = filepath.Join(t.TempDir(), "gh-invocations.ndjson")
	entries := make([]string, 0, len(newestFirst))
	for _, pr := range newestFirst {
		entries = append(entries, pr.branch+"="+pr.headRefOid)
	}
	env = hermeticEnv(
		"PATH="+toolsDir(t, withFakeGH),
		fakeGHEnv+"=1",
		fakeGHMergedEnv+"="+strings.Join(entries, ","),
		fakeGHLogEnv+"="+ghLog,
	)
	return env, ghLog
}

func ghInvocations(ghLog string) string {
	raw, err := os.ReadFile(ghLog)
	if err != nil {
		return "(gh was never invoked)"
	}
	return string(raw)
}

type ghOpts struct {
	head, state, jq, positional string
	hasJSON                     bool
}

var ghValueFlags = map[string]string{
	"--head": "head", "-H": "head", "--state": "state", "-s": "state", "--jq": "jq", "-q": "jq",
	"--json": "json", "--repo": "", "-R": "", "--base": "", "-B": "", "--limit": "", "-L": "",
	"--search": "", "-S": "", "--author": "", "-A": "", "--label": "", "-l": "", "--template": "", "-t": "",
}

func (o *ghOpts) set(key, value string) {
	switch key {
	case "head":
		o.head = value
	case "state":
		o.state = strings.ToLower(value)
	case "jq":
		o.jq = value
	case "json":
		o.hasJSON = true
	}
}

func parseGHOpts(args []string) ghOpts {
	var o ghOpts
	for i := 0; i < len(args); i++ {
		a := args[i]
		if name, value, ok := strings.Cut(a, "="); ok && strings.HasPrefix(name, "-") {
			o.set(ghValueFlags[name], value)
			continue
		}
		if key, ok := ghValueFlags[a]; ok {
			if i+1 < len(args) {
				o.set(key, args[i+1])
				i++
			}
			continue
		}
		if !strings.HasPrefix(a, "-") && o.positional == "" {
			o.positional = a
		}
	}
	return o
}

func fakePR(head, headRefOid string, number int) map[string]any {
	return map[string]any{
		"number": number, "state": "MERGED", "headRefName": head, "headRefOid": headRefOid, "baseRefName": "main",
		"mergedAt": "2026-10-01T12:00:00Z", "closed": true, "isDraft": false,
		"url": fmt.Sprintf("https://github.com/fake/fake/pull/%d", number), "title": fmt.Sprintf("feature (#%d)", number),
		"mergeCommit": map[string]any{"oid": strings.Repeat("7", 40)},
	}
}

func mergedPRsByHead() map[string][]map[string]any {
	oidsNewestFirst := map[string][]string{}
	for _, entry := range strings.Split(os.Getenv(fakeGHMergedEnv), ",") {
		branch, oid, _ := strings.Cut(strings.TrimSpace(entry), "=")
		if branch != "" {
			oidsNewestFirst[branch] = append(oidsNewestFirst[branch], oid)
		}
	}
	merged := map[string][]map[string]any{}
	for branch, oids := range oidsNewestFirst {
		for i, oid := range oids {
			merged[branch] = append(merged[branch], fakePR(branch, oid, 6+len(oids)-i))
		}
	}
	return merged
}

func serveFakeGH(args []string, stdout, stderr io.Writer) int {
	logFakeGH(args)
	merged := mergedPRsByHead()
	rest := args
	for i, a := range args {
		if a == "pr" || a == "auth" {
			rest = args[i:]
			break
		}
	}
	switch {
	case len(rest) >= 2 && rest[0] == "auth" && rest[1] == "status":
		fmt.Fprintln(stdout, "github.com: logged in as fake")
		return 0
	case len(rest) >= 2 && rest[0] == "pr" && rest[1] == "list":
		return fakePRList(parseGHOpts(rest[2:]), merged, stdout, stderr)
	case len(rest) >= 2 && rest[0] == "pr" && rest[1] == "view":
		return fakePRView(parseGHOpts(rest[2:]), merged, stdout, stderr)
	}
	fmt.Fprintf(stderr, "fake gh: unsupported invocation %q (supported: pr list, pr view, auth status)\n", args)
	return 1
}

func logFakeGH(args []string) {
	path := os.Getenv(fakeGHLogEnv)
	if path == "" {
		return
	}
	line, err := json.Marshal(args)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", line)
}

func fakePRList(o ghOpts, merged map[string][]map[string]any, stdout, stderr io.Writer) int {
	state := o.state
	if state == "" {
		state = "open"
	}
	prs := []map[string]any{}
	if state == "merged" || state == "closed" || state == "all" {
		prs = append(prs, merged[o.head]...)
	}
	if o.jq != "" {
		return fakeJQList(o.jq, prs, stdout, stderr)
	}
	if !o.hasJSON {
		for _, pr := range prs {
			fmt.Fprintf(stdout, "%v\t%s\t%s\tMERGED\n", pr["number"], pr["title"], o.head)
		}
		return 0
	}
	return writeJSON(prs, stdout, stderr)
}

func fakePRView(o ghOpts, merged map[string][]map[string]any, stdout, stderr io.Writer) int {
	head := o.positional
	if head == "" {
		head = o.head
	}
	if len(merged[head]) == 0 {
		fmt.Fprintf(stderr, "no pull requests found for branch %q\n", head)
		return 1
	}
	pr := merged[head][0]
	if o.jq == "" {
		return writeJSON(pr, stdout, stderr)
	}
	field, ok := strings.CutPrefix(strings.ReplaceAll(o.jq, " ", ""), ".")
	if v, found := pr[field]; ok && found {
		fmt.Fprintln(stdout, v)
		return 0
	}
	fmt.Fprintf(stderr, "fake gh: unsupported --jq %q for pr view (supported: .state .mergedAt .number .headRefName .headRefOid)\n", o.jq)
	return 1
}

func fakeJQList(expr string, prs []map[string]any, stdout, stderr io.Writer) int {
	e := strings.ReplaceAll(expr, " ", "")
	if e == "length" || e == ".|length" {
		fmt.Fprintln(stdout, strconv.Itoa(len(prs)))
		return 0
	}
	for _, prefix := range []string{".[].", ".[]|.", ".[0]."} {
		field, ok := strings.CutPrefix(e, prefix)
		if !ok {
			continue
		}
		if prefix == ".[0]." && len(prs) == 0 {
			fmt.Fprintln(stdout, "null")
			return 0
		}
		for i, pr := range prs {
			if prefix == ".[0]." && i > 0 {
				break
			}
			fmt.Fprintln(stdout, pr[field])
		}
		return 0
	}
	fmt.Fprintf(stderr, "fake gh: unsupported --jq %q for pr list (supported: length, .[].<field>, .[0].<field>)\n", expr)
	return 1
}

func writeJSON(v any, stdout, stderr io.Writer) int {
	body, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(stderr, "fake gh: marshal: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", body)
	return 0
}
