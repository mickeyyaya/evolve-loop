package envchain

import "testing"

func TestInt(t *testing.T) {
	const key = "TEST_TYPED_INT"
	cases := []struct {
		name   string
		reqEnv map[string]string
		def    int
		want   int
	}{
		{"unset-returns-default", nil, 7, 7},
		{"empty-returns-default", map[string]string{key: ""}, 7, 7},
		{"unparseable-returns-default", map[string]string{key: "abc"}, 7, 7},
		{"valid-positive", map[string]string{key: "42"}, 7, 42},
		{"valid-negative", map[string]string{key: "-5"}, 7, -5},
		{"zero", map[string]string{key: "0"}, 7, 0},
		{"leading-space-is-unparseable", map[string]string{key: " 3"}, 7, 7}, // strconv.Atoi parity
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Int(key, c.reqEnv, c.def); got != c.want {
				t.Errorf("Int()=%d, want %d", got, c.want)
			}
		})
	}
}

// TestIntMin pins the "below-min is invalid → default" semantics that mirror
// the phase-latency ceiling's `num > 0 else fall back` guard (NOT clamp-to-min).
func TestIntMin(t *testing.T) {
	const key = "TEST_TYPED_INTMIN"
	cases := []struct {
		name      string
		val       string
		def, min  int
		want      int
		setReqEnv bool
	}{
		{name: "unset", def: 900, min: 1, want: 900},
		{name: "zero-below-min-returns-default", val: "0", def: 900, min: 1, want: 900, setReqEnv: true},
		{name: "negative-below-min-returns-default", val: "-5", def: 900, min: 1, want: 900, setReqEnv: true},
		{name: "at-min", val: "1", def: 900, min: 1, want: 1, setReqEnv: true},
		{name: "above-min", val: "120", def: 900, min: 1, want: 120, setReqEnv: true},
		{name: "unparseable-returns-default", val: "x", def: 900, min: 1, want: 900, setReqEnv: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var reqEnv map[string]string
			if c.setReqEnv {
				reqEnv = map[string]string{key: c.val}
			}
			if got := IntMin(key, reqEnv, c.def, c.min); got != c.want {
				t.Errorf("IntMin()=%d, want %d", got, c.want)
			}
		})
	}
}

// TestIntMin_ReproducesPhaseMaxAttempts proves IntMin + an explicit upper cap
// reproduces resolvePhaseMaxAttempts exactly (the Step-3 migration target):
// <1 → def(2), >5 → 5, else n.
func TestIntMin_ReproducesPhaseMaxAttempts(t *testing.T) {
	const key = "TEST_TYPED_MAXATTEMPTS"
	resolve := func(val string, set bool) int {
		var reqEnv map[string]string
		if set {
			reqEnv = map[string]string{key: val}
		}
		n := IntMin(key, reqEnv, 2, 1)
		if n > 5 {
			return 5
		}
		return n
	}
	cases := []struct {
		val  string
		set  bool
		want int
	}{
		{"", false, 2}, // unset → def
		{"0", true, 2}, // below min → def (NOT 1)
		{"1", true, 1}, // at min
		{"3", true, 3}, // in range
		{"6", true, 5}, // above max → cap
		{"x", true, 2}, // unparseable → def
	}
	for _, c := range cases {
		if got := resolve(c.val, c.set); got != c.want {
			t.Errorf("maxAttempts(%q)=%d, want %d", c.val, got, c.want)
		}
	}
}

func TestBool(t *testing.T) {
	const key = "TEST_TYPED_BOOL"
	cases := []struct {
		name   string
		reqEnv map[string]string
		def    bool
		want   bool
	}{
		{"unset-uses-default-true", nil, true, true},
		{"unset-uses-default-false", nil, false, false},
		{"one-is-true", map[string]string{key: "1"}, false, true},
		{"zero-is-false", map[string]string{key: "0"}, true, false},
		{"true-token", map[string]string{key: "true"}, false, true},
		{"false-token", map[string]string{key: "false"}, true, false},
		{"on-token", map[string]string{key: "on"}, false, true},
		{"off-token", map[string]string{key: "off"}, true, false},
		{"uppercase-TRUE", map[string]string{key: "TRUE"}, false, true},
		{"mixedcase-TrUe", map[string]string{key: "TrUe"}, false, true},
		{"unrecognized-uses-default", map[string]string{key: "maybe"}, true, true},
		{"empty-uses-default", map[string]string{key: ""}, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Bool(key, c.reqEnv, c.def); got != c.want {
				t.Errorf("Bool()=%v, want %v", got, c.want)
			}
		})
	}
}

// TestBoolValue_NoEnvLookup confirms BoolValue interprets the raw string only
// and never consults the process env — the property that makes it safe for
// frozen-snapshot reads.
func TestBoolValue_NoEnvLookup(t *testing.T) {
	const key = "TEST_TYPED_BOOLVALUE"
	t.Setenv(key, "1") // process env set to truthy...
	// ...but BoolValue is handed an empty raw value, so it must use def, NOT
	// pick up the process env.
	if got := BoolValue("", false); got != false {
		t.Errorf("BoolValue(\"\")=%v, want false (must ignore process env)", got)
	}
	if got := BoolValue("1", false); got != true {
		t.Errorf("BoolValue(\"1\")=%v, want true", got)
	}
	if got := BoolValue("0", true); got != false {
		t.Errorf("BoolValue(\"0\")=%v, want false", got)
	}
}

// TestTyped_ReqEnvBeatsProcessEnv confirms the typed getters honor the same
// reqEnv > os.Getenv precedence as Resolve.
func TestTyped_ReqEnvBeatsProcessEnv(t *testing.T) {
	const key = "TEST_TYPED_PRECEDENCE"
	t.Setenv(key, "100")
	if got := Int(key, map[string]string{key: "42"}, 0); got != 42 {
		t.Errorf("Int reqEnv precedence: got %d, want 42", got)
	}
	if got := Int(key, nil, 0); got != 100 {
		t.Errorf("Int process-env fallback: got %d, want 100", got)
	}
}
