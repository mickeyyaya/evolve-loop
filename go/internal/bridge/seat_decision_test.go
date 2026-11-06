package bridge

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

func TestConfig_CarriesTheSeatDecisionAndZeroMeansTheStoredLogin(t *testing.T) {
	t.Parallel()
	field, ok := reflect.TypeOf(Config{}).FieldByName("Decision")
	if !ok || field.Type != reflect.TypeOf(seatlogin.Decision{}) {
		t.Fatalf("field=%+v ok=%v", field, ok)
	}
	if !reflect.ValueOf(Config{}).FieldByName("Decision").IsZero() {
		t.Fatal("the zero Config holds a decision")
	}
}

func TestManifest_SeatLoginIsIsolatedForClaudeSeatsAndStoredForTheRest(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"claude-tmux": "isolated", "claude-p": "isolated", "claude-cloud": "stored",
		"codex-tmux": "", "codex": "", "agy-tmux": "", "agy": "", "agy-claude-tmux": "", "ollama-tmux": "",
	}
	for cli, seat := range want {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatalf("%s: %v", cli, err)
		}
		if m.SeatLogin != seat {
			t.Errorf("%s: seat_login %q, want %q", cli, m.SeatLogin, seat)
		}
	}
}

func TestParseManifest_ASeatLoginOutsideTheClosedSetIsRefused(t *testing.T) {
	t.Parallel()
	body := `{"schema_version":1,"cli":"x","binary":"x","seat_login":"both","interactive_prompts":[]}`
	if _, err := parseManifestWithStderr("x", []byte(body), io.Discard); err == nil || !strings.Contains(err.Error(), "seat_login") {
		t.Fatalf("err=%v", err)
	}
}

func TestSeatLoginValues_SelectTheDecisionOfEachManifest(t *testing.T) {
	t.Parallel()
	if SeatLoginIsolated == SeatLoginStored || SeatLoginIsolated == "" || SeatLoginStored == "" {
		t.Fatalf("isolated=%q stored=%q", SeatLoginIsolated, SeatLoginStored)
	}
	for _, c := range []struct{ cli, want string }{
		{"claude-tmux", SeatLoginIsolated}, {"claude-p", SeatLoginIsolated}, {"claude-cloud", SeatLoginStored},
	} {
		m, err := LoadManifest(c.cli)
		if err != nil || m.SeatLogin != c.want {
			t.Errorf("%s: seat_login %q err=%v, want %q", c.cli, m.SeatLogin, err, c.want)
		}
	}
	for _, v := range []string{SeatLoginIsolated, SeatLoginStored} {
		body := `{"schema_version":1,"cli":"x","binary":"x","seat_login":"` + v + `","interactive_prompts":[]}`
		m, err := parseManifestWithStderr("x", []byte(body), io.Discard)
		if err != nil || m.SeatLogin != v {
			t.Errorf("value %q: got %q err=%v", v, m.SeatLogin, err)
		}
	}
}
