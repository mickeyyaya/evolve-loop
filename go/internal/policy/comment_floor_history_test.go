package policy_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCommentFloorConfig_HistoryStageDefaultsToShadow(t *testing.T) {
	t.Parallel()

	p, err := policy.Parse([]byte(`{"comment_floor": {"stage": "enforce"}}`))
	if err != nil {
		t.Fatal(err)
	}

	got := p.CommentFloorConfig().HistoryStage.Stage()
	if got != config.StageShadow {
		t.Errorf("history_stage unset = %v, want shadow", got)
	}
	if p.CommentFloorConfig().HistoryStage.Stage() == config.StageEnforce {
		t.Error("comment_floor.stage enforce must not enforce the history floor")
	}
}

func TestCommentFloorConfig_HistoryStageTakesTheGateWords(t *testing.T) {
	t.Parallel()

	for word, want := range map[string]config.Stage{"off": config.StageOff, "0": config.StageOff, "shadow": config.StageShadow, "enforce": config.StageEnforce} {
		p, err := policy.Parse([]byte(`{"comment_floor": {"history_stage": "` + word + `"}}`))
		if err != nil {
			t.Fatalf("%q: %v", word, err)
		}
		if got := p.CommentFloorConfig().HistoryStage.Stage(); got != want {
			t.Errorf("%q = %v, want %v", word, got, want)
		}
	}
}

func TestPolicyParse_RefusesAHistoryStageWordThatIsNotAGateWord(t *testing.T) {
	t.Parallel()

	if _, err := policy.Parse([]byte(`{"comment_floor": {"history_stage": "advisory"}}`)); err == nil {
		t.Fatal("advisory is not a gate word")
	}
}

func TestLoadOrDefaults_ARefusedPolicyFailsTheHistoryStageClosedToEnforce(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"comment_floor": {"history_stage": "bogus"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := policy.LoadOrDefaults(path, io.Discard).CommentFloorConfig().HistoryStage.Stage()

	if got != config.StageEnforce {
		t.Errorf("a refused policy runs the history floor at %v, want enforce", got)
	}
}

func TestStageKeys_NameTheHistoryStage(t *testing.T) {
	t.Parallel()

	for _, key := range policy.StageKeys() {
		if key == "comment_floor.history_stage" {
			return
		}
	}
	t.Fatal("comment_floor.history_stage is not a registered stage key")
}
