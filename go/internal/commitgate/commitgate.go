// Package commitgate is the native port of commit-gate/commit-gate-runner.sh —
// the pre-commit quality gate the /commit skill invokes.
//
// It detects the languages of the changed files, validates that a simplifier
// AND one reviewer (general code-reviewer OR the matching language reviewer)
// were declared via --reviewers, runs lint + TARGETED tests for each changed
// language, and on a full pass writes <root>/.commit-gate/attestation.json bound
// to sha256(`git diff HEAD`).
//
// Enforcement of that attestation happens at the commit chokepoint
// (go/internal/phases/ship/commitgate.go, the reader that
// `evolve ship --class manual` runs). The writer here and that reader agree by
// construction: both compute the tree-state SHA via internal/treestate.SHA, and
// the attestation byte format produced by Write mirrors the bash heredoc exactly
// (field order, 2-space indent, inline arrays, trailing newline) so a
// byte-for-byte differential parity test can prove equivalence before B2 deletes
// the bash runner.
//
// Exit-code contract (preserved verbatim from the bash runner):
//
//	0  pass (+attestation written)
//	1  lint/test failure OR reviewer precondition unmet OR nothing to gate
//	2  git/SHA fatal (not a repo, git diff HEAD exit >1, hasher missing)
//	3  a required tool is missing and could not be auto-installed
//	10 bad CLI args
//
// Test seams (hermetic, no PATH hacks): CG_TEST_INSTALL=ok|fail and
// CG_TEST_FORCE_MISSING="tool ..." drive the auto-install path; CG_ATTEST_DIR
// overrides the attestation directory.
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

// Exit codes — the load-bearing contract the ship-gate reader and the /commit
// skill depend on. Identical to the bash runner's documented codes.
const (
	// ExitPass is a clean gate: lint + targeted tests passed and an attestation
	// was written.
	ExitPass = 0
	// ExitFail is a lint/test failure, an unmet reviewer precondition, or an
	// empty change set — anything that is a legitimate "this commit is not
	// allowed yet" answer.
	ExitFail = 1
	// ExitGitFatal is a git/SHA-level fatal: not a repo, `git diff HEAD` exit
	// >1, or no sha256 tool available to write the attestation.
	ExitGitFatal = 2
	// ExitToolMissing is a required linter/test tool that is missing and could
	// not be auto-installed.
	ExitToolMissing = 3
	// ExitBadArgs is a malformed CLI invocation.
	ExitBadArgs = 10
)

// Runner is the command-execution seam for the lint lanes (gofmt/go vet/go test,
// ruff/pytest, eslint, cargo). It is exactly sysexec.RunFunc so production
// callers pass sysexec.DefaultRunner and tests inject a fake without touching
// PATH. dir is the working directory for the command.
type Runner = sysexec.RunFunc

// Options configures one gate run. The zero value is not usable — Run requires a
// RepoRoot, a Runner, and a Now clock.
type Options struct {
	// RepoRoot is the git working tree root (the bash runner's `git rev-parse
	// --show-toplevel`). Changed-file paths are resolved relative to it and the
	// attestation lands under RepoRoot/.commit-gate unless AttestDir overrides.
	RepoRoot string
	// Reviewers is the raw --reviewers CSV exactly as the skill passed it (e.g.
	// "code-simplifier,ecc:go-reviewer"). It is recorded verbatim (sans empties)
	// in reviewers_run; the precondition check normalizes a copy.
	Reviewers string
	// Files, when non-empty, overrides change detection (the bash --files flag):
	// a whitespace-separated path list. Empty means "use git diff --name-only
	// HEAD".
	Files string
	// NoInstall disables tool auto-install (bash --no-install): a missing tool is
	// a hard ExitToolMissing instead of an install attempt.
	NoInstall bool
	// AttestDir overrides the attestation directory (bash CG_ATTEST_DIR). Empty
	// means RepoRoot/.commit-gate.
	AttestDir string
	// Env is the environment for the lint/test subprocesses (nil inherits the
	// parent). The CG_TEST_* seams are read from TestInstall/ForceMissing below,
	// not from Env, so tests stay hermetic.
	Env []string
	// Runner executes the lint/test commands. Required.
	Runner Runner
	// Now supplies the attestation timestamp. Required (inject a fixed clock in
	// tests for a deterministic `ts`).
	Now func() time.Time

	// TestInstall mirrors CG_TEST_INSTALL ("ok"|"fail"): forces the auto-install
	// result without running an installer. Empty means "really run the install
	// command" (which Run never does on its own — see ensureTool).
	TestInstall string
	// ForceMissing mirrors CG_TEST_FORCE_MISSING: a space-separated tool list
	// treated as absent regardless of PATH.
	ForceMissing string
	// lookPath is the tool-presence probe (defaults to exec.LookPath). Overridable
	// in tests so a tool can be made present/absent deterministically.
	lookPath func(string) (string, error)
}

