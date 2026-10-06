// Package commitgate is the pre-commit quality gate that `evolve commit-gate run`
// runs for the /commit skill. See docs/architecture/packages/internal-commitgate.md.
package commitgate

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/treestate"
)

const (
	ExitPass        = 0
	ExitFail        = 1
	ExitGitFatal    = 2
	ExitToolMissing = 3
	ExitBadArgs     = 10
)

type Runner = sysexec.RunFunc

type Options struct {
	RepoRoot  string
	Reviewers string
	Files     string
	NoInstall bool
	AttestDir string
	Env       []string
	Runner    Runner
	Now       func() time.Time

	LintBudget time.Duration

	TestInstall  string
	ForceMissing string
	lookPath     func(string) (string, error)
}

type Result struct {
	ExitCode     int
	Logs         []string
	Attestation  *Attestation
	ChecksPassed []string
	Langs        []string
}

func (r *Result) log(format string, a ...any) {
	r.Logs = append(r.Logs, "[commit-gate] "+fmt.Sprintf(format, a...))
}

func (o Options) Run(ctx context.Context) *Result {
	res := &Result{ExitCode: ExitPass}
	if o.lookPath == nil {
		o.lookPath = lookPathDefault
	}

	files, code := o.changedFiles(ctx, res)
	if code != ExitPass {
		res.ExitCode = code
		return res
	}
	if code := o.refuseWhatNeverCommits(ctx, files, res); code != ExitPass {
		res.ExitCode = code
		return res
	}
	langs := detectLangs(files)
	res.Langs = langs

	waiver, code := o.reviewDecision(ctx, langs, res)
	if code != ExitPass {
		res.ExitCode = code
		return res
	}

	for _, lang := range langs {
		var laneCode int
		switch lang {
		case "go":
			laneCode = o.laneGo(ctx, files, res)
		case "python":
			laneCode = o.lanePython(ctx, files, res)
		case "ts", "js":
			laneCode = o.laneNode(ctx, files, res)
		case "rust":
			laneCode = o.laneRust(ctx, files, res)
		}
		if laneCode != ExitPass {
			res.ExitCode = laneCode
			return res
		}
	}

	att, code := o.writeAttestation(ctx, waiver, res)
	if code != ExitPass {
		res.ExitCode = code
		return res
	}
	res.Attestation = att
	return res
}

func (o Options) reviewDecision(ctx context.Context, langs []string, res *Result) (waiver string, code int) {
	waived, refused := o.reviewWaiver(ctx)
	switch {
	case waived != "":
		res.log("reviewers not required: %s", waived)
		return commentOnlyWaiver, ExitPass
	case !o.reviewersSatisfied(langs, res):
		res.log("no review waiver: %s", refused)
		return "", ExitFail
	}
	return "", ExitPass
}

func (o Options) changedFiles(ctx context.Context, res *Result) ([]string, int) {
	var raw string
	if strings.TrimSpace(o.Files) != "" {
		raw = strings.Join(strings.Fields(o.Files), "\n")
	} else {
		out, _, code, err := sysexec.Capture(ctx, o.Runner, o.RepoRoot, "git", "diff", "--name-only", "HEAD")
		if err != nil || code > 1 {
			res.log("git diff --name-only HEAD failed")
			return nil, ExitGitFatal
		}
		raw = out
	}
	var files []string
	for _, line := range strings.Split(raw, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			files = append(files, s)
		}
	}
	if len(files) == 0 {
		res.log("no changed tracked files vs HEAD — nothing to gate (stage your changes first).")
		return nil, ExitFail
	}
	return files, ExitPass
}

func detectLangs(files []string) []string {
	seen := map[string]bool{}
	for _, f := range files {
		i := strings.LastIndex(f, ".")
		if i < 0 {
			continue
		}
		ext := f[i+1:]
		var lang string
		switch ext {
		case "go":
			lang = "go"
		case "py":
			lang = "python"
		case "ts", "tsx":
			lang = "ts"
		case "js", "jsx", "mjs", "cjs":
			lang = "js"
		case "rs":
			lang = "rust"
		default:
			continue
		}
		seen[lang] = true
	}
	langs := make([]string, 0, len(seen))
	for l := range seen {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	return langs
}

func filesWithExt(files []string, ext string) []string {
	var out []string
	suffix := "." + ext
	for _, f := range files {
		if strings.HasSuffix(f, suffix) {
			out = append(out, f)
		}
	}
	return out
}

func (o Options) attestPath() string {
	dir := o.AttestDir
	if dir == "" {
		dir = filepath.Join(o.RepoRoot, ".commit-gate")
	}
	return filepath.Join(dir, "attestation.json")
}

func (o Options) have(tool string) bool {
	for _, t := range strings.Fields(o.ForceMissing) {
		if t == tool {
			return false
		}
	}
	_, err := o.lookPath(tool)
	return err == nil
}

func (o Options) ensureTool(tool, install, manual string, res *Result) int {
	if o.have(tool) {
		return ExitPass
	}
	if install == "" || o.NoInstall {
		res.log("missing '%s' (not auto-installable here). Install manually: %s", tool, manual)
		return ExitToolMissing
	}
	switch o.TestInstall {
	case "ok":
		return ExitPass
	case "fail":
		res.log("auto-install of '%s' FAILED. Install manually: %s", tool, manual)
		return ExitToolMissing
	default:
		res.log("missing '%s' — install it, then re-run. Install: %s", tool, manual)
		return ExitToolMissing
	}
}

func (o Options) writeAttestation(ctx context.Context, waiver string, res *Result) (*Attestation, int) {
	sum, err := treestate.SHA(ctx, o.Runner, o.RepoRoot, o.Env)
	if err != nil {
		res.log("cannot compute tree SHA")
		return nil, ExitGitFatal
	}
	tool := o.hasherName()
	if tool == "" {
		res.log("no shasum/sha256sum available to stamp the attestation")
		return nil, ExitGitFatal
	}
	att := &Attestation{
		TreeStateSHA: sum,
		TS:           o.Now().UTC().Format("2006-01-02T15:04:05Z"),
		ChecksPassed: res.ChecksPassed,
		ReviewersRun: splitReviewers(o.Reviewers),
		ReviewWaiver: waiver,
		Tool:         tool,
	}
	body, err := att.Marshal()
	if err == nil {
		err = atomicwrite.Bytes(o.attestPath(), body)
	}
	if err != nil {
		res.log("cannot write attestation: %v", err)
		return nil, ExitGitFatal
	}
	res.log("PASS — attestation written (%s)", att.TreeStateSHA)
	return att, ExitPass
}

func (o Options) hasherName() string {
	if o.have("shasum") {
		return "shasum"
	}
	if o.have("sha256sum") {
		return "sha256sum"
	}
	return ""
}

func (r *Result) pass(check string) { r.ChecksPassed = append(r.ChecksPassed, check) }

func (o Options) runCmd(ctx context.Context, dir, name string, args ...string) (string, bool) {
	out, errOut, code, err := sysexec.Capture(ctx, o.Runner, dir, name, args...)
	if err != nil {
		return strings.TrimSpace(out + "\n" + errOut + "\n" + err.Error()), false
	}
	return strings.TrimSpace(out + "\n" + errOut), code == 0
}
