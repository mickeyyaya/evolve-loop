package subagent

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/aggregator"
	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/detectcli"
	"github.com/mickeyyaya/evolve-loop/go/internal/fanoutdispatch"
)

type DispatchParallelRequest struct {
	Agent         string
	Cycle         int
	WorkspacePath string
	ProfilesDir   string
	AdaptersDir   string
	CapabilityDir string
	ProjectRoot   string
	PluginRoot    string
	LedgerPath    string

	Concurrency        int
	CachePrefixEnabled bool
	TrackWorkers       bool
	TestExecutor       string
	WorktreePath       string
	DispatchDepth      int
}

type DispatchParallelOptions struct {
	ReadProfile          func(path string) (string, error)
	RunFanout            func(cfg fanoutdispatch.Config, stderr io.Writer) int
	RunAggregator        func(in aggregator.Inputs, stderr io.Writer) int
	InspectCap           func(adaptersDir, cli string) (capability.Inspection, error)
	WriteFanoutLed       func(ledgerPath string, e FanoutLedgerEntry, now func() time.Time) error
	WriteCache           func(req CachePrefixRequest, opts CachePrefixOptions) error
	GitState             func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	GenToken             func() (string, error)
	VerifyWorkerArtifact func(artifact, token string) VerifyResult
	Now                  func() time.Time
}

type DispatchParallelResult struct {
	AggregatePath        string
	WorkerCount          int
	WorkerNames          []string
	FanoutExitCode       int
	AggregatorExit       int
	QualityTier          string
	ParentToken          string
	WorkerVerifyFailures []string
}

