// Package aggregator ports legacy/scripts/dispatch/aggregator.sh.
//
// Pure-shell merge of fan-out worker artifacts. Merges N worker artifacts
// into a single canonical phase artifact according to per-phase rules.
// NO LLM CALL — this script cannot be coerced by a malicious worker into
// accepting forged output.
//
// Per-phase merge rules:
//   - scout / research / discover     → concat with "## Worker:" headers
//   - audit                            → ALL-PASS verdict; any FAIL fails the aggregate
//   - learn / retrospective / retro    → union of "## Lesson:" sections, deduped
//   - plan-review                      → multi-lens score+verdict aggregation
//   - audit-consensus / cross-cli-vote → MAJORITY-PASS with FAIL-VETO
package aggregator

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Exit codes (matches aggregator.sh):
//   0 — merge succeeded (audit PASS/WARN, plan-review PROCEED/REVISE, etc.)
//   1 — audit FAIL, plan-review ABORT, or cross-cli FAIL
//   2 — usage error / input file missing / unknown phase
const (
	ExitOK         = 0
	ExitVerdictBad = 1
	ExitUsageErr   = 2
)

// MergeMode is the per-phase merge strategy.
type MergeMode int

const (
	ModeUnknown MergeMode = iota
	ModeConcat
	ModeVerdict
	ModeLessons
	ModePlanReview
	ModeCrossCLIVote
)

// ResolveMode maps a phase name to its merge mode. Unknown phases return ModeUnknown.
func ResolveMode(phase string) MergeMode {
	switch phase {
	case "scout", "research", "discover":
		return ModeConcat
	case "audit":
		return ModeVerdict
	case "learn", "retrospective", "retro":
		return ModeLessons
	case "plan-review":
		return ModePlanReview
	case "audit-consensus", "cross-cli-vote":
		return ModeCrossCLIVote
	}
	return ModeUnknown
}

// Inputs collects the arguments to Aggregate.
type Inputs struct {
	Phase   string
	Output  string
	Workers []string
	Now     func() time.Time
}

// Aggregate merges the worker artifacts into the output path according to the
// phase's merge mode. Returns the exit code (0/1/2). stderr receives
// [aggregator] error lines.
func Aggregate(in Inputs, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[aggregator] "+format+"\n", args...)
	}
	if in.Phase == "" || in.Output == "" {
		logf("usage: aggregator <phase> <output> <worker-artifact>...")
		return ExitUsageErr
	}
	if len(in.Workers) < 1 {
		logf("error: at least one worker artifact required")
		return ExitUsageErr
	}
	for _, w := range in.Workers {
		info, err := os.Stat(w)
		if err != nil {
			logf("error: worker artifact not found: %s", w)
			return ExitUsageErr
		}
		if info.Size() == 0 {
			logf("error: worker artifact is empty: %s", w)
			return ExitUsageErr
		}
	}
	mode := ResolveMode(in.Phase)
	if mode == ModeUnknown {
		logf("error: unknown phase '%s'", in.Phase)
		return ExitUsageErr
	}
	if in.Now == nil {
		in.Now = time.Now
	}

	if err := os.MkdirAll(filepath.Dir(in.Output), 0o755); err != nil {
		logf("error: mkdir %s: %v", filepath.Dir(in.Output), err)
		return ExitUsageErr
	}

	tmp := fmt.Sprintf("%s.tmp.%d", in.Output, os.Getpid())
	defer os.Remove(tmp)

	now := in.Now().UTC().Format("2006-01-02T15:04:05Z")
	var rc int
	switch mode {
	case ModeConcat:
		writeConcat(tmp, in.Phase, in.Workers, now)
	case ModeVerdict:
		rc = writeVerdict(tmp, in.Workers, now)
	case ModeLessons:
		writeLessons(tmp, in.Workers, now)
	case ModePlanReview:
		rc = writePlanReview(tmp, in.Workers, now)
	case ModeCrossCLIVote:
		rc = writeCrossCLIVote(tmp, in.Workers, now)
	}

	if err := os.Rename(tmp, in.Output); err != nil {
		logf("error: write %s: %v", in.Output, err)
		return ExitUsageErr
	}
	return rc
}

// ── concat (scout / research / discover) ──────────────────────────────────

