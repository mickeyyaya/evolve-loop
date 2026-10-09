// Package acssuite is the deterministic, host-side EGPS predicate-suite
// runner: it runs the Go predicate lane and writes acs-verdict.json to the
// schema the audit and ship gates read.
package acssuite

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

const DefaultTimeout = 60 * time.Second

const evidenceMax = 600

var suiteLockWait = verifylock.MaxWait

const SkipExitCode = 77

type Result struct {
	ACID            string   `json:"ac_id"`
	Predicate       string   `json:"predicate"`
	ExitCode        int      `json:"exit_code"`
	ResultStr       string   `json:"result"`
	DurationMS      int64    `json:"duration_ms"`
	IsRegression    bool     `json:"is_regression"`
	IsRedTeam       bool     `json:"is_red_team,omitempty"`
	EvidenceExcerpt string   `json:"evidence_excerpt,omitempty"`
	FailingTests    []string `json:"failing_tests,omitempty"`
	EvidenceNote    string   `json:"evidence_note,omitempty"`
	PhantomBindings []string `json:"phantom_bindings,omitempty"`
	Flaky           string   `json:"flaky,omitempty"`

	RetryOutcome string `json:"retry_outcome,omitempty"`

	fullEvidence string
}

type PredicateSuite struct {
	ThisCycleCount       int `json:"this_cycle_count"`
	RegressionSuiteCount int `json:"regression_suite_count"`
	RedTeamCount         int `json:"red_team_count"`
	SkippedCount         int `json:"skipped_count"`
	Total                int `json:"total"`
}

func warningsFromFlaky(results []Result) []string {
	var w []string
	for _, r := range results {
		if r.Flaky != "" {
			w = append(w, fmt.Sprintf("flaky: %s passed-on-retry (bounded single retry absorbed a non-deterministic red; investigate under host contention)", r.ACID))
		}
	}
	return w
}

type Verdict struct {
	SchemaVersion  string         `json:"schema_version"`
	Cycle          int            `json:"cycle"`
	PredicateSuite PredicateSuite `json:"predicate_suite"`
	Results        []Result       `json:"results"`
	GreenCount     int            `json:"green_count"`
	RedCount       int            `json:"red_count"`
	SkipCount      int            `json:"skip_count"`
	RedIDs         []string       `json:"red_ids"`
	SkipIDs        []string       `json:"skip_ids,omitempty"`
	Verdict        string         `json:"verdict"`
	ShipEligible   bool           `json:"ship_eligible"`
	Warnings       []string       `json:"warnings,omitempty"`
	SuiteRoot      string         `json:"suite_root,omitempty"`
	ProjectRoot    string         `json:"project_root,omitempty"`
}

type Options struct {
	Root        string
	Cycle       int
	ProjectRoot string
	GoModuleDir string
	GoTimeout   time.Duration
	GoExec      func(ctx context.Context, moduleDir, pkgPattern string, env []string) (rawJSON string, err error)
}

func Run(opts Options) (Verdict, error) {
	opts, err := resolveOptions(opts)
	if err != nil {
		return Verdict{}, err
	}
	release := acquireSuiteLock(opts.Root)
	defer release()

	cfg, refusals := loadLaneConfig(opts.stateRoot())
	results := runGoTest(opts, cfg)
	demoteWarnings := demoteOutOfScope(results, opts)
	writeRedEvidence(opts, results)
	v := Verdict{SchemaVersion: "1.0", Cycle: opts.Cycle, SuiteRoot: opts.Root, ProjectRoot: opts.ProjectRoot}
	for _, r := range results {
		v.record(r)
	}
	v.PredicateSuite.SkippedCount = v.SkipCount
	v.PredicateSuite.Total = len(v.Results)
	v.ShipEligible = v.RedCount == 0
	v.Verdict = "FAIL"
	if v.ShipEligible {
		v.Verdict = "PASS"
	}
	v.Warnings = slices.Concat(warningsFromFlaky(v.Results), demoteWarnings, refusals)
	return v, nil
}

