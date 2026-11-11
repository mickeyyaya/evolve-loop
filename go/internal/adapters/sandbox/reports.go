package sandbox

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	LogShowPath         = "/usr/bin/log"
	SandboxExecPath     = sandboxExecPath
	ReportCap           = 20
	RuleCellFence       = "cell.fence"
	logShowTimeout      = 10 * time.Second
	reportFenceDeadline = 60 * time.Second
	reportPollInterval  = time.Second
)

type Report struct {
	Denied                  bool
	Operation               string
	Target, Rule, Invariant string
	Process, PID            string
}

var reportLine = regexp.MustCompile(`^Sandbox: (?:([^\n]*?)\((\d+)\) )?(allow|deny)(?:\(\d+\))? (\S+) ([^\n]+)\n` + regexp.QuoteMeta(CellMessagePrefix) + `\S+ rule=(\S+) inv=(I\d+)`)

func ParseReports(raw []byte) []Report {
	var out []Report
	seen, perRule := map[string]bool{}, map[string]int{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var event struct {
			EventMessage string `json:"eventMessage"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		m := reportLine.FindStringSubmatch(event.EventMessage)
		if m == nil {
			continue
		}
		key := m[6] + "\x00" + m[5] + "\x00" + m[1]
		if seen[key] || perRule[m[6]] >= ReportCap {
			continue
		}
		seen[key] = true
		perRule[m[6]]++
		out = append(out, Report{Denied: m[3] == "deny", Operation: m[4], Target: m[5], Rule: m[6], Invariant: m[7], Process: m[1], PID: m[2]})
	}
	return out
}

func Reports(ctx context.Context, binary, tag string, since time.Time) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, logShowTimeout)
	defer cancel()
	predicate := fmt.Sprintf(`sender == "Sandbox" AND eventMessage CONTAINS "%s%s "`, CellMessagePrefix, tag)
	return sysexec.Command(ctx, binary, "show", "--style", "ndjson", "--start", since.Local().Format(time.DateTime), "--predicate", predicate).Output()
}

func FenceMessage(tag string) string {
	return CellMessagePrefix + tag + " rule=" + RuleCellFence
}

func Fenced(raw []byte, tag string) bool {
	want := FenceMessage(tag)
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var event struct {
			EventMessage string `json:"eventMessage"`
		}
		if json.Unmarshal(line, &event) == nil && strings.Contains(event.EventMessage, want) {
			return true
		}
	}
	return false
}

func WriteFence(ctx context.Context, binary, tag string) error {
	ctx, cancel := context.WithTimeout(ctx, logShowTimeout)
	defer cancel()
	profile := fmt.Sprintf(`(version 1)(allow default)(allow file-read-data (literal "/dev/null") (with report) (with message %q))`, FenceMessage(tag))
	return sysexec.Command(ctx, binary, "-p", profile, "/bin/cat", "/dev/null").Run()
}

type ReportSource struct {
	Reports  func(ctx context.Context, tag string, since time.Time) ([]byte, error)
	Fence    func(ctx context.Context, tag string) error
	Now      func() time.Time
	Sleep    func(time.Duration)
	Deadline time.Duration
	Warn     func(format string, args ...any)
}

func (s ReportSource) warn(format string, args ...any) {
	if s.Warn != nil {
		s.Warn(format, args...)
	}
}

func (s ReportSource) Cell(parent context.Context, tag string, since time.Time) ([]byte, bool, error) {
	ctx := context.WithoutCancel(parent)
	if s.Fence == nil {
		raw, err := s.Reports(ctx, tag, since)
		return raw, true, err
	}
	if err := s.Fence(ctx, tag); err != nil {
		s.warn("write the fence: %v", err)
		raw, err := s.Reports(ctx, tag, since)
		return raw, false, err
	}
	start, deadline := s.Now(), cmp.Or(s.Deadline, reportFenceDeadline)
	for {
		raw, err := s.Reports(ctx, tag, since)
		if err != nil || Fenced(raw, tag) {
			return raw, true, err
		}
		if elapsed := s.Now().Sub(start); elapsed >= deadline || parent.Err() != nil {
			s.warn("no fence of the cell after %s; its reads count as unmeasured", elapsed.Round(time.Second))
			return raw, false, nil
		}
		s.Sleep(reportPollInterval)
	}
}