func writeConcat(out, phase string, workers []string, now string) {
	var b strings.Builder
	fmt.Fprintf(&b, "# Aggregated %s Report\n\n", phase)
	fmt.Fprintf(&b, "_Aggregated by aggregator.sh at %s. Workers: %d._\n\n", now, len(workers))
	for _, w := range workers {
		name := strings.TrimSuffix(filepath.Base(w), ".md")
		fmt.Fprintf(&b, "## Worker: %s\n\n", name)
		body, _ := os.ReadFile(w)
		b.Write(body)
		b.WriteString("\n\n")
	}
	_ = os.WriteFile(out, []byte(b.String()), 0o644)
}

// ── verdict (audit) ────────────────────────────────────────────────────────

var verdictLineRe = regexp.MustCompile(`(?i)^\s*verdict:\s*(\S+)`)

// extractVerdict reads the first "Verdict: <token>" line from the file and
// returns the uppercase first word. Returns "" if no verdict line.
func extractVerdict(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		m := verdictLineRe.FindStringSubmatch(scanner.Text())
		if m != nil {
			return strings.ToUpper(m[1])
		}
	}
	return ""
}

func writeVerdict(out string, workers []string, now string) int {
	anyFail := false
	anyWarn := false
	allPass := true
	for _, w := range workers {
		v := extractVerdict(w)
		switch v {
		case "PASS":
			// ok
		case "FAIL":
			anyFail = true
			allPass = false
		case "WARN":
			anyWarn = true
			allPass = false
		default:
			anyWarn = true
			allPass = false
		}
	}
	_ = anyWarn
	verdict := "WARN"
	if anyFail {
		verdict = "FAIL"
	} else if allPass {
		verdict = "PASS"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Verdict: %s\n\n", verdict)
	fmt.Fprintf(&b, "# Aggregated Audit Report\n\n")
	fmt.Fprintf(&b, "_Aggregated by aggregator.sh at %s. Workers: %d. Aggregate verdict: %s._\n\n",
		now, len(workers), verdict)
	for _, w := range workers {
		name := strings.TrimSuffix(filepath.Base(w), ".md")
		fmt.Fprintf(&b, "## Worker: %s\n\n", name)
		body, _ := os.ReadFile(w)
		b.Write(body)
		b.WriteString("\n\n")
	}
	_ = os.WriteFile(out, []byte(b.String()), 0o644)
	if verdict == "FAIL" {
		return ExitVerdictBad
	}
	return ExitOK
}

// ── lessons (learn / retrospective / retro) ────────────────────────────────

var lessonHeadingRe = regexp.MustCompile(`(?i)^## Lesson:`)

// writeLessons concats all workers, then iterates lines, capturing each
// "## Lesson: <title>" block and emitting only once per title.
func writeLessons(out string, workers []string, now string) {
	var combined strings.Builder
	for _, w := range workers {
		body, _ := os.ReadFile(w)
		combined.Write(body)
		combined.WriteString("\n")
	}

	var dedup strings.Builder
	seen := map[string]bool{}
	var currentTitle string
	var currentBlock strings.Builder
	inBlock := false

	flush := func() {
		if inBlock && !seen[currentTitle] {
			dedup.WriteString(currentBlock.String())
			dedup.WriteByte('\n')
			seen[currentTitle] = true
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(combined.String()))
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if lessonHeadingRe.MatchString(line) {
			flush()
			currentTitle = line
			currentBlock.Reset()
			currentBlock.WriteString(line)
			inBlock = true
			continue
		}
		if inBlock {
			currentBlock.WriteByte('\n')
			currentBlock.WriteString(line)
		}
	}
	flush()

	var b strings.Builder
	fmt.Fprintf(&b, "# Aggregated Retrospective Report\n\n")
	fmt.Fprintf(&b, "_Aggregated by aggregator.sh at %s. Workers: %d._\n\n", now, len(workers))
	b.WriteString(dedup.String())
	_ = os.WriteFile(out, []byte(b.String()), 0o644)
}

// ── plan-review ────────────────────────────────────────────────────────────

var scoreLineRe = regexp.MustCompile(`(?i)^\s*score:\s*([0-9]*\.?[0-9]+)`)

func extractScore(path string) float64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := scoreLineRe.FindStringSubmatch(scanner.Text()); m != nil {
			v, perr := strconv.ParseFloat(m[1], 64)
			if perr == nil {
				return v
			}
		}
	}
	return 0
}

