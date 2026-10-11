package envelope

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
)

var (
	testLimits = Limits{BodyBytes: 64, SubjectBytes: 16, Kinds: DefaultKinds()}
	deadline   = time.Date(2026, 10, 9, 22, 0, 0, 0, time.UTC)
)

func note() Envelope {
	return Envelope{
		Schema: SchemaVersion,
		Kind:   KindNote,
		To:     []address.Address{{Selector: address.SelectPhase, Value: "audit"}},
		Body:   "the eval for the flag reads stdout",
		Refs:   []string{"build-report.md", "evals/flag.md"},
	}
}

func TestDefaultKinds_IsTheOpenRegistryOfTheSpec(t *testing.T) {
	want := []Kind{KindNote, KindRequest, KindReply, KindHandoff, KindFinding, KindEvidence, KindBroadcast}
	got := DefaultKinds()
	if len(got) != len(want) {
		t.Fatalf("DefaultKinds() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DefaultKinds()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	got[0] = "changed"
	if DefaultKinds()[0] != KindNote {
		t.Fatal("DefaultKinds must return a new slice each call")
	}
}

func TestValidate_AcceptsAWellFormedEnvelope(t *testing.T) {
	e := note()
	e.ID = "01JA0000000000000000000000"
	if err := e.Validate(testLimits); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}
}

func TestValidate_AcceptsAKindThatTheConfigAdds(t *testing.T) {
	e := note()
	e.Kind = "question"
	l := testLimits
	l.Kinds = append(DefaultKinds(), "question")
	if err := e.Validate(l); err != nil {
		t.Fatalf("Validate(kind question, registered) error = %v, want nil", err)
	}
	if err := e.Validate(testLimits); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Validate(kind question, not registered) error = %v, want ErrInvalid", err)
	}
}

func TestValidate_AcceptsABodyAndASubjectAtTheirCaps(t *testing.T) {
	e := note()
	e.Body = strings.Repeat("b", testLimits.BodyBytes)
	e.Subject = strings.Repeat("s", testLimits.SubjectBytes)
	if err := e.Validate(testLimits); err != nil {
		t.Fatalf("Validate(body and subject at the caps) error = %v, want nil", err)
	}
}

func TestValidate_ARequestCarriesACorrelationAndADeadline(t *testing.T) {
	e := note()
	e.Kind, e.CorrelationID, e.ExpiresAt = KindRequest, "q-1", &deadline
	if err := e.Validate(testLimits); err != nil {
		t.Fatalf("Validate(request with correlation and deadline) error = %v, want nil", err)
	}
}

func TestValidate_RefusesEachBrokenRule(t *testing.T) {
	cases := map[string]func(*Envelope){
		"another schema":                func(e *Envelope) { e.Schema = "msg/2.0" },
		"an unregistered kind":          func(e *Envelope) { e.Kind = "order" },
		"no recipient":                  func(e *Envelope) { e.To = nil },
		"a malformed recipient":         func(e *Envelope) { e.To = []address.Address{{Selector: address.SelectPhase}} },
		"a body over the cap":           func(e *Envelope) { e.Body = strings.Repeat("b", testLimits.BodyBytes+1) },
		"a subject over the cap":        func(e *Envelope) { e.Subject = strings.Repeat("s", testLimits.SubjectBytes+1) },
		"a subject with a line break":   func(e *Envelope) { e.Subject = "a\nb" },
		"an absolute ref":               func(e *Envelope) { e.Refs = []string{"/etc/passwd"} },
		"a ref that leaves the cycle":   func(e *Envelope) { e.Refs = []string{"../cycle-7/audit-report.md"} },
		"a ref that is not clean":       func(e *Envelope) { e.Refs = []string{"evals//flag.md"} },
		"an empty ref":                  func(e *Envelope) { e.Refs = []string{""} },
		"a request with no deadline":    func(e *Envelope) { e.Kind, e.CorrelationID = KindRequest, "q-1" },
		"a request with no correlation": func(e *Envelope) { e.Kind, e.ExpiresAt = KindRequest, &deadline },
		"a reply with no correlation":   func(e *Envelope) { e.Kind = KindReply },
		"a malformed id":                func(e *Envelope) { e.ID = "not-a-ulid" },
		"a negative hop":                func(e *Envelope) { e.Hop = -1 },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			e := note()
			breakIt(&e)
			if err := e.Validate(testLimits); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate error = %v, want ErrInvalid", err)
			}
		})
	}
}

type constantReader byte

func (r constantReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("entropy closed") }

func source(at time.Time, fill byte) IDSource {
	return IDSource{Now: func() time.Time { return at }, Entropy: constantReader(fill)}
}

func TestIDSource_EncodesTheTimeAndTheEntropyInCrockfordBase32(t *testing.T) {
	maxTime := time.UnixMilli(1<<48 - 1)
	cases := []struct {
		name string
		src  IDSource
		want string
	}{
		{"zero time and zero entropy", source(time.UnixMilli(0), 0x00), "00000000000000000000000000"},
		{"the largest time and full entropy", source(maxTime, 0xff), "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
		{"one millisecond and zero entropy", source(time.UnixMilli(1), 0x00), "00000000010000000000000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.src.New()
			if err != nil {
				t.Fatalf("New error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("New = %q, want %q", got, tc.want)
			}
			if !ValidID(got) {
				t.Fatalf("ValidID(%q) = false, want true", got)
			}
		})
	}
}

func TestIDSource_IDsSortByTime(t *testing.T) {
	t0 := time.Date(2026, 10, 9, 21, 0, 0, 0, time.UTC)
	early, _ := source(t0, 0xff).New()
	late, _ := source(t0.Add(time.Millisecond), 0x00).New()
	if !(early < late) {
		t.Fatalf("an id one millisecond later sorts first: %q >= %q", early, late)
	}
}

func TestIDSource_AnEntropyFailureIsAnError(t *testing.T) {
	_, err := IDSource{Now: time.Now, Entropy: failingReader{}}.New()
	if !errors.Is(err, ErrID) {
		t.Fatalf("New with a failed entropy read error = %v, want ErrID", err)
	}
}

func TestValidID_RefusesWhatTheEncoderNeverWrites(t *testing.T) {
	cases := map[string]string{
		"too short":             "0000000000000000000000000",
		"too long":              "000000000000000000000000000",
		"lower case":            "0000000000000000000000000a",
		"the excluded letter I": "0000000000000000000000000I",
		"the excluded letter U": "0000000000000000000000000U",
		"a time past 48 bits":   "80000000000000000000000000",
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			if ValidID(id) {
				t.Fatalf("ValidID(%q) = true, want false", id)
			}
		})
	}
}
