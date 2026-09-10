#!/bin/bash
source /home/vagrant/common.sh
source /vagrant/check_ha.sh

echo -e "sctpmh: SCTP Multihoming - CT-less Takeover Test. Both LBs cold-restarted under a live multipath association\n"
extIP="20.20.20.1"
port=2020

# What this checks: a loxilb that takes over an SCTP association it never saw
# the INIT of (mhon=0, closed->EST recovery) keeps ALL of the association's
# paths, not just the first one to send a packet.
#
# Each secondary VIP's NAT entry used to share the primary VIP's action block,
# so on such a node every path source-NATed from the primary VIP and the
# kernel derived the same reverse conntrack key (EP -> primary VIP) for all
# three. The first path to arrive got its conntrack entry; the other two were
# dropped at conntrack creation for as long as the association lived. Seen as
# two conntrack rows instead of six, a reverse row whose SNAT source is a
# different VIP than its key, two path counters that never move, and every
# call timing out once the client's primary path is one of the dead ones.
#
# Both loxilbs are restarted together on purpose: restarted one at a time, the
# survivor hands its conntrack to the other at start-up and no CT-less takeover
# ever happens - which is also why cases 5 and 6 do not catch this.

check_ha || { echo "NOK" > /vagrant/status7.txt; exit 1; }

echo "SCTP Multihoming service sctp-lb(Multipath traffic) -> $extIP:$port"
echo -e "------------------------------------------------------------------------------------\n"
echo -e "\nHA state Master:$master BACKUP-$backup\n"
echo -e "\nTraffic Flow: User -> LB -> EP "

# A long-lived association at a modest call rate, so it outlives the restart.
sudo docker exec -dt user ksh -c "sed -i 's/\"call-rate\" value=\"5000\"/\"call-rate\" value=\"100\"/g' /opt/seagull/diameter-env/config/conf.client.xml"

sudo docker exec -dt ep1 ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 200 stdbuf -oL seagull -conf ../config/conf.server.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.server.xml > ep1.out' 2>&1 > /dev/null &
sleep 2
sudo docker exec -dt user ksh -c 'export LD_PRELOAD=/usr/local/bin/libsctplib.so.1.0.8; export LD_LIBRARY_PATH=/usr/local/bin; cd /opt/seagull/diameter-env/run/; timeout 190 stdbuf -oL seagull -conf ../config/conf.client.xml -dico ../config/base_s6a.xml -scen ../scenario/ulr-ula.client.xml > user.out' 2>&1 > /dev/null &

# Let the association settle and the multipath entries take shape.
sleep 25

# Forward conntrack rows of the three paths, and their packet counters.
# Row layout: | service | dip | sip | dport | sport | proto | ... | state | act | packets | bytes |
ct_rows() { sudo docker exec -i $master loxicmd get ct --servName=sctpmh1 2>/dev/null | grep -w est; }
fwd_row() { ct_rows | grep "$1" | grep fdnat | head -1; }
row_pkts() { echo "$1" | xargs | cut -d '|' -f 11 | tr -d ' '; }
row_snat() { echo "$1" | grep -o 'fdnat-[0-9.]*' | cut -d '-' -f 2; }
calls() { sudo docker exec -t user bash -c "tail -n 10 /opt/seagull/diameter-env/run/user.out | grep '$1 calls'" | xargs | cut -d '|' -f 4 | tr -d ' '; }

paths=( "20.20.20.1 | 1.1.1.1" "21.21.21.1 | 2.2.2.1" "22.22.22.1 | 1.1.1.1" )
vips=( "20.20.20.1" "21.21.21.1" "22.22.22.1" )

code=0
echo -e "\nBefore restart: conntrack on MASTER $master\n"
ct_rows
for k in 0 1 2; do
    r=$(fwd_row "${paths[$k]}")
    if [[ -z "$r" ]]; then echo "Path $((k+1)) ${paths[$k]} has no conntrack before the restart [NOK]"; code=1; fi