func resolveOptions(opts Options) (Options, error) {
	if opts.Root == "" {
		return Options{}, fmt.Errorf("acssuite: Root required")
	}
	if opts.Cycle <= 0 {
		return Options{}, fmt.Errorf("acssuite: Cycle must be > 0")
	}
	for _, p := range []*string{&opts.Root, &opts.ProjectRoot, &opts.GoModuleDir} {
		if *p == "" {
			continue
		}
		abs, err := filepath.Abs(*p)
		if err != nil {
			return Options{}, fmt.Errorf("acssuite: resolve %q to an absolute path: %w", *p, err)
		}
		*p = abs
	}
	return opts, nil
}

func (o Options) stateRoot() string {
	if o.ProjectRoot != "" {
		return o.ProjectRoot
	}
	return o.Root
}

func acquireSuiteLock(root string) func() {
	release, lockErr := verifylock.AcquireWithin(context.Background(), root, suiteLockWait, os.Stderr)
	if lockErr != nil {
		fmt.Fprintf(os.Stderr, "[acs] WARN: verification single-flight unavailable (%v) — running unserialized\n", lockErr)
		return func() {}
	}
	return release
}

func (v *Verdict) record(r Result) {
	switch r.ResultStr {
	case "green":
		v.GreenCount++
	case "skip":
		v.SkipCount++
		v.SkipIDs = append(v.SkipIDs, r.ACID)
	case "red":
		v.RedCount++
		v.RedIDs = append(v.RedIDs, r.ACID)
	}
	switch {
	case r.IsRedTeam:
		v.PredicateSuite.RedTeamCount++
	case r.IsRegression:
		v.PredicateSuite.RegressionSuiteCount++
	default:
		v.PredicateSuite.ThisCycleCount++
	}
	v.Results = append(v.Results, r)
}

const (
	projectRootKey     = "EVOLVE_PROJECT_ROOT"
	changedPackagesKey = "CHANGED_PACKAGES"
)

func suiteExportKeys() []string {
	return []string{projectRootKey, ipcenv.WorktreeRootKey, changedPackagesKey}
}

func forwardableKeys(named []string) (forward, refusals []string) {
	owned := slices.Concat(ipcenv.ProtocolKeys(), suiteExportKeys())
	for _, key := range named {
		if slices.Contains(owned, key) {
			refusals = append(refusals, fmt.Sprintf("acs.predicate_env names %s, a lane protocol key or one of the suite's own exports; it is not forwarded", key))
			continue
		}
		forward = append(forward, key)
	}
	return forward, refusals
}

type predicateExports struct {
	stateRoot    string
	sourceRoot   string
	changedPkgs  []string
	operatorKeys []string
}

func predicateEnv(x predicateExports) []string {
	host := os.Environ()
	env := append(entriesNotNamed(ipcenv.Scrub(host), suiteExportKeys()), entriesNamed(host, x.operatorKeys)...)
	if x.stateRoot != "" {
		env = append(env, projectRootKey+"="+x.stateRoot)
	}
	if x.sourceRoot != "" {
		env = append(env, ipcenv.WorktreeRootKey+"="+x.sourceRoot)
	}
	if len(x.changedPkgs) > 0 {
		env = append(env, changedPackagesKey+"="+strings.Join(x.changedPkgs, " "))
	}
	return env
}

func entriesNamed(environ, keys []string) []string {
	return slices.DeleteFunc(slices.Clone(environ), func(entry string) bool { return !slices.Contains(keys, envKey(entry)) })
}

func entriesNotNamed(environ, keys []string) []string {
	return slices.DeleteFunc(slices.Clone(environ), func(entry string) bool { return slices.Contains(keys, envKey(entry)) })
}

func envKey(entry string) string {
	key, _, _ := strings.Cut(entry, "=")
	return key
}

func hasGoACSTree(moduleDir string) bool {
	if fi, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil || fi.IsDir() {
		return false
	}
	if fi, err := os.Stat(filepath.Join(moduleDir, "acs")); err != nil || !fi.IsDir() {
		return false
	}
	return true
}

