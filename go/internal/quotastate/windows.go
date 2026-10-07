package quotastate

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	KindSession        = "session"
	KindWeek           = "week"
	KindFiveHour       = "5h"
	DirectionUsed      = "used"
	DirectionRemaining = "remaining"
	AnyScope           = "*"
	fullyUsedPct       = 100
)

type WindowSpec struct {
	SectionRegex       string                 `json:"section_regex,omitempty"`
	ModelsRegex        string                 `json:"models_regex,omitempty"`
	Labels             []WindowLabel          `json:"labels"`
	ValueRegex         string                 `json:"value_regex"`
	ValueDirection     string                 `json:"value_direction"`
	ResetRegex         string                 `json:"reset_regex,omitempty"`
	ExhaustedAtUsedPct float64                `json:"exhausted_at_used_pct,omitempty"`
	Scopes             map[string]ScopeTarget `json:"scopes,omitempty"`
}

type WindowLabel struct {
	Regex string `json:"regex"`
	Kind  string `json:"kind"`
	Scope string `json:"scope,omitempty"`
}

type ScopeTarget struct {
	Family   string `json:"family"`
	PerModel bool   `json:"per_model,omitempty"`
}

type UsageWindow struct {
	Scope       string     `json:"scope"`
	Kind        string     `json:"kind"`
	PercentUsed float64    `json:"percent_used"`
	ResetsText  string     `json:"resets_text,omitempty"`
	ResetsAt    *time.Time `json:"resets_at,omitempty"`
	Models      []string   `json:"models,omitempty"`
	Family      string     `json:"family,omitempty"`
	Model       string     `json:"model,omitempty"`
	Exhausted   bool       `json:"exhausted"`
}

type labelPattern struct {
	re          *regexp.Regexp
	kind, scope string
}

type windowPatterns struct {
	section, models, value, reset *regexp.Regexp
	labels                        []labelPattern
}

type windowDraft struct {
	UsageWindow
	hasValue, hasReset bool
}

type windowReader struct {
	patterns      windowPatterns
	sectionScope  string
	sectionModels []string
	drafts        []windowDraft
	open          bool
}

var zoneSuffixRE = regexp.MustCompile(`^(.*?)\s*\(([^()]+)\)\s*$`)

var windowKinds = map[string]bool{KindSession: true, KindWeek: true, KindFiveHour: true}

func (s WindowSpec) Validate() error {
	_, err := s.compile()
	return err
}

func ReadWindows(spec WindowSpec, pane string, now time.Time) []UsageWindow {
	patterns, err := spec.compile()
	if err != nil {
		return nil
	}
	r := windowReader{patterns: patterns}
	for _, line := range strings.Split(pane, "\n") {
		r.read(line)
	}
	return r.settled(spec, now)
}

func (s WindowSpec) compile() (windowPatterns, error) {
	var p windowPatterns
	for _, field := range []struct {
		dst                 **regexp.Regexp
		name, source, group string
		required            bool
	}{
		{&p.value, "value_regex", s.ValueRegex, "pct", true},
		{&p.section, "section_regex", s.SectionRegex, "scope", false},
		{&p.models, "models_regex", s.ModelsRegex, "models", false},
		{&p.reset, "reset_regex", s.ResetRegex, "reset", false},
	} {
		re, err := compileNamed(field.source, field.group, field.required)
		if err != nil {
			return windowPatterns{}, fmt.Errorf("%s: %w", field.name, err)
		}
		*field.dst = re
	}
	if s.ValueDirection != DirectionUsed && s.ValueDirection != DirectionRemaining {
		return windowPatterns{}, fmt.Errorf("value_direction: is %q; want %q or %q", s.ValueDirection, DirectionUsed, DirectionRemaining)
	}
	labels, err := compileLabels(s.Labels)
	if err != nil {
		return windowPatterns{}, err
	}
	p.labels = labels
	return p, s.checkScopes()
}

func compileNamed(source, group string, required bool) (*regexp.Regexp, error) {
	if source == "" {
		if required {
			return nil, errors.New("is required")
		}
		return nil, nil
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return nil, err
	}
	if re.SubexpIndex(group) < 0 {
		return nil, fmt.Errorf("names no %q group", group)
	}
	return re, nil
}

func compileLabels(labels []WindowLabel) ([]labelPattern, error) {
	if len(labels) == 0 {
		return nil, errors.New("labels: needs at least one label")
	}
	out := make([]labelPattern, 0, len(labels))
	for i, label := range labels {
		re, err := regexp.Compile(label.Regex)
		if label.Regex == "" || err != nil {
			return nil, fmt.Errorf("labels[%d].regex %q does not compile", i, label.Regex)
		}
		if !windowKinds[label.Kind] {
			return nil, fmt.Errorf("labels[%d].kind is %q; want %s, %s or %s", i, label.Kind, KindSession, KindWeek, KindFiveHour)
		}
		out = append(out, labelPattern{re: re, kind: label.Kind, scope: label.Scope})
	}
	return out, nil
}

func (s WindowSpec) checkScopes() error {
	for scope, target := range s.Scopes {
		if target.Family == "" {
			return fmt.Errorf("scopes: %q names no family", scope)
		}
	}
	return nil
}

