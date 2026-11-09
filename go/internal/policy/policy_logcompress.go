package policy

import (
	"fmt"
	"slices"
	"strings"
)

type LogCompressionPolicy struct {
	BytesPerToken     *int     `json:"bytes_per_token,omitempty"`
	MinInputBytes     *int     `json:"min_input_bytes,omitempty"`
	MinReductionRatio *float64 `json:"min_reduction_ratio,omitempty"`
	PassRunMin        *int     `json:"pass_run_min,omitempty"`
	RepeatRunMin      *int     `json:"repeat_run_min,omitempty"`
	DiffFileMaxLines  *int     `json:"diff_file_max_lines,omitempty"`
	PassPrefixes      []string `json:"pass_prefixes,omitempty"`
	NoisePrefixes     []string `json:"noise_prefixes,omitempty"`
	FailureMarkers    []string `json:"failure_markers,omitempty"`
	NeverCompress     []string `json:"never_compress,omitempty"`
}

type LogCompressionSettings struct {
	BytesPerToken     int
	MinInputBytes     int
	MinReductionRatio float64
	PassRunMin        int
	RepeatRunMin      int
	DiffFileMaxLines  int
	PassPrefixes      []string
	NoisePrefixes     []string
	FailureMarkers    []string
	NeverCompress     []string
}

func (p Policy) LogCompressionConfig() (LogCompressionSettings, error) {
	c := p.LogCompression
	if c == nil {
		return LogCompressionSettings{}, fmt.Errorf("policy: log_compression block is absent: the thresholds and pattern lists have no compiled default")
	}
	var out LogCompressionSettings
	var err error
	if out.BytesPerToken, err = logCompressionPositive("bytes_per_token", c.BytesPerToken); err != nil {
		return LogCompressionSettings{}, err
	}
	if out.MinInputBytes, err = logCompressionNonNegative("min_input_bytes", c.MinInputBytes); err != nil {
		return LogCompressionSettings{}, err
	}
	if out.MinReductionRatio, err = logCompressionRatio(c.MinReductionRatio); err != nil {
		return LogCompressionSettings{}, err
	}
	if out.PassRunMin, err = logCompressionPositive("pass_run_min", c.PassRunMin); err != nil {
		return LogCompressionSettings{}, err
	}
	if out.RepeatRunMin, err = logCompressionPositive("repeat_run_min", c.RepeatRunMin); err != nil {
		return LogCompressionSettings{}, err
	}
	if out.DiffFileMaxLines, err = logCompressionPositive("diff_file_max_lines", c.DiffFileMaxLines); err != nil {
		return LogCompressionSettings{}, err
	}
	lists := []struct {
		key      string
		values   []string
		required bool
		into     *[]string
	}{
		{"pass_prefixes", c.PassPrefixes, false, &out.PassPrefixes},
		{"noise_prefixes", c.NoisePrefixes, false, &out.NoisePrefixes},
		{"failure_markers", c.FailureMarkers, true, &out.FailureMarkers},
		{"never_compress", c.NeverCompress, true, &out.NeverCompress},
	}
	for _, list := range lists {
		if err := validateLogCompressionList(list.key, list.values, list.required); err != nil {
			return LogCompressionSettings{}, err
		}
		*list.into = slices.Clone(list.values)
	}
	return out, nil
}

func logCompressionPositive(key string, set *int) (int, error) {
	if set == nil {
		return 0, fmt.Errorf("policy: log_compression.%s is required", key)
	}
	if *set < 1 {
		return 0, fmt.Errorf("policy: log_compression.%s %d: want a positive integer", key, *set)
	}
	return *set, nil
}

func logCompressionNonNegative(key string, set *int) (int, error) {
	if set == nil {
		return 0, fmt.Errorf("policy: log_compression.%s is required", key)
	}
	if *set < 0 {
		return 0, fmt.Errorf("policy: log_compression.%s %d: want zero or more", key, *set)
	}
	return *set, nil
}

func logCompressionRatio(set *float64) (float64, error) {
	if set == nil {
		return 0, fmt.Errorf("policy: log_compression.min_reduction_ratio is required")
	}
	if *set <= 0 || *set > 1 {
		return 0, fmt.Errorf("policy: log_compression.min_reduction_ratio %v: want a value above 0 and at most 1", *set)
	}
	return *set, nil
}

func validateLogCompressionList(key string, values []string, required bool) error {
	if required && len(values) == 0 {
		return fmt.Errorf("policy: log_compression.%s must name at least one entry", key)
	}
	seen := map[string]bool{}
	for _, value := range values {
		switch {
		case strings.TrimSpace(value) == "":
			return fmt.Errorf("policy: log_compression.%s: entry must not be blank", key)
		case seen[value]:
			return fmt.Errorf("policy: log_compression.%s: duplicate entry %q", key, value)
		}
		seen[value] = true
	}
	return nil
}

func (p *LogCompressionPolicy) UnmarshalJSON(raw []byte) error {
	type block LogCompressionPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("log_compression: %w", err)
	}
	*p = LogCompressionPolicy(decoded)
	return nil
}