func defaultGoExec(ctx context.Context, moduleDir, pkgPattern string, env []string) (string, error) {
	cmd := sysexec.Command(ctx, "go", "test", "-json", "-tags", "acs", "-count=1", pkgPattern)
	cmd.Dir = moduleDir
	cmd.Env = env
	cmd.WaitDelay = 30 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func goLaneTimeout(optsTimeout time.Duration, cfg policy.ACSConfig) time.Duration {
	if optsTimeout > 0 {
		return optsTimeout
	}
	if cfg.GoTimeoutS > 0 {
		return time.Duration(cfg.GoTimeoutS) * time.Second
	}
	return DefaultTimeout
}

func currentCycleGoPkgDir(moduleDir string, cycle int) string {
	return filepath.Join(moduleDir, filepath.FromSlash(CyclePackage(cycle)))
}

func CyclePackage(cycle int) string { return fmt.Sprintf("./acs/cycle%d", cycle) }

func currentCycleGoPkgExists(moduleDir string, cycle int) bool {
	fi, err := os.Stat(currentCycleGoPkgDir(moduleDir, cycle))
	return err == nil && fi.IsDir()
}

func goLanePatterns(moduleDir string, cycle int) []string {
	var pats []string
	if dirExists(currentCycleGoPkgDir(moduleDir, cycle)) {
		pats = append(pats, CyclePackage(cycle))
	}
	if entries, err := os.ReadDir(filepath.Join(moduleDir, "acs", "regression")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				pats = append(pats, "./acs/regression/"+e.Name())
			}
		}
	}
	if dirExists(filepath.Join(moduleDir, "acs", "redteam")) {
		pats = append(pats, "./acs/redteam")
	}
	return pats
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func writeRedEvidence(opts Options, results []Result) {
	dir := filepath.Join(opts.stateRoot(), ".evolve", "runs", fmt.Sprintf("cycle-%d", opts.Cycle), "acs-red-evidence")
	for _, r := range results {
		if r.ResultStr != "red" || r.fullEvidence == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence dir: %v\n", err)
			return
		}
		body := fmt.Sprintf("# cycle=%d ac_id=%s predicate=%s run=%s\n--- FIRST RUN ---\n%s",
			opts.Cycle, r.ACID, r.Predicate, time.Now().UTC().Format(time.RFC3339), r.fullEvidence)
		base := strings.ReplaceAll(r.ACID, "/", "__")
		collisions := 0
		for i := 0; i < 10; i++ {
			name := base + ".txt"
			if i > 0 {
				name = fmt.Sprintf("%s-%d.txt", base, i+1)
			}
			f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if os.IsExist(err) {
				collisions++
				continue
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence write %s: %v\n", name, err)
				break
			}
			_, werr := f.WriteString(body)
			if cerr := f.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence write %s: %v\n", name, werr)
			}
			break
		}
		if collisions == 10 {
			fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence: 10 name collisions for %s — evidence dropped\n", base)
		}
	}
}

type goEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

func parseGoTestJSON(r io.Reader, cycle int) []Result {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	type acc struct {
		pkg, test string
		result    string
		dur       int64
		output    strings.Builder
	}
	byKey := map[string]*acc{}
	order := []string{}
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var ev goEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}
		key := ev.Package + "/" + ev.Test
		a, ok := byKey[key]
		if !ok {
			a = &acc{pkg: ev.Package, test: ev.Test}
			byKey[key] = a
			order = append(order, key)
		}
		switch ev.Action {
		case "output":
			a.output.WriteString(ev.Output)
		case "pass":
			a.result = "green"
			a.dur = int64(ev.Elapsed * 1000)
		case "fail":
			a.result = "red"
			a.dur = int64(ev.Elapsed * 1000)
		case "skip":
			a.result = "skip"
			a.dur = int64(ev.Elapsed * 1000)
		}
	}
	var out []Result
	if err := scanner.Err(); err != nil {
		out = append(out, Result{
			ACID:            acsverdict.SyntheticRedPrefix + "go-lane-parse-error",
			Predicate:       acsverdict.SyntheticRedPrefix + "go-lane-parse-error",
			ExitCode:        1,
			ResultStr:       "red",
			EvidenceExcerpt: excerpt("go test -json stream parse error (results may be truncated): " + err.Error()),
		})
	}
	for _, key := range order {
		a := byKey[key]
		if a.result == "" {
			out = append(out, Result{
				ACID:      acsverdict.SyntheticRedPrefix + "go-lane-incomplete/" + a.pkg + "/" + a.test,
				Predicate: acsverdict.SyntheticRedPrefix + "go-lane-incomplete", ExitCode: 1, ResultStr: "red",
				EvidenceExcerpt: "predicate started without a terminal result: " + a.pkg + "/" + a.test,
			})
			continue
		}
		dir := path.Base(a.pkg)
		isReg, isRT := classifyGoPkg(dir, cycle)
		r := Result{
			ACID:         dir + "/" + a.test,
			Predicate:    "go/acs/" + dir + "/...:" + a.test,
			DurationMS:   a.dur,
			IsRegression: isReg,
			IsRedTeam:    isRT,
			ResultStr:    a.result,
		}
		switch a.result {
		case "green":
			r.ExitCode = 0
		case "skip":
			r.ExitCode = SkipExitCode
			r.EvidenceExcerpt = excerpt(a.output.String())
		case "red":
			r.ExitCode = 1
			full := a.output.String()
			r.fullEvidence = full
			r.EvidenceExcerpt = excerpt(full)
			for _, name := range extractFailingTests(full) {
				if name != a.test {
					r.FailingTests = append(r.FailingTests, name)
				}
			}
			if len(r.FailingTests) == 0 && !strings.Contains(full, "--- FAIL: "+a.test) {
				r.EvidenceNote = "no `--- FAIL:` marker in output — compile failure, timeout, or signal; see excerpt tail"
			}
			r.PhantomBindings = phantomBindings(full, r.FailingTests)
		}
		out = append(out, r)
	}
	return out
}

