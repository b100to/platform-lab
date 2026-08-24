#!/usr/bin/env bash
# Continuously probe the demo service and report availability.
#
#   ./scripts/probe.sh [duration_seconds] [url]
#
# Prints a per-second trace while running, then a summary: request count,
# failures, and the longest consecutive outage. The outage number is the
# one that belongs in a gameday record.
set -uo pipefail

DURATION="${1:-120}"
URL="${2:-http://localhost:18080/hostname}"
INTERVAL="${INTERVAL:-0.2}"

total=0
fail=0
cur_gap=0
max_gap=0
gap_start=""
declare -A served

start_epoch=$(date +%s)
echo "probing $URL for ${DURATION}s (interval ${INTERVAL}s)"
echo "ts        result"

while (( $(date +%s) - start_epoch < DURATION )); do
  total=$((total + 1))
  now=$(date +%H:%M:%S)

  if body=$(curl -sf --max-time 1 "$URL" 2>/dev/null); then
    served["$body"]=$(( ${served["$body"]:-0} + 1 ))
    if (( cur_gap > 0 )); then
      echo "$now  RECOVERED after ${cur_gap} failed probes (~$(awk "BEGIN{printf \"%.1f\", $cur_gap * $INTERVAL}")s)"
      (( cur_gap > max_gap )) && max_gap=$cur_gap
      cur_gap=0
    fi
  else
    fail=$((fail + 1))
    cur_gap=$((cur_gap + 1))
    (( cur_gap == 1 )) && { gap_start="$now"; echo "$now  FAIL (outage start)"; }
  fi

  sleep "$INTERVAL"
done

(( cur_gap > max_gap )) && max_gap=$cur_gap

echo
echo "---- summary ----"
echo "requests   : $total"
echo "failures   : $fail"
echo "error rate : $(awk "BEGIN{printf \"%.2f%%\", ($fail/$total)*100}")"
echo "max outage : ${max_gap} probes (~$(awk "BEGIN{printf \"%.1f\", $max_gap * $INTERVAL}")s)"
[[ -n "$gap_start" ]] && echo "first fail : $gap_start"
echo
echo "served by:"
for pod in "${!served[@]}"; do printf "  %-40s %s\n" "$pod" "${served[$pod]}"; done | sort
