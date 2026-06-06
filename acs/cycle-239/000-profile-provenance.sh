#!/usr/bin/env bash
# ACS — cycle 239 / task profile-provenance-field (retry of cycle 238)
#
# Classification: BEHAVIORAL — runs the provenance unit tests (exit-code
# authoritative via acs/lib/assert.sh), then invokes the actual CLI
# (`evolve phases validate --strict-provenance`) against (a) a planted
# UNSTAMPED fixture, asserting it is REJECTED with exit 2 EXACTLY (negative
# case — the anti-no-op signal AND the cycle-238 exit-code-propagation fix),
# and (b) the real tree, asserting exit 0 once all profiles carry
# `generated_from`. The python sweep is auxiliary diagnostics naming any
# unstamped profile (content parity — never commit presence; inbox
# normalize-vs-commit-claims).
#
# cycle-238 HIGH fix: invoke the COMPILED go/evolve binary, never
# `go run ./cmd/evolve` — go run wraps the child process and collapses
# os.Exit(2) to shell exit 1, turning the strict-branch assertion into a
# permanent false RED.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
top=$(git rev-parse --show-toplevel)
BIN="$top/go/evolve"

# 0. Compile the binary fresh so the predicate never tests a stale build.
if ! (cd "$top/go" && go build -o evolve ./cmd/evolve); then
  echo "RED: go build -o evolve ./cmd/evolve failed — cannot test CLI behavior" >&2
  exit 1
fi

# 1. Behavioral: GeneratedFrom field unit tests (exit code authoritative).
assert_go_test_pass ./internal/profiles/... 'TestProvenance' || exit 1

# 2. Negative behavioral (strict branch): the gate must REJECT an unstamped
#    profile with exit 2 EXACTLY — exit 1 here means exit codes are being
#    rewritten somewhere in the invocation chain (the cycle-238 defect).
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/.evolve/profiles"
printf '{"name":"naked","role":"naked","cli":"claude-tmux","model_tier_default":"sonnet"}\n' \
  > "$tmp/.evolve/profiles/naked.json"
rc=0
EVOLVE_PROJECT_ROOT="$tmp" EVOLVE_PLUGIN_ROOT="$tmp" EVOLVE_PROFILE_DIR= EVOLVE_PROFILES_DIR_OVERRIDE= \
  "$BIN" phases validate --strict-provenance >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 2 ]; then
  echo "RED: phases validate --strict-provenance on unstamped fixture exited $rc, want exactly 2" >&2
  exit 1
fi

# 3. Diagnostics: name every repo profile missing the stamp (content parity).
unstamped=$(python3 - "$top" <<'PY'
import json, sys, glob, os
top = sys.argv[1]
for p in sorted(glob.glob(os.path.join(top, ".evolve/profiles/*.json"))):
    try:
        d = json.load(open(p))
    except Exception as e:
        print("%s (unparseable: %s)" % (os.path.basename(p), e))
        continue
    if "generated_from" not in d:
        print(os.path.basename(p))
PY
)
if [ -n "$unstamped" ]; then
  echo "RED: profiles missing generated_from:" >&2
  echo "$unstamped" >&2
  exit 1
fi

# 4. Behavioral: strict gate passes on the fully stamped real tree.
rc=0
EVOLVE_PROJECT_ROOT="$top" EVOLVE_PLUGIN_ROOT="$top" EVOLVE_PROFILE_DIR= EVOLVE_PROFILES_DIR_OVERRIDE= \
  "$BIN" phases validate --strict-provenance >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: phases validate --strict-provenance on real tree exited $rc, want 0" >&2
  exit 1
fi

echo "GREEN: provenance field, validator, strict exit-2 rejection, and full profile stamp all hold" >&2
exit 0