func DispatchParallel(ctx context.Context, req DispatchParallelRequest, opts DispatchParallelOptions) (DispatchParallelResult, error) {
	fillDispatchParallelDefaults(&opts)

	if !agentRolePattern.MatchString(req.Agent) {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: unknown agent: %s", req.Agent)
	}
	if req.Cycle < 0 {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: cycle must be >= 0")
	}
	if info, err := os.Stat(req.WorkspacePath); err != nil || !info.IsDir() {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: workspace dir missing: %s", req.WorkspacePath)
	}
	if err := enforceChildDispatchDepth(req.DispatchDepth); err != nil {
		return DispatchParallelResult{}, err
	}

	profilePath := filepath.Join(req.ProfilesDir, req.Agent+".json")
	profileBody, err := opts.ReadProfile(profilePath)
	if err != nil {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: profile not found: %s", profilePath)
	}
	if matchField(profileBody, reFieldParallelEligible) != "true" {
		return DispatchParallelResult{},
			fmt.Errorf("dispatch-parallel: agent %s not parallel_eligible (single-writer invariant)", req.Agent)
	}

	subtasks := extractParallelSubtasks(profileBody)
	if len(subtasks) == 0 {
		return DispatchParallelResult{},
			fmt.Errorf("dispatch-parallel: profile %s has no parallel_subtasks", profilePath)
	}

	cli := detectcli.Canonical(matchField(profileBody, reFieldCLI))
	if cli == "" {
		return DispatchParallelResult{},
			fmt.Errorf("dispatch-parallel: cli unresolved for agent %s", req.Agent)
	}
	capDir := req.CapabilityDir
	if capDir == "" {
		capDir = req.AdaptersDir
	}
	insp, _ := opts.InspectCap(capDir, cli)
	tier := capabilityTier(insp.Manifest)

	workersDir := filepath.Join(req.WorkspacePath, "workers")
	if err := os.MkdirAll(workersDir, 0o755); err != nil {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: mkdir workers: %w", err)
	}

	cachePrefixPath := ""
	if req.CachePrefixEnabled {
		cachePrefixPath = filepath.Join(workersDir, "cache-prefix.md")
		if err := opts.WriteCache(CachePrefixRequest{
			Cycle:       req.Cycle,
			Agent:       req.Agent,
			Workspace:   req.WorkspacePath,
			OutPath:     cachePrefixPath,
			ProjectRoot: req.ProjectRoot,
		}, CachePrefixOptions{}); err != nil {
			return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: cache prefix: %w", err)
		}
	}

	aggTemplate := matchField(profileBody, reFieldOutputArtifact)
	var aggPath string
	if aggTemplate != "" {
		aggPath = resolveArtifactPath(aggTemplate, req.Cycle, req.ProjectRoot)
	} else {
		aggPath = filepath.Join(req.WorkspacePath, req.Agent+"-report.md")
	}
	if err := os.MkdirAll(filepath.Dir(aggPath), 0o755); err != nil {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: mkdir agg dir: %w", err)
	}

	parentToken, err := opts.GenToken()
	if err != nil {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: parent token: %w", err)
	}
	gitHead, treeDiff, _ := opts.GitState(ctx, req.ProjectRoot)
	if gitHead == "" {
		gitHead = "unknown"
	}
	if treeDiff == "" {
		treeDiff = "unknown"
	}

	commandsTSV := filepath.Join(workersDir, ".fanout-commands.tsv")
	resultsTSV := filepath.Join(workersDir, ".fanout-results.tsv")
	var cmdsBuf strings.Builder
	var workerNames []string
	var workerArtifacts []string
	var workerTokens []string

	evolveBin, err := os.Executable()
	if err != nil || evolveBin == "" {
		evolveBin = "evolve"
	}

	for _, st := range subtasks {
		workerName := req.Agent + "-" + st.Name
		artifact := filepath.Join(workersDir, workerName+".md")
		promptPath := filepath.Join(workersDir, ".prompt-"+st.Name+".txt")

		rendered := renderSubtaskPrompt(st.Template, req.Cycle, req.Agent, st.Name, req.WorkspacePath)
		if err := os.WriteFile(promptPath, []byte(rendered), 0o644); err != nil {
			return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: write prompt %s: %w", st.Name, err)
		}

		workerToken := parentToken + "-" + st.Name

		var cmd string
		if req.TestExecutor != "" {
			cmd = fmt.Sprintf(
				"EVOLVE_FANOUT_PARENT_AGENT=%s EVOLVE_FANOUT_WORKER_NAME=%s "+
					"EVOLVE_FANOUT_WORKER_ARTIFACT=%s EVOLVE_FANOUT_WORKER_TOKEN=%s "+
					"EVOLVE_FANOUT_CYCLE=%d EVOLVE_FANOUT_WORKSPACE=%s bash %s",
				req.Agent, st.Name, artifact, workerToken, req.Cycle,
				req.WorkspacePath, req.TestExecutor,
			)
		} else {
			cmd = buildWorkerRecursionCommand(
				evolveBin, req.Agent, st.Name, req.Cycle, req.DispatchDepth+1, req.WorkspacePath, promptPath, workerToken,
			)
		}
		fmt.Fprintf(&cmdsBuf, "%s\t%s\n", workerName, cmd)
		workerNames = append(workerNames, workerName)
		workerArtifacts = append(workerArtifacts, artifact)
		workerTokens = append(workerTokens, workerToken)
	}

	if err := os.WriteFile(commandsTSV, []byte(cmdsBuf.String()), 0o644); err != nil {
		return DispatchParallelResult{}, fmt.Errorf("dispatch-parallel: write commands.tsv: %w", err)
	}

	fanoutCfg := fanoutdispatch.Config{
		CommandsFile:    commandsTSV,
		ResultsFile:     resultsTSV,
		Concurrency:     req.Concurrency,
		CachePrefixFile: cachePrefixPath,
		TrackWorkers:    req.TrackWorkers,
	}
	fanoutRC := opts.RunFanout(fanoutCfg, os.Stderr)

	result := DispatchParallelResult{
		AggregatePath:  aggPath,
		WorkerCount:    len(subtasks),
		WorkerNames:    workerNames,
		FanoutExitCode: fanoutRC,
		QualityTier:    tier,
		ParentToken:    parentToken,
	}

	aggRC := 0
	if fanoutRC == 0 {
		for i, artifact := range workerArtifacts {
			verify := opts.VerifyWorkerArtifact(artifact, workerTokens[i])
			if verify.Verdict != VerdictPASS {
				result.WorkerVerifyFailures = append(result.WorkerVerifyFailures, artifact)
			}
		}
		if len(result.WorkerVerifyFailures) > 0 {
			result.AggregatorExit = aggregator.ExitUsageErr
		} else {
			mergePhase := mergePhaseFor(req.Agent)
			aggRC = opts.RunAggregator(aggregator.Inputs{
				Phase:   mergePhase,
				Output:  aggPath,
				Workers: workerArtifacts,
			}, os.Stderr)
			result.AggregatorExit = aggRC
		}
	}

	ledgerEntry := FanoutLedgerEntry{
		Cycle:          req.Cycle,
		Agent:          req.Agent,
		ChallengeToken: parentToken,
		GitHEAD:        gitHead,
		TreeStateSHA:   treeDiff,
		WorkerNames:    workerNames,
		WorkerCount:    len(subtasks),
		ExitCode:       fanoutRC,
		AggregatePath:  aggPath,
		QualityTier:    tier,
	}
	if fanoutRC != 0 {
		if _, err := os.Stat(aggPath); err != nil {
			ledgerEntry.AggregatePath = ""
		}
	}
	if req.LedgerPath != "" {
		if err := opts.WriteFanoutLed(req.LedgerPath, ledgerEntry, opts.Now); err != nil {
			return result, fmt.Errorf("dispatch-parallel: ledger write: %w", err)
		}
	}
	return result, nil
}