// Result is the structured outcome of a gate run: the exit code the caller
// returns, the human log lines (written to stderr by the command wrapper), and
// — on a pass — the attestation that was written.
type Result struct {
	// ExitCode is one of the Exit* constants.
	ExitCode int
	// Logs are diagnostic lines mirroring the bash runner's `cg_log` output. The
	// cmd wrapper streams them to stderr.
	Logs []string
	// Attestation is the attestation written on ExitPass; nil otherwise.
	Attestation *Attestation
	// ChecksPassed records the lint/test checks that passed, in execution order
	// (e.g. "go:gofmt", "go:vet", "go:test"). It is the source for the
	// attestation's checks_passed array.
	ChecksPassed []string
	// Langs are the detected languages (sorted-unique), for diagnostics.
	Langs []string
}

func (r *Result) log(format string, a ...any) {
	r.Logs = append(r.Logs, "[commit-gate] "+fmt.Sprintf(format, a...))
}

// Run executes the gate and returns its Result. It never panics on a missing
// tool, a lint failure, or a git error — every such case maps onto an Exit*
// code so the caller can return it directly.
func (o Options) Run(ctx context.Context) *Result {
	res := &Result{ExitCode: ExitPass}
	if o.lookPath == nil {
		o.lookPath = lookPathDefault
	}

	files, code := o.changedFiles(res)
	if code != ExitPass {
		res.ExitCode = code
		return res
	}
	langs := detectLangs(files)
	res.Langs = langs

	if !o.reviewersSatisfied(langs, res) {
		res.ExitCode = ExitFail
		return res
	}

	// Run each language lane in the same order the bash runner iterates LANGS
	// (sorted-unique), recording the checks that pass for the attestation.
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

	att, code := o.writeAttestation(ctx, res)
	if code != ExitPass {
		res.ExitCode = code
		return res
	}
	res.Attestation = att
	res.log("PASS — attestation written (%s)", att.TreeStateSHA)
	return res
}

// changedFiles resolves the change set: the --files override (whitespace-split)
// or `git diff --name-only HEAD`, with blank lines dropped. An empty result is
// ExitFail ("nothing to gate"), matching the bash runner.
func (o Options) changedFiles(res *Result) ([]string, int) {
	var raw string
	if strings.TrimSpace(o.Files) != "" {
		raw = strings.Join(strings.Fields(o.Files), "\n")
	} else {
		out, _, code, err := sysexec.Capture(context.Background(), o.Runner, o.RepoRoot, "git", "diff", "--name-only", "HEAD")
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

// detectLangs maps changed-file extensions to languages and returns them
// sorted-unique, byte-identical to the bash cg_detect_langs | sort -u pipeline.
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

// filesWithExt returns the changed files whose extension is ext, preserving
// input order (mirrors the bash files_ext grep, minus the working-tree-existence
// filter which the lanes apply where needed).
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

// have reports whether tool is present, honoring the ForceMissing test seam
// (a forced-missing tool reports absent regardless of PATH).
func (o Options) have(tool string) bool {
	for _, t := range strings.Fields(o.ForceMissing) {
		if t == tool {
			return false
		}
	}
	_, err := o.lookPath(tool)
	return err == nil
}

// ensureTool guarantees tool is available or returns a non-zero Exit* code.
//
// install is the auto-install command hint (empty == not auto-installable);
// manual is the human fallback. The CG_TEST_INSTALL seam (Options.TestInstall)
// short-circuits the install with a deterministic ok/fail, exactly like the bash
// runner — Run itself never shells out an installer, so a production call with a
// genuinely missing, auto-installable tool and no test seam reports
// ExitToolMissing rather than mutating the host.
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
		// No test seam and the tool is genuinely missing: the bash runner would
		// shell out the installer, but a Go gate must not mutate the host as a
		// side effect of a verification call. Report missing — the operator
		// installs it (the /commit skill surfaces the manual hint).
		res.log("missing '%s' — install it, then re-run. Install: %s", tool, manual)
		return ExitToolMissing
	}
}

// writeAttestation computes the tree-state SHA, builds the Attestation, and
// writes it atomically. A SHA failure is ExitGitFatal (mirroring the bash
// `cannot compute tree SHA`); a missing hasher is also ExitGitFatal.
func (o Options) writeAttestation(ctx context.Context, res *Result) (*Attestation, int) {
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
		Tool:         tool,
	}
	if err := atomicwrite.Bytes(o.attestPath(), att.Marshal()); err != nil {
		res.log("cannot write attestation: %v", err)
		return nil, ExitGitFatal
	}
	return att, ExitPass
}

// hasherName reports the sha256 tool the bash runner would name in the `tool`
// field: "shasum" if present, else "sha256sum" if present, else "".
func (o Options) hasherName() string {
	if o.have("shasum") {
		return "shasum"
	}
	if o.have("sha256sum") {
		return "sha256sum"
	}
	return ""
}

// pass records a passed check token for the attestation's checks_passed array.
func (r *Result) pass(check string) { r.ChecksPassed = append(r.ChecksPassed, check) }

// runCmd executes name+args in dir via the Runner, returning combined output and
// whether the command succeeded (exit 0). Unrecoverable runner errors and any
// non-zero exit are failures.
func (o Options) runCmd(ctx context.Context, dir, name string, args ...string) (string, bool) {
	out, errOut, code, err := sysexec.Capture(ctx, o.Runner, dir, name, args...)
	if err != nil {
		return strings.TrimSpace(out + "\n" + errOut + "\n" + err.Error()), false
	}
	return strings.TrimSpace(out + "\n" + errOut), code == 0
}
