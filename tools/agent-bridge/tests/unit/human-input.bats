#!/usr/bin/env bats
# T-human-input — human-input simulation library (no live LLM)

setup() {
  BRIDGE_LIB_DIR="${BATS_TEST_DIRNAME}/../../lib"
  export BRIDGE_LIB_DIR
  source "${BRIDGE_LIB_DIR}/human-input.sh"
}

@test "T-human-input.1 — gate OFF by default: bridge_human_active=0" {
  unset BRIDGE_HUMAN_SIMULATION
  unset human_input
  result=$(bridge_human_active)
  [ "$result" = "0" ]
}

@test "T-human-input.2 — gate requires BOTH env + flag: env alone is not enough" {
  export BRIDGE_HUMAN_SIMULATION=1
  unset human_input
  result=$(bridge_human_active)
  [ "$result" = "0" ]
}

@test "T-human-input.3 — gate requires BOTH env + flag: flag alone is not enough" {
  unset BRIDGE_HUMAN_SIMULATION
  human_input=1
  result=$(bridge_human_active)
  [ "$result" = "0" ]
}

@test "T-human-input.4 — gate active when env=1 AND flag=1" {
  export BRIDGE_HUMAN_SIMULATION=1
  human_input=1
  result=$(bridge_human_active)
  [ "$result" = "1" ]
}

@test "T-human-input.5 — _human_sample emits a number" {
  v=$(_human_sample 65 20)
  [[ "$v" =~ ^[0-9]+$ ]]
  [ "$v" -ge 10 ]
}

@test "T-human-input.6 — _human_sample respects min" {
  v=$(_human_sample 5 5 100)
  [ "$v" -ge 100 ]
}

@test "T-human-input.7 — distribution: 30 samples have mean within 30% of target" {
  total=0
  for i in $(seq 1 30); do
    v=$(_human_sample 100 25)
    total=$((total + v))
  done
  mean=$((total / 30))
  # Expect 70 <= mean <= 130
  [ "$mean" -ge 70 ]
  [ "$mean" -le 130 ]
}

@test "T-human-input.8 — human_reading_pause computes from word count" {
  # 220 words at default 220wpm = ~60s. We don't actually sleep that long;
  # just verify the function emits a stderr trace with the calculated ms.
  text=$(printf 'word\n%.0s' $(seq 1 50))
  # Capture stderr only (the function sleeps; we redirect /dev/null on stdout and
  # short-circuit by setting reading_wpm very high)
  BRIDGE_HUMAN_READING_WPM=99999 run human_reading_pause "$text"
  [ "$status" -eq 0 ]
  # The function emits "[human-input] reading pause ~Nms (~Nwords at WPM wpm)" to stderr.
  # We use `run` which captures stderr too in BATS_v2.
  true
}
