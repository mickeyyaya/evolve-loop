package policy

import (
	"strings"
	"testing"
)

const logCompressionBlock = `{"log_compression":{"bytes_per_token":4,"min_input_bytes":0,"min_reduction_ratio":1,"pass_run_min":2,"repeat_run_min":3,"diff_file_max_lines":9,"pass_prefixes":["PASS"],"noise_prefixes":[],"failure_markers":["FAIL"],"never_compress":["## Task Contract"]}}`

func TestLogCompressionConfig_ABlockDecodesEveryKey(t *testing.T) {
	t.Parallel()

	cfg, err := loadCIWatchPolicy(t, logCompressionBlock).LogCompressionConfig()

	if err != nil {
		t.Fatalf("LogCompressionConfig: %v", err)
	}
	if cfg.BytesPerToken != 4 || cfg.MinInputBytes != 0 || cfg.MinReductionRatio != 1 || cfg.PassRunMin != 2 || cfg.RepeatRunMin != 3 || cfg.DiffFileMaxLines != 9 {
		t.Errorf("numbers decoded wrong: %+v", cfg)
	}
	if strings.Join(cfg.PassPrefixes, ",") != "PASS" || len(cfg.NoisePrefixes) != 0 || cfg.FailureMarkers[0] != "FAIL" || cfg.NeverCompress[0] != "## Task Contract" {
		t.Errorf("lists decoded wrong: %+v", cfg)
	}
}

func TestLogCompressionConfig_AnAbsentBlockIsRefusedWithoutADefault(t *testing.T) {
	t.Parallel()

	_, err := loadCIWatchPolicy(t, `{}`).LogCompressionConfig()

	if err == nil || !strings.Contains(err.Error(), "log_compression") {
		t.Errorf("err = %v, want a refusal naming log_compression", err)
	}
}

func TestLogCompressionConfig_EachInvalidValueIsRefusedByName(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ from, to, key string }{
		"zero bytes per token":   {`"bytes_per_token":4`, `"bytes_per_token":0`, "bytes_per_token"},
		"negative min input":     {`"min_input_bytes":0`, `"min_input_bytes":-5`, "min_input_bytes"},
		"ratio above one":        {`"min_reduction_ratio":1`, `"min_reduction_ratio":1.01`, "min_reduction_ratio"},
		"zero pass run":          {`"pass_run_min":2`, `"pass_run_min":0`, "pass_run_min"},
		"zero repeat run":        {`"repeat_run_min":3`, `"repeat_run_min":0`, "repeat_run_min"},
		"zero diff file lines":   {`"diff_file_max_lines":9`, `"diff_file_max_lines":0`, "diff_file_max_lines"},
		"empty failure markers":  {`"failure_markers":["FAIL"]`, `"failure_markers":[]`, "failure_markers"},
		"blank noise prefix":     {`"noise_prefixes":[]`, `"noise_prefixes":[" "]`, "noise_prefixes"},
		"duplicate pass prefix":  {`"pass_prefixes":["PASS"]`, `"pass_prefixes":["PASS","PASS"]`, "pass_prefixes"},
		"missing never compress": {`,"never_compress":["## Task Contract"]`, ``, "never_compress"},
	} {
		body := strings.Replace(logCompressionBlock, tc.from, tc.to, 1)

		_, err := loadCIWatchPolicy(t, body).LogCompressionConfig()

		if err == nil || !strings.Contains(err.Error(), tc.key) {
			t.Errorf("%s: err = %v, want a refusal naming %s", name, err, tc.key)
		}
	}
}

func TestLogCompressionConfig_EachRequiredNumberIsRefusedWhenMissing(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"bytes_per_token", "min_input_bytes", "min_reduction_ratio", "pass_run_min", "repeat_run_min", "diff_file_max_lines"} {
		body := logCompressionBlock
		start := strings.Index(body, `"`+key+`"`)
		end := start + strings.Index(body[start:], ",") + 1
		body = body[:start] + body[end:]

		_, err := loadCIWatchPolicy(t, body).LogCompressionConfig()

		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s missing: err = %v", key, err)
		}
	}
}

func TestLogCompressionPolicy_AnUnknownKeyIsRefusedByTheDecoder(t *testing.T) {
	t.Parallel()
	var block LogCompressionPolicy
	var settings LogCompressionSettings

	err := block.UnmarshalJSON([]byte(`{"surprise":1}`))

	if err == nil || !strings.Contains(err.Error(), "log_compression") {
		t.Errorf("err = %v", err)
	}
	if settings.BytesPerToken != 0 {
		t.Errorf("zero settings = %+v", settings)
	}
}
