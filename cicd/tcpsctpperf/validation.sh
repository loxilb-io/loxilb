#!/bin/bash
set -Eeo pipefail
source ../common.sh

if [ -z "$1" ]; then
    threads=50
else
    threads=$1
fi

if [ -z "$2" ]; then
    time=10
else
    time=$2
fi

FAILED=0
DIAG_DUMPED=0
QUIET_SINK=$(mktemp -t tcpsctpperf.XXXXXX)

quiet_run() {
  "$@" >>"$QUIET_SINK" 2>&1
}

safe_ns_pkill() {
  local host="$1"
  local proc="$2"

  quiet_run $hexec "$host" pkill -x "$proc" || true
}

safe_pkill() {
  quiet_run sudo pkill -x "$1" || true
}

safe_remove() {
  quiet_run sudo rm -f "$1" || true
}

wait_for_host_listener() {
  local host="$1"
  local port="$2"
  local timeout="$3"
  local label="$4"
  local i

  for ((i=0;i<timeout;i++))
  do
    if $hexec "$host" sh -c "ss -lnH 2>/dev/null | grep -Eq '[:.]${port}([[:space:]]|$)' || netstat -an 2>/dev/null | grep -Eq '[:.]${port}([[:space:]]|$)'"; then
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for ${label} on ${host}:${port}"
  return 1
}

wait_for_host_no_listener() {
  local host="$1"
  local port="$2"
  local timeout="$3"
  local label="$4"
  local i

  for ((i=0;i<timeout;i++))
  do
    if ! $hexec "$host" sh -c "ss -lnH 2>/dev/null | grep -Eq '[:.]${port}([[:space:]]|$)' || netstat -an 2>/dev/null | grep -Eq '[:.]${port}([[:space:]]|$)'"; then
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for ${label} to stop listening on ${host}:${port}"
  return 1
}

wait_for_loxilb_ready() {
  local timeout="$1"
  local i

  for ((i=0;i<timeout;i++))
  do
    if quiet_run $dexec llb1 loxicmd get lb; then
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for loxilb to accept loxicmd"
  return 1
}

wait_for_lb_rules() {
  local timeout="$1"
  local i

  for ((i=0;i<timeout;i++))
  do
    if $dexec llb1 sh -c "
      loxicmd get lb 2>/dev/null | grep -Eq '^\| 20\\.20\\.20\\.1 \| 12865 \| tcp' &&
      loxicmd get lb 2>/dev/null | grep -Eq '^\| 20\\.20\\.20\\.1 \| 12915 \| tcp' &&
      loxicmd get lb 2>/dev/null | grep -Eq '^\| 20\\.20\\.20\\.1 \| 12964 \| tcp' &&
      loxicmd get lb 2>/dev/null | grep -Eq '^\| 20\\.20\\.20\\.1 \| 13866 \| tcp' &&
      loxicmd get lb 2>/dev/null | grep -Eq '^\| 20\\.20\\.20\\.1 \| 13866 \| sctp' &&
      loxicmd get lb 2>/dev/null | grep -Eq '^\| 20\\.20\\.20\\.1 \| 13915 \| sctp' &&
      loxicmd get lb 2>/dev/null | grep -Eq '^\| 20\\.20\\.20\\.1 \| 13965 \| sctp' &&
      loxicmd get ep 2>/dev/null | grep -q '31.31.1.1_tcp_12865' &&
      loxicmd get ep 2>/dev/null | grep -q '31.31.1.1_tcp_12915' &&
      loxicmd get ep 2>/dev/null | grep -q '31.31.1.1_tcp_12964' &&
      loxicmd get ep 2>/dev/null | grep -q '31.31.1.1_tcp_13866' &&
      loxicmd get ep 2>/dev/null | grep -q '31.31.1.1_sctp_13866' &&
      loxicmd get ep 2>/dev/null | grep -q '31.31.1.1_sctp_13915' &&
      loxicmd get ep 2>/dev/null | grep -q '31.31.1.1_sctp_13965'
    "; then
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for required TCP/SCTP LB rules and endpoints"
  return 1
}

create_lb_rules() {
  local port

  for ((port=12865;port<=12964;port++))
  do
    quiet_run $dexec llb1 loxicmd create lb 20.20.20.1 --tcp=$port:$port --endpoints=31.31.1.1:1 || true
  done

  quiet_run $dexec llb1 loxicmd create lb 20.20.20.1 --tcp=13866:13866 --endpoints=31.31.1.1:1 || true

  for ((port=13866;port<=13965;port++))
  do
    quiet_run $dexec llb1 loxicmd create lb 20.20.20.1 --sctp=$port:$port --endpoints=31.31.1.1:1 || true
  done
}

ensure_lb_rules() {
  local attempt

  for ((attempt=1;attempt<=5;attempt++))
  do
    create_lb_rules
    if wait_for_lb_rules 10; then
      return 0
    fi
    echo "Required LB rules missing after attempt ${attempt}, retrying"
    sleep 2
  done

  echo "Required LB rules missing after retries"
  return 1
}

cleanup_perf_processes() {
  safe_ns_pkill l3ep1 iperf
  safe_ns_pkill l3ep1 iperf3
  safe_ns_pkill l3h1 iperf
  safe_ns_pkill l3h1 iperf3
  safe_pkill iperf
  safe_pkill iperf3
  safe_remove iperf2s.log
  safe_remove iperf3s.log
}