func (r *windowReader) read(line string) {
	if scope, ok := capture(r.patterns.section, line, "scope"); ok {
		r.sectionScope, r.sectionModels, r.open = scope, nil, false
		return
	}
	if models, ok := capture(r.patterns.models, line, "models"); ok {
		r.sectionModels = splitList(models)
		return
	}
	if label, scope, ok := r.patterns.label(line); ok {
		r.drafts = append(r.drafts, windowDraft{UsageWindow: UsageWindow{Kind: label.kind, Scope: r.scopeOf(label, scope), Models: slices.Clone(r.sectionModels)}})
		r.open = true
	}
	if r.open {
		r.fill(&r.drafts[len(r.drafts)-1], line)
	}
}

func (p windowPatterns) label(line string) (labelPattern, string, bool) {
	for _, l := range p.labels {
		if m := l.re.FindStringSubmatch(line); m != nil {
			scope := ""
			if i := l.re.SubexpIndex("scope"); i >= 0 {
				scope = strings.TrimSpace(m[i])
			}
			return l, scope, true
		}
	}
	return labelPattern{}, "", false
}

func (r *windowReader) scopeOf(label labelPattern, captured string) string {
	switch {
	case captured != "":
		return captured
	case label.scope != "":
		return label.scope
	}
	return r.sectionScope
}

func (r *windowReader) fill(d *windowDraft, line string) {
	if pct, ok := capture(r.patterns.value, line, "pct"); ok && !d.hasValue {
		if v, err := strconv.ParseFloat(pct, 64); err == nil {
			d.PercentUsed, d.hasValue = v, true
		}
	}
	if reset, ok := capture(r.patterns.reset, line, "reset"); ok && !d.hasReset {
		d.ResetsText, d.hasReset = reset, true
	}
	r.open = !d.hasValue || !d.hasReset
}

func (r *windowReader) settled(spec WindowSpec, now time.Time) []UsageWindow {
	threshold := spec.ExhaustedAtUsedPct
	if threshold <= 0 {
		threshold = fullyUsedPct
	}
	var out []UsageWindow
	for _, d := range r.drafts {
		if !d.hasValue {
			continue
		}
		w := d.UsageWindow
		if spec.ValueDirection == DirectionRemaining {
			w.PercentUsed = fullyUsedPct - w.PercentUsed
		}
		w.PercentUsed = math.Round(w.PercentUsed*100) / 100
		w.Exhausted = w.PercentUsed >= threshold
		w.Family, w.Model = spec.target(w.Scope)
		if at, ok := resetsAt(w.ResetsText, now); ok {
			at = at.UTC()
			w.ResetsAt = &at
		}
		out = append(out, w)
	}
	return out
}

func (w UsageWindow) ExhaustsFamily() bool { return w.Exhausted && w.Model == "" }

func (w UsageWindow) Evidence() string {
	resets := w.ResetsText
	if resets == "" {
		resets = "at a time the screen does not state"
	}
	return fmt.Sprintf("%s %s window %g%% used, resets %s", w.Scope, w.Kind, w.PercentUsed, resets)
}

func (s WindowSpec) target(scope string) (family, model string) {
	t, ok := s.Scopes[scope]
	if !ok {
		t, ok = s.Scopes[AnyScope]
	}
	switch {
	case !ok:
		return "", ""
	case t.PerModel:
		return t.Family, scope
	}
	return t.Family, ""
}

func resetsAt(text string, now time.Time) (time.Time, bool) {
	if m := zoneSuffixRE.FindStringSubmatch(text); m != nil {
		text = m[1]
		if loc, err := time.LoadLocation(m[2]); err == nil {
			now = now.In(loc)
		}
	}
	if text == "" {
		return time.Time{}, false
	}
	if d, err := time.ParseDuration(strings.ReplaceAll(text, " ", "")); err == nil && d > 0 {
		return now.Add(d), true
	}
	return ParseResetWhen(text, now)
}

func capture(re *regexp.Regexp, line, name string) (string, bool) {
	if re == nil {
		return "", false
	}
	m := re.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[re.SubexpIndex(name)]), true
}

func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func StatesOf(windows []UsageWindow, now time.Time) []QuotaState {
	var out []QuotaState
	index := map[string]int{}
	for _, w := range windows {
		if w.Family == "" || w.Model != "" {
			continue
		}
		i, seen := index[w.Family]
		if !seen {
			i, index[w.Family] = len(out), len(out)
			out = append(out, QuotaState{Family: w.Family, Source: SourceProbed, ObservedAt: now})
		}
		out[i].Buckets = append(out[i].Buckets, w.bucket())
		out[i].Exhausted = out[i].Exhausted || w.ExhaustsFamily()
	}
	return out
}

func (w UsageWindow) bucket() Bucket {
	b := Bucket{Name: w.Kind, Label: w.Scope, UsedFraction: clamp01(w.PercentUsed / fullyUsedPct), ResetRaw: w.ResetsText}
	if w.ResetsAt != nil {
		b.ResetAt = *w.ResetsAt
	}
	return b
}
