#!/bin/bash
set -eo pipefail

count=$1
time=$2

require_summary() {
  local log_file="$1"
  local label="$2"
  local summary

  summary=$(grep SUM "$log_file" | tail -1 | xargs || true)
  if [ -z "$summary" ]; then
    echo "${label} output missing SUM line" >&2
    if [ -f "$log_file" ]; then
      cat "$log_file" >&2
    fi
    return 1
  fi

  printf '%s\n' "$summary"
}

parse_throughput() {
  local log_file="$1"
  local label="$2"
  local summary
  local res
  local unit

  summary=$(require_summary "$log_file" "$label")
  res=$(printf '%s\n' "$summary" | cut -d ' ' -f 6)
  unit=$(printf '%s\n' "$summary" | cut -d ' ' -f 7)
  if [ -z "$res" ] || [ -z "$unit" ]; then
    echo "${label} output missing parsed throughput" >&2
    cat "$log_file" >&2
    return 1
  fi

  printf '%s %s\n' "$res" "$unit"
}

iperf -c 20.20.20.1 -t $time -p 12865 -P $count > iperf.log 2>&1 &
tcp_pid=$!
wait "$tcp_pid"
throughput=$(parse_throughput iperf.log "TCP iperf")
read -r res unit <<< "$throughput"
echo -e "TCP throughput \t\t: $res $unit"
rm -f iperf.log

resNum=$(bc -l <<<"${res}")
if [[ $resNum < 10 ]]; then
  echo "Failed too low $resNum"
  exit 1
fi

sleep 2

iperf3 -c 20.20.20.1 -t $time -p 13866 -P $count --logfile iperf.log --sctp &
sctp_pid=$!
wait "$sctp_pid"
throughput=$(parse_throughput iperf.log "SCTP iperf3")
read -r res unit <<< "$throughput"
echo -e "SCTP throughput \t: $res $unit"
rm -f iperf.log