func writePlanReview(out string, workers []string, now string) int {
	anyAbort := false
	weakLens := false
	sum := 0.0
	for _, w := range workers {
		s := extractScore(w)
		v := extractVerdict(w)
		if v == "ABORT" {
			anyAbort = true
		}
		if s < 5 {
			weakLens = true
		}
		sum += s
	}
	avg := 0.0
	if len(workers) > 0 {
		avg = sum / float64(len(workers))
	}
	verdict := "REVISE"
	switch {
	case anyAbort || avg < 5:
		verdict = "ABORT"
	case weakLens:
		verdict = "REVISE"
	case avg >= 7:
		verdict = "PROCEED"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Verdict: %s\n", verdict)
	fmt.Fprintf(&b, "Average Score: %.2f\n\n", avg)
	fmt.Fprintf(&b, "# Aggregated Plan-Review Report\n\n")
	fmt.Fprintf(&b, "_Aggregated by aggregator.sh at %s. Lenses: %d. Average: %.2f. Verdict: %s._\n\n",
		now, len(workers), avg, verdict)
	for _, w := range workers {
		name := strings.TrimSuffix(filepath.Base(w), ".md")
		fmt.Fprintf(&b, "## Worker: %s\n\n", name)
		body, _ := os.ReadFile(w)
		b.Write(body)
		b.WriteString("\n\n")
	}
	_ = os.WriteFile(out, []byte(b.String()), 0o644)
	if verdict == "ABORT" {
		return ExitVerdictBad
	}
	return ExitOK
}

// ── cross-cli-vote / audit-consensus ───────────────────────────────────────

func writeCrossCLIVote(out string, workers []string, now string) int {
	anyFail := false
	passCount := 0
	total := 0
	perCLI := []string{}
	for _, w := range workers {
		v := extractVerdict(w)
		name := strings.TrimSuffix(filepath.Base(w), ".md")
		verdictLabel := v
		if verdictLabel == "" {
			verdictLabel = "MISSING"
		}
		perCLI = append(perCLI, fmt.Sprintf("%s=%s", name, verdictLabel))
		total++
		switch v {
		case "PASS":
			passCount++
		case "FAIL":
			anyFail = true
		}
	}

	quorum := (total + 1) / 2
	verdict := "WARN"
	reason := fmt.Sprintf("cross-cli-vote: %d of %d PASS (below quorum=%d); ships per fluent default unless EVOLVE_STRICT_AUDIT=1",
		passCount, total, quorum)
	if anyFail {
		verdict = "FAIL"
		reason = "cross-cli-vote: at least one CLI returned FAIL (veto rule)"
	} else if passCount >= quorum {
		verdict = "PASS"
		reason = fmt.Sprintf("cross-cli-vote: %d of %d CLIs returned PASS (quorum=%d)",
			passCount, total, quorum)
	}

	failVetoActive := "no"
	failVetoFlag := "0"
	if anyFail {
		failVetoActive = "yes"
		failVetoFlag = "1"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Verdict: %s\n\n", verdict)
	fmt.Fprintf(&b, "# Aggregated Cross-CLI Consensus Audit\n\n")
	fmt.Fprintf(&b, "_Aggregated by aggregator.sh at %s. CLIs voting: %d (PASS=%d, FAIL=%s-veto-active=%s, quorum=%d)._\n\n",
		now, total, passCount, failVetoFlag, failVetoActive, quorum)
	fmt.Fprintf(&b, "## Consensus Decision\n\n")
	fmt.Fprintf(&b, "**Verdict**: %s\n\n", verdict)
	fmt.Fprintf(&b, "**Reason**: %s\n\n", reason)
	fmt.Fprintf(&b, "**Per-CLI verdicts**: %s\n\n", strings.Join(perCLI, ", "))
	fmt.Fprintf(&b, "**Protocol**: MAJORITY-PASS with FAIL-VETO. Any FAIL forces consensus FAIL (defends against false-positive PASS from sycophantic same-vendor agreement). >= quorum PASS with no FAIL → consensus PASS. Otherwise WARN.\n\n")
	fmt.Fprintf(&b, "## Per-CLI Audit Reports\n\n")
	for _, w := range workers {
		name := strings.TrimSuffix(filepath.Base(w), ".md")
		fmt.Fprintf(&b, "### Worker: %s\n\n", name)
		body, _ := os.ReadFile(w)
		b.Write(body)
		b.WriteString("\n\n")
	}
	_ = os.WriteFile(out, []byte(b.String()), 0o644)
	if verdict == "FAIL" {
		return ExitVerdictBad
	}
	return ExitOK
}

// ErrUnknownPhase is returned when phase doesn't match any merge mode.
var ErrUnknownPhase = errors.New("aggregator: unknown phase")