cleanup_netperf_processes() {
  safe_ns_pkill l3ep1 netserver
  safe_ns_pkill l3h1 netperf
  safe_pkill netserver
  safe_pkill netperf
  safe_remove netserver.log
}

cleanup() {
  cleanup_perf_processes
  cleanup_netperf_processes
  rm -f "$QUIET_SINK"
}

dump_diagnostics() {
  if [ "$DIAG_DUMPED" -eq 1 ]; then
    return
  fi

  DIAG_DUMPED=1
  set +e

  echo
  echo "#########################################"
  echo "tcpsctpperf diagnostics"
  echo "#########################################"
  if [ -n "$1" ]; then
    echo "Failure at line $1 while running: $2 (exit $3)"
  fi

  echo "--- llb1 loxilb process ---"
  $dexec llb1 sh -c "ps -ef | grep '[l]oxilb'"

  echo "--- llb1 load balancers ---"
  $dexec llb1 loxicmd get lb

  echo "--- llb1 endpoints ---"
  $dexec llb1 loxicmd get ep

  echo "--- llb1 conntrack ---"
  $dexec llb1 loxicmd get ct

  echo "--- llb1 routes ---"
  $dexec llb1 ip route show

  echo "--- llb1 tc filters ---"
  $dexec llb1 tc filter show dev eth0 ingress

  echo "--- l3ep1 listeners ---"
  $hexec l3ep1 sh -c "ss -lnp 2>/dev/null || netstat -anp 2>/dev/null"

  echo "--- l3ep1 iperf/netserver processes ---"
  $hexec l3ep1 sh -c "ps -ef | grep -E '[i]perf|[n]etserver'"

  echo "--- l3h1 iperf/netperf processes ---"
  $hexec l3h1 sh -c "ps -ef | grep -E '[i]perf|[n]etperf'"

  echo "--- iperf3 server log ---"
  if sudo test -f iperf3s.log; then
    sudo tail -n 50 iperf3s.log
  else
    echo "iperf3s.log not present"
  fi

  echo "--- iperf2 server log ---"
  if sudo test -f iperf2s.log; then
    sudo tail -n 50 iperf2s.log
  else
    echo "iperf2s.log not present"
  fi

  echo "--- iperf client log ---"
  if sudo test -f iperf.log; then
    sudo tail -n 50 iperf.log
  else
    echo "iperf.log not present"
  fi

  echo "--- netserver log ---"
  if sudo test -f netserver.log; then
    sudo tail -n 50 netserver.log
  else
    echo "netserver.log not present"
  fi
}

on_error() {
  local exit_code=$?

  FAILED=1
  dump_diagnostics "$1" "$2" "$exit_code"
  exit "$exit_code"
}

trap cleanup EXIT
trap 'on_error $LINENO "$BASH_COMMAND"' ERR

start_iperf_servers() {
  cleanup_netperf_processes
  cleanup_perf_processes
  wait_for_host_no_listener l3ep1 12865 15 "netserver or iperf"
  wait_for_host_no_listener l3ep1 13866 15 "iperf3"
  quiet_run $hexec l3ep1 sh -c 'nohup iperf -s -p 12865 -o iperf2s.log >/dev/null 2>&1 </dev/null &'
  quiet_run $hexec l3ep1 sh -c 'nohup iperf3 -s -p 13866 --logfile iperf3s.log >/dev/null 2>&1 </dev/null &'
  wait_for_host_listener l3ep1 12865 15 "iperf server"
  wait_for_host_listener l3ep1 13866 15 "iperf3 server"
}

run_iperf_suite() {
  echo -e "\n\nIPERF Test - Threads: $threads  Duration: $time"
  echo "*********************************************************************"
  start_iperf_servers
  $hexec l3h1 ./iperf.sh $threads $time
  cleanup_perf_processes
  echo "*********************************************************************"
  sleep 2
}

start_netperf_server() {
  cleanup_perf_processes
  cleanup_netperf_processes
  wait_for_host_no_listener l3ep1 12865 15 "iperf or netserver"
  quiet_run $hexec l3ep1 sh -c 'nohup sudo -u nobody ./netserver -4 -p 12865 >netserver.log 2>&1 </dev/null &'
  wait_for_host_listener l3ep1 12865 15 "netserver"
}

run_netperf_suite() {
  start_netperf_server
  echo -e "\n\nNETPERF Test - Threads: $threads  Duration: $time"
  echo "*********************************************************************"
  $hexec l3h1 ./netperf.sh $threads $time
  cleanup_netperf_processes
  echo "*********************************************************************"
}

restart_loxilb_rss() {
  echo -e "\n\nRestarting loxilb in different mode"
  $dexec llb1 pkill -9 loxilb
  $dexec llb1 sh -c "ip link del llb0 >/dev/null 2>&1 || true"
  $dexec llb1 bash -c "nohup /root/loxilb-io/loxilb/loxilb --rss-enable >> /dev/null 2>&1 &"
  wait_for_loxilb_ready 60
  sleep 2
  ensure_lb_rules
}

echo SCENARIO-tcpsctpperf

run_iperf_suite
run_netperf_suite
restart_loxilb_rss
run_iperf_suite
run_netperf_suite

echo SCENARIO-tcpsctpperf [OK]