done
if [[ $code != 0 ]]; then
    echo "NOK" > /vagrant/status7.txt
    echo "sctpmh SCTP Multihoming CT-less Takeover [NOK] - association did not form"
    exit 1
fi
calls_before=$(calls Successful)
echo "Successful calls before restart: $calls_before"

echo -e "\nCold-restarting both loxilbs under the live association\n"
restart_loxilbs_together || { echo "NOK" > /vagrant/status7.txt; exit 1; }
echo -e "\nHA state after restart Master:$master BACKUP-$backup\n"

# The taking-over node builds conntrack from the packets it sees. Give it a
# moment, then sample twice so the counters can show movement. The client
# sends its calls down one path and only heartbeats down the other two, so
# the window has to be long enough for a heartbeat on each.
sleep 10
declare -a p_old p_new
for k in 0 1 2; do p_old[$k]=$(row_pkts "$(fwd_row "${paths[$k]}")"); done
fail_old=$(calls Failed)
sleep 30
for k in 0 1 2; do p_new[$k]=$(row_pkts "$(fwd_row "${paths[$k]}")"); done
fail_new=$(calls Failed)
calls_after=$(calls Successful)

echo -e "\nAfter takeover: conntrack on MASTER $master\n"
ct_rows
echo

for k in 0 1 2; do
    r=$(fwd_row "${paths[$k]}")
    if [[ -z "$r" ]]; then
        echo "Path $((k+1)) ${paths[$k]}: no conntrack after takeover [NOK]"
        code=1
        continue
    fi
    snat=$(row_snat "$r")
    if [[ "$snat" != "${vips[$k]}" ]]; then
        echo "Path $((k+1)) ${paths[$k]}: SNAT source $snat, want ${vips[$k]} [NOK]"
        code=1
    else
        echo "Path $((k+1)) ${paths[$k]}: SNAT source $snat [OK]"
    fi
    if [[ "${p_new[$k]:-0}" -gt "${p_old[$k]:-0}" ]]; then
        echo "Path $((k+1)) ${paths[$k]}: packets ${p_old[$k]} -> ${p_new[$k]} [ACTIVE]"
    else
        echo "Path $((k+1)) ${paths[$k]}: packets ${p_old[$k]:-0} -> ${p_new[$k]:-0} [NOT ACTIVE] [NOK]"
        code=1
    fi
done

if [[ "${fail_new:-0}" -gt "${fail_old:-0}" ]]; then
    printf "Failed Calls:   \t%10s -> %10s \t[INCREASING] [NOK]\n" "$fail_old" "$fail_new"
    code=1
else
    printf "Failed Calls:   \t%10s -> %10s \t[STABLE]\n" "${fail_old:-0}" "${fail_new:-0}"
fi
if [[ "${calls_after:-0}" -gt "${calls_before:-0}" ]]; then
    printf "Successful Calls: \t%10s -> %10s \t[ACTIVE]\n" "$calls_before" "$calls_after"
else
    printf "Successful Calls: \t%10s -> %10s \t[NOT ACTIVE] [NOK]\n" "${calls_before:-0}" "${calls_after:-0}"
    code=1
fi

#Restore
sudo docker exec -dt user ksh -c "sed -i 's/\"call-rate\" value=\"100\"/\"call-rate\" value=\"5000\"/g' /opt/seagull/diameter-env/config/conf.client.xml"
sudo docker exec -i user pkill -f seagull >/dev/null 2>&1
sudo docker exec -i ep1 pkill -f seagull >/dev/null 2>&1

if [[ $code == 0 ]]; then
    echo "sctpmh SCTP Multihoming CT-less Takeover [OK]"
    echo "OK" > /vagrant/status7.txt
else
    echo "NOK" > /vagrant/status7.txt
    echo "sctpmh SCTP Multihoming CT-less Takeover [NOK]"
    echo -e "\nllb1 lb-info"
    $dexec llb1 loxicmd get lb
    echo -e "\nllb2 lb-info"
    $dexec llb2 loxicmd get lb
    echo "-----------------------------"
    exit 1
fi
echo -e "------------------------------------------------------------------------------------\n\n\n"
