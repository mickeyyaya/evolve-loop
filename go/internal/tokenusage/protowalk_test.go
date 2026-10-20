package tokenusage

import (
	"errors"
	"testing"
)

func TestAgyGenerationUsage_ReadsOnlyTheUsageCountersOfEachGeneration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, hex string
		output    int
		err       bool
	}{
		{"a fixed64 and a fixed32 beside the counters are skipped", "0a12" + "2210" + "1807" + "090102030405060708" + "0d01020304", 7, false},
		{"another top-level field and another message field are ignored", "1203220118" + "0a08" + "1a020803" + "2202" + "1805", 5, false},
		{"a non-varint counter is not a count", "0a04" + "22021a00", 0, false},
		{"a truncated fixed64", "0a06" + "2204" + "09010203", 0, true},
		{"a truncated fixed32", "0a04" + "2202" + "0d01", 0, true},
		{"an unknown wire type", "0a03" + "220103", 0, true},
		{"a length past the end", "0a05", 0, true},
		{"a truncated length", "0aff", 0, true},
		{"a truncated key", "ff", 0, true},
		{"a varint past 64 bits", "08ffffffffffffffffffff01", 0, true},
		{"a truncated varint", "08ff", 0, true},
		{"not hex", "zz", 0, true},
	}
	for _, c := range cases {
		u, err := agyGenerationUsage(c.hex)
		if (err != nil) != c.err || u.Output != c.output {
			t.Errorf("%s: usage %+v err %v, want output %d err %t", c.name, u, err, c.output, c.err)
		}
		if c.err && c.name != "not hex" && !errors.Is(err, errMalformedProto) {
			t.Errorf("%s: err = %v, want errMalformedProto", c.name, err)
		}
	}
}
