package modelquery

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

type emptyClassifier struct{}

func (emptyClassifier) Classify(context.Context, string, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func TestRefresh_DetectFallbackRecordsWhyTheLiveQueryFailed(t *testing.T) {
	detect := map[string]string{"balanced": "detect-model"}
	tests := []struct {
		name       string
		lister     fakeLister
		classifier Classifier
		allowed    map[string][]string
		want       []string
	}{
		{"list failure", fakeLister{errOn: map[string]bool{"x": true}}, fakeClassifier{}, nil, []string{"list models", "boom"}},
		{"empty offering", fakeLister{ids: map[string][]string{"x": {}}}, fakeClassifier{}, nil, []string{"offered no models"}},
		{"family filter empties", fakeLister{ids: map[string][]string{"x": {"gpt-5"}}}, fakeClassifier{}, map[string][]string{"x": {"gemini"}}, []string{"allowed families", "gemini"}},
		{"classifier failure", fakeLister{ids: map[string][]string{"x": {"m1"}}}, fakeClassifier{errOn: map[string]bool{"x": true}}, nil, []string{"classify models", "classify boom"}},
		{"classifier maps nothing", fakeLister{ids: map[string][]string{"x": {"m1"}}}, emptyClassifier{}, nil, []string{"classify models", "no tier mapped"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, err := Refresh(context.Background(), RefreshDeps{
				CLIs: []string{"x"}, Lister: tt.lister, Classifier: tt.classifier,
				AllowedFamilies: tt.allowed, Fallback: map[string]map[string]string{"x": detect}, Now: fixedNow,
			})
			if err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			entry := cat.CLIs["x"]
			if entry.Source != modelcatalog.SourceDetect {
				t.Fatalf("source = %q, want detect", entry.Source)
			}
			for _, want := range tt.want {
				if !strings.Contains(entry.FallbackReason, want) {
					t.Errorf("FallbackReason %q does not say %q", entry.FallbackReason, want)
				}
			}
		})
	}
}

func TestRefresh_LiveEntryCarriesNoFallbackReason(t *testing.T) {
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs: []string{"x"}, Lister: fakeLister{ids: map[string][]string{"x": {"m1", "m2"}}},
		Classifier: fakeClassifier{}, Now: fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if entry := cat.CLIs["x"]; entry.Source != modelcatalog.SourceLive || entry.FallbackReason != "" {
		t.Errorf("live entry = source %q reason %q, want live with no reason", entry.Source, entry.FallbackReason)
	}
}
