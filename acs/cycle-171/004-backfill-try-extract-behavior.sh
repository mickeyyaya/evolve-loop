#!/usr/bin/env bash
# ACS cycle-171 T3 — backfill.TryExtract extracts/rejects correctly.
# Behavioral: the table-driven test seeds real <phase>-stdout.clean.txt fixtures
# and asserts positive extraction (file written), plus no-header / too-short /
# unknown-phase / missing-file rejections. Invokes the function under test.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
assert_go_test_pass ./internal/backfill/... 'TestTryExtract' || exit 1
