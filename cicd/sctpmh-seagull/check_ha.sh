#!/bin/bash

master="llb1"
backup="llb2"

function check_ha() {
    count=0
    while : ; do
        status1=$($hexec llb1 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
        status2=$($hexec llb2 curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/cistate/all' -H 'accept: application/json' | jq -r '.Attr[0].state')
        if [[ $status1 == "MASTER" && $status2 == "BACKUP" ]];
        then
            master="llb1"
            backup="llb2"
            break
        elif [[ $status2 == "MASTER" && $status1 == "BACKUP" ]]; 
        then
            master="llb2"
            backup="llb1"
            break
        else
            count=$(( $count + 1 ))
            if [[ $count -ge 20 ]]; then
                echo "KA llb1-$status1, llb2-$status2 [NOK] - Exiting" >&2
                return 1
            fi
            echo "KA llb1-$status1, llb2-$status2 [NOK]" >&2
            sleep 5
        fi
    done
}

function checkSync() {
    count=1
    sync=0
    while [[ $count -le 5 ]] ; do
        echo -e "\nStatus at MASTER:$master\n" >&2
        ct=`$dexec $master loxicmd get ct | grep est`
        echo "${ct//'\n'/$'\n'}" >&2

        echo -e "\nStatus at BACKUP:$backup\n" >&2
        ct=`$dexec $backup loxicmd get ct | grep est`
        echo "${ct//'\n'/$'\n'}" >&2

        nres1=$($hexec $master curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)
        nres2=$($hexec $backup curl -sX 'GET' 'http://0.0.0.0:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)

        if [[ $nres1 == 0 ]]; then
            echo -e "No active connections in Master:$master. Exiting!" >&2
            return 2
        fi

        if [[ $nres1 == $nres2 && $nres1 != 0 ]]; then
            echo -e "\nConnections sync successful!!!\n" >&2
            sync=1
            break;
        fi
        echo -e "\nConnections sync pending.. Let's wait a little more..\n" >&2
        count=$(( $count + 1 ))
        sleep 2
    done

    if [[ $sync == 0 ]]; then
        echo -e "\nConnection Sync failed\n" >&2
        return 0
    fi
    echo "$sync"
}

function restart_mloxilb() {
    if [[ $master == "llb1" ]]; then
        pat="cluster=172.17.0.3"
        copts=" --cluster=172.17.0.3"
        self=" --self=0"
        ka=" --ka=172.17.0.3:172.17.0.2"
    else
        pat="cluster=172.17.0.2"
        copts=" --cluster=172.17.0.2"
        self=" --self=1"
        ka=" --ka=172.17.0.2:172.17.0.3"
    fi
    echo "Restarting MASTER: $master"
    #pid=$(docker exec -i $master ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    pid=$(ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    echo "Killing $pid" >&2
    #docker exec -dt $master kill -9 $pid
    sudo kill -9 $pid
    docker exec -dt $master ip link del llb0
    docker exec -dt $master /root/loxilb-io/loxilb/loxilb $copts $self $ka
    #pid=$(docker exec -i $master ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    pid=$(ps -aef | grep $pat | xargs | cut -d ' ' -f 2)
    echo "New loxilb pid: $pid" >&2
}

# Cluster options are a property of the instance (llb1/llb2), not its HA role.
# _node_opts NODE sets _pat/_copts/_self/_ka for that instance.
function _node_opts() {
    if [[ "$1" == "llb1" ]]; then
        _pat="cluster=172.17.0.3"
        _copts=" --cluster=172.17.0.3"
        _self=" --self=0"
        _ka=" --ka=172.17.0.3:172.17.0.2"
    else
        _pat="cluster=172.17.0.2"
        _copts=" --cluster=172.17.0.2"
        _self=" --self=1"
        _ka=" --ka=172.17.0.2:172.17.0.3"
    fi
}

# Restart a single loxilb instance (kill + relaunch); leaves it coming up.
function restart_one() {
    local node="$1"
    _node_opts "$node"
    echo "Restarting $node" >&2
    local pid=$(ps -aef | grep "$_pat" | xargs | cut -d ' ' -f 2)
    echo "Killing $pid" >&2
    sudo kill -9 $pid
    docker exec -dt "$node" ip link del llb0
    echo "/root/loxilb-io/loxilb/loxilb $_copts $_self $_ka" >&2
    docker exec -dt "$node" /root/loxilb-io/loxilb/loxilb $_copts $_self $_ka
    pid=$(ps -aef | grep "$_pat" | xargs | cut -d ' ' -f 2)
    echo "New $node pid: $pid" >&2
}

# Confirm the routers point BOTH gateway VIPs at the current MASTER, nudging them
# if they do not. The failure this guards against is exactly "HA API is healthy
# but the router ARP is stale/split": the client-side gateway 11.11.11.11 (r1/r2)
# and the EP-side gateway 10.10.10.10 (r3/r4) are advertised independently and,
# after a restart, could latch onto different instances, breaking the SCTP
# handshake. The cistate API alone cannot see this.
function wait_gw_arp() {
    local mmac11 mmac10 a1 a2 count=0
    mmac11=$($hexec $master ip -br link show vlan11 | awk '{print $3}')
    mmac10=$($hexec $master ip -br link show vlan10 | awk '{print $3}')
    while : ; do
        a1=$($hexec r1 ip neigh show 11.11.11.11 | awk '{print $5}')
        a2=$($hexec r3 ip neigh show 10.10.10.10 | awk '{print $5}')
        if [[ -n "$mmac11" && -n "$mmac10" && "$a1" == "$mmac11" && "$a2" == "$mmac10" ]]; then
            echo "GW ARP -> master $master [OK]" >&2
            return 0
        fi
        echo "GW ARP off master (11.11.11.11->$a1 want $mmac11 ; 10.10.10.10->$a2 want $mmac10) - flushing" >&2
        for r in r1 r2; do $hexec $r ip neigh flush dev vlan11 2>/dev/null; $hexec $r ping -c1 -W1 11.11.11.11 >/dev/null 2>&1; done
        for r in r3 r4; do $hexec $r ip neigh flush dev vlan10 2>/dev/null; $hexec $r ping -c1 -W1 10.10.10.10 >/dev/null 2>&1; done
        count=$(( count + 1 ))
        if [[ $count -ge 10 ]]; then
            echo "GW ARP still not on master $master after retries [NOK] - continuing" >&2
            return 1
        fi
        sleep 2
    done
}

# Restart both loxilbs WITHOUT a simultaneous cold-start race.
#
# Restarting both at once let both instances come up as MASTER for a moment;
# both then sent a gratuitous ARP for the gateway VIPs (11.11.11.11 client-side,
# 10.10.10.10 EP-side) and the routers could latch onto the instance that a beat
# later became BACKUP. With the two gateway VIPs latching independently, the
# forward and return SCTP paths could land on different instances and the
# association never formed (sctpmh-seagull cases 2 and 3).
#
# Here we restart one instance at a time and wait for the cluster to re-stabilise
# in between, so the two are never cold-starting together. At most one instance
# ever (re)advertises the VIPs, so the router ARP cannot latch onto a
# soon-to-be-BACKUP node. Roles may legitimately swap across a restart (the
# elected MASTER is the higher BFD discriminator, not a fixed node), so we wait
# on cluster stability via check_ha rather than on a specific node's role.
function restart_loxilbs() {
    local m="$master" b="$backup"

    # 1) Restart the current backup. The current master stays up, keeps its role
    #    and VIPs, and advertises nothing new while the backup is down.
    restart_one "$b"
    sleep 3
    check_ha

    # 2) Restart the other instance. Whichever instance is master now stays up
    #    and advertises alone while this one restarts; a demoted node withdraws
    #    its VIPs without advertising, so no two nodes ever advertise at once.
    restart_one "$m"
    sleep 3
    check_ha

    # Confirm the routers actually point both gateway VIPs at the current master.
    wait_gw_arp
}

# Restart both loxilbs at once, so that whichever instance is elected MASTER
# takes over WITHOUT any conntrack state: neither node has seen the INIT of an
# association that outlives the restart, and neither has a peer to pull
# conntrack from at start-up. This is the one thing restart_loxilbs is built
# to avoid, and it is what the CT-less takeover case (validation7) needs.
#
# The simultaneous cold start also lets both instances claim the gateway VIPs
# for a moment, so the routers may latch onto the loser; wait_gw_arp puts them
# right before the caller looks at traffic.
function restart_loxilbs_together() {
    local n pid
    for n in llb1 llb2; do
        _node_opts "$n"
        pid=$(ps -aef | grep "$_pat" | xargs | cut -d ' ' -f 2)
        echo "Killing $n ($pid)" >&2
        sudo kill -9 $pid
    done
    for n in llb1 llb2; do
        _node_opts "$n"
        docker exec -dt "$n" ip link del llb0
        docker exec -dt "$n" /root/loxilb-io/loxilb/loxilb $_copts $_self $_ka
    done
    sleep 3
    # Two instances electing at the same instant occasionally both settle on
    # MASTER (BFD comes up on both within the same second and the two state
    # notifications race). Restarting one of them alone re-runs the election
    # against a stable peer. The CT-less takeover this helper exists for has
    # already happened by then.
    if ! check_ha; then
        echo "cluster did not settle after the joint restart - restarting llb2 alone" >&2
        restart_one llb2
        sleep 3
        check_ha || return 1
    fi
    wait_gw_arp
}
