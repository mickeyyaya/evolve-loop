// Package aggregator merges fan-out worker artifacts into one canonical phase artifact.
// The merge is deterministic: no model call, so a worker cannot forge the output.
package aggregator

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ExitOK         = 0
	ExitVerdictBad = 1
	ExitUsageErr   = 2
)

type MergeMode int

const (
	ModeUnknown MergeMode = iota
	ModeConcat
	ModeVerdict
	ModeLessons
	ModePlanReview
	ModeCrossCLIVote
)

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

type Inputs struct {
	Phase    string
	Output   string
	Workers  []string
	ReadFile func(string) ([]byte, error)
	Now      func() time.Time
}

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
	if in.ReadFile == nil {
		in.ReadFile = defaultReadFile
	}
	if !workersReadable(in.Workers, in.ReadFile, logf) {
		return ExitUsageErr
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
	defer func() { _ = os.Remove(tmp) }()

	now := in.Now().UTC().Format("2006-01-02T15:04:05Z")
	rc, err := writeMerged(mode, tmp, in, now)
	if err != nil {
		logf("error: read worker: %v", err)
		return ExitUsageErr
	}

	if err := os.Rename(tmp, in.Output); err != nil {
		logf("error: write %s: %v", in.Output, err)
		return ExitUsageErr
	}
	return rc
}

func workersReadable(workers []string, readFile func(string) ([]byte, error), logf func(string, ...any)) bool {
	for _, w := range workers {
		info, err := os.Stat(w)
		if err != nil {
			logf("error: worker artifact not found: %s", w)
			return false
		}
		if info.Size() == 0 {
			logf("error: worker artifact is empty: %s", w)
			return false
		}
		if _, err := readFile(w); err != nil {
			logf("error: worker artifact unreadable: %s: %v", w, err)
			return false
		}
	}
	return true
}

func writeMerged(mode MergeMode, tmp string, in Inputs, now string) (int, error) {
	var rc int
	var err error
	switch mode {
	case ModeConcat:
		err = writeConcat(tmp, in.Phase, in.Workers, now, in.ReadFile)
	case ModeVerdict:
		rc, err = writeVerdict(tmp, in.Workers, now, in.ReadFile)
	case ModeLessons:
		err = writeLessons(tmp, in.Workers, now, in.ReadFile)
	case ModePlanReview:
		rc, err = writePlanReview(tmp, in.Workers, now, in.ReadFile)
	case ModeCrossCLIVote:
		rc, err = writeCrossCLIVote(tmp, in.Workers, now, in.ReadFile)
	}
	return rc, err
}

func defaultReadFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func writeConcat(out, phase string, workers []string, now string, readFile func(string) ([]byte, error)) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Aggregated %s Report\n\n", phase)
	fmt.Fprintf(&b, "_Aggregated by aggregator.sh at %s. Workers: %d._\n\n", now, len(workers))
	if err := appendWorkerSections(&b, "## Worker: %s\n\n", workers, readFile); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

var verdictLineRe = regexp.MustCompile(`(?i)^\s*verdict:\s*(\S+)`)

func extractVerdict(path string) string {
	return strings.ToUpper(scanFirstCapture(path, verdictLineRe))
}

func scanFirstCapture(path string, re *regexp.Regexp) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		m := re.FindStringSubmatch(scanner.Text())
		if m != nil {
			return m[1]
		}
	}
	return ""
}

func writeVerdict(out string, workers []string, now string, readFile func(string) ([]byte, error)) (int, error) {
	anyFail := false
	allPass := true
	for _, w := range workers {
		v := extractVerdict(w)
		switch v {
		case "PASS":
		case "FAIL":
			anyFail = true
			allPass = false
		case "WARN":
			allPass = false
		default:
			allPass = false
		}
	}
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
	if err := appendWorkerSections(&b, "## Worker: %s\n\n", workers, readFile); err != nil {
		return ExitUsageErr, err
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return ExitUsageErr, err
	}
	if verdict == "FAIL" {
		return ExitVerdictBad, nil
	}
	return ExitOK, nil
}

var lessonHeadingRe = regexp.MustCompile(`(?i)^## Lesson:`)

func writeLessons(out string, workers []string, now string, readFile func(string) ([]byte, error)) error {
	var combined strings.Builder
	for _, w := range workers {
		body, err := readFile(w)
		if err != nil {
			return err
		}
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
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

var scoreLineRe = regexp.MustCompile(`(?i)^\s*score:\s*([0-9]*\.?[0-9]+)`)

func extractScore(path string) float64 {
	v, err := strconv.ParseFloat(scanFirstCapture(path, scoreLineRe), 64)
	if err != nil {
		return 0
	}
	return v
}

func writePlanReview(out string, workers []string, now string, readFile func(string) ([]byte, error)) (int, error) {
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
	if err := appendWorkerSections(&b, "## Worker: %s\n\n", workers, readFile); err != nil {
		return ExitUsageErr, err
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return ExitUsageErr, err
	}
	if verdict == "ABORT" {
		return ExitVerdictBad, nil
	}
	return ExitOK, nil
}

func writeCrossCLIVote(out string, workers []string, now string, readFile func(string) ([]byte, error)) (int, error) {
	perCLI, passCount, total, anyFail := tallyCrossCLIVotes(workers)
	verdict, reason, quorum := crossCLIDecision(passCount, total, anyFail)

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
	if err := appendWorkerSections(&b, "### Worker: %s\n\n", workers, readFile); err != nil {
		return ExitUsageErr, err
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return ExitUsageErr, err
	}
	if verdict == "FAIL" {
		return ExitVerdictBad, nil
	}
	return ExitOK, nil
}

func tallyCrossCLIVotes(workers []string) (perCLI []string, passCount, total int, anyFail bool) {
	perCLI = []string{}
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
	return perCLI, passCount, total, anyFail
}

func crossCLIDecision(passCount, total int, anyFail bool) (verdict, reason string, quorum int) {
	quorum = (total + 1) / 2
	verdict = "WARN"
	reason = fmt.Sprintf("cross-cli-vote: %d of %d PASS (below quorum=%d); ships per fluent default unless workflow.strict_audit",
		passCount, total, quorum)
	if anyFail {
		verdict = "FAIL"
		reason = "cross-cli-vote: at least one CLI returned FAIL (veto rule)"
	} else if passCount >= quorum {
		verdict = "PASS"
		reason = fmt.Sprintf("cross-cli-vote: %d of %d CLIs returned PASS (quorum=%d)",
			passCount, total, quorum)
	}
	return verdict, reason, quorum
}

func appendWorkerSections(b *strings.Builder, heading string, workers []string, readFile func(string) ([]byte, error)) error {
	for _, w := range workers {
		name := strings.TrimSuffix(filepath.Base(w), ".md")
		fmt.Fprintf(b, heading, name)
		body, err := readFile(w)
		if err != nil {
			return err
		}
		b.Write(body)
		b.WriteString("\n\n")
	}
	return nil
}
