package modelquery

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

const agyModelsWithClaudeRows = `Fetching available models...
gemini-3.8-flash-high	Gemini 3.8 Flash (High)
gemini-3.8-flash-medium	Gemini 3.8 Flash (Medium)
gemini-3.8-flash-low	Gemini 3.8 Flash (Low)
gemini-3.7-flash-low	Gemini 3.7 Flash (Low)
gemini-3.1-pro-high	Gemini 3.1 Pro (High)
gemini-3.1-pro-low	Gemini 3.1 Pro (Low)
claude-opus-4-6-high	Claude Opus 4.6 (High)
claude-opus-5-5-low	Claude Opus 5.5 (Low)
claude-opus-5-5-medium	Claude Opus 5.5 (Medium)
claude-opus-5-5-high	Claude Opus 5.5 (High)
claude-sonnet-4-6-low	Claude Sonnet 4.6 (Low)
claude-sonnet-5-5-low	Claude Sonnet 5.5 (Low)
claude-sonnet-5-5-medium	Claude Sonnet 5.5 (Medium)
claude-sonnet-5-5-high	Claude Sonnet 5.5 (High)
gpt-oss-120b-medium	GPT-OSS 120B (Medium)
`

type familyKeepingClassifier struct {
	t       *testing.T
	family  map[string]string
	answers map[string]map[string]string
}

func (c familyKeepingClassifier) Classify(_ context.Context, cli string, ids []string) (map[string]string, error) {
	for _, id := range ids {
		if FamilyOf(id) != c.family[cli] {
			c.t.Errorf("%s was offered %q, a %s model; its candidates are filtered to %s", cli, id, FamilyOf(id), c.family[cli])
		}
	}
	return c.answers[cli], nil
}

func TestRefresh_OneRoutedListingSplitsIntoAGeminiEntryAndAClaudeEntry(t *testing.T) {
	routed := &agyRowsLister{}
	classifier := familyKeepingClassifier{
		t:      t,
		family: map[string]string{"agy": "gemini", "agy-claude": "claude"},
		answers: map[string]map[string]string{
			"agy":        {"fast": "Gemini 3.7 Flash (Low)", "balanced": "Gemini 3.8 Flash (High)", "deep": "Gemini 3.1 Pro (High)", "top": "Gemini 3.1 Pro (High)"},
			"agy-claude": {"fast": "Claude Sonnet 4.6 (Low)", "balanced": "Claude Sonnet 5.5 (High)", "deep": "Claude Opus 4.6 (High)", "top": "Claude Opus 5.5 (High)"},
		},
	}

	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:            []string{"agy", "agy-claude"},
		Lister:          DefaultRouter(nil, routed),
		Classifier:      classifier,
		AllowedFamilies: map[string][]string{"agy": {"gemini"}, "agy-claude": {"claude"}},
		Now:             fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if routed.listings != 2 {
		t.Errorf("the router asked the routed lister %d times, want once per catalog entry", routed.listings)
	}
	want := map[string]map[string]string{
		"agy":        {"fast": "Gemini 3.8 Flash (Low)", "balanced": "Gemini 3.8 Flash (High)", "deep": "Gemini 3.1 Pro (High)", "top": "Gemini 3.1 Pro (High)"},
		"agy-claude": {"fast": "Claude Sonnet 5.5 (Low)", "balanced": "Claude Sonnet 5.5 (High)", "deep": "Claude Opus 5.5 (High)", "top": "Claude Opus 5.5 (High)"},
	}
	for cli, tiers := range want {
		entry, ok := cat.CLIs[cli]
		if !ok {
			t.Errorf("catalog has no %s entry: %v", cli, cat.CLIs)
			continue
		}
		if entry.Source != modelcatalog.SourceLive || !reflect.DeepEqual(entry.TierModels, tiers) {
			t.Errorf("%s entry = source %q tiers %v, want live %v", cli, entry.Source, entry.TierModels, tiers)
		}
	}
	if got := cat.CLIs["agy-claude"].Available; slices.ContainsFunc(got, func(id string) bool { return FamilyOf(id) != "claude" }) {
		t.Errorf("agy-claude available = %v, want Claude models only", got)
	}
}

type agyRowsLister struct{ listings int }

func (*agyRowsLister) Families() []string { return []string{"agy", "agy-claude"} }

func (l *agyRowsLister) List(context.Context, string) ([]string, error) {
	l.listings++
	var names []string
	for _, line := range strings.Split(agyModelsWithClaudeRows, "\n") {
		if _, display, ok := strings.Cut(line, "\t"); ok {
			names = append(names, display)
		}
	}
	return names, nil
}