func classifyGoPkg(dir string, cycle int) (isRegression, isRedTeam bool) {
	if strings.Contains(dir, "redteam") || strings.Contains(dir, "red-team") {
		return false, true
	}
	if n, ok := cycleNumFromDir(dir); ok && n == cycle {
		return false, false
	}
	return true, false
}

func cycleNumFromDir(dir string) (int, bool) {
	if !strings.HasPrefix(dir, "cycle") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(dir, "cycle"))
	if err != nil {
		return 0, false
	}
	return n, true
}

func changedPackagesForCycle(projectRoot string, cycle int) []string {
	if projectRoot == "" {
		return nil
	}
	dir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	for _, name := range []string{"handoff-build.json", "handoff-builder.json"} {
		if pkgs := changedpkgs.ChangedPackages(filepath.Join(dir, name)); len(pkgs) > 0 {
			return pkgs
		}
	}
	return nil
}

// output: a predicate's own t.Fatalf line — the author's diagnosis with its
// file:line — prints first. The remainder comes from the TAIL, where go test
// accumulates `--- FAIL:` detail.
const excerptHead = 200

func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= evidenceMax {
		return s
	}
	head := strings.ToValidUTF8(s[:excerptHead], "")
	tail := strings.ToValidUTF8(s[len(s)-(evidenceMax-excerptHead):], "")
	return head + "…" + tail
}

const maxFailingTests = 8

var failLineRE = regexp.MustCompile(`--- FAIL: (\S+)`)

func extractFailingTests(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range failLineRE.FindAllStringSubmatch(s, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) == maxFailingTests {
			break
		}
	}
	return out
}

var (
	writeVerdictCreateTemp = os.CreateTemp
	writeVerdictClose      = func(f *os.File) error { return f.Close() }
	writeVerdictWriteFile  = os.WriteFile
)

func WriteVerdict(evolveDir string, v Verdict) (string, error) {
	dst, err := acsverdict.Path(evolveDir, v.Cycle)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("acssuite: mkdir %s: %w", dir, err)
	}
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("acssuite: marshal: %w", err)
	}
	tmpf, err := writeVerdictCreateTemp(dir, "acs-verdict.*.tmp")
	if err != nil {
		return "", fmt.Errorf("acssuite: create tmp: %w", err)
	}
	tmp := tmpf.Name()
	if cerr := writeVerdictClose(tmpf); cerr != nil {
		return "", fmt.Errorf("acssuite: close tmp: %w", cerr)
	}
	if err := writeVerdictWriteFile(tmp, buf, 0o644); err != nil {
		return "", fmt.Errorf("acssuite: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", fmt.Errorf("acssuite: rename: %w", err)
	}
	return dst, nil
}
