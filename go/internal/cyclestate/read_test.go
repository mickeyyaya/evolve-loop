package cyclestate

import (
	"os"
	"path/filepath"
	"testing"
)

func writeStateFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadLastCycleNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    *string
		want    int
		wantOK  bool
		wantErr bool
	}{
		{name: "absent file", body: nil},
		{name: "malformed json", body: ptr(`{`), wantErr: true},
		{name: "field missing", body: ptr(`{"version":3}`)},
		{name: "present", body: ptr(`{"lastCycleNumber":1836,"lastAllocatedCycleNumber":1840}`), want: 1836, wantOK: true},
		{name: "present zero", body: ptr(`{"lastCycleNumber":0}`), wantOK: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "absent.json")
			if tc.body != nil {
				path = writeStateFile(t, *tc.body)
			}

			got, ok, err := ReadLastCycleNumber(path)

			if got != tc.want || ok != tc.wantOK || (err != nil) != tc.wantErr {
				t.Errorf("ReadLastCycleNumber = %d, %t, %v; want %d, %t, err=%t", got, ok, err, tc.want, tc.wantOK, tc.wantErr)
			}
		})
	}
}

func TestReadState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		body        *string
		wantHighest int
		wantOK      bool
		wantErr     bool
	}{
		{name: "absent file", body: nil},
		{name: "malformed json", body: ptr(`{"lastCycleNumber":`), wantErr: true},
		{name: "lease above last", body: ptr(`{"lastCycleNumber":10,"lastAllocatedCycleNumber":12}`), wantHighest: 12, wantOK: true},
		{name: "last above lease", body: ptr(`{"lastCycleNumber":10,"lastAllocatedCycleNumber":4}`), wantHighest: 10, wantOK: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "absent.json")
			if tc.body != nil {
				path = writeStateFile(t, *tc.body)
			}

			st, ok, err := ReadState(path)

			if st.HighestCycleNumber() != tc.wantHighest || ok != tc.wantOK || (err != nil) != tc.wantErr {
				t.Errorf("ReadState highest = %d, ok = %t, err = %v; want %d, %t, err=%t", st.HighestCycleNumber(), ok, err, tc.wantHighest, tc.wantOK, tc.wantErr)
			}
		})
	}
}

func ptr(s string) *string { return &s }
