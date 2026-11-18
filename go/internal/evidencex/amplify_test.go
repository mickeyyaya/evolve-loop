package evidencex_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evidencex"
)

func digestOf(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func numbered(n int, errAt map[int]bool) []byte {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		if errAt[i] {
			fmt.Fprintf(&sb, "ERROR failure at step %d\n", i)
		} else {
			fmt.Fprintf(&sb, "info heartbeat tick %d\n", i)
		}
	}
	return []byte(sb.String())
}

func TestAmplifyBudgetNeverExceeded(t *testing.T) {
	src := numbered(500, map[int]bool{3: true, 250: true, 499: true})
	for _, budget := range []int{1, 10, 100, 1000, 5000, 1 << 20} {
		res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: budget})
		if err != nil {
			t.Fatalf("budget %d: %v", budget, err)
		}
		if len(res.Text) > budget {
			t.Errorf("budget %d: text is %d bytes", budget, len(res.Text))
		}
		if res.Digest != digestOf(src) || res.Size != len(src) {
			t.Errorf("budget %d: digest or size wrong", budget)
		}
	}
}

func TestAmplifyNonPositiveBudgetRefused(t *testing.T) {
	for _, b := range []int{0, -1, -1000} {
		if _, err := evidencex.Extract([]byte("x\n"), evidencex.Options{Mode: evidencex.ModeExtract, Budget: b}); err == nil {
			t.Errorf("budget %d accepted", b)
		}
	}
}

func TestAmplifyEmptyModeRefused(t *testing.T) {
	if _, err := evidencex.Extract([]byte("x\n"), evidencex.Options{Budget: 100}); err == nil {
		t.Errorf("empty mode accepted")
	}
}

func TestAmplifyRangesSortedDisjointInBounds(t *testing.T) {
	const n = 300
	src := numbered(n, map[int]bool{1: true, 5: true, 6: true, 150: true, 299: true, 300: true})
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	prev := 0
	for _, r := range res.Ranges {
		if r.Start < 1 || r.End > n || r.Start > r.End {
			t.Errorf("range %+v out of bounds", r)
		}
		if r.Start <= prev {
			t.Errorf("range %+v overlaps or is unsorted after end %d", r, prev)
		}
		prev = r.End
	}
}

func TestAmplifyPointerModeCarriesNoText(t *testing.T) {
	src := numbered(100, map[int]bool{50: true})
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModePointer, Budget: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "" || len(res.Ranges) != 0 || len(res.Templates) != 0 {
		t.Errorf("pointer result carries content: %+v", res)
	}
	if res.Digest != digestOf(src) || res.Size != len(src) {
		t.Errorf("pointer digest or size wrong")
	}
}

func TestAmplifyNoTrailingNewlineAndCRLF(t *testing.T) {
	for name, src := range map[string][]byte{
		"no-trailing-newline": []byte("a\nb\nERROR boom"),
		"crlf":                []byte("a\r\nb\r\nERROR boom\r\nc\r\n"),
		"single-line":         []byte("ERROR only"),
		"blank-lines":         []byte("\n\n\nERROR x\n\n\n"),
	} {
		res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: 4096})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(res.Text, "boom") && !strings.Contains(res.Text, "only") && !strings.Contains(res.Text, "ERROR x") {
			t.Errorf("%s: error lost: %q", name, res.Text)
		}
		if res.Digest != digestOf(src) {
			t.Errorf("%s: digest wrong", name)
		}
	}
}

func TestAmplifyBinaryAndInvalidUTF8(t *testing.T) {
	src := append([]byte{0xff, 0xfe, 0x00, '\n'}, []byte("ERROR \xc3\x28 bad\nok\n")...)
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if res.Digest != digestOf(src) || len(res.Text) > 4096 {
		t.Errorf("bad result for binary input")
	}
}

func TestAmplifySecretsRedactedButDigestRaw(t *testing.T) {
	secret := "ghp_" + strings.Repeat("a1B2c3", 7)
	src := []byte("start\nERROR token=" + secret + " rejected\nend\n")
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, secret) {
		t.Errorf("secret survived in text")
	}
	if res.Digest != digestOf(src) {
		t.Errorf("digest must cover the raw source")
	}
}

func TestAmplifyTemplatesCountAndExamples(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "GET /item/%d 200\n", i)
	}
	res, err := evidencex.Extract([]byte(sb.String()), evidencex.Options{Mode: evidencex.ModeExtract, Budget: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, tpl := range res.Templates {
		if tpl.Count < 1 || tpl.First == "" || tpl.Last == "" || tpl.Pattern == "" {
			t.Errorf("malformed template %+v", tpl)
		}
	}
}

func TestAmplifyDeterministicLargeInput(t *testing.T) {
	src := numbered(5000, map[int]bool{10: true, 2500: true, 4990: true})
	opts := evidencex.Options{Mode: evidencex.ModeExtract, Budget: 2000}
	a, err := evidencex.Extract(src, opts)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		b, _ := evidencex.Extract(src, opts)
		if a.Text != b.Text || a.Digest != b.Digest || len(a.Templates) != len(b.Templates) {
			t.Fatalf("run %d differs", i)
		}
	}
	if !bytes.Contains([]byte(a.Text), []byte("step 10")) {
		t.Errorf("first error dropped under tight budget")
	}
}

func TestAmplifySourceNotMutated(t *testing.T) {
	src := numbered(60, map[int]bool{30: true})
	before := append([]byte(nil), src...)
	if _, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: 500}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, before) {
		t.Errorf("source mutated")
	}
}
