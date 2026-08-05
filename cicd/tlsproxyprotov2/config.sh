#!/bin/bash

# Reproduction scenario for Issue #1044 / Discussion #1089:
#   "TLS 1.3 handshake fails when PROXY protocol v2 is enabled"
#
# Topology (mirrors cicd/httpsproxy) but with two crucial differences so the
# eBPF L4 datapath (dp_ins_ppv2) is exercised instead of the L7 sockproxy path:
#   1. llb1 runs in NORMAL eBPF mode (NO --proxyonlymode).
#   2. LB rule is  --mode=fullnat --ppv2en  (NOT fullproxy / --security).
#
# loxilb does NOT terminate TLS here. Each backend runs nginx with
# 'listen ... ssl proxy_protocol' which strips the PROXY v2 header loxilb
# inserts, then terminates TLS (1.2 + 1.3). See ../../docs/tls13-proxyproto-v2-issue/.

source ../common.sh

# Allow overriding the loxilb image (e.g. a locally-built one carrying a patched
# eBPF datapath) without editing common.sh:  LOXILB_IMAGE=loxilb:ppv2test ./config.sh
lxdocker="${LOXILB_IMAGE:-$lxdocker}"
echo "Using loxilb image: $lxdocker"

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

# NOTE: no --proxyonlymode -> loxilb loads the full tc/eBPF datapath.
# Override with LOXILB_EXTRA (e.g. LOXILB_EXTRA="--proxyonlymode") to test fullproxy.
if [[ -n "$LOXILB_EXTRA" ]]; then
  spawn_docker_host --dock-type loxilb --dock-name llb1 --extra-args "$LOXILB_EXTRA"
else
  spawn_docker_host --dock-type loxilb --dock-name llb1
fi
spawn_docker_host --dock-type host --dock-name l3h1
spawn_docker_host --dock-type host --dock-name l3ep1
spawn_docker_host --dock-type host --dock-name l3ep2
spawn_docker_host --dock-type host --dock-name l3ep3

echo "#########################################"
echo "Installing nginx on endpoints (parallel)"
echo "#########################################"

eps=( l3ep1 l3ep2 l3ep3 )
names=( server1 server2 server3 )

# IMPORTANT: install BEFORE the network is reconfigured. At spawn time the host
# containers still default-route via docker0 (eth0, NAT internet). Once
# config_docker_host sets the default gw to the loxilb veth, the endpoints lose
# internet (they sit on the isolated data network), so apt would hang.
# Install nginx only if the nettest image doesn't already ship it
# (precedent: cicd/egresslb, cicd/ipsec-e2e install packages via $dexec).
# Run all three in parallel so the apt cost is ~1x, not 3x.
for ep in "${eps[@]}"; do
  $dexec "$ep" bash -c "command -v nginx >/dev/null 2>&1 || (apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends nginx)" \
    > "apt.${ep}.log" 2>&1 &
done
echo "Waiting for nginx installs to finish..."
wait
for ep in "${eps[@]}"; do
  if ! $dexec "$ep" sh -c "command -v nginx >/dev/null 2>&1"; then
    echo "ERROR: nginx install failed on $ep (see apt.${ep}.log):"
    tail -5 "apt.${ep}.log"
    exit 1
  fi
done
echo "nginx installed on all endpoints"

# Install ethtool on client + loxilb (best effort) so validation.sh can toggle
# tso/gso/gro. Done NOW while docker0 internet is still reachable (before the
# default route is moved onto the loxilb veth by config_docker_host below).
$dexec l3h1 bash -c "command -v ethtool >/dev/null 2>&1 || (apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ethtool)" >apt.l3h1.log 2>&1 &
$dexec llb1 bash -c "command -v ethtool >/dev/null 2>&1 || (apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ethtool)" >apt.llb1.log 2>&1 &
wait
$dexec l3h1 sh -c "command -v ethtool" >/dev/null 2>&1 && echo "ethtool on l3h1" || echo "WARN: ethtool missing on l3h1 (offload toggles will be skipped)"

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

connect_docker_hosts l3h1 llb1
connect_docker_hosts l3ep1 llb1
connect_docker_hosts l3ep2 llb1
connect_docker_hosts l3ep3 llb1

sleep 5

#L3 config
config_docker_host --host1 l3h1 --host2 llb1 --ptype phy --addr 10.10.10.1/24 --gw 10.10.10.254
config_docker_host --host1 l3ep1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 l3ep2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 l3ep3 --host2 llb1 --ptype phy --addr 33.33.33.1/24 --gw 33.33.33.254
config_docker_host --host1 llb1 --host2 l3h1 --ptype phy --addr 10.10.10.254/24
config_docker_host --host1 llb1 --host2 l3ep1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 l3ep2 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 llb1 --host2 l3ep3 --ptype phy --addr 33.33.33.254/24

$dexec llb1 ip addr add 10.10.10.3/32 dev lo

echo "#########################################"
echo "Generating backend TLS certificate"
echo "#########################################"

# Cert is issued for the VIP the client dials (10.10.10.254) and lives on the
# BACKENDS (nginx terminates TLS). Client uses curl --insecure.
rm -fr 10.10.10.254 minica*.pem
./minica -ip-addresses 10.10.10.254
cp 10.10.10.254/cert.pem 10.10.10.254/server.crt
cp 10.10.10.254/key.pem  10.10.10.254/server.key

echo "#########################################"
echo "Configuring & starting nginx (proxy_protocol + TLS)"
echo "#########################################"

for i in "${!eps[@]}"; do
  ep="${eps[$i]}"
  name="${names[$i]}"

  # Render the config template with this backend's identifier.
  sed "s/__NAME__/${name}/" nginx.conf.tmpl > "nginx.${ep}.conf"

  # Push config + cert into the container, then (re)start nginx.
  $dexec "$ep" mkdir -p /etc/nginx/certs
  docker cp "nginx.${ep}.conf"       "${ep}:/etc/nginx/nginx.conf"
  docker cp 10.10.10.254/server.crt  "${ep}:/etc/nginx/certs/server.crt"
  docker cp 10.10.10.254/server.key  "${ep}:/etc/nginx/certs/server.key"

  $dexec "$ep" nginx -s stop 2>/dev/null
  $dexec "$ep" nginx -t
  $dexec "$ep" nginx
done

sleep 5

echo "#########################################"
echo "Creating LB rule: fullnat + PROXY protocol v2 (--ppv2en)"
echo "#########################################"

# --ppv2en enables PROXY protocol v2 (eBPF inserts the header).
# --mode=fullnat routes via the eBPF NAT path -> CT -> dp_ins_ppv2.
# No --security: loxilb does NOT terminate TLS (backends do).
# create_lb_rule runs a tc_packet_func hook check for non-fullproxy modes,
# which fails hard if the eBPF datapath isn't loaded.
create_lb_rule llb1 10.10.10.254 --tcp=2020:8080 --endpoints=31.31.31.1:1,32.32.32.1:1,33.33.33.1:1 --mode=fullnat --ppv2en

echo "#########################################"
echo "Setup done"
echo "#########################################"
