package main

import (
	"strings"
	"testing"
)

const renderStream = `{"Action":"start","Package":"m/p"}
{"Action":"run","Package":"m/p","Test":"TestB"}
{"Action":"output","Package":"m/p","Test":"TestB","Output":"=== RUN   TestB\n"}
{"Action":"output","Package":"m/p","Test":"TestB","Output":"--- FAIL: TestB (0.01s)\n"}
{"Action":"fail","Package":"m/p","Test":"TestB","Elapsed":0.01}
{"Action":"output","Package":"m/p","Output":"FAIL\n"}
{"Action":"output","Package":"m/p","Output":"FAIL\tm/p\t0.123s\n"}
{"Action":"fail","Package":"m/p","Elapsed":0.123}
{"Action":"output","Package":"m/q","Output":"ok  \tm/q\t0.5s\n"}
{"Action":"skip","Package":"m/q","Test":"TestSkip","Elapsed":0}
{"Action":"output","Package":"m/q","Output":"no newline at the end"}
build noise
{"Action": broken
`

func TestRun_RenderRestoresThePlainTestText(t *testing.T) {
	code, out, errOut := runLatency(t, renderStream, "-render")
	want := "=== RUN   TestB\n--- FAIL: TestB (0.01s)\nFAIL\nFAIL\tm/p\t0.123s\n" +
		"ok  \tm/q\t0.5s\nno newline at the endbuild noise\n{\"Action\": broken\n"
	if code != 0 || out != want || errOut != "" {
		t.Fatalf("exit %d out %q err %q; want exit 0 and %q", code, out, errOut, want)
	}
}

func TestRun_RenderExitsZeroWhateverTheStreamHolds(t *testing.T) {
	if code, out, _ := runLatency(t, "", "-render"); code != 0 || out != "" {
		t.Fatalf("empty stream: exit %d out %q", code, out)
	}
	var errOut strings.Builder
	if code := run([]string{"-render"}, &failingReader{data: []byte(`{"Action":"output","Output":"x\n"}` + "\n")}, &strings.Builder{}, &errOut); code != 0 || !strings.Contains(errOut.String(), "testlatency: render:") {
		t.Fatalf("read error: exit %d err %q; want exit 0 and a diagnostic", code, errOut.String())
	}
}