type subtask struct {
	Name     string
	Template string
}

var subtaskNameRE = regexp.MustCompile(`"name"\s*:\s*"([^"]*)"`)
var subtaskTemplateRE = regexp.MustCompile(`"prompt_template"\s*:\s*"((?:[^"\\]|\\.)*)"`)

func extractParallelSubtasks(profileBody string) []subtask {
	body, ok := capabilityExtractArray(profileBody, "parallel_subtasks")
	if !ok {
		return nil
	}
	var out []subtask
	depth := 0
	start := -1
	for i, r := range body {
		switch r {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				obj := body[start : i+1]
				name := firstSubmatch(subtaskNameRE, obj)
				tmpl := firstSubmatch(subtaskTemplateRE, obj)
				if name != "" {
					out = append(out, subtask{Name: name, Template: unescapeJSONString(tmpl)})
				}
				start = -1
			}
		}
	}
	return out
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func unescapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

func capabilityExtractArray(body, name string) (string, bool) {
	needle := fmt.Sprintf("\"%s\"", name)
	idx := strings.Index(body, needle)
	if idx < 0 {
		return "", false
	}
	tail := strings.TrimSpace(body[idx+len(needle):])
	if len(tail) == 0 || tail[0] != ':' {
		return "", false
	}
	tail = strings.TrimSpace(tail[1:])
	if len(tail) == 0 || tail[0] != '[' {
		return "", false
	}
	depth := 0
	for i, r := range tail {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return tail[1:i], true
			}
		}
	}
	return "", false
}

func renderSubtaskPrompt(tmpl string, cycle int, agent, worker, workspace string) string {
	tmpl = strings.ReplaceAll(tmpl, "{cycle}", fmt.Sprintf("%d", cycle))
	tmpl = strings.ReplaceAll(tmpl, "{agent}", agent)
	tmpl = strings.ReplaceAll(tmpl, "{worker}", worker)
	tmpl = strings.ReplaceAll(tmpl, "{workspace}", workspace)
	return tmpl
}

func mergePhaseFor(agent string) string {
	switch agent {
	case "scout":
		return "scout"
	case "auditor":
		return "audit"
	case "retrospective":
		return "learn"
	default:
		return agent
	}
}

func fillDispatchParallelDefaults(opts *DispatchParallelOptions) {
	if opts.ReadProfile == nil {
		opts.ReadProfile = defaultReadProfile
	}
	if opts.RunFanout == nil {
		opts.RunFanout = fanoutdispatch.Run
	}
	if opts.RunAggregator == nil {
		opts.RunAggregator = aggregator.Aggregate
	}
	if opts.InspectCap == nil {
		opts.InspectCap = capability.Inspect
	}
	if opts.WriteFanoutLed == nil {
		opts.WriteFanoutLed = WriteFanoutLedgerEntry
	}
	if opts.WriteCache == nil {
		opts.WriteCache = WriteCachePrefix
	}
	if opts.GitState == nil {
		opts.GitState = defaultGitState
	}
	if opts.GenToken == nil {
		opts.GenToken = func() (string, error) { return generateRunToken(nil) }
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.VerifyWorkerArtifact == nil {
		now := opts.Now
		opts.VerifyWorkerArtifact = func(artifact, token string) VerifyResult {
			return defaultVerifyWorkerArtifact(now, artifact, token)
		}
	}
}

func defaultVerifyWorkerArtifact(now func() time.Time, artifact, token string) VerifyResult {
	in := VerifyInput{
		Now:          now(),
		MaxAge:       time.Duration(math.MaxInt64),
		ArtifactPath: artifact,
		Token:        token,
	}
	info, statErr := os.Stat(artifact)
	in.StatErr = statErr
	if statErr == nil {
		in.MTime = info.ModTime()
		body, readErr := os.ReadFile(artifact)
		in.Body = body
		in.ReadErr = readErr
	}
	return Verify(in)
}
