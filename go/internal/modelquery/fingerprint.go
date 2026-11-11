package modelquery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
)

const decisionVersion = "v2"

type FingerprintInput struct {
	CLI        string
	Candidates []string
	Policy     FreshnessPolicy
	Tiers      []string
}

func Fingerprint(in FingerprintInput) string {
	h := sha256.New()
	writeField := func(s string) {
		fmt.Fprintf(h, "%d:%s\x00", len(s), s)
	}
	writeList := func(items []string) {
		sorted := append([]string(nil), items...)
		sort.Strings(sorted)
		writeField(strconv.Itoa(len(sorted)))
		for _, it := range sorted {
			writeField(it)
		}
	}
	writeField(decisionVersion)
	writeField(in.CLI)
	writeList(in.Candidates)
	writeField(strconv.FormatBool(in.Policy.PreferAlias))
	writeList(in.Policy.AliasIDs)
	writeList(in.Tiers)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
