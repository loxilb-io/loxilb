#!/bin/bash
source ../common.sh
echo VIPADV

##
## VIP advertisement, derived from the HA-1 topology.
##
## What this checks: a routed IPv4 VIP is bound on the master and advertised on
## the data plane link, the advertisement repeats on later sweeps, the IPv6 VIP
## sits on the port that answers for it, and both leave a demoted master.
##
## What it does not check: the data path. HA-1's traffic test needs the
## keepalived VRRP address as the endpoint gateway, and this case drives HA
## state through loxilb's own API instead of running keepalived, so that
## address does not exist here. Resolution of a routed IPv6 VIP is out of scope
## too - the IPv6 VIP here is L2 adjacent.
##
## VIP_ADV_DEV picks the mode. Set it empty to let loxilb resolve the interface
## on its own; the GARP assertion is then dropped, because in this topology
## neither loxilb has a route to the VIP and automatic resolution correctly
## lands on whatever the default route points at.
##

v4vip="20.20.20.1"
v6vip="2001:db8:11::100"
advdev="${VIP_ADV_DEV-vlan11}"
code=0

api() {
    local host=$1 method=$2 path=$3 body=$4
    if [[ -n "$body" ]]; then
        $hexec $host curl -sX "$method" "http://0.0.0.0:11111/netlox/v1$path" \
            -H 'accept: application/json' -H 'Content-Type: application/json' -d "$body"
    else
        $hexec $host curl -sX "$method" "http://0.0.0.0:11111/netlox/v1$path" \
            -H 'accept: application/json'
    fi
}

# A rule created without an instance lands on cmn.CIDefault, which is the
# string "llb-inst0" - not the instance literally named "default", which the
# cluster arguments create separately. Select by name: the API returns the
# instances in no particular order, and the order differs between the two hosts.
ci_inst="llb-inst0"

ci_state() {
    api $1 GET /config/cistate/all | jq -r --arg i "$ci_inst" '.Attr[] | select(.instance==$i) | .state'
}

# set_ci_state <host> <MASTER|BACKUP> - drive HA state without keepalived
set_ci_state() {
    api $1 POST /config/cistate "{\"instance\":\"$ci_inst\",\"state\":\"$2\",\"vip\":\"0.0.0.0\"}" >/dev/null
}

vip4_on_lo() {
    $hexec $1 ip -4 -o addr show dev lo 2>/dev/null | grep -q "$v4vip/32"
}

# vip6_dev <host> - which link carries the IPv6 VIP, empty for none
vip6_dev() {
    $hexec $1 ip -6 -o addr show 2>/dev/null | grep "$v6vip/128" | awk '{print $2}' | head -1
}

# adv_if <host> <vip> - the interface loxilb last reported for a VIP
#
# loxilb names its log file after HOSTNAME, which docker sets to the container
# id, so glob for it.
adv_if() {
    $dexec $1 bash -c "grep -h 'lb-rule vip $2 - advertising on' /var/log/loxilb*.log 2>/dev/null | tail -1" \
        | sed -E 's/.*advertising on ([^ ,]+).*/\1/'
}

# garp_seen <seconds> - a gratuitous ARP for the v4 VIP on r1's VLAN 11
#
# arp[14:4] is the sender protocol address, so only frames claiming 20.20.20.1
# match. The VIP sweep comes round about every 40 seconds.
garp_seen() {
    local out
    out=$($hexec r1 timeout $1 tcpdump -l -n -i vlan11 -c 1 \
          'arp and arp[14:4] = 0x14141401' 2>/dev/null)
    [[ -n "$out" ]]
}

# garp_capture_start <seconds> <file> - collect gratuitous ARPs for the v4 VIP
# on r1's VLAN 11 into file, in the background, for the given time
garp_capture_start() {
    $hexec r1 timeout $1 tcpdump -l -n -i vlan11 \
          'arp and arp[14:4] = 0x14141401' > $2 2>/dev/null &
    # tcpdump takes a moment to attach; the flip must not outrun it.
    sleep 1
}

# check_garp_burst <capture file> <phase> - the promotion was advertised more
# than once
#
# The transition sends one gratuitous ARP and --vip-adv-repeat (default 3)
# more follow a second apart, so the window holds four. The periodic sweep may
# add one of its own, so a count of two proves nothing: transition plus sweep
# gets there without any repeat. Three does, with one frame to spare for loss.
check_garp_burst() {
    local file=$1 phase=$2
    local n=$(grep -c . $file 2>/dev/null)

    if [[ $n -ge 3 ]]; then
        echo "VIPADV $phase garp repeated on promotion ($n in window) [OK]"
        return 0
    fi
    echo "VIPADV $phase garp not repeated on promotion ($n in window, want >= 3) [FAILED]"
    return 1
}

