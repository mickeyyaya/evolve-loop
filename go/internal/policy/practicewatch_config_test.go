package policy

import (
	"strings"
	"testing"
)

func TestPracticeWatchConfig_AbsentBlockResolvesTheCompiledTimeout(t *testing.T) {
	cfg, err := loadCIWatchPolicy(t, `{}`).PracticeWatchConfig()

	if err != nil {
		t.Fatalf("PracticeWatchConfig: %v", err)
	}
	if cfg.TimeoutS == nil || *cfg.TimeoutS != 20 {
		t.Errorf("TimeoutS = %v, want the compiled default 20", cfg.TimeoutS)
	}
	if len(cfg.Sources) != 0 {
		t.Errorf("Sources = %v, want none", cfg.Sources)
	}
}

func TestPracticeWatchConfig_ThePresentBlockOverridesTheTimeoutAndKeepsEverySourceField(t *testing.T) {
	body := `{"practice_watch":{"timeout_s":5,"sources":[{"id":"codex-changelog","enabled":true},{"id":"mine","family":"claude","kind":"page","locator":"https://example.test/p"}]}}`

	cfg, err := loadCIWatchPolicy(t, body).PracticeWatchConfig()

	if err != nil {
		t.Fatalf("PracticeWatchConfig: %v", err)
	}
	var resolved PracticeWatchPolicy = cfg
	var sources []PracticeWatchSource = resolved.Sources
	if *resolved.TimeoutS != 5 {
		t.Errorf("TimeoutS = %d, want 5", *resolved.TimeoutS)
	}
	cfg.Sources = sources
	if len(cfg.Sources) != 2 || cfg.Sources[0].Enabled == nil || !*cfg.Sources[0].Enabled || cfg.Sources[1].Locator != "https://example.test/p" {
		t.Errorf("Sources = %+v", cfg.Sources)
	}
}

func TestPracticeWatchConfig_RefusesWhatCannotRun(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"zero timeout": {`{"practice_watch":{"timeout_s":0}}`, "timeout_s must be > 0"},
		"blank id":     {`{"practice_watch":{"sources":[{"id":" "}]}}`, "id must not be blank"},
		"duplicate id": {`{"practice_watch":{"sources":[{"id":"a"},{"id":"a"}]}}`, `duplicate source id "a"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadCIWatchPolicy(t, tc.body).PracticeWatchConfig()

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestPracticeWatchPolicy_DecodesStrictlyAtTheBlockAndAtEverySource(t *testing.T) {
	for name, body := range map[string]string{
		"unknown block key":  `{"practice_watch":{"timeout":5}}`,
		"unknown source key": `{"practice_watch":{"sources":[{"id":"a","url":"https://x.test"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(body))

			if err == nil || !strings.Contains(err.Error(), "practice_watch") {
				t.Errorf("Parse err = %v, want a practice_watch refusal", err)
			}
		})
	}
}

func TestPracticeWatchConfig_AcceptsTheTimeoutCapItself(t *testing.T) {
	cfg, err := loadCIWatchPolicy(t, `{"practice_watch":{"timeout_s":120}}`).PracticeWatchConfig()

	if err != nil || *cfg.TimeoutS != 120 {
		t.Errorf("cfg = %+v, err = %v, want 120 accepted", cfg, err)
	}
}

func TestPracticeWatchConfig_RefusesATimeoutOverTheCap(t *testing.T) {
	_, err := loadCIWatchPolicy(t, `{"practice_watch":{"timeout_s":121}}`).PracticeWatchConfig()

	if err == nil || !strings.Contains(err.Error(), "timeout_s must be at most 120") {
		t.Errorf("err = %v, want a refusal of 121", err)
	}
}
