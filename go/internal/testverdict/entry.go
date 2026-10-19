package testverdict

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/testkey"
)

const (
	indexDir  = "index"
	entryFile = ".json"
)

var (
	digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	gatePattern   = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
)

type envelope struct {
	Record Record
	Sum    string
}

func checksum(r Record) string {
	payload, _ := json.Marshal(r)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func sealRecord(r Record) []byte {
	data, _ := json.Marshal(envelope{Record: r, Sum: checksum(r)})
	return data
}

func open(path string) (Record, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, false
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Record{}, false
	}
	if checksum(env.Record) != env.Sum || env.Record.Outcome != Pass || !gatePattern.MatchString(env.Record.Gate) {
		return Record{}, false
	}
	if !validKey(env.Record.Key) || !env.Record.Key.Valid() || !env.Record.Key.Consistent(env.Record.Log) {
		return Record{}, false
	}
	return env.Record, true
}

func entryPath(root, file, full, gate string) string {
	return filepath.Join(root, file, full+"."+gate+entryFile)
}

func indexPath(root, pkg string) string {
	sum := sha256.Sum256([]byte(pkg))
	return filepath.Join(root, indexDir, hex.EncodeToString(sum[:])+entryFile)
}

func validKey(k testkey.Key) bool {
	return digestPattern.MatchString(k.File) && digestPattern.MatchString(k.Full)
}

func envMatches(recorded []testkey.EnvValue, current map[string]string) bool {
	for _, v := range recorded {
		value, set := current[v.Name]
		if set != v.Set || value != v.Value {
			return false
		}
	}
	return true
}
