package modelquery

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

type perCLIDispatcher struct {
	replies map[string]string
	errs    map[string]error
	calls   map[string]int
}

func newPerCLIDispatcher() *perCLIDispatcher {
	return &perCLIDispatcher{replies: map[string]string{}, errs: map[string]error{}, calls: map[string]int{}}
}

func (d *perCLIDispatcher) DispatchPrompt(_ context.Context, cli, _ string) (string, error) {
	d.calls[cli]++
	return d.replies[cli], d.errs[cli]
}

var agyOffering = []string{"Gemini 3.8 Flash (Low)", "Gemini 3.8 Flash (High)", "Gemini 3.1 Pro (High)"}

const agyTierReply = `{"fast":"Gemini 3.8 Flash (Low)","balanced":"Gemini 3.8 Flash (High)","deep":"Gemini 3.1 Pro (High)","top":"Gemini 3.1 Pro (High)"}`

func TestChainClassifier_LaunchFailureFallsThroughToTheNextReadyCLI(t *testing.T) {
	d := newPerCLIDispatcher()
	d.errs["codex"] = errors.New("bridge: launch exit=1")
	d.replies["claude"] = agyTierReply
	var log bytes.Buffer
	chain := ChainClassifier{CLIs: []string{"codex", "claude", "agy"}, Dispatcher: d, Log: &log}

	got, err := chain.Classify(context.Background(), "agy", agyOffering)

	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got["balanced"] != "Gemini 3.8 Flash (High)" {
		t.Errorf("balanced = %q, want the second CLI's answer", got["balanced"])
	}
	if d.calls["codex"] != 1 || d.calls["claude"] != 1 || d.calls["agy"] != 0 {
		t.Errorf("dispatch calls = %v, want codex then claude once each and agy never", d.calls)
	}
	if !strings.Contains(log.String(), "cli=codex") || !strings.Contains(log.String(), "launch exit=1") {
		t.Errorf("the codex failure must be logged with its CLI and reason, log = %q", log.String())
	}
}

func TestChainClassifier_UnusableRepliesFallThroughToo(t *testing.T) {
	d := newPerCLIDispatcher()
	d.replies["codex"] = "I cannot help with that."
	d.replies["claude"] = `{"fast":"Gemini 9 Imaginary (Low)"}`
	d.replies["agy"] = agyTierReply
	var log bytes.Buffer
	chain := ChainClassifier{CLIs: []string{"codex", "claude", "agy"}, Dispatcher: d, Log: &log}

	got, err := chain.Classify(context.Background(), "agy", agyOffering)

	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got["fast"] != "Gemini 3.8 Flash (Low)" {
		t.Errorf("fast = %q, want agy's answer after two unusable replies", got["fast"])
	}
	for _, want := range []string{"cli=codex", "no JSON object", "cli=claude", "no JSON object mapped a tier"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log missing %q: %q", want, log.String())
		}
	}
}

func TestChainClassifier_ExhaustedChainNamesEveryFailure(t *testing.T) {
	d := newPerCLIDispatcher()
	d.errs["codex"] = errors.New("codex down")
	d.errs["claude"] = errors.New("claude walled")
	chain := ChainClassifier{CLIs: []string{"codex", "claude"}, Dispatcher: d}

	_, err := chain.Classify(context.Background(), "agy", agyOffering)

	if err == nil {
		t.Fatal("an exhausted chain must return an error")
	}
	for _, want := range []string{"codex down", "claude walled"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name the failure %q", err, want)
		}
	}
}

func TestChainClassifier_EmptyChainIsAnError(t *testing.T) {
	if _, err := (ChainClassifier{Dispatcher: newPerCLIDispatcher()}).Classify(context.Background(), "agy", agyOffering); err == nil {
		t.Fatal("a chain with no CLIs must error rather than report an empty tier map")
	}
}

func TestChainClassifier_CancelledContextStopsBeforeDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := newPerCLIDispatcher()
	d.replies["codex"] = agyTierReply

	_, err := (ChainClassifier{CLIs: []string{"codex", "claude"}, Dispatcher: d}).Classify(ctx, "agy", agyOffering)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(d.calls) != 0 {
		t.Errorf("a cancelled refresh must not launch any classifier, calls = %v", d.calls)
	}
}

func TestRefresh_ChainClassifierRecoversLiveTiersWhenTheFirstCLIFails(t *testing.T) {
	d := newPerCLIDispatcher()
	d.errs["codex"] = errors.New("bridge: launch exit=1")
	d.replies["claude"] = agyTierReply
	deps := RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     fakeLister{ids: map[string][]string{"agy": agyOffering}},
		Classifier: ChainClassifier{CLIs: []string{"codex", "claude", "agy"}, Dispatcher: d},
		Fallback:   map[string]map[string]string{"agy": {"balanced": "Gemini 3.7 Flash (High)"}},
		Now:        fixedNow,
	}

	cat, err := Refresh(context.Background(), deps)

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	entry := cat.CLIs["agy"]
	if entry.Source != modelcatalog.SourceLive {
		t.Fatalf("agy source = %q (reason %q), want live", entry.Source, entry.FallbackReason)
	}
	if m, ok := cat.DispatchModel("agy", "balanced"); !ok || m != "Gemini 3.8 Flash (High)" {
		t.Errorf("agy balanced at dispatch = (%q,%v), want the live 3.8 Flash pick", m, ok)
	}
}
