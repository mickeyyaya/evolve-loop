package address

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParse_EveryProductionOfTheGrammar(t *testing.T) {
	cases := []struct {
		in   string
		want Address
	}{
		{"role:builder", Address{Selector: SelectRole, Value: "builder"}},
		{"phase:audit", Address{Selector: SelectPhase, Value: "audit"}},
		{"phase:spec-verify", Address{Selector: SelectPhase, Value: "spec-verify"}},
		{"role:code_reviewer2", Address{Selector: SelectRole, Value: "code_reviewer2"}},
		{"dispatch:run-a/1850/build/p4242n1t0", Address{Selector: SelectDispatch, Value: "run-a/1850/build/p4242n1t0"}},
		{"dispatch:events/0/sub-x:y.z/p9n1", Address{Selector: SelectDispatch, Value: "events/0/sub-x:y.z/p9n1"}},
		{"*", Address{Selector: SelectAll}},
		{"cycle:1850/phase:audit", Address{Cycle: 1850, Selector: SelectPhase, Value: "audit"}},
		{"cycle:7/*", Address{Cycle: 7, Selector: SelectAll}},
		{"cycle:12/dispatch:r/12/tdd/p5n2", Address{Cycle: 12, Selector: SelectDispatch, Value: "r/12/tdd/p5n2"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v, want nil", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
			if s := got.String(); s != tc.in {
				t.Fatalf("Parse(%q).String() = %q, want the input back", tc.in, s)
			}
		})
	}
}

func TestParse_RefusesEveryMalformedAddress(t *testing.T) {
	cases := map[string]string{
		"empty":                    "",
		"no selector kind":         "audit",
		"unknown selector":         "agent:audit",
		"empty role":               "role:",
		"empty phase":              "phase:",
		"empty dispatch":           "dispatch:",
		"upper case name":          "phase:Audit",
		"name starts with a dash":  "phase:-audit",
		"name with a dot":          "role:a.b",
		"name with a space":        "role:a b",
		"value after the wildcard": "*:x",
		"cycle without a selector": "cycle:5",
		"cycle with an empty tail": "cycle:5/",
		"cycle zero":               "cycle:0/*",
		"cycle with a lead zero":   "cycle:05/*",
		"cycle not a number":       "cycle:x/*",
		"negative cycle":           "cycle:-3/*",
		"empty cycle":              "cycle:/*",
		"cycle past the int range": "cycle:99999999999999999999/*",
		"two cycle prefixes":       "cycle:5/cycle:6/*",
		"dispatch with a space":    "dispatch:a b",
		"dispatch with a quote":    "dispatch:a\"b",
		"leading white space":      " phase:audit",
		"dispatch too long":        "dispatch:" + strings.Repeat("a", MaxValueBytes+1),
		"name too long":            "role:" + strings.Repeat("a", MaxValueBytes+1),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(in)
			if !errors.Is(err, ErrSyntax) {
				t.Fatalf("Parse(%q) = %+v, %v; want ErrSyntax", in, got, err)
			}
		})
	}
}

func TestParse_AcceptsAValueAtTheLengthCap(t *testing.T) {
	in := "dispatch:" + strings.Repeat("a", MaxValueBytes)
	if _, err := Parse(in); err != nil {
		t.Fatalf("Parse(value of %d bytes) error = %v, want nil", MaxValueBytes, err)
	}
}

func TestResolve_ARelativeAddressTakesTheSenderCycle(t *testing.T) {
	rel := Address{Selector: SelectPhase, Value: "audit"}
	if !rel.Relative() {
		t.Fatal("an address with no cycle must be relative")
	}
	got := rel.Resolve(1850)
	want := Address{Cycle: 1850, Selector: SelectPhase, Value: "audit"}
	if got != want {
		t.Fatalf("Resolve(1850) = %+v, want %+v", got, want)
	}
	if got.Relative() {
		t.Fatal("a resolved address must not be relative")
	}
}

func TestResolve_AnAbsoluteAddressKeepsItsCycle(t *testing.T) {
	abs := Address{Cycle: 7, Selector: SelectAll}
	if got := abs.Resolve(1850); got != abs {
		t.Fatalf("Resolve(1850) of an address in cycle 7 = %+v, want it unchanged; the policy, not the protocol, refuses it", got)
	}
}

func TestAddress_JSONIsTheTextForm(t *testing.T) {
	in := []Address{{Cycle: 3, Selector: SelectRole, Value: "auditor"}, {Selector: SelectAll}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	if string(b) != `["cycle:3/role:auditor","*"]` {
		t.Fatalf("Marshal = %s, want the text form of each address", b)
	}
	var back []Address
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	if len(back) != 2 || back[0] != in[0] || back[1] != in[1] {
		t.Fatalf("Unmarshal = %+v, want %+v", back, in)
	}
}

func TestAddress_UnmarshalRefusesAMalformedAddress(t *testing.T) {
	var a Address
	err := json.Unmarshal([]byte(`"phase:"`), &a)
	if !errors.Is(err, ErrSyntax) {
		t.Fatalf("Unmarshal(\"phase:\") error = %v, want ErrSyntax", err)
	}
}

func TestAddress_MarshalRefusesAnInvalidValue(t *testing.T) {
	_, err := Address{Selector: SelectPhase}.MarshalText()
	if !errors.Is(err, ErrSyntax) {
		t.Fatalf("MarshalText(phase with no value) error = %v, want ErrSyntax", err)
	}
}

func TestAddress_TextRoundTrips(t *testing.T) {
	in := Address{Cycle: 9, Selector: SelectDispatch, Value: "r/9/build/p4n1"}
	b, err := in.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText error = %v", err)
	}
	var out Address
	if err := out.UnmarshalText(b); err != nil || out != in {
		t.Fatalf("UnmarshalText(%s) = %+v, %v; want %+v", b, out, err, in)
	}
}
