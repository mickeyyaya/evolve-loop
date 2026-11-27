package bridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	headlessSourceStderr = "stderr"
	headlessRefPrompt    = "prompt"
	headlessRefControl   = "control"
	headlessRefRegex     = "regex"
	headlessRefParts     = 3
	headlessTailBytes    = 1 << 20
	headlessStderrLines  = 5
)

type HeadlessRule struct {
	Pattern   string `json:"pattern"`
	Source    string `json:"source"`
	Regex     string `json:"regex,omitempty"`
	RegexFrom string `json:"regex_from,omitempty"`
}

type HeadlessOutcomeSpec struct {
	Rules []HeadlessRule `json:"rules"`
}

type HeadlessEvidence struct {
	ResultText string
	Stderr     string
}

type headlessResult struct {
	text    string
	isError bool
	found   bool
}

func (r HeadlessRule) pattern() (*regexp.Regexp, error) {
	expr := r.Regex
	if r.RegexFrom != "" {
		from, err := headlessRegexFrom(r.RegexFrom)
		if err != nil {
			return nil, err
		}
		expr = from
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("bridge:headless_outcome: pattern=%s: %w", r.Pattern, err)
	}
	return re, nil
}

func headlessRegexFrom(ref string) (string, error) {
	parts := strings.SplitN(ref, ":", headlessRefParts)
	if len(parts) != headlessRefParts {
		return "", fmt.Errorf("bridge:headless_outcome: regex_from=%q is not cli:kind:name", ref)
	}
	m, err := loadManifestRaw(parts[0])
	if err != nil {
		return "", err
	}
	switch parts[1] {
	case headlessRefPrompt:
		for _, p := range m.InteractivePrompts {
			if p.Name == parts[2] {
				return p.Regex, nil
			}
		}
	case headlessRefRegex:
		if expr, ok := m.NamedRegexes[parts[2]]; ok {
			return expr, nil
		}
	case headlessRefControl:
		if spec, ok := m.Control(parts[2]); ok && spec.ExhaustedRegex != "" {
			return spec.ExhaustedRegex, nil
		}
	}
	return "", fmt.Errorf("bridge:headless_outcome: regex_from=%q names nothing", ref)
}

func (s *HeadlessOutcomeSpec) Classify(ev HeadlessEvidence) (string, error) {
	if s == nil {
		return "", nil
	}
	for _, rule := range s.Rules {
		text := ev.ResultText
		if rule.Source == headlessSourceStderr {
			text = ev.Stderr
		}
		re, err := rule.pattern()
		if err != nil {
			return "", err
		}
		if re.MatchString(text) {
			return rule.Pattern, nil
		}
	}
	return "", nil
}

func readHeadlessResult(stdoutPath string) headlessResult {
	data := readTail(stdoutPath, headlessTailBytes)
	var last headlessResult
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), headlessTailBytes)
	plain := ""
	for sc.Scan() {
		doc, isEvent := parseStreamEvent(sc.Bytes())
		switch {
		case isEvent && doc.Type == "result":
			last = headlessResult{text: doc.Result, isError: doc.IsError, found: true}
		case !isEvent:
			if line := strings.TrimSpace(sc.Text()); line != "" {
				plain = line
			}
		}
	}
	if !last.found {
		return headlessResult{text: plain, isError: true}
	}
	return last
}

type streamEvent struct {
	Type    string `json:"type"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

func parseStreamEvent(line []byte) (streamEvent, bool) {
	var doc streamEvent
	if json.Unmarshal(line, &doc) != nil || doc.Type == "" {
		return streamEvent{}, false
	}
	return doc, true
}

func readTail(path string, limit int) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if len(data) > limit {
		return data[len(data)-limit:]
	}
	return data
}

func headlessEvidenceFor(cfg *Config) HeadlessEvidence {
	res := readHeadlessResult(cfg.StdoutLog)
	if res.found && !res.isError {
		return HeadlessEvidence{}
	}
	return HeadlessEvidence{ResultText: res.text, Stderr: tailNonEmpty(readTail(cfg.StderrLog, headlessTailBytes), headlessStderrLines)}
}

func tailNonEmpty(data []byte, n int) string {
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			kept = append(kept, line)
		}
	}
	if len(kept) > n {
		kept = kept[len(kept)-n:]
	}
	return strings.Join(kept, "\n")
}