check_vip_adv() {
    local master=$1 backup=$2 phase=$3
    local rc=0

    if vip4_on_lo $master; then
        echo "VIPADV $phase vip4 bound on $master [OK]"
    else
        echo "VIPADV $phase vip4 not bound on $master [FAILED]"
        rc=1
    fi

    if vip4_on_lo $backup; then
        echo "VIPADV $phase vip4 still bound on backup $backup [FAILED]"
        rc=1
    else
        echo "VIPADV $phase vip4 absent on backup $backup [OK]"
    fi

    local mdev=$(vip6_dev $master)
    local bdev=$(vip6_dev $backup)

    if [[ "$mdev" == "vlan11" ]]; then
        echo "VIPADV $phase vip6 on $master vlan11 [OK]"
    else
        echo "VIPADV $phase vip6 on $master is '${mdev:-none}', want vlan11 [FAILED]"
        rc=1
    fi

    if [[ -z "$bdev" ]]; then
        echo "VIPADV $phase vip6 absent on backup $backup [OK]"
    else
        echo "VIPADV $phase vip6 still on backup $backup dev $bdev [FAILED]"
        rc=1
    fi

    echo "VIPADV $phase resolved v4 '$(adv_if $master $v4vip)' v6 '$(adv_if $master $v6vip)' on $master"

    if [[ -z "$advdev" ]]; then
        echo "VIPADV $phase garp assertion skipped, automatic resolution mode"
        return $rc
    fi
    if ! command -v tcpdump >/dev/null 2>&1; then
        echo "VIPADV $phase garp check skipped, no tcpdump on the host"
        return $rc
    fi

    # Two windows back to back. The first says the VIP is advertised at all, the
    # second that the periodic re-advertisement keeps going - which is where a
    # RouteGet based resolver stops, because once the VIP is bound the kernel
    # answers with the local route on lo.
    if garp_seen 60; then
        echo "VIPADV $phase garp seen on r1 vlan11 [OK]"
    else
        echo "VIPADV $phase no garp on r1 vlan11 [FAILED]"
        rc=1
    fi

    if garp_seen 60; then
        echo "VIPADV $phase garp repeated on a later sweep [OK]"
    else
        echo "VIPADV $phase garp not repeated [FAILED]"
        rc=1
    fi

    return $rc
}

status1=$(ci_state llb1)
status2=$(ci_state llb2)
echo "VIPADV HA state llb1-$status1 llb2-$status2  (adv dev '${advdev:-auto}')"

if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]]; then
    master="llb1"; backup="llb2"
elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]]; then
    master="llb2"; backup="llb1"
else
    echo "VIPADV HA state llb1-$status1 llb2-$status2 [FAILED]"
    exit 1
fi
echo "Master:$master Backup:$backup"

check_vip_adv $master $backup Phase-1 || code=1

# The burst check needs the capture running before the promotion lands. It is
# a tcpdump on the host like garp_seen, with the same reasons to skip.
burst_check="yes"
if [[ -z "$advdev" ]] || ! command -v tcpdump >/dev/null 2>&1; then
    burst_check="no"
fi
burst_file=$(mktemp)
if [[ $burst_check == "yes" ]]; then
    garp_capture_start 8 $burst_file
fi

echo "VIPADV flipping HA state through the API"
set_ci_state $master BACKUP
set_ci_state $backup MASTER
sleep 5

status1=$(ci_state $master)
status2=$(ci_state $backup)
echo "VIPADV HA state $master-$status1 $backup-$status2"

if [[ $status1 == "BACKUP" && $status2 == "MASTER" ]]; then
    echo "VIPADV HA flip [OK]"
else
    echo "VIPADV HA flip [FAILED]"
    exit 1
fi

if [[ $burst_check == "yes" ]]; then
    wait
    check_garp_burst $burst_file Phase-2 || code=1
else
    echo "VIPADV Phase-2 garp burst check skipped"
fi
rm -f $burst_file

# The demoted master has to give both VIPs up, and the promoted one take them.
# One sweep is about 40 seconds; allow two.
sleep 90
check_vip_adv $backup $master Phase-2 || code=1

if [[ $code == 0 ]]; then
    echo "VIPADV [OK]"
else
    echo "VIPADV [FAILED]"
fi

exit $code
